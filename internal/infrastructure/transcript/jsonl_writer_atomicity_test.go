//go:build !race

// This test file carries the !race build tag. Spawning child processes via
// os/exec causes the race detector to emit false-positive reports: each
// subprocess opens the JSONL file independently so no Go-level race exists,
// but the detector cannot observe cross-process memory. In-process
// concurrency safety is already covered by T040's goroutine-level concurrent
// write test, which runs under -race.

package transcript

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	domaintranscript "github.com/awf-project/cli/internal/domain/transcript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	childMarkerEnv = "JSONL_WRITER_CHILD"
	childPathEnv   = "JSONL_WRITER_PATH"
	multiProcN     = 4
	multiProcK     = 500
)

// TestJSONLWriterChild is the subprocess entrypoint invoked by
// TestJSONLWriter_MultiProcessO_APPEND_Atomic via os/exec re-exec.
// Skips silently when JSONL_WRITER_CHILD is unset to prevent accidental execution.
func TestJSONLWriterChild(t *testing.T) {
	if os.Getenv(childMarkerEnv) != "1" {
		t.Skip("not a child process")
	}

	path := os.Getenv(childPathEnv)
	require.NotEmpty(t, path, "child: %s env var must be set", childPathEnv)

	pid := os.Getpid()
	writer, err := NewJSONLWriter(path)
	require.NoError(t, err)
	defer func() { _ = writer.Close() }()

	ctx := context.Background()
	for i := range multiProcK {
		event := domaintranscript.ExchangeEvent{
			Seq:       uint64(i), //nolint:gosec // G115: controlled test input (max 499, well within uint64)
			RunID:     "multiprocess-atomicity",
			Type:      domaintranscript.EventTypeStepStarted,
			Path:      "/test/multiprocess",
			Iteration: i,
			Timestamp: time.Now(),
			Payload: &domaintranscript.StepPayload{
				Name: fmt.Sprintf("pid-%d", pid),
				Kind: "atomicity-test",
			},
		}
		require.NoError(t, writer.Write(ctx, event))
	}
}

func TestJSONLWriter_MultiProcessO_APPEND_Atomic(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	errs := make([]error, multiProcN)

	for i := range multiProcN {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run", "^TestJSONLWriterChild$") //nolint:gosec // G702: os.Args[0] is the current test binary; not user-controlled input
			cmd.Env = append(
				os.Environ(),
				childMarkerEnv+"=1",
				childPathEnv+"="+transcriptPath,
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				errs[idx] = fmt.Errorf("child %d: %w\noutput: %s", idx, err, out)
			}
		}(i)
	}

	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}

	data, err := os.ReadFile(transcriptPath)
	require.NoError(t, err)

	scanner := bufio.NewScanner(bytes.NewReader(data))
	lineCount := 0
	pidCounts := map[string]int{}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		lineCount++

		var event domaintranscript.ExchangeEvent
		require.NoError(t, json.Unmarshal([]byte(line), &event),
			"line %d must be valid JSON", lineCount)

		payload, ok := event.Payload.(*domaintranscript.StepPayload)
		require.True(t, ok, "line %d payload must be *StepPayload, got %T", lineCount, event.Payload)
		pidCounts[payload.Name]++
	}
	require.NoError(t, scanner.Err())

	assert.Equal(t, multiProcN*multiProcK, lineCount,
		"total line count must be exactly %d", multiProcN*multiProcK)
	assert.Len(t, pidCounts, multiProcN,
		"all %d children must have contributed lines", multiProcN)

	for pidTag, count := range pidCounts {
		assert.Equal(t, multiProcK, count,
			"child %s must have written exactly %d lines", pidTag, multiProcK)
	}
}
