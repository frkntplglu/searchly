package provider1

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/frkntplglu/searchly/internal/model"
)

func TestToContentsSamplePayload(t *testing.T) {
	var resp response
	if err := json.Unmarshal(readTestdata(t), &resp); err != nil {
		t.Fatal(err)
	}

	contents, err := resp.toContents()
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 4 {
		t.Fatalf("got %d contents, want 4", len(contents))
	}

	got := contents[0]

	want := model.Content{
		Provider:    "provider1",
		ProviderID:  "v1",
		Type:        model.ContentTypeVideo,
		Title:       "Go Programming Tutorial",
		PublishedAt: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC), // 10:00Z truncated to the date
		Tags:        []string{"programming", "tutorial"},
		Video:       &model.VideoMetrics{Views: 15000, Likes: 1200, DurationSec: 15*60 + 30},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("contents[0]\n got: %+v\nwant: %+v", got, want)
	}
}

func TestToContentArticle(t *testing.T) {
	item := content{
		ID: "a1", Title: "Clean Code", Type: "article",
		Metrics:     json.RawMessage(`{"reading_time":8,"reactions":450,"comments":25}`),
		PublishedAt: "2024-03-14T15:30:00Z",
	}

	got, err := item.toContent()
	if err != nil {
		t.Fatal(err)
	}
	if got.Video != nil || got.Article == nil || *got.Article != (model.ArticleMetrics{ReadingTime: 8, Reactions: 450, Comments: 25}) {
		t.Errorf("got %+v", got)
	}
}

func TestToContentsSkipsInvalidItems(t *testing.T) {
	body := `{"contents":[
		{"id":"ok","title":"Valid","type":"video","metrics":{"views":10,"likes":1,"duration":"1:00"},"published_at":"2024-03-15T10:00:00Z"},
		{"id":"bad-metrics","title":"A","type":"article","metrics":{"reading_time":5,"reactions":2,"comments":"22:45"},"published_at":"2024-03-15T10:00:00Z"},
		{"id":"bad-type","title":"P","type":"podcast","metrics":{},"published_at":"2024-03-15T10:00:00Z"},
		{"id":"bad-date","title":"D","type":"video","metrics":{"duration":"1:00"},"published_at":"2024-03-15"},
		{"id":"bad-duration","title":"X","type":"video","metrics":{"duration":"90"},"published_at":"2024-03-15T10:00:00Z"},
		{"id":"no-title","title":"","type":"video","metrics":{"duration":"1:00"},"published_at":"2024-03-15T10:00:00Z"}
	]}`
	var resp response
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatal(err)
	}

	contents, err := resp.toContents()
	if len(contents) != 1 || contents[0].ProviderID != "ok" {
		t.Fatalf("got %+v, want only the valid item", contents)
	}
	if err == nil {
		t.Fatal("expected an error describing the skipped items")
	}
	for _, id := range []string{"bad-metrics", "bad-type", "bad-date", "bad-duration", "no-title"} {
		if !strings.Contains(err.Error(), id) {
			t.Errorf("error does not mention %q: %v", id, err)
		}
	}
}

func TestParsePublishedAtKeepsUTCDate(t *testing.T) {
	tests := map[string]time.Time{
		"2024-03-15T10:00:00Z":      time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
		"2024-03-15T23:59:59Z":      time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
		"2024-03-15T01:00:00+03:00": time.Date(2024, 3, 14, 0, 0, 0, 0, time.UTC), // 2024-03-14T22:00Z
		"2024-03-15T22:00:00-05:00": time.Date(2024, 3, 16, 0, 0, 0, 0, time.UTC), // 2024-03-16T03:00Z
	}
	for in, want := range tests {
		got, err := parsePublishedAt(in)
		if err != nil || !got.Equal(want) || got.Location() != time.UTC {
			t.Errorf("parsePublishedAt(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"15:30", 930, false},
		{"0:05", 5, false},
		{"1:02:03", 3723, false},
		{"", 0, true},
		{"90", 0, true},
		{"1:60", 0, true},
		{"a:10", 0, true},
		{"1:2:3:4", 0, true},
	}
	for _, tt := range tests {
		got, err := parseDuration(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("parseDuration(%q) = %d, %v; want %d, err=%v", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}
