.PHONY: build build-monitor test verify run monitor tidy

build:
	go build ./cmd/memory-mcp

build-monitor:
	go build ./cmd/memory-monitor

test:
	go test ./...

verify:
	./scripts/verify-real-db-untouched_test.sh
	go test ./scripts -count=1
	go run ./scripts/verify-test-db-paths.go .
	./scripts/verify-real-db-untouched.sh -- go test -race -count=1 ./...
	go build ./cmd/memory-mcp ./cmd/memory-monitor ./cmd/memory-maintenance
	test -z "$$(gofmt -l cmd internal scripts)"
	rg -n 'memory\.db' --glob '*_test.go' .
	! rg -n 'data/memory\.db' --glob '*_test.go' .
	git diff --check

run:
	go run ./cmd/memory-mcp

monitor:
	go run ./cmd/memory-monitor

tidy:
	go mod tidy
