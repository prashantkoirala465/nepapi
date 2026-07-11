DATABASE_URL ?= postgres://nepapi:nepapi@localhost:5433/nepapi?sslmode=disable

.PHONY: run test test-integration lint db-up db-down backfill build

run:
	DATABASE_URL=$(DATABASE_URL) go run ./cmd/api

build:
	go build -o bin/api ./cmd/api
	go build -o bin/backfill ./cmd/backfill

test:
	go test ./...

test-integration:
	NEPAPI_TEST_DATABASE_URL=$(DATABASE_URL) go test ./... -run Integration -v

lint:
	go vet ./...
	gofmt -l . | tee /dev/stderr | wc -l | grep -q '^ *0$$'

db-up:
	docker compose up -d db

db-down:
	docker compose down

backfill:
	DATABASE_URL=$(DATABASE_URL) go run ./cmd/backfill -from $(FROM) -to $(TO)
