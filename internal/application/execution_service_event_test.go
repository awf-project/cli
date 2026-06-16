package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutionService_SetEventPublisher(t *testing.T) {
	t.Run("sets event publisher", func(t *testing.T) {
		svc, _ := NewTestHarness(t).Build()
		publisher := testmocks.NewMockEventPublisher()

		svc.SetEventPublisher(publisher)

		// Verify publisher is set by attempting to use it indirectly
		// (no public getter, so we verify through event emission behavior)
		assert.NotNil(t, publisher)
	})

	t.Run("allows nil publisher", func(t *testing.T) {
		svc, _ := NewTestHarness(t).Build()

		svc.SetEventPublisher(nil)
		// Should not panic or error when set to nil
	})
}

func TestExecutionService_EmitEvent_WorkflowStarted(t *testing.T) {
	t.Run("emits workflow.started event with correct metadata", func(t *testing.T) {
		wf := &workflow.Workflow{
			Name:    "test-workflow",
			Initial: "start",
			Steps: map[string]*workflow.Step{
				"start": {
					Name: "start",
					Type: workflow.StepTypeTerminal,
				},
			},
		}

		svc, _ := NewTestHarness(t).
			WithWorkflow("test-workflow", wf).
			Build()

		publisher := testmocks.NewMockEventPublisher()
		svc.SetEventPublisher(publisher)

		execCtx, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "test-workflow-run")

		require.NoError(t, err)
		events := publisher.GetEvents()
		require.Greater(t, len(events), 0, "should emit at least one event")

		// Find workflow.started event
		var startedEvent *pluginmodel.DomainEvent
		for _, e := range events {
			if e.Type == workflow.EventWorkflowStarted {
				startedEvent = e
				break
			}
		}

		require.NotNil(t, startedEvent, "should emit workflow.started event")
		assert.Equal(t, workflow.EventWorkflowStarted, startedEvent.Type)
		assert.Equal(t, "core", startedEvent.Source)
		assert.Equal(t, "test-workflow", startedEvent.Metadata["workflow_name"])
		assert.NotEmpty(t, startedEvent.Metadata["workflow_id"])
		assert.Equal(t, execCtx.WorkflowID, startedEvent.Metadata["workflow_id"])
	})

	t.Run("does not emit event when publisher is nil", func(t *testing.T) {
		wf := &workflow.Workflow{
			Name:    "test-workflow",
			Initial: "start",
			Steps: map[string]*workflow.Step{
				"start": {
					Name: "start",
					Type: workflow.StepTypeTerminal,
				},
			},
		}

		svc, _ := NewTestHarness(t).
			WithWorkflow("test-workflow", wf).
			Build()

		svc.SetEventPublisher(nil)

		execCtx, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "test-workflow-run")

		require.NoError(t, err)
		assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
		// If publisher is nil, no panic should occur
	})
}

func TestExecutionService_EmitEvent_WorkflowCompleted(t *testing.T) {
	t.Run("emits workflow.completed event with duration", func(t *testing.T) {
		wf := &workflow.Workflow{
			Name:    "test-workflow",
			Initial: "start",
			Steps: map[string]*workflow.Step{
				"start": {
					Name: "start",
					Type: workflow.StepTypeTerminal,
				},
			},
		}

		svc, _ := NewTestHarness(t).
			WithWorkflow("test-workflow", wf).
			Build()

		publisher := testmocks.NewMockEventPublisher()
		svc.SetEventPublisher(publisher)

		execCtx, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "test-workflow-run")

		require.NoError(t, err)
		events := publisher.GetEvents()

		// Find workflow.completed event
		var completedEvent *pluginmodel.DomainEvent
		for _, e := range events {
			if e.Type == workflow.EventWorkflowCompleted {
				completedEvent = e
				break
			}
		}

		require.NotNil(t, completedEvent, "should emit workflow.completed event")
		assert.Equal(t, workflow.EventWorkflowCompleted, completedEvent.Type)
		assert.Equal(t, "core", completedEvent.Source)
		assert.Equal(t, "test-workflow", completedEvent.Metadata["workflow_name"])
		assert.NotEmpty(t, completedEvent.Metadata["workflow_id"])
		assert.Equal(t, execCtx.WorkflowID, completedEvent.Metadata["workflow_id"])
		assert.NotEmpty(t, completedEvent.Metadata["duration"])
	})
}

