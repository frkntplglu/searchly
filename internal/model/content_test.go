package model

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestContentValidate(t *testing.T) {
	video := Content{
		Provider:    "p1",
		ProviderID:  "v1",
		Type:        ContentTypeVideo,
		Title:       "Go",
		PublishedAt: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
		Video:       &VideoMetrics{Views: 10, Likes: 1, DurationSec: 60},
	}

	tests := []struct {
		name    string
		modify  func(*Content)
		wantErr bool
	}{
		{"valid video", func(*Content) {}, false},
		{"valid article", func(c *Content) {
			c.Type, c.Video, c.Article = ContentTypeArticle, nil, &ArticleMetrics{ReadingTime: 5}
		}, false},
		{"missing provider id", func(c *Content) { c.ProviderID = "" }, true},
		{"missing title", func(c *Content) { c.Title = "" }, true},
		{"missing published date", func(c *Content) { c.PublishedAt = time.Time{} }, true},
		{"video without metrics", func(c *Content) { c.Video = nil }, true},
		{"video with article metrics", func(c *Content) { c.Article = &ArticleMetrics{} }, true},
		{"article with video metrics", func(c *Content) {
			c.Type, c.Article = ContentTypeArticle, &ArticleMetrics{}
		}, true},
		{"negative views", func(c *Content) { c.Video.Views = -1 }, true},
		{"unknown type", func(c *Content) { c.Type = "podcast" }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := video
			m := *video.Video
			c.Video = &m
			tt.modify(&c)
			if err := c.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBaseScore(t *testing.T) {
	tests := []struct {
		name string
		c    Content
		want float64
	}{
		{
			// (15000/1000 + 1200/100) * 1.5 + (1200/15000) * 10 = 40.5 + 0.8
			name: "video",
			c:    Content{Type: ContentTypeVideo, Video: &VideoMetrics{Views: 15000, Likes: 1200}},
			want: 41.3,
		},
		{
			// (8 + 450/50) * 1.0 + (450/8) * 5 = 17 + 281.25
			name: "article",
			c:    Content{Type: ContentTypeArticle, Article: &ArticleMetrics{ReadingTime: 8, Reactions: 450}},
			want: 298.25,
		},
		{
			name: "unknown type",
			c:    Content{Type: "podcast"},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.BaseScore(); math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("BaseScore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBaseScore_ZeroDenominator(t *testing.T) {
	tests := map[string]Content{
		"video without views":          {Type: ContentTypeVideo, Video: &VideoMetrics{Likes: 10}},
		"article without reading time": {Type: ContentTypeArticle, Article: &ArticleMetrics{Reactions: 10}},
	}
	for name, c := range tests {
		t.Run(name, func(t *testing.T) {
			got := c.BaseScore()
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Fatalf("expected finite score, got %v", got)
			}
		})
	}
}

func TestContentValidateReportsFields(t *testing.T) {
	c := Content{Type: ContentTypeVideo, Video: &VideoMetrics{Views: -1}}

	err := c.Validate()
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, field := range []string{"Provider", "ProviderID", "Title", "PublishedAt", "Views"} {
		if !strings.Contains(err.Error(), field) {
			t.Errorf("error does not mention %s: %v", field, err)
		}
	}
}
