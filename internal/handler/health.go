package handler

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
)

// Pinger reports whether a dependency is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Health reports that the process is alive.
func Health(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

// Ready reports whether the service's dependencies are reachable.
func Ready(db Pinger) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			slog.WarnContext(ctx, "readiness check failed", "dependency", "postgres", "err", err)
			return WriteError(c, fiber.StatusServiceUnavailable, "not_ready", "database is not reachable")
		}
		return c.JSON(fiber.Map{"status": "ready"})
	}
}
