# Implementation Plan: F014 - BadgerDB History

## Summary

Implement workflow execution history storage using BadgerDB, a pure Go key-value store. The feature records each workflow execution (success/failure/cancelled) with metadata, supports filtering by workflow name/status/date, provides statistics, and auto-cleans entries older than 30 days. This follows hexagonal architecture: domain entities → port interface → infrastructure adapter → application service → CLI command.

## Implementation Steps

### Step 1: Add BadgerDB Dependency

- **File**: `go.mod`
- **Action**: MODIFY
- **Changes**:
  ```bash
  go get github.com/dgraph-io/badger/v4
  ```

---

### Step 2: Create Domain Entities

- **File**: `internal/domain/workflow/execution_record.go`
- **Action**: CREATE
- **Changes**:
  ```go
  package workflow

  import "time"

  // ExecutionRecord represents a completed workflow execution for history.
  type ExecutionRecord struct {
      ID             string
      WorkflowID     string
      WorkflowName   string
      Status         string // success, failed, cancelled
      ExitCode       int
      StartedAt      time.Time
      CompletedAt    time.Time
      DurationMs     int64
      ErrorMessage   string
  }

  // HistoryFilter defines criteria for querying execution history.
  type HistoryFilter struct {
      WorkflowName string
      Status       string
      Since        time.Time
      Until        time.Time
      Limit        int
  }

  // HistoryStats contains aggregated execution statistics.
  type HistoryStats struct {
      TotalExecutions int
      SuccessCount    int
      FailedCount     int
      CancelledCount  int
      AvgDurationMs   int64
  }
  ```

---

### Step 3: Create HistoryStore Port Interface

- **File**: `internal/domain/ports/history.go`
- **Action**: CREATE
- **Changes**:
  ```go
  package ports

  import (
      "context"
      "time"

      "github.com/vanoix/awf/internal/domain/workflow"
  )

  // HistoryStore defines the contract for persisting workflow execution history.
  type HistoryStore interface {
      Record(ctx context.Context, record *workflow.ExecutionRecord) error
      List(ctx context.Context, filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error)
      GetStats(ctx context.Context, filter *workflow.HistoryFilter) (*workflow.HistoryStats, error)
      Cleanup(ctx context.Context, olderThan time.Duration) (int, error)
      Close() error
  }
  ```

---

### Step 4: Implement BadgerHistoryStore Adapter

- **File**: `internal/infrastructure/store/badger_history_store.go`
- **Action**: CREATE
- **Changes**:
  - Implement `HistoryStore` interface
  - Key schema:
    - `exec:{id}` → JSON(ExecutionRecord)
    - `idx:wf:{workflow_name}:{timestamp_ns}:{id}` → empty
    - `idx:st:{status}:{timestamp_ns}:{id}` → empty
    - `idx:ts:{timestamp_ns}:{id}` → empty
  - Use transactions for atomic writes (record + all indexes)
  - Prefix scan for queries with filter intersection
  - Cleanup: iterate `idx:ts:` prefix, delete old records + indexes

```go
package store

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/dgraph-io/badger/v4"
    "github.com/vanoix/awf/internal/domain/workflow"
)

type BadgerHistoryStore struct {
    db *badger.DB
}

func NewBadgerHistoryStore(path string) (*BadgerHistoryStore, error) {
    opts := badger.DefaultOptions(path).
        WithLoggingLevel(badger.WARNING)
    db, err := badger.Open(opts)
    if err != nil {
        return nil, fmt.Errorf("open badger: %w", err)
    }
    return &BadgerHistoryStore{db: db}, nil
}

func (s *BadgerHistoryStore) Record(ctx context.Context, record *workflow.ExecutionRecord) error {
    data, err := json.Marshal(record)
    if err != nil {
        return err
    }
    
    ts := record.CompletedAt.UnixNano()
    return s.db.Update(func(txn *badger.Txn) error {
        // Primary record
        if err := txn.Set([]byte(fmt.Sprintf("exec:%s", record.ID)), data); err != nil {
            return err
        }
        // Indexes (empty values)
        indexes := []string{
            fmt.Sprintf("idx:wf:%s:%020d:%s", record.WorkflowName, ts, record.ID),
            fmt.Sprintf("idx:st:%s:%020d:%s", record.Status, ts, record.ID),
            fmt.Sprintf("idx:ts:%020d:%s", ts, record.ID),
        }
        for _, idx := range indexes {
            if err := txn.Set([]byte(idx), nil); err != nil {
                return err
            }
        }
        return nil
    })
}

// List, GetStats, Cleanup, Close implementations...
```

