package ingest

import (
	"context"
	"errors"
	"fmt"
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

// fakeScheduler mirrors provider_sync: a provider can be claimed when it is
// due and not claimed, and completing it makes it due again after next.
type fakeScheduler struct {
	mu          sync.Mutex
	due         map[string]time.Time
	tokens      map[string]string
	claims      int
	completions []completion
}

type completion struct {
	provider string
	next     time.Duration
	err      error
}

func newFakeScheduler(providers ...string) *fakeScheduler {
	s := &fakeScheduler{due: map[string]time.Time{}, tokens: map[string]string{}}
	for _, p := range providers {
		s.due[p] = time.Now()
	}
	return s
}

func (s *fakeScheduler) Claim(_ context.Context, _ string, providers []string, _ time.Duration) (string, string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range providers {
		if s.tokens[p] == "" && !s.due[p].After(time.Now()) {
			s.claims++
			s.tokens[p] = fmt.Sprint(s.claims)
			return p, s.tokens[p], true, nil
		}
	}
	return "", "", false, nil
}

func (s *fakeScheduler) Complete(_ context.Context, provider, token string, next time.Duration, runErr error) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tokens[provider] != token {
		return false, nil
	}
	s.tokens[provider] = ""
	s.due[provider] = time.Now().Add(next)
	s.completions = append(s.completions, completion{provider, next, runErr})
	return true, nil
}

func (s *fakeScheduler) completed() []completion {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]completion(nil), s.completions...)
}

// blockingProvider's Fetch returns only when its context is done.
type blockingProvider struct{ started chan struct{} }

func (blockingProvider) Name() string { return "blocking" }

func (p blockingProvider) Fetch(ctx context.Context) ([]model.Content, error) {
	close(p.started)
	<-ctx.Done()
	return nil, ctx.Err()
}

func schedule(worker string, interval time.Duration) Schedule {
	return Schedule{WorkerID: worker, Interval: interval, Lease: time.Minute, Poll: 5 * time.Millisecond}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestRunIngestsDueProvidersOnceUntilDueAgain(t *testing.T) {
	st := &fakeStore{}
	sched := newFakeScheduler("p1", "p2")
	in := New(st,
		fakeProvider{name: "p1", contents: contents("a")},
		fakeProvider{name: "p2", contents: contents("b")},
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		in.Run(ctx, sched, schedule("w1", time.Hour))
		close(done)
	}()

	waitFor(t, func() bool { return len(sched.completed()) == 2 })
	time.Sleep(30 * time.Millisecond) // several polls
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after the context was cancelled")
	}
	if st.callCount() != 2 {
		t.Errorf("store called %d times, want 2: providers must not run again before they are due", st.callCount())
	}
	for _, c := range sched.completed() {
		if c.next != time.Hour || c.err != nil {
			t.Errorf("completion %+v, want next=1h and no error", c)
		}
	}
}

func TestRunIngestsAgainWhenDue(t *testing.T) {
	st := &fakeStore{}
	in := New(st, fakeProvider{name: "p1", contents: contents("a")})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go in.Run(ctx, newFakeScheduler("p1"), schedule("w1", 10*time.Millisecond))

	waitFor(t, func() bool { return st.callCount() >= 3 })
}

func TestRunRecordsFailuresAndKeepsGoing(t *testing.T) {
	fetchErr := errors.New("down")
	st := &fakeStore{}
	sched := newFakeScheduler("broken", "ok")
	in := New(st,
		fakeProvider{name: "broken", err: fetchErr},
		fakeProvider{name: "ok", contents: contents("a")},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go in.Run(ctx, sched, schedule("w1", time.Hour))

	waitFor(t, func() bool { return len(sched.completed()) == 2 })
	for _, c := range sched.completed() {
		if wantErr := c.provider == "broken"; errors.Is(c.err, fetchErr) != wantErr {
			t.Errorf("completion %+v: error recorded = %v, want %v", c, c.err != nil, wantErr)
		}
	}
	if st.callCount() != 1 {
		t.Errorf("store called %d times, want 1", st.callCount())
	}
}

func TestWorkersShareProviders(t *testing.T) {
	st := &fakeStore{}
	sched := newFakeScheduler("p1", "p2", "p3", "p4")
	in := New(st,
		fakeProvider{name: "p1", contents: contents("a")},
		fakeProvider{name: "p2", contents: contents("b")},
		fakeProvider{name: "p3", contents: contents("c")},
		fakeProvider{name: "p4", contents: contents("d")},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, w := range []string{"w1", "w2", "w3"} {
		go in.Run(ctx, sched, schedule(w, time.Hour))
	}

	waitFor(t, func() bool { return len(sched.completed()) == 4 })
	time.Sleep(30 * time.Millisecond)
	if st.callCount() != 4 {
		t.Errorf("store called %d times, want 4: each provider exactly once across workers", st.callCount())
	}
}

func TestRunMakesInterruptedProviderDueOnShutdown(t *testing.T) {
	sched := newFakeScheduler("blocking")
	p := blockingProvider{started: make(chan struct{})}
	in := New(&fakeStore{}, p)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		in.Run(ctx, sched, schedule("w1", time.Hour))
		close(done)
	}()

	<-p.started
	cancel()
	<-done

	got := sched.completed()
	if len(got) != 1 || got[0].next != 0 || got[0].err == nil {
		t.Fatalf("completions %+v, want one with next=0 and the run's error", got)
	}
}

func TestRunRejectsInvalidSchedule(t *testing.T) {
	in := New(&fakeStore{}, fakeProvider{name: "p1"})
	valid := schedule("w1", time.Hour)

	for name, change := range map[string]func(*Schedule){
		"zero interval":   func(s *Schedule) { s.Interval = 0 },
		"zero poll":       func(s *Schedule) { s.Poll = 0 },
		"lease too short": func(s *Schedule) { s.Lease = completeTimeout },
		"negative lease":  func(s *Schedule) { s.Lease = -time.Minute },
	} {
		t.Run(name, func(t *testing.T) {
			s := valid
			change(&s)
			if err := in.Run(context.Background(), newFakeScheduler("p1"), s); err == nil {
				t.Fatal("Run accepted an invalid schedule")
			}
		})
	}
}
