.PHONY: help build install dev test test-unit test-integration test-coverage test-race lint fmt vet clean tidy verify lint-arch lint-arch-map quality fix

.DEFAULT_GOAL := help

# Help target
help:
	@echo "AWF - AI Workflow CLI"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build:"
	@echo "  build            Build binary to ./bin/awf"
	@echo "  install          Build and install to /usr/local/bin"
	@echo "  dev              Run without building"
	@echo "  clean            Remove build artifacts"
	@echo ""
	@echo "Test:"
	@echo "  test             Run fast tests (excludes real CLI calls)"
	@echo "  test-unit        Run unit tests only"
	@echo "  test-integration Run integration tests (real CLI calls - slow)"
	@echo "  test-external    Run external tests (requires CLIs: claude, codex, gemini, opencode)"
	@echo "  test-all         Run all tests including integration"
	@echo "  test-coverage    Generate coverage report"
	@echo "  test-race        Run tests with race detector"
	@echo ""
	@echo "Code Quality:"
	@echo "  fmt              Format code with gofumpt"
	@echo "  vet              Run go vet"
	@echo "  lint             Run golangci-lint"
	@echo "  quality          Run all quality checks (lint+fmt+vet+test)"
	@echo "  fix              Auto-fix linter issues"
	@echo "  lint-arch        Check architecture constraints (go-arch-lint)"
	@echo "  lint-arch-map    Show component-to-package mapping"
	@echo ""
	@echo "Dependencies:"
	@echo "  tidy             Tidy go modules"
	@echo "  verify           Verify go modules"

# Build variables
BINARY_NAME := awf
BINARY_DIR := bin
CMD_DIR := ./cmd/awf
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X github.com/awf-project/awf/internal/interfaces/cli.Version=$(VERSION) \
	-X github.com/awf-project/awf/internal/interfaces/cli.Commit=$(COMMIT) \
	-X github.com/awf-project/awf/internal/interfaces/cli.BuildDate=$(BUILD_DATE)"

# Build binary
build:
	@mkdir -p $(BINARY_DIR)
	go build $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME) $(CMD_DIR)

# Install to /usr/local/bin
install: build
	cp $(BINARY_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)

# Run without building
dev:
	go run $(LDFLAGS) $(CMD_DIR)

# All tests (fast - excludes integration tests with real CLI calls)
test:
	go test -v ./...

# Unit tests only (excludes integration and tests/integration)
test-unit:
	go test -v ./internal/... ./pkg/...

# Integration tests (real CLI calls - slow)
test-integration:
	go test -v -tags=integration ./internal/infrastructure/agents/... ./tests/integration/...

# External tests (requires external CLIs: claude, codex, gemini, opencode)
test-external:
	go test -v -tags=external ./...

# All tests including integration (slow - requires CLI tools installed)
test-all:
	go test -v -tags=integration ./...

# Coverage report
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Race detector
test-race:
	go test -race ./...

# Lint with golangci-lint
lint:
	golangci-lint run

# Format code with gofumpt (stricter than gofmt)
fmt:
	go run mvdan.cc/gofumpt@latest -w .

# Vet code
vet:
	go vet ./...

# Run all quality checks
quality: lint fmt vet lint-arch test
	@echo "All quality checks passed"

# Auto-fix linter issues
fix:
	golangci-lint run --fix

# Clean build artifacts
clean:
	rm -rf $(BINARY_DIR) coverage.out coverage.html

# Tidy dependencies
tidy:
	go mod tidy

# Verify module
verify:
	go mod verify

# Architecture lint with go-arch-lint (replaces check-domain)
lint-arch:
	go-arch-lint check --project-path . --arch-file .go-arch-lint.yml

# Show component-to-package mapping
lint-arch-map:
	go-arch-lint mapping --project-path . --arch-file .go-arch-lint.yml
