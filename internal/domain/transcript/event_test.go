package transcript_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/transcript"
)

// TestExchangeEventMarshalJSON_DeterministicFieldOrder verifies that ExchangeEvent.MarshalJSON
// emits fields in exact order: seq,run_id,parent_run_id,child_run_id,type,path,iteration,timestamp,payload
func TestExchangeEventMarshalJSON_DeterministicFieldOrder(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	event := transcript.ExchangeEvent{
		Seq:         42,
		RunID:       "run-123",
		ParentRunID: "parent-456",
		ChildRunID:  "child-789",
		Type:        transcript.EventTypeRunStarted,
		Path:        "step[0]",
		Iteration:   1,
		Timestamp:   now,
		Payload:     &transcript.StepPayload{Name: "test-step"},
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	// Verify field order in JSON output
	jsonStr := string(data)
	seqPos := indexOfKey(jsonStr, "seq")
	runIDPos := indexOfKey(jsonStr, "run_id")
	parentRunIDPos := indexOfKey(jsonStr, "parent_run_id")
	childRunIDPos := indexOfKey(jsonStr, "child_run_id")
	typePos := indexOfKey(jsonStr, "type")
	pathPos := indexOfKey(jsonStr, "path")
	iterationPos := indexOfKey(jsonStr, "iteration")
	timestampPos := indexOfKey(jsonStr, "timestamp")
	payloadPos := indexOfKey(jsonStr, "payload")

	assert.True(t, seqPos < runIDPos, "seq should come before run_id")
	assert.True(t, runIDPos < parentRunIDPos, "run_id should come before parent_run_id")
	assert.True(t, parentRunIDPos < childRunIDPos, "parent_run_id should come before child_run_id")
	assert.True(t, childRunIDPos < typePos, "child_run_id should come before type")
	assert.True(t, typePos < pathPos, "type should come before path")
	assert.True(t, pathPos < iterationPos, "path should come before iteration")
	assert.True(t, iterationPos < timestampPos, "iteration should come before timestamp")
	assert.True(t, timestampPos < payloadPos, "timestamp should come before payload")
}

// TestExchangeEventMarshalJSON_OmitEmptyParentRunID verifies that empty ParentRunID is omitted
func TestExchangeEventMarshalJSON_OmitEmptyParentRunID(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	event := transcript.ExchangeEvent{
		Seq:         1,
		RunID:       "run-123",
		ParentRunID: "",
		ChildRunID:  "",
		Type:        transcript.EventTypeRunStarted,
		Path:        "step[0]",
		Iteration:   0,
		Timestamp:   now,
		Payload:     nil,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.NotContains(t, jsonStr, "parent_run_id", "empty parent_run_id should be omitted")
	assert.NotContains(t, jsonStr, "child_run_id", "empty child_run_id should be omitted")
}

// TestExchangeEventMarshalJSON_IncludeNonEmptyParentRunID verifies that non-empty ParentRunID is included
func TestExchangeEventMarshalJSON_IncludeNonEmptyParentRunID(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	event := transcript.ExchangeEvent{
		Seq:         1,
		RunID:       "run-123",
		ParentRunID: "parent-456",
		ChildRunID:  "",
		Type:        transcript.EventTypeRunStarted,
		Path:        "step[0]",
		Iteration:   0,
		Timestamp:   now,
		Payload:     nil,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.Contains(t, jsonStr, "parent_run_id", "non-empty parent_run_id should be included")
	assert.NotContains(t, jsonStr, "child_run_id", "empty child_run_id should be omitted")
}

// TestExchangeEventRoundTrip verifies that json.Marshal → json.Unmarshal recovers struct equality
func TestExchangeEventRoundTrip(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		event transcript.ExchangeEvent
	}{
		{
			name: "with message payload",
			event: transcript.ExchangeEvent{
				Seq:       1,
				RunID:     "run-123",
				Type:      transcript.EventTypeMessageUser,
				Path:      "step[0]",
				Iteration: 0,
				Timestamp: now,
				Payload: &transcript.MessagePayload{
					Role: "user",
					Blocks: []transcript.ContentBlock{
						{
							Type:     transcript.BlockTypeText,
							Fidelity: transcript.FidelityRouter,
							Text:     "hello",
						},
					},
				},
			},
		},
		{
			name: "with step payload",
			event: transcript.ExchangeEvent{
				Seq:       2,
				RunID:     "run-456",
				Type:      transcript.EventTypeStepCompleted,
				Path:      "step[1]",
				Iteration: 1,
				Timestamp: now,
				Payload: &transcript.StepPayload{
					Name:   "test-step",
					Kind:   "shell",
					Result: "success",
				},
			},
		},
		{
			name: "with parent and child run ids",
			event: transcript.ExchangeEvent{
				Seq:         3,
				RunID:       "run-789",
				ParentRunID: "parent-123",
				ChildRunID:  "child-456",
				Type:        transcript.EventTypeStepCallWorkflowStarted,
				Path:        "step[2]",
				Iteration:   0,
				Timestamp:   now,
				Payload:     &transcript.ToolPayload{Name: "workflow"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.event)
			require.NoError(t, err)

			var recovered transcript.ExchangeEvent
			err = json.Unmarshal(data, &recovered)
			require.NoError(t, err)

			assert.Equal(t, tt.event, recovered)
		})
	}
}

// TestExchangeEventUnmarshalJSON_UnknownEventType verifies that unknown EventType returns ErrUnknownEventType
func TestExchangeEventUnmarshalJSON_UnknownEventType(t *testing.T) {
	data := []byte(`{
		"seq": 1,
		"run_id": "run-123",
		"type": "unknown.event.type",
		"path": "step[0]",
		"iteration": 0,
		"timestamp": "2024-01-01T12:00:00Z",
		"payload": null
	}`)

	var event transcript.ExchangeEvent
	err := json.Unmarshal(data, &event)
	assert.ErrorIs(t, err, transcript.ErrUnknownEventType)
}

// TestExchangeEventUnmarshalJSON_AllEventTypes verifies unmarshaling all valid EventType values
func TestExchangeEventUnmarshalJSON_AllEventTypes(t *testing.T) {
	tests := []struct {
		name      string
		eventType transcript.EventType
	}{
		{name: "run.started", eventType: transcript.EventTypeRunStarted},
		{name: "run.completed", eventType: transcript.EventTypeRunCompleted},
		{name: "step.started", eventType: transcript.EventTypeStepStarted},
		{name: "step.completed", eventType: transcript.EventTypeStepCompleted},
		{name: "step.call_workflow.started", eventType: transcript.EventTypeStepCallWorkflowStarted},
		{name: "step.call_workflow.completed", eventType: transcript.EventTypeStepCallWorkflowCompleted},
		{name: "message.user", eventType: transcript.EventTypeMessageUser},
		{name: "message.assistant", eventType: transcript.EventTypeMessageAssistant},
		{name: "tool.call", eventType: transcript.EventTypeToolCall},
		{name: "tool.result", eventType: transcript.EventTypeToolResult},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData := []byte(`{
				"seq": 1,
				"run_id": "run-123",
				"type": "` + string(tt.eventType) + `",
				"path": "step[0]",
				"iteration": 0,
				"timestamp": "2024-01-01T12:00:00Z",
				"payload": null
			}`)

			var event transcript.ExchangeEvent
			err := json.Unmarshal(jsonData, &event)
			require.NoError(t, err)
			assert.Equal(t, tt.eventType, event.Type)
		})
	}
}

