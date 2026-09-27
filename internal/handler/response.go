package handler

import (
	"github.com/gofiber/fiber/v3"

	"github.com/frkntplglu/searchly/internal/model"
)

func handleSuccess(c fiber.Ctx, data any, pagination *model.Pagination) error {
	return c.JSON(model.Response{Data: data, Pagination: pagination})
}

func handleError(c fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(model.ErrorResponse{Error: model.ErrorDetail{Code: code, Message: message}})
}
