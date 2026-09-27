package handler

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/searchly/internal/model"
)

const (
	defaultPerPage = 5
	maxPerPage     = 100
)

type contentService interface {
	Search(ctx context.Context, q model.ContentQuery) (model.ContentPage, error)
}

type ContentHandler struct {
	svc contentService
}

func NewContentHandler(svc contentService) *ContentHandler {
	return &ContentHandler{svc: svc}
}

type searchResponse struct {
	Data       []contentResponse  `json:"data"`
	Pagination paginationResponse `json:"pagination"`
}

type contentResponse struct {
	ID          int64     `json:"id"`
	Provider    string    `json:"provider"`
	ProviderID  string    `json:"provider_id"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	PublishedAt time.Time `json:"published_at"`
	Tags        []string  `json:"tags"`
	Score       float64   `json:"score"`
	Metrics     any       `json:"metrics"`
}

type videoMetricsResponse struct {
	Views       int64 `json:"views"`
	Likes       int64 `json:"likes"`
	DurationSec int   `json:"duration_sec"`
}

type articleMetricsResponse struct {
	ReadingTime int   `json:"reading_time"`
	Reactions   int64 `json:"reactions"`
	Comments    int64 `json:"comments"`
}

type paginationResponse struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// Search handles GET /api/v1/contents?q=&type=&sort=&page=&per_page=.
func (h *ContentHandler) Search(c fiber.Ctx) error {
	q, err := parseContentQuery(c)
	if err != nil {
		return WriteError(c, fiber.StatusBadRequest, "invalid_parameter", err.Error())
	}

	page, err := h.svc.Search(c.Context(), q)
	if err != nil {
		return err
	}

	resp := searchResponse{
		Data: make([]contentResponse, 0, len(page.Contents)),
		Pagination: paginationResponse{
			Page:       page.Page,
			PerPage:    page.PerPage,
			Total:      page.Total,
			TotalPages: page.TotalPages(),
		},
	}
	for _, content := range page.Contents {
		resp.Data = append(resp.Data, toContentResponse(content))
	}
	return c.JSON(resp)
}

func parseContentQuery(c fiber.Ctx) (model.ContentQuery, error) {
	q := model.ContentQuery{
		Keyword: strings.TrimSpace(c.Query("q")),
		Page:    1,
		PerPage: defaultPerPage,
	}

	switch t := model.ContentType(c.Query("type")); t {
	case "", model.ContentTypeVideo, model.ContentTypeArticle:
		q.Type = t
	default:
		return q, fmt.Errorf("type must be %q or %q", model.ContentTypeVideo, model.ContentTypeArticle)
	}

	switch s := model.SortOrder(c.Query("sort")); s {
	case "":
		// Relevance is only meaningful for a keyword search.
		q.Sort = model.SortPopularity
		if q.Keyword != "" {
			q.Sort = model.SortRelevance
		}
	case model.SortPopularity, model.SortRelevance:
		q.Sort = s
	default:
		return q, fmt.Errorf("sort must be %q or %q", model.SortPopularity, model.SortRelevance)
	}

	if v := c.Query("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return q, fmt.Errorf("page must be a positive integer")
		}
		q.Page = n
	}
	if v := c.Query("per_page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxPerPage {
			return q, fmt.Errorf("per_page must be between 1 and %d", maxPerPage)
		}
		q.PerPage = n
	}
	return q, nil
}

func toContentResponse(c model.Content) contentResponse {
	resp := contentResponse{
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
		resp.Tags = []string{}
	}
	switch {
	case c.Video != nil:
		resp.Metrics = videoMetricsResponse{Views: c.Video.Views, Likes: c.Video.Likes, DurationSec: c.Video.DurationSec}
	case c.Article != nil:
		resp.Metrics = articleMetricsResponse{ReadingTime: c.Article.ReadingTime, Reactions: c.Article.Reactions, Comments: c.Article.Comments}
	}
	return resp
}
