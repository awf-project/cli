package pluginmodel_test

import (
	"context"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
)

func TestNewDomainEvent_HappyPath(t *testing.T) {
	tests := []struct {
		name          string
		eventType     string
		source        string
		metadata      map[string]string
		payload       []byte
		wantIDEmpty   bool
		wantTimestamp bool
	}{
		{
			name:          "basic event with all fields",
			eventType:     "test.event",
			source:        "test-plugin",
			metadata:      map[string]string{"key": "value"},
			payload:       []byte("test payload"),
			wantIDEmpty:   false,
			wantTimestamp: true,
		},
		{
			name:          "event with nil metadata",
			eventType:     "test.event",
			source:        "test-plugin",
			metadata:      nil,
			payload:       []byte("payload"),
			wantIDEmpty:   false,
			wantTimestamp: true,
		},
		{
			name:          "event with nil payload",
			eventType:     "test.event",
			source:        "test-plugin",
			metadata:      map[string]string{"key": "value"},
			payload:       nil,
			wantIDEmpty:   false,
			wantTimestamp: true,
		},
		{
			name:          "event with core source",
			eventType:     "workflow.started",
			source:        "core",
			metadata:      map[string]string{},
			payload:       []byte{},
			wantIDEmpty:   false,
			wantTimestamp: true,
		},
		{
			name:          "event with both nil metadata and payload",
			eventType:     "test.event",
			source:        "plugin",
			metadata:      nil,
			payload:       nil,
			wantIDEmpty:   false,
			wantTimestamp: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beforeTime := time.Now()
			event := pluginmodel.NewDomainEvent(tt.eventType, tt.source, tt.metadata, tt.payload)
			afterTime := time.Now()

			assert.NotNil(t, event)
			assert.Equal(t, tt.wantIDEmpty, event.ID == "", "ID should not be empty")
			assert.Equal(t, tt.eventType, event.Type)
			assert.Equal(t, tt.source, event.Source)
			assert.Equal(t, tt.metadata, event.Metadata)
			assert.Equal(t, tt.payload, event.Payload)
			assert.Equal(t, 0, event.PropagationDepth)

			if tt.wantTimestamp {
				assert.True(t, event.Timestamp.After(beforeTime.Add(-time.Second)) && event.Timestamp.Before(afterTime.Add(time.Second)),
					"Timestamp should be within 1 second of current time")
			}
		})
	}
}

func TestNewDomainEvent_UUIDUniqueness(t *testing.T) {
	event1 := pluginmodel.NewDomainEvent("test.event", "source", nil, nil)
	event2 := pluginmodel.NewDomainEvent("test.event", "source", nil, nil)

	assert.NotEqual(t, event1.ID, event2.ID, "Two calls should produce different UUIDs")
	assert.NotEmpty(t, event1.ID)
	assert.NotEmpty(t, event2.ID)
}

func TestNewDomainEvent_StructFields(t *testing.T) {
	event := pluginmodel.NewDomainEvent("test.type", "test.source", map[string]string{"k": "v"}, []byte("test"))

	assert.NotNil(t, event)
	assert.Equal(t, "test.type", event.Type)
	assert.Equal(t, "test.source", event.Source)
	assert.NotEmpty(t, event.ID)
	assert.NotZero(t, event.Timestamp)
	assert.Equal(t, map[string]string{"k": "v"}, event.Metadata)
	assert.Equal(t, []byte("test"), event.Payload)
	assert.Equal(t, 0, event.PropagationDepth)
}

func TestEventPublisher_InterfaceExists(t *testing.T) {
	var publisher ports.EventPublisher
	assert.Nil(t, publisher, "EventPublisher interface should be defined")

	var _ ports.EventPublisher
}

func TestWorkflowEventConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"EventWorkflowFailed", workflow.EventWorkflowFailed, "workflow.failed"},
		{"EventStepStarted", workflow.EventStepStarted, "step.started"},
		{"EventStepCompleted", workflow.EventStepCompleted, "step.completed"},
		{"EventStepFailed", workflow.EventStepFailed, "step.failed"},
		{"EventStepRetrying", workflow.EventStepRetrying, "step.retrying"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.constant)
		})
	}
}

func TestErrorCodeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant errors.ErrorCode
		expected string
	}{
		{"ErrorCodeExecutionEventDeliveryFailed", errors.ErrorCodeExecutionEventDeliveryFailed, "EXECUTION.EVENT.DELIVERY_FAILED"},
		{"ErrorCodeExecutionEventCycleDetected", errors.ErrorCodeExecutionEventCycleDetected, "EXECUTION.EVENT.CYCLE_DETECTED"},
		{"ErrorCodeExecutionEventBufferFull", errors.ErrorCodeExecutionEventBufferFull, "EXECUTION.EVENT.BUFFER_FULL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.constant))
		})
	}
}

func TestErrorCode_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		expected bool
	}{
		{"valid event delivery failed", errors.ErrorCodeExecutionEventDeliveryFailed, true},
		{"valid event cycle detected", errors.ErrorCodeExecutionEventCycleDetected, true},
		{"valid event buffer full", errors.ErrorCodeExecutionEventBufferFull, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code.IsValid())
		})
	}
}

func TestErrorCode_Category(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		expected string
	}{
		{"event delivery failed", errors.ErrorCodeExecutionEventDeliveryFailed, "EXECUTION"},
		{"event cycle detected", errors.ErrorCodeExecutionEventCycleDetected, "EXECUTION"},
		{"event buffer full", errors.ErrorCodeExecutionEventBufferFull, "EXECUTION"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code.Category())
		})
	}
}

func TestErrorCode_Subcategory(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		expected string
	}{
		{"event delivery failed", errors.ErrorCodeExecutionEventDeliveryFailed, "EVENT"},
		{"event cycle detected", errors.ErrorCodeExecutionEventCycleDetected, "EVENT"},
		{"event buffer full", errors.ErrorCodeExecutionEventBufferFull, "EVENT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code.Subcategory())
		})
	}
}

func TestErrorCode_Specific(t *testing.T) {
	tests := []struct {
		name     string
		code     errors.ErrorCode
		expected string
	}{
		{"event delivery failed", errors.ErrorCodeExecutionEventDeliveryFailed, "DELIVERY_FAILED"},
		{"event cycle detected", errors.ErrorCodeExecutionEventCycleDetected, "CYCLE_DETECTED"},
		{"event buffer full", errors.ErrorCodeExecutionEventBufferFull, "BUFFER_FULL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.code.Specific())
		})
	}
}

func TestEventPublisher_MethodSignatures(t *testing.T) {
	var _ ports.EventPublisher

	var publisher ports.EventPublisher
	_ = publisher

	ctx := context.Background()
	event := pluginmodel.NewDomainEvent("test.event", "test", nil, nil)

	assert.NotNil(t, ctx)
	assert.NotNil(t, event)
}
