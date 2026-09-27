// Package provider2 adapts the provider2 XML API to model.Content.
package provider2

import (
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"

	"github.com/frkntplglu/searchly/internal/model"
	"github.com/frkntplglu/searchly/internal/provider/httpclient"
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
	body, err := p.http.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	var resp response
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("%s: decode xml: %w", name, err)
	}
	contents, err := resp.toContents()
	if err != nil {
		slog.WarnContext(ctx, "skipped invalid contents", "provider", name, "err", err)
	}
	return contents, nil
}
