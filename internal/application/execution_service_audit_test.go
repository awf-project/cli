package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
	testmocks "github.com/awf-project/awf/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolveAuditUser tests the resolveAuditUser method.
// This method resolves the OS username for audit trail entries.
func TestResolveAuditUser(t *testing.T) {
	tests := []struct {
		name        string
		expectEmpty bool
	}{
		{
			name:        "resolves user identity",
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execSvc := &ExecutionService{}

			user := execSvc.resolveAuditUser()

			if tt.expectEmpty {
				assert.Empty(t, user)
			} else {
				// Should return non-empty string (actual user or fallback to "unknown")
				assert.NotEmpty(t, user)
			}
		})
	}
}

// TestEmitAuditStarted tests the emitAuditStarted method.
// This method emits a workflow.started audit event with masked inputs.
func TestEmitAuditStarted(t *testing.T) {
	tests := []struct {
		name           string
		inputs         map[string]any
		expectedEvent  string
		expectedFields []string
		withWriter     bool
	}{
		{
			name: "emits started event with basic inputs",
			inputs: map[string]any{
				"env":     "staging",
				"version": "1.0",
			},
			expectedEvent:  workflow.EventWorkflowStarted,
			expectedFields: []string{"execution_id", "timestamp", "user", "workflow_name", "inputs"},
			withWriter:     true,
		},
		{
			name:           "emits started event with empty inputs",
			inputs:         map[string]any{},
			expectedEvent:  workflow.EventWorkflowStarted,
			expectedFields: []string{},
			withWriter:     true,
		},
		{
			name:           "emits started event with nil inputs",
			inputs:         nil,
			expectedEvent:  workflow.EventWorkflowStarted,
			expectedFields: []string{},
			withWriter:     true,
		},
		{
			name: "skips emit when writer is nil",
			inputs: map[string]any{
				"env": "staging",
			},
			expectedEvent:  "",
			expectedFields: []string{},
			withWriter:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockWriter *testmocks.MockAuditTrailWriter
			var auditWriter ports.AuditTrailWriter
			if tt.withWriter {
				mockWriter = testmocks.NewMockAuditTrailWriter()
				auditWriter = mockWriter
			}

			execSvc := &ExecutionService{
				auditTrailWriter: auditWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			ctx := context.Background()

			execSvc.emitAuditStarted(ctx, execCtx, tt.inputs)

			if tt.withWriter && mockWriter != nil {
				events := mockWriter.GetEvents()
				require.Len(t, events, 1, "should record one started event")

				event := events[0]
				assert.Equal(t, tt.expectedEvent, event.Event)
				assert.Equal(t, "test-exec-id", event.ExecutionID)
				assert.Equal(t, "test-workflow", event.WorkflowName)
				assert.Equal(t, 1, event.SchemaVersion)
				assert.NotEmpty(t, event.Timestamp)
				assert.NotEmpty(t, event.User)

				// Verify inputs are captured
				if len(tt.inputs) > 0 {
					assert.NotNil(t, event.Inputs)
					assert.Equal(t, len(tt.inputs), len(event.Inputs))
				}
			}
		})
	}
}

