# F013: Commande Resume

## Metadata
- **Statut**: backlog
- **Phase**: 2-Core
- **Version**: v0.2.0
- **Priorité**: high
- **Estimation**: M

## Description

Implement workflow resumption from persisted state. Allow users to continue interrupted workflows from where they left off. Support listing resumable workflows and overriding inputs on resume.

## Critères d'Acceptance

- [ ] `awf resume <workflow-id>` resumes execution
- [ ] `awf resume --list` shows resumable workflows
- [ ] Resume from current_state in persisted state
- [ ] Reuse outputs from completed states
- [ ] Allow input override on resume
- [ ] Validate state file exists and is valid
- [ ] Clear error if workflow already completed

## Dépendances

- **Bloqué par**: F004, F005
- **Débloque**: _none_

## Fichiers Impactés

```
internal/interfaces/cli/commands/resume.go
internal/application/service.go
internal/infrastructure/store/json_store.go
```

## Tâches Techniques

- [ ] Implement `resume` command
  - [ ] Parse workflow-id argument
  - [ ] Parse --list flag
  - [ ] Parse input override flags
  - [ ] Call WorkflowService.Resume()
- [ ] Implement list resumable workflows
  - [ ] Filter states with status != completed
  - [ ] Show workflow_id, name, current_state, updated_at
- [ ] Implement WorkflowService.Resume()
  - [ ] Load state from store
  - [ ] Validate state is resumable
  - [ ] Merge input overrides
  - [ ] Continue from current_state
  - [ ] Skip completed states
- [ ] Handle edge cases
  - [ ] State file not found
  - [ ] Workflow already completed
  - [ ] Workflow definition changed
- [ ] Write unit tests
- [ ] Write integration tests

## Notes

Resume command usage:
```bash
# List resumable workflows
awf resume --list

# Resume specific workflow
awf resume analyze-code-20231209-143022

# Resume with input override
awf resume analyze-code-20231209-143022 --max-tokens=5000
```

State file must include:
- current_state
- completed states with outputs
- original inputs
