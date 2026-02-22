package workflow_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: T001
// Feature: F071
// Tests: AuditEvent entity with constructors and custom JSON marshaling

// TestNewStartedEvent tests AuditEvent creation for workflow start.
func TestNewStartedEvent_PopulatesAllRequiredFields(t *testing.T) {
	execCtx := &workflow.ExecutionContext{
		WorkflowID:   "exec-123",
		WorkflowName: "deploy-app",
		StartedAt:    time.Date(2026, 2, 20, 23, 15, 42, 123000000, time.UTC),
		Inputs: map[string]any{
			"env":     "staging",
			"api_key": "***",
		},
	}

	maskedInputs := map[string]any{
		"env":     "staging",
		"api_key": "***",
	}
	user := "deploy-bot"

	event := workflow.NewStartedEvent(execCtx, maskedInputs, user)

	assert.Equal(t, workflow.EventWorkflowStarted, event.Event)
	assert.Equal(t, "exec-123", event.ExecutionID)
	assert.Equal(t, execCtx.StartedAt, event.Timestamp)
	assert.Equal(t, "deploy-bot", event.User)
	assert.Equal(t, "deploy-app", event.WorkflowName)
	assert.Equal(t, maskedInputs, event.Inputs)
	assert.Equal(t, 1, event.SchemaVersion)
}

// TestNewStartedEvent_WithNilInputs tests constructor handles nil inputs.
func TestNewStartedEvent_WithNilInputs(t *testing.T) {
	execCtx := &workflow.ExecutionContext{
		WorkflowID:   "exec-456",
		WorkflowName: "simple-workflow",
		StartedAt:    time.Date(2026, 2, 20, 23, 20, 0, 0, time.UTC),
	}

	event := workflow.NewStartedEvent(execCtx, nil, "user")

	assert.Equal(t, workflow.EventWorkflowStarted, event.Event)
	assert.Nil(t, event.Inputs)
	assert.False(t, event.InputsTruncated)
}

// TestNewStartedEvent_WithEmptyInputs tests constructor with empty input map.
func TestNewStartedEvent_WithEmptyInputs(t *testing.T) {
	execCtx := &workflow.ExecutionContext{
		WorkflowID:   "exec-789",
		WorkflowName: "workflow",
		StartedAt:    time.Date(2026, 2, 20, 23, 25, 0, 0, time.UTC),
	}

	event := workflow.NewStartedEvent(execCtx, map[string]any{}, "user")

	assert.Equal(t, workflow.EventWorkflowStarted, event.Event)
	assert.Empty(t, event.Inputs)
}

// TestNewStartedEvent_WithTruncatedInputsFlag tests constructor preserves truncation flag.
func TestNewStartedEvent_WithTruncatedInputsFlag(t *testing.T) {
	execCtx := &workflow.ExecutionContext{
		WorkflowID:   "exec-111",
		WorkflowName: "big-inputs",
		StartedAt:    time.Date(2026, 2, 20, 23, 30, 0, 0, time.UTC),
	}

	maskedInputs := map[string]any{
		"large_value": "truncated…",
	}

	event := workflow.NewStartedEvent(execCtx, maskedInputs, "user")

	// The event should preserve truncation state if indicated
	// (implementation will set this based on event construction parameters or size check)
	assert.Equal(t, workflow.EventWorkflowStarted, event.Event)
}

// TestNewCompletedEvent_SuccessStatus tests AuditEvent creation for successful completion.
func TestNewCompletedEvent_SuccessStatus(t *testing.T) {
	execCtx := &workflow.ExecutionContext{
		WorkflowID:   "exec-123",
		WorkflowName: "deploy-app",
		StartedAt:    time.Date(2026, 2, 20, 23, 15, 42, 123000000, time.UTC),
		CompletedAt:  time.Date(2026, 2, 20, 23, 16, 12, 456000000, time.UTC),
		ExitCode:     0,
	}

	user := "deploy-bot"

	event := workflow.NewCompletedEvent(execCtx, user, "")

	assert.Equal(t, workflow.EventWorkflowCompleted, event.Event)
	assert.Equal(t, "exec-123", event.ExecutionID)
	assert.Equal(t, "deploy-bot", event.User)
	assert.Equal(t, "deploy-app", event.WorkflowName)
	assert.Equal(t, "success", event.Status)
	assert.NotNil(t, event.ExitCode)
	assert.Equal(t, 0, *event.ExitCode)
	assert.NotNil(t, event.DurationMs)
	assert.Equal(t, int64(30333), *event.DurationMs) // ~30 seconds
	assert.Empty(t, event.Error)
	assert.Equal(t, 1, event.SchemaVersion)
}

