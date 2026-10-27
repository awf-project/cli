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
	defaultSQLiteLimit     = 20
	executionRecordColumns = `id, workflow_id, workflow_name, status, exit_code, started_at, completed_at,
		duration_ms, error_message, attempt, resumed_from_id`
)

// ErrHistoryRecordConflict reports reuse of a record ID or workflow attempt
// identity with different persisted data.
var ErrHistoryRecordConflict = errors.New("history record conflict")

const (
	sqliteConstraintPrimaryKey = 1555
	sqliteConstraintUnique     = 2067
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

		if err := migrateLegacyAttemptHistory(db); err != nil {
			lastErr = fmt.Errorf("create schema: %w", err)
			continue
		}

		return &SQLiteHistoryStore{db: db}, nil
	}

	_ = db.Close()
	return nil, lastErr
}

func migrateLegacyAttemptHistory(db *sql.DB) (returnErr error) {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire schema connection: %w", err)
	}
	defer conn.Close()

	if _, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin schema transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			if _, rollbackErr := conn.ExecContext(context.Background(), "ROLLBACK"); rollbackErr != nil {
				returnErr = errors.Join(returnErr, fmt.Errorf("rollback schema transaction: %w", rollbackErr))
			}
		}
	}()

	exists, err := historyTableExists(ctx, conn)
	if err != nil {
		return err
	}
	if !exists {
		if err := createFinalHistorySchema(ctx, conn); err != nil {
			return err
		}
	} else if err := migrateExistingHistorySchema(ctx, conn); err != nil {
		return err
	}

	if err := ensureHistoryIndexes(ctx, conn); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("commit schema transaction: %w", err)
	}
	committed = true
	return nil
}

func historyTableExists(ctx context.Context, conn *sql.Conn) (bool, error) {
	var count int
	err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name = 'execution_records'`).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("inspect history table: %w", err)
	}
	return count == 1, nil
}

func createFinalHistorySchema(ctx context.Context, conn *sql.Conn) error {
	_, err := conn.ExecContext(ctx, `CREATE TABLE execution_records (
		id TEXT PRIMARY KEY,
		workflow_id TEXT NOT NULL,
		workflow_name TEXT NOT NULL,
		status TEXT NOT NULL,
		exit_code INTEGER NOT NULL DEFAULT 0,
		started_at INTEGER NOT NULL,
		completed_at INTEGER NOT NULL,
		duration_ms INTEGER NOT NULL DEFAULT 0,
		error_message TEXT NOT NULL DEFAULT '',
		attempt INTEGER NOT NULL DEFAULT 1,
		resumed_from_id TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		return fmt.Errorf("initialize schema: %w", err)
	}
	return nil
}

func migrateExistingHistorySchema(ctx context.Context, conn *sql.Conn) error {
	hasAttempt, hasResumedFrom, err := inspectLineageColumns(ctx, conn)
	if err != nil {
		return err
	}
	if hasAttempt != hasResumedFrom {
		return errors.New("invalid lineage schema: attempt and resumed_from_id must both be present")
	}

	if !hasAttempt {
		if _, err := conn.ExecContext(ctx,
			"ALTER TABLE execution_records ADD COLUMN attempt INTEGER NOT NULL DEFAULT 0"); err != nil {
			return fmt.Errorf("add attempt column: %w", err)
		}
		if _, err := conn.ExecContext(ctx,
			"ALTER TABLE execution_records ADD COLUMN resumed_from_id TEXT NOT NULL DEFAULT ''"); err != nil {
			return fmt.Errorf("add resumed_from_id column: %w", err)
		}
		if err := backfillLegacyLineage(ctx, conn); err != nil {
			return err
		}
	}

	return validateLineage(ctx, conn)
}

func inspectLineageColumns(ctx context.Context, conn *sql.Conn) (
	hasAttempt bool,
	hasResumedFrom bool,
	returnErr error,
) {
	rows, err := conn.QueryContext(ctx, "PRAGMA table_info(execution_records)")
	if err != nil {
		return false, false, fmt.Errorf("inspect lineage columns: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, false, fmt.Errorf("scan lineage column: %w", err)
		}
		switch name {
		case "attempt":
			hasAttempt = true
		case "resumed_from_id":
			hasResumedFrom = true
		}
	}
	if err := rows.Err(); err != nil {
		return false, false, fmt.Errorf("iterate lineage columns: %w", err)
	}
	return hasAttempt, hasResumedFrom, nil
}

