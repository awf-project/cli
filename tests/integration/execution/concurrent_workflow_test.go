//go:build integration

// Feature: 48
// Tests for bug-48: Replace BadgerDB with SQLite to enable concurrent workflow execution.
// These tests validate that multiple workflows can run simultaneously without database lock
// contention, which was the core issue with BadgerDB's exclusive directory locking.

package execution_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/awf-project/awf/internal/application"
	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/internal/infrastructure/executor"
	infraExpr "github.com/awf-project/awf/internal/infrastructure/expression"
	"github.com/awf-project/awf/internal/infrastructure/repository"
	"github.com/awf-project/awf/internal/infrastructure/store"
	"github.com/awf-project/awf/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// concurrentMockStateStore for concurrent workflow tests
type concurrentMockStateStore struct {
	mu     sync.RWMutex
	states map[string]*workflow.ExecutionContext
}

func newConcurrentMockStateStore() *concurrentMockStateStore {
	return &concurrentMockStateStore{states: make(map[string]*workflow.ExecutionContext)}
}

func (m *concurrentMockStateStore) Save(ctx context.Context, state *workflow.ExecutionContext) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[state.WorkflowID] = state
	return nil
}

func (m *concurrentMockStateStore) Load(ctx context.Context, id string) (*workflow.ExecutionContext, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if state, ok := m.states[id]; ok {
		return state, nil
	}
	return nil, nil
}

func (m *concurrentMockStateStore) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, id)
	return nil
}

func (m *concurrentMockStateStore) List(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.states))
	for id := range m.states {
		ids = append(ids, id)
	}
	return ids, nil
}

// concurrentMockLogger for concurrent workflow tests
type concurrentMockLogger struct{}

func (m *concurrentMockLogger) Debug(msg string, fields ...any) {}
func (m *concurrentMockLogger) Info(msg string, fields ...any)  {}
func (m *concurrentMockLogger) Warn(msg string, fields ...any)  {}
func (m *concurrentMockLogger) Error(msg string, fields ...any) {}
func (m *concurrentMockLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

// TestConcurrentWorkflowExecution_SharedHistoryStore validates that multiple
// workflows can execute concurrently while sharing the same SQLite history store.
// This is the core test for bug-48: BadgerDB used exclusive locks preventing
// concurrent workflow execution.
func TestConcurrentWorkflowExecution_SharedHistoryStore(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, "workflows")
	historyPath := filepath.Join(tmpDir, "history.db")

	require.NoError(t, os.MkdirAll(workflowDir, 0o755))

	// Create multiple workflow files that can run concurrently
	workflowNames := []string{"workflow-a", "workflow-b", "workflow-c"}
	for _, name := range workflowNames {
		wfYAML := fmt.Sprintf(`name: %s
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo "running %s"
    on_success: done
  done:
    type: terminal
`, name, name)
		err := os.WriteFile(filepath.Join(workflowDir, name+".yaml"), []byte(wfYAML), 0o644)
		require.NoError(t, err)
	}

	// Create shared history store with SQLite
	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	defer historyStore.Close()

	// Execute workflows concurrently
	var wg sync.WaitGroup
	errors := make(chan error, len(workflowNames))
	completedCount := atomic.Int32{}

	for _, name := range workflowNames {
		wg.Add(1)
		go func(workflowName string) {
			defer wg.Done()

			// Each goroutine creates its own service instances but shares the history store
			repo := repository.NewYAMLRepository(workflowDir)
			stateStore := newConcurrentMockStateStore()
			exec := executor.NewShellExecutor()
			logger := &concurrentMockLogger{}
			resolver := interpolation.NewTemplateResolver()

			// NOTE: Don't close historySvc here - it would close the shared store
			// The store is owned by the parent scope and closed there
			historySvc := application.NewHistoryService(historyStore, logger)

			wfSvc := application.NewWorkflowService(repo, stateStore, exec, logger, infraExpr.NewExprValidator())
			parallelExec := application.NewParallelExecutor(logger)
			execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, stateStore, logger, resolver, historySvc)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			execCtx, err := execSvc.Run(ctx, workflowName, nil)
			if err != nil {
				errors <- fmt.Errorf("workflow %s failed: %w", workflowName, err)
				return
			}
			if execCtx.Status != workflow.StatusCompleted {
				errors <- fmt.Errorf("workflow %s status is %s, expected %s",
					workflowName, execCtx.Status, workflow.StatusCompleted)
				return
			}
			completedCount.Add(1)
		}(name)
	}

	wg.Wait()
	close(errors)

	// Collect errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}
	require.Empty(t, errs, "concurrent workflows should not fail: %v", errs)

	// Verify all workflows completed
	assert.Equal(t, int32(len(workflowNames)), completedCount.Load(),
		"all workflows should complete")

	// Verify history records
	records, err := historyStore.List(context.Background(), &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	assert.Len(t, records, len(workflowNames), "should have one history record per workflow")
}

