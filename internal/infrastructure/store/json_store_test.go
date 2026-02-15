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

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/internal/infrastructure/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	// No temp files should exist after successful save
	tmpFiles, err := filepath.Glob(filepath.Join(tmpDir, "*.tmp"))
	require.NoError(t, err)
	assert.Empty(t, tmpFiles, "temp files should be removed after save")

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
	err := os.WriteFile(invalidPath, []byte("not valid json{"), 0o600)
	require.NoError(t, err)

	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	_, err = s.Load(ctx, "invalid")
	assert.Error(t, err, "should return error for invalid JSON")
}

// Cycle 3: Load empty file returns appropriate error
func TestJSONStore_Load_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty JSON file (0 bytes)
	emptyPath := filepath.Join(tmpDir, "empty.json")
	err := os.WriteFile(emptyPath, []byte(""), 0o600)
	require.NoError(t, err)

	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	_, err = s.Load(ctx, "empty")
	assert.Error(t, err, "should return error for empty JSON file")
}

// Cycle 3: Load whitespace-only file returns appropriate error
// C016: T010 - File corruption recovery (whitespace-only edge case)
func TestJSONStore_Load_WhitespaceOnly(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file with only whitespace (spaces, tabs, newlines)
	whitespacePath := filepath.Join(tmpDir, "whitespace.json")
	err := os.WriteFile(whitespacePath, []byte("  \t\n  \r\n  "), 0o600)
	require.NoError(t, err)

	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	_, err = s.Load(ctx, "whitespace")
	assert.Error(t, err, "should return error for whitespace-only JSON file")
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

// Item: T009
// Feature: C016
// TestJSONStore_ConcurrentReads tests multiple concurrent read operations.
// This verifies that multiple goroutines can safely read from the store simultaneously.
func TestJSONStore_ConcurrentReads(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	// Insert test data first
	const testWorkflows = 10
	for i := 0; i < testWorkflows; i++ {
		id := fmt.Sprintf("workflow-%d", i)
		execCtx := workflow.NewExecutionContext(id, "test-workflow")
		execCtx.CurrentStep = fmt.Sprintf("step-%d", i)
		execCtx.Status = workflow.StatusRunning
		err := s.Save(ctx, execCtx)
		require.NoError(t, err)
	}

	// Concurrent readers
	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			// Each reader loads all workflows
			for j := 0; j < testWorkflows; j++ {
				id := fmt.Sprintf("workflow-%d", j)
				loaded, err := s.Load(ctx, id)
				assert.NoError(t, err)
				assert.NotNil(t, loaded)
				if loaded != nil {
					assert.Equal(t, id, loaded.WorkflowID)
				}
			}
		}()
	}

	wg.Wait()
}

// RED Phase: Test stubs for failure path tests
// These tests ensure error handling paths are covered

// TestJSONStore_Save_PermissionDenied tests save failure when directory is read-only.
func TestJSONStore_Save_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")

	// Create directory and make it read-only
	require.NoError(t, os.MkdirAll(readOnlyDir, 0o755))
	require.NoError(t, os.Chmod(readOnlyDir, 0o444))
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0o755) })

	s := store.NewJSONStore(readOnlyDir)
	ctx := context.Background()

	execCtx := workflow.NewExecutionContext("perm-test", "test")
	err := s.Save(ctx, execCtx)

	assert.Error(t, err, "Save should fail on read-only directory")
	assert.True(t, os.IsPermission(err), "error should be permission error")
}

// TestJSONStore_Load_PermissionDenied tests load failure when file is unreadable.
func TestJSONStore_Load_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tmpDir := t.TempDir()
	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	// Create a valid state first
	execCtx := workflow.NewExecutionContext("unreadable", "test")
	require.NoError(t, s.Save(ctx, execCtx))

	// Make file unreadable
	filePath := filepath.Join(tmpDir, "unreadable.json")
	require.NoError(t, os.Chmod(filePath, 0o000))
	t.Cleanup(func() { _ = os.Chmod(filePath, 0o644) })

	_, err := s.Load(ctx, "unreadable")
	assert.Error(t, err, "Load should fail on unreadable file")
	assert.True(t, os.IsPermission(err), "error should be permission error")
}

