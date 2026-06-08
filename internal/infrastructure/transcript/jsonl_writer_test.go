package transcript_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/transcript"
	infra "github.com/awf-project/cli/internal/infrastructure/transcript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestJSONLWriter_CreatesFileAt0600(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "transcript.jsonl")

	writer, err := infra.NewJSONLWriter(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })

	info, err := os.Stat(path)
	require.NoError(t, err)

	perm := info.Mode().Perm()
	assert.Equal(t, os.FileMode(0o600), perm, "file should have 0o600 permissions")
}

func TestJSONLWriter_CreatesParentDirs(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nested", "deep", "path", "transcript.jsonl")

	writer, err := infra.NewJSONLWriter(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })

	_, err = os.Stat(path)
	assert.NoError(t, err, "file should be created with parent directories")

	parentDir := filepath.Dir(path)
	info, err := os.Stat(parentDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "parent directory should be created")
}

func TestJSONLWriter_AppendsNewlineDelimitedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "transcript.jsonl")

	writer, err := infra.NewJSONLWriter(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })

	ctx := context.Background()
	events := make([]transcript.ExchangeEvent, 5)
	for i := range 5 {
		events[i] = transcript.ExchangeEvent{
			Seq:       uint64(i),
			RunID:     fmt.Sprintf("run-%d", i),
			Type:      transcript.EventTypeRunStarted,
			Path:      "/test/path",
			Iteration: i,
			Timestamp: time.Now(),
			Payload:   nil,
		}
		err := writer.Write(ctx, events[i])
		require.NoError(t, err)
	}

	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		line := scanner.Text()
		require.NotEmpty(t, line, "line should not be empty")

		var event transcript.ExchangeEvent
		err := json.Unmarshal([]byte(line), &event)
		require.NoError(t, err, "line %d should be valid JSON", lineNum)
		assert.Equal(t, events[lineNum].Seq, event.Seq)
		assert.Equal(t, events[lineNum].RunID, event.RunID)

		lineNum++
	}
	require.NoError(t, scanner.Err())
	assert.Equal(t, 5, lineNum, "should have written exactly 5 lines")
}

func TestJSONLWriter_IdempotentClose(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "transcript.jsonl")

	writer, err := infra.NewJSONLWriter(path)
	require.NoError(t, err)

	err1 := writer.Close()
	assert.NoError(t, err1, "first Close should return nil")

	err2 := writer.Close()
	assert.NoError(t, err2, "second Close should return nil (idempotent)")
}

func TestJSONLWriter_ConcurrentWritesSerialized(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "transcript.jsonl")

	writer, err := infra.NewJSONLWriter(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })

	const numGoroutines = 64
	const numWrites = 100
	const totalWrites = numGoroutines * numWrites

	ctx := context.Background()
	eg := errgroup.Group{}

	for g := range numGoroutines {
		goroutineID := g
		eg.Go(func() error {
			for w := range numWrites {
				event := transcript.ExchangeEvent{
					Seq:       uint64(goroutineID*numWrites + w), //nolint:gosec // G115: controlled test input (max 6399, well within uint64)
					RunID:     "concurrent-test",
					Type:      transcript.EventTypeRunStarted,
					Path:      fmt.Sprintf("/test/goroutine-%d", goroutineID),
					Iteration: w,
					Timestamp: time.Now(),
					Payload:   nil,
				}
				if err := writer.Write(ctx, event); err != nil {
					return fmt.Errorf("write error: %w", err)
				}
			}
			return nil
		})
	}
	require.NoError(t, eg.Wait())

	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		var event transcript.ExchangeEvent
		err := json.Unmarshal([]byte(line), &event)
		require.NoError(t, err, "line %d should be valid JSON (no torn writes)", lineCount)

		lineCount++
	}
	require.NoError(t, scanner.Err())
	assert.Equal(t, totalWrites, lineCount, "should have written all %d events without tearing", totalWrites)
}

func TestJSONLWriter_PayloadBeyondPipeBuf(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "transcript.jsonl")

	writer, err := infra.NewJSONLWriter(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })

	largePayload := strings.Repeat("x", 16*1024)

	ctx := context.Background()
	event := transcript.ExchangeEvent{
		Seq:       1,
		RunID:     "large-payload-test",
		Type:      transcript.EventTypeRunStarted,
		Path:      "/test/path",
		Iteration: 0,
		Timestamp: time.Now(),
		Payload:   largePayload,
	}
	err = writer.Write(ctx, event)
	require.NoError(t, err)

	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		var readEvent transcript.ExchangeEvent
		err := json.Unmarshal([]byte(line), &readEvent)
		require.NoError(t, err, "large payload should unmarshal as single line")

		payload, ok := readEvent.Payload.(string)
		require.True(t, ok, "payload should be a string")
		assert.Equal(t, largePayload, payload, "payload should round-trip correctly")

		lineCount++
	}
	require.NoError(t, scanner.Err())
	assert.Equal(t, 1, lineCount, "large payload should result in exactly one line")
}

