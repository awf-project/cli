package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite" // SQLite driver for database/sql

	"github.com/awf-project/cli/internal/domain/workflow"
)

const (
	defaultSQLiteLimit = 20
)

// SQLiteHistoryStore implements HistoryStore using SQLite with WAL mode.
// This enables concurrent workflow executions without exclusive directory locks.
type SQLiteHistoryStore struct {
	db        *sql.DB
	closeOnce sync.Once
	closed    bool
	mu        sync.RWMutex
}

// NewSQLiteHistoryStore creates a new SQLite-backed history store.
// The database file will be created at the specified path.
// WAL mode is enabled for concurrent read/write access.
func NewSQLiteHistoryStore(path string) (*SQLiteHistoryStore, error) {
	// Create parent directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create history directory: %w", err)
	}

	// Open SQLite database with parameters embedded in DSN
	// Note: modernc.org/sqlite uses _pragma for setting pragmas via DSN
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Ping and create schema with retry logic.
	// When multiple processes open the same database simultaneously, the initial
	// ping/schema creation can encounter SQLITE_BUSY. Retry handles this case.
	maxRetries := 5
	retryDelay := 100 * time.Millisecond

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
			retryDelay *= 2 // Exponential backoff
		}

		// Ping to ensure connection works
		//nolint:noctx // NewSQLiteHistoryStore doesn't have context parameter
		if err := db.Ping(); err != nil {
			lastErr = fmt.Errorf("ping sqlite: %w", err)
			continue
		}

		// Create schema
		if err := createSchema(db); err != nil {
			lastErr = fmt.Errorf("create schema: %w", err)
			continue
		}

		return &SQLiteHistoryStore{db: db}, nil
	}

	_ = db.Close()
	return nil, lastErr
}

// createSchema creates the execution_records table if it doesn't exist.
func createSchema(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS execution_records (
			id TEXT PRIMARY KEY,
			workflow_id TEXT NOT NULL,
			workflow_name TEXT NOT NULL,
			status TEXT NOT NULL,
			exit_code INTEGER NOT NULL DEFAULT 0,
			started_at INTEGER NOT NULL,
			completed_at INTEGER NOT NULL,
			duration_ms INTEGER NOT NULL DEFAULT 0,
			error_message TEXT NOT NULL DEFAULT ''
		);

		CREATE INDEX IF NOT EXISTS idx_workflow_name ON execution_records(workflow_name);
		CREATE INDEX IF NOT EXISTS idx_status ON execution_records(status);
		CREATE INDEX IF NOT EXISTS idx_completed_at ON execution_records(completed_at);
	`
	//nolint:noctx // createSchema is internal function without context parameter
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("initialize schema: %w", err)
	}
	return nil
}

// Record stores a workflow execution record.
func (s *SQLiteHistoryStore) Record(ctx context.Context, record *workflow.ExecutionRecord) error {
	if record == nil {
		return errors.New("record cannot be nil")
	}

	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return errors.New("store is closed")
	}
	s.mu.RUnlock()

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	query := `
		INSERT INTO execution_records
		(id, workflow_id, workflow_name, status, exit_code, started_at, completed_at, duration_ms, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(
		ctx, query,
		record.ID,
		record.WorkflowID,
		record.WorkflowName,
		record.Status,
		record.ExitCode,
		record.StartedAt.UnixNano(),
		record.CompletedAt.UnixNano(),
		record.DurationMs,
		record.ErrorMessage,
	)
	if err != nil {
		return fmt.Errorf("insert record: %w", err)
	}
	return nil
}

// List retrieves execution records matching the filter criteria.
func (s *SQLiteHistoryStore) List(ctx context.Context, filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, errors.New("store is closed")
	}
	s.mu.RUnlock()

	if filter == nil {
		filter = &workflow.HistoryFilter{}
	}

	limit := filter.Limit
	if limit == 0 {
		limit = defaultSQLiteLimit
	}

	// Build query dynamically
	query := `SELECT id, workflow_id, workflow_name, status, exit_code, started_at, completed_at, duration_ms, error_message
		FROM execution_records WHERE 1=1`
	args := []interface{}{}

	if filter.WorkflowName != "" {
		query += " AND workflow_name = ?"
		args = append(args, filter.WorkflowName)
	}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}

	if !filter.Since.IsZero() {
		query += " AND completed_at >= ?"
		args = append(args, filter.Since.UnixNano())
	}

	if !filter.Until.IsZero() {
		query += " AND completed_at <= ?"
		args = append(args, filter.Until.UnixNano())
	}

	query += " ORDER BY completed_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query records: %w", err)
	}
	defer rows.Close()

	var records []*workflow.ExecutionRecord
	for rows.Next() {
		var r workflow.ExecutionRecord
		var startedAtNs, completedAtNs int64
		err := rows.Scan(
			&r.ID,
			&r.WorkflowID,
			&r.WorkflowName,
			&r.Status,
			&r.ExitCode,
			&startedAtNs,
			&completedAtNs,
			&r.DurationMs,
			&r.ErrorMessage,
		)
		if err != nil {
			return nil, fmt.Errorf("scan record: %w", err)
		}
		r.StartedAt = time.Unix(0, startedAtNs)
		r.CompletedAt = time.Unix(0, completedAtNs)
		records = append(records, &r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate records: %w", err)
	}

	return records, nil
}

