# F009: State Machine avec Transitions

## Metadata
- **Status**: implemented
- **Phase**: 2-Core
- **Version**: v0.2.0
- **Priority**: critical
- **Estimation**: L

## Description

Implement full state machine logic with conditional transitions. Support `on_success` and `on_failure` transitions, terminal states, and state validation. Detect cycles and unreachable states at validation time.

## Acceptance Criteria

- [x] Follow on_success transition on exit code 0
- [x] Follow on_failure transition on non-zero exit
- [x] Stop execution at terminal states
- [x] Detect cycles during validation
- [x] Detect unreachable states during validation
- [x] Require at least one terminal state
- [x] Support continue_on_error flag

## Dependencies

- **Blocked by**: F003
- **Unblocks**: F010, F015

## Impacted Files

```
internal/domain/workflow/state_machine.go
internal/domain/workflow/validator.go
internal/application/executor.go
```

## Technical Tasks

- [x] Define StateType enum
  - [x] step
  - [x] parallel
  - [x] terminal
- [x] Define TerminalStatus enum
  - [x] success
  - [x] failure
- [x] Implement StateMachine
  - [x] GetInitialState()
  - [x] GetNextState(current, result)
  - [x] IsTerminal(state)
  - [x] GetTerminalStatus(state)
- [x] Implement state graph validation
  - [x] All referenced states exist
  - [x] No orphan states (all reachable from initial)
  - [x] At least one terminal state
  - [x] Cycle detection (warning, not error)
- [x] Update ExecutionService for transitions
  - [x] Evaluate exit code
  - [x] Select on_success or on_failure
  - [x] Handle continue_on_error
- [x] Write unit tests for state machine logic
- [x] Write validation tests

## Notes

State transitions:
```yaml
states:
  initial: validate
  validate:
    type: step
    command: test -f file.txt
    on_success: process    # exit 0
    on_failure: error      # exit != 0
  process:
    type: step
    command: process.sh
    continue_on_error: true  # always go to on_success
    on_success: done
  done:
    type: terminal
    status: success
  error:
    type: terminal
    status: failure
```
