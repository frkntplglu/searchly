// Package model defines the provider-independent Content and its search types.
package model

type SortOrder string

const (
	// SortPopularity orders by score, highest first.
	SortPopularity SortOrder = "popularity"
	// SortRelevance orders by how well the title and tags match the keyword,
	// then by score. Without a keyword it falls back to SortPopularity.
	SortRelevance SortOrder = "relevance"
)

// ContentQuery filters, sorts and paginates a content search.
type ContentQuery struct {
	// Keyword is full-text matched against the title and tags. Empty matches everything.
	Keyword string
	// Type restricts results to one content type. Empty means all types.
	Type    ContentType
	Sort    SortOrder
	Page    int
	PerPage int
}

// ContentPage is one page of search results.
type ContentPage struct {
	Contents []Content
	Page     int
	PerPage  int
	Total    int
}

func (p ContentPage) TotalPages() int {
	if p.PerPage <= 0 {
		return 0
	}
	return (p.Total + p.PerPage - 1) / p.PerPage
}
