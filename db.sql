CREATE TYPE content_type AS ENUM ('video', 'article');

CREATE TABLE contents (
    id            BIGSERIAL PRIMARY KEY,
    provider      TEXT NOT NULL,
    provider_id   TEXT NOT NULL,
    type          content_type NOT NULL,
    title         TEXT NOT NULL,
    published_at  TIMESTAMPTZ NOT NULL,
    tags          TEXT[] NOT NULL DEFAULT '{}',

    views         BIGINT,
    likes         BIGINT,
    duration_sec  INTEGER,

    reading_time  INTEGER,
    reactions     BIGINT,
    comments      INTEGER,

    raw_payload   JSONB NOT NULL,

    UNIQUE (provider, provider_id),

    CONSTRAINT metrics_match_type CHECK (
        (
            type = 'video'
            AND views IS NOT NULL
            AND likes IS NOT NULL
            AND duration_sec IS NOT NULL
            AND reading_time IS NULL
            AND reactions IS NULL
            AND comments IS NULL
        )
        OR
        (
            type = 'article'
            AND views IS NULL
            AND likes IS NULL
            AND duration_sec IS NULL
            AND reading_time IS NOT NULL
            AND reactions IS NOT NULL
            AND comments IS NOT NULL
        )
    )
);