// Package httpclient fetches a single URL with a per-client rate limit,
// request timeout and retries for transient failures. It is the HTTP layer
// shared by jsonprovider and xmlprovider.
package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/time/rate"
)

const maxBodyBytes = 10 << 20 // 10 MiB

type Config struct {
	// RequestsPerSecond limits outgoing requests. Zero or negative means unlimited.
	RequestsPerSecond float64
	// Limiter, if set, is used instead of RequestsPerSecond. Share one Limiter
	// between clients that call the same provider so they draw from one budget.
	Limiter *rate.Limiter
	// Timeout bounds a single HTTP attempt. Zero means no timeout.
	Timeout time.Duration
	// MaxRetries is how many times a transient failure (network error, 5xx, 429) is retried.
	MaxRetries int
	// Backoff is the wait before the first retry, doubled on each attempt. Defaults to 500ms.
	Backoff time.Duration
}

// StatusError is returned when the server responds with a non-200 status.
type StatusError struct {
	URL        string
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("GET %s: status %d", e.URL, e.StatusCode)
}

type Client struct {
	url        string
	http       *http.Client
	limiter    *rate.Limiter
	maxRetries int
	backoff    time.Duration
}

func New(url string, cfg Config) *Client {
	limiter := cfg.Limiter
	if limiter == nil {
		limit := rate.Inf
		if cfg.RequestsPerSecond > 0 {
			limit = rate.Limit(cfg.RequestsPerSecond)
		}
		limiter = rate.NewLimiter(limit, 1)
	}
	backoff := cfg.Backoff
	if backoff <= 0 {
		backoff = 500 * time.Millisecond
	}
	return &Client{
		url:        url,
		http:       &http.Client{Timeout: cfg.Timeout},
		limiter:    limiter,
		maxRetries: cfg.MaxRetries,
		backoff:    backoff,
	}
}

// Get returns the response body of the configured URL.
func (c *Client) Get(ctx context.Context) ([]byte, error) {
	wait := c.backoff
	for attempt := 0; ; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}

		body, retryAfter, err := c.do(ctx)
		if err == nil {
			return body, nil
		}
		if retryAfter < 0 || attempt == c.maxRetries || ctx.Err() != nil {
			return nil, err
		}

		if retryAfter > 0 {
			wait = retryAfter
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		wait *= 2
	}
}

// do performs a single request. retryAfter is negative when the failure is not
// worth retrying, zero to use the default backoff, or the server-requested delay.
func (c *Client) do(ctx context.Context) (body []byte, retryAfter time.Duration, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, -1, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		body, err = io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
		return body, 0, err
	case resp.StatusCode == http.StatusTooManyRequests:
		secs, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
		return nil, time.Duration(secs) * time.Second, &StatusError{URL: c.url, StatusCode: resp.StatusCode}
	case resp.StatusCode >= 500:
		return nil, 0, &StatusError{URL: c.url, StatusCode: resp.StatusCode}
	default:
		return nil, -1, &StatusError{URL: c.url, StatusCode: resp.StatusCode}
	}
}
