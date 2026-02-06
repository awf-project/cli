# Testing

AWF follows Go testing conventions with table-driven tests and clear separation between unit and integration tests.

## Running Tests

```bash
# All tests (unit tests only, excludes integration and external)
make test

# Unit tests only (internal/ and pkg/)
make test-unit

# Integration tests (requires full system setup, tagged with //go:build integration)
make test-integration

# External tests (requires external CLIs: claude, codex, gemini, opencode)
make test-external

# All tests including integration
make test-all

# With race detector
make test-race

# With coverage report
make test-coverage

# Run tests in short mode (skips resource-intensive tests)
go test -short ./...
```

## Build Tags

AWF uses Go build tags to control which tests run in different environments. This avoids runtime skips that inflate skip counts and obscure coverage metrics.

### Available Build Tags

| Tag | Purpose | Usage | Example |
|-----|---------|-------|---------|
| `integration` | Full system tests requiring setup, state persistence, CLI execution | `make test-integration` or `go test -tags=integration ./...` | End-to-end workflow execution |
| `external` | Tests requiring external CLI tools (claude, codex, gemini, opencode) | `make test-external` or `go test -tags=external ./...` | AI provider validation |
| `slow` | Resource-intensive tests (high memory, concurrency, long-running) | `go test -tags=slow ./...` | Memory leak detection, stress tests |
| `!short` | Standard Go short mode exclusion for tests that take >100ms | `go test -short ./...` (excludes these) | Database operations, file I/O |

### Using Build Tags

Add build tags at the top of test files (before package declaration):

```go
//go:build integration

package integration_test

import "testing"

func TestFullWorkflowExecution(t *testing.T) {
    // This test only runs with: go test -tags=integration
    // No need for runtime t.Skip() calls
}
```

Multiple tags can be combined:

```go
//go:build integration && external

package integration_test
// Requires both -tags=integration,external
```

Exclude from default tests:

```go
//go:build !short

package workflow_test
// Excluded when running: go test -short ./...
```

## Test Structure

### Unit Tests

Located alongside the code they test:

```
internal/
├── domain/workflow/
│   ├── workflow.go
│   └── workflow_test.go
├── infrastructure/repository/
│   ├── yaml.go
│   └── yaml_test.go
└── ...
```

### Integration Tests

Located in `tests/integration/`:

```
tests/
├── integration/
│   ├── cli_test.go
│   └── workflow_test.go
└── fixtures/
    └── workflows/
        ├── simple.yaml
        └── parallel.yaml
```

## Table-Driven Tests

AWF uses table-driven tests for comprehensive coverage:

```go
func TestWorkflowValidation(t *testing.T) {
    tests := []struct {
        name     string
        workflow *workflow.Workflow
        wantErr  bool
        errMsg   string
    }{
        {
            name: "valid workflow",
            workflow: &workflow.Workflow{
                Name:    "test",
                Initial: "step1",
                States: map[string]workflow.State{
                    "step1": &workflow.StepState{Name: "step1"},
                },
            },
            wantErr: false,
        },
        {
            name: "missing initial state",
            workflow: &workflow.Workflow{
                Name:    "test",
                Initial: "nonexistent",
                States:  map[string]workflow.State{},
            },
            wantErr: true,
            errMsg:  "initial state 'nonexistent' not found",
        },
        // ... more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.workflow.Validate()
            if tt.wantErr {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

## Fixtures

Test workflows are in `tests/fixtures/workflows/`:

```yaml
# tests/fixtures/workflows/simple.yaml
name: simple
version: "1.0.0"

states:
  initial: step1
  step1:
    type: step
    command: echo "hello"
    on_success: done
  done:
    type: terminal
```

Load fixtures in tests:

```go
func TestLoadWorkflow(t *testing.T) {
    repo := repository.NewYAMLRepository("../../tests/fixtures/workflows")

    wf, err := repo.Load("simple")
    require.NoError(t, err)
    assert.Equal(t, "simple", wf.Name)
}
```

## Mocking

Use interfaces for easy mocking:

```go
// Mock executor for testing
type mockExecutor struct {
    results map[string]ports.Result
    err     error
}

func (m *mockExecutor) Execute(ctx context.Context, cmd ports.Command) (ports.Result, error) {
    if m.err != nil {
        return ports.Result{}, m.err
    }
    return m.results[cmd.Command], nil
}

