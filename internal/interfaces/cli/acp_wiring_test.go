package cli

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// errorEmitter is a captureEmitter variant that always returns an error on EmitSessionUpdate.
// Used to exercise the missedEmits counter in acpTextWriter.
type errorEmitter struct{}

func (e *errorEmitter) EmitSessionUpdate(_ context.Context, _, _ string, _ map[string]any) error {
	return errors.New("transport error")
}

// fakeHistoryStore implements ports.HistoryStore for testing sharedHistoryStore.
type fakeHistoryStore struct {
	onClose func()
}

func (f *fakeHistoryStore) Record(_ context.Context, _ *workflow.ExecutionRecord) error {
	return nil
}

func (f *fakeHistoryStore) List(_ context.Context, _ *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error) {
	return nil, nil
}

func (f *fakeHistoryStore) GetStats(_ context.Context, _ *workflow.HistoryFilter) (*workflow.HistoryStats, error) {
	return nil, nil
}

func (f *fakeHistoryStore) Cleanup(_ context.Context, _ time.Duration) (int, error) {
	return 0, nil
}

func (f *fakeHistoryStore) Close() error {
	if f.onClose != nil {
		f.onClose()
	}
	return nil
}

var _ ports.HistoryStore = (*fakeHistoryStore)(nil)

func TestSharedHistoryStore_CloseIsNoop(t *testing.T) {
	closed := false
	inner := &fakeHistoryStore{onClose: func() { closed = true }}
	shared := sharedHistoryStore{HistoryStore: inner}
	if err := shared.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if closed {
		t.Fatal("sharedHistoryStore.Close must NOT close the underlying store (server owns lifecycle)")
	}
}

type captureEmitter struct {
	calls []capturedUpdate
}
type capturedUpdate struct {
	sessionID string
	kind      string
	fields    map[string]any
}

func (c *captureEmitter) EmitSessionUpdate(_ context.Context, sessionID, kind string, fields map[string]any) error {
	c.calls = append(c.calls, capturedUpdate{sessionID, kind, fields})
	return nil
}

func TestACPTextWriter_EmitsAgentMessageChunk(t *testing.T) {
	em := &captureEmitter{}
	w := newACPTextWriter(context.Background(), em, "sess_1", nil)
	n, err := w.Write([]byte("hello world"))
	if err != nil || n != 11 {
		t.Fatalf("write: n=%d err=%v", n, err)
	}
	if len(em.calls) != 1 || em.calls[0].kind != "agent_message_chunk" {
		t.Fatalf("expected one agent_message_chunk, got %+v", em.calls)
	}
	content, _ := em.calls[0].fields["content"].(map[string]any)
	if content["text"] != "hello world" {
		t.Fatalf("unexpected content: %+v", em.calls[0].fields)
	}
}

func TestACPTextWriter_EmptyWrite_NoEmit(t *testing.T) {
	em := &captureEmitter{}
	w := newACPTextWriter(context.Background(), em, "sess_1", nil)
	_, _ = w.Write(nil)
	_, _ = w.Write([]byte(""))
	if len(em.calls) != 0 {
		t.Fatalf("empty writes must not emit, got %+v", em.calls)
	}
}

func TestACPWiring_NoACPServerImport(t *testing.T) {
	// acp_wiring.go must not import pkg/acpserver after the SDK migration.
	// All handler adapters now use SDK types; the legacy acpserver package is removed.
	data, err := os.ReadFile("acp_wiring.go")
	require.NoError(t, err)
	content := string(data)

	assert.NotContains(t, content, "pkg/acpserver")
	assert.NotContains(t, content, "acpserver.")
}

func TestACPWiring_NoACPErrorCode(t *testing.T) {
	// acpErrorCode must be removed from acp_wiring.go after the SDK migration.
	// Call sites now use toACPError from T029 (infrastructure/acp layer); the
	// interfaces layer no longer performs kind→code mapping.
	data, err := os.ReadFile("acp_wiring.go")
	require.NoError(t, err)
	content := string(data)

	assert.NotContains(t, content, "acpErrorCode")
	assert.NotContains(t, content, "adaptACPHandler")
}