// TestExchangeEventMarshalJSON_AllPayloadVariants verifies marshaling all payload types
func TestExchangeEventMarshalJSON_AllPayloadVariants(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		payload any
	}{
		{
			name:    "message payload",
			payload: &transcript.MessagePayload{Role: "user"},
		},
		{
			name:    "step payload",
			payload: &transcript.StepPayload{Name: "step-1"},
		},
		{
			name:    "tool payload",
			payload: &transcript.ToolPayload{Name: "bash"},
		},
		{
			name:    "content block array",
			payload: []transcript.ContentBlock{{Type: transcript.BlockTypeText}},
		},
		{
			name:    "nil payload",
			payload: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := transcript.ExchangeEvent{
				Seq:       1,
				RunID:     "run-123",
				Type:      transcript.EventTypeRunStarted,
				Path:      "step[0]",
				Iteration: 0,
				Timestamp: now,
				Payload:   tt.payload,
			}

			data, err := json.Marshal(event)
			require.NoError(t, err)
			assert.NotNil(t, data)
		})
	}
}

// TestExchangeEventFields verifies all ExchangeEvent fields are accessible
func TestExchangeEventFields(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	event := transcript.ExchangeEvent{
		Seq:         42,
		RunID:       "run-123",
		ParentRunID: "parent-456",
		ChildRunID:  "child-789",
		Type:        transcript.EventTypeRunStarted,
		Path:        "step[0]",
		Iteration:   5,
		Timestamp:   now,
		Payload:     nil,
	}

	assert.Equal(t, uint64(42), event.Seq)
	assert.Equal(t, "run-123", event.RunID)
	assert.Equal(t, "parent-456", event.ParentRunID)
	assert.Equal(t, "child-789", event.ChildRunID)
	assert.Equal(t, transcript.EventTypeRunStarted, event.Type)
	assert.Equal(t, "step[0]", event.Path)
	assert.Equal(t, 5, event.Iteration)
	assert.Equal(t, now, event.Timestamp)
	assert.Nil(t, event.Payload)
}

// TestEventTypeEnumCoverage verifies all EventType constants are defined
func TestEventTypeEnumCoverage(t *testing.T) {
	assert.Equal(t, transcript.EventType("run.started"), transcript.EventTypeRunStarted)
	assert.Equal(t, transcript.EventType("run.completed"), transcript.EventTypeRunCompleted)
	assert.Equal(t, transcript.EventType("step.started"), transcript.EventTypeStepStarted)
	assert.Equal(t, transcript.EventType("step.completed"), transcript.EventTypeStepCompleted)
	assert.Equal(t, transcript.EventType("step.call_workflow.started"), transcript.EventTypeStepCallWorkflowStarted)
	assert.Equal(t, transcript.EventType("step.call_workflow.completed"), transcript.EventTypeStepCallWorkflowCompleted)
	assert.Equal(t, transcript.EventType("message.user"), transcript.EventTypeMessageUser)
	assert.Equal(t, transcript.EventType("message.assistant"), transcript.EventTypeMessageAssistant)
	assert.Equal(t, transcript.EventType("tool.call"), transcript.EventTypeToolCall)
	assert.Equal(t, transcript.EventType("tool.result"), transcript.EventTypeToolResult)
}

// indexOfKey finds the position of a JSON key (followed by colon) to distinguish
// keys from string values that may share the same text (e.g. key "text" vs value "text").
func indexOfKey(jsonStr, key string) int {
	pos := strings.Index(jsonStr, "\""+key+"\":")
	return pos
}
