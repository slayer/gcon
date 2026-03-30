.PHONY: build run test lint clean deps tidy install-lint test-golden test-golden-update demo-setup demo-teardown demos

# golangci-lint version (update in .github/workflows/ci.yml when changing)
GOLANGCI_LINT_VERSION := v2.6

# Binary name
BINARY_NAME=gcon
BUILD_DIR=bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOVET=$(GOCMD) vet

# Build flags — embed version from git for local builds
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Default target
all: build

# Download dependencies
deps:
	$(GOMOD) download

# Tidy dependencies
tidy:
	$(GOMOD) tidy

# Build the binary
build: tidy
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/gcon

# Run the application (uses compiled binary for consistency)
run: build
	./bin/gcon

debug-run: build
	GCON_DEBUG=1 ./bin/gcon

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Run golden snapshot tests
test-golden:
	$(GOTEST) -v ./internal/ui/... -run Golden

# Regenerate golden files after intentional rendering changes
test-golden-update:
	$(GOTEST) ./internal/ui/... -run Golden -update

# Install golangci-lint
install-lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Run linter with auto-fix
lint-fix:
	golangci-lint run --fix ./...

# Run go vet
vet:
	$(GOVET) ./...

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Install the binary to GOPATH/bin
install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/

# Development: run with race detector
dev:
	$(GORUN) -race ./cmd/gcon

# Demo recordings: create/destroy GCP resources and record GIFs with VHS
demo-setup:
	@bash -c 'cd demos && source .envrc && ./resources.sh setup'

demo-teardown:
	@bash -c 'cd demos && source .envrc && ./resources.sh teardown'

demos: build
	@bash -c 'cd demos && source .envrc && export PATH="$$PWD/../bin:$$PATH" && for f in *.tape; do vhs "$$f"; done'

# Help
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  run          - Run the application"
	@echo "  test         - Run tests"
	@echo "  install-lint - Install golangci-lint"
	@echo "  lint         - Run linter"
	@echo "  lint-fix     - Run linter with auto-fix"
	@echo "  clean        - Clean build artifacts"
	@echo "  deps         - Download dependencies"
	@echo "  tidy         - Tidy go.mod"
	@echo "  test-golden  - Run golden snapshot tests"
	@echo "  test-golden-update - Regenerate golden files"
	@echo "  dev          - Run with race detector"
	@echo "  demo-setup   - Create demo GCP resources"
	@echo "  demo-teardown- Destroy demo GCP resources"
	@echo "  demos        - Record demo GIFs with VHS"
