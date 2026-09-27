package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/frkntplglu/searchly/internal/config"
	"github.com/frkntplglu/searchly/internal/handler"
)

func TestPanicIsRecoveredAs500(t *testing.T) {
	app := New(config.Config{}, handler.NewContentHandler(nil))
	app.Get("/panic", func(fiber.Ctx) error { panic("db password is secret") })

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
	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"error":{"code":"internal_error","message":"internal server error"}}` {
		t.Errorf("body = %s; the panic value must be logged, never returned", body)
	}
}

func TestUnknownRouteReturns404JSON(t *testing.T) {
	app := New(config.Config{}, handler.NewContentHandler(nil))

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/nope", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.HasPrefix(string(body), `{"error":{"code":"not_found"`) {
		t.Errorf("body = %s, want the standard error envelope", body)
	}
}

func TestAPIDocsAreServed(t *testing.T) {
	app := New(config.Config{}, handler.NewContentHandler(nil))

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
	app := New(config.Config{}, handler.NewContentHandler(nil))

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
		})
	}
}
