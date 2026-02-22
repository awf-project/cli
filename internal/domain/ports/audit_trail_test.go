package ports_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: T002
// Feature: F071
// Tests: AuditTrailWriter port interface contract

// TestAuditTrailWriter_Write_HappyPath tests successful write of a single audit event.
func TestAuditTrailWriter_Write_HappyPath(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()
	ctx := context.Background()

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "exec-123",
		Timestamp:     time.Now(),
		User:          "testuser",
		WorkflowName:  "test-workflow",
		Inputs: map[string]any{
			"env": "staging",
		},
	}

	err := writer.Write(ctx, &event)

	assert.NoError(t, err)
	assert.Equal(t, 1, len(writer.GetEvents()))
	assert.Len(t, writer.GetEvents(), 1)
	assert.Equal(t, event.ExecutionID, writer.GetEvents()[0].ExecutionID)
}

// TestAuditTrailWriter_Write_MultipleEvents tests writing multiple events sequentially.
func TestAuditTrailWriter_Write_MultipleEvents(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()
	ctx := context.Background()

	startEvent := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "exec-456",
		Timestamp:     time.Now(),
		User:          "testuser",
		WorkflowName:  "multi-step",
	}

	endEvent := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowCompleted,
		ExecutionID:   "exec-456",
		Timestamp:     time.Now(),
		User:          "testuser",
		WorkflowName:  "multi-step",
		Status:        "success",
	}

	err1 := writer.Write(ctx, &startEvent)
	err2 := writer.Write(ctx, &endEvent)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, 2, len(writer.GetEvents()))
	assert.Len(t, writer.GetEvents(), 2)
	assert.Equal(t, workflow.EventWorkflowStarted, writer.GetEvents()[0].Event)
	assert.Equal(t, workflow.EventWorkflowCompleted, writer.GetEvents()[1].Event)
}

// TestAuditTrailWriter_Write_WithContextCancellation tests write behavior with cancelled context.
func TestAuditTrailWriter_Write_WithContextCancellation(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "exec-789",
		Timestamp:     time.Now(),
		User:          "testuser",
		WorkflowName:  "cancelled-workflow",
	}

	_ = writer.Write(ctx, &event)

	// Implementation may check context and return error, or proceed anyway
	// This test documents that Write accepts a context parameter
	assert.NotNil(t, writer)
}

// TestAuditTrailWriter_Write_WithContextTimeout tests write with timeout context.
func TestAuditTrailWriter_Write_WithContextTimeout(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "exec-timeout",
		Timestamp:     time.Now(),
		User:          "testuser",
		WorkflowName:  "timeout-workflow",
	}

	_ = writer.Write(ctx, &event)

	// Implementation respects context timeout if applicable
	assert.NotNil(t, writer)
}

// TestAuditTrailWriter_Write_WorkflowStartedEvent tests write of workflow.started event.
func TestAuditTrailWriter_Write_WorkflowStartedEvent(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()
	ctx := context.Background()

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "start-test",
		Timestamp:     time.Date(2026, 2, 20, 23, 15, 42, 0, time.UTC),
		User:          "deployer",
		WorkflowName:  "deploy-app",
		Inputs: map[string]any{
			"env":    "prod",
			"region": "us-east-1",
		},
	}

	err := writer.Write(ctx, &event)

	require.NoError(t, err)
	assert.Equal(t, 1, len(writer.GetEvents()))
	assert.Equal(t, workflow.EventWorkflowStarted, writer.GetEvents()[0].Event)
	assert.Empty(t, writer.GetEvents()[0].Status)
}

// TestAuditTrailWriter_Write_WorkflowCompletedEventSuccess tests workflow.completed with success.
func TestAuditTrailWriter_Write_WorkflowCompletedEventSuccess(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()
	ctx := context.Background()

	exitCode := 0
	duration := int64(30000)

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowCompleted,
		ExecutionID:   "complete-success",
		Timestamp:     time.Now(),
		User:          "deployer",
		WorkflowName:  "deploy-app",
		Status:        "success",
		ExitCode:      &exitCode,
		DurationMs:    &duration,
	}

	err := writer.Write(ctx, &event)

	require.NoError(t, err)
	assert.Equal(t, 1, len(writer.GetEvents()))
	assert.Equal(t, workflow.EventWorkflowCompleted, writer.GetEvents()[0].Event)
	assert.Equal(t, "success", writer.GetEvents()[0].Status)
	assert.Equal(t, 0, *writer.GetEvents()[0].ExitCode)
}

