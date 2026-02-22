package workflow

import (
	"bytes"
	"encoding/json"
	"time"
)

const (
	EventWorkflowStarted   = "workflow.started"
	EventWorkflowCompleted = "workflow.completed"
	auditSchemaVersion     = 1
)

// AuditEvent represents a single audit trail entry for a workflow execution.
// Two events are emitted per execution: workflow.started and workflow.completed.
type AuditEvent struct {
	SchemaVersion   int            `json:"schema_version"`
	Event           string         `json:"event"`
	Timestamp       time.Time      `json:"timestamp"`
	ExecutionID     string         `json:"execution_id"`
	User            string         `json:"user"`
	WorkflowName    string         `json:"workflow_name"`
	Inputs          map[string]any `json:"inputs,omitempty"`
	InputsTruncated bool           `json:"inputs_truncated,omitempty"`
	Status          string         `json:"status,omitempty"`
	ExitCode        *int           `json:"exit_code,omitempty"`
	DurationMs      *int64         `json:"duration_ms,omitempty"`
	Error           string         `json:"error,omitempty"`
}

// NewStartedEvent creates an audit event for workflow start.
// Inputs should already be secret-masked by the caller.
// Called immediately after ExecutionContext creation, before WorkflowStart hooks.
func NewStartedEvent(execCtx *ExecutionContext, maskedInputs map[string]any, user string) AuditEvent {
	return AuditEvent{
		SchemaVersion: auditSchemaVersion,
		Event:         EventWorkflowStarted,
		ExecutionID:   execCtx.WorkflowID,
		Timestamp:     execCtx.StartedAt,
		User:          user,
		WorkflowName:  execCtx.WorkflowName,
		Inputs:        maskedInputs,
	}
}

// NewCompletedEvent creates an audit event for workflow completion.
func NewCompletedEvent(execCtx *ExecutionContext, user, errorMsg string) AuditEvent {
	status := "success"
	if execCtx.ExitCode != 0 {
		status = "failure"
	}

	exitCode := execCtx.ExitCode
	durationMs := execCtx.CompletedAt.Sub(execCtx.StartedAt).Milliseconds()

	return AuditEvent{
		SchemaVersion: auditSchemaVersion,
		Event:         EventWorkflowCompleted,
		ExecutionID:   execCtx.WorkflowID,
		Timestamp:     execCtx.CompletedAt,
		User:          user,
		WorkflowName:  execCtx.WorkflowName,
		Status:        status,
		ExitCode:      &exitCode,
		DurationMs:    &durationMs,
		Error:         errorMsg,
	}
}

// MarshalJSON produces ordered JSON with millisecond timestamp precision.
// Field order: event, execution_id, timestamp, user, workflow_name,
// then event-specific fields, then schema_version.
func (e AuditEvent) MarshalJSON() ([]byte, error) { //nolint:gocritic // hugeParam: value receiver required so json.Marshal(event) invokes custom marshaler
	var buf bytes.Buffer
	buf.WriteByte('{')

	writeJSONField(&buf, "event", e.Event, false)
	writeJSONField(&buf, "execution_id", e.ExecutionID, true)
	writeJSONField(&buf, "timestamp", e.Timestamp.Format("2006-01-02T15:04:05.000Z07:00"), true)
	writeJSONField(&buf, "user", e.User, true)
	writeJSONField(&buf, "workflow_name", e.WorkflowName, true)

	if e.Inputs != nil {
		writeJSONField(&buf, "inputs", e.Inputs, true)
	}
	if e.InputsTruncated {
		writeJSONField(&buf, "inputs_truncated", true, true)
	}
	if e.Status != "" {
		writeJSONField(&buf, "status", e.Status, true)
	}
	if e.ExitCode != nil {
		writeJSONField(&buf, "exit_code", int64(*e.ExitCode), true)
	}
	if e.DurationMs != nil {
		writeJSONField(&buf, "duration_ms", *e.DurationMs, true)
	}
	if e.Error != "" {
		writeJSONField(&buf, "error", e.Error, true)
	}

	writeJSONField(&buf, "schema_version", int64(e.SchemaVersion), true)

	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func writeJSONField(buf *bytes.Buffer, key string, value any, comma bool) {
	if comma {
		buf.WriteByte(',')
	}
	keyBytes, _ := json.Marshal(key)
	valBytes, _ := json.Marshal(value)
	buf.Write(keyBytes)
	buf.WriteByte(':')
	buf.Write(valBytes)
}
