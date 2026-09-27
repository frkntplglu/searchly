package repository

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/searchly/internal/model"
)

type ContentRepository struct {
	db *pgxpool.Pool
}

func NewContentRepository(db *pgxpool.Pool) *ContentRepository {
	return &ContentRepository{db: db}
}

const upsertContentSQL = `
INSERT INTO contents (
    provider, provider_id, type, title, published_at, tags,
    views, likes, duration_sec,
    reading_time, reactions, comments,
    base_score
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT (provider, provider_id) DO UPDATE SET
    type         = EXCLUDED.type,
    title        = EXCLUDED.title,
    published_at = EXCLUDED.published_at,
    tags         = EXCLUDED.tags,
    views        = EXCLUDED.views,
    likes        = EXCLUDED.likes,
    duration_sec = EXCLUDED.duration_sec,
    reading_time = EXCLUDED.reading_time,
    reactions    = EXCLUDED.reactions,
    comments     = EXCLUDED.comments,
    base_score   = EXCLUDED.base_score`

// Upsert inserts the contents or updates the existing rows with the same
// (provider, provider_id). All rows are written in a single transaction.
func (r *ContentRepository) Upsert(ctx context.Context, contents []model.Content) error {
	if len(contents) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, c := range contents {
		var views, likes, reactions, comments *int64
		var durationSec, readingTime *int
		if c.Video != nil {
			views, likes, durationSec = &c.Video.Views, &c.Video.Likes, &c.Video.DurationSec
		}
		if c.Article != nil {
			readingTime, reactions, comments = &c.Article.ReadingTime, &c.Article.Reactions, &c.Article.Comments
		}
		tags := c.Tags
		if tags == nil {
			tags = []string{} // the column is NOT NULL; a nil slice would be sent as NULL
		}

		batch.Queue(upsertContentSQL,
			c.Provider, c.ProviderID, string(c.Type), c.Title, c.PublishedAt, tags,
			views, likes, durationSec,
			readingTime, reactions, comments,
			c.BaseScore(),
		)
	}

	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		return tx.SendBatch(ctx, batch).Close()
	})
	if err != nil {
		return fmt.Errorf("upsert contents: %w", err)
	}
	return nil
}

// scoreExpr is the full score: the stored base score plus recency points for
// the content's age in UTC calendar days. Recency depends on the current date,
// so it is computed at query time instead of being stored. Future-dated
// content counts as just published.
const scoreExpr = `base_score + CASE
        WHEN (now() AT TIME ZONE 'UTC')::date - (published_at AT TIME ZONE 'UTC')::date <= 7  THEN 5
        WHEN (now() AT TIME ZONE 'UTC')::date - (published_at AT TIME ZONE 'UTC')::date <= 30 THEN 3
        WHEN (now() AT TIME ZONE 'UTC')::date - (published_at AT TIME ZONE 'UTC')::date <= 90 THEN 1
        ELSE 0
    END`

const selectContentColumns = `
    id, provider, provider_id, type, title, published_at, tags,
    views, likes, duration_sec,
    reading_time, reactions, comments,
    ` + scoreExpr + ` AS score`

// Search returns one page of contents matching q, in the order requested by
// q.Sort, and the total number of matching contents.
func (r *ContentRepository) Search(ctx context.Context, q model.ContentQuery) ([]model.Content, int, error) {
	where, args := searchFilter(q)

	var total int
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM contents"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count contents: %w", err)
	}

	orderBy, orderArgs := searchOrder(q, len(args))
	args = append(args, orderArgs...)
	args = append(args, q.PerPage, (q.Page-1)*q.PerPage)
	sql := fmt.Sprintf("SELECT %s FROM contents%s ORDER BY %s LIMIT $%d OFFSET $%d",
		selectContentColumns, where, orderBy, len(args)-1, len(args))

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search contents: %w", err)
	}
	contents, err := pgx.CollectRows(rows, scanContent)
	if err != nil {
		return nil, 0, fmt.Errorf("scan contents: %w", err)
	}
	return contents, total, nil
}

// searchFilter returns the WHERE clause for q and its arguments. User input is
// only ever passed as a parameter, never interpolated into the SQL.
//
// Every keyword word is matched as a prefix ("concur" finds "concurrency") so
// results appear while the user is typing. The prefix is matched against the
// stemmed vector and against the unstemmed one, because a partial word longer
// than its stem ("concurren", stem "concurr") only matches the unstemmed words.
func searchFilter(q model.ContentQuery) (string, []any) {
	var conds []string
	var args []any

	if prefix, _ := tsQueries(q.Keyword); prefix != "" {
		args = append(args, prefix)
		n := len(args)
		conds = append(conds, fmt.Sprintf(
			"(search_vector @@ to_tsquery('english', $%d) OR search_vector_simple @@ to_tsquery('simple', $%d))", n, n))
	}
	if q.Type != "" {
		args = append(args, string(q.Type))
		conds = append(conds, fmt.Sprintf("type = $%d::content_type", len(args)))
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// searchOrder returns the ORDER BY clause for q and its arguments, numbered
// after the n filter arguments. For relevance, exact word matches weigh twice
// as much as prefix matches, so "go" ranks "Go" above "Google". The prefix
// score is the better of the two vectors rather than their sum, so a word
// matching in both is not counted twice.
func searchOrder(q model.ContentQuery, n int) (string, []any) {
	prefix, exact := tsQueries(q.Keyword)
	if q.Sort != model.SortRelevance || prefix == "" {
		return "score DESC, id", nil
	}
	return fmt.Sprintf(`ts_rank(search_vector, to_tsquery('english', $%[1]d)) * 2
        + GREATEST(
            ts_rank(search_vector, to_tsquery('english', $%[2]d)),
            ts_rank(search_vector_simple, to_tsquery('simple', $%[2]d))
        ) DESC, score DESC, id`, n+1, n+2), []any{exact, prefix}
}

// tsQueries turns a keyword into to_tsquery input: prefix is "go:* & concur:*"
// and exact is "go & concur". Only letters and digits are kept, so tsquery
// operators in the keyword (& | ! : ( ) and so on) cannot break the query.
// Both are empty when the keyword has no words.
func tsQueries(keyword string) (prefix, exact string) {
	words := strings.FieldsFunc(strings.ToLower(keyword), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(words) == 0 {
		return "", ""
	}

	prefixed := make([]string, len(words))
	for i, w := range words {
		prefixed[i] = w + ":*"
	}
	return strings.Join(prefixed, " & "), strings.Join(words, " & ")
}

func scanContent(row pgx.CollectableRow) (model.Content, error) {
	var (
		c                   model.Content
		typ                 string
		views, likes        *int64
		durationSec         *int
		readingTime         *int
		reactions, comments *int64
	)
	err := row.Scan(
		&c.ID, &c.Provider, &c.ProviderID, &typ, &c.Title, &c.PublishedAt, &c.Tags,
		&views, &likes, &durationSec,
		&readingTime, &reactions, &comments,
		&c.Score,
	)
	if err != nil {
		return model.Content{}, err
	}

	c.Type = model.ContentType(typ)
	c.PublishedAt = c.PublishedAt.UTC()

	// The table's CHECK constraint guarantees the metric columns match the type.
	switch c.Type {
	case model.ContentTypeVideo:
		c.Video = &model.VideoMetrics{Views: *views, Likes: *likes, DurationSec: *durationSec}
	case model.ContentTypeArticle:
		c.Article = &model.ArticleMetrics{ReadingTime: *readingTime, Reactions: *reactions, Comments: *comments}
	}
	return c, nil
}
