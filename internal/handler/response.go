package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteError writes an error in the API's standard error format.
func WriteError(c fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(errorBody{Error: apiError{Code: code, Message: message}})
}

// ErrorHandler converts errors returned from handlers (including recovered panics)
// into the standard error format. Internal details are logged, never exposed.
func ErrorHandler(c fiber.Ctx, err error) error {
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return WriteError(c, fe.Code, errorCode(fe.Code), fe.Message)
	}

	slog.ErrorContext(c.Context(), "unhandled error",
		"err", err,
		"method", c.Method(),
		"path", c.Path(),
		"request_id", requestid.FromContext(c),
	)
	return WriteError(c, fiber.StatusInternalServerError, "internal_error", "internal server error")
}

// errorCode maps an HTTP status to a snake_case code, e.g. 404 -> "not_found".
func errorCode(status int) string {
	text := http.StatusText(status)
	if text == "" {
		return "error"
	}
	return strings.ReplaceAll(strings.ToLower(text), " ", "_")
}
