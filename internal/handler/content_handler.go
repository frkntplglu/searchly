package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"github.com/frkntplglu/searchly/internal/model"
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

func (h *ContentHandler) Search(c fiber.Ctx) error {
	q, err := parseContentQuery(c)
	if err != nil {
		return handleError(c, fiber.StatusBadRequest, "invalid_parameter", err.Error())
	}

	page, err := h.svc.Search(c.Context(), q)
	if err != nil {
		slog.ErrorContext(c.Context(), "search contents failed",
			"err", err,
			"request_id", requestid.FromContext(c),
		)
		return handleError(c, fiber.StatusInternalServerError, "internal_error", "internal server error")
	}

	return handleSuccess(c, model.NewContentResponses(page.Contents), model.NewPagination(page))
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
