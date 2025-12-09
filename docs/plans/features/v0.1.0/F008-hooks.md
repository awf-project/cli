# F008: Hooks Pre/Post

## Metadata
- **Statut**: done
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priorité**: medium
- **Estimation**: M

## Description

Implement hook execution at step and workflow levels. Execute pre-hooks before step commands, post-hooks after. Support workflow-level hooks for start, end, error, and cancel events. Hooks can log messages or run commands.

## Critères d'Acceptance

- [x] Execute step pre-hooks before step command
- [x] Execute step post-hooks after step command
- [x] Execute workflow_start hook at beginning
- [x] Execute workflow_end hook on success
- [x] Execute workflow_error hook on failure
- [x] Execute workflow_cancel hook on SIGINT/SIGTERM
- [x] Hooks support `log:` and `command:` actions
- [x] Hook failures don't stop workflow (unless configured)

## Dépendances

- **Bloqué par**: F001, F003, F006, F007
- **Débloque**: _none in MVP_

## Fichiers Impactés

```
internal/domain/workflow/hooks.go          # Hook domain models
internal/application/hook_executor.go      # HookExecutor service
internal/application/execution_service.go  # Integration into workflow execution
internal/domain/workflow/step.go           # Step.Hooks field
internal/domain/workflow/workflow.go       # Workflow.Hooks field
```

## Tâches Techniques

- [x] Define Hook struct
  - [x] Type (log, command)
  - [x] Value (message or command string)
- [x] Define HookSet struct
  - [x] Pre hooks list
  - [x] Post hooks list
- [x] Define WorkflowHooks struct
  - [x] workflow_start
  - [x] workflow_end
  - [x] workflow_error
  - [x] workflow_cancel
- [x] Implement HookExecutor
  - [x] ExecuteHooks(hooks []Hook, context) error
  - [x] Handle log action (write to logger)
  - [x] Handle command action (shell exec)
  - [x] Variable interpolation in hooks
- [x] Integrate hooks into execution flow
  - [x] Before step execution
  - [x] After step execution
  - [x] At workflow lifecycle events
- [x] Handle hook failures
  - [x] Log warning but continue by default
  - [x] Option to fail on hook error
- [x] Write unit tests
- [x] Write integration tests

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
