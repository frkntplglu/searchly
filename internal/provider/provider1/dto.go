package provider1

import "encoding/json"

type response struct {
	Contents   []content  `json:"contents"`
	Pagination pagination `json:"pagination"`
}

type pagination struct {
	Total   int `json:"total"`
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

type content struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Type        string          `json:"type"`
	Metrics     json.RawMessage `json:"metrics"`
	PublishedAt string          `json:"published_at"`
	Tags        []string        `json:"tags"`
}

type videoMetrics struct {
	Views    int64  `json:"views"`
	Likes    int64  `json:"likes"`
	Duration string `json:"duration"`
}

type articleMetrics struct {
	ReadingTime int   `json:"reading_time"`
	Reactions   int64 `json:"reactions"`
	Comments    int64 `json:"comments"`
}
