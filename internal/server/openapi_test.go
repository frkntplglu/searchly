package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"

	"github.com/searchly/api"
	"github.com/searchly/internal/config"
	"github.com/searchly/internal/handler"
	"github.com/searchly/internal/model"
)

type stubContentService struct {
	page model.ContentPage
	err  error
}

func (s stubContentService) Search(_ context.Context, q model.ContentQuery) (model.ContentPage, error) {
	s.page.Page, s.page.PerPage = q.Page, q.PerPage
	return s.page, s.err
}

type stubPinger struct{ err error }

func (p stubPinger) Ping(context.Context) error { return p.err }

var samplePage = model.ContentPage{
	Total: 2,
	Contents: []model.Content{
		{
			ID: 1, Provider: "provider1", ProviderID: "v1", Type: model.ContentTypeVideo,
			Title: "Go Programming Tutorial", Tags: []string{"programming"},
			PublishedAt: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC), Score: 41.3,
			Video: &model.VideoMetrics{Views: 15000, Likes: 1200, DurationSec: 930},
		},
		{
			ID: 2, Provider: "provider2", ProviderID: "a1", Type: model.ContentTypeArticle,
			Title:       "Clean Architecture in Go",
			PublishedAt: time.Date(2024, 3, 14, 0, 0, 0, 0, time.UTC), Score: 298.25,
			Article: &model.ArticleMetrics{ReadingTime: 8, Reactions: 450, Comments: 25},
		},
	},
}

// TestAPIMatchesOpenAPISpec sends real requests to the API and checks that both
// the requests and the responses conform to api/openapi.yaml, so the
// documentation cannot silently drift from the code. It also fails when an
// operation in the spec is not exercised here.
func TestAPIMatchesOpenAPISpec(t *testing.T) {
	ctx := context.Background()
	doc, err := openapi3.NewLoader().LoadFromData(api.Spec)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if err := doc.Validate(ctx); err != nil {
		t.Fatalf("invalid spec: %v", err)
	}
	router, err := legacy.NewRouter(doc)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		url        string
		svc        stubContentService
		db         stubPinger
		wantStatus int
		// validRequest is false for requests that deliberately break the spec,
		// to check the documented error response.
		validRequest bool
	}{
		{"search without params", "/api/v1/contents", stubContentService{page: samplePage}, stubPinger{}, 200, true},
		{"search with every param", "/api/v1/contents?q=go&type=video&sort=relevance&page=2&per_page=10",
			stubContentService{page: samplePage}, stubPinger{}, 200, true},
		{"search with no results", "/api/v1/contents?q=rust", stubContentService{}, stubPinger{}, 200, true},
		{"search with invalid per_page", "/api/v1/contents?per_page=101", stubContentService{}, stubPinger{}, 400, false},
		{"search with invalid type", "/api/v1/contents?type=podcast", stubContentService{}, stubPinger{}, 400, false},
		{"search fails", "/api/v1/contents", stubContentService{err: errors.New("db down")}, stubPinger{}, 500, true},
		{"health", "/healthz", stubContentService{}, stubPinger{}, 200, true},
		{"ready", "/readyz", stubContentService{}, stubPinger{}, 200, true},
		{"not ready", "/readyz", stubContentService{}, stubPinger{err: errors.New("down")}, 503, true},
	}

	covered := map[string]bool{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := New(config.Config{}, tt.db, handler.NewContentHandler(tt.svc))

			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			route, pathParams, err := router.FindRoute(req)
			if err != nil {
				t.Fatalf("%s is not documented: %v", tt.url, err)
			}
			covered[route.Method+" "+route.Path] = true

			reqInput := &openapi3filter.RequestValidationInput{
				Request:    req,
				PathParams: pathParams,
				Route:      route,
				Options:    &openapi3filter.Options{IncludeResponseStatus: true},
			}
			err = openapi3filter.ValidateRequest(ctx, reqInput)
			if tt.validRequest && err != nil {
				t.Fatalf("request does not match the spec: %v", err)
			}
			if !tt.validRequest && err == nil {
				t.Fatal("request was expected to violate the spec")
			}

			resp, err := app.Test(httptest.NewRequest(http.MethodGet, tt.url, nil))
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", resp.StatusCode, tt.wantStatus, body)
			}

			err = openapi3filter.ValidateResponse(ctx, &openapi3filter.ResponseValidationInput{
				RequestValidationInput: reqInput,
				Status:                 resp.StatusCode,
				Header:                 resp.Header,
				Body:                   io.NopCloser(bytes.NewReader(body)),
				Options:                reqInput.Options,
			})
			if err != nil {
				t.Errorf("response does not match the spec: %v\nbody: %s", err, body)
			}
		})
	}

	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			if !covered[method+" "+path] {
				t.Errorf("%s %s is documented but not exercised by this test", method, path)
			}
		}
	}
}
