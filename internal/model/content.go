package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type ContentType string

const (
	ContentTypeVideo   ContentType = "video"
	ContentTypeArticle ContentType = "article"
)

// Content is a provider-independent content item. Exactly one of Video or
// Article is set, matching Type.
type Content struct {
	Provider    string
	ProviderID  string
	Type        ContentType
	Title       string
	PublishedAt time.Time
	Tags        []string

	Video   *VideoMetrics
	Article *ArticleMetrics

	RawPayload json.RawMessage
}

type VideoMetrics struct {
	Views       int64
	Likes       int64
	DurationSec int
}

type ArticleMetrics struct {
	ReadingTime int
	Reactions   int64
	Comments    int
}

// Validate reports whether the content is complete and its metrics match its type.
func (c Content) Validate() error {
	switch {
	case c.Provider == "":
		return errors.New("missing provider")
	case c.ProviderID == "":
		return errors.New("missing provider id")
	case c.Title == "":
		return errors.New("missing title")
	case c.PublishedAt.IsZero():
		return errors.New("missing published date")
	case len(c.RawPayload) == 0:
		return errors.New("missing raw payload")
	}

	switch c.Type {
	case ContentTypeVideo:
		if c.Video == nil || c.Article != nil {
			return errors.New("video must have only video metrics")
		}
		if c.Video.Views < 0 || c.Video.Likes < 0 || c.Video.DurationSec < 0 {
			return errors.New("negative video metric")
		}
	case ContentTypeArticle:
		if c.Article == nil || c.Video != nil {
			return errors.New("article must have only article metrics")
		}
		if c.Article.ReadingTime < 0 || c.Article.Reactions < 0 || c.Article.Comments < 0 {
			return errors.New("negative article metric")
		}
	default:
		return fmt.Errorf("unknown content type %q", c.Type)
	}
	return nil
}
