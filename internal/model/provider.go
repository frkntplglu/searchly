// Package model defines the provider response payloads and the
// provider-independent Content they are converted into.
package model

import "encoding/json"

// Provider1Response is the JSON payload served by provider1.
type Provider1Response struct {
	Contents []Provider1Content `json:"contents"`
}

type Provider1Content struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
	// Metrics has a different shape per type (Provider1VideoMetrics or
	// Provider1ArticleMetrics), so it is kept raw and decoded in ToContent once
	// the type is known. One malformed item therefore cannot fail the whole response.
	Metrics     json.RawMessage `json:"metrics"`
	PublishedAt string          `json:"published_at"`
	Tags        []string        `json:"tags"`
}

type Provider1VideoMetrics struct {
	Views    int64  `json:"views"`
	Likes    int64  `json:"likes"`
	Duration string `json:"duration"`
}

type Provider1ArticleMetrics struct {
	ReadingTime int   `json:"reading_time"`
	Reactions   int64 `json:"reactions"`
	Comments    int64 `json:"comments"`
}

func (r Provider1Response) ToContents() ([]Content, error) {
	return nil, nil
}

// Provider2Response is the XML payload served by provider2.
type Provider2Response struct {
	Items []Provider2Item `xml:"items>item"`
}

type Provider2Item struct {
	ID              string         `xml:"id"`
	Headline        string         `xml:"headline"`
	Type            string         `xml:"type"`
	Stats           Provider2Stats `xml:"stats"`
	PublicationDate string         `xml:"publication_date"`
	Categories      []string       `xml:"categories>category"`
}

type Provider2Stats struct {
	Views       int64  `xml:"views"`
	Likes       int64  `xml:"likes"`
	Duration    string `xml:"duration"`
	ReadingTime int    `xml:"reading_time"`
	Reactions   int64  `xml:"reactions"`
	Comments    int64  `xml:"comments"`
}

func (r Provider2Response) ToContents() ([]Content, error) {
	return nil, nil
}
