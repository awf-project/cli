# F009: State Machine avec Transitions

## Metadata
- **Statut**: backlog
- **Phase**: 2-Core
- **Version**: v0.2.0
- **Priorité**: critical
- **Estimation**: L

## Description

Implement full state machine logic with conditional transitions. Support `on_success` and `on_failure` transitions, terminal states, and state validation. Detect cycles and unreachable states at validation time.

## Critères d'Acceptance

- [ ] Follow on_success transition on exit code 0
- [ ] Follow on_failure transition on non-zero exit
- [ ] Stop execution at terminal states
- [ ] Detect cycles during validation
- [ ] Detect unreachable states during validation
- [ ] Require at least one terminal state
- [ ] Support continue_on_error flag

## Dépendances

- **Bloqué par**: F003
- **Débloque**: F010, F015

## Fichiers Impactés

```
internal/domain/workflow/state_machine.go
internal/domain/workflow/validator.go
internal/application/executor.go
```

## Tâches Techniques

- [ ] Define StateType enum
  - [ ] step
  - [ ] parallel
  - [ ] terminal
- [ ] Define TerminalStatus enum
  - [ ] success
  - [ ] failure
- [ ] Implement StateMachine
  - [ ] GetInitialState()
  - [ ] GetNextState(current, result)
  - [ ] IsTerminal(state)
  - [ ] GetTerminalStatus(state)
- [ ] Implement state graph validation
  - [ ] All referenced states exist
  - [ ] No orphan states (all reachable from initial)
  - [ ] At least one terminal state
  - [ ] Cycle detection (warning, not error)
- [ ] Update ExecutionService for transitions
  - [ ] Evaluate exit code
  - [ ] Select on_success or on_failure
  - [ ] Handle continue_on_error
- [ ] Write unit tests for state machine logic
- [ ] Write validation tests

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
