//go:build integration

// Feature: F106
//
// Canonical Agent Exchange Transcript (JSONL) — end-to-end behavior:
//
//	US1 — single append-only JSONL per run, monotonic Seq, lossless round-trip
//	US4 — bounded fan-out: slow subscriber does not block disk writes
//	NFR-002 — file mode 0o600
//	NFR-005 — reader is forward-compatible with unknown event types
package features_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/transcript"
	infraTranscript "github.com/awf-project/cli/internal/infrastructure/transcript"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FR-001 + FR-002 + FR-003: canonical JSONL with monotonic Seq, 0o600 perms,
// lossless round-trip across the full payload vocabulary (StepPayload,
// MessagePayload with ContentBlocks, ToolPayload).
func TestTranscript_CanonicalLifecycle_RoundTripsLosslessly(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "run-canonical.jsonl")

	rec, err := infraTranscript.NewRecorder(transcriptPath)
	require.NoError(t, err)

	ctx := context.Background()
	runID := "run-canonical"
	ts := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

	emitted := []transcript.ExchangeEvent{
		{
			Type:      transcript.EventTypeRunStarted,
			RunID:     runID,
			Path:      "f106-canonical",
			Timestamp: ts,
			Payload:   &transcript.StepPayload{Name: "f106-canonical", Kind: "workflow"},
		},
		{
			Type:      transcript.EventTypeStepStarted,
			RunID:     runID,
			Path:      "greet",
			Timestamp: ts,
			Payload:   &transcript.StepPayload{Name: "greet", Kind: "agent"},
		},
		{
			Type:      transcript.EventTypeMessageUser,
			RunID:     runID,
			Path:      "greet",
			Timestamp: ts,
			Payload: &transcript.MessagePayload{
				Role: "user",
				Blocks: []transcript.ContentBlock{
					{Type: transcript.BlockTypeText, Text: "hello F106"},
					{Type: transcript.BlockTypeText, Text: "you are a helpful assistant"},
				},
			},
		},
		{
			Type:      transcript.EventTypeToolCall,
			RunID:     runID,
			Path:      "greet",
			Timestamp: ts,
			Payload: &transcript.ToolPayload{
				Name:     "bash",
				CallID:   "call-001",
				Input:    map[string]any{"command": "echo hi"},
				Fidelity: transcript.FidelityRouter,
			},
		},
		{
			Type:      transcript.EventTypeToolResult,
			RunID:     runID,
			Path:      "greet",
			Timestamp: ts,
			Payload: &transcript.ToolPayload{
				CallID:   "call-001",
				Output:   "hi\n",
				Fidelity: transcript.FidelityRouter,
			},
		},
		{
			Type:      transcript.EventTypeStepCompleted,
			RunID:     runID,
			Path:      "greet",
			Timestamp: ts,
			Payload:   &transcript.StepPayload{Name: "greet", Kind: "agent"},
		},
		{
			Type:      transcript.EventTypeRunCompleted,
			RunID:     runID,
			Path:      "f106-canonical",
			Timestamp: ts,
			Payload:   &transcript.StepPayload{Name: "f106-canonical", Kind: "workflow"},
		},
	}

	for i := range emitted {
		require.NoError(t, rec.Record(ctx, emitted[i]))
	}
	require.NoError(t, rec.Close())

	info, err := os.Stat(transcriptPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "transcript file must be 0o600 (NFR-002)")

	f, err := os.Open(transcriptPath)
	require.NoError(t, err)
	defer f.Close()

	read, err := infraTranscript.NewReader(f).ReadAll()
	require.NoError(t, err)
	require.Len(t, read, len(emitted), "every recorded event must be readable back")

	var prevSeq uint64
	for i, ev := range read {
		assert.Greater(t, ev.Seq, prevSeq, "Seq must be strictly monotonic (FR-003)")
		prevSeq = ev.Seq
		assert.Equal(t, emitted[i].Type, ev.Type, "event[%d] type must round-trip", i)
		assert.Equal(t, emitted[i].RunID, ev.RunID, "event[%d] run_id must round-trip", i)
		assert.Equal(t, emitted[i].Path, ev.Path, "event[%d] path must round-trip", i)
	}

	msgEvent := read[2]
	msgPayload, ok := msgEvent.Payload.(*transcript.MessagePayload)
	require.True(t, ok, "message.user payload must decode as MessagePayload")
	require.Len(t, msgPayload.Blocks, 2, "agent seam emission carries prompt + system_prompt as separate blocks (FR-005)")
	assert.Equal(t, "hello F106", msgPayload.Blocks[0].Text)
	assert.Equal(t, "you are a helpful assistant", msgPayload.Blocks[1].Text)

	toolCall, ok := read[3].Payload.(*transcript.ToolPayload)
	require.True(t, ok)
	assert.Equal(t, transcript.FidelityRouter, toolCall.Fidelity,
		"tool.call captured at router seam must carry fidelity=router (FR-008)")
}

