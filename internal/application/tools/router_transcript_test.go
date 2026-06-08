package tools

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
)

// mockRecorder captures Record calls for test verification.
type mockRecorder struct {
	mu          sync.Mutex
	events      []transcript.ExchangeEvent
	recordErr   error
	recordCalls int
}

func (m *mockRecorder) Record(ctx context.Context, event transcript.ExchangeEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordCalls++
	m.events = append(m.events, event)
	return m.recordErr
}

func (m *mockRecorder) Subscribe() (<-chan transcript.ExchangeEvent, func()) {
	ch := make(chan transcript.ExchangeEvent)
	return ch, func() { close(ch) }
}

func (m *mockRecorder) Close() error {
	return nil
}

func (m *mockRecorder) getEvents() []transcript.ExchangeEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]transcript.ExchangeEvent, len(m.events))
	copy(result, m.events)
	return result
}

// TestRouter_SetRecorder stores the recorder in the router field.
func TestRouter_SetRecorder(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	rec := &mockRecorder{}
	router.SetRecorder(rec)

	assert.NotNil(t, router.recorder)
	assert.Equal(t, rec, router.recorder)
}

// TestRouter_SetRunID_PropagatesToEmittedEvents verifies the run ID set via SetRunID
// is stamped onto both the tool.call and tool.result events so tool exchanges can be
// correlated to their originating workflow run.
func TestRouter_SetRunID_PropagatesToEmittedEvents(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	rec := &mockRecorder{}
	router.SetRecorder(rec)
	router.SetRunID("run-1234")

	provider := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool1", Source: "test"},
		},
	}
	require.NoError(t, router.Register(context.Background(), provider))

	_, err := router.CallTool(context.Background(), "tool1", map[string]any{})
	require.NoError(t, err)

	events := rec.getEvents()
	require.Len(t, events, 2)
	assert.Equal(t, "run-1234", events[0].RunID, "tool.call must carry the run ID")
	assert.Equal(t, "run-1234", events[1].RunID, "tool.result must carry the run ID")
}

// TestRouter_NilRecorderIsNoOp verifies no transcript emission when recorder unset.
func TestRouter_NilRecorderIsNoOp(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	provider := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool1", Source: "test"},
		},
	}
	router.Register(context.Background(), provider)

	result, err := router.CallTool(context.Background(), "tool1", map[string]any{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestRouter_BuiltinToolEmitsRouterFidelityPair verifies tool.call + tool.result events
// with Fidelity:"router" when recorder is set.
func TestRouter_BuiltinToolEmitsRouterFidelityPair(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	rec := &mockRecorder{}
	router.SetRecorder(rec)

	provider := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "builtintool", Source: "builtin"},
		},
	}
	router.Register(context.Background(), provider)

	result, err := router.CallTool(context.Background(), "builtintool", map[string]any{"key": "value"})

	require.NoError(t, err)
	assert.NotNil(t, result)

	events := rec.getEvents()
	require.Len(t, events, 2, "must emit exactly one tool.call + one tool.result event")

	callEvent := events[0]
	resultEvent := events[1]

	assert.Equal(t, transcript.EventTypeToolCall, callEvent.Type)
	assert.Equal(t, transcript.EventTypeToolResult, resultEvent.Type)

	require.IsType(t, (*transcript.ToolPayload)(nil), callEvent.Payload)
	require.IsType(t, (*transcript.ToolPayload)(nil), resultEvent.Payload)

	callPayload := callEvent.Payload.(*transcript.ToolPayload)
	resultPayload := resultEvent.Payload.(*transcript.ToolPayload)

	assert.Equal(t, "builtintool", callPayload.Name)
	assert.Equal(t, transcript.FidelityRouter, callPayload.Fidelity, "tool.call must have Fidelity:router")
	assert.Equal(t, map[string]any{"key": "value"}, callPayload.Input)
	assert.NotEmpty(t, callPayload.CallID, "tool.call must have CallID")

	assert.Equal(t, "builtintool", resultPayload.Name)
	assert.Equal(t, transcript.FidelityRouter, resultPayload.Fidelity, "tool.result must have Fidelity:router")
	assert.Equal(t, callPayload.CallID, resultPayload.CallID, "both events must carry same CallID")
	assert.NotNil(t, resultPayload.Output, "tool.result must have Output")
}

// TestRouter_ToolCallAndResultCarrySameCallID verifies CallID consistency.
func TestRouter_ToolCallAndResultCarrySameCallID(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	rec := &mockRecorder{}
	router.SetRecorder(rec)

	provider := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool1", Source: "src"},
		},
	}
	router.Register(context.Background(), provider)

	router.CallTool(context.Background(), "tool1", map[string]any{})

	events := rec.getEvents()
	require.Len(t, events, 2)

	callID1 := events[0].Payload.(*transcript.ToolPayload).CallID
	callID2 := events[1].Payload.(*transcript.ToolPayload).CallID

	assert.Equal(t, callID1, callID2, "tool.call and tool.result must carry same CallID")
}

