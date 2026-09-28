// Package provider1 adapts the provider1 JSON API to model.Content.
package provider1

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"

	"github.com/frkntplglu/searchly/internal/model"
	"github.com/frkntplglu/searchly/internal/provider/httpclient"
	"github.com/frkntplglu/searchly/internal/provider/paging"
)

const name = "provider1"

type Provider struct {
	http *httpclient.Client
}

func New(url string, cfg httpclient.Config) *Provider {
	return &Provider{http: httpclient.New(url, cfg)}
}

func (p *Provider) Name() string { return name }

func (p *Provider) Fetch(ctx context.Context) ([]model.Content, error) {
	items, err := paging.Collect(ctx, p.fetchPage)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	contents, err := response{Contents: items}.toContents()
	if err != nil {
		slog.WarnContext(ctx, "skipped invalid contents", "provider", name, "err", err)
	}
	return contents, nil
}

func (p *Provider) fetchPage(ctx context.Context, page int) (paging.Page[content], error) {
	body, err := p.http.Get(ctx, url.Values{"page": {strconv.Itoa(page)}})
	if err != nil {
		return paging.Page[content]{}, err
	}
	var resp response
	if err := json.Unmarshal(body, &resp); err != nil {
		return paging.Page[content]{}, fmt.Errorf("decode json: %w", err)
	}
	return paging.Page[content]{Items: resp.Contents, PerPage: resp.Pagination.PerPage, Total: resp.Pagination.Total}, nil
}