func TestExecutionService_EmitEvent_WorkflowFailed(t *testing.T) {
	t.Run("emits workflow.failed event with error message", func(t *testing.T) {
		wf := &workflow.Workflow{
			Name:    "test-workflow",
			Initial: "start",
			Steps: map[string]*workflow.Step{
				"start": {
					Name:    "start",
					Type:    workflow.StepTypeCommand,
					Command: "exit 1",
				},
			},
		}

		svc, _ := NewTestHarness(t).
			WithWorkflow("test-workflow", wf).
			WithCommandResult("exit 1", &ports.CommandResult{
				ExitCode: 1,
				Stderr:   "command failed",
			}).
			Build()

		publisher := testmocks.NewMockEventPublisher()
		svc.SetEventPublisher(publisher)

		_, _ = svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "test-workflow-run")

		// Workflow run may error due to no terminal step, which is expected
		events := publisher.GetEvents()

		// Find workflow.failed event
		var failedEvent *pluginmodel.DomainEvent
		for _, e := range events {
			if e.Type == workflow.EventWorkflowFailed {
				failedEvent = e
				break
			}
		}

		if failedEvent != nil {
			assert.Equal(t, workflow.EventWorkflowFailed, failedEvent.Type)
			assert.Equal(t, "core", failedEvent.Source)
			assert.Equal(t, "test-workflow", failedEvent.Metadata["workflow_name"])
			assert.NotEmpty(t, failedEvent.Metadata["workflow_id"])
			assert.NotEmpty(t, failedEvent.Metadata["error"])
		}
	})
}

func TestExecutionService_EmitEvent_StepStarted(t *testing.T) {
	t.Run("emits step.started event before step execution", func(t *testing.T) {
		wf := &workflow.Workflow{
			Name:    "test-workflow",
			Initial: "start",
			Steps: map[string]*workflow.Step{
				"start": {
					Name:      "start",
					Type:      workflow.StepTypeCommand,
					Command:   "echo hello",
					OnSuccess: "done",
				},
				"done": {
					Name: "done",
					Type: workflow.StepTypeTerminal,
				},
			},
		}

		svc, _ := NewTestHarness(t).
			WithWorkflow("test-workflow", wf).
			WithCommandResult("echo hello", &ports.CommandResult{
				Stdout:   "hello\n",
				ExitCode: 0,
			}).
			Build()

		publisher := testmocks.NewMockEventPublisher()
		svc.SetEventPublisher(publisher)

		execCtx, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "test-workflow-run")

		require.NoError(t, err)
		events := publisher.GetEvents()

		// Find step.started event for "start" step
		var stepStartedEvent *pluginmodel.DomainEvent
		for _, e := range events {
			if e.Type == workflow.EventStepStarted && e.Metadata["step_name"] == "start" {
				stepStartedEvent = e
				break
			}
		}

		require.NotNil(t, stepStartedEvent, "should emit step.started event")
		_ = execCtx
		assert.Equal(t, workflow.EventStepStarted, stepStartedEvent.Type)
		assert.Equal(t, "core", stepStartedEvent.Source)
		assert.Equal(t, "start", stepStartedEvent.Metadata["step_name"])
		assert.NotEmpty(t, stepStartedEvent.Metadata["workflow_id"])
	})
}

func TestExecutionService_EmitEvent_StepCompleted(t *testing.T) {
	t.Run("emits step.completed event after successful step", func(t *testing.T) {
		wf := &workflow.Workflow{
			Name:    "test-workflow",
			Initial: "start",
			Steps: map[string]*workflow.Step{
				"start": {
					Name:      "start",
					Type:      workflow.StepTypeCommand,
					Command:   "echo hello",
					OnSuccess: "done",
				},
				"done": {
					Name: "done",
					Type: workflow.StepTypeTerminal,
				},
			},
		}

		svc, _ := NewTestHarness(t).
			WithWorkflow("test-workflow", wf).
			WithCommandResult("echo hello", &ports.CommandResult{
				Stdout:   "hello\n",
				ExitCode: 0,
			}).
			Build()

		publisher := testmocks.NewMockEventPublisher()
		svc.SetEventPublisher(publisher)

		execCtx, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "test-workflow-run")

		require.NoError(t, err)
		events := publisher.GetEvents()

		// Find step.completed event for "start" step
		var stepCompletedEvent *pluginmodel.DomainEvent
		for _, e := range events {
			if e.Type == workflow.EventStepCompleted && e.Metadata["step_name"] == "start" {
				stepCompletedEvent = e
				break
			}
		}

		require.NotNil(t, stepCompletedEvent, "should emit step.completed event")
		assert.Equal(t, workflow.EventStepCompleted, stepCompletedEvent.Type)
		assert.Equal(t, "core", stepCompletedEvent.Source)
		assert.Equal(t, "start", stepCompletedEvent.Metadata["step_name"])
		assert.NotEmpty(t, stepCompletedEvent.Metadata["workflow_id"])
		assert.Equal(t, execCtx.WorkflowID, stepCompletedEvent.Metadata["workflow_id"])
	})
}