func backfillLegacyLineage(ctx context.Context, conn *sql.Conn) error {
	_, err := conn.ExecContext(ctx, `WITH lineage AS (
		SELECT id,
			ROW_NUMBER() OVER (
				PARTITION BY workflow_id ORDER BY completed_at ASC, started_at ASC, id ASC
			) AS ordinal,
			LAG(id, 1, '') OVER (
				PARTITION BY workflow_id ORDER BY completed_at ASC, started_at ASC, id ASC
			) AS predecessor
		FROM execution_records
	)
	UPDATE execution_records
	SET attempt = (SELECT ordinal FROM lineage WHERE lineage.id = execution_records.id),
		resumed_from_id = (SELECT predecessor FROM lineage WHERE lineage.id = execution_records.id)`)
	if err != nil {
		return fmt.Errorf("backfill lineage: %w", err)
	}
	return nil
}

func validateLineage(ctx context.Context, conn *sql.Conn) error {
	checks := []struct {
		name  string
		query string
	}{
		{"nonpositive attempt", `SELECT id FROM execution_records WHERE attempt <= 0 LIMIT 1`},
		{"duplicate attempt", `SELECT MIN(id) FROM execution_records
			GROUP BY workflow_id, attempt HAVING COUNT(*) > 1 LIMIT 1`},
		{"attempt 1 predecessor", `SELECT id FROM execution_records
			WHERE attempt = 1 AND resumed_from_id <> '' LIMIT 1`},
		{"missing predecessor identifier", `SELECT id FROM execution_records
			WHERE attempt > 1 AND resumed_from_id = '' LIMIT 1`},
		{"self predecessor", `SELECT id FROM execution_records
			WHERE resumed_from_id = id LIMIT 1`},
		{"inconsistent predecessor", `SELECT current.id
			FROM execution_records AS current
			JOIN execution_records AS predecessor ON predecessor.id = current.resumed_from_id
			WHERE predecessor.workflow_id <> current.workflow_id
				OR predecessor.attempt <> current.attempt - 1
			LIMIT 1`},
	}
	for _, check := range checks {
		var id string
		err := conn.QueryRowContext(ctx, check.query).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("validate lineage %s: %w", check.name, err)
		}
		return fmt.Errorf("invalid lineage: %s for record %q", check.name, id)
	}
	return nil
}

func ensureHistoryIndexes(ctx context.Context, conn *sql.Conn) error {
	valid, exists, err := workflowAttemptIndexStatus(ctx, conn)
	if err != nil {
		return err
	}
	if exists && !valid {
		return errors.New("index idx_workflow_attempt does not uniquely cover workflow_id, attempt")
	}

	statements := []string{
		"CREATE INDEX IF NOT EXISTS idx_workflow_name ON execution_records(workflow_name)",
		"CREATE INDEX IF NOT EXISTS idx_status ON execution_records(status)",
		"CREATE INDEX IF NOT EXISTS idx_completed_at ON execution_records(completed_at)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_workflow_attempt ON execution_records(workflow_id, attempt)",
	}
	for _, statement := range statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("create history index: %w", err)
		}
	}
	return nil
}

