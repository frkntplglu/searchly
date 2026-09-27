package provider2

type response struct {
	Items []item `xml:"items>item"`
}

type item struct {
	ID              string   `xml:"id"`
	Headline        string   `xml:"headline"`
	Type            string   `xml:"type"`
	Stats           stats    `xml:"stats"`
	PublicationDate string   `xml:"publication_date"`
	Categories      []string `xml:"categories>category"`
}

type stats struct {
	Views       int64  `xml:"views"`
	Likes       int64  `xml:"likes"`
	Duration    string `xml:"duration"`
	ReadingTime int    `xml:"reading_time"`
	Reactions   int64  `xml:"reactions"`
	Comments    int64  `xml:"comments"`
}