func TestJSONLWriter_ContextCancelled(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "transcript.jsonl")

	writer, err := infra.NewJSONLWriter(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event := transcript.ExchangeEvent{
		Seq:       1,
		RunID:     "cancelled-test",
		Type:      transcript.EventTypeRunStarted,
		Path:      "/test/path",
		Iteration: 0,
		Timestamp: time.Now(),
		Payload:   nil,
	}

	err = writer.Write(ctx, event)
	assert.ErrorIs(t, err, context.Canceled, "Write should return context.Canceled")

	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}
	require.NoError(t, scanner.Err())
	assert.Equal(t, 0, lineCount, "cancelled context should prevent write")
}

func TestJSONLWriter_ContextDeadlineExceeded(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "transcript.jsonl")

	writer, err := infra.NewJSONLWriter(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	event := transcript.ExchangeEvent{
		Seq:       1,
		RunID:     "deadline-test",
		Type:      transcript.EventTypeRunStarted,
		Path:      "/test/path",
		Iteration: 0,
		Timestamp: time.Now(),
		Payload:   nil,
	}

	err = writer.Write(ctx, event)
	assert.ErrorIs(t, err, context.DeadlineExceeded, "Write should return context.DeadlineExceeded")

	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}
	require.NoError(t, scanner.Err())
	assert.Equal(t, 0, lineCount, "deadline exceeded should prevent write")
}

func TestJSONLWriter_MultipleWritesSequential(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "transcript.jsonl")

	writer, err := infra.NewJSONLWriter(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })

	ctx := context.Background()
	event1 := transcript.ExchangeEvent{
		Seq:       1,
		RunID:     "run-1",
		Type:      transcript.EventTypeRunStarted,
		Path:      "/test/path",
		Iteration: 0,
		Timestamp: time.Now(),
		Payload:   "payload1",
	}

	event2 := transcript.ExchangeEvent{
		Seq:       2,
		RunID:     "run-1",
		Type:      transcript.EventTypeRunCompleted,
		Path:      "/test/path",
		Iteration: 0,
		Timestamp: time.Now(),
		Payload:   "payload2",
	}

	err = writer.Write(ctx, event1)
	require.NoError(t, err)

	err = writer.Write(ctx, event2)
	require.NoError(t, err)

	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	require.NoError(t, scanner.Err())

	require.Len(t, lines, 2)

	var readEvent1, readEvent2 transcript.ExchangeEvent
	err = json.Unmarshal([]byte(lines[0]), &readEvent1)
	require.NoError(t, err)
	err = json.Unmarshal([]byte(lines[1]), &readEvent2)
	require.NoError(t, err)

	assert.Equal(t, event1.Seq, readEvent1.Seq)
	assert.Equal(t, event2.Seq, readEvent2.Seq)
}

func TestJSONLWriter_FileModeWithDifferentPath(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "custom_name.jsonl")

	writer, err := infra.NewJSONLWriter(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })

	info, err := os.Stat(path)
	require.NoError(t, err)

	perm := info.Mode().Perm()
	assert.Equal(t, os.FileMode(0o600), perm, "custom named file should also have 0o600 permissions")
}

func TestJSONLWriter_ParentDirsWithMode(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "level1", "level2", "level3", "transcript.jsonl")

	writer, err := infra.NewJSONLWriter(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })

	parentDir := filepath.Join(tmpDir, "level1", "level2", "level3")
	info, err := os.Stat(parentDir)
	require.NoError(t, err)

	parentPerm := info.Mode().Perm()
	assert.Equal(t, os.FileMode(0o700), parentPerm, "parent directory should have 0o700 permissions")
}

func TestJSONLWriter_WriteWithValidContext(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "transcript.jsonl")

	writer, err := infra.NewJSONLWriter(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	event := transcript.ExchangeEvent{
		Seq:       1,
		RunID:     "test-run",
		Type:      transcript.EventTypeRunStarted,
		Path:      "/test/path",
		Iteration: 0,
		Timestamp: time.Now(),
		Payload:   "test",
	}

	err = writer.Write(ctx, event)
	assert.NoError(t, err, "write should succeed with valid context")

	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	require.True(t, scanner.Scan())

	var readEvent transcript.ExchangeEvent
	err = json.Unmarshal([]byte(scanner.Text()), &readEvent)
	require.NoError(t, err)
	assert.Equal(t, event.Seq, readEvent.Seq)
}
