package xmlprovider

import (
	"context"
	"encoding/xml"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/searchly/internal/provider/httpclient"
)

type feed struct {
	Title string   `xml:"title"`
	Items []string `xml:"items>item"`
	Count int      `xml:"count"`
}

func serve(t *testing.T, status int, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestFetchDecodesIntoT(t *testing.T) {
	url := serve(t, http.StatusOK, `<?xml version="1.0"?><feed><title>news</title><items><item>a</item><item>b</item></items><count>2</count></feed>`)

	got, err := New[feed](url, httpclient.Config{}).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "news" || got.Count != 2 || len(got.Items) != 2 || got.Items[1] != "b" {
		t.Errorf("got %+v", got)
	}
}

func TestFetchErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		check  func(error) bool
	}{
		{"malformed xml", http.StatusOK, `<feed><title>news</feed>`, func(err error) bool {
			var se *xml.SyntaxError
			return errors.As(err, &se)
		}},
		{"invalid number", http.StatusOK, `<feed><count>many</count></feed>`, func(err error) bool {
			return err != nil
		}},
		{"http status", http.StatusBadRequest, ``, func(err error) bool {
			var se *httpclient.StatusError
			return errors.As(err, &se) && se.StatusCode == http.StatusBadRequest
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New[feed](serve(t, tt.status, tt.body), httpclient.Config{}).Fetch(context.Background())
			if err == nil || !tt.check(err) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
