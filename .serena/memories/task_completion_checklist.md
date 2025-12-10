# Task Completion Checklist

## Before Committing

### 1. Code Quality
- [ ] `go build ./...` - Code compiles
- [ ] `go vet ./...` - No issues
- [ ] `make lint` - Linting passes (if available)

### 2. Testing
- [ ] `go test ./...` - All unit tests pass
- [ ] `go test -race ./...` - No race conditions
- [ ] `go test -tags=integration ./tests/integration/...` - Integration tests pass

### 3. Documentation
- [ ] Update CHANGELOG.md for notable changes
- [ ] Update CLAUDE.md if architecture changes
- [ ] Add comments only where logic is non-obvious

### 4. Breaking Changes
- [ ] Document in CHANGELOG.md
- [ ] Update affected tests
- [ ] Consider migration path

## File Checklist by Type

### Domain Changes (`internal/domain/`)
- Ensure no infrastructure imports
- Add validation if new fields
- Update tests in same package

### Infrastructure Changes (`internal/infrastructure/`)
- Implement port interfaces correctly
- Add integration tests if I/O involved
- Handle errors with context

### CLI Changes (`internal/interfaces/cli/`)
- Test with `--verbose` and `--quiet` flags
- Verify exit codes match error types
- Update help text if flags change

## Quick Validation
```bash
# One-liner to validate before commit
go build ./... && go test ./... && go vet ./...
```
