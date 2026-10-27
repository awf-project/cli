package application

import (
	"context"
	"errors"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResumePreflightUsesOverridesWithoutMutatingRejectedState(t *testing.T) {
	failure := errors.New("authentication unavailable")
	provider := &preflightTestProvider{name: "test", preflightErr: failure}
	wf := preflightWorkflow("{{inputs.provider}}", "single")
	wf.Inputs = []workflow.Input{{Name: "provider", Type: "string"}, {Name: "model", Type: "string"}}
	wf.Steps["agent"].Agent.Options = map[string]any{"model": "{{inputs.model}}"}
	rig := newFinalizationTestRig(t, wf, true)
	rig.service.workflowSvc.SetAgentRegistry(&preflightTestRegistry{provider: provider})
	saved := rig.store.states["b017-resume"]
	saved.SetInput("provider", "test")
	saved.SetInput("model", "original")
	saved.SetStepState("done", workflow.StepState{Status: workflow.StatusFailed})
	overrides := map[string]any{"model": "replacement"}
	before := saved.GetInputs()
	updatedAt := saved.UpdatedAt

	result, err := rig.service.Resume(context.Background(), saved.WorkflowID, overrides, "done")

	require.ErrorIs(t, err, failure)
	assert.Nil(t, result)
	assert.Equal(t, map[string]any{"model": "replacement"}, provider.options)
	assert.Equal(t, before, saved.GetInputs())
	assert.Equal(t, updatedAt, saved.UpdatedAt)
	assert.Equal(t, wf.Initial, saved.CurrentStep)
	assert.Equal(t, workflow.StatusFailed, saved.Status)
	assert.Empty(t, rig.trace.calls, "rejection must precede checkpoints, starts and execution")
	assert.Zero(t, provider.executeCalls.Load())
}

func TestResumeRejectsInvalidOperationBeforeStarting(t *testing.T) {
	wf := &workflow.Workflow{
		Name: "resume-invalid-operation", Initial: "request",
		Steps: map[string]*workflow.Step{
			"request": {Name: "request", Type: workflow.StepTypeOperation, Operation: "http.request", OperationInputs: map[string]any{"method": "GET"}, OnSuccess: "done"},
			"done":    {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}
	rig := newFinalizationTestRig(t, wf, true)
	rig.service.workflowSvc.SetPluginOperationProvider(NewBuiltinOperationProvider(rig.logger, NotifyConfig{}))
	_, err := rig.service.Resume(context.Background(), "b017-resume", nil, "current")
	require.ErrorContains(t, err, "url")
	assert.Empty(t, rig.trace.calls)
}
