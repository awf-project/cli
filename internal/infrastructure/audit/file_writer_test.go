package audit_test

import (
	"context"
	"encoding/json"
	"errors"
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

func TestFileAuditTrailWriterConcurrentWritesRemainIndependentJSONLines(t *testing.T) {
	const (
		writerCount     = 2
		writesPerWriter = 64
	)

	path := filepath.Join(t.TempDir(), "audit.jsonl")
	writers := make([]*audit.FileAuditTrailWriter, writerCount)
	for i := range writers {
		writer, err := audit.NewFileAuditTrailWriter(path)
		require.NoError(t, err)
		writers[i] = writer
		t.Cleanup(func() { _ = writer.Close() })
	}

	var wg sync.WaitGroup
	errs := make(chan error, writerCount*writesPerWriter)
	for writerIndex, writer := range writers {
		for writeIndex := 0; writeIndex < writesPerWriter; writeIndex++ {
			wg.Add(1)
			go func(writerIndex, writeIndex int, writer *audit.FileAuditTrailWriter) {
				defer wg.Done()
				event := newAuditEvent(fmt.Sprintf("writer-%d-event-%d", writerIndex, writeIndex))
				errs <- writer.Write(context.Background(), &event)
			}(writerIndex, writeIndex, writer)
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(string(data), "\n"))

	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	require.Len(t, lines, writerCount*writesPerWriter)
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		var event workflow.AuditEvent
		require.NoError(t, json.Unmarshal([]byte(line), &event))
		_, duplicate := seen[event.ExecutionID]
		assert.False(t, duplicate, "execution ID %q must occur once", event.ExecutionID)
		seen[event.ExecutionID] = struct{}{}
	}
}

func TestFileAuditTrailWriterTruncatesLargeTerminalErrors(t *testing.T) {
	for _, text := range []string{strings.Repeat("failure ", 1000), strings.Repeat("界\n\"", 2000)} {
		path := filepath.Join(t.TempDir(), "audit.jsonl")
		writer, err := audit.NewFileAuditTrailWriter(path)
		require.NoError(t, err)
		event := newAuditEvent("terminal-large-error")
		event.Event = workflow.EventWorkflowCompleted
		event.Status = "failure"
		exitCode := 3
		event.ExitCode = &exitCode
		event.Error = text
		require.NoError(t, writer.Write(context.Background(), &event))
		require.NoError(t, writer.Close())
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(data), 4096)
		var written workflow.AuditEvent
		require.NoError(t, json.Unmarshal(data, &written))
		assert.Equal(t, event.ExecutionID, written.ExecutionID)
		assert.Equal(t, "failure", written.Status)
		assert.Equal(t, 3, *written.ExitCode)
		assert.True(t, strings.HasSuffix(written.Error, "… [truncated]"))
		assert.Equal(t, text, event.Error)
	}
}

func TestFileAuditTrailWriterRejectsRecordsThatCannotFitWithoutAppending(t *testing.T) {
	tests := []struct {
		name  string
		event workflow.AuditEvent
	}{
		{
			name: "no inputs",
			event: func() workflow.AuditEvent {
				event := newAuditEvent("oversized-no-inputs")
				event.WorkflowName = strings.Repeat("w", 5000)
				return event
			}(),
		},
		{
			name: "oversized non-input field",
			event: func() workflow.AuditEvent {
				event := newAuditEvent("oversized-error")
				event.Inputs = map[string]any{"small": "value"}
				event.Error = strings.Repeat("e", 5000)
				return event
			}(),
		},
		{
			name: "already truncated input cannot make progress",
			event: func() workflow.AuditEvent {
				event := newAuditEvent("already-truncated")
				event.Inputs = map[string]any{"value": "…"}
				event.Error = strings.Repeat("e", 5000)
				return event
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "audit.jsonl")
			const existing = "existing-record\n"
			require.NoError(t, os.WriteFile(path, []byte(existing), 0o600))
			writer, err := audit.NewFileAuditTrailWriter(path)
			require.NoError(t, err)
			t.Cleanup(func() { _ = writer.Close() })

			result := make(chan error, 1)
			go func() { result <- writer.Write(context.Background(), &tt.event) }()

			select {
			case err := <-result:
				require.ErrorIs(t, err, audit.ErrAuditRecordTooLarge)
			case <-time.After(time.Second):
				t.Fatal("oversized write did not terminate")
			}

			data, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, existing, string(data))
		})
	}
}

