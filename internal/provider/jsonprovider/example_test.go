package jsonprovider_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/searchly/internal/provider/httpclient"
	"github.com/searchly/internal/provider/jsonprovider"
)

func ExampleNew() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"name":"searchly","stars":42}`)
	}))
	defer srv.Close()

	// The caller owns the response model; the client only decodes into it.
	type repo struct {
		Name  string `json:"name"`
		Stars int    `json:"stars"`
	}

	client := jsonprovider.New[repo](srv.URL, httpclient.Config{
		RequestsPerSecond: 5,
		Timeout:           2 * time.Second,
		MaxRetries:        3,
	})

	r, err := client.Fetch(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(r.Name, r.Stars)
	// Output: searchly 42
}