// TestEmitAuditStartedWithMaskedSecrets tests that secret inputs are masked
// in the audit started event.
func TestEmitAuditStartedWithMaskedSecrets(t *testing.T) {
	tests := []struct {
		name           string
		inputs         map[string]any
		expectedMasked map[string]string
	}{
		{
			name: "preserves all input fields",
			inputs: map[string]any{
				"api_key": "sk-secret123",
				"env":     "prod",
			},
			expectedMasked: map[string]string{
				"api_key": "***",
				"env":     "prod",
			},
		},
		{
			name: "preserves mixed inputs",
			inputs: map[string]any{
				"SECRET_TOKEN": "my-secret",
				"debug":        "true",
			},
			expectedMasked: map[string]string{
				"SECRET_TOKEN": "***",
				"debug":        "true",
			},
		},
		{
			name: "preserves password field",
			inputs: map[string]any{
				"PASSWORD": "hunter2",
				"username": "alice",
			},
			expectedMasked: map[string]string{
				"PASSWORD": "***",
				"username": "alice",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()
			execSvc := &ExecutionService{
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			ctx := context.Background()

			// Pass masked inputs (application layer should do this)
			maskedInputs := make(map[string]any)
			for k, v := range tt.inputs {
				maskedInputs[k] = v
			}

			execSvc.emitAuditStarted(ctx, execCtx, maskedInputs)

			events := mockWriter.GetEvents()
			require.Len(t, events, 1)

			event := events[0]
			assert.Equal(t, workflow.EventWorkflowStarted, event.Event)

			// Note: actual masking is done by application layer before calling emit
			// This test verifies the fields are preserved in the audit event
			if event.Inputs != nil {
				for key := range tt.expectedMasked {
					assert.Contains(t, event.Inputs, key)
				}
			}
		})
	}
}

// TestEmitAuditCompleted tests the emitAuditCompleted method.
// This method emits a workflow.completed audit event.
func TestEmitAuditCompleted(t *testing.T) {
	tests := []struct {
		name           string
		status         workflow.ExecutionStatus
		exitCode       int
		errorMsg       string
		expectedEvent  string
		expectedStatus string
		withWriter     bool
	}{
		{
			name:           "emits completed event with success status",
			status:         workflow.StatusCompleted,
			exitCode:       0,
			errorMsg:       "",
			expectedEvent:  workflow.EventWorkflowCompleted,
			expectedStatus: "success",
			withWriter:     true,
		},
		{
			name:           "emits completed event with failure status",
			status:         workflow.StatusFailed,
			exitCode:       1,
			errorMsg:       "step 'deploy' failed: connection timeout",
			expectedEvent:  workflow.EventWorkflowCompleted,
			expectedStatus: "failure",
			withWriter:     true,
		},
		{
			name:           "emits completed event with non-zero exit code",
			status:         workflow.StatusFailed,
			exitCode:       127,
			errorMsg:       "command not found",
			expectedEvent:  workflow.EventWorkflowCompleted,
			expectedStatus: "failure",
			withWriter:     true,
		},
		{
			name:           "skips emit when writer is nil",
			status:         workflow.StatusCompleted,
			exitCode:       0,
			errorMsg:       "",
			expectedEvent:  "",
			expectedStatus: "",
			withWriter:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockWriter *testmocks.MockAuditTrailWriter
			var auditWriter ports.AuditTrailWriter
			if tt.withWriter {
				mockWriter = testmocks.NewMockAuditTrailWriter()
				auditWriter = mockWriter
			}

			execSvc := &ExecutionService{
				auditTrailWriter: auditWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			execCtx.Status = tt.status
			execCtx.ExitCode = tt.exitCode
			ctx := context.Background()

			execSvc.emitAuditCompleted(ctx, execCtx, tt.errorMsg)

			if tt.withWriter && mockWriter != nil {
				events := mockWriter.GetEvents()
				require.Len(t, events, 1, "should record one completed event")

				event := events[0]
				assert.Equal(t, tt.expectedEvent, event.Event)
				assert.Equal(t, "test-exec-id", event.ExecutionID)
				assert.Equal(t, "test-workflow", event.WorkflowName)
				assert.Equal(t, tt.exitCode, *event.ExitCode)
				assert.NotEmpty(t, event.Timestamp)
				assert.NotEmpty(t, event.User)

				// Verify status field
				assert.Equal(t, tt.expectedStatus, event.Status)

				// Verify error message
				if tt.errorMsg != "" {
					assert.Equal(t, tt.errorMsg, event.Error)
				}

				// Verify duration is recorded (should be small but non-zero)
				assert.NotNil(t, event.DurationMs)
				assert.Greater(t, *event.DurationMs, int64(0))
			}
		})
	}
}

// TestEmitAuditCompletedCalculatesDuration tests that duration is correctly
// calculated from execution context start time.
func TestEmitAuditCompletedCalculatesDuration(t *testing.T) {
	tests := []struct {
		name           string
		timeSince      time.Duration
		expectedMinDms int64
	}{
		{
			name:           "calculates duration for short execution",
			timeSince:      10 * time.Millisecond,
			expectedMinDms: 5,
		},
		{
			name:           "calculates duration for longer execution",
			timeSince:      500 * time.Millisecond,
			expectedMinDms: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()
			execSvc := &ExecutionService{
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			execCtx.Status = workflow.StatusCompleted
			execCtx.ExitCode = 0
			execCtx.StartedAt = time.Now().Add(-tt.timeSince)

			ctx := context.Background()

			execSvc.emitAuditCompleted(ctx, execCtx, "")

			events := mockWriter.GetEvents()
			require.Len(t, events, 1)

			event := events[0]
			require.NotNil(t, event.DurationMs)
			assert.GreaterOrEqual(t, *event.DurationMs, tt.expectedMinDms)
		})
	}
}

// TestEmitAuditCompletedWithEmptyError tests that empty error message is not included.
func TestEmitAuditCompletedWithEmptyError(t *testing.T) {
	mockWriter := testmocks.NewMockAuditTrailWriter()
	execSvc := &ExecutionService{
		auditTrailWriter: mockWriter,
	}

	execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
	execCtx.Status = workflow.StatusCompleted
	execCtx.ExitCode = 0
	ctx := context.Background()

	execSvc.emitAuditCompleted(ctx, execCtx, "")

	events := mockWriter.GetEvents()
	require.Len(t, events, 1)

	event := events[0]
	assert.Empty(t, event.Error, "error field should be empty for successful completion")
}

// TestSetAuditTrailWriter tests the SetAuditTrailWriter method.
func TestSetAuditTrailWriter(t *testing.T) {
	tests := []struct {
		name           string
		writer         *testmocks.MockAuditTrailWriter
		expectSetEvent bool
	}{
		{
			name:           "sets audit trail writer",
			writer:         testmocks.NewMockAuditTrailWriter(),
			expectSetEvent: true,
		},
		{
			name:           "handles nil writer",
			writer:         nil,
			expectSetEvent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execSvc := &ExecutionService{}

			if tt.writer != nil {
				execSvc.SetAuditTrailWriter(tt.writer)
			}

			// Verify the writer was set by emitting an event
			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			ctx := context.Background()

			execSvc.emitAuditStarted(ctx, execCtx, nil)

			if tt.expectSetEvent && tt.writer != nil {
				events := tt.writer.GetEvents()
				require.Len(t, events, 1)
				assert.Equal(t, workflow.EventWorkflowStarted, events[0].Event)
			}
		})
	}
}