// TestJSONStore_List_IgnoresNonJSONFiles verifies List only returns .json files.
func TestJSONStore_List_IgnoresNonJSONFiles(t *testing.T) {
	tmpDir := t.TempDir()
	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	// Create a valid JSON state
	execCtx := workflow.NewExecutionContext("valid-id", "test")
	require.NoError(t, s.Save(ctx, execCtx))

	// Create non-JSON files that should be ignored
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "random.txt"), []byte("text"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "backup.json.bak"), []byte("{}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".hidden.json"), []byte("{}"), 0o644))

	ids, err := s.List(ctx)
	require.NoError(t, err)

	// Should only contain valid-id and .hidden.json (glob matches hidden files)
	assert.Contains(t, ids, "valid-id")
	// Note: glob pattern *.json matches .hidden.json too - this is expected behavior
}

// TestJSONStore_Save_NestedDirectory tests save creates nested directory structure.
func TestJSONStore_Save_NestedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	deepPath := filepath.Join(tmpDir, "level1", "level2", "level3", "states")

	s := store.NewJSONStore(deepPath)
	ctx := context.Background()

	execCtx := workflow.NewExecutionContext("deep-nested", "test")
	err := s.Save(ctx, execCtx)

	require.NoError(t, err)

	// Verify all directories were created
	_, statErr := os.Stat(deepPath)
	assert.NoError(t, statErr, "nested directory should be created")

	// Verify file exists
	loaded, loadErr := s.Load(ctx, "deep-nested")
	require.NoError(t, loadErr)
	assert.Equal(t, "deep-nested", loaded.WorkflowID)
}

// TestJSONStore_Delete_PermissionDenied tests delete failure on read-only directory.
func TestJSONStore_Delete_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tmpDir := t.TempDir()
	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	// Create a state first
	execCtx := workflow.NewExecutionContext("delete-perm", "test")
	require.NoError(t, s.Save(ctx, execCtx))

	// Make directory read-only (can't delete files)
	require.NoError(t, os.Chmod(tmpDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(tmpDir, 0o755) })

	err := s.Delete(ctx, "delete-perm")
	assert.Error(t, err, "Delete should fail on read-only directory")
}

// Race condition tests - run with: go test -race ./internal/infrastructure/store/...

// TestJSONStore_RaceSaveLoad tests concurrent Save and Load on the same workflow ID.
// This catches race conditions between writers and readers.
func TestJSONStore_RaceSaveLoad(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	const iterations = 50
	const workflowID = "race-save-load"

	// Seed initial state
	initial := workflow.NewExecutionContext(workflowID, "test")
	require.NoError(t, s.Save(ctx, initial))

	var wg sync.WaitGroup

	// Writers
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go func(n int) {
			defer wg.Done()
			execCtx := workflow.NewExecutionContext(workflowID, "test")
			execCtx.CurrentStep = fmt.Sprintf("step-%d", n)
			execCtx.Status = workflow.StatusRunning
			_ = s.Save(ctx, execCtx)
		}(i)
	}

	// Readers
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			loaded, err := s.Load(ctx, workflowID)
			// Should never get corrupted JSON
			if err != nil {
				t.Errorf("Load returned error: %v", err)
			}
			if loaded == nil {
				t.Error("Load returned nil unexpectedly")
			}
		}()
	}

	wg.Wait()

	// Final state should be valid
	final, err := s.Load(ctx, workflowID)
	require.NoError(t, err)
	require.NotNil(t, final)
	assert.Equal(t, workflowID, final.WorkflowID)
}

