# F014: BadgerDB History

## Metadata
- **Status**: implemented
- **Phase**: 2-Core
- **Version**: v0.2.0
- **Priority**: medium
- **Estimation**: M

## Description

Store workflow execution history in BadgerDB (pure Go key-value store). Track all executions with metadata for reporting and analysis. Support querying history by workflow name, status, date range. Enable `awf history` command with 30-day auto-cleanup.

## Acceptance Criteria

- [x] Record each workflow execution on completion
- [x] Store workflow_id, name, status, duration, timestamps
- [x] `awf history` lists recent executions (default: 20)
- [x] Filter by workflow name (`--workflow`)
- [x] Filter by status (`--status`: success, failed, interrupted)
- [x] Filter by date range (`--since`)
- [x] Show summary statistics (`--stats`)
- [x] Auto-cleanup entries older than 30 days at startup
- [x] Support output formats (text, json, table)

## Dependencies

- **Blocked by**: F001
- **Unblocks**: _none_

## Impacted Files

```
internal/domain/workflow/execution_record.go
internal/domain/ports/history.go
internal/infrastructure/store/badger_history_store.go
internal/application/history_service.go
internal/application/execution_service.go
internal/interfaces/cli/commands/history.go
storage/history/
go.mod (add github.com/dgraph-io/badger/v4)
```

## Technical Tasks

- [x] Add badger/v4 dependency
  - [x] `go get github.com/dgraph-io/badger/v4`
- [x] Define ExecutionRecord entity
  - [x] ID (string)
  - [x] WorkflowID (string)
  - [x] WorkflowName (string)
  - [x] WorkflowVersion (string, optional)
  - [x] Status (string: success, failed, interrupted)
  - [x] ExitCode (int)
  - [x] StartedAt (time.Time)
  - [x] CompletedAt (time.Time)
  - [x] DurationMs (int64)
  - [x] ErrorMessage (string, optional)
- [x] Define HistoryStore port interface
  - [x] Record(ctx, record) error
  - [x] List(ctx, filter) ([]record, error)
  - [x] GetStats(ctx, filter) (*stats, error)
  - [x] Cleanup(ctx, olderThan) (count, error)
  - [x] Close() error
- [x] Define HistoryFilter struct
  - [x] WorkflowName
  - [x] Status
  - [x] Since / Until
  - [x] Limit
- [x] Define HistoryStats struct
  - [x] TotalExecutions
  - [x] SuccessCount
  - [x] FailedCount
  - [x] AvgDurationMs
- [x] Implement BadgerHistoryStore
  - [x] Open(path) with default options
  - [x] Record() - write record + indexes in transaction
  - [x] List() - prefix scan with filters
  - [x] GetStats() - aggregate statistics
  - [x] Cleanup() - delete old entries + indexes
  - [x] Close() - graceful shutdown
- [x] Create HistoryService
  - [x] Wrap HistoryStore with business logic
  - [x] Auto-cleanup on initialization
- [x] Integrate into ExecutionService
  - [x] Record execution on terminal state
- [x] Implement `history` command
  - [x] --workflow, -w flag
  - [x] --status, -s flag
  - [x] --since flag
  - [x] --limit, -n flag (default: 20)
  - [x] --stats flag
  - [x] Table formatting for text output
- [x] Write unit tests
- [x] Write integration tests

## Notes

### Key Schema

```
Primary:     exec:{id}                               -> JSON(ExecutionRecord)
Index:       idx:wf:{workflow_name}:{timestamp}:{id} -> empty
Index:       idx:st:{status}:{timestamp}:{id}        -> empty
Index:       idx:ts:{timestamp}:{id}                 -> empty
```

### Query Strategy

- By workflow: prefix scan `idx:wf:{name}:`
- By status: prefix scan `idx:st:{status}:`
- By date range: prefix scan `idx:ts:` with timestamp bounds
- Combined filters: intersect results from multiple index scans

### Why BadgerDB over SQLite

- **Pure Go**: No CGO required, simpler cross-compilation
- **Embedded**: No external dependencies, single directory storage
- **Performance**: Optimized for append-heavy workloads (LSM-tree)
- **Simple API**: Key-value with prefix scanning

### Configuration

- **Storage path**: `storage/history/` (BadgerDB creates multiple files)
- **Retention**: 30 days auto-cleanup at startup
- **Default limit**: 20 entries for `awf history`

### Example Usage

```bash
# List recent executions
awf history

# Filter by workflow
awf history --workflow deploy

# Filter by status
awf history --status failed

# Show executions since date
awf history --since 2025-12-01

# Show statistics only
awf history --stats

# JSON output for scripting
awf history -f json

# Combined filters
awf history -w deploy -s success --since 2025-12-01 -n 50
```
