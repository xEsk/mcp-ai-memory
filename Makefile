.PHONY: build build-monitor test run monitor tidy

build:
	go build ./cmd/memory-mcp

build-monitor:
	go build ./cmd/memory-monitor

test:
	go test ./...

run:
	go run ./cmd/memory-mcp

monitor:
	go run ./cmd/memory-monitor

tidy:
	go mod tidy
