CREATE TYPE content_type AS ENUM ('video', 'article');

-- array_to_string is only STABLE, but generated columns require IMMUTABLE
-- expressions. Joining a TEXT[] with a fixed separator is deterministic.
CREATE FUNCTION tags_to_text(tags TEXT[]) RETURNS TEXT
    LANGUAGE sql IMMUTABLE PARALLEL SAFE
    AS $$ SELECT array_to_string(tags, ' ') $$;

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

    base_score    DOUBLE PRECISION NOT NULL,

    -- Title matches (weight A) rank above tag matches (weight B).
    search_vector TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('english', title), 'A') ||
        setweight(to_tsvector('english', tags_to_text(tags)), 'B')
    ) STORED,

    -- The same words without stemming, so a partial word longer than its stem
    -- still matches as a prefix ("concurren" -> "concurrency", stem "concurr").
    search_vector_simple TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', title), 'A') ||
        setweight(to_tsvector('simple', tags_to_text(tags)), 'B')
    ) STORED,

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

CREATE INDEX contents_search_idx ON contents USING GIN (search_vector);
CREATE INDEX contents_search_simple_idx ON contents USING GIN (search_vector_simple);
