package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// parseLine tests
// ---------------------------------------------------------------------------

func TestParseLine_WorkflowStarted(t *testing.T) {
	data := []byte(`{"event":"workflow.started","execution_id":"abc-123","timestamp":"2026-01-01T10:00:00+01:00","user":"pocky","workflow_name":"deploy","schema_version":1}`)
	entry, err := parseLine(data)
	require.NoError(t, err)
	assert.Equal(t, "2026-01-01T10:00:00+01:00", entry.Timestamp)
	assert.Equal(t, "workflow.started", entry.Event)
	assert.Equal(t, "deploy", entry.WorkflowName)
	assert.Equal(t, "abc-123", entry.ExecutionID)
	assert.Equal(t, "pocky", entry.User)
	assert.NotNil(t, entry.Fields)
}

func TestParseLine_WorkflowCompleted(t *testing.T) {
	data := []byte(`{"event":"workflow.completed","execution_id":"abc-123","timestamp":"2026-01-01T10:00:05+01:00","user":"pocky","workflow_name":"deploy","status":"success","exit_code":0,"duration_ms":5000,"schema_version":1}`)
	entry, err := parseLine(data)
	require.NoError(t, err)
	assert.Equal(t, "workflow.completed", entry.Event)
	assert.Equal(t, "success", entry.Status)
	assert.InDelta(t, 5000.0, entry.DurationMs, 0.001)
}

func TestParseLine_WorkflowCompletedWithError(t *testing.T) {
	data := []byte(`{"event":"workflow.completed","execution_id":"abc-123","timestamp":"2026-01-01T10:00:05+01:00","workflow_name":"deploy","status":"success","duration_ms":100,"error":"step failed: exit code 1","schema_version":1}`)
	entry, err := parseLine(data)
	require.NoError(t, err)
	assert.Equal(t, "step failed: exit code 1", entry.Error)
}

func TestParseLine_InvalidJSON(t *testing.T) {
	data := []byte(`{"event":"workflow.started","timestamp":"2026-01-01"`) // missing closing brace
	_, err := parseLine(data)
	assert.Error(t, err)
}

func TestParseLine_EmptyObject(t *testing.T) {
	data := []byte(`{}`)
	entry, err := parseLine(data)
	require.NoError(t, err)
	assert.Equal(t, "", entry.Timestamp)
	assert.Equal(t, "", entry.Event)
	assert.Equal(t, "", entry.WorkflowName)
}

func TestParseLine_ExtraFields(t *testing.T) {
	data := []byte(`{"event":"workflow.started","timestamp":"2026-01-01T10:00:00Z","workflow_name":"test","custom_field":"value","code":42}`)
	entry, err := parseLine(data)
	require.NoError(t, err)
	assert.Equal(t, "value", entry.Fields["custom_field"])
	assert.InDelta(t, float64(42), entry.Fields["code"], 0.001)
}

