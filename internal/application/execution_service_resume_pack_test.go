package application_test

import (
	"context"
	"fmt"
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

func TestExecutionService_ResumeLoadsTheWorkflowFromItsPack(t *testing.T) {
	execSvc, mocks := NewTestHarness(t).Build()
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
}

func TestExecutionService_ResumeOfAPackWorkflowNeedsAPackLoader(t *testing.T) {
	execSvc, mocks := NewTestHarness(t).Build()
	state := workflow.NewExecutionContext("run-1", "child")
	state.PackName = "probe"
	state.Status = workflow.StatusFailed
	state.CurrentStep = "run"
	require.NoError(t, mocks.StateStore.Save(context.Background(), state))

	_, err := execSvc.Resume(context.Background(), "run-1", nil, "current")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "probe/child comes from a pack")
}
