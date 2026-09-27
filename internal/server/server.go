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
		ErrorHandler: handler.ErrorHandler,
	})

	app.Use(requestid.New())
	app.Use(requestLogger)
	app.Use(recover.New(recover.Config{
		EnableStackTrace:  true,
		StackTraceHandler: logPanic,
	}))

	app.Get("/health", handler.Health)

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
	//
	// Embedded files have no modification time, so the file server reports a
	// Last-Modified of year 0001 and answers any If-Modified-Since with 304, even
	// after a rebuild. The files are small: drop the validators and always send
	// them fresh.
	app.Get("/*", func(c fiber.Ctx) error {
		c.Request().Header.Del(fiber.HeaderIfModifiedSince)
		return c.Next()
	}, static.New("", static.Config{
		FS: dashboard.FS(),
		ModifyResponse: func(c fiber.Ctx) error {
			c.Response().Header.Del(fiber.HeaderLastModified)
			c.Set(fiber.HeaderCacheControl, "no-cache")
			return nil
		},
	}))

	return app
}
