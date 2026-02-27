package application_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDryRunExecutor_BuildStepPlan_ScriptFile_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "deploy.sh")
	scriptContent := "echo 'Deploying to production'\nkubectl apply -f deployment.yaml"
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0o644))

	wf := &workflow.Workflow{
		Name:      "deploy-workflow",
		SourceDir: tmpDir,
		Initial:   "deploy",
		Steps: map[string]*workflow.Step{
			"deploy": {
				Name:       "deploy",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "deploy.sh",
				OnSuccess:  "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	repo := newMockRepository()
	repo.workflows["deploy-workflow"] = wf
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})
	executor.SetAWFPaths(map[string]string{
		"config_dir":  filepath.Join(tmpDir, ".config/awf"),
		"scripts_dir": filepath.Join(tmpDir, ".config/awf/scripts"),
	})

	plan, err := executor.Execute(context.Background(), "deploy-workflow", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Len(t, plan.Steps, 2)

	deployStep := plan.Steps[0]
	assert.Equal(t, "deploy", deployStep.Name)
	assert.Equal(t, scriptContent, deployStep.Command)
	assert.Equal(t, "deploy.sh", deployStep.ScriptFile)
}

func TestDryRunExecutor_BuildStepPlan_ScriptFile_WithPathInterpolation(t *testing.T) {
	tmpDir := t.TempDir()
	scriptsDir := filepath.Join(tmpDir, ".config/awf/scripts")
	require.NoError(t, os.MkdirAll(scriptsDir, 0o755))

	scriptPath := filepath.Join(scriptsDir, "build.sh")
	scriptContent := "make build\nmake test"
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0o644))

	wf := &workflow.Workflow{
		Name:      "build-workflow",
		SourceDir: tmpDir,
		Initial:   "build",
		Steps: map[string]*workflow.Step{
			"build": {
				Name:       "build",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "{{.awf.scripts_dir}}/build.sh",
				OnSuccess:  "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	repo := newMockRepository()
	repo.workflows["build-workflow"] = wf
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})
	executor.SetAWFPaths(map[string]string{
		"config_dir":  filepath.Join(tmpDir, ".config/awf"),
		"scripts_dir": scriptsDir,
	})

	plan, err := executor.Execute(context.Background(), "build-workflow", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Len(t, plan.Steps, 2)

	buildStep := plan.Steps[0]
	assert.Equal(t, "build", buildStep.Name)
	assert.Equal(t, scriptContent, buildStep.Command)
	assert.Contains(t, buildStep.ScriptFile, "build.sh")
}

func TestDryRunExecutor_BuildStepPlan_ScriptFile_WithContentInterpolation(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "greet.sh")
	scriptContent := "echo 'Hello {{.inputs.name}}'\necho 'Environment: {{.inputs.env}}'"
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0o644))

	wf := &workflow.Workflow{
		Name:      "greet-workflow",
		SourceDir: tmpDir,
		Initial:   "greet",
		Inputs: []workflow.Input{
			{Name: "name", Required: true},
			{Name: "env", Required: true},
		},
		Steps: map[string]*workflow.Step{
			"greet": {
				Name:       "greet",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "greet.sh",
				OnSuccess:  "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	repo := newMockRepository()
	repo.workflows["greet-workflow"] = wf
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})
	executor.SetAWFPaths(map[string]string{
		"config_dir":  filepath.Join(tmpDir, ".config/awf"),
		"scripts_dir": filepath.Join(tmpDir, ".config/awf/scripts"),
	})

	inputs := map[string]any{
		"name": "world",
		"env":  "production",
	}
	plan, err := executor.Execute(context.Background(), "greet-workflow", inputs)

	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Len(t, plan.Steps, 2)

	greetStep := plan.Steps[0]
	assert.Equal(t, "greet", greetStep.Name)
	assert.Contains(t, greetStep.Command, "Hello world")
	assert.Contains(t, greetStep.Command, "Environment: production")
	assert.Equal(t, "greet.sh", greetStep.ScriptFile)
}

func TestDryRunExecutor_BuildStepPlan_ScriptFile_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	wf := &workflow.Workflow{
		Name:      "missing-script",
		SourceDir: tmpDir,
		Initial:   "run",
		Steps: map[string]*workflow.Step{
			"run": {
				Name:       "run",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "missing.sh",
				OnSuccess:  "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	repo := newMockRepository()
	repo.workflows["missing-script"] = wf
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})
	executor.SetAWFPaths(map[string]string{
		"config_dir":  filepath.Join(tmpDir, ".config/awf"),
		"scripts_dir": filepath.Join(tmpDir, ".config/awf/scripts"),
	})

	plan, err := executor.Execute(context.Background(), "missing-script", nil)

	require.Error(t, err)
	assert.Nil(t, plan)
	assert.Contains(t, err.Error(), "missing.sh")
}

func TestDryRunExecutor_BuildStepPlan_ScriptFile_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "absolute.sh")
	scriptContent := "/usr/local/bin/deploy"
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0o644))

	wf := &workflow.Workflow{
		Name:      "absolute-path",
		SourceDir: "/some/other/dir",
		Initial:   "deploy",
		Steps: map[string]*workflow.Step{
			"deploy": {
				Name:       "deploy",
				Type:       workflow.StepTypeCommand,
				ScriptFile: scriptPath,
				OnSuccess:  "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	repo := newMockRepository()
	repo.workflows["absolute-path"] = wf
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})
	executor.SetAWFPaths(map[string]string{
		"config_dir":  filepath.Join(tmpDir, ".config/awf"),
		"scripts_dir": filepath.Join(tmpDir, ".config/awf/scripts"),
	})

	plan, err := executor.Execute(context.Background(), "absolute-path", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Len(t, plan.Steps, 2)

	deployStep := plan.Steps[0]
	assert.Equal(t, "deploy", deployStep.Name)
	assert.Equal(t, scriptContent, deployStep.Command)
	assert.Equal(t, scriptPath, deployStep.ScriptFile)
}

func TestDryRunExecutor_BuildStepPlan_ScriptFile_NoCommandField(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.sh")
	scriptContent := "npm test"
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0o644))

	wf := &workflow.Workflow{
		Name:      "test-workflow",
		SourceDir: tmpDir,
		Initial:   "test",
		Steps: map[string]*workflow.Step{
			"test": {
				Name:       "test",
				Type:       workflow.StepTypeCommand,
				ScriptFile: "test.sh",
				OnSuccess:  "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	repo := newMockRepository()
	repo.workflows["test-workflow"] = wf
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := mocks.NewMockExpressionEvaluator()
	executor := application.NewDryRunExecutor(wfSvc, resolver, evaluator, &mockLogger{})
	executor.SetAWFPaths(map[string]string{
		"config_dir":  filepath.Join(tmpDir, ".config/awf"),
		"scripts_dir": filepath.Join(tmpDir, ".config/awf/scripts"),
	})

	plan, err := executor.Execute(context.Background(), "test-workflow", nil)

	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Len(t, plan.Steps, 2)

	testStep := plan.Steps[0]
	assert.Equal(t, "test", testStep.Name)
	assert.Equal(t, scriptContent, testStep.Command)
	assert.Equal(t, "test.sh", testStep.ScriptFile)
	assert.NotEmpty(t, testStep.Command)
}
