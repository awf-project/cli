package acp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/agents"
	infralogger "github.com/awf-project/cli/internal/infrastructure/logger"
	"github.com/awf-project/cli/pkg/display"
)

// MockSender records sent messages (and the ctx they were sent with) for testing.
type MockSender struct {
	messages []Message
	ctxs     []context.Context
	errors   map[int]error // map of call index to error
	mu       sync.Mutex
}

func NewMockSender() *MockSender {
	return &MockSender{
		messages: []Message{},
		errors:   make(map[int]error),
	}
}

func (m *MockSender) Send(ctx context.Context, msg Message) error { //nolint:gocritic
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := len(m.messages)
	m.messages = append(m.messages, msg)
	m.ctxs = append(m.ctxs, ctx)
	if err, ok := m.errors[idx]; ok {
		return err
	}
	return nil
}

// Contexts returns a copy of the contexts captured by each Send call.
func (m *MockSender) Contexts() []context.Context {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]context.Context, len(m.ctxs))
	copy(cp, m.ctxs)
	return cp
}

func (m *MockSender) Messages() []Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]Message, len(m.messages))
	copy(cp, m.messages)
	return cp
}

func (m *MockSender) SetError(callIndex int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[callIndex] = err
}

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

// MockLogger captures log calls.
type MockLogger struct {
	warnings []string
	errors   []string
	mu       sync.Mutex
}

func (m *MockLogger) Debug(msg string, fields ...any) {}
func (m *MockLogger) Info(msg string, fields ...any)  {}

func (m *MockLogger) Warn(msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.warnings = append(m.warnings, msg)
}

func (m *MockLogger) Error(msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors = append(m.errors, msg)
}

func (m *MockLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

func (m *MockLogger) Warnings() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]string, len(m.warnings))
	copy(cp, m.warnings)
	return cp
}

// Test: Render EventText to MsgAgentMessageChunk
func TestACPRenderer_RenderEventText(t *testing.T) {
	sender := NewMockSender()
	masker := &MockMasker{}
	logger := &MockLogger{}
	env := map[string]string{}

	renderer := NewACPRenderer("step-1", sender, masker, logger, env)

	event := display.DisplayEvent{
		Type: string(display.EventText),
		Kind: display.EventText,
		Text: "hello world",
	}

	err := renderer.Render(context.Background(), &event)
	require.NoError(t, err)

	messages := sender.Messages()
	require.Len(t, messages, 1)
	assert.Equal(t, MsgAgentMessageChunk, messages[0].Type)
	assert.Equal(t, "step-1", messages[0].StepID)
	assert.Equal(t, uint64(1), messages[0].Seq)
	assert.Equal(t, "hello world", messages[0].Content)
}

// Test: Render propagates the workflow ctx to Sender.Send so a disconnected peer
// (cancelled ctx) stops emission instead of writing with a detached context.
func TestACPRenderer_PropagatesContextToSend(t *testing.T) {
	sender := NewMockSender()
	masker := &MockMasker{}
	logger := &MockLogger{}
	env := map[string]string{}

	renderer := NewACPRenderer("step-1", sender, masker, logger, env)

	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "workflow")

	event := display.DisplayEvent{
		Type: string(display.EventText),
		Kind: display.EventText,
		Text: "hello",
	}

	require.NoError(t, renderer.Render(ctx, &event))

	ctxs := sender.Contexts()
	require.Len(t, ctxs, 1)
	assert.Equal(t, "workflow", ctxs[0].Value(ctxKey{}),
		"Render must forward its ctx to Sender.Send, not a detached context")
}