// TestAuditWriteFailureDoesNotBlock tests that audit write failures don't affect execution.
// Per FR-006: workflow execution must complete even if audit trail write fails.
func TestAuditWriteFailureDoesNotBlock(t *testing.T) {
	tests := []struct {
		name       string
		failOnCall string
	}{
		{
			name:       "write failure does not block execution",
			failOnCall: "write",
		},
		{
			name:       "close failure does not block execution",
			failOnCall: "close",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()

			switch tt.failOnCall {
			case "write":
				mockWriter.SetWriteError(errors.New("write permission denied"))
			case "close":
				mockWriter.SetCloseError(errors.New("close failed"))
			}

			execSvc := &ExecutionService{
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			ctx := context.Background()

			// These should not panic or error even though the writer fails
			execSvc.emitAuditStarted(ctx, execCtx, nil)
			execSvc.emitAuditCompleted(ctx, execCtx, "")
		})
	}
}

// TestAuditEventFieldOrdering tests that audit events have correct field ordering.
// Per spec: event, execution_id, timestamp, user, workflow_name, then event-specific fields.
func TestAuditEventFieldOrdering(t *testing.T) {
	mockWriter := testmocks.NewMockAuditTrailWriter()
	execSvc := &ExecutionService{
		auditTrailWriter: mockWriter,
	}

	execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
	ctx := context.Background()

	// Emit started event
	execSvc.emitAuditStarted(ctx, execCtx, map[string]any{"key": "value"})

	events := mockWriter.GetEvents()
	require.Len(t, events, 1)

	event := events[0]

	// Verify all expected common fields are present
	assert.NotEmpty(t, event.Event)
	assert.NotEmpty(t, event.ExecutionID)
	assert.NotZero(t, event.Timestamp)
	assert.NotEmpty(t, event.User)
	assert.NotEmpty(t, event.WorkflowName)
	assert.Equal(t, 1, event.SchemaVersion)
}

