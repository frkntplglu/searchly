package paging

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func pages(ps ...Page[int]) (fetch func(context.Context, int) (Page[int], error), calls *int) {
	calls = new(int)
	return func(_ context.Context, page int) (Page[int], error) {
		*calls++
		if page > len(ps) {
			return Page[int]{PerPage: 2}, nil
		}
		return ps[page-1], nil
	}, calls
}

func TestCollect(t *testing.T) {
	tests := []struct {
		name      string
		pages     []Page[int]
		wantItems []int
		wantCalls int
	}{
		{
			"stops at a short page",
			[]Page[int]{{Items: []int{1, 2}, PerPage: 2}, {Items: []int{3, 4}, PerPage: 2}, {Items: []int{5}, PerPage: 2}},
			[]int{1, 2, 3, 4, 5}, 3,
		},
		{
			"stops when the total is reached",
			[]Page[int]{{Items: []int{1, 2}, PerPage: 2, Total: 4}, {Items: []int{3, 4}, PerPage: 2, Total: 4}},
			[]int{1, 2, 3, 4}, 2,
		},
		{
			"stops at an empty page",
			[]Page[int]{{Items: []int{1, 2}, PerPage: 2}},
			[]int{1, 2}, 2,
		},
		{
			"takes a response without a page size as the only page",
			[]Page[int]{{Items: []int{1, 2, 3}}},
			[]int{1, 2, 3}, 1,
		},
		{
			"does not follow a total the items contradict",
			[]Page[int]{{Items: []int{1, 2, 3, 4}, PerPage: 10, Total: 150}},
			[]int{1, 2, 3, 4}, 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetch, calls := pages(tt.pages...)
			items, err := Collect(context.Background(), fetch)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(items, tt.wantItems) {
				t.Errorf("items = %v, want %v", items, tt.wantItems)
			}
			if *calls != tt.wantCalls {
				t.Errorf("fetched %d pages, want %d", *calls, tt.wantCalls)
			}
		})
	}
}

func TestCollectStopsAtMaxPages(t *testing.T) {
	calls := 0
	items, err := Collect(context.Background(), func(context.Context, int) (Page[int], error) {
		calls++
		return Page[int]{Items: []int{calls}, PerPage: 1}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != MaxPages || len(items) != MaxPages {
		t.Fatalf("fetched %d pages and %d items, want %d of each", calls, len(items), MaxPages)
	}
}

func TestCollectFailsOnAFailedPage(t *testing.T) {
	pageErr := errors.New("status 503")
	items, err := Collect(context.Background(), func(_ context.Context, page int) (Page[int], error) {
		if page == 2 {
			return Page[int]{}, pageErr
		}
		return Page[int]{Items: []int{page}, PerPage: 1}, nil
	})
	if !errors.Is(err, pageErr) || err.Error() != "page 2: status 503" {
		t.Fatalf("err = %v, want it to wrap %v and name page 2", err, pageErr)
	}
	if items != nil {
		t.Errorf("items = %v, want none: a partial walk must not be stored", items)
	}
}