// Test: Render reasoning event to MsgAgentThoughtChunk.
// Verifies that display.EventReasoning ("reasoning") maps to MsgAgentThoughtChunk,
// and that the constant is used consistently — no magic string in renderer or test.
func TestACPRenderer_RenderReasoning(t *testing.T) {
	sender := NewMockSender()
	masker := &MockMasker{}
	logger := &MockLogger{}
	env := map[string]string{}

	renderer := NewACPRenderer("step-1", sender, masker, logger, env)

	event := display.DisplayEvent{
		Type: string(display.EventReasoning),
		Kind: display.EventReasoning,
		Text: "thinking about the problem",
	}

	err := renderer.Render(context.Background(), &event)
	require.NoError(t, err)

	messages := sender.Messages()
	require.Len(t, messages, 1)
	assert.Equal(t, MsgAgentThoughtChunk, messages[0].Type)
	assert.Equal(t, "step-1", messages[0].StepID)
	assert.Equal(t, uint64(1), messages[0].Seq)
	assert.Equal(t, "thinking about the problem", messages[0].Content)
}

// Test: First EventToolUse with given ID becomes MsgToolCall
func TestACPRenderer_RenderToolUseFirstSighting(t *testing.T) {
	sender := NewMockSender()
	masker := &MockMasker{}
	logger := &MockLogger{}
	env := map[string]string{}

	renderer := NewACPRenderer("step-1", sender, masker, logger, env)

	event := display.DisplayEvent{
		Type: string(display.EventToolUse),
		Kind: display.EventToolUse,
		ID:   "tool-123",
		Name: "bash",
		Arg:  "echo hello",
	}

	err := renderer.Render(context.Background(), &event)
	require.NoError(t, err)

	messages := sender.Messages()
	require.Len(t, messages, 1)
	assert.Equal(t, MsgToolCall, messages[0].Type)
	assert.Equal(t, "step-1", messages[0].StepID)
	assert.Equal(t, uint64(1), messages[0].Seq)
	assert.Equal(t, "tool-123", messages[0].ToolID)
	assert.Equal(t, "bash", messages[0].Tool)
	assert.Equal(t, "echo hello", messages[0].Content)
}

// Test: Subsequent same-ID EventToolUse becomes MsgToolCallUpdate
func TestACPRenderer_RenderToolUseSubsequentSighting(t *testing.T) {
	sender := NewMockSender()
	masker := &MockMasker{}
	logger := &MockLogger{}
	env := map[string]string{}

	renderer := NewACPRenderer("step-1", sender, masker, logger, env)

	// First sighting
	event1 := display.DisplayEvent{
		Type: string(display.EventToolUse),
		Kind: display.EventToolUse,
		ID:   "tool-123",
		Name: "bash",
		Arg:  "echo hello",
	}
	err := renderer.Render(context.Background(), &event1)
	require.NoError(t, err)

	// Second sighting with same ID
	event2 := display.DisplayEvent{
		Type: string(display.EventToolUse),
		Kind: display.EventToolUse,
		ID:   "tool-123",
		Name: "bash",
		Arg:  "echo world",
	}
	err = renderer.Render(context.Background(), &event2)
	require.NoError(t, err)

	messages := sender.Messages()
	require.Len(t, messages, 2)
	assert.Equal(t, MsgToolCall, messages[0].Type)
	assert.Equal(t, "step-1", messages[0].StepID)
	assert.Equal(t, uint64(1), messages[0].Seq)
	assert.Equal(t, "tool-123", messages[0].ToolID)
	assert.Equal(t, "bash", messages[0].Tool)
	assert.Equal(t, MsgToolCallUpdate, messages[1].Type)
	assert.Equal(t, "step-1", messages[1].StepID)
	assert.Equal(t, uint64(2), messages[1].Seq)
	assert.Equal(t, "tool-123", messages[1].ToolID)
	assert.Equal(t, "bash", messages[1].Tool)
}