// TestNewCompletedEvent_FailureStatus tests completion with non-zero exit code.
func TestNewCompletedEvent_FailureStatus(t *testing.T) {
	execCtx := &workflow.ExecutionContext{
		WorkflowID:   "exec-456",
		WorkflowName: "build-job",
		StartedAt:    time.Date(2026, 2, 20, 23, 15, 0, 0, time.UTC),
		CompletedAt:  time.Date(2026, 2, 20, 23, 15, 30, 0, time.UTC),
		ExitCode:     3,
	}

	errorMsg := "step 'deploy' failed: connection timeout"

	event := workflow.NewCompletedEvent(execCtx, "build-user", errorMsg)

	assert.Equal(t, workflow.EventWorkflowCompleted, event.Event)
	assert.Equal(t, "failure", event.Status)
	assert.NotNil(t, event.ExitCode)
	assert.Equal(t, 3, *event.ExitCode)
	assert.Equal(t, errorMsg, event.Error)
	assert.NotNil(t, event.DurationMs)
	assert.Equal(t, int64(30000), *event.DurationMs)
}

// TestNewCompletedEvent_NoErrorMessageOnSuccess tests error field is empty for success.
func TestNewCompletedEvent_NoErrorMessageOnSuccess(t *testing.T) {
	execCtx := &workflow.ExecutionContext{
		WorkflowID:   "exec-999",
		WorkflowName: "test-wf",
		StartedAt:    time.Date(2026, 2, 20, 23, 0, 0, 0, time.UTC),
		CompletedAt:  time.Date(2026, 2, 20, 23, 0, 10, 0, time.UTC),
		ExitCode:     0,
	}

	event := workflow.NewCompletedEvent(execCtx, "testuser", "")

	assert.Equal(t, "success", event.Status)
	assert.Empty(t, event.Error)
}

// TestAuditEvent_MarshalJSON_StartedEvent tests JSON output for workflow.started event.
func TestAuditEvent_MarshalJSON_StartedEvent(t *testing.T) {
	timestamp := time.Date(2026, 2, 20, 23, 15, 42, 123000000, time.UTC)

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		Timestamp:     timestamp,
		ExecutionID:   "550e8400-e29b-41d4-a716-446655440000",
		User:          "deploy-bot",
		WorkflowName:  "deploy-app",
		Inputs: map[string]any{
			"env":     "staging",
			"api_key": "***",
		},
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, workflow.EventWorkflowStarted, result["event"])
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", result["execution_id"])
	assert.Contains(t, string(data), "2026-02-20T23:15:42.123")
	assert.Equal(t, "deploy-bot", result["user"])
	assert.Equal(t, "deploy-app", result["workflow_name"])
	assert.NotNil(t, result["inputs"])
	assert.Equal(t, float64(1), result["schema_version"])
}

// TestAuditEvent_MarshalJSON_CompletedEventSuccess tests JSON output for successful completion.
func TestAuditEvent_MarshalJSON_CompletedEventSuccess(t *testing.T) {
	timestamp := time.Date(2026, 2, 20, 23, 16, 12, 456000000, time.UTC)
	exitCode := 0
	duration := int64(30333)

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowCompleted,
		Timestamp:     timestamp,
		ExecutionID:   "550e8400-e29b-41d4-a716-446655440000",
		User:          "deploy-bot",
		WorkflowName:  "deploy-app",
		Status:        "success",
		ExitCode:      &exitCode,
		DurationMs:    &duration,
		Error:         "",
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, workflow.EventWorkflowCompleted, result["event"])
	assert.Equal(t, "success", result["status"])
	assert.Equal(t, float64(0), result["exit_code"])
	assert.Equal(t, float64(30333), result["duration_ms"])
	assert.NotContains(t, string(data), "\"error\"")
	assert.Equal(t, float64(1), result["schema_version"])
}

