SHELL := /bin/bash

BIN_DIR := bin
BINARY  := $(BIN_DIR)/svibe
PKG     := github.com/mwenkdev/structured-vibe

# Version is injected at build time. Release builds override VERSION.
VERSION ?= 0.0.0-dev
LDFLAGS := -s -w -X $(PKG)/internal/buildinfo.Version=$(VERSION)

GO ?= go

.PHONY: all
all: check build

.PHONY: build
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd/svibe

.PHONY: test
test:
	$(GO) test ./...

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: fmt
fmt:
	$(GO) fmt ./...

.PHONY: fmt-check
fmt-check:
	@out="$$(gofmt -l .)"; \
	if [[ -n "$$out" ]]; then \
		echo "gofmt needed for:"; echo "$$out"; exit 1; \
	fi

.PHONY: lint
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed; skipping"; \
	fi

.PHONY: generate
generate:
	$(GO) generate ./...

# Regenerates managed-file metadata and fails if it drifts from what is committed.
# Generated output must never be hand-edited.
.PHONY: generate-check
generate-check: generate
	@if ! git diff --quiet -- ':(glob)**/*_gen.go'; then \
		echo "generated files are stale; run 'make generate' and commit the result"; \
		git --no-pager diff -- ':(glob)**/*_gen.go'; \
		exit 1; \
	fi

# The validation contract the release workflow requires for an exact commit.
.PHONY: check
check: fmt-check vet lint test

.PHONY: clean
clean:
	rm -rf $(BIN_DIR) dist
