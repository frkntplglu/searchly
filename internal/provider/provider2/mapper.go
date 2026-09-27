package provider2

import (
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
	contents := make([]model.Content, 0, len(r.Items))
	var errs []error
	for _, it := range r.Items {
		c, err := it.toContent()
		if err != nil {
			errs = append(errs, fmt.Errorf("item %q: %w", it.ID, err))
			continue
		}
		contents = append(contents, c)
	}
	return contents, errors.Join(errs...)
}

func (it item) toContent() (model.Content, error) {
	publishedAt, err := time.Parse(time.DateOnly, it.PublicationDate)
	if err != nil {
		return model.Content{}, fmt.Errorf("invalid publication_date: %w", err)
	}

	c := model.Content{
		Provider:    name,
		ProviderID:  it.ID,
		Type:        model.ContentType(it.Type),
		Title:       it.Headline,
		PublishedAt: publishedAt,
		Tags:        it.Categories,
	}

	switch c.Type {
	case model.ContentTypeVideo:
		duration, err := parseDuration(it.Stats.Duration)
		if err != nil {
			return model.Content{}, err
		}
		c.Video = &model.VideoMetrics{Views: it.Stats.Views, Likes: it.Stats.Likes, DurationSec: duration}
	case model.ContentTypeArticle:
		c.Article = &model.ArticleMetrics{
			ReadingTime: it.Stats.ReadingTime,
			Reactions:   it.Stats.Reactions,
			Comments:    it.Stats.Comments,
		}
	}

	return c, c.Validate()
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
