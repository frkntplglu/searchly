package ingest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/frkntplglu/searchly/internal/model"
)

type fakeProvider struct {
	name     string
	contents []model.Content
	err      error
}

func (p fakeProvider) Name() string { return p.name }

func (p fakeProvider) Fetch(context.Context) ([]model.Content, error) { return p.contents, p.err }

type fakeStore struct {
	mu     sync.Mutex
	stored []model.Content
	calls  int
	err    error
}

func (s *fakeStore) Upsert(_ context.Context, contents []model.Content) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return s.err
	}
	s.stored = append(s.stored, contents...)
	return nil
}

func (s *fakeStore) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func contents(ids ...string) []model.Content {
	out := make([]model.Content, len(ids))
	for i, id := range ids {
		out[i] = model.Content{ProviderID: id}
	}
	return out
}

func TestRunOnceStoresEveryProvider(t *testing.T) {
	s := &fakeStore{}
	in := New(s,
		fakeProvider{name: "p1", contents: contents("a", "b")},
		fakeProvider{name: "p2", contents: contents("c")},
	)

	if err := in.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(s.stored) != 3 {
		t.Fatalf("stored %d contents, want 3", len(s.stored))
	}
}

func TestRunOnceContinuesAfterAFailingProvider(t *testing.T) {
	fetchErr := errors.New("provider down")
	s := &fakeStore{}
	in := New(s,
		fakeProvider{name: "p1", err: fetchErr},
		fakeProvider{name: "p2", contents: contents("c")},
	)

	err := in.RunOnce(context.Background())
	if !errors.Is(err, fetchErr) {
		t.Fatalf("err = %v, want it to wrap %v", err, fetchErr)
	}
	if len(s.stored) != 1 || s.stored[0].ProviderID != "c" {
		t.Fatalf("stored %+v, want only p2's content", s.stored)
	}
}

func TestRunOnceReportsStoreErrors(t *testing.T) {
	storeErr := errors.New("db down")
	s := &fakeStore{err: storeErr}
	in := New(s,
		fakeProvider{name: "p1", contents: contents("a")},
		fakeProvider{name: "p2", contents: contents("b")},
	)

	err := in.RunOnce(context.Background())
	if !errors.Is(err, storeErr) {
		t.Fatalf("err = %v, want it to wrap %v", err, storeErr)
	}
	if s.callCount() != 2 {
		t.Errorf("store called %d times, want 2: a store error must not skip the next provider", s.callCount())
	}
}

func TestRunIngestsImmediatelyThenOnEveryTick(t *testing.T) {
	s := &fakeStore{}
	in := New(s, fakeProvider{name: "p1", contents: contents("a")})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		in.Run(ctx, 20*time.Millisecond)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for s.callCount() < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after the context was cancelled")
	}
	if s.callCount() < 3 {
		t.Fatalf("store called %d times, want at least 3 (first run plus ticks)", s.callCount())
	}
}

func TestRunKeepsGoingAfterFailedRuns(t *testing.T) {
	s := &fakeStore{}
	in := New(s,
		fakeProvider{name: "broken", err: errors.New("down")},
		fakeProvider{name: "ok", contents: contents("a")},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go in.Run(ctx, 10*time.Millisecond)

	deadline := time.Now().Add(2 * time.Second)
	for s.callCount() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if s.callCount() < 2 {
		t.Fatal("Run stopped ingesting after a run with errors")
	}
}
