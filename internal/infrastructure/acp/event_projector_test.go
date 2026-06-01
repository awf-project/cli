package acp_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/acp"
)

// spySessionNotifier captures calls to NotifySessionUpdate for assertion
type spySessionNotifier struct {
	calls []spySessionUpdate
}

type spySessionUpdate struct {
	ctx        context.Context
	workflowID string
	update     acp.SessionUpdate
	err        error
}

func (s *spySessionNotifier) NotifySessionUpdate(ctx context.Context, workflowID string, update acp.SessionUpdate) error {
	s.calls = append(s.calls, spySessionUpdate{
		ctx:        ctx,
		workflowID: workflowID,
		update:     update,
		err:        nil,
	})
	return nil
}

// spyLogger captures debug and warn logs for assertion
type spyLogger struct {
	debugs []spyWarn
	warns  []spyWarn
}

type spyWarn struct {
	msg  string
	args []any
}

func (s *spyLogger) Debug(msg string, args ...any) {
	s.debugs = append(s.debugs, spyWarn{msg: msg, args: args})
}
func (s *spyLogger) Info(msg string, args ...any) {}
func (s *spyLogger) Warn(msg string, args ...any) {
	s.warns = append(s.warns, spyWarn{msg: msg, args: args})
}
func (s *spyLogger) Error(msg string, args ...any) {}
func (s *spyLogger) WithContext(ctx map[string]any) ports.Logger {
	return s
}

func TestWorkflowEventProjector_MapsEventToSessionUpdateKind(t *testing.T) {
	tests := []struct {
		name           string
		eventType      string
		metadata       map[string]string
		expectedKind   string
		expectedFields func(t *testing.T, update acp.SessionUpdate)
	}{
		{
			name:         "workflow started event maps to workflow_started kind",
			eventType:    workflow.EventWorkflowStarted,
			metadata:     map[string]string{"workflow_id": "wf-123", "workflow_name": "test-workflow"},
			expectedKind: "workflow_started",
			expectedFields: func(t *testing.T, update acp.SessionUpdate) {
				assert.Empty(t, update.StepName)
				assert.Empty(t, update.Error)
				assert.Empty(t, update.Duration)
			},
		},
		{
			name:         "workflow completed event maps to workflow_completed kind",
			eventType:    workflow.EventWorkflowCompleted,
			metadata:     map[string]string{"workflow_id": "wf-123", "workflow_name": "test-workflow", "duration_ms": "5000"},
			expectedKind: "workflow_completed",
			expectedFields: func(t *testing.T, update acp.SessionUpdate) {
				assert.Empty(t, update.StepName)
				assert.Empty(t, update.Error)
				assert.NotEmpty(t, update.Duration)
			},
		},
		{
			name:         "workflow failed event maps to workflow_failed kind",
			eventType:    workflow.EventWorkflowFailed,
			metadata:     map[string]string{"workflow_id": "wf-123", "workflow_name": "test-workflow", "error": "step failed"},
			expectedKind: "workflow_failed",
			expectedFields: func(t *testing.T, update acp.SessionUpdate) {
				assert.Empty(t, update.StepName)
				assert.NotEmpty(t, update.Error)
				assert.Empty(t, update.Duration)
			},
		},
		{
			name:         "step started event maps to step_started kind",
			eventType:    workflow.EventStepStarted,
			metadata:     map[string]string{"workflow_id": "wf-123", "step_name": "validate"},
			expectedKind: "step_started",
			expectedFields: func(t *testing.T, update acp.SessionUpdate) {
				assert.Equal(t, "validate", update.StepName)
				assert.Empty(t, update.Error)
				assert.Empty(t, update.Duration)
			},
		},
		{
			name:         "step completed event maps to step_completed kind",
			eventType:    workflow.EventStepCompleted,
			metadata:     map[string]string{"workflow_id": "wf-123", "step_name": "validate"},
			expectedKind: "step_completed",
			expectedFields: func(t *testing.T, update acp.SessionUpdate) {
				assert.Equal(t, "validate", update.StepName)
				assert.Empty(t, update.Error)
				assert.Empty(t, update.Duration)
			},
		},
		{
			name:         "step failed event maps to step_failed kind",
			eventType:    workflow.EventStepFailed,
			metadata:     map[string]string{"workflow_id": "wf-123", "step_name": "validate", "error": "validation failed"},
			expectedKind: "step_failed",
			expectedFields: func(t *testing.T, update acp.SessionUpdate) {
				assert.Equal(t, "validate", update.StepName)
				assert.NotEmpty(t, update.Error)
				assert.Empty(t, update.Duration)
			},
		},
		{
			name:         "step retrying event maps to step_retrying kind",
			eventType:    workflow.EventStepRetrying,
			metadata:     map[string]string{"workflow_id": "wf-123", "step_name": "validate"},
			expectedKind: "step_retrying",
			expectedFields: func(t *testing.T, update acp.SessionUpdate) {
				assert.Equal(t, "validate", update.StepName)
				assert.Empty(t, update.Error)
				assert.Empty(t, update.Duration)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier := &spySessionNotifier{}
			logger := &spyLogger{}
			projector := acp.NewWorkflowEventProjector(notifier, logger)

			event := pluginmodel.NewDomainEvent(tt.eventType, "core", tt.metadata, nil)
			err := projector.Publish(context.Background(), event)

			require.NoError(t, err)
			require.Len(t, notifier.calls, 1, "NotifySessionUpdate should be called exactly once")

			call := notifier.calls[0]
			assert.Equal(t, "wf-123", call.workflowID)
			assert.Equal(t, tt.expectedKind, call.update.Kind)
			tt.expectedFields(t, call.update)
		})
	}
}

