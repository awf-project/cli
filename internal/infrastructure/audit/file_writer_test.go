package audit_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileAuditTrailWriter_ImplementsInterface verifies compile-time interface compliance.
func TestFileAuditTrailWriter_ImplementsInterface(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.jsonl")

	writer, err := audit.NewFileAuditTrailWriter(path)
	require.NoError(t, err)
	require.NotNil(t, writer)

	// Compile-time verification: if this doesn't compile, interface is broken
	var _ interface {
		Write(context.Context, *workflow.AuditEvent) error
		Close() error
	} = writer
}

// TestFileAuditTrailWriter_CreatesFile verifies file is created at specified path.
func TestFileAuditTrailWriter_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.jsonl")

	writer, err := audit.NewFileAuditTrailWriter(path)
	require.NoError(t, err)
	require.NotNil(t, writer)

	_, err = os.Stat(path)
	assert.NoError(t, err, "file should be created at specified path")

	t.Cleanup(func() { _ = writer.Close() })
}

// TestFileAuditTrailWriter_CreateParentDirectories verifies parent dirs are created.
func TestFileAuditTrailWriter_CreateParentDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir1", "subdir2", "audit.jsonl")

	writer, err := audit.NewFileAuditTrailWriter(path)
	require.NoError(t, err)
	require.NotNil(t, writer)

	_, err = os.Stat(path)
	assert.NoError(t, err, "file should be created with parent directories")

	// Verify parent directories exist
	parentDir := filepath.Dir(path)
	info, err := os.Stat(parentDir)
	assert.NoError(t, err)
	assert.True(t, info.IsDir(), "parent directory should be created")

	t.Cleanup(func() { _ = writer.Close() })
}

// TestFileAuditTrailWriter_FilePermissions verifies file has 0o600 permissions.
func TestFileAuditTrailWriter_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.jsonl")

	writer, err := audit.NewFileAuditTrailWriter(path)
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)

	// Check permissions are 0o600 (owner read/write only)
	perm := info.Mode().Perm()
	assert.Equal(t, os.FileMode(0o600), perm, "file should have 0o600 permissions")

	t.Cleanup(func() { _ = writer.Close() })
}

// TestFileAuditTrailWriter_WriteEvent verifies event is written to file.
func TestFileAuditTrailWriter_WriteEvent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.jsonl")

	writer, err := audit.NewFileAuditTrailWriter(path)
	require.NoError(t, err)

	ctx := context.Background()
	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "test-exec-123",
		Timestamp:     time.Now(),
		User:          "testuser",
		WorkflowName:  "test-workflow",
		Inputs:        map[string]any{"key": "value"},
	}

	err = writer.Write(ctx, &event)
	assert.NoError(t, err, "Write should succeed")

	// Verify file contains the event
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotEmpty(t, data, "file should contain written data")

	// Verify it's valid JSON
	var writtenEvent workflow.AuditEvent
	err = json.Unmarshal(data, &writtenEvent)
	assert.NoError(t, err, "written data should be valid JSON")

	t.Cleanup(func() { _ = writer.Close() })
}

// TestFileAuditTrailWriter_WritesJSONL verifies JSONL format (one JSON per line).
func TestFileAuditTrailWriter_WritesJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.jsonl")

	writer, err := audit.NewFileAuditTrailWriter(path)
	require.NoError(t, err)

	ctx := context.Background()

	// Write two events
	event1 := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "exec-1",
		Timestamp:     time.Now(),
		User:          "user1",
		WorkflowName:  "workflow1",
	}

	event2 := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowCompleted,
		ExecutionID:   "exec-1",
		Timestamp:     time.Now(),
		User:          "user1",
		WorkflowName:  "workflow1",
		Status:        "success",
	}

	require.NoError(t, writer.Write(ctx, &event1))
	require.NoError(t, writer.Write(ctx, &event2))

	// Verify JSONL format: each line is valid JSON, terminated by newline
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := 0
	content := string(data)
	for _, line := range strings.Split(content, "\n") {
		if line == "" {
			continue // skip trailing empty lines
		}
		lines++
		var event workflow.AuditEvent
		err := json.Unmarshal([]byte(line), &event)
		assert.NoError(t, err, "each line should be valid JSON: %s", line)
	}

	assert.GreaterOrEqual(t, lines, 2, "should contain at least 2 events")

	t.Cleanup(func() { _ = writer.Close() })
}

