package xmlprovider_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/searchly/internal/provider/httpclient"
	"github.com/searchly/internal/provider/xmlprovider"
)

func ExampleNew() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<feed><items><item>go</item><item>xml</item></items></feed>`)
	}))
	defer srv.Close()

	// The caller owns the response model; the client only decodes into it.
	type feed struct {
		Items []string `xml:"items>item"`
	}

	client := xmlprovider.New[feed](srv.URL, httpclient.Config{
		RequestsPerSecond: 5,
		Timeout:           2 * time.Second,
		MaxRetries:        3,
	})

	f, err := client.Fetch(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(f.Items)
	// Output: [go xml]
}
