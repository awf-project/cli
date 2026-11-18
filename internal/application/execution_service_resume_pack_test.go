package application_test

import (
	"context"
	"fmt"
	"maps"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// packChildWorkflow is a workflow that only a pack loader knows: one step, then a terminal state.
func packChildWorkflow() *workflow.Workflow {
	return &workflow.Workflow{
		Name:    "child",
		Initial: "run",
		Steps: map[string]*workflow.Step{
			"run":  {Name: "run", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}
}

func TestExecutionService_RunRecordsThePackOfTheWorkflow(t *testing.T) {
	tests := []struct {
		name     string
		packName string
	}{
		{name: "pack workflow", packName: "probe"},
		{name: "local workflow"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execSvc, mocks := NewTestHarness(t).Build()
			if tt.packName != "" {
				execSvc.SetAWFPaths(map[string]string{application.AWFPackNameKey: tt.packName})
			}

			_, err := execSvc.RunWithWorkflowAndRunID(context.Background(), packChildWorkflow(), nil, "run-1")
			require.NoError(t, err)

			saved, err := mocks.StateStore.Load(context.Background(), "run-1")
			require.NoError(t, err)
			require.NotNil(t, saved)
			assert.Equal(t, tt.packName, saved.PackName)
		})
	}
}

func TestExecutionService_RunRecordsTheEffectivePackOfANamespacedChildWorkflow(t *testing.T) {
	childWorkflow := packChildWorkflow()
	parentWorkflow := &workflow.Workflow{
		Name:    "parent",
		Initial: "call_child",
		Steps: map[string]*workflow.Step{
			"call_child": {
				Name: "call_child",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "probe/child",
					Inputs:   map[string]string{"marker": "from-parent"},
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("parent", parentWorkflow).
		Build()
	execSvc.SetPackWorkflowLoader(func(_ context.Context, packName, workflowName string) (*workflow.Workflow, string, error) {
		require.Equal(t, "probe", packName)
		require.Equal(t, "child", workflowName)
		return childWorkflow, "/packs/probe", nil
	})

	parentCtx, err := execSvc.RunWithWorkflowAndRunID(context.Background(), parentWorkflow, nil, "parent-run")

	require.NoError(t, err)
	require.Equal(t, workflow.StatusCompleted, parentCtx.Status)
	childIDs, err := mocks.StateStore.List(context.Background())
	require.NoError(t, err)
	require.Len(t, childIDs, 2)

	var childID string
	for _, id := range childIDs {
		if id != parentCtx.WorkflowID {
			childID = id
			break
		}
	}
	require.NotEmpty(t, childID)

	childCtx, err := mocks.StateStore.Load(context.Background(), childID)
	require.NoError(t, err)
	require.NotNil(t, childCtx)
	assert.Equal(t, "probe", childCtx.PackName)
	assert.Equal(t, "child", childCtx.WorkflowName)
	assert.Equal(t, "from-parent", childCtx.GetInputs()["marker"])
}

func TestExecutionService_ResumeLoadsTheWorkflowFromItsPack(t *testing.T) {
	execSvc, mocks := NewTestHarness(t).Build()
	configuredPaths := map[string]string{
		application.AWFPackNameKey: "configured-pack",
		"scripts_dir":              "/configured/scripts",
		"prompts_dir":              "/configured/prompts",
	}
	originalPaths := maps.Clone(configuredPaths)
	execSvc.SetAWFPaths(configuredPaths)
	var loaded []string
	execSvc.SetPackWorkflowLoader(func(_ context.Context, packName, workflowName string) (*workflow.Workflow, string, error) {
		loaded = append(loaded, packName+"/"+workflowName)
		if packName == "probe" && workflowName == "child" {
			return packChildWorkflow(), "/packs/probe", nil
		}
		return nil, "", fmt.Errorf("unexpected pack workflow %s/%s", packName, workflowName)
	})
	state := workflow.NewExecutionContext("run-1", "child")
	state.PackName = "probe"
	state.Status = workflow.StatusFailed
	state.CurrentStep = "run"
	require.NoError(t, mocks.StateStore.Save(context.Background(), state))

	execCtx, err := execSvc.Resume(context.Background(), "run-1", nil, "current")

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, []string{"probe/child"}, loaded)
	assert.Equal(t, "probe", execCtx.PackName, "a resumed run stays tied to its pack")
	assert.Equal(t, originalPaths, configuredPaths)
}

func TestExecutionService_ResumeOfAPackWorkflowNeedsAPackLoader(t *testing.T) {
	execSvc, mocks := NewTestHarness(t).Build()
	state := workflow.NewExecutionContext("run-1", "child")
	state.PackName = "probe"
	state.Status = workflow.StatusFailed
	state.CurrentStep = "run"
	require.NoError(t, mocks.StateStore.Save(context.Background(), state))

	execCtx, err := execSvc.Resume(context.Background(), "run-1", nil, "current")

	require.Error(t, err)
	assert.Nil(t, execCtx)
	assert.Equal(t, "load workflow: workflow probe/child comes from a pack, and no pack workflow loader is configured", err.Error())
}