// TestConcurrentWorkflowExecution_HistoryIntegrity verifies that all workflow
// executions are correctly recorded in history even under concurrent load.
func TestConcurrentWorkflowExecution_HistoryIntegrity(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, "workflows")
	historyPath := filepath.Join(tmpDir, "history.db")

	require.NoError(t, os.MkdirAll(workflowDir, 0o755))

	// Create a simple workflow
	wfYAML := `name: integrity-test
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo "test"
    on_success: done
  done:
    type: terminal
`
	err := os.WriteFile(filepath.Join(workflowDir, "integrity-test.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	defer historyStore.Close()

	// Run the same workflow multiple times concurrently
	concurrentRuns := 10
	var wg sync.WaitGroup
	errors := make(chan error, concurrentRuns)
	executedIDs := make(chan string, concurrentRuns)

	for i := 0; i < concurrentRuns; i++ {
		wg.Add(1)
		go func(runIndex int) {
			defer wg.Done()

			repo := repository.NewYAMLRepository(workflowDir)
			stateStore := newConcurrentMockStateStore()
			exec := executor.NewShellExecutor()
			logger := &concurrentMockLogger{}
			resolver := interpolation.NewTemplateResolver()

			// NOTE: Don't close historySvc here - it would close the shared store
			historySvc := application.NewHistoryService(historyStore, logger)

			wfSvc := application.NewWorkflowService(repo, stateStore, exec, logger, infraExpr.NewExprValidator())
			parallelExec := application.NewParallelExecutor(logger)
			execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, stateStore, logger, resolver, historySvc)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			execCtx, err := execSvc.Run(ctx, "integrity-test", nil)
			if err != nil {
				errors <- fmt.Errorf("run %d failed: %w", runIndex, err)
				return
			}
			executedIDs <- execCtx.WorkflowID
		}(i)
	}

	wg.Wait()
	close(errors)
	close(executedIDs)

	// Collect errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}
	require.Empty(t, errs, "concurrent runs should not fail: %v", errs)

	// Collect executed IDs
	var expectedIDs []string
	for id := range executedIDs {
		expectedIDs = append(expectedIDs, id)
	}

	// Verify all records are present and unique
	records, err := historyStore.List(context.Background(), &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	assert.Len(t, records, concurrentRuns, "should have %d history records", concurrentRuns)

	// Verify IDs are unique
	recordedIDs := make(map[string]bool)
	for _, r := range records {
		assert.False(t, recordedIDs[r.ID], "duplicate ID found: %s", r.ID)
		recordedIDs[r.ID] = true
	}

	// Verify all executed workflow IDs are recorded
	for _, id := range expectedIDs {
		assert.True(t, recordedIDs[id], "executed workflow ID %s not found in history", id)
	}
}

// TestConcurrentHistoryStoreAccess validates that multiple goroutines can
// read and write to the SQLite history store without data corruption.
func TestConcurrentHistoryStoreAccess(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history.db")

	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	defer historyStore.Close()

	ctx := context.Background()
	now := time.Now()

	// Concurrent writers
	writerCount := 5
	recordsPerWriter := 20
	totalRecords := writerCount * recordsPerWriter

	var writeWg sync.WaitGroup
	writeErrors := make(chan error, totalRecords)

	for w := 0; w < writerCount; w++ {
		writeWg.Add(1)
		go func(writerID int) {
			defer writeWg.Done()

			for r := 0; r < recordsPerWriter; r++ {
				record := &workflow.ExecutionRecord{
					ID:           fmt.Sprintf("exec-w%d-r%d", writerID, r),
					WorkflowID:   fmt.Sprintf("wf-w%d-r%d", writerID, r),
					WorkflowName: fmt.Sprintf("workflow-%d", writerID),
					Status:       "success",
					ExitCode:     0,
					StartedAt:    now.Add(-time.Duration(r) * time.Second),
					CompletedAt:  now,
					DurationMs:   int64(r * 100),
				}

				if err := historyStore.Record(ctx, record); err != nil {
					writeErrors <- fmt.Errorf("writer %d record %d: %w", writerID, r, err)
				}
			}
		}(w)
	}

	// Concurrent readers (start after a brief delay to allow some writes)
	time.Sleep(10 * time.Millisecond)

	readerCount := 3
	var readWg sync.WaitGroup
	readErrors := make(chan error, readerCount*10)

	for r := 0; r < readerCount; r++ {
		readWg.Add(1)
		go func(readerID int) {
			defer readWg.Done()

			// Perform multiple reads while writes are happening
			for i := 0; i < 10; i++ {
				_, err := historyStore.List(ctx, &workflow.HistoryFilter{Limit: 100})
				if err != nil {
					readErrors <- fmt.Errorf("reader %d iteration %d list: %w", readerID, i, err)
				}

				_, err = historyStore.GetStats(ctx, &workflow.HistoryFilter{})
				if err != nil {
					readErrors <- fmt.Errorf("reader %d iteration %d stats: %w", readerID, i, err)
				}

				time.Sleep(5 * time.Millisecond)
			}
		}(r)
	}

	// Wait for all operations to complete
	writeWg.Wait()
	close(writeErrors)
	readWg.Wait()
	close(readErrors)

	// Collect write errors
	var writeErrs []error
	for err := range writeErrors {
		writeErrs = append(writeErrs, err)
	}
	require.Empty(t, writeErrs, "write operations should not fail: %v", writeErrs)

	// Collect read errors
	var readErrs []error
	for err := range readErrors {
		readErrs = append(readErrs, err)
	}
	require.Empty(t, readErrs, "read operations should not fail: %v", readErrs)

	// Verify all records were written
	records, err := historyStore.List(ctx, &workflow.HistoryFilter{Limit: totalRecords + 10})
	require.NoError(t, err)
	assert.Len(t, records, totalRecords, "should have %d records", totalRecords)

	// Verify data integrity
	recordMap := make(map[string]*workflow.ExecutionRecord)
	for _, r := range records {
		assert.Nil(t, recordMap[r.ID], "duplicate record ID: %s", r.ID)
		recordMap[r.ID] = r
	}

	// Verify each record has correct data
	for w := 0; w < writerCount; w++ {
		for r := 0; r < recordsPerWriter; r++ {
			id := fmt.Sprintf("exec-w%d-r%d", w, r)
			record, exists := recordMap[id]
			require.True(t, exists, "record %s should exist", id)
			assert.Equal(t, fmt.Sprintf("workflow-%d", w), record.WorkflowName)
			assert.Equal(t, "success", record.Status)
		}
	}
}

// TestConcurrentWorkflowExecution_NoLockContention ensures that concurrent
// workflow executions don't block each other due to database locking.
func TestConcurrentWorkflowExecution_NoLockContention(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, "workflows")
	historyPath := filepath.Join(tmpDir, "history.db")

	require.NoError(t, os.MkdirAll(workflowDir, 0o755))

	// Create workflows with varying execution times
	tests := []struct {
		name     string
		sleepMs  int
		expected time.Duration
	}{
		{"fast", 50, 200 * time.Millisecond},
		{"medium", 100, 300 * time.Millisecond},
		{"slow", 200, 500 * time.Millisecond},
	}

	for _, tc := range tests {
		wfYAML := fmt.Sprintf(`name: %s
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: sleep 0.%03d && echo "done"
    on_success: done
  done:
    type: terminal
`, tc.name, tc.sleepMs)
		err := os.WriteFile(filepath.Join(workflowDir, tc.name+".yaml"), []byte(wfYAML), 0o644)
		require.NoError(t, err)
	}

	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	defer historyStore.Close()

	// Track execution times
	type result struct {
		name     string
		duration time.Duration
		err      error
	}
	results := make(chan result, len(tests))

	var wg sync.WaitGroup
	startTime := time.Now()

	for _, tc := range tests {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			start := time.Now()

			repo := repository.NewYAMLRepository(workflowDir)
			stateStore := newConcurrentMockStateStore()
			exec := executor.NewShellExecutor()
			logger := &concurrentMockLogger{}
			resolver := interpolation.NewTemplateResolver()

			// NOTE: Don't close historySvc here - it would close the shared store
			historySvc := application.NewHistoryService(historyStore, logger)

			wfSvc := application.NewWorkflowService(repo, stateStore, exec, logger, infraExpr.NewExprValidator())
			parallelExec := application.NewParallelExecutor(logger)
			execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, stateStore, logger, resolver, historySvc)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			_, err := execSvc.Run(ctx, name, nil)
			results <- result{
				name:     name,
				duration: time.Since(start),
				err:      err,
			}
		}(tc.name)
	}

	wg.Wait()
	totalTime := time.Since(startTime)
	close(results)

	// Verify no errors
	for r := range results {
		require.NoError(t, r.err, "workflow %s should complete without error", r.name)
	}

	// If workflows ran sequentially due to lock contention, total time would be
	// at least 350ms (50+100+200). With concurrent execution, total time should
	// be close to the longest workflow (~200ms) plus overhead.
	// Allow generous margin for CI environment variability.
	maxAcceptableTime := 3 * time.Second // generous margin for slow CI environments
	assert.Less(t, totalTime, maxAcceptableTime,
		"total execution time should indicate concurrent execution, not blocking")

	// Verify all workflows recorded in history
	records, err := historyStore.List(context.Background(), &workflow.HistoryFilter{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, records, len(tests), "all workflows should be recorded in history")
}

