-- +goose Up
-- One row per provider. Ingest workers claim due providers from here, so any
-- number of workers share the providers instead of each ingesting all of them.
CREATE TABLE provider_sync (
    provider         TEXT PRIMARY KEY,
    next_run_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- The current lease: a new token per claim, the worker holding it (for
    -- visibility) and until when. An expired lease can be claimed by another
    -- worker, so a crashed worker does not block a provider.
    claim_token      UUID,
    locked_by        TEXT,
    locked_until     TIMESTAMPTZ,
    last_success_at  TIMESTAMPTZ,
    last_error       TEXT
);

-- +goose Down
DROP TABLE provider_sync;