func TestWorkflowEventProjector_SkipsEventsWithoutWorkflowID(t *testing.T) {
	notifier := &spySessionNotifier{}
	logger := &spyLogger{}
	projector := acp.NewWorkflowEventProjector(notifier, logger)

	// Event with empty workflow_id metadata
	event := pluginmodel.NewDomainEvent(
		workflow.EventWorkflowStarted,
		"core",
		map[string]string{
			"workflow_id":   "", // empty
			"workflow_name": "test-workflow",
		},
		nil,
	)

	err := projector.Publish(context.Background(), event)

	require.NoError(t, err)
	assert.Len(t, notifier.calls, 0, "NotifySessionUpdate should not be called for event without workflow_id")
}

func TestWorkflowEventProjector_SkipsUnknownEventTypes(t *testing.T) {
	notifier := &spySessionNotifier{}
	logger := &spyLogger{}
	projector := acp.NewWorkflowEventProjector(notifier, logger)

	// Event with non-workflow event type
	event := pluginmodel.NewDomainEvent(
		"unknown.event",
		"core",
		map[string]string{
			"workflow_id": "wf-123",
		},
		nil,
	)

	err := projector.Publish(context.Background(), event)

	require.NoError(t, err)
	assert.Len(t, notifier.calls, 0, "NotifySessionUpdate should not be called for unknown event type")
	// m-7: unhandled event types must emit a Debug log so they are traceable
	require.Len(t, logger.debugs, 1, "unknown event type must emit a Debug log")
	assert.Equal(t, "acp projector: unhandled event type", logger.debugs[0].msg)
}

func TestWorkflowEventProjector_NotifierErrorPropagated(t *testing.T) {
	notifierErr := &errorSessionNotifier{err: assert.AnError}
	logger := &spyLogger{}
	projector := acp.NewWorkflowEventProjector(notifierErr, logger)

	event := pluginmodel.NewDomainEvent(
		workflow.EventWorkflowStarted,
		"core",
		map[string]string{
			"workflow_id":   "wf-123",
			"workflow_name": "test-workflow",
		},
		nil,
	)

	err := projector.Publish(context.Background(), event)

	// M-3: notifier errors must be propagated so callers can react
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)

	// Error must also be logged as Warn before returning
	require.Len(t, logger.warns, 1, "Logger should capture one warn call")
	assert.Contains(t, logger.warns[0].msg, "notify")
}

func TestWorkflowEventProjector_Close(t *testing.T) {
	notifier := &spySessionNotifier{}
	logger := &spyLogger{}
	projector := acp.NewWorkflowEventProjector(notifier, logger)

	err := projector.Close()

	assert.NoError(t, err)
}

// errorSessionNotifier is a SessionNotifier that returns an error
type errorSessionNotifier struct {
	err error
}

func (e *errorSessionNotifier) NotifySessionUpdate(ctx context.Context, workflowID string, update acp.SessionUpdate) error {
	return e.err
}

// TestWorkflowEventProjector_NilEventDoesNotPanic verifies the nil-guard contract:
// passing a nil event must return nil without panicking (C3 fix) and must log
// a WARN so a buggy caller is visible in diagnostics.
func TestWorkflowEventProjector_NilEventDoesNotPanic(t *testing.T) {
	notifier := &spySessionNotifier{}
	logger := &spyLogger{}
	projector := acp.NewWorkflowEventProjector(notifier, logger)

	require.NotPanics(t, func() {
		err := projector.Publish(context.Background(), nil)
		assert.NoError(t, err)
	})
	assert.Len(t, notifier.calls, 0, "nil event must not trigger any notification")
	require.Len(t, logger.warns, 1, "nil event must log a WARN so the buggy caller is visible")
	assert.Equal(t, "acp projector: nil event dropped", logger.warns[0].msg)
}
