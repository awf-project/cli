// Package store provides infrastructure adapters for state persistence and execution history.
//
// This package implements StateStore and HistoryStore ports from the domain layer,
// providing durable storage for workflow state and execution records:
//   - JSONStore: Atomic JSON file persistence for workflow execution state
//   - SQLiteHistoryStore: SQLite-based execution history with WAL mode for concurrent access
//
// Architecture:
//   - Domain defines: StateStore and HistoryStore port interfaces
//   - Infrastructure provides: JSONStore (state), SQLiteHistoryStore (history)
//   - Application injects: Store implementations via dependency injection
//
// JSONStore Example:
//
//	store := store.NewJSONStore(basePath)
//	err := store.Save(ctx, executionContext)
//	if err != nil {
//	    // Handle save error
//	}
//	// State persisted atomically to JSON file
//
// SQLiteHistoryStore Example:
//
//	histStore, err := store.NewSQLiteHistoryStore(dbPath)
//	if err != nil {
//	    // Handle init error
//	}
//	defer histStore.Close()
//	err = histStore.Record(ctx, executionRecord)
//	// Execution history persisted to SQLite
//
// State Persistence (JSONStore):
//   - Atomic writes via temp file + rename pattern
//   - File locking for concurrent access prevention
//   - One JSON file per workflow execution (workflowID.json)
//   - ExecutionContext serialization with full state tree
//
// History Persistence (SQLiteHistoryStore):
//   - SQLite WAL mode enables concurrent reads/writes
//   - Execution records include: workflow ID, status, duration, timestamps
//   - Query filtering by status, time range, workflow name
//   - Statistics aggregation and cleanup for old records
//   - Thread-safe with internal mutex protection
//
// Component: C056 Infrastructure Package Documentation
// Layer: Infrastructure
package store
