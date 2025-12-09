# F004: Persistance d'État JSON

## Metadata
- **Statut**: backlog
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priorité**: high
- **Estimation**: M

## Description

Persist workflow execution state to JSON files. Save state after each step completion for crash recovery. Support atomic writes to prevent corruption. Enable workflow resumption from persisted state.

## Critères d'Acceptance

- [ ] Save state after each step completion
- [ ] Atomic writes (temp file + rename)
- [ ] Load state from file
- [ ] State includes all execution context
- [ ] File locking for concurrent access prevention
- [ ] Implements StateStore port interface

## Dépendances

- **Bloqué par**: F001, F003
- **Débloque**: F013

## Fichiers Impactés

```
internal/infrastructure/store/json_store.go
internal/domain/workflow/state.go
internal/domain/ports/store.go
internal/application/state_manager.go
storage/states/
```

## Tâches Techniques

- [ ] Define State struct
  - [ ] workflow_id, workflow_name, workflow_version
  - [ ] status (pending, running, completed, failed)
  - [ ] current_state
  - [ ] started_at, updated_at, completed_at
  - [ ] inputs map
  - [ ] states map (per-state results)
  - [ ] context (working_dir, user, hostname)
  - [ ] metadata
- [ ] Implement JSONStore
  - [ ] Save(state) with atomic write
  - [ ] Load(workflowID)
  - [ ] Delete(workflowID)
  - [ ] List() for resumable workflows
- [ ] Implement atomic write
  - [ ] Write to temp file
  - [ ] os.Rename for atomic replace
- [ ] Implement file locking
  - [ ] syscall.Flock for exclusive access
- [ ] Implement StateManager in application layer
  - [ ] Checkpoint after each step
  - [ ] Recovery on startup (cleanup temp files)
- [ ] Write unit tests
- [ ] Write integration tests

## Notes

State file location: `storage/states/{workflow-id}.json`

Atomic write pattern:
```go
tmpFile := stateFile + ".tmp"
os.WriteFile(tmpFile, data, 0600)
os.Rename(tmpFile, stateFile) // Atomic on POSIX
```
