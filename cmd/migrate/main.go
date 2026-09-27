// Command migrate creates the database schema from db.sql. Run it once against
// an empty database before starting the service.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/searchly/internal/config"
	"github.com/searchly/internal/database"
)

func main() {
	file := flag.String("file", "db.sql", "path to the schema file")
	flag.Parse()

	if err := run(*file); err != nil {
		slog.Error("migration failed", "err", err)
		os.Exit(1)
	}
	slog.Info("schema applied", "file", *file)
}

func run(file string) error {
	ctx := context.Background()
	db, err := database.Connect(ctx, config.Load().DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	return database.Migrate(ctx, db, file)
}
