package repository

import "testing"

func TestTSQueries(t *testing.T) {
	tests := []struct {
		keyword, prefix, exact string
	}{
		{"go", "go:*", "go"},
		{"  Go   Concur ", "go:* & concur:*", "go & concur"},
		{"go & | ! ( ) :* <-> 'x'", "go:* & x:*", "go & x"},
		{"c++ 2024", "c:* & 2024:*", "c & 2024"},
		{"çalışma", "çalışma:*", "çalışma"},
		{"", "", ""},
		{"&& !!", "", ""},
	}
	for _, tt := range tests {
		prefix, exact := tsQueries(tt.keyword)
		if prefix != tt.prefix || exact != tt.exact {
			t.Errorf("tsQueries(%q) = %q, %q; want %q, %q", tt.keyword, prefix, exact, tt.prefix, tt.exact)
		}
	}
}
