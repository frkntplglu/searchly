//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/frkntplglu/searchly/internal/database"
	"github.com/frkntplglu/searchly/internal/model"
)

const (
	defaultAdminURL = "postgres://searchly:searchly@localhost:5432/searchly?sslmode=disable"
	testDBName      = "searchly_test"
)

// newTestDB returns a pool to an empty searchly_test database with the schema
// from db.sql, creating the database if needed. The development database is never touched.
func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	adminURL := os.Getenv("DATABASE_URL")
	if adminURL == "" {
		adminURL = defaultAdminURL
	}
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect admin db: %v", err)
	}
	defer admin.Close(ctx)

	_, err = admin.Exec(ctx, "CREATE DATABASE "+testDBName)
	var pgErr *pgconn.PgError
	if err != nil && !(errors.As(err, &pgErr) && pgErr.Code == "42P04") { // duplicate_database
		t.Fatalf("create test db: %v", err)
	}

	cfg := admin.Config()
	testURL := strings.Replace(adminURL, "/"+cfg.Database+"?", "/"+testDBName+"?", 1)
	pool, err := database.Connect(ctx, testURL)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)

	// Recreate the schema from db.sql so every test starts from the real, empty schema.
	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if err := database.Migrate(ctx, pool, "../../db.sql"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

func video(id, title string, published time.Time, tags ...string) model.Content {
	return model.Content{
		Provider: "provider1", ProviderID: id, Type: model.ContentTypeVideo, Title: title,
		PublishedAt: published, Tags: tags,
		Video: &model.VideoMetrics{Views: 1000, Likes: 100, DurationSec: 930},
	}
}

func article(id, title string, published time.Time, tags ...string) model.Content {
	return model.Content{
		Provider: "provider2", ProviderID: id, Type: model.ContentTypeArticle, Title: title,
		PublishedAt: published, Tags: tags,
		Article: &model.ArticleMetrics{ReadingTime: 8, Reactions: 450, Comments: 25},
	}
}

var day = time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)

