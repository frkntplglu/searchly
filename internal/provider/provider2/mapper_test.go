package provider2

import (
	"encoding/xml"
	"reflect"
	"testing"
	"time"

	"github.com/searchly/internal/model"
)

func TestToContentsSamplePayload(t *testing.T) {
	var resp response
	if err := xml.Unmarshal(readTestdata(t), &resp); err != nil {
		t.Fatal(err)
	}

	contents, err := resp.toContents()
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 4 {
		t.Fatalf("got %d contents, want 4", len(contents))
	}
	video, article := contents[0], contents[3]

	wantVideo := model.Content{
		Provider:    "provider2",
		ProviderID:  "v1",
		Type:        model.ContentTypeVideo,
		Title:       "Introduction to Docker",
		PublishedAt: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
		Tags:        []string{"devops", "containers"},
		Video:       &model.VideoMetrics{Views: 22000, Likes: 1800, DurationSec: 25*60 + 15},
	}
	if !reflect.DeepEqual(video, wantVideo) {
		t.Errorf("video\n got: %+v\nwant: %+v", video, wantVideo)
	}

	wantArticle := model.Content{
		Provider:    "provider2",
		ProviderID:  "a1",
		Type:        model.ContentTypeArticle,
		Title:       "Clean Architecture in Go",
		PublishedAt: time.Date(2024, 3, 14, 0, 0, 0, 0, time.UTC),
		Tags:        []string{"programming", "architecture"},
		Article:     &model.ArticleMetrics{ReadingTime: 8, Reactions: 450, Comments: 25},
	}
	if !reflect.DeepEqual(article, wantArticle) {
		t.Errorf("article\n got: %+v\nwant: %+v", article, wantArticle)
	}
}

func TestToContentsSkipsInvalidItems(t *testing.T) {
	body := `<feed><items>
		<item><id>ok</id><headline>Valid</headline><type>article</type><stats><reading_time>5</reading_time></stats><publication_date>2024-03-14</publication_date></item>
		<item><id>no-title</id><headline></headline><type>video</type><stats><duration>1:00</duration></stats><publication_date>2024-03-14</publication_date></item>
		<item><id>bad-date</id><headline>X</headline><type>video</type><stats><duration>1:00</duration></stats><publication_date>14/03/2024</publication_date></item>
		<item><id>bad-duration</id><headline>X</headline><type>video</type><stats><duration>abc</duration></stats><publication_date>2024-03-14</publication_date></item>
		<item><id>bad-type</id><headline>X</headline><type>podcast</type><publication_date>2024-03-14</publication_date></item>
	</items></feed>`
	var resp response
	if err := xml.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatal(err)
	}

	contents, err := resp.toContents()
	if len(contents) != 1 || contents[0].ProviderID != "ok" {
		t.Fatalf("got %+v, want only the valid item", contents)
	}
	if err == nil {
		t.Fatal("expected an error describing the skipped items")
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"25:15", 1515, false},
		{"1:02:03", 3723, false},
		{"", 0, true},
		{"abc", 0, true},
		{"1:60", 0, true},
	}
	for _, tt := range tests {
		got, err := parseDuration(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("parseDuration(%q) = %d, %v; want %d, err=%v", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}
