package acp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/infrastructure/agents"
	infralogger "github.com/awf-project/cli/internal/infrastructure/logger"
	"github.com/awf-project/cli/pkg/display"
)

// MockMasker replaces any env value found in text with "***".
type MockMasker struct{}

func (m *MockMasker) MaskText(text string, env map[string]string) string {
	result := text
	for _, value := range env {
		if value != "" {
			result = strings.ReplaceAll(result, value, "***")
		}
	}
	return result
}

// mockSessionUpdateEmitter records session updates for testing.
type mockSessionUpdateEmitter struct {
	updates []emittedUpdate
	mu      sync.Mutex
	err     error
	errAt   int
	calls   int
}

type emittedUpdate struct {
	sessionID string
	kind      string
	fields    map[string]any
}

func (m *mockSessionUpdateEmitter) EmitSessionUpdate(_ context.Context, sessionID, kind string, fields map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.calls
	m.calls++
	m.updates = append(m.updates, emittedUpdate{sessionID, kind, fields})
	if m.err != nil && idx == m.errAt {
		return m.err
	}
	return nil
}

func (m *mockSessionUpdateEmitter) Updates() []emittedUpdate {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]emittedUpdate, len(m.updates))
	copy(cp, m.updates)
	return cp
}

// contentText extracts fields["content"].(map)["text"] for agent message/thought chunks.
func contentText(t *testing.T, fields map[string]any) string {
	t.Helper()
	content, ok := fields["content"].(map[string]any)
	require.True(t, ok, "content field must be a map")
	text, ok := content["text"].(string)
	require.True(t, ok, "content.text must be a string")
	return text
}

// rawInputText extracts fields["rawInput"].(map)["text"] for tool calls.
func rawInputText(t *testing.T, fields map[string]any) string {
	t.Helper()
	raw, ok := fields["rawInput"].(map[string]any)
	require.True(t, ok, "rawInput field must be a map")
	text, ok := raw["text"].(string)
	require.True(t, ok, "rawInput.text must be a string")
	return text
}

func newTestRenderer(sessionID, stepID string, emitter *mockSessionUpdateEmitter) *Renderer {
	return NewRenderer(sessionID, stepID, emitter, &MockMasker{}, slog.New(slog.NewTextHandler(&discardWriter{}, nil)), map[string]string{})
}

// TestRenderer_RenderEventText verifies EventText → agent_message_chunk with the ACP content shape.
func TestRenderer_RenderEventText(t *testing.T) {
	emitter := &mockSessionUpdateEmitter{}
	renderer := newTestRenderer("sess-1", "step-1", emitter)

	event := display.DisplayEvent{Type: string(display.EventText), Kind: display.EventText, Text: "hello world"}
	require.NoError(t, renderer.Render(context.Background(), &event))

	updates := emitter.Updates()
	require.Len(t, updates, 1)
	assert.Equal(t, "sess-1", updates[0].sessionID, "must route to the ACP session, not the step")
	assert.Equal(t, "agent_message_chunk", updates[0].kind)
	assert.Equal(t, "hello world", contentText(t, updates[0].fields))
}

// TestRenderer_RenderReasoning verifies EventReasoning → agent_thought_chunk.
func TestRenderer_RenderReasoning(t *testing.T) {
	emitter := &mockSessionUpdateEmitter{}
	renderer := newTestRenderer("sess-1", "step-1", emitter)

	event := display.DisplayEvent{Type: string(display.EventReasoning), Kind: display.EventReasoning, Text: "thinking"}
	require.NoError(t, renderer.Render(context.Background(), &event))

	updates := emitter.Updates()
	require.Len(t, updates, 1)
	assert.Equal(t, "agent_thought_chunk", updates[0].kind)
	assert.Equal(t, "thinking", contentText(t, updates[0].fields))
}

// TestRenderer_RenderToolUseFirstSighting verifies first tool use → tool_call with ACP fields.
func TestRenderer_RenderToolUseFirstSighting(t *testing.T) {
	emitter := &mockSessionUpdateEmitter{}
	renderer := newTestRenderer("sess-1", "step-1", emitter)

	event := display.DisplayEvent{
		Type: string(display.EventToolUse), Kind: display.EventToolUse,
		ID: "tool-123", Name: "bash", Arg: "echo hello",
	}
	require.NoError(t, renderer.Render(context.Background(), &event))

	updates := emitter.Updates()
	require.Len(t, updates, 1)
	assert.Equal(t, "tool_call", updates[0].kind)
	assert.Equal(t, "tool-123", updates[0].fields["toolCallId"])
	assert.Equal(t, "bash", updates[0].fields["title"])
	assert.Equal(t, "echo hello", rawInputText(t, updates[0].fields))
}

