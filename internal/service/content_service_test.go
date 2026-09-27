package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/searchly/internal/model"
)

type fakeRepo struct {
	upserted []model.Content
	query    model.ContentQuery
	contents []model.Content
	total    int
	err      error
}

func (f *fakeRepo) Upsert(_ context.Context, contents []model.Content) error {
	f.upserted = contents
	return f.err
}

func (f *fakeRepo) Search(_ context.Context, q model.ContentQuery) ([]model.Content, int, error) {
	f.query = q
	return f.contents, f.total, f.err
}

func validVideo(id string) model.Content {
	return model.Content{
		Provider:    "provider1",
		ProviderID:  id,
		Type:        model.ContentTypeVideo,
		Title:       "Go",
		PublishedAt: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
		Video:       &model.VideoMetrics{Views: 10, Likes: 1, DurationSec: 60},
	}
}

func TestUpsertStoresValidContents(t *testing.T) {
	repo := &fakeRepo{}
	contents := []model.Content{validVideo("v1"), validVideo("v2")}

	if err := NewContentService(repo).Upsert(context.Background(), contents); err != nil {
		t.Fatal(err)
	}
	if len(repo.upserted) != 2 {
		t.Fatalf("stored %d contents, want 2", len(repo.upserted))
	}
}

func TestUpsertRejectsInvalidContentWithoutWriting(t *testing.T) {
	repo := &fakeRepo{}
	invalid := validVideo("v2")
	invalid.Title = ""

	err := NewContentService(repo).Upsert(context.Background(), []model.Content{validVideo("v1"), invalid})
	if err == nil {
		t.Fatal("expected a validation error")
	}
	if repo.upserted != nil {
		t.Error("repository must not be called when a content is invalid")
	}
}

func TestSearch(t *testing.T) {
	repo := &fakeRepo{contents: []model.Content{validVideo("v1")}, total: 41}
	q := model.ContentQuery{Keyword: "go", Type: model.ContentTypeVideo, Page: 2, PerPage: 20}

	page, err := NewContentService(repo).Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if repo.query != q {
		t.Errorf("repository got %+v, want %+v", repo.query, q)
	}
	if page.Page != 2 || page.PerPage != 20 || page.Total != 41 || page.TotalPages() != 3 || len(page.Contents) != 1 {
		t.Errorf("unexpected page: %+v (total pages %d)", page, page.TotalPages())
	}
}

func TestSearchPropagatesRepositoryError(t *testing.T) {
	repoErr := errors.New("db down")
	_, err := NewContentService(&fakeRepo{err: repoErr}).Search(context.Background(), model.ContentQuery{Page: 1, PerPage: 20})
	if !errors.Is(err, repoErr) {
		t.Fatalf("err = %v, want %v", err, repoErr)
	}
}