// Test: Two distinct tool IDs in same step both emit MsgToolCall
func TestACPRenderer_DifferentToolIdsEmitMsgToolCall(t *testing.T) {
	sender := NewMockSender()
	masker := &MockMasker{}
	logger := &MockLogger{}
	env := map[string]string{}

	renderer := NewACPRenderer("step-1", sender, masker, logger, env)

	// First tool
	event1 := display.DisplayEvent{
		Type: string(display.EventToolUse),
		Kind: display.EventToolUse,
		ID:   "tool-123",
		Name: "bash",
		Arg:  "echo hello",
	}
	err := renderer.Render(context.Background(), &event1)
	require.NoError(t, err)

	// Different tool ID
	event2 := display.DisplayEvent{
		Type: string(display.EventToolUse),
		Kind: display.EventToolUse,
		ID:   "tool-456",
		Name: "read",
		Arg:  "/etc/passwd",
	}
	err = renderer.Render(context.Background(), &event2)
	require.NoError(t, err)

	messages := sender.Messages()
	require.Len(t, messages, 2)
	assert.Equal(t, MsgToolCall, messages[0].Type)
	assert.Equal(t, "step-1", messages[0].StepID)
	assert.Equal(t, uint64(1), messages[0].Seq)
	assert.Equal(t, "tool-123", messages[0].ToolID)
	assert.Equal(t, "bash", messages[0].Tool)
	assert.Equal(t, MsgToolCall, messages[1].Type)
	assert.Equal(t, "step-1", messages[1].StepID)
	assert.Equal(t, uint64(2), messages[1].Seq)
	assert.Equal(t, "tool-456", messages[1].ToolID)
	assert.Equal(t, "read", messages[1].Tool)
}

// Test: Empty event.ID is synthesized to stable ID
func TestACPRenderer_SynthesizeIdWhenEmpty(t *testing.T) {
	sender := NewMockSender()
	masker := &MockMasker{}
	logger := &MockLogger{}
	env := map[string]string{}

	renderer := NewACPRenderer("step-1", sender, masker, logger, env)

	// Event with empty ID should be synthesized
	event := display.DisplayEvent{
		Type: string(display.EventToolUse),
		Kind: display.EventToolUse,
		ID:   "", // empty
		Name: "bash",
		Arg:  "echo test",
	}

	err := renderer.Render(context.Background(), &event)
	require.NoError(t, err)

	messages := sender.Messages()
	require.Len(t, messages, 1)
	assert.Equal(t, MsgToolCall, messages[0].Type)
	assert.NotEmpty(t, messages[0].ToolID)
	// Synthesized ID should be in format: step-ID + "-tool-" + seq
	assert.Contains(t, messages[0].ToolID, "step-1-tool-")
}

// Test: Streaming tool chunks without event.ID use a name-stable synthesized ID.
// Issue #4: when event.ID is empty the previous implementation synthesized
// "<stepID>-tool-<seq>", which is unique per event — every chunk looked like a
// first sighting and was classified MsgToolCall. The fix synthesizes
// "<stepID>-tool-<toolName>" so all chunks of the same tool share a stable ID;
// only the first chunk is MsgToolCall and subsequent chunks are MsgToolCallUpdate.
func TestACPRenderer_EmptyIDUsesStableNameBasedID(t *testing.T) {
	sender := NewMockSender()
	masker := &MockMasker{}
	logger := &MockLogger{}
	env := map[string]string{}

	renderer := NewACPRenderer("step-1", sender, masker, logger, env)

	// Three streaming chunks for the same tool — all with empty ID and same Name.
	for i := range 3 {
		event := display.DisplayEvent{
			Type: string(display.EventToolUse),
			Kind: display.EventToolUse,
			ID:   "", // provider does not populate ID
			Name: "bash",
			Arg:  fmt.Sprintf("arg-chunk-%d", i),
		}
		err := renderer.Render(context.Background(), &event)
		require.NoError(t, err)
	}

	messages := sender.Messages()
	require.Len(t, messages, 3)

	// First chunk: must be MsgToolCall (first sighting of stable synthesized ID).
	assert.Equal(t, MsgToolCall, messages[0].Type, "first chunk without ID must be MsgToolCall")
	assert.Equal(t, "step-1-tool-bash", messages[0].ToolID, "synthesized ID must be stable (name-based)")
	assert.Equal(t, "bash", messages[0].Tool)

	// Second and third chunks: same tool name => same synthesized ID => MsgToolCallUpdate.
	assert.Equal(t, MsgToolCallUpdate, messages[1].Type, "second chunk same tool must be MsgToolCallUpdate")
	assert.Equal(t, "step-1-tool-bash", messages[1].ToolID)

	assert.Equal(t, MsgToolCallUpdate, messages[2].Type, "third chunk same tool must be MsgToolCallUpdate")
	assert.Equal(t, "step-1-tool-bash", messages[2].ToolID)
}

