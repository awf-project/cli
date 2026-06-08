package cli_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAttachMirrorSubscriber_BadPathDoesNotBlock verifies that when the mirror file
// cannot be opened, the subscriber goroutine unsubscribes and drains its channel so the
// recorder keeps writing without blocking and Close completes (no leaked subscriber).
func TestAttachMirrorSubscriber_BadPathDoesNotBlock(t *testing.T) {
	tmpDir := t.TempDir()
	rec, cleanup, err := cli.WireTranscript("mirror-badpath", tmpDir)
	require.NoError(t, err)
	defer cleanup()

	// A mirror path inside a nonexistent directory cannot be opened.
	badPath := filepath.Join(tmpDir, "does-not-exist", "mirror.jsonl")
	mirrorCancel := cli.AttachMirrorSubscriber(rec, badPath)
	defer mirrorCancel()

	done := make(chan struct{})
	go func() {
		for range 200 {
			_ = rec.Record(context.Background(), transcript.ExchangeEvent{Type: transcript.EventTypeStepStarted})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("recording blocked — mirror subscriber with a bad path leaked its channel")
	}
}

func TestWiringTranscript_BuildsRecorderAndCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	runID := "test-run-123"

	rec, cleanup, err := cli.WireTranscript(runID, tmpDir)

	require.NoError(t, err)
	require.NotNil(t, rec, "Recorder must not be nil")
	require.NotNil(t, cleanup, "Cleanup function must not be nil")

	// Verify cleanup works without error
	cleanupErr := cleanup()
	assert.NoError(t, cleanupErr)
}

func TestWiringTranscript_FilePathUsesRunID(t *testing.T) {
	tmpDir := t.TempDir()
	runID := "test-run-456"

	rec, cleanup, err := cli.WireTranscript(runID, tmpDir)
	defer cleanup()

	require.NoError(t, err, "WireTranscript should not error")

	// Record an event to trigger file creation
	testEvent := transcript.ExchangeEvent{
		Seq:       1,
		RunID:     runID,
		Type:      transcript.EventTypeRunStarted,
		Path:      "/test",
		Iteration: 0,
		Timestamp: time.Now(),
	}
	err = rec.Record(context.Background(), testEvent)
	require.NoError(t, err, "Recording event should not error")

	// Verify the file was created at the expected path
	expectedPath := filepath.Join(tmpDir, "transcripts", runID+".jsonl")
	_, err = os.Stat(expectedPath)
	require.NoError(t, err, "Transcript file should exist at %s", expectedPath)
	require.True(t, fileExists(expectedPath), "File must exist at the expected path")
}

func TestWiringTranscript_MirrorFlagAttachesSubscriber(t *testing.T) {
	tmpDir := t.TempDir()
	runID := "test-run-789"
	mirrorPath := filepath.Join(tmpDir, "mirror.jsonl")

	// Create transcripts directory (required before recording can succeed)
	transcriptDir := filepath.Join(tmpDir, "transcripts")
	err := os.MkdirAll(transcriptDir, 0o755)
	require.NoError(t, err, "Creating transcripts directory should not error")

	rec, cleanup, err := cli.WireTranscript(runID, tmpDir)
	defer cleanup()

	require.NoError(t, err)

	// Attach mirror subscriber
	cancel := cli.AttachMirrorSubscriber(rec, mirrorPath)
	defer cancel()

	// Record a test event
	testEvent := transcript.ExchangeEvent{
		Seq:       1,
		RunID:     runID,
		Type:      transcript.EventTypeRunStarted,
		Path:      "/test",
		Iteration: 0,
		Timestamp: time.Now(),
		Payload:   nil,
	}
	err = rec.Record(context.Background(), testEvent)
	require.NoError(t, err, "Recording event should not error")

	// Wait for the subscriber goroutine to flush the event to the mirror file
	require.Eventually(t, func() bool {
		info, statErr := os.Stat(mirrorPath)
		return statErr == nil && info.Size() > 0
	}, 2*time.Second, 10*time.Millisecond, "Mirror file should be created with content")

	// Read and verify the mirror file contains the event (JSONL: one JSON object per line)
	data, err := os.ReadFile(mirrorPath)
	require.NoError(t, err, "Reading mirror file should not error")
	require.NotEmpty(t, data, "Mirror file should contain event data")

	// Parse first line from JSONL format (each line is a separate JSON object)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	require.True(t, scanner.Scan(), "Mirror file must contain at least one JSONL line")
	var recordedEvent transcript.ExchangeEvent
	err = json.Unmarshal(scanner.Bytes(), &recordedEvent)
	require.NoError(t, err, "First JSONL line should be valid JSON")

	// Verify the event matches what we recorded
	assert.Equal(t, testEvent.Seq, recordedEvent.Seq, "Event sequence should match")
	assert.Equal(t, testEvent.RunID, recordedEvent.RunID, "Event RunID should match")
	assert.Equal(t, testEvent.Type, recordedEvent.Type, "Event type should match")
}

func TestWiringTranscript_EmptyMirrorPathIsNoop(t *testing.T) {
	tmpDir := t.TempDir()

	rec, cleanup, err := cli.WireTranscript("test-run", tmpDir)
	defer cleanup()

	require.NoError(t, err)

	// Empty mirror path should be a no-op
	cancel := cli.AttachMirrorSubscriber(rec, "")

	// Calling cancel multiple times should be safe
	cancel()
	cancel()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