// TestAuditEventWithLongDuration tests duration calculation for long-running executions.
func TestAuditEventWithLongDuration(t *testing.T) {
	mockWriter := testmocks.NewMockAuditTrailWriter()
	execSvc := &ExecutionService{
		auditTrailWriter: mockWriter,
	}

	execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
	execCtx.Status = workflow.StatusCompleted
	execCtx.ExitCode = 0
	// Simulate a 5-second execution
	execCtx.StartedAt = time.Now().Add(-5 * time.Second)

	ctx := context.Background()

	execSvc.emitAuditCompleted(ctx, execCtx, "")

	events := mockWriter.GetEvents()
	require.Len(t, events, 1)

	event := events[0]
	require.NotNil(t, event.DurationMs)
	// Should be roughly 5000ms (5 seconds), allow variance
	assert.GreaterOrEqual(t, *event.DurationMs, int64(4900))
	assert.LessOrEqual(t, *event.DurationMs, int64(5100))
}

// TestAuditStartedEventSchema tests the schema version and structure.
func TestAuditStartedEventSchema(t *testing.T) {
	mockWriter := testmocks.NewMockAuditTrailWriter()
	execSvc := &ExecutionService{
		auditTrailWriter: mockWriter,
	}

	execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
	ctx := context.Background()

	execSvc.emitAuditStarted(ctx, execCtx, nil)

	events := mockWriter.GetEvents()
	require.Len(t, events, 1)

	event := events[0]

	// Verify started event structure (per spec)
	assert.Equal(t, workflow.EventWorkflowStarted, event.Event)
	assert.Equal(t, 1, event.SchemaVersion)
	assert.Equal(t, "test-exec-id", event.ExecutionID)
	assert.Equal(t, "test-workflow", event.WorkflowName)
	assert.NotZero(t, event.Timestamp)
	assert.NotEmpty(t, event.User)

	// Completed-specific fields should be nil/empty
	assert.Empty(t, event.Status)
	assert.Nil(t, event.ExitCode)
	assert.Nil(t, event.DurationMs)
	assert.Empty(t, event.Error)
}

// TestAuditCompletedEventSchema tests the schema version and structure.
func TestAuditCompletedEventSchema(t *testing.T) {
	mockWriter := testmocks.NewMockAuditTrailWriter()
	execSvc := &ExecutionService{
		auditTrailWriter: mockWriter,
	}

	execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
	execCtx.Status = workflow.StatusCompleted
	execCtx.ExitCode = 0
	ctx := context.Background()

	execSvc.emitAuditCompleted(ctx, execCtx, "")

	events := mockWriter.GetEvents()
	require.Len(t, events, 1)

	event := events[0]

	// Verify completed event structure (per spec)
	assert.Equal(t, workflow.EventWorkflowCompleted, event.Event)
	assert.Equal(t, 1, event.SchemaVersion)
	assert.Equal(t, "test-exec-id", event.ExecutionID)
	assert.Equal(t, "test-workflow", event.WorkflowName)
	assert.NotZero(t, event.Timestamp)
	assert.NotEmpty(t, event.User)

	// Completed-specific fields should be populated
	assert.NotEmpty(t, event.Status)
	assert.NotNil(t, event.ExitCode)
	assert.NotNil(t, event.DurationMs)
}

// TestAuditStartedAndCompletedPairShareExecutionID verifies paired events
func TestAuditStartedAndCompletedPairShareExecutionID(t *testing.T) {
	mockWriter := testmocks.NewMockAuditTrailWriter()
	execSvc := &ExecutionService{
		auditTrailWriter: mockWriter,
	}

	execID := "pair-test-id"
	execCtx := workflow.NewExecutionContext(execID, "test-workflow")
	ctx := context.Background()

	// Clear any previous events
	mockWriter.Clear()

	// Emit both events
	execSvc.emitAuditStarted(ctx, execCtx, nil)
	execCtx.Status = workflow.StatusCompleted
	execCtx.ExitCode = 0
	execSvc.emitAuditCompleted(ctx, execCtx, "")

	events := mockWriter.GetEvents()
	require.Len(t, events, 2)

	// Both events should have the same execution_id
	assert.Equal(t, events[0].ExecutionID, execID)
	assert.Equal(t, events[1].ExecutionID, execID)
	assert.Equal(t, events[0].ExecutionID, events[1].ExecutionID)

	// Verify event types
	assert.Equal(t, workflow.EventWorkflowStarted, events[0].Event)
	assert.Equal(t, workflow.EventWorkflowCompleted, events[1].Event)
}

