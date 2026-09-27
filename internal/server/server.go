package server

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"github.com/searchly/internal/config"
	"github.com/searchly/internal/handler"
)

func New(cfg config.Config, db handler.Pinger) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "searchly",
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		ErrorHandler: handler.ErrorHandler,
	})

	app.Use(requestid.New())
	app.Use(requestLogger)
	app.Use(recover.New(recover.Config{
		EnableStackTrace:  true,
		StackTraceHandler: logPanic,
	}))

	app.Get("/healthz", handler.Health)
	app.Get("/readyz", handler.Ready(db))

	return app
}