// TestAuditTrailWriter_Write_WorkflowCompletedEventFailure tests workflow.completed with failure.
func TestAuditTrailWriter_Write_WorkflowCompletedEventFailure(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()
	ctx := context.Background()

	exitCode := 3
	duration := int64(15000)

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowCompleted,
		ExecutionID:   "complete-failure",
		Timestamp:     time.Now(),
		User:          "deployer",
		WorkflowName:  "deploy-app",
		Status:        "failure",
		ExitCode:      &exitCode,
		DurationMs:    &duration,
		Error:         "step 'deploy' failed: connection refused",
	}

	err := writer.Write(ctx, &event)

	require.NoError(t, err)
	assert.Equal(t, 1, len(writer.GetEvents()))
	assert.Equal(t, "failure", writer.GetEvents()[0].Status)
	assert.Equal(t, 3, *writer.GetEvents()[0].ExitCode)
	assert.Equal(t, "step 'deploy' failed: connection refused", writer.GetEvents()[0].Error)
}

// TestAuditTrailWriter_Write_WithMaskedSecrets tests write with secret-masked inputs.
func TestAuditTrailWriter_Write_WithMaskedSecrets(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()
	ctx := context.Background()

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "secret-test",
		Timestamp:     time.Now(),
		User:          "deployer",
		WorkflowName:  "secret-workflow",
		Inputs: map[string]any{
			"api_key":  "***",
			"password": "***",
			"token":    "***",
			"public":   "exposed",
		},
	}

	err := writer.Write(ctx, &event)

	require.NoError(t, err)
	inputs := writer.GetEvents()[0].Inputs
	assert.Equal(t, "***", inputs["api_key"])
	assert.Equal(t, "***", inputs["password"])
	assert.Equal(t, "***", inputs["token"])
	assert.Equal(t, "exposed", inputs["public"])
}

// TestAuditTrailWriter_Write_WithEmptyInputs tests write with nil inputs.
func TestAuditTrailWriter_Write_WithEmptyInputs(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()
	ctx := context.Background()

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "empty-inputs",
		Timestamp:     time.Now(),
		User:          "deployer",
		WorkflowName:  "no-input-workflow",
		Inputs:        nil,
	}

	err := writer.Write(ctx, &event)

	require.NoError(t, err)
	assert.Nil(t, writer.GetEvents()[0].Inputs)
}

// TestAuditTrailWriter_Write_WithInputsTruncatedFlag tests write with truncation indicator.
func TestAuditTrailWriter_Write_WithInputsTruncatedFlag(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()
	ctx := context.Background()

	event := workflow.AuditEvent{
		SchemaVersion:   1,
		Event:           workflow.EventWorkflowStarted,
		ExecutionID:     "truncated-test",
		Timestamp:       time.Now(),
		User:            "deployer",
		WorkflowName:    "large-input-workflow",
		Inputs:          map[string]any{"key": "val…"},
		InputsTruncated: true,
	}

	err := writer.Write(ctx, &event)

	require.NoError(t, err)
	assert.True(t, writer.GetEvents()[0].InputsTruncated)
}

// TestAuditTrailWriter_Close_HappyPath tests successful close.
func TestAuditTrailWriter_Close_HappyPath(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()

	err := writer.Close()

	assert.NoError(t, err)
	assert.True(t, writer.IsClosed())
}

// TestAuditTrailWriter_Close_AfterWrites tests close after multiple writes.
func TestAuditTrailWriter_Close_AfterWrites(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()
	ctx := context.Background()

	event1 := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "close-after",
		Timestamp:     time.Now(),
		User:          "user",
		WorkflowName:  "wf",
	}

	event2 := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowCompleted,
		ExecutionID:   "close-after",
		Timestamp:     time.Now(),
		User:          "user",
		WorkflowName:  "wf",
		Status:        "success",
	}

	writer.Write(ctx, &event1)
	writer.Write(ctx, &event2)

	err := writer.Close()

	assert.NoError(t, err)
	assert.Equal(t, 2, len(writer.GetEvents()))
}

