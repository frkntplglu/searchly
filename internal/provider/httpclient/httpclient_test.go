package httpclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// newServer responds with the given status codes in order, then 200 "ok".
func newServer(t *testing.T, statuses ...int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := int(calls.Add(1)) - 1
		if n < len(statuses) {
			w.WriteHeader(statuses[n])
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestGetRetries(t *testing.T) {
	tests := []struct {
		name       string
		statuses   []int
		wantStatus int // 0 means success
		wantCalls  int32
	}{
		{"success", nil, 0, 1},
		{"retries 5xx then succeeds", []int{500, 503}, 0, 3},
		{"retries 429 then succeeds", []int{429}, 0, 2},
		{"gives up after max retries", []int{502, 502, 502, 502}, 502, 4},
		{"does not retry 4xx", []int{404}, 404, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, calls := newServer(t, tt.statuses...)
			c := New(srv.URL, Config{MaxRetries: 3, Backoff: time.Millisecond})

			body, err := c.Get(context.Background(), nil)

			if got := calls.Load(); got != tt.wantCalls {
				t.Errorf("calls = %d, want %d", got, tt.wantCalls)
			}
			if tt.wantStatus == 0 {
				if err != nil || string(body) != "ok" {
					t.Fatalf("got %q, %v; want ok", body, err)
				}
				return
			}
			var se *StatusError
			if !errors.As(err, &se) || se.StatusCode != tt.wantStatus {
				t.Fatalf("err = %v, want StatusError %d", err, tt.wantStatus)
			}
		})
	}
}

func TestGetHonorsRetryAfter(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	start := time.Now()
	if _, err := New(srv.URL, Config{MaxRetries: 1, Backoff: time.Millisecond}).Get(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < time.Second {
		t.Fatalf("retried after %v, want at least the 1s Retry-After", elapsed)
	}
}

func TestRateLimit(t *testing.T) {
	srv, _ := newServer(t)
	c := New(srv.URL, Config{RequestsPerSecond: 20})

	start := time.Now()
	for range 4 {
		if _, err := c.Get(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
	}
	// At 20 req/s with burst 1, requests 2-4 each wait ~50ms.
	if elapsed := time.Since(start); elapsed < 140*time.Millisecond {
		t.Fatalf("4 requests took %v; rate limit not enforced", elapsed)
	}
}

func TestSharedLimiter(t *testing.T) {
	srv, _ := newServer(t)
	limiter := rate.NewLimiter(20, 1)
	a := New(srv.URL, Config{Limiter: limiter})
	b := New(srv.URL, Config{Limiter: limiter})

	start := time.Now()
	for range 2 {
		if _, err := a.Get(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
		if _, err := b.Get(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
	}
	// Four requests from two clients share one 20 req/s budget.
	if elapsed := time.Since(start); elapsed < 140*time.Millisecond {
		t.Fatalf("4 requests took %v; limiter not shared", elapsed)
	}
}

func TestTimeoutIsRetriedThenFails(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		select {
		case <-time.After(time.Second):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	_, err := New(srv.URL, Config{Timeout: 20 * time.Millisecond, MaxRetries: 1, Backoff: time.Millisecond}).Get(context.Background(), nil)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("calls = %d, want 2", got)
	}
}

func TestGetStopsOnContextCancel(t *testing.T) {
	srv, calls := newServer(t, 500, 500, 500)
	c := New(srv.URL, Config{MaxRetries: 3, Backoff: time.Second})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if _, err := c.Get(ctx, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("calls = %d, want 1", got)
	}
}

func TestGetAddsQuery(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = r.URL.RawQuery
	}))
	t.Cleanup(srv.Close)

	if _, err := New(srv.URL+"/feed?lang=en", Config{}).Get(context.Background(), url.Values{"page": {"2"}}); err != nil {
		t.Fatal(err)
	}
	if got != "lang=en&page=2" {
		t.Errorf("query = %q, want the URL's own query plus page=2", got)
	}
}