// TestRenderer_RenderToolUseSubsequentSighting verifies second sighting → tool_call_update.
func TestRenderer_RenderToolUseSubsequentSighting(t *testing.T) {
	emitter := &mockSessionUpdateEmitter{}
	renderer := newTestRenderer("sess-1", "step-1", emitter)

	event1 := display.DisplayEvent{Type: string(display.EventToolUse), Kind: display.EventToolUse, ID: "tool-123", Name: "bash", Arg: "echo hello"}
	require.NoError(t, renderer.Render(context.Background(), &event1))
	event2 := display.DisplayEvent{Type: string(display.EventToolUse), Kind: display.EventToolUse, ID: "tool-123", Name: "bash", Arg: "echo world"}
	require.NoError(t, renderer.Render(context.Background(), &event2))

	updates := emitter.Updates()
	require.Len(t, updates, 2)
	assert.Equal(t, "tool_call", updates[0].kind)
	assert.Equal(t, "tool-123", updates[0].fields["toolCallId"])
	assert.Equal(t, "tool_call_update", updates[1].kind)
	assert.Equal(t, "tool-123", updates[1].fields["toolCallId"])
}

// TestRenderer_DifferentToolIDsEmitToolCall verifies distinct tools both emit tool_call.
func TestRenderer_DifferentToolIDsEmitToolCall(t *testing.T) {
	emitter := &mockSessionUpdateEmitter{}
	renderer := newTestRenderer("sess-1", "step-1", emitter)

	event1 := display.DisplayEvent{Type: string(display.EventToolUse), Kind: display.EventToolUse, ID: "tool-123", Name: "bash", Arg: "echo hello"}
	require.NoError(t, renderer.Render(context.Background(), &event1))
	event2 := display.DisplayEvent{Type: string(display.EventToolUse), Kind: display.EventToolUse, ID: "tool-456", Name: "read", Arg: "/etc/passwd"}
	require.NoError(t, renderer.Render(context.Background(), &event2))

	updates := emitter.Updates()
	require.Len(t, updates, 2)
	assert.Equal(t, "tool_call", updates[0].kind)
	assert.Equal(t, "tool-123", updates[0].fields["toolCallId"])
	assert.Equal(t, "tool_call", updates[1].kind)
	assert.Equal(t, "tool-456", updates[1].fields["toolCallId"])
}

// TestRenderer_EmptyIDUsesStableNameBasedID verifies streaming chunks with empty ID
// use name-based synthesis so all chunks of one tool share a stable ID (issue #4).
func TestRenderer_EmptyIDUsesStableNameBasedID(t *testing.T) {
	emitter := &mockSessionUpdateEmitter{}
	renderer := newTestRenderer("sess-1", "step-1", emitter)

	for i := range 3 {
		event := display.DisplayEvent{
			Type: string(display.EventToolUse), Kind: display.EventToolUse,
			ID: "", Name: "bash", Arg: fmt.Sprintf("arg-chunk-%d", i),
		}
		require.NoError(t, renderer.Render(context.Background(), &event))
	}

	updates := emitter.Updates()
	require.Len(t, updates, 3)
	assert.Equal(t, "tool_call", updates[0].kind, "first chunk without ID must be tool_call")
	assert.Equal(t, "step-1-tool-bash", updates[0].fields["toolCallId"], "synthesized ID must be stable (name-based)")
	assert.Equal(t, "tool_call_update", updates[1].kind)
	assert.Equal(t, "step-1-tool-bash", updates[1].fields["toolCallId"])
	assert.Equal(t, "tool_call_update", updates[2].kind)
	assert.Equal(t, "step-1-tool-bash", updates[2].fields["toolCallId"])
}

// TestRenderer_SecretMaskingApplied verifies secrets are redacted before emission,
// using the real logger.SecretMasker.
func TestRenderer_SecretMaskingApplied(t *testing.T) {
	emitter := &mockSessionUpdateEmitter{}
	env := map[string]string{"API_KEY": "sk-secret-123"}
	renderer := NewRenderer("sess-1", "step-1", emitter, infralogger.NewSecretMasker(), slog.New(slog.NewTextHandler(&discardWriter{}, nil)), env)

	event := display.DisplayEvent{Type: string(display.EventText), Kind: display.EventText, Text: "using key sk-secret-123 for auth"}
	require.NoError(t, renderer.Render(context.Background(), &event))

	updates := emitter.Updates()
	require.Len(t, updates, 1)
	got := contentText(t, updates[0].fields)
	assert.NotContains(t, got, "sk-secret-123")
	assert.Contains(t, got, "***")
}

// TestRenderer_Concurrent verifies concurrent Render calls produce no race.
func TestRenderer_Concurrent(t *testing.T) {
	emitter := &mockSessionUpdateEmitter{}
	renderer := newTestRenderer("sess-1", "step-1", emitter)

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			event := display.DisplayEvent{Type: string(display.EventText), Kind: display.EventText, Text: fmt.Sprintf("message %d", idx)}
			_ = renderer.Render(context.Background(), &event)
		}(i)
	}
	wg.Wait()

	assert.Len(t, emitter.Updates(), 10)
}

// TestRenderer_NilEventDoesNotPanic verifies nil events are dropped with a WARN.
func TestRenderer_NilEventDoesNotPanic(t *testing.T) {
	emitter := &mockSessionUpdateEmitter{}
	renderer := newTestRenderer("sess-1", "step-1", emitter)

	require.NotPanics(t, func() {
		assert.NoError(t, renderer.Render(context.Background(), nil))
	})
	assert.Empty(t, emitter.Updates(), "nil event must not produce any update")
}

