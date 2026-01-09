# Contributing to AWF

Thank you for considering contributing to AWF (AI Workflow CLI). This document explains how to contribute.

## Code of Conduct

This project follows the [Contributor Covenant](.github/CODE_OF_CONDUCT.md). By participating, you agree to uphold this code.

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, check existing issues.

**Bug Report Template:**
- **Description**: Clear description of the bug
- **Steps to Reproduce**: Numbered steps
- **Expected Behavior**: What should happen
- **Actual Behavior**: What actually happens
- **Environment**: OS, Go version, AWF version

### Suggesting Features

Open an issue with:
- **Problem**: What problem does this solve?
- **Solution**: Proposed solution
- **Alternatives**: Other solutions considered

### Pull Requests

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests (`make test`)
5. Run linter (`make lint`)
6. Commit with conventional commits (`feat: add amazing feature`)
7. Push to your fork (`git push origin feature/amazing-feature`)
8. Open a Pull Request

## Development Setup

```bash
# Clone your fork
git clone https://github.com/your-username/gustave.git
cd gustave

# Install dependencies
go mod download

# Build
make build

# Run tests
make test

# Run linter
make lint
```

### Prerequisites

- Go 1.23+
- golangci-lint (for linting)
- Make

## Style Guide

### Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Run `make lint` before committing
- Follow existing patterns in codebase
- Variable names should be complete words, concise but specific

### Architecture

This project follows Hexagonal/Clean Architecture:

```
Domain → Application → Infrastructure → Interfaces
```

- Domain layer has no external dependencies
- All other layers depend inward toward domain
- Use ports (interfaces) for external dependencies

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>
```

**Types**: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `perf`

**Examples:**
- `feat(cli): add interactive mode`
- `fix(executor): handle timeout correctly`
- `docs(readme): update installation steps`

**Rules:**
- Max 50 characters for subject
- Imperative mood ("add" not "added")
- No period at the end

### Testing

- Write tests for new features
- Table-driven tests preferred
- Integration tests go in `tests/integration/`
- Run `make test-coverage` to check coverage

```go
func TestFeature(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        // test cases
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

## Project Structure

```
cmd/awf/           # CLI entry point
internal/
├── domain/        # Business logic, no external deps
├── application/   # Services orchestrating domain
├── infrastructure/# Concrete implementations
└── interfaces/    # CLI handlers
pkg/               # Public packages
tests/             # Integration tests
```

## Dependency Updates

AWF uses [Renovate](https://www.renovatebot.com/) for automated dependency updates. When reviewing Renovate pull requests:

1. **Automerge-eligible PRs** (minor/patch with passing CI): These merge automatically
2. **Major version updates**: Review the changelog and test locally before approving
3. **CI failures**: Investigate if the failure is dependency-related or a flaky test
4. **Questions**: See [Dependency Management](docs/development/dependency-management.md) for more details

To temporarily stop updates, comment `@renovatebot stop` on a Renovate PR. To resume, comment `@renovatebot start`.

## Review Process

1. Maintainers review within 1-2 weeks
2. Address feedback in new commits
3. Squash commits before merge (if requested)
4. Celebrate your contribution!

## Questions?

Open an issue with the `question` label.
