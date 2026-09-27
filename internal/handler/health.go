package handler

import "github.com/gofiber/fiber/v3"

// Health reports that the process is alive.
func Health(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}
