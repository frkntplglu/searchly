package provider2

import (
	"context"
	"encoding/xml"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/frkntplglu/searchly/internal/provider/httpclient"
)

func readTestdata(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/provider2.xml")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func serve(t *testing.T, status int, body []byte) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestResponseDecodesSamplePayload(t *testing.T) {
	var resp response
	if err := xml.Unmarshal(readTestdata(t), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 4 {
		t.Fatalf("got %d items, want 4", len(resp.Items))
	}

	video := resp.Items[0]
	if video.ID != "v1" || video.Headline != "Introduction to Docker" || video.Type != "video" ||
		video.PublicationDate != "2024-03-15" || len(video.Categories) != 2 {
		t.Errorf("unexpected video: %+v", video)
	}
	if video.Stats != (stats{Views: 22000, Likes: 1800, Duration: "25:15"}) {
		t.Errorf("video stats = %+v", video.Stats)
	}

	article := resp.Items[3]
	if article.Type != "article" || article.Stats != (stats{ReadingTime: 8, Reactions: 450, Comments: 25}) {
		t.Errorf("unexpected article: %+v", article)
	}
}

func TestFetch(t *testing.T) {
	p := New(serve(t, http.StatusOK, readTestdata(t)), httpclient.Config{})
	if p.Name() != "provider2" {
		t.Errorf("Name() = %q", p.Name())
	}
	contents, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 4 {
		t.Fatalf("got %d contents, want 4", len(contents))
	}
}

func TestFetchErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		check  func(error) bool
	}{
		{"http status", http.StatusBadGateway, "", func(err error) bool {
			var se *httpclient.StatusError
			return errors.As(err, &se) && se.StatusCode == http.StatusBadGateway
		}},
		{"malformed xml", http.StatusOK, `<feed><items>`, func(err error) bool {
			var se *xml.SyntaxError
			return errors.As(err, &se)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(serve(t, tt.status, []byte(tt.body)), httpclient.Config{}).Fetch(context.Background())
			if err == nil || !tt.check(err) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestFetchWalksPages(t *testing.T) {
	var sample response
	if err := xml.Unmarshal(readTestdata(t), &sample); err != nil {
		t.Fatal(err)
	}
	all := sample.Items

	const perPage = 3
	var requested []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		requested = append(requested, page)
		start, end := min((page-1)*perPage, len(all)), min(page*perPage, len(all))
		body, err := xml.Marshal(response{Items: all[start:end], Meta: meta{TotalCount: len(all), CurrentPage: page, ItemsPerPage: perPage}})
		if err != nil {
			t.Error(err)
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	contents, err := New(srv.URL, httpclient.Config{}).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != len(all) {
		t.Errorf("got %d contents, want %d from both pages", len(contents), len(all))
	}
	if !slices.Equal(requested, []int{1, 2}) {
		t.Errorf("requested pages %v, want [1 2]", requested)
	}
}

func TestFetchStopsWhenPagesDoNotMatchTheReportedTotal(t *testing.T) {
	var calls atomic.Int32
	body := readTestdata(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	contents, err := New(srv.URL, httpclient.Config{}).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || len(contents) != 4 {
		t.Fatalf("made %d requests for %d contents, want 1 request for 4", calls.Load(), len(contents))
	}
}
