package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/searchly/internal/config"
)

type okPinger struct{}

func (okPinger) Ping(context.Context) error { return nil }

func TestPanicIsRecoveredAs500(t *testing.T) {
	app := New(config.Config{}, okPinger{})
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
	app := New(config.Config{}, okPinger{})

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