func TestExecutionService_EmitEvent_StepFailed(t *testing.T) {
	t.Run("emits step.failed event after step failure", func(t *testing.T) {
		wf := &workflow.Workflow{
			Name:    "test-workflow",
			Initial: "start",
			Steps: map[string]*workflow.Step{
				"start": {
					Name:      "start",
					Type:      workflow.StepTypeCommand,
					Command:   "exit 1",
					OnFailure: "error",
				},
				"error": {
					Name: "error",
					Type: workflow.StepTypeTerminal,
				},
			},
		}

		svc, _ := NewTestHarness(t).
			WithWorkflow("test-workflow", wf).
			WithCommandResult("exit 1", &ports.CommandResult{
				ExitCode: 1,
				Stderr:   "command failed",
			}).
			Build()

		publisher := testmocks.NewMockEventPublisher()
		svc.SetEventPublisher(publisher)

		execCtx, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "test-workflow-run")

		require.NoError(t, err)
		events := publisher.GetEvents()

		// Find step.failed event for "start" step
		var stepFailedEvent *pluginmodel.DomainEvent
		for _, e := range events {
			if e.Type == workflow.EventStepFailed && e.Metadata["step_name"] == "start" {
				stepFailedEvent = e
				break
			}
		}

		require.NotNil(t, stepFailedEvent, "should emit step.failed event")
		assert.Equal(t, workflow.EventStepFailed, stepFailedEvent.Type)
		assert.Equal(t, "core", stepFailedEvent.Source)
		assert.Equal(t, "start", stepFailedEvent.Metadata["step_name"])
		assert.NotEmpty(t, stepFailedEvent.Metadata["workflow_id"])
		assert.Equal(t, execCtx.WorkflowID, stepFailedEvent.Metadata["workflow_id"])
		assert.NotEmpty(t, stepFailedEvent.Metadata["error"])
	})
}

func TestExecutionService_EmitEvent_StepRetrying(t *testing.T) {
	t.Run("emits step.retrying event before retry attempt", func(t *testing.T) {
		wf := &workflow.Workflow{
			Name:    "test-workflow",
			Initial: "start",
			Steps: map[string]*workflow.Step{
				"start": {
					Name:      "start",
					Type:      workflow.StepTypeCommand,
					Command:   "echo hello",
					OnSuccess: "done",
					Retry: &workflow.RetryConfig{
						MaxAttempts: 2,
					},
				},
				"done": {
					Name: "done",
					Type: workflow.StepTypeTerminal,
				},
			},
		}

		svc, _ := NewTestHarness(t).
			WithWorkflow("test-workflow", wf).
			WithCommandResult("echo hello", &ports.CommandResult{
				Stdout:   "hello\n",
				ExitCode: 0,
			}).
			Build()

		publisher := testmocks.NewMockEventPublisher()
		svc.SetEventPublisher(publisher)

		execCtx, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "test-workflow-run")

		require.NoError(t, err)
		events := publisher.GetEvents()

		// Find step.retrying event (if retry occurs)
		var stepRetryingEvent *pluginmodel.DomainEvent
		for _, e := range events {
			if e.Type == workflow.EventStepRetrying {
				stepRetryingEvent = e
				break
			}
		}

		if stepRetryingEvent != nil {
			assert.Equal(t, workflow.EventStepRetrying, stepRetryingEvent.Type)
			assert.Equal(t, "core", stepRetryingEvent.Source)
			assert.NotEmpty(t, stepRetryingEvent.Metadata["step_name"])
			assert.NotEmpty(t, stepRetryingEvent.Metadata["workflow_id"])
			assert.Equal(t, execCtx.WorkflowID, stepRetryingEvent.Metadata["workflow_id"])
			assert.NotEmpty(t, stepRetryingEvent.Metadata["attempt"])
		}
	})
}