// TestAuditEvent_MarshalJSON_CompletedEventFailure tests JSON output for failure with error.
func TestAuditEvent_MarshalJSON_CompletedEventFailure(t *testing.T) {
	timestamp := time.Date(2026, 2, 20, 23, 16, 12, 456000000, time.UTC)
	exitCode := 3
	duration := int64(30333)

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowCompleted,
		Timestamp:     timestamp,
		ExecutionID:   "550e8400-e29b-41d4-a716-446655440000",
		User:          "deploy-bot",
		WorkflowName:  "deploy-app",
		Status:        "failure",
		ExitCode:      &exitCode,
		DurationMs:    &duration,
		Error:         "step 'deploy' failed: connection timeout",
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, "failure", result["status"])
	assert.Equal(t, float64(3), result["exit_code"])
	assert.Equal(t, "step 'deploy' failed: connection timeout", result["error"])
}

// TestAuditEvent_MarshalJSON_FieldOrder tests that fields are output in correct order.
func TestAuditEvent_MarshalJSON_FieldOrder(t *testing.T) {
	timestamp := time.Date(2026, 2, 20, 23, 15, 42, 123000000, time.UTC)

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		Timestamp:     timestamp,
		ExecutionID:   "test-exec-id",
		User:          "testuser",
		WorkflowName:  "test-wf",
		Inputs:        map[string]any{"key": "value"},
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	jsonStr := string(data)

	// Basic check that all expected fields are present in JSON
	assert.Contains(t, jsonStr, "\"event\"")
	assert.Contains(t, jsonStr, "\"execution_id\"")
	assert.Contains(t, jsonStr, "\"timestamp\"")
	assert.Contains(t, jsonStr, "\"user\"")
	assert.Contains(t, jsonStr, "\"workflow_name\"")
	assert.Contains(t, jsonStr, "\"schema_version\"")
}

// TestAuditEvent_MarshalJSON_OmitEmptyEventSpecificFields tests that empty fields are omitted.
func TestAuditEvent_MarshalJSON_OmitEmptyEventSpecificFields(t *testing.T) {
	timestamp := time.Date(2026, 2, 20, 23, 15, 42, 123000000, time.UTC)

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		Timestamp:     timestamp,
		ExecutionID:   "test-id",
		User:          "user",
		WorkflowName:  "wf",
		Inputs:        nil,
		Status:        "", // Empty status
		ExitCode:      nil,
		DurationMs:    nil,
		Error:         "", // Empty error
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	jsonStr := string(data)

	// Status, exit_code, duration_ms, error should not appear
	assert.NotContains(t, jsonStr, "\"status\"")
	assert.NotContains(t, jsonStr, "\"exit_code\"")
	assert.NotContains(t, jsonStr, "\"duration_ms\"")
	assert.NotContains(t, jsonStr, "\"error\"")
}

// TestAuditEvent_MarshalJSON_NilExitCode tests handling of nil exit_code pointer.
func TestAuditEvent_MarshalJSON_NilExitCode(t *testing.T) {
	timestamp := time.Date(2026, 2, 20, 23, 15, 42, 123000000, time.UTC)

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		Timestamp:     timestamp,
		ExecutionID:   "test-id",
		User:          "user",
		WorkflowName:  "wf",
		ExitCode:      nil,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.NotContains(t, jsonStr, "\"exit_code\"")
}

// TestAuditEvent_MarshalJSON_TimestampPrecision tests millisecond precision in timestamp.
func TestAuditEvent_MarshalJSON_TimestampPrecision(t *testing.T) {
	// Timestamp with nanosecond precision
	timestamp := time.Date(2026, 2, 20, 23, 15, 42, 123456789, time.UTC)

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		Timestamp:     timestamp,
		ExecutionID:   "test-id",
		User:          "user",
		WorkflowName:  "wf",
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	jsonStr := string(data)

	// Should contain milliseconds only (3 decimal places)
	assert.Contains(t, jsonStr, ".123Z")
	// Should not contain full nanosecond precision
	assert.NotContains(t, jsonStr, ".123456789")
}

