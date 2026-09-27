package model

import (
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

const (
	videoFactor   = 1.5
	articleFactor = 1.0
)

type ContentType string

const (
	ContentTypeVideo   ContentType = "video"
	ContentTypeArticle ContentType = "article"
)

// Content is a provider-independent content item. Exactly one of Video or
// Article is set, matching Type.
type Content struct {
	ID          int64 // assigned by the database; zero before the content is stored
	Provider    string
	ProviderID  string
	Type        ContentType
	Title       string
	PublishedAt time.Time
	Tags        []string

	Video   *VideoMetrics
	Article *ArticleMetrics

	// Score is BaseScore plus the recency points. It is computed by the
	// database when contents are read and is zero before that.
	Score float64
}

type VideoMetrics struct {
	Views       int64
	Likes       int64
	DurationSec int
}

type ArticleMetrics struct {
	ReadingTime int
	Reactions   int64
	Comments    int64
}

// BaseScore is the time-independent part of the score:
// (base points * type factor) + interaction points. The recency points depend
// on the current date, so they are added by the database at query time.
func (c Content) BaseScore() float64 {
	var base, interaction float64

	switch c.Type {
	case ContentTypeVideo:
		views := float64(c.Video.Views)
		likes := float64(c.Video.Likes)
		base = (views/1000 + likes/100) * videoFactor
		interaction = safeRatio(likes, views) * 10

	case ContentTypeArticle:
		readingTime := float64(c.Article.ReadingTime)
		reactions := float64(c.Article.Reactions)
		base = (readingTime + reactions/50) * articleFactor
		interaction = safeRatio(reactions, readingTime) * 5

	default:
		return 0
	}

	return base + interaction
}

func safeRatio(num, den float64) float64 {
	if den <= 0 {
		return 0
	}
	return num / den
}

// Validate reports whether the content is complete and exactly the metrics
// matching its type are set. The metrics are validated by their own Validate.
func (c Content) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.Provider, validation.Required),
		validation.Field(&c.ProviderID, validation.Required),
		validation.Field(&c.Title, validation.Required),
		validation.Field(&c.PublishedAt, validation.Required),
		validation.Field(&c.Type, validation.Required, validation.In(ContentTypeVideo, ContentTypeArticle)),
		validation.Field(&c.Video,
			validation.When(c.Type == ContentTypeVideo, validation.Required).Else(validation.Nil)),
		validation.Field(&c.Article,
			validation.When(c.Type == ContentTypeArticle, validation.Required).Else(validation.Nil)),
	)
}

func (m VideoMetrics) Validate() error {
	return validation.ValidateStruct(&m,
		validation.Field(&m.Views, validation.Min(0)),
		validation.Field(&m.Likes, validation.Min(0)),
		validation.Field(&m.DurationSec, validation.Min(0)),
	)
}

func (m ArticleMetrics) Validate() error {
	return validation.ValidateStruct(&m,
		validation.Field(&m.ReadingTime, validation.Min(0)),
		validation.Field(&m.Reactions, validation.Min(0)),
		validation.Field(&m.Comments, validation.Min(0)),
	)
}
