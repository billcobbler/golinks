SERVER_BIN := golinks-server
CLI_BIN    := golinks
SERVER_CMD := ./cmd/server
CLI_CMD    := ./cmd/cli
BUILD_DIR  := ./bin
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS    := -ldflags "-s -w -X main.version=$(VERSION)"
GO         := go

EXTENSION_DIR  := ./extension
EXTENSION_NAME := golinks-extension

# ── Node.js runtime ────────────────────────────────────────────────────────────
# Use a Docker container for all Node.js operations so the host doesn't need
# Node installed.  Override by setting NODE=node if you have Node ≥ 18 locally.
#   make extension-test NODE=node
#
# On Windows/Git Bash, pwd -W returns a Windows-format path (C:/...) that
# Docker Desktop accepts for volume mounts; falls back to Unix path on Linux/macOS.
MOUNT_PWD := $(shell pwd -W 2>/dev/null || pwd)
NODE      ?= docker run --rm \
               -v "$(MOUNT_PWD)/extension://ext" \
               -w //ext \
               node:lts node
NODE_SH   ?= docker run --rm \
               -v "$(MOUNT_PWD)/extension://ext" \
               -w //ext \
               node:lts sh

.PHONY: all build build-cli run test test-verbose lint clean docker-build docker-up docker-down tidy \
        extension-icons extension-test extension-pack-chrome extension-pack-firefox extension-pack help

## Build the server binary
build:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(SERVER_BIN) $(SERVER_CMD)
	@echo "Built $(BUILD_DIR)/$(SERVER_BIN)"

## Build the CLI binary
build-cli:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_BIN) $(CLI_CMD)
	@echo "Built $(BUILD_DIR)/$(CLI_BIN)"

## Build both server and CLI
build-all-local: build build-cli

## Run the server (auto-creates golinks.db in the current directory)
run: build
	$(BUILD_DIR)/$(SERVER_BIN)

## Run all tests
test:
	$(GO) test ./... -timeout 30s

## Run tests with verbose output
test-verbose:
	$(GO) test ./... -v -timeout 30s

## Run tests with race detector
test-race:
	$(GO) test ./... -race -timeout 60s

## Download Pico CSS and htmx into internal/web/static/ for fully embedded builds
download-assets:
	curl -sfLo internal/web/static/pico.min.css "https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css"
	curl -sfLo internal/web/static/htmx.min.js  "https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js"
	@echo "Assets downloaded to internal/web/static/"

## Download and tidy dependencies (run this first!)
tidy:
	$(GO) mod tidy

## Lint with golangci-lint (install: https://golangci-lint.run/usage/install/)
lint:
	golangci-lint run ./...

## Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)

## Build Docker image
docker-build:
	docker build -t golinks:latest .

## Start with docker-compose
docker-up:
	docker compose up -d

## Stop docker-compose services
docker-down:
	docker compose down

## Cross-compile server and CLI for common platforms
build-all:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux   GOARCH=amd64  CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(SERVER_BIN)-linux-amd64        $(SERVER_CMD)
	GOOS=linux   GOARCH=arm64  CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(SERVER_BIN)-linux-arm64        $(SERVER_CMD)
	GOOS=darwin  GOARCH=amd64  CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(SERVER_BIN)-darwin-amd64       $(SERVER_CMD)
	GOOS=darwin  GOARCH=arm64  CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(SERVER_BIN)-darwin-arm64       $(SERVER_CMD)
	GOOS=windows GOARCH=amd64  CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(SERVER_BIN)-windows-amd64.exe  $(SERVER_CMD)
	GOOS=linux   GOARCH=amd64  CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_BIN)-linux-amd64           $(CLI_CMD)
	GOOS=linux   GOARCH=arm64  CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_BIN)-linux-arm64           $(CLI_CMD)
	GOOS=darwin  GOARCH=amd64  CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_BIN)-darwin-amd64          $(CLI_CMD)
	GOOS=darwin  GOARCH=arm64  CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_BIN)-darwin-arm64          $(CLI_CMD)
	GOOS=windows GOARCH=amd64  CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_BIN)-windows-amd64.exe     $(CLI_CMD)
	@echo "Cross-compilation complete. Binaries in $(BUILD_DIR)/"

## ── Extension ─────────────────────────────────────────────────────────────────

## Generate PNG icons (uses Docker node:lts; override with NODE=node)
extension-icons:
	$(NODE_SH) icons/generate-run.sh

## Run extension unit tests (uses Docker node:lts; override with NODE=node)
extension-test:
	$(NODE) --test \
	    tests/service_worker.test.js \
	    tests/popup.test.js

## Package extension for Chrome (zip, excludes tests & generator)
extension-pack-chrome: extension-icons
	@mkdir -p $(BUILD_DIR)
	cd $(EXTENSION_DIR) && zip -qr \
	    ../$(BUILD_DIR)/$(EXTENSION_NAME)-chrome-$(VERSION).zip . \
	    --exclude "tests/*" \
	    --exclude "icons/generate.js"
	@echo "Packaged $(BUILD_DIR)/$(EXTENSION_NAME)-chrome-$(VERSION).zip"

## Package extension for Firefox (same source, .zip renamed for AMO upload)
extension-pack-firefox: extension-icons
	@mkdir -p $(BUILD_DIR)
	cd $(EXTENSION_DIR) && zip -qr \
	    ../$(BUILD_DIR)/$(EXTENSION_NAME)-firefox-$(VERSION).zip . \
	    --exclude "tests/*" \
	    --exclude "icons/generate.js"
	@echo "Packaged $(BUILD_DIR)/$(EXTENSION_NAME)-firefox-$(VERSION).zip"

## Package extension for both Chrome and Firefox
extension-pack: extension-pack-chrome extension-pack-firefox

## Show this help message
help:
	@echo "Usage: make <target>"
	@grep -E '^## ' Makefile | sed 's/## /  /'