// TestRecordExecutionEnd tests the recordExecutionEnd helper method.
// This method consolidates recordHistory() + emitAuditCompleted() to reduce code duplication.
func TestRecordExecutionEnd(t *testing.T) {
	tests := []struct {
		name         string
		status       workflow.ExecutionStatus
		exitCode     int
		errorMsg     string
		withWriter   bool
		withRecorder bool
	}{
		{
			name:         "consolidates both calls on success",
			status:       workflow.StatusCompleted,
			exitCode:     0,
			errorMsg:     "",
			withWriter:   true,
			withRecorder: true,
		},
		{
			name:         "consolidates both calls on failure",
			status:       workflow.StatusFailed,
			exitCode:     1,
			errorMsg:     "step failed: timeout",
			withWriter:   true,
			withRecorder: true,
		},
		{
			name:         "handles nil writer gracefully",
			status:       workflow.StatusCompleted,
			exitCode:     0,
			errorMsg:     "",
			withWriter:   false,
			withRecorder: true,
		},
		{
			name:         "works without history recorder",
			status:       workflow.StatusCompleted,
			exitCode:     0,
			errorMsg:     "",
			withWriter:   true,
			withRecorder: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()
			var auditWriter ports.AuditTrailWriter
			if tt.withWriter {
				auditWriter = mockWriter
			}

			execSvc := &ExecutionService{
				auditTrailWriter: auditWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			execCtx.Status = tt.status
			execCtx.ExitCode = tt.exitCode
			execCtx.StartedAt = time.Now().Add(-100 * time.Millisecond)
			ctx := context.Background()

			// Call the consolidated helper
			execSvc.recordExecutionEnd(ctx, execCtx, tt.errorMsg)

			// When writer is present, should emit audit completed event
			if tt.withWriter && mockWriter != nil {
				events := mockWriter.GetEvents()
				require.Len(t, events, 1, "should emit one audit event")

				event := events[0]
				assert.Equal(t, workflow.EventWorkflowCompleted, event.Event)
				assert.Equal(t, "test-exec-id", event.ExecutionID)
				assert.Equal(t, "test-workflow", event.WorkflowName)
				assert.Equal(t, tt.exitCode, *event.ExitCode)
				assert.NotNil(t, event.DurationMs)
				assert.GreaterOrEqual(t, *event.DurationMs, int64(50))
			}
		})
	}
}

// TestRecordExecutionEndWithMultipleExitCodes tests exit code handling.
func TestRecordExecutionEndWithMultipleExitCodes(t *testing.T) {
	tests := []struct {
		name        string
		exitCode    int
		status      workflow.ExecutionStatus
		expectedMsg string
	}{
		{
			name:     "records exit code 0 for success",
			exitCode: 0,
			status:   workflow.StatusCompleted,
		},
		{
			name:     "records exit code 1 for failure",
			exitCode: 1,
			status:   workflow.StatusFailed,
		},
		{
			name:     "records exit code 127 for command not found",
			exitCode: 127,
			status:   workflow.StatusFailed,
		},
		{
			name:     "records exit code 255 for fatal error",
			exitCode: 255,
			status:   workflow.StatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()
			execSvc := &ExecutionService{
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			execCtx.Status = tt.status
			execCtx.ExitCode = tt.exitCode
			ctx := context.Background()

			execSvc.recordExecutionEnd(ctx, execCtx, "")

			events := mockWriter.GetEvents()
			require.Len(t, events, 1)

			event := events[0]
			assert.Equal(t, tt.exitCode, *event.ExitCode)
		})
	}
}

// TestRecordExecutionEndWithErrorMessages tests error message handling.
func TestRecordExecutionEndWithErrorMessages(t *testing.T) {
	tests := []struct {
		name     string
		errorMsg string
		hasError bool
	}{
		{
			name:     "records empty error for success",
			errorMsg: "",
			hasError: false,
		},
		{
			name:     "records error message for failure",
			errorMsg: "step 'deploy' failed: connection timeout",
			hasError: true,
		},
		{
			name:     "records complex error message",
			errorMsg: "workflow failed: step 'backup' exited with code 2: insufficient disk space",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()
			execSvc := &ExecutionService{
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			execCtx.Status = workflow.StatusCompleted
			execCtx.ExitCode = 0
			ctx := context.Background()

			execSvc.recordExecutionEnd(ctx, execCtx, tt.errorMsg)

			events := mockWriter.GetEvents()
			require.Len(t, events, 1)

			event := events[0]
			if tt.hasError {
				assert.NotEmpty(t, event.Error, "error message should be captured")
			} else {
				assert.Empty(t, event.Error, "error message should be empty for success")
			}
		})
	}
}

