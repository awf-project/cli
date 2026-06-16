package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
)

func TestProjectEvent_StepPayloadToEnrichedStepPayload(t *testing.T) {
	tests := []struct {
		name            string
		ev              transcript.ExchangeEvent
		wantKind        ports.EventKind
		wantStepName    string
		wantError       string
		wantDurationMs  int64
		shouldHaveError bool
	}{
		{
			name: "step.started with step payload",
			ev: transcript.ExchangeEvent{
				Seq:       1,
				RunID:     "run-123",
				Type:      transcript.EventTypeStepStarted,
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Payload: &transcript.StepPayload{
					Name:  "my-step",
					Kind:  "shell",
					Error: "",
				},
			},
			wantKind:        ports.EventStepStarted,
			wantStepName:    "my-step",
			wantError:       "",
			shouldHaveError: false,
		},
		{
			name: "step.completed with error",
			ev: transcript.ExchangeEvent{
				Seq:       2,
				RunID:     "run-123",
				Type:      transcript.EventTypeStepCompleted,
				Timestamp: time.Date(2024, 1, 1, 12, 0, 5, 0, time.UTC),
				Payload: &transcript.StepPayload{
					Name:  "my-step",
					Kind:  "shell",
					Error: "step failed",
				},
			},
			wantKind:        ports.EventStepCompleted,
			wantStepName:    "my-step",
			wantError:       "step failed",
			shouldHaveError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ProjectEvent(tt.ev)

			if tt.shouldHaveError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantKind, result.Kind)
			assert.Equal(t, tt.ev.Seq, result.Seq)
			assert.Equal(t, tt.ev.RunID, result.RunID)

			// Verify enriched step payload
			enriched, ok := result.Payload.(*ports.EnrichedStepPayload)
			require.True(t, ok, "payload should be *EnrichedStepPayload")
			assert.Equal(t, tt.wantStepName, enriched.StepName)
			assert.Equal(t, tt.wantError, enriched.Error)
		})
	}
}

func TestProjectEvent_MessagePayloadToEnrichedMessagePayload(t *testing.T) {
	tests := []struct {
		name            string
		ev              transcript.ExchangeEvent
		wantContent     string
		shouldHaveError bool
	}{
		{
			name: "single text block",
			ev: transcript.ExchangeEvent{
				Seq:   3,
				RunID: "run-123",
				Type:  transcript.EventTypeMessageAssistant,
				Payload: &transcript.MessagePayload{
					Role: "assistant",
					Blocks: []transcript.ContentBlock{
						{Type: transcript.BlockTypeText, Text: "Hello world"},
					},
				},
			},
			wantContent:     "Hello world",
			shouldHaveError: false,
		},
		{
			name: "multiple text blocks concatenated",
			ev: transcript.ExchangeEvent{
				Seq:   4,
				RunID: "run-123",
				Type:  transcript.EventTypeMessageAssistant,
				Payload: &transcript.MessagePayload{
					Role: "assistant",
					Blocks: []transcript.ContentBlock{
						{Type: transcript.BlockTypeText, Text: "First"},
						{Type: transcript.BlockTypeText, Text: "Second"},
						{Type: transcript.BlockTypeText, Text: "Third"},
					},
				},
			},
			wantContent:     "FirstSecondThird",
			shouldHaveError: false,
		},
		{
			name: "empty message blocks",
			ev: transcript.ExchangeEvent{
				Seq:   5,
				RunID: "run-123",
				Type:  transcript.EventTypeMessageAssistant,
				Payload: &transcript.MessagePayload{
					Role:   "assistant",
					Blocks: []transcript.ContentBlock{},
				},
			},
			wantContent:     "",
			shouldHaveError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ProjectEvent(tt.ev)

			if tt.shouldHaveError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, ports.EventMessageAssistant, result.Kind)

			// Verify enriched message payload
			enriched, ok := result.Payload.(*ports.EnrichedMessagePayload)
			require.True(t, ok, "payload should be *EnrichedMessagePayload")
			assert.Equal(t, tt.wantContent, enriched.Content)
		})
	}
}

func TestProjectEvent_UnknownEventType(t *testing.T) {
	ev := transcript.ExchangeEvent{
		Seq:       99,
		RunID:     "run-123",
		Type:      transcript.EventType("invalid.event.type"),
		Timestamp: time.Now(),
		Payload:   nil,
	}

	result, err := ProjectEvent(ev)

	assert.Error(t, err)
	assert.Equal(t, ports.EventKindUnknown, result.Kind)
}

func TestProjectEvent_PreservesEventMetadata(t *testing.T) {
	ts := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)
	ev := transcript.ExchangeEvent{
		Seq:         42,
		RunID:       "run-abc-123",
		ParentRunID: "parent-xyz",
		Type:        transcript.EventTypeRunStarted,
		Timestamp:   ts,
		Payload:     nil,
	}

	result, err := ProjectEvent(ev)

	require.NoError(t, err)
	assert.Equal(t, uint64(42), result.Seq)
	assert.Equal(t, "run-abc-123", result.RunID)
	assert.Equal(t, "parent-xyz", result.ParentRunID)
	assert.Equal(t, ts, result.Timestamp)
}