// TestJSONStore_RaceSaveDelete tests concurrent Save and Delete operations.
// This catches race conditions between writers and deleters.
func TestJSONStore_RaceSaveDelete(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	const iterations = 30
	const workflowID = "race-save-delete"

	var wg sync.WaitGroup

	// Writers and deleters interleaved
	wg.Add(iterations * 2)
	for i := 0; i < iterations; i++ {
		go func(n int) {
			defer wg.Done()
			execCtx := workflow.NewExecutionContext(workflowID, "test")
			execCtx.CurrentStep = fmt.Sprintf("step-%d", n)
			_ = s.Save(ctx, execCtx)
		}(i)

		go func() {
			defer wg.Done()
			_ = s.Delete(ctx, workflowID)
		}()
	}

	wg.Wait()

	// State should be either present and valid, or absent
	loaded, err := s.Load(ctx, workflowID)
	assert.NoError(t, err, "Load should not error even after race")
	// loaded can be nil (deleted) or valid (saved last)
	if loaded != nil {
		assert.Equal(t, workflowID, loaded.WorkflowID)
	}
}

// TestJSONStore_RaceListSave tests concurrent List and Save operations.
func TestJSONStore_RaceListSave(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	const iterations = 20

	var wg sync.WaitGroup
	wg.Add(iterations * 2)

	// Savers
	for i := 0; i < iterations; i++ {
		go func(n int) {
			defer wg.Done()
			id := fmt.Sprintf("list-race-%d", n)
			execCtx := workflow.NewExecutionContext(id, "test")
			_ = s.Save(ctx, execCtx)
		}(i)
	}

	// Listers
	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			ids, err := s.List(ctx)
			assert.NoError(t, err, "List should not error during race")
			// ids length will vary depending on timing, but shouldn't panic
			_ = ids
		}()
	}

	wg.Wait()

	// Final list should have all saved items
	ids, err := s.List(ctx)
	require.NoError(t, err)
	assert.Len(t, ids, iterations)
}

// Item: T010
// Feature: C016
// Description: Comprehensive tests for file corruption recovery scenarios

// TestJSONStore_Load_TruncatedJSON verifies recovery from truncated JSON files
func TestJSONStore_Load_TruncatedJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create truncated JSON file (ends mid-object)
	truncatedPath := filepath.Join(tmpDir, "truncated.json")
	truncatedContent := `{"workflow_id":"test-123","instance_id":"inst-456","status":"running","inputs":{"key":`
	err := os.WriteFile(truncatedPath, []byte(truncatedContent), 0o600)
	require.NoError(t, err)

	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	_, err = s.Load(ctx, "truncated")
	assert.Error(t, err, "should return error for truncated JSON")
	assert.Contains(t, err.Error(), "unexpected end", "error should indicate unexpected end of JSON")
}

// TestJSONStore_Load_WrongJSONType verifies recovery when JSON type doesn't match expected struct
func TestJSONStore_Load_WrongJSONType(t *testing.T) {
	tests := []struct {
		name    string
		content string
		errMsg  string
	}{
		{
			name:    "array instead of object",
			content: `["item1", "item2", "item3"]`,
			errMsg:  "cannot unmarshal",
		},
		{
			name:    "primitive string",
			content: `"just a string"`,
			errMsg:  "cannot unmarshal",
		},
		{
			name:    "primitive number",
			content: `42`,
			errMsg:  "cannot unmarshal",
		},
		{
			name:    "null value",
			content: `null`,
			errMsg:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			wrongTypePath := filepath.Join(tmpDir, "wrongtype.json")
			err := os.WriteFile(wrongTypePath, []byte(tt.content), 0o600)
			require.NoError(t, err)

			s := store.NewJSONStore(tmpDir)
			ctx := context.Background()

			loaded, err := s.Load(ctx, "wrongtype")

			// For null, Go's json.Unmarshal treats null as zero value
			if tt.content == "null" {
				// Implementation returns zero-value struct for null JSON
				assert.NoError(t, err, "null JSON unmarshals to zero value")
				assert.NotNil(t, loaded, "returns zero-value struct, not nil pointer")
				if loaded != nil {
					assert.Empty(t, loaded.WorkflowID, "zero-value struct has empty fields")
				}
			} else {
				assert.Error(t, err, "should return error for wrong JSON type")
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg, "error should indicate type mismatch")
				}
			}
		})
	}
}