func TestExecutionWithMock(t *testing.T) {
    mock := &mockExecutor{
        results: map[string]ports.Result{
            "echo hello": {Output: "hello\n", ExitCode: 0},
        },
    }

    service := application.NewExecutionService(repo, store, mock)
    // ... test execution
}
```

## Race Detection

Test concurrent code with race detector:

```bash
make test-race
```

Example race condition test:

```go
func TestJSONStore_RaceSaveLoad(t *testing.T) {
    store := state.NewJSONStore(t.TempDir())
    ctx := &workflow.ExecutionContext{ID: "test-123"}

    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(2)
        go func() {
            defer wg.Done()
            _ = store.Save(ctx)
        }()
        go func() {
            defer wg.Done()
            _, _ = store.Load(ctx.ID)
        }()
    }
    wg.Wait()
}
```

## Deterministic Assertions

Test assertions must be deterministic — they should produce the same result on every run regardless of system state, parallel jobs, or CI environment.

### Avoid Testing OS Guarantees

Do not write assertions that verify operating system behavior (e.g., process group signal delivery, file descriptor cleanup). Test your application's response to those behaviors instead.

**Example (bad — tests OS behavior with system-wide search):**

```go
// Searches ALL system processes — matches unrelated commands from parallel CI jobs
time.Sleep(200 * time.Millisecond)
cmd := exec.CommandContext(ctx, "pgrep", "-f", "sleep 10")
output, _ := cmd.Output()
assert.Empty(t, output, "orphan processes should be cleaned up")
```

**Example (good — tests application behavior deterministically):**

```go
// Verify the application correctly propagates context cancellation
assert.True(t, errors.Is(err, context.Canceled))

// Verify process group configuration is set (structural check)
assert.True(t, cmd.SysProcAttr.Setpgid)
```

### Guidelines

- Assert on application-level return values, errors, and state — not on system-level side effects
- Avoid `time.Sleep` before assertions — if timing is needed, use channels or `sync.WaitGroup`
- Never use system-wide searches (`pgrep`, `ps aux`) in tests — they match unrelated processes
- Prefer structural assertions (configuration is set) over behavioral assertions (effect was observed)

## Coverage

Generate coverage report:

```bash
make test-coverage
# Opens coverage.html in browser
```

Coverage goals:
- Domain layer: >90%
- Application layer: >80%
- Infrastructure layer: >70%
- CLI: Integration tests cover main paths

## Integration Tests

Integration tests use build tags instead of runtime skips:

```go
//go:build integration

package integration_test