func TestUpsertAndSearchRoundTrip(t *testing.T) {
	repo := NewContentRepository(newTestDB(t))
	ctx := context.Background()

	want := []model.Content{
		video("v1", "Go Tutorial", day.AddDate(0, 0, 2), "go"),
		article("a1", "Clean Architecture", day.AddDate(0, 0, 1)),
	}
	if err := repo.Upsert(ctx, want); err != nil {
		t.Fatal(err)
	}

	got, total, err := repo.Search(ctx, model.ContentQuery{Sort: model.SortPopularity, Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(got) != 2 {
		t.Fatalf("total=%d len=%d, want 2", total, len(got))
	}

	a, v := got[0], got[1] // the article's base score (298.25) beats the video's (4)
	if v.ID == 0 || v.ProviderID != "v1" || v.Video == nil || *v.Video != *want[0].Video || v.Article != nil {
		t.Errorf("video round trip: %+v", v)
	}
	if !v.PublishedAt.Equal(want[0].PublishedAt) || len(v.Tags) != 1 {
		t.Errorf("video fields: published=%v tags=%v", v.PublishedAt, v.Tags)
	}
	if a.Article == nil || *a.Article != *want[1].Article || a.Video != nil {
		t.Errorf("article round trip: %+v", a)
	}
	// Both are dated 2024, so no recency points: score == base score.
	if v.Score != 4 || a.Score != 298.25 {
		t.Errorf("scores = %v, %v; want 4, 298.25", v.Score, a.Score)
	}
	if a.Tags == nil {
		t.Error("nil tags must be stored as an empty array")
	}
}

func TestUpsertUpdatesExistingRow(t *testing.T) {
	repo := NewContentRepository(newTestDB(t))
	ctx := context.Background()

	if err := repo.Upsert(ctx, []model.Content{video("v1", "Old title", day)}); err != nil {
		t.Fatal(err)
	}
	updated := video("v1", "New title", day)
	updated.Video.Views = 5000
	if err := repo.Upsert(ctx, []model.Content{updated}); err != nil {
		t.Fatal(err)
	}

	got, total, err := repo.Search(ctx, model.ContentQuery{Sort: model.SortPopularity, Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || got[0].Title != "New title" || got[0].Video.Views != 5000 {
		t.Fatalf("total=%d got=%+v", total, got)
	}
}

func TestUpsertIsAtomic(t *testing.T) {
	repo := NewContentRepository(newTestDB(t))
	ctx := context.Background()

	broken := video("v2", "Broken", day)
	broken.Article = &model.ArticleMetrics{} // violates the metrics_match_type CHECK

	if err := repo.Upsert(ctx, []model.Content{video("v1", "Fine", day), broken}); err == nil {
		t.Fatal("expected the CHECK constraint to reject the batch")
	}
	_, total, err := repo.Search(ctx, model.ContentQuery{Sort: model.SortPopularity, Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("total = %d, want 0: a failed batch must not write anything", total)
	}
}

func TestSearch(t *testing.T) {
	repo := NewContentRepository(newTestDB(t))
	ctx := context.Background()

	// Videos have base score 4, articles 298.25, and every date is old enough
	// to add no recency points, so popularity order is articles first, then id.
	err := repo.Upsert(ctx, []model.Content{
		video("v1", "Go Concurrency Patterns", day, "programming"),
		video("v2", "Google Cloud Basics", day, "devops"),
		article("a1", "Clean Code", day, "go"),
		article("a2", "Unit Testing Tips", day, "testing"),
	})
	if err != nil {
		t.Fatal(err)
	}

	pop, rel := model.SortPopularity, model.SortRelevance
	tests := []struct {
		name    string
		q       model.ContentQuery
		wantIDs []string
		total   int
	}{
		{"no keyword, by popularity", model.ContentQuery{Sort: pop}, []string{"a1", "a2", "v1", "v2"}, 4},
		{"relevance without keyword falls back to popularity", model.ContentQuery{Sort: rel}, []string{"a1", "a2", "v1", "v2"}, 4},
		{"keyword without words matches everything", model.ContentQuery{Keyword: "&& !!", Sort: rel}, []string{"a1", "a2", "v1", "v2"}, 4},

		{"keyword by popularity", model.ContentQuery{Keyword: "go", Sort: pop}, []string{"a1", "v1", "v2"}, 3},
		// exact title (v1) > exact tag (a1) > prefix-only title "Google" (v2)
		{"exact matches outrank prefix matches", model.ContentQuery{Keyword: "go", Sort: rel}, []string{"v1", "a1", "v2"}, 3},
		{"partial word", model.ContentQuery{Keyword: "concur", Sort: rel}, []string{"v1"}, 1},
		// "concurrency" is stemmed to "concurr"; these are longer than the stem.
		{"partial word longer than the stem", model.ContentQuery{Keyword: "concurren", Sort: rel}, []string{"v1"}, 1},
		{"partial tag longer than the stem", model.ContentQuery{Keyword: "programmin", Sort: rel}, []string{"v1"}, 1},
		{"partial word inside a longer word", model.ContentQuery{Keyword: "goo", Sort: rel}, []string{"v2"}, 1},
		{"tag match", model.ContentQuery{Keyword: "devops", Sort: rel}, []string{"v2"}, 1},
		{"stemming: pattern matches Patterns", model.ContentQuery{Keyword: "pattern", Sort: rel}, []string{"v1"}, 1},
		{"stemming: tips matches Tips", model.ContentQuery{Keyword: "tips", Sort: rel}, []string{"a2"}, 1},
		{"case-insensitive", model.ContentQuery{Keyword: "CLOUD", Sort: rel}, []string{"v2"}, 1},
		{"all words must match", model.ContentQuery{Keyword: "go concur", Sort: rel}, []string{"v1"}, 1},
		{"no content has every word", model.ContentQuery{Keyword: "clean testing", Sort: rel}, nil, 0},
		{"tsquery operators are ignored", model.ContentQuery{Keyword: "go & | ( ! :*", Sort: rel}, []string{"v1", "a1", "v2"}, 3},

		{"keyword and type", model.ContentQuery{Keyword: "go", Type: model.ContentTypeArticle, Sort: rel}, []string{"a1"}, 1},
		{"type only", model.ContentQuery{Type: model.ContentTypeVideo, Sort: pop}, []string{"v1", "v2"}, 2},

		{"second page", model.ContentQuery{Sort: pop, Page: 2, PerPage: 3}, []string{"v2"}, 4},
		{"page past the end", model.ContentQuery{Sort: pop, Page: 5, PerPage: 3}, nil, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.q.Page == 0 {
				tt.q.Page, tt.q.PerPage = 1, 10
			}
			got, total, err := repo.Search(ctx, tt.q)
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, c := range got {
				ids = append(ids, c.ProviderID)
			}
			if total != tt.total || strings.Join(ids, ",") != strings.Join(tt.wantIDs, ",") {
				t.Errorf("got ids=%v total=%d, want ids=%v total=%d", ids, total, tt.wantIDs, tt.total)
			}
		})
	}
}

func TestSearchScoreIncludesRecency(t *testing.T) {
	repo := NewContentRepository(newTestDB(t))
	ctx := context.Background()

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	daysAgo := func(n int) time.Time { return today.AddDate(0, 0, -n) }

	// Every video has the same metrics, so base score is 4 and only recency differs.
	err := repo.Upsert(ctx, []model.Content{
		video("future", "Future", daysAgo(-3)),
		video("d0", "Today", daysAgo(0)),
		video("d7", "Seven days", daysAgo(7)),
		video("d8", "Eight days", daysAgo(8)),
		video("d30", "Thirty days", daysAgo(30)),
		video("d31", "Thirty-one days", daysAgo(31)),
		video("d90", "Ninety days", daysAgo(90)),
		video("d91", "Ninety-one days", daysAgo(91)),
	})
	if err != nil {
		t.Fatal(err)
	}

	got, _, err := repo.Search(ctx, model.ContentQuery{Sort: model.SortPopularity, Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]float64{
		"future": 9, "d0": 9, "d7": 9, // within a week: +5
		"d8": 7, "d30": 7, // within a month: +3
		"d31": 5, "d90": 5, // within three months: +1
		"d91": 4, // older: +0
	}
	prev := 1e9
	for _, c := range got {
		if c.Score != want[c.ProviderID] {
			t.Errorf("%s: score = %v, want %v", c.ProviderID, c.Score, want[c.ProviderID])
		}
		if c.Score > prev {
			t.Errorf("results not sorted by score: %s (%v) after %v", c.ProviderID, c.Score, prev)
		}
		prev = c.Score
	}
}
