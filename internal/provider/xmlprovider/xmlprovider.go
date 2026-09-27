// Package xmlprovider fetches an XML endpoint and decodes the whole response into T.
package xmlprovider

import (
	"context"
	"encoding/xml"
	"fmt"

	"github.com/searchly/internal/provider/httpclient"
)

type Client[T any] struct {
	http *httpclient.Client
}

func New[T any](url string, cfg httpclient.Config) *Client[T] {
	return &Client[T]{http: httpclient.New(url, cfg)}
}

// Fetch requests the endpoint and decodes the response body into T.
func (c *Client[T]) Fetch(ctx context.Context) (T, error) {
	var v T
	body, err := c.http.Get(ctx)
	if err != nil {
		return v, err
	}
	if err := xml.Unmarshal(body, &v); err != nil {
		return v, fmt.Errorf("decode xml: %w", err)
	}
	return v, nil
}
