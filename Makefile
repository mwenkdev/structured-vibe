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

PLUGIN_DIR := integrations/opencode
PLUGIN_ARTIFACT := $(PLUGIN_DIR)/dist/svibe.js

# Builds the OpenCode integration from source. The built artifact is part of
# the managed runtime payload, so it must exist before the managed manifest is
# generated. Uses npm ci against the committed lockfile for reproducibility:
# a differing compiler version would change the artifact and its hash.
.PHONY: plugin
plugin:
	@cd $(PLUGIN_DIR) && npm ci --no-audit --no-fund >/dev/null && npm run build >/dev/null
	@echo "built $(PLUGIN_ARTIFACT)"

.PHONY: plugin-typecheck
plugin-typecheck:
	@cd $(PLUGIN_DIR) && npm ci --no-audit --no-fund >/dev/null && npm run typecheck

.PHONY: generate
generate: plugin
	$(GO) generate ./...

# Regenerates managed-file metadata and fails if the result differs from what
# is on disk. Generated output must never be hand-edited.
#
# This deliberately does not use `git diff`: git is blind to untracked files,
# so a newly added generated file would silently pass. Comparing content
# before and after regeneration is correct regardless of git state.
GEN_FILES = $(shell find . -name '*_gen.go' -not -path './.dev/*' -not -path './bin/*' | sort)

.PHONY: generate-check
generate-check: plugin
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
install-dev: build plugin
	@rm -rf '$(SVIBE_DEV_HOME)'
	@mkdir -p '$(SVIBE_DEV_HOME)/config' '$(SVIBE_DEV_HOME)/integrations/opencode'
	@cp -R core '$(SVIBE_DEV_HOME)/core'
	@cp config/models.yaml '$(SVIBE_DEV_HOME)/config/models.yaml'
	@cp $(PLUGIN_ARTIFACT) '$(SVIBE_DEV_HOME)/integrations/opencode/svibe.js'
	@echo "installed managed payload to $(SVIBE_DEV_HOME)"
	@echo "run: SVIBE_CONFIG_HOME='$(SVIBE_DEV_HOME)' $(BINARY) status"

.PHONY: clean
clean:
	rm -rf $(BIN_DIR) dist .dev $(PLUGIN_DIR)/dist
