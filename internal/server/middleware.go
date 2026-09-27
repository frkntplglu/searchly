package server

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"github.com/frkntplglu/searchly/internal/model"
)

// requestLogger logs one line per request with status and duration using slog.
func requestLogger(c fiber.Ctx) error {
	start := time.Now()
	if err := c.Next(); err != nil {
		if err := c.App().Config().ErrorHandler(c, err); err != nil {
			_ = c.SendStatus(fiber.StatusInternalServerError)
		}
	}

	slog.InfoContext(c.Context(), "http request",
		"method", c.Method(),
		"path", c.Path(),
		"status", c.Response().StatusCode(),
		"duration_ms", float64(time.Since(start).Microseconds())/1000,
		"request_id", requestid.FromContext(c),
	)
	return nil
}
func errorHandler(c fiber.Ctx, err error) error {
	status, code, message := fiber.StatusInternalServerError, "internal_error", "internal server error"

	var fe *fiber.Error
	if errors.As(err, &fe) {
		status, message = fe.Code, fe.Message
		code = strings.ReplaceAll(strings.ToLower(http.StatusText(fe.Code)), " ", "_") // 404 -> not_found
	} else {
		slog.ErrorContext(c.Context(), "unhandled error", "err", err, "request_id", requestid.FromContext(c))
	}

	return c.Status(status).JSON(model.ErrorResponse{Error: model.ErrorDetail{Code: code, Message: message}})
}

// logPanic logs a recovered panic with its stack trace.
func logPanic(c fiber.Ctx, v any) {
	slog.ErrorContext(c.Context(), "panic recovered",
		"panic", v,
		"request_id", requestid.FromContext(c),
		"stack", string(debug.Stack()),
	)
}
