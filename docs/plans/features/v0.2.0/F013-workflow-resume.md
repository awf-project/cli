# F013: Commande Resume

## Metadata
- **Status**: implemented
- **Phase**: 2-Core
- **Version**: v0.2.0
- **Priority**: high
- **Estimation**: M

## Description

Implement workflow resumption from persisted state. Allow users to continue interrupted workflows from where they left off. Support listing resumable workflows and overriding inputs on resume.

## Acceptance Criteria

- [x] `awf resume <workflow-id>` resumes execution
- [x] `awf resume --list` shows resumable workflows
- [x] Resume from current_state in persisted state
- [x] Reuse outputs from completed states
- [x] Allow input override on resume
- [x] Validate state file exists and is valid
- [x] Clear error if workflow already completed

## Dependencies

- **Blocked by**: F004, F005
- **Unblocks**: _none_

## Impacted Files

```
internal/interfaces/cli/commands/resume.go
internal/application/service.go
internal/infrastructure/store/json_store.go
```

## Technical Tasks

- [x] Implement `resume` command
  - [x] Parse workflow-id argument
  - [x] Parse --list flag
  - [x] Parse input override flags
  - [x] Call WorkflowService.Resume()
- [x] Implement list resumable workflows
  - [x] Filter states with status != completed
  - [x] Show workflow_id, name, current_state, updated_at
- [x] Implement WorkflowService.Resume()
  - [x] Load state from store
  - [x] Validate state is resumable
  - [x] Merge input overrides
  - [x] Continue from current_state
  - [x] Skip completed states
- [x] Handle edge cases
  - [x] State file not found
  - [x] Workflow already completed
  - [x] Workflow definition changed
- [x] Write unit tests
- [x] Write integration tests

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
