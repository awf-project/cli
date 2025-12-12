# Testing

AWF follows Go testing conventions with table-driven tests and clear separation between unit and integration tests.

## Running Tests

```bash
# All tests
make test

# Unit tests only (internal/ and pkg/)
make test-unit

# Integration tests only (tests/integration/)
make test-integration

# With race detector
make test-race

# With coverage report
make test-coverage
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

Test full workflows end-to-end:

```go
func TestCLI_Run_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

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

Skip integration tests in CI with short flag:

```bash
go test -short ./...
```

## Test Helpers

Common test utilities:

```go
// tests/helpers.go
package tests

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
      - run: make test
      - run: make test-race
```

## See Also

- [Architecture](architecture.md) - Code organization
- [Project Structure](project-structure.md) - Directory layout
- [Contributing](../../CONTRIBUTING.md) - Development workflow
