package server

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/gofiber/fiber/v3/middleware/static"

	"github.com/frkntplglu/searchly/api"
	"github.com/frkntplglu/searchly/internal/config"
	"github.com/frkntplglu/searchly/internal/dashboard"
	"github.com/frkntplglu/searchly/internal/handler"
)

func New(cfg config.Config, contents *handler.ContentHandler) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "searchly",
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	app.Use(requestid.New())
	app.Use(requestLogger)
	app.Use(recover.New(recover.Config{
		EnableStackTrace:  true,
		StackTraceHandler: logPanic,
	}))

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	v1 := app.Group("/api/v1")
	v1.Get("/contents", contents.Search)

	app.Get("/openapi.yaml", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, "application/yaml")
		return c.Send(api.Spec)
	})
	app.Get("/docs", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(api.DocsPage)
	})

	// Registered last so the API routes above take precedence.
	app.Get("/*", static.New("", static.Config{FS: dashboard.FS()}))

	return app
}
