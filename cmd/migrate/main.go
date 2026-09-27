package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/frkntplglu/searchly/internal/config"
	"github.com/frkntplglu/searchly/internal/database"
)

func main() {
	file := flag.String("file", "db.sql", "path to the schema file")
	flag.Parse()

	if err := run(*file); err != nil {
		slog.Error("migration failed", "err", err)
		os.Exit(1)
	}
}

func run(file string) error {
	ctx := context.Background()
	db, err := database.Connect(ctx, config.Load().DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	var exists bool
	if err := db.QueryRow(ctx, "SELECT to_regclass('public.contents') IS NOT NULL").Scan(&exists); err != nil {
		return fmt.Errorf("check schema: %w", err)
	}
	if exists {
		slog.Info("schema already exists, skipping", "file", file)
		return nil
	}

	if err := database.Migrate(ctx, db, file); err != nil {
		return err
	}
	slog.Info("schema applied", "file", file)
	return nil
}
