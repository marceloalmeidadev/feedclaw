# FeedClaw — build automation.
# UI/packaging targets land in later phases; kept as documented stubs for now.

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION) -X github.com/marceloalmeidadev/feedclaw/internal/fetch.Version=$(VERSION)
BIN     := bin/feedclaw

.PHONY: all build test vet fmt lint tidy clean ui-build package

all: build

build: ## Build the feedclaw binary
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/feedclaw

test: ## Run all Go tests
	go test ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go code
	gofmt -w cmd internal assets.go

lint: ## Run golangci-lint (requires golangci-lint installed)
	golangci-lint run ./...

tidy: ## Tidy go.mod/go.sum
	go mod tidy

clean:
	rm -rf bin dist

# --- Later phases -----------------------------------------------------------

ui-build: ## (Phase 6) Build the Nuxt UI into ui/.output/public
	@echo "ui-build: implemented in Phase 6"

package: ## (Phase 7) Build the OpenClaw bundle plugin
	@echo "package: implemented in Phase 7"
