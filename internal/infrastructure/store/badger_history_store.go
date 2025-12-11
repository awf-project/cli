package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/vanoix/awf/internal/domain/workflow"
)

const (
	prefixExec   = "exec:"   // Primary record: exec:{id} -> JSON(ExecutionRecord)
	prefixWf     = "idx:wf:" // Index by workflow: idx:wf:{workflow_name}:{timestamp_ns}:{id}
	prefixSt     = "idx:st:" // Index by status: idx:st:{status}:{timestamp_ns}:{id}
	prefixTs     = "idx:ts:" // Index by timestamp: idx:ts:{timestamp_ns}:{id}
	defaultLimit = 20
)

// BadgerHistoryStore implements HistoryStore using BadgerDB.
type BadgerHistoryStore struct {
	db        *badger.DB
	closeOnce sync.Once
	closeErr  error
}

// NewBadgerHistoryStore creates a new BadgerDB-backed history store.
func NewBadgerHistoryStore(path string) (*BadgerHistoryStore, error) {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("create history directory: %w", err)
	}

	opts := badger.DefaultOptions(path).
		WithLoggingLevel(badger.WARNING)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open badger: %w", err)
	}

	return &BadgerHistoryStore{db: db}, nil
}

// Record stores a workflow execution record with indexes.
func (s *BadgerHistoryStore) Record(ctx context.Context, record *workflow.ExecutionRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}

	ts := record.CompletedAt.UnixNano()

	return s.db.Update(func(txn *badger.Txn) error {
		// Primary record
		primaryKey := fmt.Sprintf("%s%s", prefixExec, record.ID)
		if err := txn.Set([]byte(primaryKey), data); err != nil {
			return fmt.Errorf("store record: %w", err)
		}

		// Index by workflow name
		wfKey := fmt.Sprintf("%s%s:%020d:%s", prefixWf, record.WorkflowName, ts, record.ID)
		if err := txn.Set([]byte(wfKey), nil); err != nil {
			return fmt.Errorf("store workflow index: %w", err)
		}

		// Index by status
		stKey := fmt.Sprintf("%s%s:%020d:%s", prefixSt, record.Status, ts, record.ID)
		if err := txn.Set([]byte(stKey), nil); err != nil {
			return fmt.Errorf("store status index: %w", err)
		}

		// Index by timestamp
		tsKey := fmt.Sprintf("%s%020d:%s", prefixTs, ts, record.ID)
		if err := txn.Set([]byte(tsKey), nil); err != nil {
			return fmt.Errorf("store timestamp index: %w", err)
		}

		return nil
	})
}

