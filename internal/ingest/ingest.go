// Package ingest fetches content from the providers and stores it.
package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/frkntplglu/searchly/internal/model"
)

type provider interface {
	Name() string
	Fetch(ctx context.Context) ([]model.Content, error)
}

type store interface {
	Upsert(ctx context.Context, contents []model.Content) error
}

type Ingester struct {
	store     store
	providers []provider
}

func New(store store, providers ...provider) *Ingester {
	return &Ingester{store: store, providers: providers}
}

func (in *Ingester) RunOnce(ctx context.Context) error {
	var errs []error
	for _, p := range in.providers {
		start := time.Now()
		contents, err := p.Fetch(ctx)
		if err == nil {
			err = in.store.Upsert(ctx, contents)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.Name(), err))
			slog.ErrorContext(ctx, "provider failed", "provider", p.Name(), "err", err)
			continue
		}
		slog.InfoContext(ctx, "provider ingested", "provider", p.Name(), "contents", len(contents),
			"duration_ms", float64(time.Since(start).Microseconds())/1000)
	}
	return errors.Join(errs...)
}

func (in *Ingester) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if err := in.RunOnce(ctx); err != nil && ctx.Err() == nil {
			slog.WarnContext(ctx, "ingest run finished with errors", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
