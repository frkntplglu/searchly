package jsonprovider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/searchly/internal/provider/httpclient"
)

type repo struct {
	Name  string   `json:"name"`
	Stars int      `json:"stars"`
	Tags  []string `json:"tags"`
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
	url := serve(t, http.StatusOK, `{"name":"searchly","stars":42,"tags":["go","api"]}`)

	got, err := New[repo](url, httpclient.Config{}).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "searchly" || got.Stars != 42 || len(got.Tags) != 2 {
		t.Errorf("got %+v", got)
	}
}

func TestFetchDecodesSliceT(t *testing.T) {
	url := serve(t, http.StatusOK, `[{"name":"a"},{"name":"b"}]`)

	got, err := New[[]repo](url, httpclient.Config{}).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Name != "b" {
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
		{"malformed json", http.StatusOK, `{"name":`, func(err error) bool {
			var se *json.SyntaxError
			return errors.As(err, &se)
		}},
		{"type mismatch", http.StatusOK, `{"stars":"many"}`, func(err error) bool {
			var te *json.UnmarshalTypeError
			return errors.As(err, &te)
		}},
		{"http status", http.StatusNotFound, ``, func(err error) bool {
			var se *httpclient.StatusError
			return errors.As(err, &se) && se.StatusCode == http.StatusNotFound
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New[repo](serve(t, tt.status, tt.body), httpclient.Config{}).Fetch(context.Background())
			if err == nil || !tt.check(err) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