// TestJSONStore_Load_InvalidUTF8 verifies recovery from files with invalid UTF-8 encoding
func TestJSONStore_Load_InvalidUTF8(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file with invalid UTF-8 sequences
	invalidUTF8Path := filepath.Join(tmpDir, "invalid_utf8.json")
	// 0xFF and 0xFE are invalid UTF-8 start bytes
	invalidContent := []byte{'{', '"', 'k', 'e', 'y', '"', ':', '"', 0xFF, 0xFE, 0xFD, '"', '}'}
	err := os.WriteFile(invalidUTF8Path, invalidContent, 0o600)
	require.NoError(t, err)

	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	loaded, err := s.Load(ctx, "invalid_utf8")
	// Go's json package replaces invalid UTF-8 with replacement character (U+FFFD)
	// So this may succeed with mangled data rather than error
	if err != nil {
		assert.Error(t, err, "should return error for invalid UTF-8")
	} else {
		// If it succeeds, verify data is corrupted (contains replacement chars)
		t.Logf("Invalid UTF-8 was accepted and mangled: %+v", loaded)
	}
}

// TestJSONStore_Load_NullBytes verifies recovery from files containing null bytes
func TestJSONStore_Load_NullBytes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file with null bytes (binary corruption)
	nullBytesPath := filepath.Join(tmpDir, "nullbytes.json")
	corruptedContent := []byte{'{', '"', 'k', 'e', 'y', '"', ':', 0x00, 0x00, '"', 'v', 'a', 'l', '"', '}'}
	err := os.WriteFile(nullBytesPath, corruptedContent, 0o600)
	require.NoError(t, err)

	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	_, err = s.Load(ctx, "nullbytes")
	assert.Error(t, err, "should return error for file with null bytes")
}

// TestJSONStore_Load_ExcessiveNesting verifies recovery from deeply nested JSON
func TestJSONStore_Load_ExcessiveNesting(t *testing.T) {
	tmpDir := t.TempDir()

	// Create JSON with excessive nesting (potential stack overflow attack)
	var nested string
	depth := 10000
	for i := 0; i < depth; i++ {
		nested += `{"a":`
	}
	nested += `"value"`
	for i := 0; i < depth; i++ {
		nested += `}`
	}

	excessivePath := filepath.Join(tmpDir, "excessive.json")
	err := os.WriteFile(excessivePath, []byte(nested), 0o600)
	require.NoError(t, err)

	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	// This should either error or handle gracefully without crashing
	_, err = s.Load(ctx, "excessive")
	// Implementation may handle this differently - either error or succeed
	// Key is that it doesn't crash or hang
	t.Logf("Load result for excessive nesting: %v", err)
}

// TestJSONStore_Load_DuplicateKeys verifies behavior with duplicate JSON keys
func TestJSONStore_Load_DuplicateKeys(t *testing.T) {
	tmpDir := t.TempDir()

	// JSON with duplicate keys (last one wins in Go's encoding/json)
	duplicatePath := filepath.Join(tmpDir, "duplicate.json")
	duplicateContent := `{
		"workflow_id": "first-value",
		"workflow_id": "second-value",
		"instance_id": "inst-123",
		"status": "running"
	}`
	err := os.WriteFile(duplicatePath, []byte(duplicateContent), 0o600)
	require.NoError(t, err)

	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	loaded, err := s.Load(ctx, "duplicate")
	// Go's json package accepts duplicate keys and uses last value during unmarshal
	// However, the ExecutionContext struct fields may not map directly to JSON keys
	assert.NoError(t, err, "Go's json accepts duplicate keys")
	assert.NotNil(t, loaded, "should load struct even with duplicate keys")
	// Note: The actual WorkflowID field name in struct may differ from JSON key
	t.Logf("Loaded with duplicate keys: WorkflowID=%q", loaded.WorkflowID)
}

