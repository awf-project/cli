.PHONY: help build install dev test test-unit test-integration test-coverage test-race lint fmt vet clean tidy verify check-domain

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
	@echo "  test             Run all tests"
	@echo "  test-unit        Run unit tests only"
	@echo "  test-integration Run integration tests"
	@echo "  test-coverage    Generate coverage report"
	@echo "  test-race        Run tests with race detector"
	@echo ""
	@echo "Code Quality:"
	@echo "  fmt              Format code"
	@echo "  vet              Run go vet"
	@echo "  lint             Run golangci-lint"
	@echo "  check-domain     Verify domain layer purity"
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
LDFLAGS := -ldflags "-X github.com/vanoix/awf/internal/interfaces/cli.Version=$(VERSION) \
	-X github.com/vanoix/awf/internal/interfaces/cli.Commit=$(COMMIT) \
	-X github.com/vanoix/awf/internal/interfaces/cli.BuildDate=$(BUILD_DATE)"

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

# All tests
test:
	go test -v ./...

# Unit tests only
test-unit:
	go test -v ./internal/... ./pkg/...

# Integration tests
test-integration:
	go test -v -tags=integration ./tests/integration/...

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

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

# Clean build artifacts
clean:
	rm -rf $(BINARY_DIR) coverage.out coverage.html

# Tidy dependencies
tidy:
	go mod tidy

# Verify module
verify:
	go mod verify

# Check domain purity (no external deps)
check-domain:
	@echo "Checking domain layer imports..."
	@go list -f '{{.ImportPath}}: {{.Imports}}' ./internal/domain/... | grep -v "github.com/vanoix/awf" || true
