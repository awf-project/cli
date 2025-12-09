# F003: Exécution Linéaire de Steps

## Metadata
- **Statut**: done
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priorité**: critical
- **Estimation**: M

## Description

Implement basic sequential workflow execution. Execute steps one at a time, following `on_success` transitions until reaching a terminal state. Handle command execution via shell, capture stdout/stderr, and track exit codes.

## Critères d'Acceptance

- [x] Execute shell commands via `/bin/sh -c`
- [x] Follow on_success transitions between states
- [x] Stop at terminal states
- [x] Capture stdout to state output
- [x] Capture stderr separately
- [x] Track exit code per step
- [x] Respect step timeout
- [x] Implements CommandExecutor port interface

## Dépendances

- **Bloqué par**: F001, F002
- **Débloque**: F004, F008, F009

## Fichiers Impactés

```
internal/infrastructure/executor/shell_executor.go      # NEW
internal/infrastructure/executor/shell_executor_test.go # NEW
internal/application/execution_service.go               # NEW
internal/application/execution_service_test.go          # NEW
tests/integration/execution_test.go                     # NEW
internal/domain/ports/executor.go                       # EXISTING (interface)
internal/domain/workflow/context.go                     # EXISTING (entities)
```

## Tâches Techniques

- [x] Implement ShellExecutor
  - [x] Execute command with context
  - [x] Capture stdout/stderr
  - [x] Return exit code
  - [x] Handle timeout via context.WithTimeout
- [x] Implement ExecutionService
  - [x] Run workflow from initial state
  - [x] Execute current state
  - [x] Determine next state from transitions
  - [x] Loop until terminal state
- [x] Reuse CommandResult from ports (no separate OperationResult needed)
  - [x] Output (stdout)
  - [x] Errors (stderr)
  - [x] ExitCode
- [x] Use existing ExecutionContext
  - [x] Current state
  - [x] State outputs map
  - [x] Inputs
  - [x] Metadata
- [x] Handle process groups for clean termination
- [x] Write unit tests with mock executor
- [x] Write integration tests with real shell

## Notes

Use `exec.CommandContext` for timeout support. Set `SysProcAttr.Setpgid = true` to create process group for clean termination.

```go
cmd := exec.CommandContext(ctx, "sh", "-c", command)
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
```