---

### Step 5: Create HistoryService (Application Layer)

- **File**: `internal/application/history_service.go`
- **Action**: CREATE
- **Changes**:
  ```go
  package application

  import (
      "context"
      "time"

      "github.com/vanoix/awf/internal/domain/ports"
      "github.com/vanoix/awf/internal/domain/workflow"
  )

  type HistoryService struct {
      store  ports.HistoryStore
      logger ports.Logger
  }

  func NewHistoryService(store ports.HistoryStore, logger ports.Logger) *HistoryService {
      return &HistoryService{store: store, logger: logger}
  }

  func (s *HistoryService) Record(ctx context.Context, record *workflow.ExecutionRecord) error {
      return s.store.Record(ctx, record)
  }

  func (s *HistoryService) List(ctx context.Context, filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error) {
      if filter.Limit == 0 {
          filter.Limit = 20 // default
      }
      return s.store.List(ctx, filter)
  }

  func (s *HistoryService) GetStats(ctx context.Context, filter *workflow.HistoryFilter) (*workflow.HistoryStats, error) {
      return s.store.GetStats(ctx, filter)
  }

  func (s *HistoryService) Cleanup(ctx context.Context) (int, error) {
      return s.store.Cleanup(ctx, 30*24*time.Hour) // 30 days
  }

  func (s *HistoryService) Close() error {
      return s.store.Close()
  }
  ```

---

### Step 6: Integrate into ExecutionService

- **File**: `internal/application/execution_service.go`
- **Action**: MODIFY
- **Changes**:

1. Add `historyStore` field to struct:
   ```go
   type ExecutionService struct {
       // ... existing fields
       historyStore ports.HistoryStore
   }
   ```

2. Add to constructor:
   ```go
   func NewExecutionService(
       // ... existing params
       historyStore ports.HistoryStore, // NEW - can be nil
   ) *ExecutionService
   ```

3. Record on workflow completion (in `Run()` method, after terminal state):
   ```go
   // After line ~121 (terminal state) and ~137 (failed state)
   func (s *ExecutionService) recordHistory(execCtx *workflow.ExecutionContext, execErr error) {
       if s.historyStore == nil {
           return
       }
       record := &workflow.ExecutionRecord{
           ID:           execCtx.WorkflowID,
           WorkflowID:   execCtx.WorkflowID,
           WorkflowName: execCtx.WorkflowName,
           Status:       mapStatus(execCtx.Status),
           StartedAt:    execCtx.StartedAt,
           CompletedAt:  time.Now(),
       }
       record.DurationMs = record.CompletedAt.Sub(record.StartedAt).Milliseconds()
       if execErr != nil {
           record.ErrorMessage = execErr.Error()
       }
       // Best effort - don't fail workflow on history error
       if err := s.historyStore.Record(context.Background(), record); err != nil {
           s.logger.Warn("failed to record history", "error", err)
       }
   }
   ```

4. Call `recordHistory()` at these points:
   - Line ~119: After `workflow completed` log
   - Line ~139: After `step failed` + checkpoint
   - Line ~159: After `workflow cancelled` log

---

### Step 7: Implement CLI History Command

