# Implementation Plan: F039 - Run Single Step

## Summary

Add a `--step` flag to the `awf run` command that allows executing a single step from a workflow in isolation. This requires modifying the CLI to accept the new flag and adding execution logic that bypasses the state machine for direct step execution.

## Implementation Steps

### 1. Add CLI Flag
- **File**: `internal/interfaces/cli/run.go`
- **Action**: MODIFY
- **Changes**:
  - Add `--step` string flag to run command
  - Add `--mock` string slice flag for state injection
  - Parse mock values into map[string]string

### 2. Add GetStep Method (if missing)
- **File**: `internal/domain/workflow/workflow.go`
- **Action**: MODIFY
- **Changes**:
  - Add `GetStep(name string) (*State, error)` method
  - Return error if step not found

### 3. Implement Single Step Execution
- **File**: `internal/application/execution_service.go`
- **Action**: MODIFY
- **Changes**:
  - Add `ExecuteSingleStep(ctx, workflow, stepName, inputs, mocks) (StepResult, error)`
  - Build execution context with mocked state values
  - Execute step command via executor
  - Run pre/post hooks
  - Return captured output

### 4. Wire Up in CLI
- **File**: `internal/interfaces/cli/run.go`
- **Action**: MODIFY
- **Changes**:
  - Check if `--step` flag is set
  - If set, call `ExecuteSingleStep` instead of `Execute`
  - Parse `--mock` values and pass to execution

## Test Plan

### Unit Tests
- `TestRunCommand_StepFlag` - Flag parsing
- `TestExecuteSingleStep_Success` - Normal execution
- `TestExecuteSingleStep_NotFound` - Invalid step name
- `TestExecuteSingleStep_WithMocks` - State injection
- `TestExecuteSingleStep_Hooks` - Pre/post hooks run

### Integration Tests
- Execute single step from real workflow
- Verify mock injection works with interpolation

## Files to Modify

| File | Action | Complexity |
|------|--------|------------|
| internal/interfaces/cli/run.go | MODIFY | M |
| internal/application/execution_service.go | MODIFY | M |
| internal/domain/workflow/workflow.go | MODIFY | S |
| internal/interfaces/cli/run_test.go | MODIFY | S |
| internal/application/execution_service_test.go | CREATE | M |

## Risks

- **Mock value parsing complexity**: Keep format simple (`key=value`)
- **Interpolation with mocked states**: Ensure template engine handles mock data
- **Hook execution context**: Hooks may expect full workflow context - handle gracefully
