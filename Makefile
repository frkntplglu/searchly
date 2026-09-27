.PHONY: build run test tidy lint db-up db-down db-reset db-psql

build:
	go build -o bin/searchly ./cmd/searchly

run:
	go run ./cmd/searchly

test:
	go test ./...

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