// Test: When both ID and Name are empty, fallback to seq-based ID (degenerate case).
// Dedup won't work without a name, but the fallback must not panic and must produce
// a non-empty ToolID. Each such event gets a unique seq-based ID (all MsgToolCall).
func TestACPRenderer_EmptyIDAndEmptyNameFallsBackToSeq(t *testing.T) {
	sender := NewMockSender()
	masker := &MockMasker{}
	logger := &MockLogger{}
	env := map[string]string{}

	renderer := NewACPRenderer("step-1", sender, masker, logger, env)

	for range 2 {
		event := display.DisplayEvent{
			Type: string(display.EventToolUse),
			Kind: display.EventToolUse,
			ID:   "", // no provider ID
			Name: "", // no tool name either — degenerate case
			Arg:  "x",
		}
		err := renderer.Render(context.Background(), &event)
		require.NoError(t, err)
	}

	messages := sender.Messages()
	require.Len(t, messages, 2)

	// Both get unique seq-based IDs so both are MsgToolCall (no dedup possible).
	assert.Equal(t, MsgToolCall, messages[0].Type)
	assert.Contains(t, messages[0].ToolID, "step-1-tool-")

	assert.Equal(t, MsgToolCall, messages[1].Type, "empty name fallback: each event gets unique seq ID => always MsgToolCall")
	assert.Contains(t, messages[1].ToolID, "step-1-tool-")

	// The two fallback IDs must be distinct (seq-based uniqueness).
	assert.NotEqual(t, messages[0].ToolID, messages[1].ToolID, "seq-based fallback IDs must differ")
}

// Test: Secret masking is applied using the real logger.SecretMasker
func TestACPRenderer_SecretMaskingApplied(t *testing.T) {
	sender := NewMockSender()
	logger := &MockLogger{}

	masker := infralogger.NewSecretMasker()

	env := map[string]string{
		"API_KEY": "sk-secret-123",
	}

	renderer := NewACPRenderer("step-1", sender, masker, logger, env)

	event := display.DisplayEvent{
		Type: string(display.EventText),
		Kind: display.EventText,
		Text: "using key sk-secret-123 for auth",
	}

	err := renderer.Render(context.Background(), &event)
	require.NoError(t, err)

	messages := sender.Messages()
	require.Len(t, messages, 1)
	// Content should be masked
	assert.NotContains(t, messages[0].Content, "sk-secret-123")
	assert.Contains(t, messages[0].Content, "***")
}

// Test: Concurrent Render calls produce no race and all seq values are unique
func TestACPRenderer_Concurrent(t *testing.T) {
	sender := NewMockSender()
	masker := &MockMasker{}
	logger := &MockLogger{}
	env := map[string]string{}

	renderer := NewACPRenderer("step-1", sender, masker, logger, env)

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			event := display.DisplayEvent{
				Type: string(display.EventText),
				Kind: display.EventText,
				Text: fmt.Sprintf("message %d", idx),
			}
			_ = renderer.Render(context.Background(), &event)
		}(i)
	}
	wg.Wait()

	messages := sender.Messages()
	assert.Len(t, messages, 10)

	// All seq values must be unique and span exactly 1..10
	seqs := make([]int, 0, len(messages))
	for _, msg := range messages {
		seqs = append(seqs, int(msg.Seq)) //nolint:gosec // controlled test values, no overflow risk
	}
	sort.Ints(seqs)
	for i, s := range seqs {
		assert.Equal(t, i+1, s, "expected seq %d at position %d", i+1, i)
	}
}

