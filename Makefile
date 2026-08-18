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

# Regenerates managed-file metadata and fails if the result differs from what
# is on disk. Generated output must never be hand-edited.
#
# This deliberately does not use `git diff`: git is blind to untracked files,
# so a newly added generated file would silently pass. Comparing content
# before and after regeneration is correct regardless of git state.
GEN_FILES = $(shell find . -name '*_gen.go' -not -path './.dev/*' -not -path './bin/*' | sort)

.PHONY: generate-check
generate-check:
	@before=$$(cat $(GEN_FILES) 2>/dev/null | sha256sum); \
	$(GO) generate ./... >/dev/null; \
	after=$$(cat $(GEN_FILES) 2>/dev/null | sha256sum); \
	if [ "$$before" != "$$after" ]; then \
		echo "generated files are stale; run 'make generate' and commit the result"; \
		git --no-pager diff -- ':(glob)**/*_gen.go' || true; \
		exit 1; \
	fi; \
	echo "generated files are current"

# The validation contract the release workflow requires for an exact commit.
.PHONY: check
check: fmt-check vet lint test

# Installs the managed runtime payload into a local config root so the
# development binary is a complete installation. The embedded manifest is
# generated from these same files, so the hashes match.
#
# Set SVIBE_CONFIG_HOME to the same path when running ./bin/svibe.
SVIBE_DEV_HOME ?= $(CURDIR)/.dev/svibe

.PHONY: install-dev
install-dev: build
	@rm -rf '$(SVIBE_DEV_HOME)'
	@mkdir -p '$(SVIBE_DEV_HOME)/config'
	@cp -R core '$(SVIBE_DEV_HOME)/core'
	@cp config/models.yaml '$(SVIBE_DEV_HOME)/config/models.yaml'
	@echo "installed managed payload to $(SVIBE_DEV_HOME)"
	@echo "run: SVIBE_CONFIG_HOME='$(SVIBE_DEV_HOME)' $(BINARY) status"

.PHONY: clean
clean:
	rm -rf $(BIN_DIR) dist .dev