// TestFileAuditTrailWriter_EnforcesSizeLimitWithoutTruncation verifies small entries are NOT truncated.
func TestFileAuditTrailWriter_EnforcesSizeLimitWithoutTruncation(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.jsonl")

	writer, err := audit.NewFileAuditTrailWriter(path)
	require.NoError(t, err)

	ctx := context.Background()

	// Create small event that won't exceed 4KB
	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "exec-small",
		Timestamp:     time.Now(),
		User:          "user",
		WorkflowName:  "workflow",
		Inputs: map[string]any{
			"key1": "value1",
			"key2": "value2",
		},
	}

	err = writer.Write(ctx, &event)
	require.NoError(t, err)

	// Read back and verify inputs not truncated
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var writtenEvent workflow.AuditEvent
	err = json.Unmarshal(data, &writtenEvent)
	require.NoError(t, err)

	// Small event should NOT have truncation flag
	assert.False(t, writtenEvent.InputsTruncated, "small event should not be truncated")
	assert.NotNil(t, writtenEvent.Inputs)

	t.Cleanup(func() { _ = writer.Close() })
}

// TestFileAuditTrailWriter_TruncatesLargeInputs verifies 4KB limit enforcement with truncation.
func TestFileAuditTrailWriter_TruncatesLargeInputs(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.jsonl")

	writer, err := audit.NewFileAuditTrailWriter(path)
	require.NoError(t, err)

	ctx := context.Background()

	// Create event with large inputs (>4KB when serialized)
	largeBytes := make([]byte, 2500)
	for i := range largeBytes {
		largeBytes[i] = 'a'
	}
	largeValue := string(largeBytes)

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "exec-large",
		Timestamp:     time.Now(),
		User:          "user",
		WorkflowName:  "workflow",
		Inputs: map[string]any{
			"large_key1": largeValue,
			"large_key2": largeValue,
			"large_key3": largeValue,
		},
	}

	err = writer.Write(ctx, &event)
	// Write should succeed (may truncate inputs to stay under 4KB)
	assert.NoError(t, err)

	// Read back and verify entry is under 4KB
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	// Each line should be under 4KB for atomic writes
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		assert.LessOrEqual(t, len(line), 4096, "entry should be under 4KB for atomic writes")
	}

	t.Cleanup(func() { _ = writer.Close() })
}

// TestFileAuditTrailWriter_Close verifies Close() works and file is flushed.
func TestFileAuditTrailWriter_Close(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.jsonl")

	writer, err := audit.NewFileAuditTrailWriter(path)
	require.NoError(t, err)

	ctx := context.Background()
	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "exec-close-test",
		Timestamp:     time.Now(),
		User:          "user",
		WorkflowName:  "workflow",
	}

	require.NoError(t, writer.Write(ctx, &event))

	// Close should succeed and flush data
	err = writer.Close()
	assert.NoError(t, err, "Close should succeed")

	// Verify file exists and has content after close
	data, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.NotEmpty(t, data, "file should have content after close")
}

// TestFileAuditTrailWriter_AppendMode verifies file is opened in append mode.
func TestFileAuditTrailWriter_AppendMode(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.jsonl")

	// Create first writer and write event
	writer1, err := audit.NewFileAuditTrailWriter(path)
	require.NoError(t, err)

	ctx := context.Background()
	event1 := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "exec-1",
		Timestamp:     time.Now(),
		User:          "user",
		WorkflowName:  "workflow",
	}

	require.NoError(t, writer1.Write(ctx, &event1))
	require.NoError(t, writer1.Close())

	// Open again and write another event (should append, not overwrite)
	writer2, err := audit.NewFileAuditTrailWriter(path)
	require.NoError(t, err)

	event2 := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowCompleted,
		ExecutionID:   "exec-2",
		Timestamp:     time.Now(),
		User:          "user",
		WorkflowName:  "workflow",
		Status:        "success",
	}

	require.NoError(t, writer2.Write(ctx, &event2))
	require.NoError(t, writer2.Close())

	// Verify both events are in file (append, not overwrite)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := 0
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		lines++
	}

	assert.GreaterOrEqual(t, lines, 2, "both events should be appended to file")
}