// Test: RenderFunc logs and continues on Send error; uses agents.DisplayEvent slice type;
// ctx passed to RenderFunc is captured and propagated to each per-event Render call.
func TestACPRenderer_RenderFunc_LogsAndContinuesOnSendError(t *testing.T) {
	sender := NewMockSender()
	masker := &MockMasker{}
	logger := &MockLogger{}
	env := map[string]string{}

	renderer := NewACPRenderer("step-1", sender, masker, logger, env)

	// Set error on second call
	sender.SetError(1, fmt.Errorf("send failed"))

	// Use a specific derived context — the closure must capture it and forward it to
	// each Render(ctx, event) call rather than using context.Background() internally.
	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "renderFuncCtx")

	renderFunc := renderer.RenderFunc(ctx)

	// Use agents.DisplayEvent to validate the actual adapter type bridge
	events := []agents.DisplayEvent{
		{
			Type: string(display.EventText),
			Kind: display.EventText,
			Text: "first event",
		},
		{
			Type: string(display.EventText),
			Kind: display.EventText,
			Text: "second event (will fail)",
		},
		{
			Type: string(display.EventText),
			Kind: display.EventText,
			Text: "third event",
		},
	}

	// Should not panic and should process all events
	renderFunc(events)

	// Verify logger captured exactly one warning from the failed Send
	warnings := logger.Warnings()
	assert.Len(t, warnings, 1)
	assert.Equal(t, "acp render failed", warnings[0])

	// All three events should have been attempted (log+continue, not abort)
	messages := sender.Messages()
	assert.Len(t, messages, 3)
}

// Test: nil event must not panic — C3 nil-guard contract.
// A nil event is dropped silently (no message sent) but a WARN is logged
// so a buggy caller is visible in diagnostics.
func TestACPRenderer_NilEventDoesNotPanic(t *testing.T) {
	sender := NewMockSender()
	masker := &MockMasker{}
	logger := &MockLogger{}
	env := map[string]string{}

	renderer := NewACPRenderer("step-1", sender, masker, logger, env)

	require.NotPanics(t, func() {
		err := renderer.Render(context.Background(), nil)
		assert.NoError(t, err)
	})
	assert.Empty(t, sender.Messages(), "nil event must not produce any message")
	warnings := logger.Warnings()
	require.Len(t, warnings, 1, "nil event must log a WARN so the buggy caller is visible")
	assert.Equal(t, "acp renderer: nil event dropped", warnings[0])
}

// Test: Unknown event type gracefully no-ops
func TestACPRenderer_UnknownEventTypeNoOp(t *testing.T) {
	sender := NewMockSender()
	masker := &MockMasker{}
	logger := &MockLogger{}
	env := map[string]string{}

	renderer := NewACPRenderer("step-1", sender, masker, logger, env)

	event := display.DisplayEvent{
		Type: "unknown_event_type",
		Text: "should be ignored",
	}

	err := renderer.Render(context.Background(), &event)
	require.NoError(t, err)

	// No message should be sent
	messages := sender.Messages()
	assert.Empty(t, messages)
}

// Test: M3 — Sender.Send must NOT be called while the mutex is held.
// A slow Sender must not serialize concurrent Render callers: two goroutines
// must be able to reach Sender.Send concurrently (no deadlock, no global serialization).
func TestACPRenderer_SlowSenderDoesNotSerializeCallers(t *testing.T) {
	// SlowSender blocks for a short time to amplify serialization effects.
	type slowSender struct {
		mu       sync.Mutex
		messages []Message
		// concurrent tracks how many goroutines are inside Send simultaneously.
		concurrent    atomic.Int64
		maxConcurrent atomic.Int64
	}
	slow := &slowSender{}
	slow.messages = []Message{}

	sendFn := func(ctx context.Context, msg Message) error { //nolint:gocritic // hugeParam: Message is ~112 bytes; accept by value per Sender interface
		n := slow.concurrent.Add(1)
		// Track the high-water mark of concurrent Send calls.
		for {
			prev := slow.maxConcurrent.Load()
			if n <= prev {
				break
			}
			if slow.maxConcurrent.CompareAndSwap(prev, n) {
				break
			}
		}
		// Simulate slow I/O.
		time.Sleep(5 * time.Millisecond)
		slow.concurrent.Add(-1)
		slow.mu.Lock()
		slow.messages = append(slow.messages, msg)
		slow.mu.Unlock()
		return nil
	}

	fs := &funcSender{fn: sendFn}

	masker := &MockMasker{}
	logger := &MockLogger{}
	env := map[string]string{}
	renderer := NewACPRenderer("step-1", fs, masker, logger, env)

	const n = 8
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			event := display.DisplayEvent{
				Type: string(display.EventText),
				Kind: display.EventText,
				Text: fmt.Sprintf("msg-%d", idx),
			}
			_ = renderer.Render(context.Background(), &event)
		}(i)
	}
	wg.Wait()

	slow.mu.Lock()
	gotCount := len(slow.messages)
	slow.mu.Unlock()
	assert.Equal(t, n, gotCount, "all messages must be delivered")

	// With the mutex released before Send, at least 2 goroutines must have overlapped
	// inside Send. If the mutex were held during Send, maxConcurrent would always be 1.
	assert.Greater(t, slow.maxConcurrent.Load(), int64(1),
		"Send must be called outside the mutex: expected concurrent Send calls, got max=%d",
		slow.maxConcurrent.Load())
}