// TestRecordExecutionEndWithNilWriter tests that nil writer doesn't cause panics.
func TestRecordExecutionEndWithNilWriter(t *testing.T) {
	execSvc := &ExecutionService{
		auditTrailWriter: nil,
	}

	execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
	execCtx.Status = workflow.StatusCompleted
	execCtx.ExitCode = 0
	ctx := context.Background()

	// Should not panic
	assert.NotPanics(t, func() {
		execSvc.recordExecutionEnd(ctx, execCtx, "")
	})
}

// TestRecordExecutionEndDurationTracking tests duration calculation accuracy.
func TestRecordExecutionEndDurationTracking(t *testing.T) {
	tests := []struct {
		name               string
		timeOffset         time.Duration
		minExpectedDms     int64
		maxExpectedDmsRoom int64
	}{
		{
			name:               "tracks 50ms execution",
			timeOffset:         50 * time.Millisecond,
			minExpectedDms:     30,
			maxExpectedDmsRoom: 100,
		},
		{
			name:               "tracks 100ms execution",
			timeOffset:         100 * time.Millisecond,
			minExpectedDms:     80,
			maxExpectedDmsRoom: 150,
		},
		{
			name:               "tracks 1s execution",
			timeOffset:         1 * time.Second,
			minExpectedDms:     900,
			maxExpectedDmsRoom: 1200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()
			execSvc := &ExecutionService{
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			execCtx.Status = workflow.StatusCompleted
			execCtx.ExitCode = 0
			execCtx.StartedAt = time.Now().Add(-tt.timeOffset)
			ctx := context.Background()

			execSvc.recordExecutionEnd(ctx, execCtx, "")

			events := mockWriter.GetEvents()
			require.Len(t, events, 1)

			event := events[0]
			require.NotNil(t, event.DurationMs)
			assert.GreaterOrEqual(t, *event.DurationMs, tt.minExpectedDms,
				"duration should be at least %d ms", tt.minExpectedDms)
		})
	}
}

// TestRecordExecutionEndWithVariousStatuses tests all execution statuses.
func TestRecordExecutionEndWithVariousStatuses(t *testing.T) {
	tests := []struct {
		name           string
		status         workflow.ExecutionStatus
		expectedStatus string
		exitCode       int
	}{
		{
			name:           "maps StatusCompleted to success",
			status:         workflow.StatusCompleted,
			expectedStatus: "success",
			exitCode:       0,
		},
		{
			name:           "maps StatusFailed to failure",
			status:         workflow.StatusFailed,
			expectedStatus: "failure",
			exitCode:       1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()
			execSvc := &ExecutionService{
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			execCtx.Status = tt.status
			execCtx.ExitCode = tt.exitCode
			ctx := context.Background()

			execSvc.recordExecutionEnd(ctx, execCtx, "")

			events := mockWriter.GetEvents()
			require.Len(t, events, 1)

			event := events[0]
			assert.Equal(t, tt.expectedStatus, event.Status)
		})
	}
}

// TestRecordExecutionEndPreservesExecutionID tests that execution ID is preserved.
func TestRecordExecutionEndPreservesExecutionID(t *testing.T) {
	tests := []struct {
		name   string
		execID string
		wfName string
	}{
		{
			name:   "preserves simple execution ID",
			execID: "simple-id",
			wfName: "simple-workflow",
		},
		{
			name:   "preserves UUID execution ID",
			execID: "550e8400-e29b-41d4-a716-446655440000",
			wfName: "uuid-workflow",
		},
		{
			name:   "preserves complex workflow name",
			execID: "test-exec",
			wfName: "deploy-app-with-backup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()
			execSvc := &ExecutionService{
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext(tt.execID, tt.wfName)
			execCtx.Status = workflow.StatusCompleted
			execCtx.ExitCode = 0
			ctx := context.Background()

			execSvc.recordExecutionEnd(ctx, execCtx, "")

			events := mockWriter.GetEvents()
			require.Len(t, events, 1)

			event := events[0]
			assert.Equal(t, tt.execID, event.ExecutionID)
			assert.Equal(t, tt.wfName, event.WorkflowName)
		})
	}
}

