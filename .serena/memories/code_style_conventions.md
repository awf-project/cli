# Code Style & Conventions

## Go Style
- Use complete, descriptive variable/function names
- Comments only when purpose is not obvious
- No comments that restate function/variable names
- Use ast-grep instead of grep for code search

## Architecture Rules
- **Domain layer has ZERO external dependencies** (only stdlib)
- Ports (interfaces) defined in `internal/domain/ports/`
- Adapters implement ports in `internal/infrastructure/`
- Application layer orchestrates, domain defines rules
- CLI layer handles dependency injection wiring

## Testing Conventions
- Table-driven tests with `tests []struct{...}`
- Use `t.TempDir()` for file-based tests
- Integration tests tagged with `//go:build integration`
- Mock interfaces, not implementations
- Test file naming: `*_test.go` in same package or `_test` package

## Error Handling
- Use `errors.Join()` for aggregating multiple errors (Go 1.20+)
- Wrap errors with context: `fmt.Errorf("context: %w", err)`
- Infrastructure has typed errors (e.g., `ParseError`)
- Map errors to exit codes in CLI layer

## YAML Workflow Syntax
- Template interpolation: `{{var}}` (Go template style, not `${var}`)
- Available variables: `{{inputs.name}}`, `{{states.step.output}}`, `{{env.VAR}}`
- State types: `step`, `parallel`, `terminal`
- Parallel strategies: `all_succeed`, `any_succeed`, `best_effort`

## Git Commits
- Format: `<type>(<scope>): <subject>`
- Types: feat, fix, docs, style, refactor, test, chore, perf
- Max 50 chars subject, imperative mood
- Never commit prematurely, verify tests pass first
