// Package provider1 adapts the provider1 JSON API to model.Content.
package provider1

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/frkntplglu/searchly/internal/model"
	"github.com/frkntplglu/searchly/internal/provider/httpclient"
)

const name = "provider1"

type Provider struct {
	http *httpclient.Client
}

func New(url string, cfg httpclient.Config) *Provider {
	return &Provider{http: httpclient.New(url, cfg)}
}

func (p *Provider) Name() string { return name }

// Fetch requests the provider and returns its content. Items that cannot be
// converted are logged and skipped; an error means the whole fetch failed.
func (p *Provider) Fetch(ctx context.Context) ([]model.Content, error) {
	body, err := p.http.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	var resp response
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("%s: decode json: %w", name, err)
	}
	contents, err := resp.toContents()
	if err != nil {
		slog.WarnContext(ctx, "skipped invalid contents", "provider", name, "err", err)
	}
	return contents, nil
}
