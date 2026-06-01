package cli

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/acp"
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

func TestACPMessageSender_MapsMessageTypes(t *testing.T) {
	em := &captureEmitter{}
	s := newACPMessageSender(em, "sess_1", nil)
	cases := []struct {
		msg  acp.Message
		kind string
	}{
		{acp.Message{Type: acp.MsgAgentMessageChunk, Content: "t"}, "agent_message_chunk"},
		{acp.Message{Type: acp.MsgAgentThoughtChunk, Content: "r"}, "agent_thought_chunk"},
		{acp.Message{Type: acp.MsgToolCall, ToolID: "id1", Tool: "bash", Content: "ls"}, "tool_call"},
		{acp.Message{Type: acp.MsgToolCallUpdate, ToolID: "id1", Tool: "bash", Content: "ls"}, "tool_call_update"},
	}
	for _, tc := range cases {
		if err := s.Send(context.Background(), tc.msg); err != nil {
			t.Fatalf("send: %v", err)
		}
	}
	if len(em.calls) != len(cases) {
		t.Fatalf("expected %d emits, got %d", len(cases), len(em.calls))
	}
	for i, tc := range cases {
		if em.calls[i].kind != tc.kind {
			t.Fatalf("case %d: want kind %q got %q", i, tc.kind, em.calls[i].kind)
		}
	}
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

func TestACPSessionNotifier_MapsSessionUpdate(t *testing.T) {
	em := &captureEmitter{}
	n := newACPSessionNotifier(em, "sess_1")
	err := n.NotifySessionUpdate(context.Background(), "wf-123", acp.SessionUpdate{
		Kind: "step_started", StepName: "build",
	})
	if err != nil {
		t.Fatalf("notify: %v", err)
	}
	if len(em.calls) != 1 || em.calls[0].kind != "step_started" || em.calls[0].sessionID != "sess_1" {
		t.Fatalf("unexpected emit: %+v", em.calls)
	}
	if em.calls[0].fields["stepName"] != "build" {
		t.Fatalf("stepName not mapped: %+v", em.calls[0].fields)
	}
}