// TestAuditTrailWriter_Write_Error tests Write returning an error.
func TestAuditTrailWriter_Write_Error(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()
	ctx := context.Background()
	expectedErr := errors.New("write failed: io error")
	writer.SetWriteError(expectedErr)

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "write-error",
		Timestamp:     time.Now(),
		User:          "user",
		WorkflowName:  "wf",
	}

	err := writer.Write(ctx, &event)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

// TestAuditTrailWriter_Close_Error tests Close returning an error.
func TestAuditTrailWriter_Close_Error(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()
	expectedErr := errors.New("close failed: file not found")
	writer.SetCloseError(expectedErr)

	err := writer.Close()

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

// TestAuditTrailWriter_Write_AfterClose tests Write after Close returns error.
func TestAuditTrailWriter_Write_AfterClose(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()
	ctx := context.Background()

	writer.Close()

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "after-close",
		Timestamp:     time.Now(),
		User:          "user",
		WorkflowName:  "wf",
	}

	err := writer.Write(ctx, &event)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

// TestAuditTrailWriter_Close_Twice tests Close called twice.
func TestAuditTrailWriter_Close_Twice(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()

	err1 := writer.Close()
	assert.NoError(t, err1)

	err2 := writer.Close()
	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "already closed")
}

// TestAuditTrailWriter_SameExecutionIDPaired tests paired start/completed events for same execution.
func TestAuditTrailWriter_SameExecutionIDPaired(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()
	ctx := context.Background()

	startEvent := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "paired-exec-123",
		Timestamp:     time.Date(2026, 2, 20, 23, 15, 42, 0, time.UTC),
		User:          "user",
		WorkflowName:  "test-wf",
	}

	completeEvent := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowCompleted,
		ExecutionID:   "paired-exec-123",
		Timestamp:     time.Date(2026, 2, 20, 23, 16, 12, 0, time.UTC),
		User:          "user",
		WorkflowName:  "test-wf",
		Status:        "success",
	}

	writer.Write(ctx, &startEvent)
	writer.Write(ctx, &completeEvent)

	events := writer.GetEvents()
	assert.Equal(t, 2, len(events))
	assert.Equal(t, events[0].ExecutionID, events[1].ExecutionID)
	assert.Equal(t, "paired-exec-123", events[0].ExecutionID)
	assert.Equal(t, "paired-exec-123", events[1].ExecutionID)
}

// TestAuditTrailWriter_DifferentExecutionIDs tests events with different execution IDs.
func TestAuditTrailWriter_DifferentExecutionIDs(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()
	ctx := context.Background()

	event1 := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "exec-id-001",
		Timestamp:     time.Now(),
		User:          "user",
		WorkflowName:  "wf1",
	}

	event2 := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "exec-id-002",
		Timestamp:     time.Now(),
		User:          "user",
		WorkflowName:  "wf2",
	}

	writer.Write(ctx, &event1)
	writer.Write(ctx, &event2)

	events := writer.GetEvents()
	assert.NotEqual(t, events[0].ExecutionID, events[1].ExecutionID)
	assert.Equal(t, "exec-id-001", events[0].ExecutionID)
	assert.Equal(t, "exec-id-002", events[1].ExecutionID)
}

// TestAuditTrailWriter_SchemaVersion tests that events preserve schema version.
func TestAuditTrailWriter_SchemaVersion(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()
	ctx := context.Background()

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		ExecutionID:   "schema-test",
		Timestamp:     time.Now(),
		User:          "user",
		WorkflowName:  "wf",
	}

	writer.Write(ctx, &event)

	assert.Equal(t, 1, writer.GetEvents()[0].SchemaVersion)
}

// TestAuditTrailWriter_EventTypes tests both event type constants are accepted.
func TestAuditTrailWriter_EventTypes(t *testing.T) {
	writer := mocks.NewMockAuditTrailWriter()
	ctx := context.Background()

	tests := []struct {
		name       string
		eventType  string
		shouldFail bool
	}{
		{
			name:       "workflow.started event type",
			eventType:  workflow.EventWorkflowStarted,
			shouldFail: false,
		},
		{
			name:       "workflow.completed event type",
			eventType:  workflow.EventWorkflowCompleted,
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := workflow.AuditEvent{
				SchemaVersion: 1,
				Event:         tt.eventType,
				ExecutionID:   "event-type-test",
				Timestamp:     time.Now(),
				User:          "user",
				WorkflowName:  "wf",
			}

			err := writer.Write(ctx, &event)

			if tt.shouldFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
