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
	if err != nil {
		// Let the app's ErrorHandler write the response so the logged status is final.
		if handlerErr := c.App().Config().ErrorHandler(c, err); handlerErr != nil {
			_ = c.SendStatus(fiber.StatusInternalServerError)
		}
	}

	slog.InfoContext(c.Context(), "http request",
		"method", c.Method(),
		"path", c.Path(),
		"status", c.Response().StatusCode(),
		"duration", time.Since(start),
		"request_id", requestid.FromContext(c),
	)
	return nil
}

// logPanic logs a recovered panic with its stack trace. The panic itself is
// turned into a 500 response by the recover middleware and ErrorHandler.
func logPanic(c fiber.Ctx, v any) {
	slog.ErrorContext(c.Context(), "panic recovered",
		"panic", v,
		"request_id", requestid.FromContext(c),
		"stack", string(debug.Stack()),
	)
}