// TestRouter_RecorderErrorsLoggedAsWarn verifies Record errors don't propagate.
func TestRouter_RecorderErrorsLoggedAsWarn(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	recErr := errors.New("recorder failed")
	rec := &mockRecorder{recordErr: recErr}
	router.SetRecorder(rec)

	provider := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool1", Source: "src"},
		},
	}
	router.Register(context.Background(), provider)

	result, err := router.CallTool(context.Background(), "tool1", map[string]any{})

	assert.NoError(t, err, "CallTool must not propagate recorder errors")
	assert.NotNil(t, result, "CallTool must return result despite recorder error")

	logger.mu.Lock()
	warnLogs := logger.warnLogs
	logger.mu.Unlock()
	assert.NotEmpty(t, warnLogs, "recorder error must be logged at WARN level (T049 convention)")
}

// TestRouter_ToolResultPayloadIncludesOutput verifies tool.result contains output.
func TestRouter_ToolResultPayloadIncludesOutput(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	rec := &mockRecorder{}
	router.SetRecorder(rec)

	provider := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool1", Source: "src"},
		},
	}
	router.Register(context.Background(), provider)

	result, err := router.CallTool(context.Background(), "tool1", map[string]any{})

	require.NoError(t, err)
	require.NotNil(t, result)

	events := rec.getEvents()
	require.Len(t, events, 2)

	resultPayload := events[1].Payload.(*transcript.ToolPayload)
	assert.NotNil(t, resultPayload.Output, "tool.result payload must include Output field")
	assert.Equal(t, result, resultPayload.Output, "tool.result Output must match CallTool return value")
}

// TestRouter_ToolResultPayloadIncludesError verifies tool.result captures error string on failure.
func TestRouter_ToolResultPayloadIncludesError(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	rec := &mockRecorder{}
	router.SetRecorder(rec)

	testErr := errors.New("tool execution failed")
	provider := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool1", Source: "src"},
		},
		callToolErr: testErr,
	}
	router.Register(context.Background(), provider)

	result, err := router.CallTool(context.Background(), "tool1", map[string]any{})

	assert.Error(t, err)
	assert.Nil(t, result)

	events := rec.getEvents()
	require.Len(t, events, 2)

	resultPayload := events[1].Payload.(*transcript.ToolPayload)
	assert.Equal(t, "tool execution failed", resultPayload.Error)
}

// TestRouter_FidelityDistinctFromAgentEmitted verifies router Fidelity is distinct from agent_emitted.
func TestRouter_FidelityDistinctFromAgentEmitted(t *testing.T) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	rec := &mockRecorder{}
	router.SetRecorder(rec)

	provider := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool1", Source: "src"},
		},
	}
	router.Register(context.Background(), provider)

	router.CallTool(context.Background(), "tool1", map[string]any{})

	events := rec.getEvents()
	require.Len(t, events, 2)

	callPayload := events[0].Payload.(*transcript.ToolPayload)
	resultPayload := events[1].Payload.(*transcript.ToolPayload)

	assert.Equal(t, transcript.FidelityRouter, callPayload.Fidelity)
	assert.Equal(t, transcript.FidelityRouter, resultPayload.Fidelity)
	assert.NotEqual(t, transcript.FidelityAgentEmitted, callPayload.Fidelity)
	assert.NotEqual(t, transcript.FidelityAgentEmitted, resultPayload.Fidelity)
}

// BenchmarkRouterCallTool_WithRecorder measures overhead of transcript recording.
// Should be <5% overhead compared to baseline (no recorder).
func BenchmarkRouterCallTool_WithRecorder(b *testing.B) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	rec := &mockRecorder{}
	router.SetRecorder(rec)

	provider := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "benchedtool", Source: "bench"},
		},
	}
	router.Register(context.Background(), provider)

	args := map[string]any{"key": "value"}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		router.CallTool(context.Background(), "benchedtool", args)
	}
}

// BenchmarkRouterCallTool_WithoutRecorder measures baseline overhead without recorder.
func BenchmarkRouterCallTool_WithoutRecorder(b *testing.B) {
	tracer := &mockTracer{}
	logger := &mockLogger{}
	router := NewRouter(tracer, logger)

	provider := &mockToolProvider{
		tools: []ports.ToolDefinition{
			{Name: "benchedtool", Source: "bench"},
		},
	}
	router.Register(context.Background(), provider)

	args := map[string]any{"key": "value"}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		router.CallTool(context.Background(), "benchedtool", args)
	}
}
