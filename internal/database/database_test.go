package database

import "testing"

func TestToMigrateURL(t *testing.T) {
	tests := map[string]string{
		"postgres://u:p@localhost:5433/db?sslmode=disable":   "pgx5://u:p@localhost:5433/db?sslmode=disable",
		"postgresql://u:p@localhost:5433/db?sslmode=disable": "pgx5://u:p@localhost:5433/db?sslmode=disable",
		"pgx5://u:p@localhost/db":                            "pgx5://u:p@localhost/db",
	}
	for in, want := range tests {
		if got := toMigrateURL(in); got != want {
			t.Errorf("toMigrateURL(%q) = %q, want %q", in, got, want)
		}
	}
}
