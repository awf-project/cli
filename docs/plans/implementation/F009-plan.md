Now I have a complete picture. Let me create the implementation plan:

# Implementation Plan: F009 - State Machine Validation

## Summary

F009 adds full state machine validation with graph analysis to detect cycles, unreachable states, and ensure proper state transitions. Most of the execution logic already exists (`on_success`/`on_failure` transitions work), but validation is incomplete. The implementation focuses on: (1) adding `TerminalStatus` enum to the domain, (2) implementing graph validation algorithms (cycle detection, reachability), (3) handling `continue_on_error` flag correctly, and (4) extracting a `StateMachine` abstraction for cleaner code.

```
┌────────────────────────────────────────────────────────────────┐
│                     STATE MACHINE FLOW                         │
├────────────────────────────────────────────────────────────────┤
│                                                                │
│   ┌─────────┐  exit=0   ┌─────────┐  exit=0   ┌──────────┐   │
│   │ initial │──────────▶│  step   │──────────▶│ terminal │   │
│   └─────────┘           └─────────┘           │ (success)│   │
│                              │                └──────────┘   │
│                              │ exit≠0                         │
│                              ▼                                │
│                        ┌──────────┐                           │
│                        │ terminal │                           │
│                        │ (failure)│                           │
│                        └──────────┘                           │
│                                                                │
└────────────────────────────────────────────────────────────────┘
```

## Implementation Steps

### 1. Add `TerminalStatus` to Domain
   - **File**: `internal/domain/workflow/step.go`
   - **Action**: MODIFY
   - **Changes**:
     - Add `TerminalStatus` type with constants `TerminalSuccess`, `TerminalFailure`
     - Add `Status` field to `Step` struct for terminal steps
     - Update `Step.Validate()` to validate terminal status values

### 2. Create State Machine Graph Validator
   - **File**: `internal/domain/workflow/graph.go`
   - **Action**: CREATE
   - **Changes**:
     - Implement `ValidateGraph(steps map[string]*Step, initial string) []ValidationError`
     - DFS for reachability check (detect orphan states)
     - DFS with color marking for cycle detection (warn, not error)
     - Validate all `on_success`/`on_failure` references exist
     - Return structured errors/warnings

### 3. Integrate Graph Validation into Workflow.Validate()
   - **File**: `internal/domain/workflow/workflow.go`
   - **Action**: MODIFY
   - **Changes**:
     - Call `ValidateGraph()` from `Workflow.Validate()`
     - Aggregate errors using `errors.Join()`
     - Add warnings to return value (cycles are warnings)

### 4. Handle `continue_on_error` in Execution
   - **File**: `internal/application/execution_service.go`
   - **Action**: MODIFY
   - **Changes**:
     - When `step.ContinueOnError == true`, always follow `on_success` regardless of exit code
     - Update state recording to reflect this behavior
     - Current code already has the field but doesn't use it

### 5. Update YAML Mapper for Terminal Status
   - **File**: `internal/infrastructure/repository/yaml_mapper.go`
   - **Action**: MODIFY
   - **Changes**:
     - Map `status` field from YAML to `Step.Status`
     - `yamlStep` already has `Status string` field

### 6. Add Validation Result Types
   - **File**: `internal/domain/workflow/validation_errors.go`
   - **Action**: CREATE
   - **Changes**:
     - Define `ValidationError` struct with `Level` (error/warning), `Code`, `Message`, `Path`
     - Define error codes: `ErrCycleDetected`, `ErrUnreachableState`, `ErrInvalidTransition`
     - Implement `Error()` interface

## Test Plan

### Unit Tests
- `internal/domain/workflow/graph_test.go`:
  - Test cycle detection (simple cycle, no cycle, self-loop)
  - Test reachability (all reachable, orphan states)
  - Test transition validation (valid refs, invalid refs)
  
- `internal/domain/workflow/step_test.go`:
  - Test `TerminalStatus` validation
  - Test terminal step with valid/invalid status

- `internal/application/execution_service_test.go`:
  - Test `continue_on_error`: step fails but follows `on_success`
  - Test normal failure follows `on_failure`

### Integration Tests
- `tests/integration/validation_test.go`:
  - Load workflow with cycle, verify warning
  - Load workflow with orphan state, verify error
  - Load workflow with invalid transition, verify error

## Files to Modify

| File | Action | Complexity |
|------|--------|------------|
| `internal/domain/workflow/step.go` | MODIFY | S |
| `internal/domain/workflow/graph.go` | CREATE | M |
| `internal/domain/workflow/validation_errors.go` | CREATE | S |
| `internal/domain/workflow/graph_test.go` | CREATE | M |
| `internal/domain/workflow/workflow.go` | MODIFY | S |
| `internal/application/execution_service.go` | MODIFY | S |
| `internal/infrastructure/repository/yaml_mapper.go` | MODIFY | S |
| `tests/integration/validation_test.go` | CREATE | M |

## Risks

1. **Cycle detection edge cases**: Cycles involving parallel branches need careful handling. Parallel execution (F010) depends on F009, so design the graph traversal to support `type: parallel` with `branches` array.
   - *Mitigation*: Treat parallel branches as multiple outbound edges in the graph.

2. **Breaking change in validation output**: Existing workflows that have cycles will now produce warnings. Users may be surprised.
   - *Mitigation*: Cycles are warnings, not errors. Document in changelog.

3. **Error aggregation complexity**: `errors.Join()` produces multi-line output. The CLI error formatting may need adjustment.
   - *Mitigation*: Use structured `ValidationError` slice, let CLI format appropriately.

4. **`continue_on_error` semantic ambiguity**: Does it mean "always succeed" or "always go to on_success"? The spec says "always go to on_success".
   - *Mitigation*: Follow spec literally. Document behavior clearly in YAML syntax section.

