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

type scheduler interface {
	Claim(ctx context.Context, worker string, providers []string, lease time.Duration) (provider, token string, ok bool, err error)
	Complete(ctx context.Context, provider, token string, next time.Duration, runErr error) (held bool, err error)
}

type Schedule struct {
	WorkerID string
	Interval time.Duration
	Lease    time.Duration
	Poll     time.Duration
}

const completeTimeout = 5 * time.Second

type Ingester struct {
	store     store
	providers []provider
}

func New(store store, providers ...provider) *Ingester {
	return &Ingester{store: store, providers: providers}
}

func (in *Ingester) Names() []string {
	names := make([]string, len(in.providers))
	for i, p := range in.providers {
		names[i] = p.Name()
	}
	return names
}

func (in *Ingester) RunOnce(ctx context.Context) error {
	var errs []error
	for _, p := range in.providers {
		if err := in.ingest(ctx, p); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.Name(), err))
		}
	}
	return errors.Join(errs...)
}

func (in *Ingester) Run(ctx context.Context, sched scheduler, s Schedule) error {
	switch {
	case s.Interval <= 0:
		return fmt.Errorf("interval must be positive, got %s", s.Interval)
	case s.Poll <= 0:
		return fmt.Errorf("poll interval must be positive, got %s", s.Poll)
	case s.Lease <= completeTimeout:
		return fmt.Errorf("lease must be longer than %s, got %s", completeTimeout, s.Lease)
	}

	ticker := time.NewTicker(s.Poll)
	defer ticker.Stop()

	for {
		in.runDue(ctx, sched, s)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (in *Ingester) runDue(ctx context.Context, sched scheduler, s Schedule) {
	names := in.Names()
	for ctx.Err() == nil {
		name, token, ok, err := sched.Claim(ctx, s.WorkerID, names, s.Lease)
		if err != nil {
			if ctx.Err() == nil {
				slog.ErrorContext(ctx, "claim provider failed", "err", err)
			}
			return
		}
		if !ok {
			return
		}

		var runErr error
		if p, found := in.provider(name); found {
			runCtx, cancel := context.WithTimeout(ctx, s.Lease-completeTimeout)
			runErr = in.ingest(runCtx, p)
			cancel()
		} else {
			runErr = fmt.Errorf("unknown provider %q", name)
			slog.ErrorContext(ctx, "claimed unknown provider", "provider", name)
		}

		next := s.Interval
		if ctx.Err() != nil {
			next = 0
		}
		completeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), completeTimeout)
		held, err := sched.Complete(completeCtx, name, token, next, runErr)
		cancel()
		switch {
		case err != nil:
			slog.ErrorContext(ctx, "complete provider failed", "provider", name, "err", err)
		case !held:
			slog.WarnContext(ctx, "provider lease expired before the run finished", "provider", name)
		}
	}
}

func (in *Ingester) provider(name string) (provider, bool) {
	for _, p := range in.providers {
		if p.Name() == name {
			return p, true
		}
	}
	return nil, false
}

func (in *Ingester) ingest(ctx context.Context, p provider) error {
	start := time.Now()
	contents, err := p.Fetch(ctx)
	if err == nil {
		err = in.store.Upsert(ctx, contents)
	}
	if err != nil {
		slog.ErrorContext(ctx, "provider failed", "provider", p.Name(), "err", err)
		return err
	}
	slog.InfoContext(ctx, "provider ingested", "provider", p.Name(), "contents", len(contents),
		"duration_ms", float64(time.Since(start).Microseconds())/1000)
	return nil
}
