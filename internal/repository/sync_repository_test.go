//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func newSyncRepo(t *testing.T, providers ...string) (*SyncRepository, *pgxpool.Pool) {
	t.Helper()
	db := newTestDB(t)
	r := NewSyncRepository(db)
	if err := r.Register(context.Background(), providers); err != nil {
		t.Fatal(err)
	}
	return r, db
}

// claim returns the claimed provider and the lease's token, or empty strings when nothing was due.
func claim(t *testing.T, r *SyncRepository, worker string, lease time.Duration, providers ...string) (provider, token string) {
	t.Helper()
	provider, token, _, err := r.Claim(context.Background(), worker, providers, lease)
	if err != nil {
		t.Fatal(err)
	}
	return provider, token
}

func TestSyncRegisterKeepsExistingSchedule(t *testing.T) {
	r, _ := newSyncRepo(t, "p1")
	ctx := context.Background()

	_, token := claim(t, r, "w1", time.Minute, "p1")
	if _, err := r.Complete(ctx, "p1", token, time.Hour, nil); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(ctx, []string{"p1", "p2"}); err != nil {
		t.Fatal(err)
	}

	if p, _ := claim(t, r, "w1", time.Minute, "p1", "p2"); p != "p2" {
		t.Fatalf("claimed %q, want the new p2: registering again must not make p1 due", p)
	}
}

func TestSyncClaimHandsEachProviderToOneWorker(t *testing.T) {
	r, _ := newSyncRepo(t, "p1", "p2")

	a, _ := claim(t, r, "w1", time.Minute, "p1", "p2")
	b, _ := claim(t, r, "w2", time.Minute, "p1", "p2")
	if a == "" || b == "" || a == b {
		t.Fatalf("claimed %q and %q, want p1 and p2 once each", a, b)
	}
	if p, _ := claim(t, r, "w3", time.Minute, "p1", "p2"); p != "" {
		t.Fatalf("claimed %q, want nothing while both are leased", p)
	}
}

func TestSyncClaimOnlyGivenProviders(t *testing.T) {
	r, _ := newSyncRepo(t, "p1", "p2")

	if p, _ := claim(t, r, "w1", time.Minute, "p2"); p != "p2" {
		t.Fatalf("claimed %q, want p2", p)
	}
	if p, _ := claim(t, r, "w1", time.Minute, "p2", "unknown"); p != "" {
		t.Fatalf("claimed %q, want nothing", p)
	}
}

func TestSyncConcurrentClaims(t *testing.T) {
	r, _ := newSyncRepo(t, "p1")

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		wins int
	)
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, ok, err := r.Claim(context.Background(), fmt.Sprintf("w%d", i), []string{"p1"}, time.Minute)
			if err != nil {
				t.Error(err)
			}
			if ok {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("%d workers claimed p1, want exactly 1", wins)
	}
}

func TestSyncCompleteSchedulesNextRunAndRecordsResult(t *testing.T) {
	r, db := newSyncRepo(t, "p1")
	ctx := context.Background()

	_, token := claim(t, r, "w1", time.Minute, "p1")
	if held, err := r.Complete(ctx, "p1", token, time.Hour, errors.New("provider down")); err != nil || !held {
		t.Fatalf("Complete = %v, %v, want held", held, err)
	}
	if p, _ := claim(t, r, "w1", time.Minute, "p1"); p != "" {
		t.Fatalf("claimed %q, want nothing before the next run is due", p)
	}

	var (
		lockedBy    *string
		lastError   *string
		lastSuccess *time.Time
		inAnHour    bool
	)
	err := db.QueryRow(ctx, `
SELECT locked_by, last_error, last_success_at, next_run_at > now() + interval '59 minutes'
FROM provider_sync WHERE provider = 'p1'`).Scan(&lockedBy, &lastError, &lastSuccess, &inAnHour)
	if err != nil {
		t.Fatal(err)
	}
	if lockedBy != nil || lastError == nil || *lastError != "provider down" || lastSuccess != nil || !inAnHour {
		t.Fatalf("locked_by=%v last_error=%v last_success_at=%v next run in an hour=%v", lockedBy, lastError, lastSuccess, inAnHour)
	}

	// A later success clears the error.
	if _, err := db.Exec(ctx, "UPDATE provider_sync SET next_run_at = now()"); err != nil {
		t.Fatal(err)
	}
	_, token = claim(t, r, "w1", time.Minute, "p1")
	if _, err := r.Complete(ctx, "p1", token, 0, nil); err != nil {
		t.Fatal(err)
	}
	err = db.QueryRow(ctx, "SELECT last_error, last_success_at FROM provider_sync WHERE provider = 'p1'").Scan(&lastError, &lastSuccess)
	if err != nil {
		t.Fatal(err)
	}
	if lastError != nil || lastSuccess == nil {
		t.Fatalf("last_error=%v last_success_at=%v, want the error cleared and a success time", lastError, lastSuccess)
	}
}

func TestSyncExpiredLeaseIsTakenOver(t *testing.T) {
	// The same worker ID on both sides: leases are told apart by their token,
	// so this also covers two workers that were started with the same ID.
	for _, second := range []string{"w2", "w1"} {
		t.Run(second, func(t *testing.T) {
			r, _ := newSyncRepo(t, "p1")
			ctx := context.Background()

			_, oldToken := claim(t, r, "w1", 50*time.Millisecond, "p1")
			time.Sleep(100 * time.Millisecond)

			p, newToken := claim(t, r, second, time.Minute, "p1")
			if p != "p1" {
				t.Fatalf("claimed %q, want p1 after the first lease expired", p)
			}
			if held, err := r.Complete(ctx, "p1", oldToken, time.Hour, nil); err != nil || held {
				t.Fatalf("Complete with the expired lease = %v, %v, want not held", held, err)
			}
			if held, err := r.Complete(ctx, "p1", newToken, time.Hour, nil); err != nil || !held {
				t.Fatalf("Complete with the current lease = %v, %v, want held", held, err)
			}
		})
	}
}

func TestSyncExpiredLeaseCanStillComplete(t *testing.T) {
	r, _ := newSyncRepo(t, "p1")

	_, token := claim(t, r, "w1", 50*time.Millisecond, "p1")
	time.Sleep(100 * time.Millisecond)

	// Nobody claimed it since, so the finished run's result is still recorded.
	if held, err := r.Complete(context.Background(), "p1", token, time.Hour, nil); err != nil || !held {
		t.Fatalf("Complete = %v, %v, want held", held, err)
	}
}
