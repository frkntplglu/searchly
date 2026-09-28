package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SyncRepository schedules providers across ingest workers through the
// provider_sync table.
type SyncRepository struct {
	db *pgxpool.Pool
}

func NewSyncRepository(db *pgxpool.Pool) *SyncRepository {
	return &SyncRepository{db: db}
}

func (r *SyncRepository) Register(ctx context.Context, providers []string) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO provider_sync (provider)
SELECT unnest($1::text[])
ON CONFLICT (provider) DO NOTHING`, providers)
	if err != nil {
		return fmt.Errorf("register providers: %w", err)
	}
	return nil
}

func (r *SyncRepository) Claim(ctx context.Context, worker string, providers []string, lease time.Duration) (provider, token string, ok bool, err error) {
	err = r.db.QueryRow(ctx, `
UPDATE provider_sync
SET claim_token = gen_random_uuid(), locked_by = $1, locked_until = now() + make_interval(secs => $2)
WHERE provider = (
    SELECT provider FROM provider_sync
    WHERE provider = ANY($3)
      AND next_run_at <= now()
      AND (locked_until IS NULL OR locked_until < now())
    ORDER BY next_run_at
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING provider, claim_token::text`, worker, lease.Seconds(), providers).Scan(&provider, &token)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("claim provider: %w", err)
	}
	return provider, token, true, nil
}

func (r *SyncRepository) Complete(ctx context.Context, provider, token string, next time.Duration, runErr error) (held bool, err error) {
	var errText *string
	if runErr != nil {
		s := runErr.Error()
		errText = &s
	}
	tag, err := r.db.Exec(ctx, `
UPDATE provider_sync
SET claim_token = NULL,
    locked_by = NULL,
    locked_until = NULL,
    next_run_at = now() + make_interval(secs => $3),
    last_success_at = CASE WHEN $4::text IS NULL THEN now() ELSE last_success_at END,
    last_error = $4
WHERE provider = $1 AND claim_token = $2::uuid`, provider, token, next.Seconds(), errText)
	if err != nil {
		return false, fmt.Errorf("complete provider: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
