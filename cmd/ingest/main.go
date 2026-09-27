package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/frkntplglu/searchly/internal/config"
	"github.com/frkntplglu/searchly/internal/database"
	"github.com/frkntplglu/searchly/internal/ingest"
	"github.com/frkntplglu/searchly/internal/provider/httpclient"
	"github.com/frkntplglu/searchly/internal/provider/provider1"
	"github.com/frkntplglu/searchly/internal/provider/provider2"
	"github.com/frkntplglu/searchly/internal/repository"
	"github.com/frkntplglu/searchly/internal/service"
)

// runTimeout bounds a single run-once invocation.
const runTimeout = 2 * time.Minute

func main() {
	cfg := config.Load()
	interval := flag.Duration("interval", cfg.IngestInterval, "run repeatedly at this interval; 0 runs once and exits")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(cfg, *interval); err != nil {
		slog.Error("ingest failed", "err", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, interval time.Duration) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	// Each provider gets its own client, so each has its own rate limit.
	httpCfg := httpclient.Config{
		RequestsPerSecond: cfg.ProviderRateLimit,
		Timeout:           cfg.ProviderTimeout,
		MaxRetries:        cfg.ProviderMaxRetries,
	}
	ingester := ingest.New(
		service.NewContentService(repository.NewContentRepository(db)),
		provider1.New(cfg.Provider1URL, httpCfg),
		provider2.New(cfg.Provider2URL, httpCfg),
	)

	if interval <= 0 {
		ctx, cancel := context.WithTimeout(ctx, runTimeout)
		defer cancel()
		return ingester.RunOnce(ctx)
	}

	slog.Info("ingest worker started", "interval", interval.String())
	ingester.Run(ctx, interval)
	slog.Info("ingest worker stopped")
	return nil
}
