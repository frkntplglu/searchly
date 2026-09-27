package provider1

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/frkntplglu/searchly/internal/model"
)

// toContents converts every item. Invalid items are skipped; the returned
// error joins the reason for each one and is nil when all items converted.
func (r response) toContents() ([]model.Content, error) {
	contents := make([]model.Content, 0, len(r.Contents))
	var errs []error
	for _, item := range r.Contents {
		c, err := item.toContent()
		if err != nil {
			errs = append(errs, fmt.Errorf("item %q: %w", item.ID, err))
			continue
		}
		contents = append(contents, c)
	}
	return contents, errors.Join(errs...)
}

func (item content) toContent() (model.Content, error) {
	publishedAt, err := parsePublishedAt(item.PublishedAt)
	if err != nil {
		return model.Content{}, err
	}

	c := model.Content{
		Provider:    name,
		ProviderID:  item.ID,
		Type:        model.ContentType(item.Type),
		Title:       item.Title,
		PublishedAt: publishedAt,
		Tags:        item.Tags,
	}

	switch c.Type {
	case model.ContentTypeVideo:
		var m videoMetrics
		if err := json.Unmarshal(item.Metrics, &m); err != nil {
			return model.Content{}, fmt.Errorf("invalid video metrics: %w", err)
		}
		duration, err := parseDuration(m.Duration)
		if err != nil {
			return model.Content{}, err
		}
		c.Video = &model.VideoMetrics{Views: m.Views, Likes: m.Likes, DurationSec: duration}
	case model.ContentTypeArticle:
		var m articleMetrics
		if err := json.Unmarshal(item.Metrics, &m); err != nil {
			return model.Content{}, fmt.Errorf("invalid article metrics: %w", err)
		}
		c.Article = &model.ArticleMetrics{ReadingTime: m.ReadingTime, Reactions: m.Reactions, Comments: m.Comments}
	}

	return c, c.Validate()
}

// parsePublishedAt parses an RFC 3339 timestamp and keeps only its UTC date.
// provider2 only reports dates, so both providers are compared at day
// precision to keep the recency score fair.
func parsePublishedAt(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid published_at: %w", err)
	}
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), nil
}

// parseDuration converts "mm:ss" or "hh:mm:ss" into seconds.
func parseDuration(s string) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	total := 0
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || (i > 0 && n >= 60) {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		total = total*60 + n
	}
	return total, nil
}
