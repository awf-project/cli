# F003: Exécution Linéaire de Steps

## Metadata
- **Statut**: backlog
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priorité**: critical
- **Estimation**: M

## Description

Implement basic sequential workflow execution. Execute steps one at a time, following `on_success` transitions until reaching a terminal state. Handle command execution via shell, capture stdout/stderr, and track exit codes.

## Critères d'Acceptance

- [ ] Execute shell commands via `/bin/sh -c`
- [ ] Follow on_success transitions between states
- [ ] Stop at terminal states
- [ ] Capture stdout to state output
- [ ] Capture stderr separately
- [ ] Track exit code per step
- [ ] Respect step timeout
- [ ] Implements CommandExecutor port interface

## Dépendances

- **Bloqué par**: F001, F002
- **Débloque**: F004, F008, F009

## Fichiers Impactés

```
internal/infrastructure/executor/shell_executor.go
internal/application/executor.go
internal/domain/workflow/state.go
internal/domain/workflow/context.go
internal/domain/operation/result.go
internal/domain/ports/executor.go
```

## Tâches Techniques

- [ ] Implement ShellExecutor
  - [ ] Execute command with context
  - [ ] Capture stdout/stderr
  - [ ] Return exit code
  - [ ] Handle timeout via context.WithTimeout
- [ ] Implement ExecutionService
  - [ ] Run workflow from initial state
  - [ ] Execute current state
  - [ ] Determine next state from transitions
  - [ ] Loop until terminal state
- [ ] Define OperationResult
  - [ ] Output (stdout)
  - [ ] Errors (stderr)
  - [ ] ExitCode
  - [ ] Duration
- [ ] Define ExecutionContext
  - [ ] Current state
  - [ ] State outputs map
  - [ ] Inputs
  - [ ] Metadata
- [ ] Handle process groups for clean termination
- [ ] Write unit tests with mock executor
- [ ] Write integration tests with real shell

## Notes

Use `exec.CommandContext` for timeout support. Set `SysProcAttr.Setpgid = true` to create process group for clean termination.

```go
cmd := exec.CommandContext(ctx, "sh", "-c", command)
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
```
