// Package provider2 adapts the provider2 XML API to model.Content.
package provider2

import (
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"

	"github.com/frkntplglu/searchly/internal/model"
	"github.com/frkntplglu/searchly/internal/provider/httpclient"
	"github.com/frkntplglu/searchly/internal/provider/paging"
)

const name = "provider2"

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

	contents, err := response{Items: items}.toContents()
	if err != nil {
		slog.WarnContext(ctx, "skipped invalid contents", "provider", name, "err", err)
	}
	return contents, nil
}

func (p *Provider) fetchPage(ctx context.Context, page int) (paging.Page[item], error) {
	body, err := p.http.Get(ctx, url.Values{"page": {strconv.Itoa(page)}})
	if err != nil {
		return paging.Page[item]{}, err
	}
	var resp response
	if err := xml.Unmarshal(body, &resp); err != nil {
		return paging.Page[item]{}, fmt.Errorf("decode xml: %w", err)
	}
	return paging.Page[item]{Items: resp.Items, PerPage: resp.Meta.ItemsPerPage, Total: resp.Meta.TotalCount}, nil
}