import (
    "os/exec"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestCLI_Run_Integration(t *testing.T) {
    // No runtime skip needed - build tag controls execution

    // Setup temp directory with workflow
    dir := t.TempDir()
    setupFixtures(t, dir)

    // Run CLI command
    cmd := exec.Command("./bin/awf", "run", "simple", "--storage", dir)
    output, err := cmd.CombinedOutput()

    require.NoError(t, err)
    assert.Contains(t, string(output), "hello")
}
```

Run integration tests explicitly:

```bash
# Integration tests only
make test-integration

# All tests including integration
make test-all
```

## Test Helpers

Common test utilities in `tests/integration/test_helpers_test.go`:

```go
package integration_test

func TempWorkflow(t *testing.T, content string) string {
    t.Helper()
    dir := t.TempDir()
    path := filepath.Join(dir, "workflow.yaml")
    err := os.WriteFile(path, []byte(content), 0644)
    require.NoError(t, err)
    return path
}

func AssertExitCode(t *testing.T, err error, expected int) {
    t.Helper()
    var exitErr *exec.ExitError
    if errors.As(err, &exitErr) {
        assert.Equal(t, expected, exitErr.ExitCode())
    } else if expected != 0 {
        t.Errorf("expected exit code %d, got no error", expected)
    }
}
```

### Skip Helper Functions

For cases where runtime skip checks are necessary (environment-dependent tests):

```go
// skipIfCLINotInstalled skips test if required CLI tool is not in PATH
func skipIfCLINotInstalled(t *testing.T, cliName string) {
    t.Helper()
    if _, err := exec.LookPath(cliName); err != nil {
        t.Skipf("ENVIRONMENT: %s CLI not installed", cliName)
    }
}

// skipIfToolNotAvailable skips test if specified command/tool is unavailable
func skipIfToolNotAvailable(t *testing.T, toolName, checkCmd string) {
    t.Helper()
    cmd := exec.Command("sh", "-c", checkCmd)
    if err := cmd.Run(); err != nil {
        t.Skipf("ENVIRONMENT: %s not available", toolName)
    }
}

// skipOnPlatform skips test on specified OS/arch combinations
func skipOnPlatform(t *testing.T, goos, goarch, reason string) {
    t.Helper()
    if runtime.GOOS == goos && (goarch == "" || runtime.GOARCH == goarch) {
        t.Skipf("PLATFORM: %s (OS=%s, ARCH=%s)", reason, goos, goarch)
    }
}

// skipIfNotRoot skips test if not running with root/admin privileges
func skipIfNotRoot(t *testing.T) {
    t.Helper()
    if os.Geteuid() != 0 {
        t.Skip("PERMISSION: requires root privileges")
    }
}
```

**Usage:**

```go
func TestProviderValidation(t *testing.T) {
    skipIfCLINotInstalled(t, "claude")
    // Test code using claude CLI
}

func TestDockerWorkflow(t *testing.T) {
    skipIfToolNotAvailable(t, "docker", "docker ps")
    // Test code using docker
}

func TestWindowsPathHandling(t *testing.T) {
    skipOnPlatform(t, "linux", "", "Windows-specific path handling")
    // Windows-only test code
}
```

## Assertions

AWF uses testify for assertions:

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestExample(t *testing.T) {
    // require stops test on failure
    require.NoError(t, err)
    require.NotNil(t, result)

    // assert continues after failure
    assert.Equal(t, expected, actual)
    assert.Contains(t, haystack, needle)
    assert.Len(t, slice, 5)
}
```

## Test Naming

Follow Go conventions:

```go
// Unit test: Test<Function>_<Scenario>
func TestValidate_MissingInitialState(t *testing.T)

// Integration test: Test<Component>_<Action>_Integration
func TestCLI_Run_FailingCommand_Integration(t *testing.T)

// Benchmark: Benchmark<Function>
func BenchmarkInterpolate(b *testing.B)
```

## Skip Policy

AWF minimizes runtime test skips to maintain accurate coverage metrics and test signal. This policy was enforced by C053, which cleaned up 50+ problematic `t.Skip()` calls: removing dead code, deleting empty stubs, implementing missing nil-guard behavior, and converting unconditional skips to proper `testing.Short()` patterns. Follow these guidelines:

### When NOT to Skip

**Use build tags instead of runtime skips for:**

- Integration tests requiring system setup → `//go:build integration`
- Tests requiring external CLI tools → `//go:build external`
- Resource-intensive tests → `//go:build slow` or `//go:build !short`
- Platform-specific tests → `//go:build linux` or `//go:build windows`

**Example (❌ BAD - runtime skip):**

```go
func TestCLIExecution(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    // test code
}
```

**Example (✅ GOOD - build tag):**

```go
//go:build integration

package integration_test

func TestCLIExecution(t *testing.T) {
    // No runtime skip needed
    // test code
}
```

### When to Skip

Runtime skips are acceptable ONLY for:

1. **Environment checks** - Missing tools, permissions, or platform features
2. **CI-specific conditions** - Flaky external dependencies
3. **Pending work** - Must have tracking issue

### Skip Documentation Format

All runtime skips MUST follow this format:

```go
t.Skip("CATEGORY: description [#issue]")
```

Categories:
- `ENVIRONMENT` - Missing tool, permission, or platform feature
- `PLATFORM` - OS/arch specific issue
- `PERMISSION` - Requires root or special privileges
- `PENDING` - Awaiting design decision or implementation
- `FLAKY` - Known intermittent failure (use sparingly, prefer fix)

**Examples:**

```go
// Environment check with helper
skipIfCLINotInstalled(t, "claude") // Outputs: "ENVIRONMENT: claude CLI not installed"

// Platform-specific skip
skipOnPlatform(t, "windows", "", "Unix socket support") // Outputs: "PLATFORM: Unix socket support (OS=windows, ARCH=)"

// Pending work (MUST link tracking issue)
t.Skip("PENDING: max_turns validation not yet implemented [#142]")

// Flaky test (discouraged - prefer fixing)
t.Skip("FLAKY: external API timeout in CI [#156]")
```

### Skip Verification

Before committing:

```bash
# Count runtime skips (target: minimize)
grep -r "t\.Skip(" --include="*_test.go" | wc -l

# Verify all skips have proper format
grep -r "t\.Skip(" --include="*_test.go" | grep -v "ENVIRONMENT:\|PLATFORM:\|PERMISSION:\|PENDING:\|FLAKY:"
# Should return empty (no undocumented skips)

# Verify build tags work
go test ./...                          # Unit tests only
go test -tags=integration ./...        # Include integration tests
go test -tags=external ./...           # Include external CLI tests
```

## CI Integration

Tests run in GitHub Actions:

```yaml
# .github/workflows/ci.yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      - run: make test              # Unit tests only
      - run: make test-integration  # Integration tests
      - run: make test-race         # Race detection
```

Integration and external tests may be run in separate CI jobs or only on specific branches to optimize CI time.

## See Also

- [Architecture](architecture.md) - Code organization
- [Project Structure](project-structure.md) - Directory layout
- [Contributing](../../CONTRIBUTING.md) - Development workflow
