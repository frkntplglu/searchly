package service

import (
	"context"
	"fmt"

	"github.com/searchly/internal/model"
)

type contentRepository interface {
	Upsert(ctx context.Context, contents []model.Content) error
	Search(ctx context.Context, q model.ContentQuery) ([]model.Content, int, error)
}

type ContentService struct {
	repo contentRepository
}

func NewContentService(repo contentRepository) *ContentService {
	return &ContentService{repo: repo}
}

// Upsert validates every content and stores them. Nothing is written if any
// content is invalid.
func (s *ContentService) Upsert(ctx context.Context, contents []model.Content) error {
	for _, c := range contents {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("content %s/%s: %w", c.Provider, c.ProviderID, err)
		}
	}
	return s.repo.Upsert(ctx, contents)
}

// Search returns one page of contents matching q. q is expected to be
// validated by the caller (page >= 1, per page > 0).
func (s *ContentService) Search(ctx context.Context, q model.ContentQuery) (model.ContentPage, error) {
	contents, total, err := s.repo.Search(ctx, q)
	if err != nil {
		return model.ContentPage{}, err
	}
	return model.ContentPage{
		Contents: contents,
		Page:     q.Page,
		PerPage:  q.PerPage,
		Total:    total,
	}, nil
}