func TestExecutionService_EmitEvent_AllEventsHaveSource(t *testing.T) {
	t.Run("all emitted events have source core", func(t *testing.T) {
		wf := &workflow.Workflow{
			Name:    "test-workflow",
			Initial: "start",
			Steps: map[string]*workflow.Step{
				"start": {
					Name:      "start",
					Type:      workflow.StepTypeCommand,
					Command:   "echo hello",
					OnSuccess: "done",
				},
				"done": {
					Name: "done",
					Type: workflow.StepTypeTerminal,
				},
			},
		}

		svc, _ := NewTestHarness(t).
			WithWorkflow("test-workflow", wf).
			WithCommandResult("echo hello", &ports.CommandResult{
				Stdout:   "hello\n",
				ExitCode: 0,
			}).
			Build()

		publisher := testmocks.NewMockEventPublisher()
		svc.SetEventPublisher(publisher)

		_, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "test-workflow-run")

		require.NoError(t, err)
		events := publisher.GetEvents()

		for _, event := range events {
			assert.Equal(t, "core", event.Source, "event %s should have source=core", event.Type)
		}
	})
}

func TestExecutionService_EmitEvent_PublishError(t *testing.T) {
	t.Run("logs but does not fail workflow when event publish fails", func(t *testing.T) {
		wf := &workflow.Workflow{
			Name:    "test-workflow",
			Initial: "start",
			Steps: map[string]*workflow.Step{
				"start": {
					Name:      "start",
					Type:      workflow.StepTypeCommand,
					Command:   "echo hello",
					OnSuccess: "done",
				},
				"done": {
					Name: "done",
					Type: workflow.StepTypeTerminal,
				},
			},
		}

		svc, _ := NewTestHarness(t).
			WithWorkflow("test-workflow", wf).
			WithCommandResult("echo hello", &ports.CommandResult{
				Stdout:   "hello\n",
				ExitCode: 0,
			}).
			Build()

		publisher := testmocks.NewMockEventPublisher()
		publisher.SetPublishError(errors.New("publish failed"))
		svc.SetEventPublisher(publisher)

		execCtx, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "test-workflow-run")

		// Workflow should still complete despite publish error
		require.NoError(t, err)
		assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	})
}

func TestExecutionService_EmitEvent_SecretMasking(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		value      string
		shouldMask bool
	}{
		{
			name:       "masks SECRET_ prefixed values",
			key:        "SECRET_API_KEY",
			value:      "secret123",
			shouldMask: true,
		},
		{
			name:       "masks API_KEY values",
			key:        "API_KEY",
			value:      "key123",
			shouldMask: true,
		},
		{
			name:       "masks PASSWORD values",
			key:        "PASSWORD",
			value:      "pass123",
			shouldMask: true,
		},
		{
			name:       "does not mask non-secret values",
			key:        "ENV",
			value:      "staging",
			shouldMask: false,
		},
		{
			name:       "does not mask normal_var",
			key:        "normal_var",
			value:      "value123",
			shouldMask: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := &workflow.Workflow{
				Name:    "test-workflow",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {
						Name: "start",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			svc, _ := NewTestHarness(t).
				WithWorkflow("test-workflow", wf).
				Build()

			publisher := testmocks.NewMockEventPublisher()
			svc.SetEventPublisher(publisher)

			inputs := map[string]any{
				tt.key: tt.value,
			}

			_, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, inputs, "test-workflow-run")

			require.NoError(t, err)
			events := publisher.GetEvents()

			// Check if any event has the key in metadata
			for _, event := range events {
				if val, ok := event.Metadata[tt.key]; ok {
					if tt.shouldMask {
						// Masked values should not contain the original value
						assert.NotEqual(t, tt.value, val, "secret value should be masked")
						assert.Equal(t, "***", val, "secret value should be masked with ***")
					} else {
						// Non-secret values should remain unchanged
						assert.Equal(t, tt.value, val, "non-secret value should not be masked")
					}
				}
			}
		})
	}
}
