package model

import "time"

// Response is the envelope of every successful API response.
type Response struct {
	Data       any         `json:"data"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// ErrorResponse is the envelope of every failed API response.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Pagination struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type ContentResponse struct {
	ID          int64     `json:"id"`
	Provider    string    `json:"provider"`
	ProviderID  string    `json:"provider_id"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	PublishedAt time.Time `json:"published_at"`
	Tags        []string  `json:"tags"`
	Score       float64   `json:"score"`
	// Metrics is a VideoMetricsResponse or an ArticleMetricsResponse, depending on Type.
	Metrics any `json:"metrics"`
}

type VideoMetricsResponse struct {
	Views       int64 `json:"views"`
	Likes       int64 `json:"likes"`
	DurationSec int   `json:"duration_sec"`
}

type ArticleMetricsResponse struct {
	ReadingTime int   `json:"reading_time"`
	Reactions   int64 `json:"reactions"`
	Comments    int64 `json:"comments"`
}

func NewPagination(page ContentPage) *Pagination {
	return &Pagination{
		Page:       page.Page,
		PerPage:    page.PerPage,
		Total:      page.Total,
		TotalPages: page.TotalPages(),
	}
}

func NewContentResponses(contents []Content) []ContentResponse {
	out := make([]ContentResponse, 0, len(contents))
	for _, c := range contents {
		out = append(out, NewContentResponse(c))
	}
	return out
}

func NewContentResponse(c Content) ContentResponse {
	resp := ContentResponse{
		ID:          c.ID,
		Provider:    c.Provider,
		ProviderID:  c.ProviderID,
		Type:        string(c.Type),
		Title:       c.Title,
		PublishedAt: c.PublishedAt,
		Tags:        c.Tags,
		Score:       c.Score,
	}
	if resp.Tags == nil {
		resp.Tags = []string{} // an empty list, not null
	}
	switch {
	case c.Video != nil:
		resp.Metrics = VideoMetricsResponse{Views: c.Video.Views, Likes: c.Video.Likes, DurationSec: c.Video.DurationSec}
	case c.Article != nil:
		resp.Metrics = ArticleMetricsResponse{ReadingTime: c.Article.ReadingTime, Reactions: c.Article.Reactions, Comments: c.Article.Comments}
	}
	return resp
}
