package provider1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/frkntplglu/searchly/internal/provider/httpclient"
)

func readTestdata(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/provider1.json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func serve(t *testing.T, status int, body []byte) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestResponseDecodesSamplePayload(t *testing.T) {
	var resp response
	if err := json.Unmarshal(readTestdata(t), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Contents) != 4 {
		t.Fatalf("got %d contents, want 4", len(resp.Contents))
	}

	c := resp.Contents[0]
	if c.ID != "v1" || c.Title != "Go Programming Tutorial" || c.Type != "video" ||
		c.PublishedAt != "2024-03-15T10:00:00Z" || len(c.Tags) != 2 {
		t.Errorf("unexpected content: %+v", c)
	}

	var m videoMetrics
	if err := json.Unmarshal(c.Metrics, &m); err != nil {
		t.Fatal(err)
	}
	if m != (videoMetrics{Views: 15000, Likes: 1200, Duration: "15:30"}) {
		t.Errorf("metrics = %+v", m)
	}
}

func TestResponseKeepsMalformedMetricsRaw(t *testing.T) {
	// A metrics value of the wrong type must not fail decoding the whole response.
	body := `{"contents":[
		{"id":"ok","type":"video","metrics":{"views":10,"likes":1,"duration":"1:00"}},
		{"id":"bad","type":"article","metrics":{"reading_time":5,"reactions":2,"comments":"22:45"}}
	]}`

	var resp response
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Contents) != 2 {
		t.Fatalf("got %d contents, want 2", len(resp.Contents))
	}
	var m articleMetrics
	if err := json.Unmarshal(resp.Contents[1].Metrics, &m); err == nil {
		t.Error("expected the malformed article metrics to fail on their own")
	}
}

func TestFetch(t *testing.T) {
	p := New(serve(t, http.StatusOK, readTestdata(t)), httpclient.Config{})
	if p.Name() != "provider1" {
		t.Errorf("Name() = %q", p.Name())
	}
	contents, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 4 {
		t.Fatalf("got %d contents, want 4", len(contents))
	}
}

func TestFetchErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		check  func(error) bool
	}{
		{"http status", http.StatusNotFound, "", func(err error) bool {
			var se *httpclient.StatusError
			return errors.As(err, &se) && se.StatusCode == http.StatusNotFound
		}},
		{"malformed json", http.StatusOK, `{"contents": [`, func(err error) bool {
			var se *json.SyntaxError
			return errors.As(err, &se)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(serve(t, tt.status, []byte(tt.body)), httpclient.Config{}).Fetch(context.Background())
			if err == nil || !tt.check(err) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
