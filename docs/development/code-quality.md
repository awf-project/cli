# Code Quality Tools

This document explains AWF's code quality tooling strategy, covering all 17 linters, the formatter, and how to use them effectively.

## Overview

AWF uses **golangci-lint v2** as a single aggregator for 17 specialized linters that enforce Go best practices, catch bugs early, and maintain consistent code style. The configuration follows late-2025 standards with strict cognitive complexity limits and modern error handling patterns.

**Quality Philosophy**:
- **Correct** - Tests prove functionality works
- **Secure** - No injection vulnerabilities, validated inputs
- **Clear** - Readable by unfamiliar developers in <5 minutes
- **Minimal** - No unnecessary abstractions or complexity

**Linter Categories**:
- **Core quality** (5 linters): Bug detection and correctness
- **Readability** (2 linters): Clear, maintainable code
- **Error handling** (1 linter): Proper error wrapping
- **Security** (1 linter): Vulnerability detection
- **Architecture** (1 linter): Dependency constraints
- **Modern Go** (7 linters): Late-2025 best practices

## How It Works

### golangci-lint as Aggregator

All linters run through a single command via golangci-lint v2:

```bash
make lint          # Run all 17 linters
make fix           # Auto-fix issues where possible
golangci-lint run  # Direct invocation
```

**Why golangci-lint?**
- Single binary with 100+ bundled linters
- Parallel execution for fast performance
- Unified configuration in `.golangci.yml`
- CI integration via golangci-lint-action@v9

### CI Integration

GitHub Actions workflow (`.github/workflows/ci.yaml`) runs quality checks on every push:

```yaml
- name: Lint
  uses: golangci/golangci-lint-action@v9
  with:
    version: latest
    args: --timeout=5m
```

**Quality Gates**:
1. `make lint` must pass (all 17 linters)
2. `make fmt` must leave no changes (gofumpt formatting)
3. `make test` must pass (all tests)

CI fails if any gate fails, blocking merge.

### Makefile Targets

| Target | Description | Use Case |
|--------|-------------|----------|
| `make lint` | Run golangci-lint with all 17 linters | Before committing |
| `make fmt` | Format code with gofumpt | Before committing |
| `make vet` | Run go vet static analysis | Before committing |
| `make quality` | Run lint + fmt + vet + test | Final check before PR |
| `make fix` | Auto-fix linter issues | After running `make lint` |

**Typical Workflow**:
```bash
# 1. Make changes
vim internal/domain/workflow/workflow.go

# 2. Format code
make fmt

# 3. Check for issues
make lint

# 4. Fix auto-fixable issues
make fix

# 5. Run all quality checks
make quality

# 6. Commit if all checks pass
git add .
git commit -m "feat(workflow): add validation"
```

## Configuration Reference

### Core Quality Linters

#### errcheck - Unhandled Errors
**Purpose**: Detects unchecked errors that could cause silent failures.

**Example violation**:
```go
file, _ := os.Open("config.yaml")  // ❌ Ignores error
defer file.Close()
```

**Fix**:
```go
file, err := os.Open("config.yaml")  // ✅ Check error
if err != nil {
    return fmt.Errorf("open config: %w", err)
}
defer file.Close()
```

**Configuration**: Excludes common patterns:
- `(io.Closer).Close` - Deferred closes
- `(net/http.ResponseWriter).Write` - HTTP response writes
- `(*zap.Logger).Sync` - Logger flushes

#### govet - Official Go Analyzer
**Purpose**: Official Go team static analyzer for suspicious constructs.

**Example violation**:
```go
fmt.Printf("User %s has %d", user.Name)  // ❌ Missing argument
```

**Fix**:
```go
fmt.Printf("User %s has %d points", user.Name, user.Points)  // ✅ Correct
```

**Configuration**: Enables all checks except `fieldalignment` (micro-optimization).

#### staticcheck - Comprehensive Analysis
**Purpose**: Industry-standard static analyzer (includes gosimple, stylecheck).

**Example violation**:
```go
if err := validate(); err != nil {
    return err  // ❌ Could return nil interface
}
return nil
```

**Fix**:
```go
return validate()  // ✅ Direct return preserves nil correctness
```

#### ineffassign - Wasted Assignments
**Purpose**: Detects assignments that are never read.

**Example violation**:
```go
result := compute()  // ❌ Never used
result = compute2()
return result
```

**Fix**:
```go
result := compute2()  // ✅ Remove unused assignment
return result
```

#### unused - Dead Code
**Purpose**: Detects unused constants, variables, functions, types.

**Example violation**:
```go
const maxRetries = 5  // ❌ Never referenced
```

**Fix**: Delete the unused constant.

### Readability Linters