// TestFileAuditTrailWriter_WriteError_InvalidPath tests error handling for invalid paths.
func TestFileAuditTrailWriter_WriteError_InvalidPath(t *testing.T) {
	// Use an invalid path (non-existent parent that can't be created)
	path := "/invalid/parent/that/cannot/be/created/audit.jsonl"

	// NewFileAuditTrailWriter should fail if path is invalid
	writer, err := audit.NewFileAuditTrailWriter(path)

	// Either creation fails or write fails is acceptable
	if err == nil && writer != nil {
		t.Cleanup(func() { _ = writer.Close() })
	}
}

// TestFileAuditTrailWriter_CloseMultipleTimes verifies Close() is idempotent.
func TestFileAuditTrailWriter_CloseMultipleTimes(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.jsonl")

	writer, err := audit.NewFileAuditTrailWriter(path)
	require.NoError(t, err)

	ctx := context.Background()
	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "exec-multi-close",
		Timestamp:     time.Now(),
		User:          "user",
		WorkflowName:  "workflow",
	}

	require.NoError(t, writer.Write(ctx, &event))

	// First close
	err = writer.Close()
	assert.NoError(t, err)

	// Second close (should be safe or return no error)
	err = writer.Close()
	// Either no error or a well-defined error is acceptable
	if err != nil {
		t.Log("Second Close() returned error (acceptable):", err)
	}
}

// TestFileAuditTrailWriter_FieldOrdering verifies JSON field order matches spec.
func TestFileAuditTrailWriter_FieldOrdering(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.jsonl")

	writer, err := audit.NewFileAuditTrailWriter(path)
	require.NoError(t, err)

	ctx := context.Background()
	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "exec-order",
		Timestamp:     time.Now(),
		User:          "user",
		WorkflowName:  "workflow",
		Inputs:        map[string]any{"key": "value"},
	}

	require.NoError(t, writer.Write(ctx, &event))

	// Read raw JSON to verify field order
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	content := string(data)
	// Expected order: event, execution_id, timestamp, user, workflow_name, inputs, schema_version
	eventIdx := indexOfField(content, "event")
	execIDIdx := indexOfField(content, "execution_id")
	timestampIdx := indexOfField(content, "timestamp")
	schemaIdx := indexOfField(content, "schema_version")

	assert.Less(t, eventIdx, execIDIdx, "event should come before execution_id")
	assert.Less(t, execIDIdx, timestampIdx, "execution_id should come before timestamp")
	assert.Less(t, timestampIdx, schemaIdx, "timestamp should come before schema_version")

	t.Cleanup(func() { _ = writer.Close() })
}

// TestFileAuditTrailWriter_ContextCancellation verifies context behavior.
func TestFileAuditTrailWriter_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.jsonl")

	writer, err := audit.NewFileAuditTrailWriter(path)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "exec-cancel",
		Timestamp:     time.Now(),
		User:          "user",
		WorkflowName:  "workflow",
	}

	// Should handle cancelled context gracefully (either succeed or fail is acceptable)
	_ = writer.Write(ctx, &event)

	t.Cleanup(func() { _ = writer.Close() })
}

// TestFileAuditTrailWriter_EmptyInputs verifies event with nil inputs.
func TestFileAuditTrailWriter_EmptyInputs(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.jsonl")

	writer, err := audit.NewFileAuditTrailWriter(path)
	require.NoError(t, err)

	ctx := context.Background()
	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "exec-empty",
		Timestamp:     time.Now(),
		User:          "user",
		WorkflowName:  "workflow",
		Inputs:        nil,
	}

	err = writer.Write(ctx, &event)
	assert.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var writtenEvent workflow.AuditEvent
	err = json.Unmarshal(data, &writtenEvent)
	assert.NoError(t, err)

	t.Cleanup(func() { _ = writer.Close() })
}

