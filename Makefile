.PHONY: build run migrate ingest test test-integration tidy lint db-up db-down db-reset db-psql

build:
	go build -o bin/searchly ./cmd/searchly

run:
	go run ./cmd/searchly

# Creates the schema from db.sql. Run once against an empty database.
migrate:
	go run ./cmd/migrate

# Fetches every provider once and stores the content.
ingest:
	go run ./cmd/ingest

test:
	go test ./...

# Requires the database: make db-up
test-integration:
	go test -tags=integration ./...

tidy:
	go mod tidy

lint:
	go vet ./...

db-up:
	docker compose up -d --wait postgres

db-down:
	docker compose down

db-reset:
	docker compose down -v

db-psql:
	docker compose exec postgres psql -U searchly -d searchly
