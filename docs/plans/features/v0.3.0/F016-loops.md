# F016: Boucles (for/while)

## Metadata
- **Status**: implemented
- **Phase**: 3-Advanced
- **Version**: v0.3.0
- **Priority**: medium
- **Estimation**: L

## Description

Add loop constructs for iterating over lists or until a condition is met. Support for-each iteration over arrays and while loops with conditions. Enable data processing workflows that handle multiple items.

## Acceptance Criteria

- [x] `for_each:` iterates over list
- [x] `while:` repeats until condition false
- [x] Loop variable accessible via `{{loop.item}}`
- [x] Loop index via `{{loop.index}}`
- [x] Max iterations safety limit
- [x] Break condition support
- [x] Collect loop outputs

## Dependencies

- **Blocked by**: F015
- **Unblocks**: _none_

## Impacted Files

```
internal/domain/workflow/loop.go
internal/domain/workflow/state.go
internal/application/executor.go
internal/application/loop_executor.go
```

## Technical Tasks

- [x] Define LoopState struct
  - [x] Type (for_each, while)
  - [x] Items (for for_each)
  - [x] Condition (for while)
  - [x] Body (states to execute)
  - [x] MaxIterations
  - [x] BreakCondition
- [x] Implement ForEachExecutor
  - [x] Parse items from variable or literal
  - [x] Execute body for each item
  - [x] Set loop.item, loop.index
  - [x] Collect outputs
- [x] Implement WhileExecutor
  - [x] Evaluate condition
  - [x] Execute body while true
  - [x] Check max iterations
  - [x] Check break condition
- [x] Add loop context variables
  - [x] loop.item
  - [x] loop.index
  - [x] loop.first
  - [x] loop.last
  - [x] loop.length
- [x] Handle nested loops
- [x] Write unit tests
- [x] Write integration tests

## Notes

For-each syntax:
```yaml
process_files:
  type: for_each
  items: "{{inputs.files}}"  # or literal: ["a.txt", "b.txt"]
  max_iterations: 100
  body:
    - process_single:
        type: step
        command: "process {{loop.item}}"
        capture:
          stdout: "result_{{loop.index}}"
  on_complete: aggregate
```

While syntax:
```yaml
poll_status:
  type: while
  condition: "states.check.output != 'ready'"
  max_iterations: 60
  body:
    - check:
        type: step
        command: "curl -s api/status"
        capture:
          stdout: status
    - wait:
        type: step
        command: "sleep 5"
  on_complete: proceed
```