func workflowAttemptIndexStatus(ctx context.Context, conn *sql.Conn) (
	valid bool,
	exists bool,
	returnErr error,
) {
	rows, err := conn.QueryContext(ctx, "PRAGMA index_list(execution_records)")
	if err != nil {
		return false, false, fmt.Errorf("inspect history indexes: %w", err)
	}
	var unique, partial bool
	for rows.Next() {
		var sequence, uniqueFlag, partialFlag int
		var name, origin string
		scanErr := rows.Scan(&sequence, &name, &uniqueFlag, &origin, &partialFlag)
		if scanErr != nil {
			closeErr := rows.Close()
			return false, false, errors.Join(
				fmt.Errorf("scan history index: %w", scanErr),
				closeErr,
			)
		}
		if name == "idx_workflow_attempt" {
			exists = true
			unique = uniqueFlag == 1
			partial = partialFlag == 1
		}
	}
	closeErr := rows.Close()
	if closeErr != nil {
		return false, false, fmt.Errorf("close history index inspection: %w", closeErr)
	}
	if !exists || !unique || partial {
		return false, exists, nil
	}

	columns, err := conn.QueryContext(ctx, "PRAGMA index_info(idx_workflow_attempt)")
	if err != nil {
		return false, true, fmt.Errorf("inspect idx_workflow_attempt: %w", err)
	}
	defer columns.Close()
	var names []string
	for columns.Next() {
		var sequence, cid int
		var name string
		if err := columns.Scan(&sequence, &cid, &name); err != nil {
			return false, true, fmt.Errorf("scan idx_workflow_attempt: %w", err)
		}
		names = append(names, name)
	}
	if err := columns.Err(); err != nil {
		return false, true, fmt.Errorf("iterate idx_workflow_attempt: %w", err)
	}
	return len(names) == 2 && names[0] == "workflow_id" && names[1] == "attempt", true, nil
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
	if err := validateRecordLineage(record); err != nil {
		return err
	}

	query := `
		INSERT INTO execution_records
		(id, workflow_id, workflow_name, status, exit_code, started_at, completed_at, duration_ms, error_message,
		 attempt, resumed_from_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		record.Attempt,
		record.ResumedFromID,
	)
	if err != nil {
		if isHistoryIdentityConstraint(err) {
			return s.resolveHistoryIdentityConflict(ctx, record)
		}
		return fmt.Errorf("insert record: %w", err)
	}
	return nil
}

func validateRecordLineage(record *workflow.ExecutionRecord) error {
	switch {
	case record.Attempt <= 0:
		return fmt.Errorf("%w: attempt must be positive", workflow.ErrInvalidHistoryAttempt)
	case record.Attempt == 1 && record.ResumedFromID != "":
		return fmt.Errorf("%w: first attempt cannot have a predecessor", workflow.ErrInvalidHistoryAttempt)
	case record.Attempt > 1 && record.ResumedFromID == "":
		return fmt.Errorf("%w: resumed attempt requires a predecessor", workflow.ErrInvalidHistoryAttempt)
	case record.ResumedFromID != "" && record.ResumedFromID == record.ID:
		return fmt.Errorf("%w: attempt cannot resume from itself", workflow.ErrInvalidHistoryAttempt)
	default:
		return nil
	}
}

type sqliteErrorCoder interface {
	Code() int
}

func isHistoryIdentityConstraint(err error) bool {
	var sqliteErr sqliteErrorCoder
	if !errors.As(err, &sqliteErr) {
		return false
	}
	code := sqliteErr.Code()
	return code == sqliteConstraintPrimaryKey || code == sqliteConstraintUnique
}

func (s *SQLiteHistoryStore) resolveHistoryIdentityConflict(
	ctx context.Context,
	record *workflow.ExecutionRecord,
) error {
	persisted, err := s.recordByID(ctx, record.ID)
	if err == nil && executionRecordsEqual(persisted, record) {
		return nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read conflicting history record: %w", err)
	}
	return fmt.Errorf(
		"%w: record_id=%q workflow_id=%q attempt=%d",
		ErrHistoryRecordConflict,
		record.ID,
		record.WorkflowID,
		record.Attempt,
	)
}

func (s *SQLiteHistoryStore) recordByID(ctx context.Context, id string) (*workflow.ExecutionRecord, error) {
	query := "SELECT " + executionRecordColumns + " FROM execution_records WHERE id = ?"
	return scanExecutionRecord(s.db.QueryRowContext(ctx, query, id))
}

type executionRecordScanner interface {
	Scan(dest ...any) error
}

func scanExecutionRecord(scanner executionRecordScanner) (*workflow.ExecutionRecord, error) {
	var record workflow.ExecutionRecord
	var startedAtNs, completedAtNs int64
	err := scanner.Scan(
		&record.ID,
		&record.WorkflowID,
		&record.WorkflowName,
		&record.Status,
		&record.ExitCode,
		&startedAtNs,
		&completedAtNs,
		&record.DurationMs,
		&record.ErrorMessage,
		&record.Attempt,
		&record.ResumedFromID,
	)
	if err != nil {
		return nil, err
	}
	record.StartedAt = time.Unix(0, startedAtNs)
	record.CompletedAt = time.Unix(0, completedAtNs)
	return &record, nil
}

func executionRecordsEqual(left, right *workflow.ExecutionRecord) bool {
	return left.ID == right.ID &&
		left.WorkflowID == right.WorkflowID &&
		left.Attempt == right.Attempt &&
		left.ResumedFromID == right.ResumedFromID &&
		left.WorkflowName == right.WorkflowName &&
		left.Status == right.Status &&
		left.ExitCode == right.ExitCode &&
		left.StartedAt.UnixNano() == right.StartedAt.UnixNano() &&
		left.CompletedAt.UnixNano() == right.CompletedAt.UnixNano() &&
		left.DurationMs == right.DurationMs &&
		left.ErrorMessage == right.ErrorMessage
}

func appendHistoryFilters(
	query string,
	args []interface{},
	filter *workflow.HistoryFilter,
) (filteredQuery string, filteredArgs []interface{}) {
	if filter.WorkflowID != "" {
		query += " AND workflow_id = ?"
		args = append(args, filter.WorkflowID)
	}
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
	return query, args
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
	query := "SELECT " + executionRecordColumns + " FROM execution_records WHERE 1=1"
	args := make([]interface{}, 0, 1)

	query, args = appendHistoryFilters(query, args, filter)

	query += " ORDER BY completed_at DESC, attempt DESC, id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query records: %w", err)
	}
	defer rows.Close()

	var records []*workflow.ExecutionRecord
	for rows.Next() {
		record, err := scanExecutionRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("scan record: %w", err)
		}
		records = append(records, record)
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

	query, args = appendHistoryFilters(query, args, filter)

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
