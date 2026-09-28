package main

import (
	"context"
	"flag"
	"fmt"
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
		ctx, cancel := context.WithTimeout(ctx, cfg.IngestRunTimeout)
		defer cancel()
		return ingester.RunOnce(ctx)
	}

	syncRepo := repository.NewSyncRepository(db)
	if err := syncRepo.Register(ctx, ingester.Names()); err != nil {
		return err
	}
	host, _ := os.Hostname()
	schedule := ingest.Schedule{
		WorkerID: fmt.Sprintf("%s-%d", host, os.Getpid()),
		Interval: interval,
		Lease:    cfg.IngestRunTimeout,
		Poll:     cfg.IngestPollInterval,
	}

	slog.Info("ingest worker started", "worker", schedule.WorkerID, "interval", interval.String())
	if err := ingester.Run(ctx, syncRepo, schedule); err != nil {
		return err
	}
	slog.Info("ingest worker stopped")
	return nil
}
