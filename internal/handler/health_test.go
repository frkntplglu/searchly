package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func newTestApp() *fiber.App {
	return fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
}

func decodeError(t *testing.T, resp *http.Response) apiError {
	t.Helper()
	var body errorBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body.Error
}

func TestReady(t *testing.T) {
	tests := []struct {
		name       string
		pingErr    error
		wantStatus int
	}{
		{"database reachable", nil, http.StatusOK},
		{"database down", errors.New("connection refused"), http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newTestApp()
			app.Get("/readyz", Ready(fakePinger{err: tt.pingErr}))

			resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestErrorHandler(t *testing.T) {
	tests := []struct {
		name       string
		handler    fiber.Handler
		wantStatus int
		wantCode   string
	}{
		{
			name:       "fiber error keeps status",
			handler:    func(fiber.Ctx) error { return fiber.ErrNotFound },
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "unknown error hides details",
			handler:    func(fiber.Ctx) error { return errors.New("db password leaked") },
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newTestApp()
			app.Get("/", tt.handler)

			resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if got := decodeError(t, resp); got.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", got.Code, tt.wantCode)
			}
		})
	}
}
