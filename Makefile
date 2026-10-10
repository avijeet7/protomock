.PHONY: run test test-integration bench test-all vet

run: ## Run the mock server (HTTP :8085, gRPC :8086)
	go run ./cmd/server

test: ## Unit tests only (fast, no network)
	go test ./...

test-integration: ## Integration tests: real loader + servers on random local ports
	go test -tags integration ./test/integration/...

bench: ## Benchmarks
	go test -tags bench -run '^$$' -bench . -benchmem ./test/benchmark/...

test-all: vet test test-integration

vet: ## Vet all code, including tagged test packages
	go vet -tags integration,bench ./...