- **File**: `internal/interfaces/cli/history.go`
- **Action**: CREATE
- **Changes**:
  ```go
  package cli

  import (
      "context"
      "fmt"
      "time"

      "github.com/spf13/cobra"
      "github.com/vanoix/awf/internal/application"
      "github.com/vanoix/awf/internal/domain/workflow"
      "github.com/vanoix/awf/internal/infrastructure/store"
      "github.com/vanoix/awf/internal/interfaces/cli/ui"
  )

  func newHistoryCommand(cfg *Config) *cobra.Command {
      var (
          workflowName string
          status       string
          since        string
          limit        int
          showStats    bool
      )

      cmd := &cobra.Command{
          Use:   "history",
          Short: "Show workflow execution history",
          Long: `Display past workflow executions with filtering and statistics.

  Examples:
    awf history
    awf history --workflow deploy
    awf history --status failed --since 2025-12-01
    awf history --stats`,
          RunE: func(cmd *cobra.Command, args []string) error {
              return runHistory(cmd, cfg, workflowName, status, since, limit, showStats)
          },
      }

      cmd.Flags().StringVarP(&workflowName, "workflow", "w", "", "Filter by workflow name")
      cmd.Flags().StringVarP(&status, "status", "s", "", "Filter by status (success, failed, cancelled)")
      cmd.Flags().StringVar(&since, "since", "", "Show executions since date (YYYY-MM-DD)")
      cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Maximum entries to show")
      cmd.Flags().BoolVar(&showStats, "stats", false, "Show statistics only")

      return cmd
  }

  func runHistory(cmd *cobra.Command, cfg *Config, workflowName, status, since string, limit int, showStats bool) error {
      ctx := context.Background()
      writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor)

      // Open history store
      historyStore, err := store.NewBadgerHistoryStore(cfg.StoragePath + "/history")
      if err != nil {
          return fmt.Errorf("open history store: %w", err)
      }
      defer historyStore.Close()

      // Build filter
      filter := &workflow.HistoryFilter{
          WorkflowName: workflowName,
          Status:       status,
          Limit:        limit,
      }
      if since != "" {
          t, err := time.Parse("2006-01-02", since)
          if err != nil {
              return fmt.Errorf("invalid --since format: %w", err)
          }
          filter.Since = t
      }

      if showStats {
          stats, err := historyStore.GetStats(ctx, filter)
          if err != nil {
              return err
          }
          return writer.WriteJSON(stats) // or custom stats output
      }

      records, err := historyStore.List(ctx, filter)
      if err != nil {
          return err
      }

      // Output based on format
      return outputHistory(writer, cfg, records)
  }
  ```

---

### Step 8: Register Command in Root

- **File**: `internal/interfaces/cli/root.go`
- **Action**: MODIFY
- **Changes**: Add after line 93:
  ```go
  cmd.AddCommand(newHistoryCommand(cfg))
  ```

---

### Step 9: Add UI Output Support

- **File**: `internal/interfaces/cli/ui/output.go`
- **Action**: MODIFY
- **Changes**: Add `WriteHistory()` and `WriteHistoryStats()` methods to OutputWriter

---

### Step 10: Update .gitignore

- **File**: `.gitignore`
- **Action**: MODIFY
- **Changes**: Add `storage/history/` (already covered by `storage/` if present)

---

## Test Plan

### Unit Tests

| File | Tests |
|------|-------|
| `internal/domain/workflow/execution_record_test.go` | Entity creation, field validation |
| `internal/infrastructure/store/badger_history_store_test.go` | Record, List (with filters), GetStats, Cleanup, concurrent access |
| `internal/application/history_service_test.go` | Service logic, default limit |
| `internal/interfaces/cli/history_test.go` | Command parsing, flag validation |

### Integration Tests

| File | Tests |
|------|-------|
| `tests/integration/history_test.go` | Full flow: run workflow → check history → filter → cleanup |

### Test Fixtures

- Create sample execution records for filter/stats testing

---

## Files to Modify

| File | Action | Complexity | Reason |
|------|--------|------------|--------|
| `go.mod` | MODIFY | S | Add badger/v4 dependency |
| `internal/domain/workflow/execution_record.go` | CREATE | S | Domain entities |
| `internal/domain/ports/history.go` | CREATE | S | Port interface |
| `internal/infrastructure/store/badger_history_store.go` | CREATE | L | BadgerDB adapter with indexing |
| `internal/application/history_service.go` | CREATE | S | Thin service wrapper |
| `internal/application/execution_service.go` | MODIFY | M | Add history recording |
| `internal/interfaces/cli/history.go` | CREATE | M | CLI command with flags |
| `internal/interfaces/cli/root.go` | MODIFY | S | Register command |
| `internal/interfaces/cli/ui/output.go` | MODIFY | S | History output methods |
| `.gitignore` | MODIFY | S | Ensure storage/history covered |

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| BadgerDB not closed on crash | Data corruption | Use `defer Close()`, document recovery |
| Double DB open | Panic | Single instance via dependency injection |
| Slow first startup (cleanup) | UX delay | Run cleanup async or in background |
| Index inconsistency | Query errors | Atomic transactions for record + indexes |
| Large history growth | Disk space | 30-day auto-cleanup, document limits |
| Breaking ExecutionService signature | Existing code | Make historyStore optional (nil check) |

---

## Validation Checklist

- [ ] `make lint` passes
- [ ] `make test-unit` passes
- [ ] `make test-integration` passes
- [ ] `awf history` works with all flags
- [ ] JSON/table/text output formats work
- [ ] Auto-cleanup removes old entries
- [ ] No data loss on normal shutdown