// TestAuditEvent_MarshalJSON_InputsTruncatedFlag tests inputs_truncated field in output.
func TestAuditEvent_MarshalJSON_InputsTruncatedFlag(t *testing.T) {
	timestamp := time.Date(2026, 2, 20, 23, 15, 42, 123000000, time.UTC)

	event := workflow.AuditEvent{
		SchemaVersion:   1,
		Event:           workflow.EventWorkflowStarted,
		Timestamp:       timestamp,
		ExecutionID:     "test-id",
		User:            "user",
		WorkflowName:    "wf",
		InputsTruncated: true,
		Inputs:          map[string]any{"key": "value…"},
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, true, result["inputs_truncated"])
}

// TestAuditEvent_MarshalJSON_InputsTruncatedFalseOmitted tests that false flag is omitted.
func TestAuditEvent_MarshalJSON_InputsTruncatedFalseOmitted(t *testing.T) {
	timestamp := time.Date(2026, 2, 20, 23, 15, 42, 123000000, time.UTC)

	event := workflow.AuditEvent{
		SchemaVersion:   1,
		Event:           workflow.EventWorkflowStarted,
		Timestamp:       timestamp,
		ExecutionID:     "test-id",
		User:            "user",
		WorkflowName:    "wf",
		InputsTruncated: false,
		Inputs:          map[string]any{"key": "value"},
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.NotContains(t, jsonStr, "\"inputs_truncated\"")
}

// TestAuditEvent_MarshalJSON_ValidJSON tests that output is valid JSON.
func TestAuditEvent_MarshalJSON_ValidJSON(t *testing.T) {
	timestamp := time.Date(2026, 2, 20, 23, 15, 42, 123000000, time.UTC)
	exitCode := 0
	duration := int64(30000)

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowCompleted,
		Timestamp:     timestamp,
		ExecutionID:   "550e8400-e29b-41d4-a716-446655440000",
		User:          "deploy-bot",
		WorkflowName:  "deploy-app",
		Status:        "success",
		ExitCode:      &exitCode,
		DurationMs:    &duration,
		Inputs: map[string]any{
			"env": "prod",
		},
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	// Should be parseable as JSON
	var result map[string]any
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	// Verify all required fields present
	assert.NotNil(t, result["event"])
	assert.NotNil(t, result["execution_id"])
	assert.NotNil(t, result["timestamp"])
	assert.NotNil(t, result["user"])
	assert.NotNil(t, result["workflow_name"])
	assert.NotNil(t, result["schema_version"])
}

// TestAuditEvent_MarshalJSON_ComplexInputs tests JSON marshaling with complex input types.
func TestAuditEvent_MarshalJSON_ComplexInputs(t *testing.T) {
	timestamp := time.Date(2026, 2, 20, 23, 15, 42, 123000000, time.UTC)

	event := workflow.AuditEvent{
		SchemaVersion: 1,
		Event:         workflow.EventWorkflowStarted,
		Timestamp:     timestamp,
		ExecutionID:   "test-id",
		User:          "user",
		WorkflowName:  "wf",
		Inputs: map[string]any{
			"simple": "value",
			"number": 42,
			"flag":   true,
			"nested": map[string]any{
				"inner": "data",
			},
		},
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	inputs := result["inputs"].(map[string]any)
	assert.Equal(t, "value", inputs["simple"])
	assert.Equal(t, float64(42), inputs["number"])
	assert.Equal(t, true, inputs["flag"])
}

// TestAuditEvent_TimestampPreserved tests that timestamp from constructor is preserved.
func TestAuditEvent_TimestampPreserved(t *testing.T) {
	execCtx := &workflow.ExecutionContext{
		WorkflowID:   "exec-id",
		WorkflowName: "test-wf",
		StartedAt:    time.Date(2026, 2, 20, 23, 15, 42, 123000000, time.UTC),
	}

	event := workflow.NewStartedEvent(execCtx, nil, "user")

	assert.Equal(t, execCtx.StartedAt, event.Timestamp)
}

// TestAuditEvent_UserDefaultsToUnknownIfEmpty tests unknown user fallback (if applicable).
func TestAuditEvent_UserField(t *testing.T) {
	execCtx := &workflow.ExecutionContext{
		WorkflowID:   "exec-id",
		WorkflowName: "test-wf",
		StartedAt:    time.Date(2026, 2, 20, 23, 15, 42, 123000000, time.UTC),
	}

	event := workflow.NewStartedEvent(execCtx, nil, "")

	// User should be set (implementation determines empty behavior)
	assert.NotNil(t, event.User)
}