func TestProjectPayload_StepPayload(t *testing.T) {
	tests := []struct {
		name           string
		payload        any
		expectedType   string
		expectStepName string
		expectError    string
	}{
		{
			name: "step payload with values",
			payload: &transcript.StepPayload{
				Name:  "process-data",
				Kind:  "shell",
				Error: "execution timeout",
			},
			expectedType:   "*ports.EnrichedStepPayload",
			expectStepName: "process-data",
			expectError:    "execution timeout",
		},
		{
			name: "step payload without error",
			payload: &transcript.StepPayload{
				Name: "validate",
				Kind: "workflow",
			},
			expectedType:   "*ports.EnrichedStepPayload",
			expectStepName: "validate",
			expectError:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := projectPayload(tt.payload)

			enriched, ok := result.(*ports.EnrichedStepPayload)
			require.True(t, ok, "should return *EnrichedStepPayload")
			assert.Equal(t, tt.expectStepName, enriched.StepName)
			assert.Equal(t, tt.expectError, enriched.Error)
		})
	}
}

// TestProjectPayload_StepPayload_CarriesOutput verifies that the (in-memory, json:"-")
// Output/Stderr captured on a completed step's transcript payload are projected onto the
// facade EnrichedStepPayload, so event-only consumers (TUI monitoring, SSE) can render
// per-step output without an ExecutionContext.
func TestProjectPayload_StepPayload_CarriesOutput(t *testing.T) {
	payload := &transcript.StepPayload{
		Name:   "greet",
		Kind:   "command",
		Result: true,
		Output: "Hello, World!\n",
		Stderr: "warning: deprecated\n",
	}

	result := projectPayload(payload)

	enriched, ok := result.(*ports.EnrichedStepPayload)
	require.True(t, ok, "should return *EnrichedStepPayload")
	assert.Equal(t, "greet", enriched.StepName)
	assert.True(t, enriched.HadOutput)
	assert.Equal(t, "Hello, World!\n", enriched.Output)
	assert.Equal(t, "warning: deprecated\n", enriched.Stderr)
}

func TestProjectPayload_MessagePayload(t *testing.T) {
	tests := []struct {
		name            string
		payload         any
		expectedContent string
	}{
		{
			name: "message with text",
			payload: &transcript.MessagePayload{
				Role: "assistant",
				Blocks: []transcript.ContentBlock{
					{Type: transcript.BlockTypeText, Text: "Response text"},
				},
			},
			expectedContent: "Response text",
		},
		{
			name: "empty message",
			payload: &transcript.MessagePayload{
				Role:   "assistant",
				Blocks: []transcript.ContentBlock{},
			},
			expectedContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := projectPayload(tt.payload)

			enriched, ok := result.(*ports.EnrichedMessagePayload)
			require.True(t, ok, "should return *EnrichedMessagePayload")
			assert.Equal(t, tt.expectedContent, enriched.Content)
		})
	}
}

// TestProjectPayload_UnmappedPayload verifies the fail-closed policy: any payload
// type that is not explicitly handled returns nil rather than leaking an internal
// transcript type to facade consumers (consistent with mapEventKind returning
// EventKindUnknown for unrecognized event types).
func TestProjectPayload_UnmappedPayload(t *testing.T) {
	payload := []transcript.ContentBlock{
		{Type: transcript.BlockTypeText, Text: "raw content"},
	}

	result := projectPayload(payload)

	assert.Nil(t, result, "unrecognized payload types must return nil (fail-closed policy)")
}

func TestProjectEvent_InvalidContentBlocks(t *testing.T) {
	// Payloads are *transcript.MessagePayload (not raw []ContentBlock); the corrected
	// validateContentBlocks unwraps the message struct before checking block types (M3).
	ev := transcript.ExchangeEvent{
		Seq:       1,
		RunID:     "run-123",
		Type:      transcript.EventTypeMessageUser,
		Timestamp: time.Now(),
		Payload: &transcript.MessagePayload{
			Role: "user",
			Blocks: []transcript.ContentBlock{
				{Type: transcript.BlockType("invalid.block.type")},
			},
		},
	}

	_, err := ProjectEvent(ev)

	assert.Error(t, err)
}

func TestProjectEvent_AllEventTypes(t *testing.T) {
	tests := []struct {
		name         string
		eventType    transcript.EventType
		expectedKind ports.EventKind
	}{
		{name: "run.started", eventType: transcript.EventTypeRunStarted, expectedKind: ports.EventRunStarted},
		{name: "run.completed", eventType: transcript.EventTypeRunCompleted, expectedKind: ports.EventRunCompleted},
		{name: "step.started", eventType: transcript.EventTypeStepStarted, expectedKind: ports.EventStepStarted},
		{name: "step.completed", eventType: transcript.EventTypeStepCompleted, expectedKind: ports.EventStepCompleted},
		{name: "step.call_workflow.started", eventType: transcript.EventTypeStepCallWorkflowStarted, expectedKind: ports.EventStepCallWorkflowStarted},
		{name: "step.call_workflow.completed", eventType: transcript.EventTypeStepCallWorkflowCompleted, expectedKind: ports.EventStepCallWorkflowCompleted},
		{name: "message.user", eventType: transcript.EventTypeMessageUser, expectedKind: ports.EventMessageUser},
		{name: "message.assistant", eventType: transcript.EventTypeMessageAssistant, expectedKind: ports.EventMessageAssistant},
		{name: "tool.call", eventType: transcript.EventTypeToolCall, expectedKind: ports.EventToolCall},
		{name: "tool.result", eventType: transcript.EventTypeToolResult, expectedKind: ports.EventToolResult},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := transcript.ExchangeEvent{
				Seq:       1,
				RunID:     "run-123",
				Type:      tt.eventType,
				Timestamp: time.Now(),
				Payload:   nil,
			}

			result, err := ProjectEvent(ev)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedKind, result.Kind)
		})
	}
}
