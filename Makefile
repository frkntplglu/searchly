.PHONY: build run migrate ingest ingest-worker test test-integration tidy lint db-up db-down db-reset db-psql

build:
	go build -o bin/searchly ./cmd/searchly

run:
	go run ./cmd/searchly

# Applies the migrations that have not been applied yet.
migrate:
	go run ./cmd/migrate

# Fetches every provider once and stores the content.
ingest:
	go run ./cmd/ingest -interval=0

# Keeps ingesting every 5 minutes until stopped.
ingest-worker:
	go run ./cmd/ingest -interval=5m

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
	docker compose exec postgres sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"'
