package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/frkntplglu/searchly/internal/config"
	"github.com/frkntplglu/searchly/internal/database"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migration failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	db, err := database.Connect(ctx, config.Load().DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := database.Migrate(ctx, db); err != nil {
		return err
	}
	slog.Info("database is up to date")
	return nil
}
