package paging

import (
	"context"
	"fmt"
	"log/slog"
)

const MaxPages = 1000

type Page[T any] struct {
	Items   []T
	PerPage int
	Total   int
}

func Collect[T any](ctx context.Context, fetch func(ctx context.Context, page int) (Page[T], error)) ([]T, error) {
	var items []T
	for page := 1; ; page++ {
		p, err := fetch(ctx, page)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}
		items = append(items, p.Items...)

		switch {
		case len(p.Items) == 0, p.PerPage <= 0, len(p.Items) < p.PerPage:
			return items, nil
		case p.Total > 0 && page*p.PerPage >= p.Total:
			return items, nil
		case page == MaxPages:
			slog.WarnContext(ctx, "stopped at the page limit", "pages", page, "items", len(items))
			return items, nil
		}
	}
}
