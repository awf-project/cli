Now I have a complete picture. Let me create the implementation plan:

# Implementation Plan: F019 - Dry-Run Mode

## Summary

Add a `--dry-run` flag to `awf run` that walks through the workflow state machine without executing commands. The feature resolves variable interpolations, shows the execution plan with resolved commands, displays all possible state transitions (including conditionals), and shows hooks that would run. This is implemented via a dedicated `DryRunExecutor` in the application layer and a `DryRunFormatter` in the UI layer.

## ASCII Wireframe

```
┌───────────────────────────────────────────────────────────────────┐
│  $ awf run my-workflow --input=val --dry-run                      │
├───────────────────────────────────────────────────────────────────┤
│                                                                   │
│  Dry Run: my-workflow                                             │
│  ═══════════════════                                              │
│                                                                   │
│  Inputs:                                                          │
│    file_path: app.py                                              │
│    output_format: markdown (default)                              │
│                                                                   │
│  Execution Plan:                                                  │
│  ───────────────                                                  │
│                                                                   │
│  [1] validate                                                     │
│      Hook (pre): log "Validating file: app.py"                    │
│      Command: test -f "app.py" && echo "valid"                    │
│      Timeout: 5s                                                  │
│      → on_success: extract                                        │
│      → on_failure: error                                          │
│                                                                   │
│  [2] extract                                                      │
│      Command: cat "app.py"                                        │
│      Capture: stdout → file_content                               │
│      Timeout: 10s                                                 │
│      → on_success: analyze                                        │
│                                                                   │
│  [PARALLEL] multi_check                                           │
│      Branches: lint, test, audit                                  │
│      Strategy: all_succeed                                        │
│      → on_success: done                                           │
│                                                                   │
│  [LOOP:for_each] process_files                                    │
│      Items: {{inputs.files}}                                      │
│      Body: process_item                                           │
│      → on_complete: done                                          │
│                                                                   │
│  [T] done (success)                                               │
│                                                                   │
│  ✓ No commands will be executed (dry-run mode)                    │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘
```

## Implementation Steps

### 1. Add domain types for dry-run plan
- **File**: `internal/domain/workflow/dry_run.go`
- **Action**: CREATE
- **Changes**:
  - `DryRunPlan` struct: workflow name, inputs, steps plan
  - `DryRunStep` struct: step name, type, resolved command, hooks, transitions, timeout, retry info
  - `DryRunHook` struct: type (pre/post), resolved command
  - `DryRunTransition` struct: condition (if any), target step

### 2. Add DryRunExecutor to application layer
- **File**: `internal/application/dry_run_executor.go`
- **Action**: CREATE
- **Changes**:
  - `DryRunExecutor` struct with dependencies (resolver, evaluator, wfSvc)
  - `Execute(ctx, workflowName, inputs) (*workflow.DryRunPlan, error)` method
  - Walk state machine from initial to terminal states
  - Resolve all interpolations using provided inputs (with mock state outputs)
  - Collect all possible execution paths (no command execution)
  - Handle parallel steps (list branches, strategy)
  - Handle loop steps (show items expression, body steps)
  - Handle conditional transitions (show all conditions and targets)

### 3. Add --dry-run flag to run command
- **File**: `internal/interfaces/cli/run.go`
- **Action**: MODIFY
- **Changes**:
  - Add `dryRunFlag bool` variable
  - Add `cmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Show execution plan without running")`
  - In `RunE`, check if `dryRunFlag` is set and call `runDryRun()` instead of `runWorkflow()`
  - Create `runDryRun(cmd, cfg, workflowName, inputFlags)` function

### 4. Add DryRunFormatter for output
- **File**: `internal/interfaces/cli/ui/dry_run_formatter.go`
- **Action**: CREATE
- **Changes**:
  - `DryRunFormatter` struct with writer and colorizer
  - `Format(plan *workflow.DryRunPlan)` method for text output
  - Step numbering with type indicators: `[1]`, `[PARALLEL]`, `[LOOP:for_each]`, `[T]`
  - Show resolved commands with highlighted interpolations
  - Show hooks with (pre)/(post) labels
  - Show transitions with arrows (→)
  - Add color coding: success paths (green), failure paths (red), conditions (yellow)

### 5. Add JSON output support for dry-run
- **File**: `internal/interfaces/cli/ui/output.go`
- **Action**: MODIFY
- **Changes**:
  - Add `DryRunResult` struct with JSON tags
  - Add `DryRunStepInfo` struct with JSON tags
  - Add `WriteDryRun(result DryRunResult)` method to `OutputWriter`

### 6. Add unit tests for DryRunExecutor
- **File**: `internal/application/dry_run_executor_test.go`
- **Action**: CREATE
- **Changes**:
  - Test linear workflow dry-run
  - Test parallel step dry-run (branches listed)
  - Test loop step dry-run (for_each, while)
  - Test conditional transitions (all paths shown)
  - Test input interpolation resolution
  - Test with missing inputs (should error or show placeholders)
  - Test hooks are included

### 7. Add integration test for CLI
- **File**: `tests/integration/dry_run_test.go`
- **Action**: CREATE
- **Changes**:
  - Test `awf run sample --dry-run` produces expected output
  - Test `awf run sample --dry-run --format=json` produces valid JSON
  - Verify no state files are created
  - Verify no history entries are recorded

## Test Plan

### Unit Tests
- `internal/application/dry_run_executor_test.go`:
  - Linear workflow produces correct step sequence
  - Parallel step shows branches and strategy
  - Loop step shows items/condition and body
  - Conditional transitions all listed
  - Hooks (pre/post, workflow-level) are captured
  - Variable interpolation resolves correctly
  - Missing required inputs return error

### Integration Tests
- `tests/integration/dry_run_test.go`:
  - CLI flag `--dry-run` recognized
  - Output matches expected format (text mode)
  - JSON output is valid and complete
  - No side effects (no state files, no history)
  - Works with `--input` flags

## Files to Modify

| File | Action | Complexity |
|------|--------|------------|
| `internal/domain/workflow/dry_run.go` | CREATE | S |
| `internal/application/dry_run_executor.go` | CREATE | M |
| `internal/interfaces/cli/run.go` | MODIFY | S |
| `internal/interfaces/cli/ui/dry_run_formatter.go` | CREATE | M |
| `internal/interfaces/cli/ui/output.go` | MODIFY | S |
| `internal/application/dry_run_executor_test.go` | CREATE | M |
| `tests/integration/dry_run_test.go` | CREATE | S |

## Risks

| Risk | Mitigation |
|------|------------|
| **Conditional transitions require expression evaluation** | Reuse existing `ExpressionEvaluator` but show "condition: `expr`" instead of evaluating when states.* values are unavailable |
| **Loop items expression may reference runtime data** | Show raw expression `{{inputs.files}}` when can't resolve; only resolve if inputs are provided |
| **Parallel branches may reference non-existent steps** | Leverage existing workflow validation (already checks branch references) |
| **State interpolation `{{states.step.output}}` has no value** | Use placeholder `<pending>` or `<step_name.output>` in dry-run output |
| **Template references need expansion** | Reuse existing `TemplateService.ExpandWorkflow()` before dry-run analysis |