#### misspell - Typo Detection
**Purpose**: Catches typos in comments and strings using US English dictionary.

**Example violation**:
```go
// Proces the workflow  // ❌ Typo: "Proces"
```

**Fix**:
```go
// Process the workflow  // ✅ Correct spelling
```

**Configuration**: Allows British spellings "cancelled" and "cancelling" (used in API).

#### revive - Modern Linter
**Purpose**: Replacement for deprecated golint with 47 configurable rules.

**Example violation**:
```go
func (w *Workflow) GetID() string {}  // ❌ Stutters: Workflow.GetID
```

**Fix**:
```go
func (w *Workflow) ID() string {}  // ✅ Concise
```

### Error Handling Linters

#### errorlint - Error Wrapping
**Purpose**: Enforces proper `%w` wrapping for error chains and `errors.Is`/`errors.As` usage.

**Example violation**:
```go
if err != nil {
    return fmt.Errorf("validation failed: %v", err)  // ❌ Use %w
}
```

**Fix**:
```go
if err != nil {
    return fmt.Errorf("validation failed: %w", err)  // ✅ Wraps error
}
```

### Security Linters

#### gosec - Security Audit
**Purpose**: Scans for common security vulnerabilities (SQL injection, path traversal, etc.).

**Example violation**:
```go
cmd := exec.Command("sh", "-c", userInput)  // ❌ Command injection risk
```

**Fix**:
```go
// Use ShellEscape() from pkg/interpolation
safeInput := interpolation.ShellEscape(userInput)
cmd := exec.Command("sh", "-c", safeInput)  // ✅ Escaped
```

**Configuration**: Excludes intentional patterns:
- `G204` - Shell executor intentionally uses variable commands
- `G304` - Workflow loader intentionally reads user-specified files

### Architecture Linters

#### depguard - Dependency Constraints
**Purpose**: Enforces hexagonal architecture by preventing invalid imports in domain layer.

**Example violation**:
```go
// internal/domain/workflow/workflow.go
import "github.com/spf13/cobra"  // ❌ Domain depends on CLI framework
```

**Fix**: Move CLI dependencies to `internal/interfaces/cli/`. Domain uses `ports.Logger` interface.

**Blocked imports in domain**:
- `github.com/spf13/cobra` - CLI framework
- `go.uber.org/zap` - Concrete logger
- `github.com/fatih/color` - UI components
- `github.com/schollz/progressbar/v3` - UI components

### Modern Go Quality Linters (Late-2025)

#### gofumpt - Stricter Formatting
**Purpose**: Stricter formatter than `gofmt` with deterministic rules (extra blank lines, import grouping).

**Example violation**:
```go
import (
    "fmt"
    "github.com/vanoix/awf/internal/domain"
    "os"
)
```

**Fix** (gofumpt adds grouping):
```go
import (
    "fmt"
    "os"

    "github.com/vanoix/awf/internal/domain"
)
```

**Configuration**: Runs automatically via `make fmt`. No manual configuration needed.

#### gocognit - Cognitive Complexity
**Purpose**: Measures how difficult a function is to understand (nesting, conditionals, loops).

**Threshold**: **15** (strict - default is 30)

**Example violation**:
```go
func ProcessWorkflow(w *Workflow) error {
    for _, step := range w.Steps {
        if step.Type == "parallel" {
            for _, branch := range step.Branches {
                if branch.Condition != "" {
                    if EvalCondition(branch.Condition) {
                        // ... nested logic continues
                    }
                }
            }
        } else if step.Type == "loop" {
            // ... more nesting
        }
    }
    return nil
}
// Cognitive complexity: 18 (exceeds 15)
```

**Fix**: Extract helper functions:
```go
func ProcessWorkflow(w *Workflow) error {
    for _, step := range w.Steps {
        if err := processStep(step); err != nil {
            return err
        }
    }
    return nil
}

func processStep(step Step) error {
    switch step.Type {
    case "parallel":
        return processParallelStep(step)
    case "loop":
        return processLoopStep(step)
    default:
        return processSimpleStep(step)
    }
}
```

#### gocritic - Advanced Static Analysis
**Purpose**: 100+ checks for bugs, style issues, and performance problems.

**Enabled check categories**:
- **diagnostic** - Bug detection (nil dereference, type assertions)
- **style** - Idiomatic Go (range loop optimizations, assignment ops)
- **performance** - Efficiency improvements (unnecessary allocations)

**Example violation**:
```go
if _, err := validate(); err != nil {  // ❌ Nested if
    return err
}
```

**Fix**:
```go
if _, err := validate(); err != nil {
    return err  // ✅ gocritic prefers this style
}
```

#### exhaustive - Enum Switch Exhaustiveness
**Purpose**: Ensures switch statements on enums handle all cases.

