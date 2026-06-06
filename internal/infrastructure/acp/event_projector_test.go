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

// fakeSessionUpdateEmitter records EmitSessionUpdate calls for testing.
type fakeSessionUpdateEmitter struct {
	calls []fakeEmitterCall
}

type fakeEmitterCall struct {
	ctx       context.Context
	sessionID string
	kind      string
	fields    map[string]any
}

func (f *fakeSessionUpdateEmitter) EmitSessionUpdate(ctx context.Context, sessionID, kind string, fields map[string]any) error {
	f.calls = append(f.calls, fakeEmitterCall{
		ctx:       ctx,
		sessionID: sessionID,
		kind:      kind,
		fields:    fields,
	})
	return nil
}

// spyLogger captures debug and warn logs for assertion (reused from event_projector_test.go)
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

func TestWorkflowEventProjector_PublishWorkflowStarted(t *testing.T) {
	tests := []struct {
		name           string
		eventType      string
		metadata       map[string]string
		expectedKind   string
		expectedFields func(t *testing.T, fields map[string]any)
	}{
		{
			name:         "workflow_started event emitted correctly",
			eventType:    workflow.EventWorkflowStarted,
			metadata:     map[string]string{"workflow_id": "wf-123", "workflow_name": "test-workflow"},
			expectedKind: "workflow_started",
			expectedFields: func(t *testing.T, fields map[string]any) {
				assert.Empty(t, fields, "workflow_started should have empty fields")
			},
		},
		{
			name:         "workflow_completed event emitted correctly",
			eventType:    workflow.EventWorkflowCompleted,
			metadata:     map[string]string{"workflow_id": "wf-123", "workflow_name": "test-workflow", "duration_ms": "5000"},
			expectedKind: "workflow_completed",
			expectedFields: func(t *testing.T, fields map[string]any) {
				assert.NotNil(t, fields["duration_ms"], "workflow_completed should include duration_ms")
			},
		},
		{
			name:         "workflow_failed event emitted correctly",
			eventType:    workflow.EventWorkflowFailed,
			metadata:     map[string]string{"workflow_id": "wf-123", "workflow_name": "test-workflow", "error": "step failed"},
			expectedKind: "workflow_failed",
			expectedFields: func(t *testing.T, fields map[string]any) {
				assert.NotNil(t, fields["error"], "workflow_failed should include error")
			},
		},
		{
			name:         "step_started event emitted correctly",
			eventType:    workflow.EventStepStarted,
			metadata:     map[string]string{"workflow_id": "wf-123", "step_name": "validate"},
			expectedKind: "step_started",
			expectedFields: func(t *testing.T, fields map[string]any) {
				assert.Equal(t, "validate", fields["step_name"], "step_started should include step_name")
			},
		},
		{
			name:         "step_completed event emitted correctly",
			eventType:    workflow.EventStepCompleted,
			metadata:     map[string]string{"workflow_id": "wf-123", "step_name": "validate"},
			expectedKind: "step_completed",
			expectedFields: func(t *testing.T, fields map[string]any) {
				assert.Equal(t, "validate", fields["step_name"], "step_completed should include step_name")
			},
		},
		{
			name:         "step_failed event emitted correctly",
			eventType:    workflow.EventStepFailed,
			metadata:     map[string]string{"workflow_id": "wf-123", "step_name": "validate", "error": "validation failed"},
			expectedKind: "step_failed",
			expectedFields: func(t *testing.T, fields map[string]any) {
				assert.Equal(t, "validate", fields["step_name"], "step_failed should include step_name")
				assert.NotNil(t, fields["error"], "step_failed should include error")
			},
		},
		{
			name:         "step_retrying event emitted correctly",
			eventType:    workflow.EventStepRetrying,
			metadata:     map[string]string{"workflow_id": "wf-123", "step_name": "validate"},
			expectedKind: "step_retrying",
			expectedFields: func(t *testing.T, fields map[string]any) {
				assert.Equal(t, "validate", fields["step_name"], "step_retrying should include step_name")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitter := &fakeSessionUpdateEmitter{}
			logger := &spyLogger{}
			projector := acp.NewWorkflowEventProjector("sess_test", emitter, logger)

			event := pluginmodel.NewDomainEvent(tt.eventType, "core", tt.metadata, nil)
			err := projector.Publish(context.Background(), event)

			require.NoError(t, err)
			require.Len(t, emitter.calls, 1, "EmitSessionUpdate should be called exactly once")

			call := emitter.calls[0]
			// The emit MUST target the ACP session ID bound at construction, NOT the run's
			// workflow_id metadata ("wf-123"). Routing by workflow_id sent updates to a
			// session the editor never created, so they were silently dropped.
			assert.Equal(t, "sess_test", call.sessionID, "emitter must be called with the ACP session ID, not workflow_id")
			assert.Equal(t, tt.expectedKind, call.kind, "emitter should be called with correct kind")
			tt.expectedFields(t, call.fields)
		})
	}
}

func TestWorkflowEventProjector_PublishNilEvent(t *testing.T) {
	emitter := &fakeSessionUpdateEmitter{}
	logger := &spyLogger{}
	projector := acp.NewWorkflowEventProjector("sess_test", emitter, logger)

	require.NotPanics(t, func() {
		err := projector.Publish(context.Background(), nil)
		assert.NoError(t, err)
	})
	assert.Len(t, emitter.calls, 0, "nil event must not trigger any emission")
	require.Len(t, logger.warns, 1, "nil event must log a WARN so the buggy caller is visible")
	assert.Equal(t, "acp projector: nil event dropped", logger.warns[0].msg)
}

func TestWorkflowEventProjector_SkipsEventsWithoutWorkflowID(t *testing.T) {
	emitter := &fakeSessionUpdateEmitter{}
	logger := &spyLogger{}
	projector := acp.NewWorkflowEventProjector("sess_test", emitter, logger)

	// Event with empty workflow_id metadata
	event := pluginmodel.NewDomainEvent(
		workflow.EventWorkflowStarted,
		"core",
		map[string]string{
			"workflow_id":   "",
			"workflow_name": "test-workflow",
		},
		nil,
	)

	err := projector.Publish(context.Background(), event)

	require.NoError(t, err)
	assert.Len(t, emitter.calls, 0, "EmitSessionUpdate should not be called for event without workflow_id")
}

func TestWorkflowEventProjector_ImplementsEventPublisher(t *testing.T) {
	emitter := &fakeSessionUpdateEmitter{}
	logger := &spyLogger{}
	projector := acp.NewWorkflowEventProjector("sess_test", emitter, logger)

	// Verify projector satisfies ports.EventPublisher interface
	// (compile-time assertion in event_projector.go: var _ ports.EventPublisher = (*WorkflowEventProjector)(nil))
	event := pluginmodel.NewDomainEvent(
		workflow.EventWorkflowStarted,
		"core",
		map[string]string{"workflow_id": "wf-123"},
		nil,
	)
	err := projector.Publish(context.Background(), event)

	assert.NoError(t, err)
}

func TestWorkflowEventProjector_Close(t *testing.T) {
	emitter := &fakeSessionUpdateEmitter{}
	logger := &spyLogger{}
	projector := acp.NewWorkflowEventProjector("sess_test", emitter, logger)

	err := projector.Close()

	assert.NoError(t, err)
}
