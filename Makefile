.PHONY: build test lint run docker-up docker-down fmt vet check

build:
	go build -ldflags="-s -w" -o bin/server ./cmd/server

run:
	go run ./cmd/server

test:
	go test -v -race -count=1 ./...

test-cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down -v

check: fmt vet lint test
