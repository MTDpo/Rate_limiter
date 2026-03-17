.PHONY: build run test bench docker clean load-test

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./...

bench:
	go test -bench=. -benchmem ./internal/limiter/... ./internal/middleware/...

docker:
	docker compose up --build

docker-build:
	docker compose build

clean:
	rm -rf bin/
	go clean -cache

load-test:
	@echo "Starting load test (server must be running)..."
	go run ./scripts/loadtest.go -url http://localhost:8080/ -n 1000 -c 50
