# F066 Code Review Corrections Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix 2 blocking issues and 5 quality issues found in F066 code review before merge.

**Architecture:** Three independent correction streams designed for parallel subagent execution. Stream A fixes interactive executor terminal handling (application layer). Stream B adds ExitCode propagation (domain → infra → app). Stream C applies code quality fixes across test and source files.

**Tech Stack:** Go, testify, TDD (red-green-refactor)

---

## Parallelization Map

```
┌─────────────────────┐  ┌─────────────────────┐  ┌─────────────────────┐
│  STREAM A (Agent 1)  │  │  STREAM B (Agent 2)  │  │  STREAM C (Agent 3)  │
│                      │  │                      │  │                      │
│  Issue #2: Interactive│  │  Issues #1,#3,#4:    │  │  Issues #6,#7,#8,    │
│  executor terminal   │  │  ExitCode field +    │  │  #9,#10,#11:         │
│  failure handling    │  │  status propagation  │  │  Code quality fixes  │
│                      │  │                      │  │                      │
│  Files:              │  │  Files:              │  │  Files:              │
│  - interactive_      │  │  - step.go           │  │  - execution_        │
│    executor.go       │  │  - yaml_mapper.go    │  │    service.go (:260) │
│  - interactive_      │  │  - yaml_mapper_on_   │  │  - yaml_mapper.go    │
│    executor_test.go  │  │    failure_test.go   │  │    (doc comments)    │
│    (NEW)             │  │  - builders.go       │  │  - on_failure_       │
│                      │  │  - execution_service │  │    inline_test.go    │
│  NO OVERLAP         │  │    .go (:260,:1482)  │  │    (statesDir,       │
│                      │  │  - execution_service │  │     helper)          │
│                      │  │    _on_failure_      │  │                      │
│                      │  │    inline_test.go    │  │                      │
│                      │  │  - on_failure_       │  │                      │
│                      │  │    inline_test.go    │  │                      │
└─────────────────────┘  └─────────────────────┘  └─────────────────────┘
         │                         │                         │
         └─────────────┬───────────┘─────────────────────────┘
                       ▼
              Final: run all tests
```

**Conflict zones:** Stream B and C both touch `execution_service.go` but at different lines (B: terminal message construction; C: `fmt.Errorf` → `errors.New`). Stream C's `fmt.Errorf` fix at `:260` and `:1482` overlaps with Stream B's ExitCode changes at the same lines. **Resolution: Stream C must NOT touch lines 260 and 1482. Stream B owns those lines and will apply the `errors.New` fix as part of its work.**

---

## Stream A: Interactive Executor Terminal Failure Handling

**Review issue:** #2 (high severity) — Interactive executor treats ALL terminal steps as success
**Spec reference:** Plan risk R4

### Task A1: Write failing test for interactive executor terminal failure

**Files:**
- Create: `internal/application/interactive_executor_terminal_test.go`

**Step 1: Write the failing test**

The test must verify that when an interactive executor reaches a `TerminalFailure` step with a `Message`, it returns an error containing that message and sets `StatusFailed`.

```go
package application

import (
	"context"
	"testing"

	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInteractiveExecutor_TerminalFailure_ReturnsError(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test-terminal-failure",
		Initial: "failing_step",
		Steps: map[string]*workflow.Step{
			"failing_step": {
				Name:      "failing_step",
				Type:      workflow.StepTypeCommand,
				Command:   "exit 1",
				OnFailure: "__inline_error_failing_step",
			},
			"__inline_error_failing_step": {
				Name:    "__inline_error_failing_step",
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "Custom failure message",
			},
		},
	}

	// Build interactive executor with mocks
	// Agent must discover the actual constructor and mock patterns
	// by reading NewInteractiveExecutor and existing test files
	// Key: the executor must reach the terminal step and return error

	_ = wf // Agent implements full test setup
}

func TestInteractiveExecutor_TerminalSuccess_NoError(t *testing.T) {
	// Verify terminal success still returns nil error (no regression)
}

func TestInteractiveExecutor_TerminalFailure_MessageInterpolated(t *testing.T) {
	// Verify message interpolation works in interactive mode
}
```

> **Note to agent:** The test skeleton above is illustrative. You MUST read `NewInteractiveExecutor`, the `InteractiveExecutor.Run` method, and existing test patterns in the application package to write the actual working test. The interactive executor uses a `prompt` interface — you'll need a mock that auto-continues past breakpoints.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/application/ -run TestInteractiveExecutor_Terminal -v`
Expected: FAIL — terminal failure returns nil error and StatusCompleted

### Task A2: Fix interactive executor terminal handling

**Files:**
- Modify: `internal/application/interactive_executor.go:157-163`

**Step 1: Implement the fix**

Replace the current terminal block:

```go
// BEFORE (lines 157-163):
if step.Type == workflow.StepTypeTerminal {
    execCtx.Status = workflow.StatusCompleted
    execCtx.CompletedAt = time.Now()
    e.checkpoint(ctx, execCtx)
    e.prompt.ShowCompleted(execCtx.Status)
    return execCtx, nil
}
```

With the pattern from ExecutionService (lines 255-271):

```go
// AFTER:
if step.Type == workflow.StepTypeTerminal {
    if step.Status == workflow.TerminalFailure {
        execCtx.Status = workflow.StatusFailed
        interpCtx := e.buildInterpolationContext(execCtx)
        if msg := e.interpolateTerminalMessage(step.Message, interpCtx); msg != "" {
            execCtx.CompletedAt = time.Now()
            e.checkpoint(ctx, execCtx)
            e.prompt.ShowCompleted(execCtx.Status)
            return execCtx, errors.New(msg)
        }
        execCtx.CompletedAt = time.Now()
        e.checkpoint(ctx, execCtx)
        e.prompt.ShowCompleted(execCtx.Status)
        return execCtx, fmt.Errorf("workflow reached terminal failure state: %s", currentStep)
    }
    execCtx.Status = workflow.StatusCompleted
    execCtx.CompletedAt = time.Now()
    e.checkpoint(ctx, execCtx)
    e.prompt.ShowCompleted(execCtx.Status)
    return execCtx, nil
}
```

**Step 2: Add `interpolateTerminalMessage` method to InteractiveExecutor**

The InteractiveExecutor already has a `resolver` field and `buildInterpolationContext`. Add:

```go
// interpolateTerminalMessage interpolates a terminal step message template.
func (e *InteractiveExecutor) interpolateTerminalMessage(message string, intCtx *interpolation.Context) string {
    if message == "" {
        return ""
    }
    interpolated, err := e.resolver.Resolve(message, intCtx)
    if err != nil {
        e.logger.Warn("terminal message interpolation failed", "error", err, "message", message)
        return message
    }
    return interpolated
}
```

> **Note to agent:** Check if `InteractiveExecutor` has a `resolver` field. If not, check how it accesses the interpolation resolver — it may delegate through a different mechanism.

**Step 3: Run test to verify it passes**

Run: `go test ./internal/application/ -run TestInteractiveExecutor_Terminal -v`
Expected: PASS

**Step 4: Run full test suite**

Run: `go test ./internal/application/... -count=1`
Expected: All PASS (no regressions)

---

## Stream B: ExitCode Field + Status Propagation

**Review issues:** #1 (high), #3 (medium), #4 (high) — `status` field silently discarded, integration test never asserts exit code
**Spec reference:** FR-004 ("status shall default to exit code 1")

### Task B1: Add ExitCode field to domain Step

**Files:**
- Modify: `internal/domain/workflow/step.go`
- Modify: `internal/testutil/builders/builders.go`
- Modify: `internal/testutil/builders/step_builder_message_test.go` (or new test)

**Step 1: Write failing test**

Add test in `internal/domain/workflow/step_message_test.go`:

```go
func TestStepExitCodeField_DefaultsToZero(t *testing.T) {
    step := &Step{
        Type:   StepTypeTerminal,
        Status: TerminalFailure,
    }
    assert.Equal(t, 0, step.ExitCode)
}
```

**Step 2: Add ExitCode field to Step struct**

In `internal/domain/workflow/step.go`, after the `Message` field (line 89):

```go
Message         string               // for terminal type: message template (interpolated at runtime)
ExitCode        int                  // for terminal type: process exit code (default 0, FR-004: inline default 1)
```

**Step 3: Add WithExitCode to StepBuilder**

In `internal/testutil/builders/builders.go`, add:

```go
func (b *StepBuilder) WithExitCode(code int) *StepBuilder {
    b.exitCode = code
    return b
}
```

And wire it in the `Build()` method.

**Step 4: Run tests**

Run: `go test ./internal/domain/workflow/... ./internal/testutil/builders/... -v`
Expected: PASS

### Task B2: Propagate status from YAML inline object

**Files:**
- Modify: `internal/infrastructure/repository/yaml_mapper.go` (synthesizeInlineErrorTerminal)
- Modify: `internal/infrastructure/repository/yaml_mapper.go` (mapStep)
- Modify: `internal/infrastructure/repository/yaml_types.go` (yamlStep — add ExitCode field if needed)
- Modify: `internal/infrastructure/repository/yaml_mapper_on_failure_test.go`

**Step 1: Write failing tests**

Add to `yaml_mapper_on_failure_test.go`:

```go
func TestSynthesizeInlineErrorTerminal_StatusMapsToExitCode(t *testing.T) {
    obj := map[string]any{
        "message": "Deploy failed",
        "status":  3,
    }

    got, err := synthesizeInlineErrorTerminal("deploy_step", obj)

    require.NoError(t, err)
    assert.Equal(t, 3, got.ExitCode)
}

func TestSynthesizeInlineErrorTerminal_DefaultExitCode1(t *testing.T) {
    // FR-004: When status omitted, default to exit code 1
    obj := map[string]any{
        "message": "Something went wrong",
    }

    got, err := synthesizeInlineErrorTerminal("step1", obj)

    require.NoError(t, err)
    assert.Equal(t, 1, got.ExitCode)
}
```

Also update `TestSynthesizeInlineErrorTerminal_MessageAndStatus` to assert `ExitCode`.

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/infrastructure/repository/ -run TestSynthesize -v`
Expected: FAIL (ExitCode is 0 for all cases)

**Step 3: Implement status extraction in synthesizeInlineErrorTerminal**

```go
// synthesizeInlineErrorTerminal creates a terminal yamlStep from an inline error object.
func synthesizeInlineErrorTerminal(stepName string, inlineError map[string]any) (*yamlStep, error) {
    msgVal := inlineError["message"]
    msg, ok := msgVal.(string)
    if !ok {
        return nil, fmt.Errorf("step %s: on_failure.message must be a string", stepName)
    }

    exitCode := 1 // FR-004: default exit code
    if statusVal, exists := inlineError["status"]; exists {
        switch v := statusVal.(type) {
        case int:
            exitCode = v
        case float64:
            exitCode = int(v)
        }
    }

    return &yamlStep{
        Type:     "terminal",
        Status:   "failure",
        Message:  msg,
        ExitCode: exitCode,
    }, nil
}
```

> **Note to agent:** YAML unmarshals numbers as `int` but JSON round-trips may produce `float64`. Handle both. Check if `yamlStep` already has an `ExitCode` field. If not, add it to `yaml_types.go`. Also ensure `mapStep` maps `ExitCode` to the domain Step.

**Step 4: Wire ExitCode in mapStep**

In `mapStep`, add `ExitCode: y.ExitCode` to the `workflow.Step` struct literal.

**Step 5: Run tests**

Run: `go test ./internal/infrastructure/repository/ -run TestSynthesize -v`
Expected: PASS

### Task B3: Use ExitCode in execution service terminal handling

**Files:**
- Modify: `internal/application/execution_service.go:257-263` and `:1479-1485`
- Modify: `internal/application/execution_service_on_failure_inline_test.go`

**Step 1: Write failing test**

Add to `execution_service_on_failure_inline_test.go`:

```go
func TestExecutionService_InlineOnFailure_ExitCodePropagated(t *testing.T) {
    wf := &workflow.Workflow{
        Name:    "test",
        Initial: "deploy",
        Steps: map[string]*workflow.Step{
            "deploy": {
                Name:      "deploy",
                Type:      workflow.StepTypeCommand,
                Command:   "make deploy",
                OnFailure: "__inline_error_deploy",
            },
            "__inline_error_deploy": {
                Name:     "__inline_error_deploy",
                Type:     workflow.StepTypeTerminal,
                Status:   workflow.TerminalFailure,
                Message:  "Deploy failed",
                ExitCode: 3,
            },
        },
    }

    execSvc, _ := NewTestHarness(t).
        WithWorkflow("test", wf).
        WithCommandResult("make deploy", &ports.CommandResult{Stdout: "", Stderr: "error", ExitCode: 1}).
        Build()

    execCtx, err := execSvc.Run(context.Background(), "test", nil)

    require.Error(t, err)
    assert.Equal(t, workflow.StatusFailed, execCtx.Status)
    assert.Equal(t, 3, execCtx.ExitCode)
}
```

> **Note to agent:** Check if `ExecutionContext` has an `ExitCode` field. If not, determine how exit codes are propagated to the CLI layer (it may be through the error type, a wrapper, or a context field). Adapt the test accordingly.

**Step 2: Implement ExitCode propagation in terminal handling**

In `execution_service.go`, modify both terminal handling blocks (Run ~L258 and Resume ~L1480):

```go
if step.Status == workflow.TerminalFailure {
    execCtx.Status = workflow.StatusFailed
    if step.ExitCode > 0 {
        execCtx.ExitCode = step.ExitCode
    }
    if msg := s.interpolateTerminalMessage(step.Message, s.buildInterpolationContext(execCtx)); msg != "" {
        execErr = errors.New(msg)  // Also fixes review issue #8: fmt.Errorf("%s", msg) → errors.New
    } else {
        execErr = fmt.Errorf("workflow reached terminal failure state: %s", currentStep)
    }
}
```

**Step 3: Run tests**

Run: `go test ./internal/application/ -run TestExecutionService_InlineOnFailure -v`
Expected: PASS

### Task B4: Fix integration test exit code assertion

**Files:**
- Modify: `tests/integration/features/on_failure_inline_test.go`

**Step 1: Fix TestOnFailureInline_MessageWithStatus to assert exit code**

The test declares `expectedExitCode` but never asserts it. Add assertion:

```go
// After the error message assertion, add:
// Assert exit code propagation (FR-004)
// Agent must determine how ExitCode is exposed — via execCtx or error type
```

> **Note to agent:** The integration test currently only calls `svc.Run()` and checks the error. You need to also capture the `ExecutionContext` return value and assert `ExitCode`. Adjust the test structure from `_, err := svc.Run(...)` to `execCtx, err := svc.Run(...)`.

**Step 2: Run integration tests**

Run: `go test -tags=integration ./tests/integration/features/ -run TestOnFailureInline_MessageWithStatus -v`
Expected: PASS

---

## Stream C: Code Quality Fixes

**Review issues:** #6 (dead code), #7 (doc comments), #9 (duplication), #10/#11 (dead statesDir)

### Task C1: Add doc comments to new functions in yaml_mapper.go

**Files:**
- Modify: `internal/infrastructure/repository/yaml_mapper.go`

**Step 1: Add doc comments**

```go
// synthesizeInlineErrorTerminal creates a terminal yamlStep from an inline error object.
// It extracts the message and optional status fields to build the synthesized terminal definition.
func synthesizeInlineErrorTerminal(...)