**Example violation**:
```go
type Status int
const (
    StatusPending Status = iota
    StatusRunning
    StatusCompleted
    StatusFailed
)

func Handle(status Status) {
    switch status {
    case StatusPending:
        // ...
    case StatusRunning:
        // ...
    // ❌ Missing StatusCompleted, StatusFailed
    }
}
```

**Fix**:
```go
func Handle(status Status) {
    switch status {
    case StatusPending:
        // ...
    case StatusRunning:
        // ...
    case StatusCompleted:
        // ...
    case StatusFailed:
        // ...
    // ✅ All cases handled
    }
}
```

**Configuration**: `default-signifies-exhaustive: true` - Allows `default:` to satisfy exhaustiveness.

#### noctx - HTTP Context Requirements
**Purpose**: Ensures HTTP requests include context for cancellation/timeouts.

**Example violation**:
```go
req, _ := http.NewRequest("GET", url, nil)  // ❌ No context
resp, err := client.Do(req)
```

**Fix**:
```go
req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)  // ✅ Has context
resp, err := client.Do(req)
```

#### prealloc - Slice Capacity Optimization
**Purpose**: Detects slices that could be preallocated with known capacity.

**Example violation**:
```go
var results []Result  // ❌ Will grow multiple times
for _, item := range items {
    results = append(results, Process(item))
}
```

**Fix**:
```go
results := make([]Result, 0, len(items))  // ✅ Preallocated capacity
for _, item := range items {
    results = append(results, Process(item))
}
```

#### wrapcheck - Error Wrapping at Boundaries
**Purpose**: Enforces error wrapping when crossing package boundaries.

**Example violation**:
```go
// internal/interfaces/cli/run.go
result, err := workflowService.Execute(ctx, workflow)
if err != nil {
    return err  // ❌ Error crosses boundary unwrapped
}
```

**Fix**:
```go
result, err := workflowService.Execute(ctx, workflow)
if err != nil {
    return fmt.Errorf("execute workflow: %w", err)  // ✅ Wrapped with context
}
```

**Configuration**: `ignorePackageGlobs: ["**/internal/**"]` - Allows unwrapped errors within internal packages.

## Common Issues

### "Cognitive complexity 18 of func exceeds max of 15"

**Problem**: Function has too much nesting/branching.

**Solution**:
1. Extract nested blocks into helper functions
2. Replace nested `if`/`else` with early returns
3. Use switch statements instead of chained `if`/`else if`

**Example**:
```go
// Before (complexity 18)
func Validate(w *Workflow) error {
    if w.Name == "" {
        return errors.New("name required")
    } else {
        if len(w.States) == 0 {
            return errors.New("states required")
        } else {
            for _, state := range w.States {
                if state.Type == "step" {
                    if state.Command == "" {
                        return errors.New("command required")
                    }
                }
            }
        }
    }
    return nil
}

// After (complexity 8)
func Validate(w *Workflow) error {
    if w.Name == "" {
        return errors.New("name required")
    }
    if len(w.States) == 0 {
        return errors.New("states required")
    }
    return validateStates(w.States)
}

func validateStates(states []State) error {
    for _, state := range states {
        if err := validateState(state); err != nil {
            return err
        }
    }
    return nil
}
```

### "Missing cases in switch of type"

**Problem**: Switch on enum doesn't handle all cases.

**Solution**:
1. Add missing cases explicitly
2. Add `default:` case if intentionally ignoring some values

**Example**:
```go
// Before
switch status {
case StatusRunning:
    return "running"
}

// After (explicit)
switch status {
case StatusRunning:
    return "running"
case StatusCompleted:
    return "completed"
case StatusFailed:
    return "failed"
default:
    return "unknown"
}
```

### "Error return value of fmt.Errorf should be wrapped with %w"

**Problem**: Using `%v` instead of `%w` breaks error chains.

**Solution**: Always use `%w` for errors:

```go
// Before
return fmt.Errorf("failed: %v", err)

// After
return fmt.Errorf("failed: %w", err)
```

### "Subprocess launched with variable (G204)"

**Problem**: gosec detects potential command injection.