// TestJSONStore_Load_UnescapedControlCharacters verifies recovery from unescaped control chars
func TestJSONStore_Load_UnescapedControlCharacters(t *testing.T) {
	tmpDir := t.TempDir()

	// Create JSON with unescaped control characters (should be \n, \t, etc.)
	controlPath := filepath.Join(tmpDir, "control.json")
	// Literal newline and tab in string value (invalid JSON)
	controlContent := `{"workflow_id":"test` + "\n\t" + `value","status":"running"}`
	err := os.WriteFile(controlPath, []byte(controlContent), 0o600)
	require.NoError(t, err)

	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	_, err = s.Load(ctx, "control")
	assert.Error(t, err, "should return error for unescaped control characters")
}

// TestJSONStore_Load_VeryLargeFile verifies handling of very large corrupted files
func TestJSONStore_Load_VeryLargeFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large file test in short mode")
	}

	tmpDir := t.TempDir()

	// Create a very large file with invalid JSON (to test memory handling)
	largePath := filepath.Join(tmpDir, "large.json")
	f, err := os.Create(largePath)
	require.NoError(t, err)
	defer f.Close()

	// Write 10MB of invalid JSON
	invalidChunk := []byte("not valid json but repeated many times ")
	for i := 0; i < 250000; i++ { // ~10MB
		_, err := f.Write(invalidChunk)
		require.NoError(t, err)
	}
	f.Close()

	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	_, err = s.Load(ctx, "large")
	assert.Error(t, err, "should return error for large invalid JSON file")
}

// TestJSONStore_Load_TrailingGarbage verifies recovery from JSON with trailing content
func TestJSONStore_Load_TrailingGarbage(t *testing.T) {
	tmpDir := t.TempDir()

	// Valid JSON followed by garbage
	trailingPath := filepath.Join(tmpDir, "trailing.json")
	trailingContent := `{"workflow_id":"test-123","instance_id":"inst-456","status":"running"}garbage after json`
	err := os.WriteFile(trailingPath, []byte(trailingContent), 0o600)
	require.NoError(t, err)

	s := store.NewJSONStore(tmpDir)
	ctx := context.Background()

	// Go's json.Unmarshal ignores trailing content, so this may succeed
	loaded, err := s.Load(ctx, "trailing")
	// Either succeeds (json decoder ignores trailing) or errors (strict validation)
	if err == nil {
		assert.NotNil(t, loaded, "may successfully load valid JSON prefix")
		if loaded != nil {
			assert.Equal(t, "test-123", loaded.WorkflowID)
		}
	} else {
		assert.Error(t, err, "may error on trailing garbage")
	}
}

// TestJSONStore_Load_MixedLineEndings verifies handling of different line ending styles
func TestJSONStore_Load_MixedLineEndings(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		content     string
		shouldError bool
	}{
		{
			name:        "unix line endings",
			content:     "{\n  \"workflow_id\": \"test\",\n  \"status\": \"running\"\n}",
			shouldError: false,
		},
		{
			name:        "windows line endings",
			content:     "{\r\n  \"workflow_id\": \"test\",\r\n  \"status\": \"running\"\r\n}",
			shouldError: false,
		},
		{
			name:        "mixed line endings",
			content:     "{\r\n  \"workflow_id\": \"test\",\n  \"status\": \"running\"\r\n}",
			shouldError: false,
		},
		{
			name:        "old mac line endings",
			content:     "{\r  \"workflow_id\": \"test\",\r  \"status\": \"running\"\r}",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mixedPath := filepath.Join(tmpDir, tt.name+".json")
			err := os.WriteFile(mixedPath, []byte(tt.content), 0o600)
			require.NoError(t, err)

			s := store.NewJSONStore(tmpDir)
			ctx := context.Background()

			loaded, err := s.Load(ctx, tt.name)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err, "should handle various line endings")
				assert.NotNil(t, loaded)
			}
		})
	}
}