// validateInlineErrorObject validates the inline error object fields.
// It ensures the required message field is present and non-empty.
func validateInlineErrorObject(...)
```

### Task C2: Remove dead statesDir from integration tests

**Files:**
- Modify: `tests/integration/features/on_failure_inline_test.go`

**Step 1: Remove statesDir declarations**

In all 5 test functions that create `statesDir` + `os.MkdirAll(statesDir, ...)`, remove both lines since `mocks.NewMockStateStore()` is in-memory.

Lines to remove pattern (in each test function):
```go
statesDir := filepath.Join(tmpDir, "states")    // DELETE
require.NoError(t, os.MkdirAll(statesDir, 0o755)) // DELETE
```

Affected tests:
- `TestOnFailureInline_BasicMessage`
- `TestOnFailureInline_MessageWithStatus`
- `TestOnFailureInline_MessageInterpolation`
- `TestOnFailureInline_StringFormBackwardCompat`
- `TestOnFailureInline_ParallelBranch`

### Task C3: Extract test helper in integration tests

**Files:**
- Modify: `tests/integration/features/on_failure_inline_test.go`

**Step 1: Extract buildTestService helper**

Replace the 6-line duplicated setup block in 5 test functions:

```go
func buildTestService(t *testing.T, workflowsDir string) *application.ExecutionService {
    t.Helper()
    repo := repository.NewYAMLRepository(workflowsDir)
    store := mocks.NewMockStateStore()
    exec := executor.NewShellExecutor()
    logger := mocks.NewMockLogger()

    return builders.NewExecutionServiceBuilder().
        WithWorkflowRepository(repo).
        WithStateStore(store).
        WithExecutor(exec).
        WithLogger(logger).
        Build()
}
```

Then replace each occurrence with `svc := buildTestService(t, workflowsDir)`.

### Task C4: Remove redundant type-assertion in synthesizeInlineErrorTerminal

**Files:**
- Modify: `internal/infrastructure/repository/yaml_mapper.go`

**Step 1: Assess**

> **Note to agent:** Review issue #6 says the type-assertion on `message` in `synthesizeInlineErrorTerminal` is redundant because `validateInlineErrorObject` already validated it. This is defensive programming. If you remove it, ensure `normalizeOnFailure` ALWAYS calls `validateInlineErrorObject` before `synthesizeInlineErrorTerminal` is called. If the call chain guarantees this, simplify. If not, keep it as defensive code and add a comment explaining why.

### Task C5: Run full test suite

**Step 1: Run all tests**

```bash
go test ./internal/... ./pkg/... -count=1
go test -tags=integration ./tests/integration/... -count=1
```

**Step 2: Run linters**

```bash
golangci-lint run
```

Expected: All PASS, no lint errors.

---

## Execution Order

1. **Streams A, B, C run in parallel** (independent file sets, except noted conflict zone)
2. **After all 3 complete:** Run full test suite + linters as final validation
3. **Commit:** Single atomic commit with all corrections

## Commit Message

```
fix(workflow): address F066 code review issues

- Fix interactive executor terminal failure handling (was silently succeeding)
- Implement FR-004 status/ExitCode propagation from inline on_failure
- Fix integration test exit code assertion (was declared but never checked)
- Replace fmt.Errorf("%s", msg) with errors.New(msg)
- Add doc comments to synthesizeInlineErrorTerminal, validateInlineErrorObject
- Remove dead statesDir from integration tests
- Extract buildTestService helper in integration tests
```
