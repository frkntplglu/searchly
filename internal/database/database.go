package database

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

func Migrate(ctx context.Context, db *pgxpool.Pool, path string) error {
	schema, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	if _, err := db.Exec(ctx, string(schema)); err != nil {
		return fmt.Errorf("apply schema %s: %w", path, err)
	}
	return nil
}