// TestRenderer_UnknownEventTypeNoOp verifies unknown kinds are ignored.
func TestRenderer_UnknownEventTypeNoOp(t *testing.T) {
	emitter := &mockSessionUpdateEmitter{}
	renderer := newTestRenderer("sess-1", "step-1", emitter)

	event := display.DisplayEvent{Type: "unknown_event_type"}
	require.NoError(t, renderer.Render(context.Background(), &event))
	assert.Empty(t, emitter.Updates())
}

// TestRenderer_PerStepIsolation verifies two renderers each have independent seenTools.
func TestRenderer_PerStepIsolation(t *testing.T) {
	emitterA := &mockSessionUpdateEmitter{}
	emitterB := &mockSessionUpdateEmitter{}
	rendererA := newTestRenderer("sess-1", "step-A", emitterA)
	rendererB := newTestRenderer("sess-1", "step-B", emitterB)

	toolEvent := display.DisplayEvent{Type: string(display.EventToolUse), Kind: display.EventToolUse, ID: "tool-shared", Name: "bash", Arg: "echo hi"}
	require.NoError(t, rendererA.Render(context.Background(), &toolEvent))
	require.NoError(t, rendererB.Render(context.Background(), &toolEvent))

	updatesA := emitterA.Updates()
	updatesB := emitterB.Updates()
	require.Len(t, updatesA, 1)
	require.Len(t, updatesB, 1)
	assert.Equal(t, "tool_call", updatesA[0].kind)
	assert.Equal(t, "tool_call", updatesB[0].kind, "step-B must see fresh seenTools — same tool ID is a first sighting")
}

// TestRenderer_RenderFunc verifies the closure satisfies agents.DisplayEventRenderer
// and processes all events.
func TestRenderer_RenderFunc(t *testing.T) {
	emitter := &mockSessionUpdateEmitter{}
	renderer := newTestRenderer("sess-1", "step-1", emitter)

	renderFunc := renderer.RenderFunc(context.Background())
	require.NotNil(t, renderFunc)

	events := []agents.DisplayEvent{
		{Type: string(display.EventText), Kind: display.EventText, Text: "event-1"},
		{Type: string(display.EventText), Kind: display.EventText, Text: "event-2"},
	}
	renderFunc(events)
	assert.Len(t, emitter.Updates(), 2)
}

// TestRenderer_RenderFunc_StopsOnCancelledCtx verifies a cancelled ctx skips all events.
func TestRenderer_RenderFunc_StopsOnCancelledCtx(t *testing.T) {
	emitter := &mockSessionUpdateEmitter{}
	renderer := newTestRenderer("sess-1", "step-1", emitter)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	renderFunc := renderer.RenderFunc(ctx)
	renderFunc([]agents.DisplayEvent{
		{Type: string(display.EventText), Kind: display.EventText, Text: "event-1"},
		{Type: string(display.EventText), Kind: display.EventText, Text: "event-2"},
	})
	assert.Empty(t, emitter.Updates(), "no updates when ctx is already cancelled")
}

// TestRenderer_SlowEmitterDoesNotSerializeCallers verifies EmitSessionUpdate runs
// outside the seenTools mutex: concurrent callers must overlap inside the emitter.
func TestRenderer_SlowEmitterDoesNotSerializeCallers(t *testing.T) {
	var concurrent, maxConcurrent atomic.Int64
	var mu sync.Mutex
	var count int

	slow := &sessionUpdateEmitterAdapter{fn: func(_ context.Context, _, _ string, _ map[string]any) error {
		n := concurrent.Add(1)
		for {
			prev := maxConcurrent.Load()
			if n <= prev {
				break
			}
			if maxConcurrent.CompareAndSwap(prev, n) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		concurrent.Add(-1)
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	}}

	renderer := NewRenderer("sess-1", "step-1", slow, &MockMasker{}, slog.New(slog.NewTextHandler(&discardWriter{}, nil)), map[string]string{})

	const n = 8
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			event := display.DisplayEvent{Type: string(display.EventText), Kind: display.EventText, Text: fmt.Sprintf("msg-%d", idx)}
			_ = renderer.Render(context.Background(), &event)
		}(i)
	}
	wg.Wait()

	mu.Lock()
	gotCount := count
	mu.Unlock()
	assert.Equal(t, n, gotCount, "all updates must be emitted")
	assert.Greater(t, maxConcurrent.Load(), int64(1), "emitting must run outside the mutex")
}

// sessionUpdateEmitterAdapter adapts a function to application.SessionUpdateEmitter.
type sessionUpdateEmitterAdapter struct {
	fn func(ctx context.Context, sessionID, kind string, fields map[string]any) error
}

func (a *sessionUpdateEmitterAdapter) EmitSessionUpdate(ctx context.Context, sessionID, kind string, fields map[string]any) error {
	return a.fn(ctx, sessionID, kind, fields)
}
