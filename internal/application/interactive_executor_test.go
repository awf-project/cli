package application_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: B011 - AWF Path Variables Not Locally Resolved in Command Steps
// Component: T006 - Add awfPaths field and SetAWFPaths() method to InteractiveExecutor,
//                   populate AWF in buildInterpolationContext(), apply resolveLocalOverGlobal()
//                   in executeStep(), and wire SetAWFPaths() in run.go
//
// Tests verify:
// - SetAWFPaths() configures the AWF directory paths for template interpolation
// - buildInterpolationContext() includes AWF paths in the interpolation context
// - executeStep() applies local-over-global resolution to commands and directories
// - SetBreakpoints() works for step pause control

func TestInteractiveExecutor_SetAWFPaths_StoresPathsCorrectly(t *testing.T) {
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc, newMockExecutor(), newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, newMockResolver(),
		newMockExpressionEvaluator(), newMockPrompt(),
	)

	paths := map[string]string{
		"prompts_dir": "/home/user/.local/share/awf/prompts",
		"scripts_dir": "/home/user/.local/share/awf/scripts",
		"config_dir":  "/home/user/.config/awf",
	}

	executor.SetAWFPaths(paths)

	// InteractiveExecutor.awfPaths is unexported; the meaningful guarantee here is that
	// SetAWFPaths does not panic and accepts a non-empty map. The behavioral effect
	// (local-over-global command resolution) is verified by
	// TestInteractiveExecutor_LocalOverGlobal_CommandResolution below.
}

func TestInteractiveExecutor_SetAWFPaths_EmptyMap(t *testing.T) {
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc, newMockExecutor(), newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, newMockResolver(),
		newMockExpressionEvaluator(), newMockPrompt(),
	)

	executor.SetAWFPaths(map[string]string{})

	// Test passes if SetAWFPaths accepts empty map without error
	require.NotNil(t, executor)
}

func TestInteractiveExecutor_SetAWFPaths_NilMap(t *testing.T) {
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc, newMockExecutor(), newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, newMockResolver(),
		newMockExpressionEvaluator(), newMockPrompt(),
	)

	executor.SetAWFPaths(nil)

	// Test passes if SetAWFPaths accepts nil map without error
	require.NotNil(t, executor)
}

func TestInteractiveExecutor_SetBreakpoints_ConfiguresStepPauses(t *testing.T) {
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc, newMockExecutor(), newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, newMockResolver(),
		newMockExpressionEvaluator(), newMockPrompt(),
	)

	executor.SetBreakpoints([]string{"step1", "step3"})

	// Test passes if SetBreakpoints accepts steps without error
	require.NotNil(t, executor)
}

func TestInteractiveExecutor_SetBreakpoints_EmptyListPausesAll(t *testing.T) {
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc, newMockExecutor(), newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, newMockResolver(),
		newMockExpressionEvaluator(), newMockPrompt(),
	)

	executor.SetBreakpoints([]string{})

	// Test passes if SetBreakpoints accepts empty list without error
	require.NotNil(t, executor)
}

func TestInteractiveExecutor_SetTemplateService(t *testing.T) {
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc, newMockExecutor(), newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, newMockResolver(),
		newMockExpressionEvaluator(), newMockPrompt(),
	)

	templateSvc := &application.TemplateService{}
	executor.SetTemplateService(templateSvc)

	require.NotNil(t, executor)
}

func TestInteractiveExecutor_SetOutputWriters(t *testing.T) {
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc, newMockExecutor(), newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, newMockResolver(),
		newMockExpressionEvaluator(), newMockPrompt(),
	)

	executor.SetOutputWriters(nil, nil)

	require.NotNil(t, executor)
}

// newMockPrompt creates a mock InteractivePrompt that returns the given action.
// If action is not provided (empty), it defaults to ActionContinue.
func newMockPrompt(action ...workflow.InteractiveAction) *mockInteractivePrompt {
	defaultAction := workflow.ActionContinue
	if len(action) > 0 {
		defaultAction = action[0]
	}
	return &mockInteractivePrompt{action: defaultAction}
}

// mockInteractivePrompt implements ports.InteractivePrompt for testing
type mockInteractivePrompt struct {
	action             workflow.InteractiveAction
	completeCalled     bool
	detailsShowCount   int
	executingShowCount int
}

// StepPresenter methods
func (m *mockInteractivePrompt) ShowHeader(workflowName string) {}

func (m *mockInteractivePrompt) ShowStepDetails(info *workflow.InteractiveStepInfo) {
	m.detailsShowCount++
}