func TestParseLine_NotAnObject(t *testing.T) {
	_, err := parseLine([]byte(`["array","value"]`))
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Tailer tests
// ---------------------------------------------------------------------------

func TestNewTailer(t *testing.T) {
	path := "/tmp/test.jsonl"
	tailer := NewTailer(path)

	require.NotNil(t, tailer)
	assert.Equal(t, path, tailer.path)
	assert.Equal(t, int64(0), tailer.offset)
	assert.False(t, tailer.seeded)
}

func TestTailer_Tail_LoadsLastEntries(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.jsonl")

	line1 := `{"event":"workflow.started","timestamp":"2026-01-01T10:00:00Z","workflow_name":"deploy"}` + "\n"
	line2 := `{"event":"workflow.completed","timestamp":"2026-01-01T10:00:05Z","workflow_name":"deploy","status":"success"}` + "\n"
	err := os.WriteFile(logFile, []byte(line1+line2), 0o644)
	require.NoError(t, err)

	tailer := NewTailer(logFile)
	msg := tailer.Tail()()

	batch, ok := msg.(LogBatchMsg)
	require.True(t, ok, "expected LogBatchMsg, got %T", msg)
	require.Len(t, batch.Entries, 2)
	assert.Equal(t, "workflow.started", batch.Entries[0].Event)
	assert.Equal(t, "workflow.completed", batch.Entries[1].Event)
	assert.True(t, tailer.seeded)
	assert.Greater(t, tailer.offset, int64(0))
}

func TestTailer_Next_FirstCallUseTail(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.jsonl")

	jsonLine := `{"event":"workflow.started","execution_id":"abc-123","timestamp":"2026-01-01T10:00:00+01:00","user":"pocky","workflow_name":"deploy"}` + "\n"
	err := os.WriteFile(logFile, []byte(jsonLine), 0o644)
	require.NoError(t, err)

	tailer := NewTailer(logFile)
	msg := tailer.Next()()

	batch, ok := msg.(LogBatchMsg)
	require.True(t, ok)
	require.Len(t, batch.Entries, 1)
	assert.Equal(t, "deploy", batch.Entries[0].WorkflowName)
	assert.True(t, tailer.seeded)
}

func TestTailer_Follow_NoNewData(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.jsonl")

	jsonLine := `{"event":"workflow.started","timestamp":"2026-01-01T10:00:00Z","workflow_name":"test"}` + "\n"
	err := os.WriteFile(logFile, []byte(jsonLine), 0o644)
	require.NoError(t, err)

	tailer := NewTailer(logFile)
	tailer.Next()() // initial tail

	msg := tailer.Follow()()
	assert.Nil(t, msg)
}

func TestTailer_Follow_ReadsAppendedLines(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.jsonl")

	line1 := `{"event":"workflow.started","timestamp":"2026-01-01T10:00:00Z","workflow_name":"deploy"}` + "\n"
	err := os.WriteFile(logFile, []byte(line1), 0o644)
	require.NoError(t, err)

	tailer := NewTailer(logFile)
	tailer.Next()() // initial tail

	// Append new lines.
	line2 := `{"event":"workflow.completed","timestamp":"2026-01-01T10:00:05Z","workflow_name":"deploy","status":"success","duration_ms":5000}` + "\n"
	line3 := `{"event":"workflow.started","timestamp":"2026-01-01T10:01:00Z","workflow_name":"build"}` + "\n"
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	_, err = f.WriteString(line2 + line3)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	msg := tailer.Follow()()
	batch, ok := msg.(LogBatchMsg)
	require.True(t, ok)
	require.Len(t, batch.Entries, 2)
	assert.Equal(t, "workflow.completed", batch.Entries[0].Event)
	assert.Equal(t, "workflow.started", batch.Entries[1].Event)
}

func TestTailer_Follow_FileNotFound(t *testing.T) {
	tailer := &Tailer{path: "/nonexistent/path/file.jsonl", seeded: true}

	msg := tailer.Follow()()

	rotMsg, ok := msg.(logRotationMsg)
	require.True(t, ok, "expected logRotationMsg, got %T", msg)
	assert.Equal(t, "/nonexistent/path/file.jsonl", rotMsg.path)
	assert.False(t, tailer.seeded)
}

func TestTailer_Tail_FileNotFound_ReturnsNil(t *testing.T) {
	tailer := NewTailer("/nonexistent/path/file.jsonl")

	msg := tailer.Tail()()
	assert.Nil(t, msg)
}

func TestTailer_Follow_MalformedJSON_Skipped(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.jsonl")

	line1 := `{"event":"workflow.started","workflow_name":"test"}` + "\n"
	err := os.WriteFile(logFile, []byte(line1), 0o644)
	require.NoError(t, err)

	tailer := NewTailer(logFile)
	tailer.Next()() // initial tail

	// Append malformed + valid line.
	bad := `{"event":"broken` + "\n"
	good := `{"event":"workflow.completed","workflow_name":"test","status":"success"}` + "\n"
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	_, err = f.WriteString(bad + good)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	msg := tailer.Follow()()
	batch, ok := msg.(LogBatchMsg)
	require.True(t, ok)
	require.Len(t, batch.Entries, 1)
	assert.Equal(t, "workflow.completed", batch.Entries[0].Event)
}

func TestTailer_Tail_AllFieldsParsed(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.jsonl")

	jsonLine := `{"event":"workflow.completed","execution_id":"abc-123","timestamp":"2026-01-01T10:00:05Z","user":"pocky","workflow_name":"deploy","status":"success","duration_ms":5000}` + "\n"
	err := os.WriteFile(logFile, []byte(jsonLine), 0o644)
	require.NoError(t, err)

	tailer := NewTailer(logFile)
	msg := tailer.Tail()()

	batch, ok := msg.(LogBatchMsg)
	require.True(t, ok)
	require.Len(t, batch.Entries, 1)
	e := batch.Entries[0]
	assert.Equal(t, "2026-01-01T10:00:05Z", e.Timestamp)
	assert.Equal(t, "workflow.completed", e.Event)
	assert.Equal(t, "deploy", e.WorkflowName)
	assert.Equal(t, "abc-123", e.ExecutionID)
	assert.Equal(t, "pocky", e.User)
	assert.Equal(t, "success", e.Status)
	assert.InDelta(t, 5000.0, e.DurationMs, 0.001)
}