**Solution**:
1. If intentional (like AWF's shell executor), add `//nolint:gosec // G204 intentional`
2. Otherwise, use `ShellEscape()` from `pkg/interpolation`

### "File inclusion via variable (G304)"

**Problem**: gosec detects potential path traversal.

**Solution**:
1. Validate file paths against allowed directories
2. Use `filepath.Clean()` to normalize paths
3. Add `//nolint:gosec // G304 validated` if checks are in place

### "Append result not assigned to the same slice"

**Problem**: Forgetting that `append` returns new slice.

**Solution**:
```go
// Before
append(items, newItem)  // ❌ Result discarded

// After
items = append(items, newItem)  // ✅ Assigned
```

## Best Practices

### When to Use //nolint

Use `//nolint` directives sparingly and only when:
1. **Linter is wrong** - False positive that can't be fixed
2. **Intentional violation** - Pattern required by design (e.g., shell executor using variable commands)
3. **External constraint** - Third-party API forces non-idiomatic code

**Format**:
```go
//nolint:lintername // Reason for suppression
cmd := exec.Command("sh", "-c", userCmd)  // G204 intentional - shell executor
```

**Good reasons**:
- `G204 intentional - shell executor design`
- `G304 validated - path checked against allowlist`
- `gocognit - Cobra command setup legitimately complex`

**Bad reasons**:
- `TODO fix later` - Fix now or file issue
- `linter is annoying` - Linter is usually right
- No comment - Always explain

### Rejected Linters Rationale

Some popular linters are **intentionally not enabled** because they conflict with CLI tool patterns:

#### funlen - Function Length
**Why rejected**: CLI command handlers are legitimately long due to:
- Cobra command setup (flags, descriptions, examples)
- Flag parsing and validation
- Business logic invocation
- Output formatting

**Example**: `internal/interfaces/cli/run.go` RunCmd is 150 lines but clear and readable. Breaking it into artificial helpers would reduce clarity.

#### gochecknoglobals - Global Variables
**Why rejected**: CLI tools require package-level variables for:
- Logger instances (`var logger *zap.Logger`)
- Configuration paths (`var configPath string`)
- Cobra root command (`var rootCmd *cobra.Command`)

**Pattern**: Globals acceptable in `cmd/` and `internal/interfaces/cli/`. Domain layer must not use globals.

#### wsl - Whitespace Linter
**Why rejected**: Too opinionated and controversial. Team prefers gofumpt's deterministic rules over wsl's subjective whitespace requirements.

### Cognitive Complexity Guidelines

**Target**: Keep functions under complexity 15

**Strategies**:
1. **Extract helpers**: Move nested blocks to separate functions
2. **Early returns**: Reduce nesting with guard clauses
3. **Table-driven logic**: Replace nested switches with maps
4. **Strategy pattern**: Replace conditional chains with interfaces

**Complexity Budget**:
- **0-5**: Simple functions (getters, formatters)
- **6-10**: Standard business logic
- **11-15**: Complex coordination (acceptable with clear structure)
- **16+**: Refactor required (unless `//nolint:gocognit` justified)

### Error Wrapping Strategy

**Rule**: Wrap errors at every layer boundary with context:

```go
// Domain layer (internal/domain/workflow)
func (s *Service) Validate(w *Workflow) error {
    if err := validateStates(w.States); err != nil {
        return fmt.Errorf("validate states: %w", err)
    }
    return nil
}

// Application layer (internal/application)
func (s *WorkflowService) Execute(ctx context.Context, w *Workflow) error {
    if err := s.validator.Validate(w); err != nil {
        return fmt.Errorf("validate workflow: %w", err)
    }
    return nil
}

// Interface layer (internal/interfaces/cli)
func RunCmd(cmd *cobra.Command, args []string) error {
    if err := service.Execute(ctx, workflow); err != nil {
        return fmt.Errorf("execute workflow %s: %w", workflow.Name, err)
    }
    return nil
}
```

**Result**: Error messages show full context:
```
execute workflow deploy: validate workflow: validate states: state "step1" missing command
```

### Pre-Commit Checklist

Before committing, run:

```bash
make quality  # Runs lint + fmt + vet + test
```

If `make lint` reports issues:

```bash
make fix      # Auto-fix issues
make lint     # Verify remaining issues
```

For remaining issues:
1. Read error message carefully
2. Consult "Common Issues" section above
3. Fix manually or add justified `//nolint`
4. Verify fix: `make lint`

### CI Pipeline Integration

GitHub Actions runs the same checks locally:

```yaml
# .github/workflows/ci.yaml
- name: Lint
  uses: golangci/golangci-lint-action@v9
  with:
    version: latest
```

**Local == CI**: If `make quality` passes locally, CI will pass.

### Formatter Integration

Always run formatter before linting:

```bash
make fmt   # Format with gofumpt
make lint  # Check for issues
```

**Why?** Some lint issues are auto-fixed by formatting (import order, blank lines).

## References

- [golangci-lint documentation](https://golangci-lint.run/)
- [gofumpt repository](https://github.com/mvdan/gofumpt)
- [Effective Go](https://go.dev/doc/effective_go)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- AWF hexagonal architecture: `docs/architecture/hexagonal.md`
- Contributing guidelines: `CONTRIBUTING.md`