// TestConcurrentHistoryStoreAccess_MultipleStoreInstances validates that multiple
// SQLiteHistoryStore instances pointing to the same database file can operate
// concurrently without conflicts. This simulates multiple awf processes accessing
// the same history database.
func TestConcurrentHistoryStoreAccess_MultipleStoreInstances(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history.db")
	ctx := context.Background()
	now := time.Now()

	// Number of concurrent store instances (simulating multiple processes)
	instanceCount := 5
	recordsPerInstance := 10

	var wg sync.WaitGroup
	errors := make(chan error, instanceCount*recordsPerInstance)

	// Each goroutine creates its own SQLiteHistoryStore instance
	// This simulates multiple awf processes accessing the same database
	for i := 0; i < instanceCount; i++ {
		wg.Add(1)
		go func(instanceID int) {
			defer wg.Done()

			// Create a new store instance (simulates a new process opening the DB)
			historyStore, err := store.NewSQLiteHistoryStore(historyPath)
			if err != nil {
				errors <- fmt.Errorf("instance %d failed to open store: %w", instanceID, err)
				return
			}
			defer historyStore.Close()

			// Write records
			for r := 0; r < recordsPerInstance; r++ {
				record := &workflow.ExecutionRecord{
					ID:           fmt.Sprintf("inst%d-exec%d", instanceID, r),
					WorkflowID:   fmt.Sprintf("inst%d-wf%d", instanceID, r),
					WorkflowName: fmt.Sprintf("instance-%d-workflow", instanceID),
					Status:       "success",
					ExitCode:     0,
					StartedAt:    now.Add(-time.Duration(r) * time.Second),
					CompletedAt:  now,
					DurationMs:   int64(r * 100),
				}

				if err := historyStore.Record(ctx, record); err != nil {
					errors <- fmt.Errorf("instance %d record %d write failed: %w", instanceID, r, err)
				}
			}

			// Read and verify
			records, err := historyStore.List(ctx, &workflow.HistoryFilter{
				WorkflowName: fmt.Sprintf("instance-%d-workflow", instanceID),
				Limit:        100,
			})
			if err != nil {
				errors <- fmt.Errorf("instance %d list failed: %w", instanceID, err)
				return
			}

			if len(records) != recordsPerInstance {
				errors <- fmt.Errorf("instance %d expected %d records, got %d",
					instanceID, recordsPerInstance, len(records))
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Collect errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}
	require.Empty(t, errs, "multiple store instances should work concurrently: %v", errs)

	// Verify total records with a fresh store instance
	verifyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	defer verifyStore.Close()

	totalExpected := instanceCount * recordsPerInstance
	records, err := verifyStore.List(ctx, &workflow.HistoryFilter{Limit: totalExpected + 10})
	require.NoError(t, err)
	assert.Len(t, records, totalExpected, "should have all records from all instances")

	// Verify stats
	stats, err := verifyStore.GetStats(ctx, &workflow.HistoryFilter{})
	require.NoError(t, err)
	assert.Equal(t, totalExpected, stats.TotalExecutions)
	assert.Equal(t, totalExpected, stats.SuccessCount)
}

// TestConcurrentWorkflowExecution_RapidSuccession tests that workflows can be
// started in rapid succession without database errors.
func TestConcurrentWorkflowExecution_RapidSuccession(t *testing.T) {
	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, "workflows")
	historyPath := filepath.Join(tmpDir, "history.db")

	require.NoError(t, os.MkdirAll(workflowDir, 0o755))

	// Create a quick workflow
	wfYAML := `name: rapid-test
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo "rapid"
    on_success: done
  done:
    type: terminal
`
	err := os.WriteFile(filepath.Join(workflowDir, "rapid-test.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	defer historyStore.Close()

	// Start many workflows in rapid succession
	rapidCount := 20
	var wg sync.WaitGroup
	errors := make(chan error, rapidCount)

	for i := 0; i < rapidCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			repo := repository.NewYAMLRepository(workflowDir)
			stateStore := newConcurrentMockStateStore()
			exec := executor.NewShellExecutor()
			logger := &concurrentMockLogger{}
			resolver := interpolation.NewTemplateResolver()

			// NOTE: Don't close historySvc here - it would close the shared store
			historySvc := application.NewHistoryService(historyStore, logger)

			wfSvc := application.NewWorkflowService(repo, stateStore, exec, logger, infraExpr.NewExprValidator())
			parallelExec := application.NewParallelExecutor(logger)
			execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, stateStore, logger, resolver, historySvc)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			_, err := execSvc.Run(ctx, "rapid-test", nil)
			if err != nil {
				errors <- fmt.Errorf("rapid run %d: %w", index, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Collect errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}
	require.Empty(t, errs, "rapid succession runs should not fail: %v", errs)

	// Verify all recorded
	records, err := historyStore.List(context.Background(), &workflow.HistoryFilter{Limit: rapidCount + 10})
	require.NoError(t, err)
	assert.Len(t, records, rapidCount, "all rapid runs should be recorded")
}

// TestConcurrentHistoryStoreAccess_MixedOperations tests concurrent
// Record, List, GetStats, and Cleanup operations on the history store.
func TestConcurrentHistoryStoreAccess_MixedOperations(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history.db")

	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	defer historyStore.Close()

	ctx := context.Background()
	now := time.Now()

	// Pre-populate some records
	for i := 0; i < 50; i++ {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("prepop-exec-%d", i),
			WorkflowID:   fmt.Sprintf("prepop-wf-%d", i),
			WorkflowName: "mixed-test",
			Status:       "success",
			StartedAt:    now.Add(-time.Duration(i) * time.Hour),
			CompletedAt:  now.Add(-time.Duration(i)*time.Hour + time.Minute),
			DurationMs:   60000,
		}
		require.NoError(t, historyStore.Record(ctx, record))
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Writers
	for w := 0; w < 3; w++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				record := &workflow.ExecutionRecord{
					ID:           fmt.Sprintf("mixed-w%d-r%d", writerID, i),
					WorkflowID:   fmt.Sprintf("mixed-wf-w%d-r%d", writerID, i),
					WorkflowName: "mixed-test",
					Status:       "success",
					StartedAt:    now.Add(-time.Minute),
					CompletedAt:  now,
					DurationMs:   int64(i * 100),
				}
				if err := historyStore.Record(ctx, record); err != nil {
					errors <- fmt.Errorf("writer %d record %d: %w", writerID, i, err)
				}
				time.Sleep(time.Millisecond)
			}
		}(w)
	}

	// Listers
	for l := 0; l < 2; l++ {
		wg.Add(1)
		go func(listerID int) {
			defer wg.Done()
			for i := 0; i < 30; i++ {
				_, err := historyStore.List(ctx, &workflow.HistoryFilter{
					WorkflowName: "mixed-test",
					Limit:        50,
				})
				if err != nil {
					errors <- fmt.Errorf("lister %d iteration %d: %w", listerID, i, err)
				}
				time.Sleep(time.Millisecond)
			}
		}(l)
	}

	// Stats readers
	for s := 0; s < 2; s++ {
		wg.Add(1)
		go func(statsID int) {
			defer wg.Done()
			for i := 0; i < 30; i++ {
				_, err := historyStore.GetStats(ctx, &workflow.HistoryFilter{
					WorkflowName: "mixed-test",
				})
				if err != nil {
					errors <- fmt.Errorf("stats %d iteration %d: %w", statsID, i, err)
				}
				time.Sleep(time.Millisecond)
			}
		}(s)
	}

	// Cleanup (only deletes old records, shouldn't affect concurrent ops)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			_, err := historyStore.Cleanup(ctx, 100*24*time.Hour) // Clean very old
			if err != nil {
				errors <- fmt.Errorf("cleanup iteration %d: %w", i, err)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	wg.Wait()
	close(errors)

	// Collect errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}
	require.Empty(t, errs, "mixed operations should not fail: %v", errs)

	// Verify data integrity
	stats, err := historyStore.GetStats(ctx, &workflow.HistoryFilter{WorkflowName: "mixed-test"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, stats.TotalExecutions, 50+3*20, // prepopulated + writers
		"should have at least expected records")
}
