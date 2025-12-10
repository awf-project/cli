# Suggested Commands

## Build & Run
```bash
make build              # Build binary to ./bin/awf
make install            # Build and install to /usr/local/bin
make dev                # Run without building: go run ./cmd/awf
```

## Testing
```bash
make test               # All tests
make test-unit          # Unit tests: ./internal/... ./pkg/...
make test-integration   # Integration tests: ./tests/integration/...
make test-coverage      # Generate coverage.html
make test-race          # Race detector: go test -race ./...

# Run specific test
go test ./internal/domain/workflow/... -v -run TestStepValidation

# Integration tests (require build tag)
go test -tags=integration ./tests/integration/...
```

## Code Quality
```bash
make lint               # golangci-lint
make fmt                # go fmt
make vet                # go vet
go mod tidy             # Clean dependencies
```

## CLI Usage
```bash
awf run <workflow> --input=key=value    # Execute workflow
awf resume <workflow-id>                # Resume interrupted workflow
awf validate <workflow>                 # Static validation
awf list                                # List available workflows
awf status <workflow-id>                # Check running workflow
awf history                             # Execution history
```

## Useful Patterns
```bash
# Run all tests with race detection
go test -race ./...

# Run integration tests only
go test -tags=integration ./tests/integration/... -v

# Check for unused code
go vet ./...

# Format all Go files
gofmt -w .
```