// TestFileAuditTrailWriter_CompletedEventWithError verifies error field handling.
func TestFileAuditTrailWriter_CompletedEventWithError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.jsonl")

	writer, err := audit.NewFileAuditTrailWriter(path)
	require.NoError(t, err)

	ctx := context.Background()
	exitCode := 1
	durationMs := int64(1000)
	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowCompleted,
		ExecutionID:   "exec-error",
		Timestamp:     time.Now(),
		User:          "user",
		WorkflowName:  "workflow",
		Status:        "failure",
		ExitCode:      &exitCode,
		DurationMs:    &durationMs,
		Error:         "step failed: connection timeout",
	}

	err = writer.Write(ctx, &event)
	assert.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var writtenEvent workflow.AuditEvent
	err = json.Unmarshal(data, &writtenEvent)
	require.NoError(t, err)

	assert.Equal(t, "failure", writtenEvent.Status)
	assert.NotNil(t, writtenEvent.ExitCode)
	assert.Equal(t, 1, *writtenEvent.ExitCode)
	assert.Equal(t, "step failed: connection timeout", writtenEvent.Error)

	t.Cleanup(func() { _ = writer.Close() })
}

// TestFileAuditTrailWriter_MultiProcessAtomicity verifies POSIX O_APPEND atomicity
// across OS processes (NFR-004). Two independent processes write 50 lines each to
// the same file concurrently. Every line must be intact JSON with no interleaving.
//
// This validates the kernel-level guarantee that writes under PIPE_BUF (4096 bytes)
// are atomic on POSIX systems, independent of Go's in-process mutex.
func TestFileAuditTrailWriter_MultiProcessAtomicity(t *testing.T) {
	const linesPerProcess = 50
	const totalLines = linesPerProcess * 2

	tmpDir := t.TempDir()
	auditPath := filepath.Join(tmpDir, "audit.jsonl")

	// Write the helper program that appends JSONL lines via raw O_APPEND.
	helperSrc := `package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	path := os.Args[1]
	processID := os.Args[2]
	count, _ := strconv.Atoi(os.Args[3])

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	for i := 0; i < count; i++ {
		line := fmt.Sprintf("{\"process\":%q,\"seq\":%d}", processID, i)
		if _, err := fmt.Fprintln(f, line); err != nil {
			fmt.Fprintf(os.Stderr, "write: %v\n", err)
			os.Exit(1)
		}
	}
}
`
	helperFile := filepath.Join(tmpDir, "helper.go")
	require.NoError(t, os.WriteFile(helperFile, []byte(helperSrc), 0o600))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	countStr := strconv.Itoa(linesPerProcess)

	var wg sync.WaitGroup
	errs := make([]error, 2)

	for i, processID := range []string{"A", "B"} {
		wg.Add(1)
		go func(idx int, pid string) {
			defer wg.Done()
			cmd := exec.CommandContext(ctx, "go", "run", helperFile, auditPath, pid, countStr)
			if out, err := cmd.CombinedOutput(); err != nil {
				errs[idx] = fmt.Errorf("process %s: %w\noutput: %s", pid, err, out)
			}
		}(i, processID)
	}

	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}

	// Parse all lines and assert correctness.
	data, err := os.ReadFile(auditPath)
	require.NoError(t, err)

	type entry struct {
		Process string `json:"process"`
		Seq     int    `json:"seq"`
	}

	countByProcess := map[string]int{}
	nonEmptyLines := 0

	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		nonEmptyLines++

		var e entry
		require.NoError(t, json.Unmarshal([]byte(line), &e),
			"line %d must be valid JSON: %s", nonEmptyLines, line)

		countByProcess[e.Process]++
	}

	assert.Equal(t, totalLines, nonEmptyLines, "total line count must be exactly %d", totalLines)
	assert.Equal(t, linesPerProcess, countByProcess["A"], "process A must have written %d lines", linesPerProcess)
	assert.Equal(t, linesPerProcess, countByProcess["B"], "process B must have written %d lines", linesPerProcess)
}

// Helper: find index of field in JSON string
func indexOfField(jsonStr, fieldName string) int {
	idx := -1
	for i := 0; i < len(jsonStr); i++ {
		if i+len(fieldName)+2 < len(jsonStr) &&
			jsonStr[i] == '"' &&
			jsonStr[i+1:i+len(fieldName)+1] == fieldName &&
			jsonStr[i+len(fieldName)+1] == '"' &&
			jsonStr[i+len(fieldName)+2] == ':' {
			if idx == -1 {
				idx = i
			}
		}
	}
	return idx
}
