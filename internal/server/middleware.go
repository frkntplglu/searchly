package server

import (
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

// requestLogger logs one line per request with status and duration using slog.
func requestLogger(c fiber.Ctx) error {
	start := time.Now()
	err := c.Next()

	slog.InfoContext(c.Context(), "http request",
		"method", c.Method(),
		"path", c.Path(),
		"status", c.Response().StatusCode(),
		"duration_ms", float64(time.Since(start).Microseconds())/1000,
		"request_id", requestid.FromContext(c),
	)
	return err
}

// logPanic logs a recovered panic with its stack trace.
func logPanic(c fiber.Ctx, v any) {
	slog.ErrorContext(c.Context(), "panic recovered",
		"panic", v,
		"request_id", requestid.FromContext(c),
		"stack", string(debug.Stack()),
	)
}
