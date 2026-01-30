//go:build integration

package integration_test

// Feature: C016

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/store"
)

// =============================================================================
// State Persistence Functional Tests
// Validates that JSON store and SQLite history store work correctly
// under various conditions including concurrency and corruption.
// =============================================================================

// TestJSONStore_ConcurrentAccess verifies that the JSON store handles
// concurrent access correctly after C016 improvements.
func TestJSONStore_ConcurrentAccess(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	jsonStore := store.NewJSONStore(statesDir)

	// Act - Save same ID concurrently
	const numConcurrent = 10
	var wg sync.WaitGroup
	errors := make([]error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx := context.Background()
			execCtx := &workflow.ExecutionContext{
				WorkflowID:   "concurrent-test",
				WorkflowName: "test",
				CurrentStep:  "step1",
				Status:       workflow.StatusRunning,
			}
			errors[idx] = jsonStore.Save(ctx, execCtx)
		}(i)
	}

	wg.Wait()

	// Assert - All saves succeeded (no corruption)
	for i := 0; i < numConcurrent; i++ {
		assert.NoError(t, errors[i], "concurrent save %d should succeed", i)
	}

	// Verify state can be loaded
	ctx := context.Background()
	execCtx, err := jsonStore.Load(ctx, "concurrent-test")
	require.NoError(t, err)
	assert.NotNil(t, execCtx)
	assert.Equal(t, "concurrent-test", execCtx.WorkflowID)
}

// TestJSONStore_FileCorruption verifies that the JSON store handles
// file corruption gracefully after C016 improvements.
func TestJSONStore_FileCorruption(t *testing.T) {
	tests := []struct {
		name          string
		setupFile     func(t *testing.T, path string)
		expectError   bool
		errorContains string
	}{
		{
			name: "empty file returns error",
			setupFile: func(t *testing.T, path string) {
				require.NoError(t, os.WriteFile(path, []byte(""), 0o644))
			},
			expectError:   true,
			errorContains: "",
		},
		{
			name: "whitespace-only file returns error",
			setupFile: func(t *testing.T, path string) {
				require.NoError(t, os.WriteFile(path, []byte("   \n\t\n  "), 0o644))
			},
			expectError:   true,
			errorContains: "",
		},
		{
			name: "invalid JSON returns error",
			setupFile: func(t *testing.T, path string) {
				require.NoError(t, os.WriteFile(path, []byte("{invalid json}"), 0o644))
			},
			expectError:   true,
			errorContains: "",
		},
		{
			name: "non-existent file returns nil",
			setupFile: func(t *testing.T, path string) {
				// Don't create file
			},
			expectError:   false,
			errorContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			tmpDir := t.TempDir()
			statesDir := filepath.Join(tmpDir, "states")
			require.NoError(t, os.MkdirAll(statesDir, 0o755))

			jsonStore := store.NewJSONStore(statesDir)
			statePath := filepath.Join(statesDir, "test-state.json")

			if tt.setupFile != nil {
				tt.setupFile(t, statePath)
			}

			// Act
			ctx := context.Background()
			state, err := jsonStore.Load(ctx, "test-state")

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				// For non-existent file, state should be nil
				if tt.name == "non-existent file returns nil" {
					assert.Nil(t, state)
				}
			}
		})
	}
}

// TestSQLiteHistoryStore_Integration verifies that the SQLite history
// store works correctly after C016 improvements.
func TestSQLiteHistoryStore_Integration(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history.db")

	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	defer func() { _ = historyStore.Close() }()

	ctx := context.Background()

	// Act - Record multiple executions
	for i := 0; i < 5; i++ {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("exec-%d", i+1),
			WorkflowID:   fmt.Sprintf("wf-%d", i+1),
			WorkflowName: "test",
			Status:       "success",
			ExitCode:     0,
			StartedAt:    time.Now().Add(-5 * time.Minute),
			CompletedAt:  time.Now(),
			DurationMs:   300000,
		}
		err := historyStore.Record(ctx, record)
		require.NoError(t, err)
	}

	// Query history
	records, err := historyStore.List(ctx, &workflow.HistoryFilter{
		WorkflowName: "test",
	})

	// Assert
	require.NoError(t, err)
	assert.Len(t, records, 5, "should have 5 history records")

	for _, record := range records {
		assert.Equal(t, "test", record.WorkflowName)
		assert.Equal(t, "success", record.Status)
		assert.Equal(t, 0, record.ExitCode)
	}
}
