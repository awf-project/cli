# F014: Historique SQLite

## Metadata
- **Statut**: backlog
- **Phase**: 2-Core
- **Version**: v0.2.0
- **Priorité**: medium
- **Estimation**: M

## Description

Store workflow execution history in SQLite database. Track all executions with metadata for reporting and analysis. Support querying history by workflow name, status, date range. Enable `awf history` command.

## Critères d'Acceptance

- [ ] Record each workflow execution
- [ ] Store workflow_id, name, status, duration, timestamps
- [ ] `awf history` lists recent executions
- [ ] Filter by workflow name
- [ ] Filter by status (success, failed)
- [ ] Filter by date range
- [ ] Show summary statistics
- [ ] Configurable retention period

## Dépendances

- **Bloqué par**: F001
- **Débloque**: _none_

## Fichiers Impactés

```
internal/infrastructure/store/sqlite_store.go
internal/interfaces/cli/commands/history.go
internal/domain/ports/history.go
storage/history.db
go.mod (add sqlite3)
```

## Tâches Techniques

- [ ] Add sqlite3 dependency
  - [ ] `go get github.com/mattn/go-sqlite3`
- [ ] Define HistoryStore port interface
  - [ ] Record(execution)
  - [ ] List(filters)
  - [ ] GetStats()
  - [ ] Cleanup(olderThan)
- [ ] Define ExecutionRecord struct
  - [ ] ID
  - [ ] WorkflowID
  - [ ] WorkflowName
  - [ ] WorkflowVersion
  - [ ] Status
  - [ ] ExitCode
  - [ ] StartedAt
  - [ ] CompletedAt
  - [ ] Duration
  - [ ] ErrorMessage
- [ ] Implement SQLiteHistoryStore
  - [ ] Initialize schema
  - [ ] CRUD operations
  - [ ] Query with filters
  - [ ] Aggregate stats
- [ ] Implement `history` command
  - [ ] --limit flag
  - [ ] --workflow flag
  - [ ] --status flag
  - [ ] --since flag
  - [ ] Table formatting
- [ ] Implement auto-cleanup based on retention
- [ ] Write unit tests
- [ ] Write integration tests

## Notes

Schema:
```sql
CREATE TABLE executions (
    id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL,
    workflow_name TEXT NOT NULL,
    workflow_version TEXT,
    status TEXT NOT NULL,
    exit_code INTEGER,
    started_at DATETIME NOT NULL,
    completed_at DATETIME,
    duration_ms INTEGER,
    error_message TEXT
);

CREATE INDEX idx_workflow_name ON executions(workflow_name);
CREATE INDEX idx_status ON executions(status);
CREATE INDEX idx_started_at ON executions(started_at);
```

Note: sqlite3 requires CGO. Consider BoltDB/BadgerDB for pure Go alternative.