// GetStats returns aggregated statistics for executions matching the filter.
func (s *SQLiteHistoryStore) GetStats(ctx context.Context, filter *workflow.HistoryFilter) (*workflow.HistoryStats, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, errors.New("store is closed")
	}
	s.mu.RUnlock()

	if filter == nil {
		filter = &workflow.HistoryFilter{}
	}

	// Build query with filters
	query := `SELECT
		COUNT(*) as total,
		COALESCE(SUM(CASE WHEN status = 'success' OR status = 'completed' THEN 1 ELSE 0 END), 0) as success_count,
		COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0) as failed_count,
		COALESCE(SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END), 0) as cancelled_count,
		COALESCE(AVG(duration_ms), 0) as avg_duration
		FROM execution_records WHERE 1=1`
	args := []interface{}{}

	if filter.WorkflowName != "" {
		query += " AND workflow_name = ?"
		args = append(args, filter.WorkflowName)
	}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}

	if !filter.Since.IsZero() {
		query += " AND completed_at >= ?"
		args = append(args, filter.Since.UnixNano())
	}

	if !filter.Until.IsZero() {
		query += " AND completed_at <= ?"
		args = append(args, filter.Until.UnixNano())
	}

	var stats workflow.HistoryStats
	var avgDuration float64

	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.TotalExecutions,
		&stats.SuccessCount,
		&stats.FailedCount,
		&stats.CancelledCount,
		&avgDuration,
	)
	if err != nil {
		return nil, fmt.Errorf("query stats: %w", err)
	}

	stats.AvgDurationMs = int64(avgDuration)
	return &stats, nil
}

// Cleanup removes execution records older than the specified duration.
// Returns the number of records deleted.
func (s *SQLiteHistoryStore) Cleanup(ctx context.Context, olderThan time.Duration) (int, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return 0, errors.New("store is closed")
	}
	s.mu.RUnlock()

	// Negative duration doesn't make sense for cleanup - return early
	if olderThan < 0 {
		return 0, nil
	}

	cutoff := time.Now().Add(-olderThan)
	cutoffNs := cutoff.UnixNano()

	result, err := s.db.ExecContext(
		ctx,
		"DELETE FROM execution_records WHERE completed_at < ?",
		cutoffNs,
	)
	if err != nil {
		return 0, fmt.Errorf("delete old records: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("get rows affected: %w", err)
	}

	return int(affected), nil
}

// Close gracefully shuts down the SQLite connection.
// Safe to call multiple times.
func (s *SQLiteHistoryStore) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		closeErr = s.db.Close()
	})
	if closeErr != nil {
		return fmt.Errorf("close database: %w", closeErr)
	}
	return nil
}