// TestACPTextWriter_MissedEmitsCounter verifies that each Write whose EmitSessionUpdate
// fails increments the missedEmits counter atomically, while the Write call still
// returns (len(p), nil) as required by the best-effort io.Writer contract.
func TestACPTextWriter_MissedEmitsCounter(t *testing.T) {
	w := newACPTextWriter(context.Background(), &errorEmitter{}, "sess_1", nil)

	for i := range 3 {
		n, err := w.Write([]byte("chunk"))
		if err != nil {
			t.Fatalf("write %d: unexpected error: %v", i, err)
		}
		if n != 5 {
			t.Fatalf("write %d: expected n=5, got %d", i, n)
		}
	}

	if got := w.MissedEmits(); got != 3 {
		t.Fatalf("MissedEmits: expected 3, got %d", got)
	}
}

// TestACPTextWriter_MissedEmits_NotIncrementedOnSuccess verifies that successful
// emissions do NOT increment the missedEmits counter.
func TestACPTextWriter_MissedEmits_NotIncrementedOnSuccess(t *testing.T) {
	em := &captureEmitter{}
	w := newACPTextWriter(context.Background(), em, "sess_1", nil)

	if _, err := w.Write([]byte("ok")); err != nil {
		t.Fatalf("write: %v", err)
	}

	if got := w.MissedEmits(); got != 0 {
		t.Fatalf("MissedEmits: expected 0 after successful emit, got %d", got)
	}
}

// TestStreamFlaggingEmitter_FlipsStreamedOnSuccess verifies the wrapper sets the shared
// streamed flag to true only when the wrapped emit succeeds, and forwards the call
// (sessionID/kind/fields) to the underlying emitter unchanged.
func TestStreamFlaggingEmitter_FlipsStreamedOnSuccess(t *testing.T) {
	em := &captureEmitter{}
	streamed := &atomic.Bool{}
	e := newStreamFlaggingEmitter(em, streamed)

	err := e.EmitSessionUpdate(context.Background(), "sess_1", "agent_message_chunk", map[string]any{"k": "v"})

	require.NoError(t, err)
	assert.True(t, streamed.Load(), "streamed must flip to true after a successful emit")
	require.Len(t, em.calls, 1, "the call must be forwarded to the underlying emitter")
	assert.Equal(t, "sess_1", em.calls[0].sessionID)
	assert.Equal(t, "agent_message_chunk", em.calls[0].kind)
}

// TestStreamFlaggingEmitter_DoesNotFlipOnError verifies that when the wrapped emitter
// returns an error, streamed stays false and the error is propagated. This guards the
// invariant that HandleSessionPrompt only suppresses its post-run aggregate when output
// was actually delivered.
func TestStreamFlaggingEmitter_DoesNotFlipOnError(t *testing.T) {
	streamed := &atomic.Bool{}
	e := newStreamFlaggingEmitter(&errorEmitter{}, streamed)

	err := e.EmitSessionUpdate(context.Background(), "sess_1", "agent_message_chunk", nil)

	require.Error(t, err)
	assert.False(t, streamed.Load(), "streamed must stay false when the emit fails")
}

// TestStreamFlaggingEmitter_NilStreamedSafe verifies a nil streamed pointer is tolerated
// (no panic) — the wrapper degrades to a transparent pass-through.
func TestStreamFlaggingEmitter_NilStreamedSafe(t *testing.T) {
	em := &captureEmitter{}
	e := newStreamFlaggingEmitter(em, nil)

	err := e.EmitSessionUpdate(context.Background(), "sess_1", "agent_message_chunk", nil)

	require.NoError(t, err)
	require.Len(t, em.calls, 1)
}