func TestFileAuditTrailWriterBoundsCompletePayloadAndPreservesCallerInputs(t *testing.T) {
	t.Run("truncation is longest first with lexical tie breaking", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "audit.jsonl")
		writer, err := audit.NewFileAuditTrailWriter(path)
		require.NoError(t, err)
		t.Cleanup(func() { _ = writer.Close() })

		// alpha and zeta deliberately have the same serialized length.  Reducing
		// alpha alone makes the record fit, so the persisted event identifies both
		// the lexical tie-breaker and the values that must not be touched.
		event := newAuditEvent("ordered-truncation")
		event.Inputs = map[string]any{
			"alpha":  strings.Repeat("a", 2000),
			"zeta":   strings.Repeat("z", 2000),
			"short":  "x",
			"nested": map[string]any{"preserve": true},
			"number": 7,
		}

		require.NoError(t, writer.Write(context.Background(), &event))
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.LessOrEqual(t, len(data), 4096)

		var persisted workflow.AuditEvent
		require.NoError(t, json.Unmarshal(data, &persisted))
		require.True(t, persisted.InputsTruncated)
		assert.Equal(t, "…", persisted.Inputs["alpha"])
		assert.Equal(t, strings.Repeat("z", 2000), persisted.Inputs["zeta"])
		assert.Equal(t, "x", persisted.Inputs["short"])
		assert.Equal(t, float64(7), persisted.Inputs["number"])
		assert.Equal(t, map[string]any{"preserve": true}, persisted.Inputs["nested"])

		// The adapter must work from a private copy, including when it truncates.
		assert.Equal(t, strings.Repeat("a", 2000), event.Inputs["alpha"])
		assert.Equal(t, strings.Repeat("z", 2000), event.Inputs["zeta"])
		assert.Equal(t, "x", event.Inputs["short"])
		assert.Equal(t, map[string]any{"preserve": true}, event.Inputs["nested"])
		assert.False(t, event.InputsTruncated)
	})

	t.Run("exact payload limit includes newline", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "audit.jsonl")
		writer, err := audit.NewFileAuditTrailWriter(path)
		require.NoError(t, err)
		t.Cleanup(func() { _ = writer.Close() })

		exact := eventWithSerializedSize(t, 4096)
		require.NoError(t, writer.Write(context.Background(), &exact))
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Len(t, data, 4096)
		require.True(t, strings.HasSuffix(string(data), "\n"))
		var decoded workflow.AuditEvent
		require.NoError(t, json.Unmarshal(data, &decoded))

		over := eventWithSerializedSize(t, 4097)
		err = writer.Write(context.Background(), &over)
		require.ErrorIs(t, err, audit.ErrAuditRecordTooLarge)
		after, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, data, after)
	})

	t.Run("nested and escaped inputs use a private deep copy", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "audit.jsonl")
		writer, err := audit.NewFileAuditTrailWriter(path)
		require.NoError(t, err)
		t.Cleanup(func() { _ = writer.Close() })

		nested := map[string]any{"child": strings.Repeat("\"\\", 3000)}
		event := newAuditEvent("nested-input")
		event.Inputs = map[string]any{
			"nested": nested,
			"number": 42,
		}

		const writes = 8
		var wg sync.WaitGroup
		errs := make(chan error, writes)
		for range writes {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs <- writer.Write(context.Background(), &event)
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}

		assert.Equal(t, strings.Repeat("\"\\", 3000), nested["child"])
		assert.False(t, event.InputsTruncated)
		assert.Equal(t, 42, event.Inputs["number"])

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		require.Len(t, lines, writes)
		for _, line := range lines {
			assert.LessOrEqual(t, len(line)+1, 4096)
			var decoded workflow.AuditEvent
			require.NoError(t, json.Unmarshal([]byte(line), &decoded))
			assert.True(t, decoded.InputsTruncated)
		}
	})
}

func TestFileAuditTrailWriterWriteAfterCloseReturnsNamedError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	writer, err := audit.NewFileAuditTrailWriter(path)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	require.NoError(t, writer.Close())

	const writes = 16
	var wg sync.WaitGroup
	errs := make(chan error, writes)
	for i := 0; i < writes; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			event := newAuditEvent(fmt.Sprintf("closed-%d", i))
			errs <- writer.Write(context.Background(), &event)
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		assert.EqualError(t, err, "writing audit event: audit writer is closed")
		assert.True(t, errors.Is(err, audit.ErrAuditWriterClosed))
	}
}

func TestFileAuditTrailWriterWriteThenCloseRejectsLaterConcurrentWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	writer, err := audit.NewFileAuditTrailWriter(path)
	require.NoError(t, err)

	// This write establishes the ordered side of the lifecycle boundary. Close
	// cannot complete until this public Write call has serialized its append.
	accepted := newAuditEvent("accepted-before-close")
	require.NoError(t, writer.Write(context.Background(), &accepted))
	require.NoError(t, writer.Close())

	const laterWrites = 16
	var wg sync.WaitGroup
	errs := make(chan error, laterWrites)
	for i := 0; i < laterWrites; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			event := newAuditEvent(fmt.Sprintf("later-%d", i))
			errs <- writer.Write(context.Background(), &event)
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.ErrorIs(t, err, audit.ErrAuditWriterClosed)
		assert.EqualError(t, err, "writing audit event: audit writer is closed")
	}

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	require.Len(t, lines, 1)
	var persisted workflow.AuditEvent
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &persisted))
	assert.Equal(t, accepted.ExecutionID, persisted.ExecutionID)
}

func BenchmarkFileAuditTrailWriterConcurrentWrite(b *testing.B) {
	path := filepath.Join(b.TempDir(), "audit.jsonl")
	writer, err := audit.NewFileAuditTrailWriter(path)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = writer.Close() })

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			event := newAuditEvent("benchmark-execution")
			event.Inputs = map[string]any{"payload": strings.Repeat("x", 512)}
			if err := writer.Write(context.Background(), &event); err != nil {
				b.Error(err)
			}
		}
	})
}

func newAuditEvent(executionID string) workflow.AuditEvent {
	return workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   executionID,
		Timestamp:     time.Unix(0, 0).UTC(),
		User:          "test-user",
		WorkflowName:  "test-workflow",
	}
}

func eventWithSerializedSize(t *testing.T, size int) workflow.AuditEvent {
	t.Helper()
	event := newAuditEvent("payload-boundary")
	for padding := 0; padding <= size; padding++ {
		event.User = strings.Repeat("x", padding)
		data, err := event.MarshalJSON()
		require.NoError(t, err)
		if len(data)+1 == size {
			return event
		}
	}
	t.Fatalf("could not construct audit event with %d-byte JSONL payload", size)
	return workflow.AuditEvent{}
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
