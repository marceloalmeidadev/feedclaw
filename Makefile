# FeedClaw — build automation.

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION) -X github.com/marceloalmeidadev/feedclaw/internal/fetch.Version=$(VERSION)
BIN     := bin/feedclaw
UI_DIST := internal/api/dist
DIST    := dist
BUNDLE  := $(DIST)/feedclaw

.PHONY: all build release test vet fmt lint tidy clean ui-build embed-ui package checksums

all: build

build: ## Build the feedclaw binary (no embedded UI)
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
	rm -rf bin dist $(UI_DIST)

# --- UI + release -----------------------------------------------------------

ui-build: ## Build the Nuxt UI (static SPA) into ui/.output/public
	cd ui && npm ci && npm run generate

embed-ui: ui-build ## Copy the built UI into the Go embed directory
	rm -rf $(UI_DIST)
	mkdir -p $(UI_DIST)
	cp -r ui/.output/public/. $(UI_DIST)/

release: embed-ui ## Build the binary with the UI embedded (serves same-origin)
	@mkdir -p bin
	go build -tags embedui -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/feedclaw
	@echo "built $(BIN) $(VERSION) with embedded UI"

# --- OpenClaw bundle plugin -------------------------------------------------

package: release ## Assemble the installable OpenClaw bundle (binary + skills)
	rm -rf $(BUNDLE)
	mkdir -p $(BUNDLE)/scripts
	cp $(BIN) $(BUNDLE)/feedclaw
	cp -r skill/feedclaw skill/feedclaw-digest $(BUNDLE)/
	cp skill/scripts/feedclaw.sh $(BUNDLE)/scripts/feedclaw.sh
	chmod +x $(BUNDLE)/feedclaw $(BUNDLE)/scripts/feedclaw.sh
	cp docs/INSTALL.md $(BUNDLE)/INSTALL.md
	tar -C $(DIST) -czf $(DIST)/feedclaw-$(VERSION).tar.gz feedclaw
	$(MAKE) checksums
	@echo "bundle: $(DIST)/feedclaw-$(VERSION).tar.gz"

checksums: ## Generate SHA256SUMS for the release artifacts
	cd $(DIST) && sha256sum feedclaw-*.tar.gz feedclaw/feedclaw > SHA256SUMS
	@echo "checksums written to $(DIST)/SHA256SUMS"
