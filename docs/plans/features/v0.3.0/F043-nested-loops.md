# F043: Nested Loop Execution

## Metadata
- **Status**: implemented
- **Phase**: 3-Advanced
- **Version**: v0.3.0
- **Priority**: medium
- **Estimation**: M

## Description

Support for nested loop execution where a loop body can contain another loop state. This enables complex iteration patterns where inner loops can iterate over data derived from outer loop iterations. Outer loop context must be preserved during inner loop execution and properly restored after inner loop completes, ensuring variable isolation between loop levels.

## Acceptance Criteria

- [x] A `for_each` or `while` loop can contain another loop state in its body
- [x] Outer loop context variables (`loop.index`, `loop.value`, `loop.first`, `loop.last`) are preserved during inner loop execution
- [x] Inner loop has its own isolated context variables that don't overwrite outer loop context
- [x] After inner loop completes, outer loop context is fully restored
- [x] Nested loops can access outer loop variables via `loop.parent` reference (e.g., `loop.parent.index`)
- [x] Arbitrary nesting depth is supported (loop within loop within loop)
- [x] State persistence correctly tracks nested loop positions for resume capability
- [x] Error in inner loop can be handled independently or propagate to outer loop based on configuration

## Dependencies

- **Blocked by**: F016 (Loops), F042 (Loop Context Variables)
- **Unblocks**: _none_

## Impacted Files

```
internal/domain/workflow/loop.go
internal/domain/workflow/context.go
internal/application/execution_service.go
pkg/interpolation/resolver.go
tests/integration/loop_test.go
tests/fixtures/workflows/loop-nested.yaml
```

## Technical Tasks

- [x] Design loop context stack for nested execution
  - [x] Define context stack data structure
  - [x] Implement push/pop operations for loop context
- [x] Implement `loop.parent` variable resolution
  - [x] Add parent reference to loop context
  - [x] Update interpolation resolver for parent access
- [x] Modify execution service for nested loop handling
  - [x] Detect nested loop states during execution
  - [x] Preserve outer context before inner loop
  - [x] Restore outer context after inner loop
- [x] Update state persistence for nested loops
  - [x] Serialize loop context stack
  - [x] Support resume from nested loop position
- [x] Write unit tests
- [x] Write integration tests (enable pending tests in loop_test.go)
- [x] Update documentation

## Notes

- Integration tests for nested loops already exist in `tests/integration/loop_test.go` but are marked as pending
- Fixture workflow `tests/fixtures/workflows/loop-nested.yaml` is already created
- F042 deferred nested loop context scoping to this feature
- Consider memory implications for deeply nested loops with large datasets
- The `loop.parent` chain should support arbitrary depth (e.g., `loop.parent.parent.index`)
