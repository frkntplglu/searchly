package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/searchly/internal/config"
	"github.com/searchly/internal/handler"
)

type okPinger struct{}

func (okPinger) Ping(context.Context) error { return nil }

func TestPanicIsRecoveredAs500(t *testing.T) {
	app := New(config.Config{}, okPinger{}, handler.NewContentHandler(nil))
	app.Get("/panic", func(fiber.Ctx) error { panic("boom") })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/panic", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	if resp.Header.Get(fiber.HeaderXRequestID) == "" {
		t.Error("expected X-Request-ID header on error responses")
	}
}

func TestUnknownRouteReturns404JSON(t *testing.T) {
	app := New(config.Config{}, okPinger{}, handler.NewContentHandler(nil))

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/nope", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if ct := resp.Header.Get(fiber.HeaderContentType); !strings.HasPrefix(ct, fiber.MIMEApplicationJSON) {
		t.Errorf("Content-Type = %q, want JSON", ct)
	}
}

func TestAPIDocsAreServed(t *testing.T) {
	app := New(config.Config{}, okPinger{}, handler.NewContentHandler(nil))

	tests := map[string]string{
		"/openapi.yaml": "application/yaml",
		"/docs":         "text/html",
	}
	for path, wantType := range tests {
		t.Run(path, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			if ct := resp.Header.Get(fiber.HeaderContentType); !strings.Contains(ct, wantType) {
				t.Errorf("Content-Type = %q, want %q", ct, wantType)
			}
		})
	}
}

func TestDashboardIsServed(t *testing.T) {
	app := New(config.Config{}, okPinger{}, handler.NewContentHandler(nil))

	tests := map[string]string{
		"/":           "text/html",
		"/index.html": "text/html",
		"/app.js":     "javascript",
		"/style.css":  "text/css",
	}
	for path, wantType := range tests {
		t.Run(path, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			if ct := resp.Header.Get(fiber.HeaderContentType); !strings.Contains(ct, wantType) {
				t.Errorf("Content-Type = %q, want %q", ct, wantType)
			}
			if cc := resp.Header.Get(fiber.HeaderCacheControl); cc != "no-cache" {
				t.Errorf("Cache-Control = %q, want no-cache", cc)
			}
			if lm := resp.Header.Get(fiber.HeaderLastModified); lm != "" {
				t.Errorf("Last-Modified = %q, want none", lm)
			}
		})
	}
}

func TestDashboardIgnoresStaleCacheValidators(t *testing.T) {
	app := New(config.Config{}, okPinger{}, handler.NewContentHandler(nil))

	req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
	req.Header.Set(fiber.HeaderIfModifiedSince, "Mon, 01 Jan 0001 00:00:00 GMT")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: a cached copy from an older build must not be reused", resp.StatusCode)
	}
}
