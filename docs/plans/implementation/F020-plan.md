# Implementation Plan: F020 Interactive Mode

## Summary

Implement step-by-step interactive execution mode for `awf run --interactive`. The executor pauses before each step, displays step details, prompts for user action (continue/skip/abort/inspect/edit/retry), executes the step, then shows results. Following the F019 dry-run pattern: self-contained executor with copied execution loop, new UI prompt component, and minimal domain changes.

## ASCII Wireframe - Final UI

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  $ awf run analyze-code --input file=app.py --interactive                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Interactive Mode: analyze-code                                             │
│  ══════════════════════════════                                             │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │ [Step 1/4] validate                                                     ││
│  │ Type: step                                                              ││
│  │ Command: test -f "app.py" && echo "valid"                               ││
│  │ Timeout: 5s                                                             ││
│  │ → on_success: extract                                                   ││
│  │ → on_failure: error                                                     ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                             │
│  [c]ontinue [s]kip [a]bort [i]nspect [e]dit > c                             │
│                                                                             │
│  Executing validate...                                                      │
│  ────────────────────                                                       │
│  Output: valid                                                              │
│  Exit: 0 | Duration: 0.12s | Status: ✓ completed                            │
│                                                                             │
│  → Next: extract                                                            │
│                                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│  [Step 2/4] extract                                                         │
│  ...                                                                        │
│                                                                             │
│  [c]ontinue [s]kip [a]bort [i]nspect [e]dit [r]etry > i                     │
│                                                                             │
│  ┌─ Context ────────────────────────────────────────────────────────────────┐│
│  │ Inputs:                                                                 ││
│  │   file: app.py                                                          ││
│  │ States:                                                                 ││
│  │   validate.output: valid                                                ││
│  │   validate.exit_code: 0                                                 ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                                                             │
│  [c]ontinue [s]kip [a]bort [i]nspect [e]dit [r]etry > c                     │
│  ...                                                                        │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Implementation Steps

### Step 1: Create Interactive Action Types
- **File**: `internal/domain/workflow/interactive.go`
- **Action**: CREATE
- **Changes**:
  ```go
  type InteractiveAction string
  const (
      ActionContinue InteractiveAction = "continue"
      ActionSkip     InteractiveAction = "skip"  
      ActionAbort    InteractiveAction = "abort"
      ActionInspect  InteractiveAction = "inspect"
      ActionEdit     InteractiveAction = "edit"
      ActionRetry    InteractiveAction = "retry"
  )
  
  type InteractiveStepInfo struct {
      Name        string
      Index       int
      Total       int
      Step        *Step
      Command     string  // resolved command
      Transitions []string
  }
  ```

### Step 2: Create Interactive Prompt Port Interface
- **File**: `internal/domain/ports/interactive.go`
- **Action**: CREATE
- **Changes**:
  ```go
  type InteractivePrompt interface {
      ShowHeader(workflowName string)
      ShowStepDetails(info *workflow.InteractiveStepInfo)
      PromptAction(hasRetry bool) (workflow.InteractiveAction, error)
      ShowExecuting(stepName string)
      ShowStepResult(state *workflow.StepState, nextStep string)
      ShowContext(ctx *interpolation.Context)
      EditInput(name string, current any) (any, error)
      ShowAborted()
      ShowSkipped(stepName string, nextStep string)
  }
  ```

### Step 3: Create CLI Prompt Implementation
- **File**: `internal/interfaces/cli/ui/interactive_prompt.go`
- **Action**: CREATE
- **Changes**:
  - `CLIPrompt` struct with `io.Reader`, `io.Writer`, `*Colorizer`
  - Implement all `InteractivePrompt` interface methods
  - Single-char input parsing (`c`, `s`, `a`, `i`, `e`, `r`)
  - Context display formatting (inputs, states)
  - Input editing with readline-style prompt

### Step 4: Create Interactive Executor
- **File**: `internal/application/interactive_executor.go`
- **Action**: CREATE
- **Changes**:
  ```go
  type InteractiveExecutor struct {
      wfSvc           *WorkflowService
      executor        ports.CommandExecutor
      parallelExecutor ports.ParallelExecutor
      store           ports.StateStore
      logger          ports.Logger
      resolver        interpolation.Resolver
      evaluator       ExpressionEvaluator
      hookExecutor    *HookExecutor
      loopExecutor    *LoopExecutor
      templateSvc     *TemplateService
      prompt          ports.InteractivePrompt
      breakpoints     map[string]bool  // steps to pause at (nil = all)
      stdoutWriter    io.Writer
      stderrWriter    io.Writer
  }
  
  func (e *InteractiveExecutor) Run(ctx context.Context, workflowName string, inputs map[string]any) (*workflow.ExecutionContext, error)
  ```
  - Copy execution loop from `ExecutionService.Run()`
  - Add pause point before each step
  - Handle all 6 actions
  - Track step count for "Step N/M" display
  - Support `--breakpoint` for selective pausing