// funcSender wraps a function as a Sender (used by SlowSender test above).
type funcSender struct {
	fn func(ctx context.Context, msg Message) error
}

func (f *funcSender) Send(ctx context.Context, msg Message) error { //nolint:gocritic // hugeParam: Message is ~112 bytes; accept by value per Sender interface
	return f.fn(ctx, msg)
}

// Test: RenderFunc stops processing events once ctx is cancelled (M5 fix).
func TestACPRenderer_RenderFunc_StopsOnCancelledCtx(t *testing.T) {
	sender := NewMockSender()
	masker := &MockMasker{}
	logger := &MockLogger{}
	env := map[string]string{}

	renderer := NewACPRenderer("step-1", sender, masker, logger, env)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately — all events should be skipped

	renderFunc := renderer.RenderFunc(ctx)

	events := []agents.DisplayEvent{
		{Type: string(display.EventText), Kind: display.EventText, Text: "event-1"},
		{Type: string(display.EventText), Kind: display.EventText, Text: "event-2"},
		{Type: string(display.EventText), Kind: display.EventText, Text: "event-3"},
	}
	renderFunc(events)

	// With cancelled ctx, no events should be processed.
	assert.Empty(t, sender.Messages(), "no messages must be sent when ctx is already cancelled")
}

// Test: Two renderers with different stepIDs each have a fresh seenTools index —
// a tool ID seen in step-A must still emit MsgToolCall (first sighting) in step-B.
func TestACPRenderer_PerStepIsolation(t *testing.T) {
	senderA := NewMockSender()
	senderB := NewMockSender()
	masker := &MockMasker{}
	logger := &MockLogger{}
	env := map[string]string{}

	rendererA := NewACPRenderer("step-A", senderA, masker, logger, env)
	rendererB := NewACPRenderer("step-B", senderB, masker, logger, env)

	toolEvent := display.DisplayEvent{
		Type: string(display.EventToolUse),
		Kind: display.EventToolUse,
		ID:   "tool-shared",
		Name: "bash",
		Arg:  "echo hi",
	}

	// First sighting in step-A
	err := rendererA.Render(context.Background(), &toolEvent)
	require.NoError(t, err)

	// Same tool ID in step-B — must still be a first sighting (fresh seenTools)
	err = rendererB.Render(context.Background(), &toolEvent)
	require.NoError(t, err)

	msgsA := senderA.Messages()
	msgsB := senderB.Messages()
	require.Len(t, msgsA, 1)
	require.Len(t, msgsB, 1)

	assert.Equal(t, MsgToolCall, msgsA[0].Type)
	assert.Equal(t, "step-A", msgsA[0].StepID)

	assert.Equal(t, MsgToolCall, msgsB[0].Type, "step-B must see fresh seenTools — same tool ID is a first sighting")
	assert.Equal(t, "step-B", msgsB[0].StepID)
}
