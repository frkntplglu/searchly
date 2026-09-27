package model

import (
	"encoding/json"
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
		RawPayload:  json.RawMessage(`{}`),
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
		{"missing raw payload", func(c *Content) { c.RawPayload = nil }, true},
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