### Step 5: Add CLI Flags and Wiring
- **File**: `internal/interfaces/cli/run.go`
- **Action**: MODIFY
- **Changes**:
  - Add `--interactive` bool flag
  - Add `--breakpoint` string array flag
  - Add `runInteractive()` function
  - TTY detection: skip interactive if `!term.IsTerminal(int(os.Stdin.Fd()))`
  - Wire up dependencies similar to `runDryRun()`

### Step 6: Unit Tests for Prompt Component
- **File**: `internal/interfaces/cli/ui/interactive_prompt_test.go`
- **Action**: CREATE
- **Changes**:
  - Table-driven tests for action parsing
  - Mock stdin with `bytes.Buffer`
  - Test each action response
  - Test invalid input handling
  - Test context formatting

### Step 7: Unit Tests for Interactive Executor
- **File**: `internal/application/interactive_executor_test.go`
- **Action**: CREATE
- **Changes**:
  - Mock prompt returning action sequences
  - Test continue flow
  - Test skip flow
  - Test abort flow
  - Test retry after failure
  - Test edit input during execution
  - Test breakpoint filtering

### Step 8: Integration Tests
- **File**: `tests/integration/interactive_test.go`
- **Action**: CREATE
- **Changes**:
  - Pre-scripted stdin responses
  - Test full workflow with continue
  - Test abort mid-workflow
  - Test skip step
  - Test breakpoint-only pausing
  - Verify state persistence after abort

## Files to Modify

| File | Action | Complexity | Description |
|------|--------|------------|-------------|
| `internal/domain/workflow/interactive.go` | CREATE | S | Action types, step info struct |
| `internal/domain/ports/interactive.go` | CREATE | S | Prompt port interface |
| `internal/interfaces/cli/ui/interactive_prompt.go` | CREATE | M | CLI prompt implementation |
| `internal/interfaces/cli/ui/interactive_prompt_test.go` | CREATE | M | Prompt unit tests |
| `internal/application/interactive_executor.go` | CREATE | L | Main executor with pause logic |
| `internal/application/interactive_executor_test.go` | CREATE | M | Executor unit tests |
| `internal/interfaces/cli/run.go` | MODIFY | S | Add flags, wire up executor |
| `tests/integration/interactive_test.go` | CREATE | M | Integration tests |

## Test Plan

### Unit Tests
- **Prompt parsing**: `c`, `s`, `a`, `i`, `e`, `r` → correct actions
- **Invalid input**: `x`, `123`, empty → re-prompt
- **Context display**: Inputs/states formatted correctly
- **Edit flow**: Read new value, validate type
- **Executor actions**: Each action triggers correct behavior
- **Breakpoints**: Only specified steps pause

### Integration Tests
```go
func TestInteractive_ContinueThrough_AllSteps(t *testing.T)
func TestInteractive_Abort_StopsExecution(t *testing.T)  
func TestInteractive_Skip_JumpsToOnSuccess(t *testing.T)
func TestInteractive_Inspect_ShowsContext(t *testing.T)
func TestInteractive_Edit_ModifiesInput(t *testing.T)
func TestInteractive_Retry_ReExecutesStep(t *testing.T)
func TestInteractive_Breakpoint_PausesOnlyAtSpecified(t *testing.T)
func TestInteractive_NonTTY_FallsBackToNormal(t *testing.T)
func TestInteractive_AbortPersistsState_CanResume(t *testing.T)
```

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Non-TTY environments (CI/pipes) | High | Medium | Auto-detect with `term.IsTerminal()`, fall back to non-interactive with warning |
| Signal handling during prompts | Medium | Medium | Context cancellation propagates cleanly; prompt returns `ActionAbort` on SIGINT |
| Edit type coercion | Low | Low | Parse edited value same as CLI input flags (string→any) |
| Parallel/loop steps complexity | Medium | Medium | For v1: pause before parallel/loop, not inside; document limitation |
| Test flakiness with stdin | Medium | Low | Use deterministic `bytes.Buffer`, avoid timing-dependent tests |

## Acceptance Criteria Checklist

- [ ] `awf run --interactive` enables step-by-step mode
- [ ] Pause before each step with prompt
- [ ] Options: continue, skip, abort, inspect, edit, retry
- [ ] Show step details before execution (name, type, command, timeout, transitions)
- [ ] Show output after execution (stdout, exit code, duration, status)
- [ ] Allow modifying inputs during execution (`e` action)
- [ ] Support breakpoints on specific states (`--breakpoint step1,step2`)
- [ ] Non-TTY fallback to normal execution

## Suggested Implementation Order

1. Domain types (Step 1) - 15 min
2. Port interface (Step 2) - 15 min  
3. CLI prompt (Step 3) + tests (Step 6) - 1.5 hr
4. Interactive executor (Step 4) + tests (Step 7) - 2 hr
5. CLI wiring (Step 5) - 30 min
6. Integration tests (Step 8) - 1 hr

**Total estimate**: ~6 hours (M complexity as spec states)