// US1 AC3: prior events on disk remain intact across recorder restart (simulates kill + restart).
func TestTranscript_AppendOnly_PreservesPriorLines(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "run-restart.jsonl")

	first, err := infraTranscript.NewRecorder(transcriptPath)
	require.NoError(t, err)
	require.NoError(t, first.Record(context.Background(), transcript.ExchangeEvent{
		Type:      transcript.EventTypeRunStarted,
		RunID:     "run-restart",
		Path:      "f106-restart",
		Timestamp: time.Now(),
	}))
	require.NoError(t, first.Close())

	beforeRestart, err := os.ReadFile(transcriptPath)
	require.NoError(t, err)
	require.NotEmpty(t, beforeRestart)

	second, err := infraTranscript.NewRecorder(transcriptPath)
	require.NoError(t, err)
	require.NoError(t, second.Record(context.Background(), transcript.ExchangeEvent{
		Type:      transcript.EventTypeRunCompleted,
		RunID:     "run-restart",
		Path:      "f106-restart",
		Timestamp: time.Now(),
	}))
	require.NoError(t, second.Close())

	final, err := os.ReadFile(transcriptPath)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(final), string(beforeRestart)),
		"appended writes must preserve the first run's bytes verbatim")
	assert.Greater(t, len(final), len(beforeRestart), "restart must extend the file, not truncate it")
}

// US4 / SC-005: a slow subscriber consuming far below producer rate must not block disk writes.
func TestTranscript_SlowSubscriber_DoesNotBlockDiskWrites(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "run-fanout.jsonl")

	rec, err := infraTranscript.NewRecorder(transcriptPath, infraTranscript.WithFanOutBufferSize(8))
	require.NoError(t, err)
	t.Cleanup(func() { _ = rec.Close() })

	ch, cancel := rec.Subscribe()
	defer cancel()

	var received atomic.Int64
	subscriberDone := make(chan struct{})
	go func() {
		defer close(subscriberDone)
		for range ch {
			received.Add(1)
			time.Sleep(20 * time.Millisecond)
		}
	}()

	const total = 200
	writeStart := time.Now()
	ctx := context.Background()
	for i := 0; i < total; i++ {
		require.NoError(t, rec.Record(ctx, transcript.ExchangeEvent{
			Type:      transcript.EventTypeRunStarted,
			RunID:     "run-fanout",
			Path:      "f106-fanout",
			Timestamp: time.Now(),
		}))
	}
	writeElapsed := time.Since(writeStart)

	assert.Less(t, writeElapsed, 2*time.Second,
		"writer must not block on slow subscriber (US4 / SC-005); took %s", writeElapsed)

	f, err := os.Open(transcriptPath)
	require.NoError(t, err)
	defer f.Close()

	events, err := infraTranscript.NewReader(f).ReadAll()
	require.NoError(t, err)
	assert.Equal(t, total, len(events), "every Record call must hit the disk regardless of subscriber speed")

	require.NoError(t, rec.Close())
	<-subscriberDone
	assert.Less(t, received.Load(), int64(total),
		"slow subscriber must observe drops, not the full stream (drop policy active)")
}

// NFR-005: reader must tolerate unknown EventType values (forward-compat decode).
func TestTranscript_Reader_TolerantOfUnknownEventTypes(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "future.jsonl")

	knownLine, err := json.Marshal(transcript.ExchangeEvent{
		Seq:       1,
		RunID:     "run-future",
		Type:      transcript.EventTypeRunStarted,
		Path:      "f106-future",
		Timestamp: time.Now(),
	})
	require.NoError(t, err)

	unknownLine := []byte(`{"seq":2,"run_id":"run-future","type":"future.unknown.event","path":"f106-future","iteration":0,"timestamp":"2026-06-07T00:00:00Z","payload":null}`)

	contents := append(knownLine, '\n')
	contents = append(contents, unknownLine...)
	contents = append(contents, '\n')
	require.NoError(t, os.WriteFile(path, contents, 0o600))

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	events, err := infraTranscript.NewReader(f).ReadAll()
	require.NoError(t, err, "reader must accept unknown event types without error (NFR-005)")
	require.Len(t, events, 2)

	assert.Equal(t, transcript.EventTypeRunStarted, events[0].Type)
	assert.Equal(t, transcript.EventType("future.unknown.event"), events[1].Type,
		"unknown EventType must be preserved verbatim for forward-compat")
}
