.PHONY: run test test-integration bench test-all vet lint cover ci

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

lint: ## golangci-lint (config in .golangci.yml)
	golangci-lint run ./...

cover: ## Unit test coverage report (opens in browser)
	go test -covermode=atomic -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

ci: ## Run the same checks as GitHub CI (except the Docker build)
	go mod tidy && git diff --exit-code go.mod go.sum
	$(MAKE) vet lint
	go test -race ./...
	go test -race -tags integration ./test/integration/...
