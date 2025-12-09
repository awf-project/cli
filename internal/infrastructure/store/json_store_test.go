package store_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/store"
)

// Cycle 1: Interface compliance - compile-time check
func TestJSONStore_ImplementsInterface(t *testing.T) {
	var _ ports.StateStore = (*store.JSONStore)(nil)
}

// Cycle 1: Core Save functionality
func TestJSONStore_Save_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	execCtx := workflow.NewExecutionContext("test-123", "test-workflow")
	execCtx.Status = workflow.StatusRunning
	execCtx.SetInput("key", "value")

	err := s.Save(ctx, execCtx)
	require.NoError(t, err)

	// Verify file exists
	filePath := filepath.Join(tmpDir, "test-123.json")
	_, err = os.Stat(filePath)
	assert.NoError(t, err, "state file should exist")

	// Verify content is valid JSON
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var loaded workflow.ExecutionContext
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)
	assert.Equal(t, "test-123", loaded.WorkflowID)
	assert.Equal(t, workflow.StatusRunning, loaded.Status)
}

// Cycle 1: Core Load functionality
func TestJSONStore_Load_ExistingState(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	// Save first
	original := workflow.NewExecutionContext("load-test", "workflow")
	original.Status = workflow.StatusCompleted
	original.SetInput("input1", "value1")
	original.SetStepState("step1", workflow.StepState{
		Name:        "step1",
		Status:      workflow.StatusCompleted,
		Output:      "output data",
		ExitCode:    0,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	})
	require.NoError(t, s.Save(ctx, original))

	// Load
	loaded, err := s.Load(ctx, "load-test")
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, original.WorkflowID, loaded.WorkflowID)
	assert.Equal(t, original.Status, loaded.Status)

	val, ok := loaded.GetInput("input1")
	assert.True(t, ok)
	assert.Equal(t, "value1", val)

	state, ok := loaded.GetStepState("step1")
	assert.True(t, ok)
	assert.Equal(t, "output data", state.Output)
}

// Cycle 1: Save overwrites existing state
func TestJSONStore_Save_Overwrites(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	// First save
	execCtx := workflow.NewExecutionContext("overwrite-test", "workflow")
	execCtx.Status = workflow.StatusRunning
	require.NoError(t, s.Save(ctx, execCtx))

	// Update and save again
	execCtx.Status = workflow.StatusCompleted
	execCtx.SetInput("new-key", "new-value")
	require.NoError(t, s.Save(ctx, execCtx))

	// Verify updated content
	loaded, err := s.Load(ctx, "overwrite-test")
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, loaded.Status)

	val, ok := loaded.GetInput("new-key")
	assert.True(t, ok)
	assert.Equal(t, "new-value", val)
}

// Cycle 2: Atomic write - temp file should not exist after save
func TestJSONStore_Save_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	execCtx := workflow.NewExecutionContext("atomic-test", "test")
	err := s.Save(ctx, execCtx)
	require.NoError(t, err)

	// Temp file should not exist after successful save
	tmpPath := filepath.Join(tmpDir, "atomic-test.json.tmp")
	_, err = os.Stat(tmpPath)
	assert.True(t, os.IsNotExist(err), "temp file should be removed after save")

	// Final file should exist
	finalPath := filepath.Join(tmpDir, "atomic-test.json")
	_, err = os.Stat(finalPath)
	assert.NoError(t, err, "final file should exist")
}

// Cycle 3: Load non-existent state returns nil, nil
func TestJSONStore_Load_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	loaded, err := s.Load(ctx, "nonexistent")
	assert.NoError(t, err, "should not return error for missing state")
	assert.Nil(t, loaded, "should return nil for missing state")
}

// Cycle 3: Load invalid JSON returns error
func TestJSONStore_Load_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create invalid JSON file
	invalidPath := filepath.Join(tmpDir, "invalid.json")
	err := os.WriteFile(invalidPath, []byte("not valid json{"), 0600)
	require.NoError(t, err)

	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	_, err = s.Load(ctx, "invalid")
	assert.Error(t, err, "should return error for invalid JSON")
}

// Cycle 4: Delete existing state
func TestJSONStore_Delete_ExistingState(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	// Create state
	execCtx := workflow.NewExecutionContext("to-delete", "test")
	require.NoError(t, s.Save(ctx, execCtx))

	// Delete
	err := s.Delete(ctx, "to-delete")
	require.NoError(t, err)

	// Verify deleted
	loaded, _ := s.Load(ctx, "to-delete")
	assert.Nil(t, loaded)
}

// Cycle 4: Delete non-existent is idempotent
func TestJSONStore_Delete_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	// Should not error on non-existent
	err := s.Delete(ctx, "nonexistent")
	assert.NoError(t, err, "delete should be idempotent")
}

// Cycle 4: List empty directory
func TestJSONStore_List_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	ids, err := s.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, ids)
}

// Cycle 4: List multiple states
func TestJSONStore_List_MultipleStates(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	// Create multiple states
	for _, id := range []string{"id1", "id2", "id3"} {
		execCtx := workflow.NewExecutionContext(id, "test")
		require.NoError(t, s.Save(ctx, execCtx))
	}

	ids, err := s.List(ctx)
	require.NoError(t, err)
	assert.Len(t, ids, 3)
	assert.Contains(t, ids, "id1")
	assert.Contains(t, ids, "id2")
	assert.Contains(t, ids, "id3")
}

// Cycle 5: Save creates directory if missing
func TestJSONStore_Save_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "nested", "states")
	s := store.NewJSONStore(nestedPath)
	ctx := context.Background()

	execCtx := workflow.NewExecutionContext("nested-test", "test")
	err := s.Save(ctx, execCtx)
	require.NoError(t, err)

	// Directory should be created
	_, err = os.Stat(nestedPath)
	assert.NoError(t, err, "directory should be created")
}

// Cycle 6: Concurrent saves should not corrupt data
func TestJSONStore_ConcurrentSaves(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			execCtx := workflow.NewExecutionContext("concurrent", "test")
			execCtx.CurrentStep = fmt.Sprintf("step-%d", n)
			_ = s.Save(ctx, execCtx)
		}(i)
	}

	wg.Wait()

	// File should exist and be valid JSON
	loaded, err := s.Load(ctx, "concurrent")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "concurrent", loaded.WorkflowID)
}

// Cycle 6: Concurrent different IDs
func TestJSONStore_ConcurrentDifferentIDs(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			id := fmt.Sprintf("workflow-%d", n)
			execCtx := workflow.NewExecutionContext(id, "test")
			err := s.Save(ctx, execCtx)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// All files should exist
	ids, err := s.List(ctx)
	require.NoError(t, err)
	assert.Len(t, ids, goroutines)
}
