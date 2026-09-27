// Command ingest fetches content from every provider once and stores it.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/searchly/internal/config"
	"github.com/searchly/internal/database"
	"github.com/searchly/internal/model"
	"github.com/searchly/internal/provider/httpclient"
	"github.com/searchly/internal/provider/provider1"
	"github.com/searchly/internal/provider/provider2"
	"github.com/searchly/internal/repository"
	"github.com/searchly/internal/service"
)

type provider interface {
	Name() string
	Fetch(ctx context.Context) ([]model.Content, error)
}

func main() {
	cfg := config.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(cfg); err != nil {
		slog.Error("ingest failed", "err", err)
		os.Exit(1)
	}
}

func run(cfg config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	contents := service.NewContentService(repository.NewContentRepository(db))

	// Each provider gets its own client, so each has its own rate limit.
	httpCfg := httpclient.Config{
		RequestsPerSecond: cfg.ProviderRateLimit,
		Timeout:           cfg.ProviderTimeout,
		MaxRetries:        cfg.ProviderMaxRetries,
	}
	providers := []provider{
		provider1.New(cfg.Provider1URL, httpCfg),
		provider2.New(cfg.Provider2URL, httpCfg),
	}

	// A failing provider is reported but does not stop the others.
	var errs []error
	for _, p := range providers {
		start := time.Now()
		items, err := p.Fetch(ctx)
		if err == nil {
			err = contents.Upsert(ctx, items)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.Name(), err))
			slog.Error("provider failed", "provider", p.Name(), "err", err)
			continue
		}
		slog.Info("provider ingested", "provider", p.Name(), "contents", len(items),
			"duration_ms", float64(time.Since(start).Microseconds())/1000)
	}
	return errors.Join(errs...)
}
