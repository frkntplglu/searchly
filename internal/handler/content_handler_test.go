package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/frkntplglu/searchly/internal/model"
)

type fakeContentService struct {
	query model.ContentQuery
	page  model.ContentPage
	err   error
}

func (f *fakeContentService) Search(_ context.Context, q model.ContentQuery) (model.ContentPage, error) {
	f.query = q
	return f.page, f.err
}

func newContentApp(svc contentService) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	app.Get("/api/v1/contents", NewContentHandler(svc).Search)
	return app
}

func TestSearchParsesQuery(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want model.ContentQuery
	}{
		{"defaults", "/api/v1/contents",
			model.ContentQuery{Sort: model.SortPopularity, Page: 1, PerPage: 5}},
		{"keyword defaults to relevance", "/api/v1/contents?q=go",
			model.ContentQuery{Keyword: "go", Sort: model.SortRelevance, Page: 1, PerPage: 5}},
		{"all params", "/api/v1/contents?q=+go+&type=article&sort=popularity&page=3&per_page=50",
			model.ContentQuery{Keyword: "go", Type: model.ContentTypeArticle, Sort: model.SortPopularity, Page: 3, PerPage: 50}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeContentService{}
			resp, err := newContentApp(svc).Test(httptest.NewRequest(http.MethodGet, tt.url, nil))
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			if svc.query != tt.want {
				t.Errorf("service got %+v, want %+v", svc.query, tt.want)
			}
		})
	}
}

func TestSearchRejectsInvalidParams(t *testing.T) {
	urls := []string{
		"/api/v1/contents?type=podcast",
		"/api/v1/contents?sort=newest",
		"/api/v1/contents?page=0",
		"/api/v1/contents?page=abc",
		"/api/v1/contents?per_page=0",
		"/api/v1/contents?per_page=101",
	}
	for _, url := range urls {
		t.Run(url, func(t *testing.T) {
			svc := &fakeContentService{}
			resp, err := newContentApp(svc).Test(httptest.NewRequest(http.MethodGet, url, nil))
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			var body errorBody
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Error.Code != "invalid_parameter" {
				t.Errorf("unexpected body: %+v, %v", body, err)
			}
			if svc.query != (model.ContentQuery{}) {
				t.Error("service must not be called for invalid params")
			}
		})
	}
}

func TestSearchResponse(t *testing.T) {
	published := time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC)
	svc := &fakeContentService{page: model.ContentPage{
		Contents: []model.Content{
			{ID: 1, Provider: "provider1", ProviderID: "v1", Type: model.ContentTypeVideo, Title: "Go",
				PublishedAt: published, Tags: []string{"go"}, Score: 41.3,
				Video: &model.VideoMetrics{Views: 100, Likes: 10, DurationSec: 930}},
			{ID: 2, Provider: "provider2", ProviderID: "a1", Type: model.ContentTypeArticle, Title: "Clean",
				PublishedAt: published,
				Article:     &model.ArticleMetrics{ReadingTime: 8, Reactions: 450, Comments: 25}},
		},
		Page: 1, PerPage: 20, Total: 2,
	}}

	resp, err := newContentApp(svc).Test(httptest.NewRequest(http.MethodGet, "/api/v1/contents", nil))
	if err != nil {
		t.Fatal(err)
	}

	var body struct {
		Data []struct {
			ID      int64          `json:"id"`
			Type    string         `json:"type"`
			Score   float64        `json:"score"`
			Tags    []string       `json:"tags"`
			Metrics map[string]any `json:"metrics"`
		} `json:"data"`
		Pagination paginationResponse `json:"pagination"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	if len(body.Data) != 2 {
		t.Fatalf("got %d items, want 2", len(body.Data))
	}
	if body.Data[0].Score != 41.3 {
		t.Errorf("score = %v, want 41.3", body.Data[0].Score)
	}
	if body.Data[0].Metrics["duration_sec"] != float64(930) || body.Data[0].Metrics["reading_time"] != nil {
		t.Errorf("video metrics = %v", body.Data[0].Metrics)
	}
	if body.Data[1].Metrics["reading_time"] != float64(8) || body.Data[1].Metrics["views"] != nil {
		t.Errorf("article metrics = %v", body.Data[1].Metrics)
	}
	if body.Data[1].Tags == nil {
		t.Error("tags must be an empty array, not null")
	}
	if body.Pagination != (paginationResponse{Page: 1, PerPage: 20, Total: 2, TotalPages: 1}) {
		t.Errorf("pagination = %+v", body.Pagination)
	}
}

func TestSearchServiceErrorIs500(t *testing.T) {
	svc := &fakeContentService{err: errors.New("db down")}
	resp, err := newContentApp(svc).Test(httptest.NewRequest(http.MethodGet, "/api/v1/contents", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}