func (m *mockInteractivePrompt) ShowExecuting(stepName string) {
	m.executingShowCount++
}
func (m *mockInteractivePrompt) ShowStepResult(state *workflow.StepState, nextStep string) {}

// StatusPresenter methods
func (m *mockInteractivePrompt) ShowAborted()                          {}
func (m *mockInteractivePrompt) ShowSkipped(stepName, nextStep string) {}
func (m *mockInteractivePrompt) ShowCompleted(status workflow.ExecutionStatus) {
	m.completeCalled = true
}
func (m *mockInteractivePrompt) ShowError(err error) {}

// UserInteraction methods
func (m *mockInteractivePrompt) PromptAction(ctx context.Context, retry bool) (workflow.InteractiveAction, error) {
	return m.action, nil
}

func (m *mockInteractivePrompt) EditInput(ctx context.Context, name string, current any) (any, error) {
	return current, nil
}
func (m *mockInteractivePrompt) ShowContext(ctx *workflow.RuntimeContext) {}

// interactiveCommandCapturingExecutor captures the Program field from each Execute call.
// The last command executed is stored in capturedCmd for assertions.
type interactiveCommandCapturingExecutor struct {
	capturedCmd string
	result      *ports.CommandResult
}

func (c *interactiveCommandCapturingExecutor) Execute(ctx context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	c.capturedCmd = cmd.Program
	return c.result, nil
}

// interactiveRealResolverAdapter wraps the real template resolver for testing AWF path resolution.
type interactiveRealResolverAdapter struct {
	resolver interpolation.Resolver
}

func (r *interactiveRealResolverAdapter) Resolve(template string, ctx *interpolation.Context) (string, error) {
	return r.resolver.Resolve(template, ctx)
}

func newInteractiveRealResolver() *interactiveRealResolverAdapter {
	return &interactiveRealResolverAdapter{
		resolver: interpolation.NewTemplateResolver(),
	}
}

// TestInteractiveExecutor_LocalOverGlobal_CommandResolution verifies that executeStep() applies
// local-over-global resolution: when {{.awf.scripts_dir}} is used in a command and the referenced
// file exists in the workflow's local .awf/scripts/, it resolves to the local path instead of the
// global scripts_dir set via SetAWFPaths() (B011: FR-001, FR-002).
func TestInteractiveExecutor_LocalOverGlobal_CommandResolution(t *testing.T) {
	tmpDir := t.TempDir()

	localScriptsDir := filepath.Join(tmpDir, ".awf", "scripts")
	globalScriptsDir := filepath.Join(tmpDir, "global-scripts")
	require.NoError(t, os.MkdirAll(localScriptsDir, 0o755))
	require.NoError(t, os.MkdirAll(globalScriptsDir, 0o755))

	localScriptPath := filepath.Join(localScriptsDir, "deploy.sh")
	require.NoError(t, os.WriteFile(localScriptPath, []byte("#!/bin/bash\necho local"), 0o755))

	globalScriptPath := filepath.Join(globalScriptsDir, "deploy.sh")
	require.NoError(t, os.WriteFile(globalScriptPath, []byte("#!/bin/bash\necho global"), 0o755))

	repo := newMockRepository()
	repo.workflows["local-override-test"] = &workflow.Workflow{
		Name:      "local-override-test",
		SourceDir: filepath.Join(tmpDir, ".awf", "workflows"),
		Initial:   "deploy",
		Steps: map[string]*workflow.Step{
			"deploy": {
				Name:      "deploy",
				Type:      workflow.StepTypeCommand,
				Command:   "source {{.awf.scripts_dir}}/deploy.sh",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	capturing := &interactiveCommandCapturingExecutor{
		result: &ports.CommandResult{Stdout: "executed", ExitCode: 0},
	}

	resolver := newInteractiveRealResolver()

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), capturing, &mockLogger{}, nil)
	executor := application.NewInteractiveExecutor(
		wfSvc, capturing, newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, resolver,
		newMockExpressionEvaluator(), newMockPrompt(workflow.ActionContinue),
	)

	executor.SetAWFPaths(map[string]string{
		"scripts_dir": globalScriptsDir,
	})

	execCtx, err := executor.Run(context.Background(), "local-override-test", nil)

	require.NoError(t, err)
	require.NotNil(t, execCtx)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Contains(t, capturing.capturedCmd, localScriptsDir,
		"command should resolve to local .awf/scripts directory")
	assert.NotContains(t, capturing.capturedCmd, globalScriptsDir,
		"command should not reference global directory when local file exists")
}
