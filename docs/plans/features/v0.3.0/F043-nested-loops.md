# F043: Nested Loop Execution

## Metadata
- **Status**: planned
- **Phase**: 3-Advanced
- **Version**: v0.3.0
- **Priority**: medium
- **Estimation**: M

## Description

Support for nested loop execution where a loop body can contain another loop state. This enables complex iteration patterns where inner loops can iterate over data derived from outer loop iterations. Outer loop context must be preserved during inner loop execution and properly restored after inner loop completes, ensuring variable isolation between loop levels.

## Acceptance Criteria

- [ ] A `for_each` or `while` loop can contain another loop state in its body
- [ ] Outer loop context variables (`loop.index`, `loop.value`, `loop.first`, `loop.last`) are preserved during inner loop execution
- [ ] Inner loop has its own isolated context variables that don't overwrite outer loop context
- [ ] After inner loop completes, outer loop context is fully restored
- [ ] Nested loops can access outer loop variables via `loop.parent` reference (e.g., `loop.parent.index`)
- [ ] Arbitrary nesting depth is supported (loop within loop within loop)
- [ ] State persistence correctly tracks nested loop positions for resume capability
- [ ] Error in inner loop can be handled independently or propagate to outer loop based on configuration

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

- [ ] Design loop context stack for nested execution
  - [ ] Define context stack data structure
  - [ ] Implement push/pop operations for loop context
- [ ] Implement `loop.parent` variable resolution
  - [ ] Add parent reference to loop context
  - [ ] Update interpolation resolver for parent access
- [ ] Modify execution service for nested loop handling
  - [ ] Detect nested loop states during execution
  - [ ] Preserve outer context before inner loop
  - [ ] Restore outer context after inner loop
- [ ] Update state persistence for nested loops
  - [ ] Serialize loop context stack
  - [ ] Support resume from nested loop position
- [ ] Write unit tests
- [ ] Write integration tests (enable pending tests in loop_test.go)
- [ ] Update documentation

## Notes

- Integration tests for nested loops already exist in `tests/integration/loop_test.go` but are marked as pending
- Fixture workflow `tests/fixtures/workflows/loop-nested.yaml` is already created
- F042 deferred nested loop context scoping to this feature
- Consider memory implications for deeply nested loops with large datasets
- The `loop.parent` chain should support arbitrary depth (e.g., `loop.parent.parent.index`)
