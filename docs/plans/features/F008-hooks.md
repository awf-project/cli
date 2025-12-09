# F008: Hooks Pre/Post

## Metadata
- **Statut**: backlog
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priorité**: medium
- **Estimation**: M

## Description

Implement hook execution at step and workflow levels. Execute pre-hooks before step commands, post-hooks after. Support workflow-level hooks for start, end, error, and cancel events. Hooks can log messages or run commands.

## Critères d'Acceptance

- [ ] Execute step pre-hooks before step command
- [ ] Execute step post-hooks after step command
- [ ] Execute workflow_start hook at beginning
- [ ] Execute workflow_end hook on success
- [ ] Execute workflow_error hook on failure
- [ ] Execute workflow_cancel hook on SIGINT/SIGTERM
- [ ] Hooks support `log:` and `command:` actions
- [ ] Hook failures don't stop workflow (unless configured)

## Dépendances

- **Bloqué par**: F001, F003, F006, F007
- **Débloque**: _none in MVP_

## Fichiers Impactés

```
internal/domain/workflow/hooks.go
internal/application/hook_executor.go
internal/domain/workflow/step.go
internal/domain/workflow/workflow.go
```

## Tâches Techniques

- [ ] Define Hook struct
  - [ ] Type (log, command)
  - [ ] Value (message or command string)
- [ ] Define HookSet struct
  - [ ] Pre hooks list
  - [ ] Post hooks list
- [ ] Define WorkflowHooks struct
  - [ ] workflow_start
  - [ ] workflow_end
  - [ ] workflow_error
  - [ ] workflow_cancel
- [ ] Implement HookExecutor
  - [ ] ExecuteHooks(hooks []Hook, context) error
  - [ ] Handle log action (write to logger)
  - [ ] Handle command action (shell exec)
  - [ ] Variable interpolation in hooks
- [ ] Integrate hooks into execution flow
  - [ ] Before step execution
  - [ ] After step execution
  - [ ] At workflow lifecycle events
- [ ] Handle hook failures
  - [ ] Log warning but continue by default
  - [ ] Option to fail on hook error
- [ ] Write unit tests
- [ ] Write integration tests

## Notes

Hook execution order:
```
workflow_start → step.pre → step.command → step.post → ... → workflow_end
                                   ↓ (on error)
                            workflow_error
                                   ↓ (on Ctrl-C)
                            workflow_cancel
```

Hook YAML syntax:
```yaml
hooks:
  pre:
    - log: "Starting step..."
    - command: mkdir -p output
  post:
    - log: "Step completed"
```