// List retrieves execution records matching the filter criteria.
func (s *BadgerHistoryStore) List(ctx context.Context, filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error) {
	if filter == nil {
		filter = &workflow.HistoryFilter{}
	}

	limit := filter.Limit
	if limit == 0 {
		limit = defaultLimit
	}

	// Collect matching record IDs from indexes
	var recordIDs []string
	var err error

	// Choose the most selective index to scan
	if filter.WorkflowName != "" {
		recordIDs, err = s.scanWorkflowIndex(filter.WorkflowName, filter.Since, filter.Until)
	} else if filter.Status != "" {
		recordIDs, err = s.scanStatusIndex(filter.Status, filter.Since, filter.Until)
	} else {
		recordIDs, err = s.scanTimestampIndex(filter.Since, filter.Until)
	}
	if err != nil {
		return nil, err
	}

	// Fetch and filter records
	records := make([]*workflow.ExecutionRecord, 0, len(recordIDs))
	err = s.db.View(func(txn *badger.Txn) error {
		for _, id := range recordIDs {
			record, err := s.getRecord(txn, id)
			if err != nil {
				continue // Skip missing records
			}

			// Apply remaining filters
			if filter.WorkflowName != "" && record.WorkflowName != filter.WorkflowName {
				continue
			}
			if filter.Status != "" && record.Status != filter.Status {
				continue
			}
			if !filter.Since.IsZero() && record.CompletedAt.Before(filter.Since) {
				continue
			}
			if !filter.Until.IsZero() && record.CompletedAt.After(filter.Until) {
				continue
			}

			records = append(records, record)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort by CompletedAt descending (most recent first)
	sort.Slice(records, func(i, j int) bool {
		return records[i].CompletedAt.After(records[j].CompletedAt)
	})

	// Apply limit
	if len(records) > limit {
		records = records[:limit]
	}

	return records, nil
}

// GetStats returns aggregated statistics for executions matching the filter.
func (s *BadgerHistoryStore) GetStats(ctx context.Context, filter *workflow.HistoryFilter) (*workflow.HistoryStats, error) {
	if filter == nil {
		filter = &workflow.HistoryFilter{}
	}

	// Set a high limit to get all records for stats
	statsFilter := *filter
	statsFilter.Limit = 100000

	records, err := s.List(ctx, &statsFilter)
	if err != nil {
		return nil, err
	}

	stats := &workflow.HistoryStats{
		TotalExecutions: len(records),
	}

	var totalDuration int64
	for _, r := range records {
		switch r.Status {
		case "success", "completed":
			stats.SuccessCount++
		case "failed":
			stats.FailedCount++
		case "cancelled":
			stats.CancelledCount++
		}
		totalDuration += r.DurationMs
	}

	if stats.TotalExecutions > 0 {
		stats.AvgDurationMs = totalDuration / int64(stats.TotalExecutions)
	}

	return stats, nil
}

// Cleanup removes execution records older than the specified duration.
// Returns the number of records deleted.
func (s *BadgerHistoryStore) Cleanup(ctx context.Context, olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan)
	cutoffNs := cutoff.UnixNano()

	var deleteCount int
	var keysToDelete [][]byte
	var recordIDsToDelete []string

	// Find old records by scanning timestamp index
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(prefixTs)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().Key()
			// Parse timestamp from key: idx:ts:{timestamp_ns}:{id}
			keyStr := string(key)
			parts := strings.Split(keyStr[len(prefixTs):], ":")
			if len(parts) < 2 {
				continue
			}

			var ts int64
			if _, err := fmt.Sscanf(parts[0], "%d", &ts); err != nil {
				continue
			}

			if ts < cutoffNs {
				recordIDsToDelete = append(recordIDsToDelete, parts[1])
			}
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("scan for old records: %w", err)
	}

	if len(recordIDsToDelete) == 0 {
		return 0, nil
	}

	// Get full record data to find all associated indexes
	err = s.db.View(func(txn *badger.Txn) error {
		for _, id := range recordIDsToDelete {
			record, err := s.getRecord(txn, id)
			if err != nil {
				continue
			}

			ts := record.CompletedAt.UnixNano()

			// Collect all keys to delete
			keysToDelete = append(keysToDelete,
				[]byte(fmt.Sprintf("%s%s", prefixExec, id)),
				[]byte(fmt.Sprintf("%s%s:%020d:%s", prefixWf, record.WorkflowName, ts, id)),
				[]byte(fmt.Sprintf("%s%s:%020d:%s", prefixSt, record.Status, ts, id)),
				[]byte(fmt.Sprintf("%s%020d:%s", prefixTs, ts, id)),
			)
			deleteCount++
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("collect keys to delete: %w", err)
	}

	// Delete in batches
	err = s.db.Update(func(txn *badger.Txn) error {
		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				// Ignore not found errors
				if err != badger.ErrKeyNotFound {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("delete old records: %w", err)
	}

	return deleteCount, nil
}

// Close gracefully shuts down the BadgerDB connection.
// Safe to call multiple times - subsequent calls are no-ops.
func (s *BadgerHistoryStore) Close() error {
	s.closeOnce.Do(func() {
		s.closeErr = s.db.Close()
	})
	return s.closeErr
}

// scanWorkflowIndex scans the workflow index for matching records.
func (s *BadgerHistoryStore) scanWorkflowIndex(workflowName string, since, until time.Time) ([]string, error) {
	var ids []string

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(fmt.Sprintf("%s%s:", prefixWf, workflowName))
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().Key()
			id := extractIDFromIndexKey(string(key), prefixWf+workflowName+":")
			if id != "" {
				ids = append(ids, id)
			}
		}
		return nil
	})

	return ids, err
}

// scanStatusIndex scans the status index for matching records.
func (s *BadgerHistoryStore) scanStatusIndex(status string, since, until time.Time) ([]string, error) {
	var ids []string

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(fmt.Sprintf("%s%s:", prefixSt, status))
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().Key()
			id := extractIDFromIndexKey(string(key), prefixSt+status+":")
			if id != "" {
				ids = append(ids, id)
			}
		}
		return nil
	})

	return ids, err
}

// scanTimestampIndex scans the timestamp index for all records.
func (s *BadgerHistoryStore) scanTimestampIndex(since, until time.Time) ([]string, error) {
	var ids []string

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(prefixTs)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().Key()
			id := extractIDFromTimestampKey(string(key))
			if id != "" {
				ids = append(ids, id)
			}
		}
		return nil
	})

	return ids, err
}

// getRecord fetches a record by ID from the database.
func (s *BadgerHistoryStore) getRecord(txn *badger.Txn, id string) (*workflow.ExecutionRecord, error) {
	key := fmt.Sprintf("%s%s", prefixExec, id)
	item, err := txn.Get([]byte(key))
	if err != nil {
		return nil, err
	}

	var record workflow.ExecutionRecord
	err = item.Value(func(val []byte) error {
		return json.Unmarshal(val, &record)
	})
	if err != nil {
		return nil, err
	}

	return &record, nil
}

// extractIDFromIndexKey extracts the record ID from an index key.
// Key format: {prefix}{timestamp}:{id}
func extractIDFromIndexKey(key, prefix string) string {
	suffix := key[len(prefix):]
	parts := strings.Split(suffix, ":")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// extractIDFromTimestampKey extracts the record ID from a timestamp index key.
// Key format: idx:ts:{timestamp}:{id}
func extractIDFromTimestampKey(key string) string {
	suffix := key[len(prefixTs):]
	parts := strings.Split(suffix, ":")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}
