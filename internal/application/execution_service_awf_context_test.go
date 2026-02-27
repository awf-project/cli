package application_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/xdg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: F063 - Prompt File Loading for Agent Steps
// Component: T012 - Populate .awf context in buildInterpolationContext()
//
// Tests verify that buildInterpolationContext() populates the AWF map with XDG directory paths.
// These paths enable template interpolation like {{.awf.prompts_dir}}/template.md in workflows.

func TestExecutionService_buildInterpolationContext_PopulatesAWFPaths(t *testing.T) {
	promptsDir := xdg.AWFPromptsDir()

	wf := &workflow.Workflow{
		Name:    "test-awf-context",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.awf.prompts_dir}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-awf-context", wf).
		WithCommandResult("echo "+promptsDir, &ports.CommandResult{
			Stdout:   promptsDir,
			ExitCode: 0,
		}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test-awf-context", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	state, ok := ctx.GetStepState("start")
	require.True(t, ok)
	assert.Contains(t, state.Output, promptsDir, "AWF prompts_dir should be interpolated")
}

func TestExecutionService_buildInterpolationContext_AWFConfigDir(t *testing.T) {
	configDir := xdg.AWFConfigDir()

	wf := &workflow.Workflow{
		Name:    "test-config-dir",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.awf.config_dir}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-config-dir", wf).
		WithCommandResult("echo "+configDir, &ports.CommandResult{
			Stdout:   configDir,
			ExitCode: 0,
		}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test-config-dir", nil)

	require.NoError(t, err)
	state, ok := ctx.GetStepState("start")
	require.True(t, ok)
	assert.Contains(t, state.Output, configDir, "AWF config_dir should be interpolated")
}

func TestExecutionService_buildInterpolationContext_AWFDataDir(t *testing.T) {
	dataDir := xdg.AWFDataDir()

	wf := &workflow.Workflow{
		Name:    "test-data-dir",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.awf.data_dir}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-data-dir", wf).
		WithCommandResult("echo "+dataDir, &ports.CommandResult{
			Stdout:   dataDir,
			ExitCode: 0,
		}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test-data-dir", nil)

	require.NoError(t, err)
	state, ok := ctx.GetStepState("start")
	require.True(t, ok)
	assert.Contains(t, state.Output, dataDir, "AWF data_dir should be interpolated")
}

func TestExecutionService_buildInterpolationContext_AWFWorkflowsDir(t *testing.T) {
	workflowsDir := xdg.AWFWorkflowsDir()

	wf := &workflow.Workflow{
		Name:    "test-workflows-dir",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.awf.workflows_dir}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-workflows-dir", wf).
		WithCommandResult("echo "+workflowsDir, &ports.CommandResult{
			Stdout:   workflowsDir,
			ExitCode: 0,
		}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test-workflows-dir", nil)

	require.NoError(t, err)
	state, ok := ctx.GetStepState("start")
	require.True(t, ok)
	assert.Contains(t, state.Output, workflowsDir, "AWF workflows_dir should be interpolated")
}

func TestExecutionService_buildInterpolationContext_AWFPluginsDir(t *testing.T) {
	pluginsDir := xdg.AWFPluginsDir()

	wf := &workflow.Workflow{
		Name:    "test-plugins-dir",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.awf.plugins_dir}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-plugins-dir", wf).
		WithCommandResult("echo "+pluginsDir, &ports.CommandResult{
			Stdout:   pluginsDir,
			ExitCode: 0,
		}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test-plugins-dir", nil)

	require.NoError(t, err)
	state, ok := ctx.GetStepState("start")
	require.True(t, ok)
	assert.Contains(t, state.Output, pluginsDir, "AWF plugins_dir should be interpolated")
}

func TestExecutionService_buildInterpolationContext_AllAWFPathsAvailable(t *testing.T) {
	promptsDir := xdg.AWFPromptsDir()
	configDir := xdg.AWFConfigDir()
	dataDir := xdg.AWFDataDir()
	workflowsDir := xdg.AWFWorkflowsDir()
	pluginsDir := xdg.AWFPluginsDir()

	wf := &workflow.Workflow{
		Name:    "test-all-paths",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.awf.prompts_dir}} {{.awf.config_dir}} {{.awf.data_dir}} {{.awf.workflows_dir}} {{.awf.plugins_dir}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	expectedOutput := promptsDir + " " + configDir + " " + dataDir + " " + workflowsDir + " " + pluginsDir

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-all-paths", wf).
		WithCommandResult("echo "+expectedOutput, &ports.CommandResult{
			Stdout:   expectedOutput,
			ExitCode: 0,
		}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test-all-paths", nil)

	require.NoError(t, err)
	state, ok := ctx.GetStepState("start")
	require.True(t, ok)

	assert.Contains(t, state.Output, promptsDir)
	assert.Contains(t, state.Output, configDir)
	assert.Contains(t, state.Output, dataDir)
	assert.Contains(t, state.Output, workflowsDir)
	assert.Contains(t, state.Output, pluginsDir)
}

func TestExecutionService_buildInterpolationContext_XDGOverride(t *testing.T) {
	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalDataHome := os.Getenv("XDG_DATA_HOME")
	t.Cleanup(func() {
		if originalConfigHome != "" {
			os.Setenv("XDG_CONFIG_HOME", originalConfigHome)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
		if originalDataHome != "" {
			os.Setenv("XDG_DATA_HOME", originalDataHome)
		} else {
			os.Unsetenv("XDG_DATA_HOME")
		}
	})

	tmpDir := t.TempDir()
	customConfigHome := filepath.Join(tmpDir, "custom-config")
	customDataHome := filepath.Join(tmpDir, "custom-data")

	os.Setenv("XDG_CONFIG_HOME", customConfigHome)
	os.Setenv("XDG_DATA_HOME", customDataHome)

	expectedConfigDir := filepath.Join(customConfigHome, "awf")

	wf := &workflow.Workflow{
		Name:    "test-xdg-override",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.awf.config_dir}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-xdg-override", wf).
		WithCommandResult("echo "+expectedConfigDir, &ports.CommandResult{
			Stdout:   expectedConfigDir,
			ExitCode: 0,
		}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test-xdg-override", nil)

	require.NoError(t, err)
	state, ok := ctx.GetStepState("start")
	require.True(t, ok)
	assert.Contains(t, state.Output, expectedConfigDir, "AWF paths should respect XDG_CONFIG_HOME override")
}

func TestExecutionService_buildInterpolationContext_AWFContextInMultipleSteps(t *testing.T) {
	promptsDir := xdg.AWFPromptsDir()
	configDir := xdg.AWFConfigDir()

	wf := &workflow.Workflow{
		Name:    "test-multi-step-awf",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.awf.prompts_dir}}",
				OnSuccess: "step2",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.awf.config_dir}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-multi-step-awf", wf).
		WithCommandResult("echo "+promptsDir, &ports.CommandResult{
			Stdout:   promptsDir,
			ExitCode: 0,
		}).
		WithCommandResult("echo "+configDir, &ports.CommandResult{
			Stdout:   configDir,
			ExitCode: 0,
		}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test-multi-step-awf", nil)

	require.NoError(t, err)

	step1State, ok := ctx.GetStepState("step1")
	require.True(t, ok)
	assert.Contains(t, step1State.Output, promptsDir)

	step2State, ok := ctx.GetStepState("step2")
	require.True(t, ok)
	assert.Contains(t, step2State.Output, configDir)
}

func TestExecutionService_buildInterpolationContext_AWFContextWithStateReference(t *testing.T) {
	promptsDir := xdg.AWFPromptsDir()
	expectedPath := filepath.Join(promptsDir, "template.md")

	wf := &workflow.Workflow{
		Name:    "test-awf-with-state",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo template.md",
				OnSuccess: "step2",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{.awf.prompts_dir}}/{{.states.step1.Output}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-awf-with-state", wf).
		WithCommandResult("echo template.md", &ports.CommandResult{
			Stdout:   "template.md",
			ExitCode: 0,
		}).
		WithCommandResult("echo "+expectedPath, &ports.CommandResult{
			Stdout:   expectedPath,
			ExitCode: 0,
		}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test-awf-with-state", nil)

	require.NoError(t, err)

	step2State, ok := ctx.GetStepState("step2")
	require.True(t, ok)
	assert.Contains(t, step2State.Output, expectedPath, "AWF path should combine with state output")
}

func TestExecutionService_buildInterpolationContext_EmptyAWFMapInitialized(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "test-empty-awf",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("test-empty-awf", wf).
		WithCommandResult("echo test", &ports.CommandResult{
			Stdout:   "test",
			ExitCode: 0,
		}).
		Build()

	ctx, err := execSvc.Run(context.Background(), "test-empty-awf", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}
