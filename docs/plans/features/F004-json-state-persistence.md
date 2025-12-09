# F004: Persistance d'État JSON

## Metadata
- **Statut**: done
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priorité**: high
- **Estimation**: M

## Description

Persist workflow execution state to JSON files. Save state after each step completion for crash recovery. Support atomic writes to prevent corruption. Enable workflow resumption from persisted state.

## Critères d'Acceptance

- [x] Save state after each step completion
- [x] Atomic writes (temp file + rename)
- [x] Load state from file
- [x] State includes all execution context
- [x] File locking for concurrent access prevention
- [x] Implements StateStore port interface

## Dépendances

- **Bloqué par**: F001, F003
- **Débloque**: F013

## Fichiers Impactés

```
internal/infrastructure/store/json_store.go      # NEW
internal/infrastructure/store/json_store_test.go # NEW
internal/application/execution_service.go        # MODIFIED (checkpoint)
internal/domain/ports/store.go                   # EXISTING (interface)
storage/states/
```

## Tâches Techniques

- [x] Define State struct (using existing ExecutionContext)
  - [x] workflow_id, workflow_name, workflow_version
  - [x] status (pending, running, completed, failed)
  - [x] current_state
  - [x] started_at, updated_at, completed_at
  - [x] inputs map
  - [x] states map (per-state results)
- [x] Implement JSONStore
  - [x] Save(state) with atomic write
  - [x] Load(workflowID)
  - [x] Delete(workflowID)
  - [x] List() for resumable workflows
- [x] Implement atomic write
  - [x] Write to temp file
  - [x] os.Rename for atomic replace
- [x] Implement file locking
  - [x] syscall.Flock for exclusive access
- [x] Implement checkpoint in ExecutionService
  - [x] Checkpoint after each step
  - [x] Checkpoint on completion/failure
- [x] Write unit tests (14 tests)
- [x] Write integration tests (1 checkpoint test)

## Notes

State file location: `storage/states/{workflow-id}.json`

Atomic write pattern:
```go
tmpFile := stateFile + ".tmp"
os.WriteFile(tmpFile, data, 0600)
os.Rename(tmpFile, stateFile) // Atomic on POSIX
```

## Implementation Summary

- `JSONStore` implements `ports.StateStore` interface
- Atomic writes via temp file + `os.Rename`
- File locking via `syscall.Flock(LOCK_EX)`
- `ExecutionService.checkpoint()` saves state after each step
- 14 unit tests + 1 integration test (all passing)
