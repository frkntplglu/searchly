// Package model defines the provider-independent Content and its search types.
package model

type SortOrder string

const (
	SortPopularity SortOrder = "popularity"
	SortRelevance  SortOrder = "relevance"
)

type ContentQuery struct {
	Keyword string
	Type    ContentType
	Sort    SortOrder
	Page    int
	PerPage int
}

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