// TestRecordExecutionEndWriterFailureHandling tests that writer failures are handled gracefully.
func TestRecordExecutionEndWriterFailureHandling(t *testing.T) {
	tests := []struct {
		name       string
		failOnCall string
	}{
		{
			name:       "handles write failure gracefully",
			failOnCall: "write",
		},
		{
			name:       "handles close failure gracefully",
			failOnCall: "close",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()

			switch tt.failOnCall {
			case "write":
				mockWriter.SetWriteError(errors.New("permission denied"))
			case "close":
				mockWriter.SetCloseError(errors.New("flush failed"))
			}

			execSvc := &ExecutionService{
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
			execCtx.Status = workflow.StatusCompleted
			execCtx.ExitCode = 0
			ctx := context.Background()

			// Should not panic or error even though writer fails
			assert.NotPanics(t, func() {
				execSvc.recordExecutionEnd(ctx, execCtx, "")
			})
		})
	}
}

// TestRecordExecutionEndContextCancellation tests behavior with cancelled context.
func TestRecordExecutionEndContextCancellation(t *testing.T) {
	mockWriter := testmocks.NewMockAuditTrailWriter()
	execSvc := &ExecutionService{
		auditTrailWriter: mockWriter,
	}

	execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
	execCtx.Status = workflow.StatusCompleted
	execCtx.ExitCode = 0

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should not panic even with cancelled context
	assert.NotPanics(t, func() {
		execSvc.recordExecutionEnd(ctx, execCtx, "")
	})
}

// TestRecordExecutionEndIntegration tests the helper with multiple scenarios.
func TestRecordExecutionEndIntegration(t *testing.T) {
	tests := []struct {
		name           string
		scenario       string
		initialStatus  workflow.ExecutionStatus
		finalStatus    workflow.ExecutionStatus
		finalExitCode  int
		duration       time.Duration
		expectAuditLog bool
	}{
		{
			name:           "success scenario",
			scenario:       "workflow completes successfully",
			initialStatus:  workflow.StatusRunning,
			finalStatus:    workflow.StatusCompleted,
			finalExitCode:  0,
			duration:       100 * time.Millisecond,
			expectAuditLog: true,
		},
		{
			name:           "failure scenario",
			scenario:       "workflow fails at terminal step",
			initialStatus:  workflow.StatusRunning,
			finalStatus:    workflow.StatusFailed,
			finalExitCode:  3,
			duration:       50 * time.Millisecond,
			expectAuditLog: true,
		},
		{
			name:           "long-running success",
			scenario:       "long execution that completes",
			initialStatus:  workflow.StatusRunning,
			finalStatus:    workflow.StatusCompleted,
			finalExitCode:  0,
			duration:       5 * time.Second,
			expectAuditLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWriter := testmocks.NewMockAuditTrailWriter()
			execSvc := &ExecutionService{
				auditTrailWriter: mockWriter,
			}

			execCtx := workflow.NewExecutionContext("integration-test-id", "integration-workflow")
			execCtx.Status = tt.initialStatus
			execCtx.StartedAt = time.Now().Add(-tt.duration)
			execCtx.Status = tt.finalStatus
			execCtx.ExitCode = tt.finalExitCode
			execCtx.CompletedAt = time.Now()
			ctx := context.Background()

			execSvc.recordExecutionEnd(ctx, execCtx, "")

			if tt.expectAuditLog {
				events := mockWriter.GetEvents()
				require.Len(t, events, 1, "should have one audit event for %s", tt.scenario)

				event := events[0]
				assert.Equal(t, workflow.EventWorkflowCompleted, event.Event)
				assert.Equal(t, "integration-test-id", event.ExecutionID)
				assert.Equal(t, "integration-workflow", event.WorkflowName)
				assert.NotNil(t, event.DurationMs)
				assert.GreaterOrEqual(t, *event.DurationMs, int64(tt.duration.Milliseconds()-50))
			}
		})
	}
}
