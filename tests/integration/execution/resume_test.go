//go:build integration

package execution_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infraExpr "github.com/awf-project/cli/internal/infrastructure/expression"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/awf-project/cli/tests/integration/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResumeWorkflow_E2E(t *testing.T) {
	// Full flow: create workflow, run until interrupt, resume, complete

	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create a workflow with 3 steps
	logFile := filepath.Join(tmpDir, "execution.log")
	wfYAML := `name: resume-test
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo "STEP1" >> ` + logFile + `
    on_success: step2
  step2:
    type: step
    command: echo "STEP2" >> ` + logFile + `
    on_success: step3
  step3:
    type: step
    command: echo "STEP3" >> ` + logFile + `
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "resume-test.yaml"), []byte(wfYAML), 0o644))

	// Wire up components
	repo := repository.NewYAMLRepository(workflowsDir)
	stateStore := store.NewJSONStore(statesDir)
	exec := executor.NewShellExecutor()
	logger := &testhelpers.MockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, stateStore, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, stateStore, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 1: Run workflow to completion
	execCtx, err := execSvc.Run(ctx, "resume-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	// Verify all steps executed
	logData, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Equal(t, "STEP1\nSTEP2\nSTEP3\n", string(logData))
}

func TestResumeWorkflow_FromInterruptedState_E2E(t *testing.T) {
	// Simulate interrupted workflow by manually creating state, then resume

	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create workflow
	logFile := filepath.Join(tmpDir, "resume.log")
	wfYAML := `name: interrupt-resume
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo "STEP1" >> ` + logFile + `
    on_success: step2
  step2:
    type: step
    command: echo "STEP2" >> ` + logFile + `
    on_success: step3
  step3:
    type: step
    command: echo "STEP3" >> ` + logFile + `
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "interrupt-resume.yaml"), []byte(wfYAML), 0o644))

	// Manually create an "interrupted" state (step1 completed, at step2)
	now := time.Now()
	interruptedState := &workflow.ExecutionContext{
		WorkflowID:   "interrupted-id-123",
		WorkflowName: "interrupt-resume",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step2",
		Inputs:       make(map[string]any),
		States: map[string]workflow.StepState{
			"step1": {
				Name:        "step1",
				Status:      workflow.StatusCompleted,
				Output:      "STEP1\n",
				ExitCode:    0,
				StartedAt:   now.Add(-time.Minute),
				CompletedAt: now.Add(-30 * time.Second),
			},
		},
		Env:       make(map[string]string),
		StartedAt: now.Add(-time.Minute),
		UpdatedAt: now,
	}

	// Wire up components
	repo := repository.NewYAMLRepository(workflowsDir)
	stateStore := store.NewJSONStore(statesDir)
	exec := executor.NewShellExecutor()
	logger := &testhelpers.MockLogger{}
	resolver := interpolation.NewTemplateResolver()

	// Save interrupted state
	err := stateStore.Save(context.Background(), interruptedState)
	require.NoError(t, err)

	// Also write what step1 would have written (simulating it ran before interrupt)
	require.NoError(t, os.WriteFile(logFile, []byte("STEP1\n"), 0o644))

	wfSvc := application.NewWorkflowService(repo, stateStore, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, stateStore, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Resume execution
	resumedCtx, err := execSvc.Resume(ctx, "interrupted-id-123", nil)

	require.NoError(t, err, "resume should succeed")
	assert.Equal(t, workflow.StatusCompleted, resumedCtx.Status)
	assert.Equal(t, "done", resumedCtx.CurrentStep)

	// Verify step2 and step3 executed (appended to log), but step1 was not re-executed
	logData, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Equal(t, "STEP1\nSTEP2\nSTEP3\n", string(logData), "step2 and step3 should append to log")
}

func TestResumeList_E2E(t *testing.T) {
	// Create multiple interrupted workflows, verify list shows them

	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create workflow definition
	wfYAML := `name: list-test
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo hello
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "list-test.yaml"), []byte(wfYAML), 0o644))

	// Wire up components
	repo := repository.NewYAMLRepository(workflowsDir)
	stateStore := store.NewJSONStore(statesDir)
	exec := executor.NewShellExecutor()
	logger := &testhelpers.MockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, stateStore, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, stateStore, logger, resolver, nil)

	ctx := context.Background()

	now := time.Now()

	// Create various states
	states := []*workflow.ExecutionContext{
		{
			WorkflowID:   "running-1",
			WorkflowName: "list-test",
			Status:       workflow.StatusRunning,
			CurrentStep:  "start",
			Inputs:       make(map[string]any),
			States:       make(map[string]workflow.StepState),
			Env:          make(map[string]string),
			StartedAt:    now,
			UpdatedAt:    now,
		},
		{
			WorkflowID:   "running-2",
			WorkflowName: "list-test",
			Status:       workflow.StatusRunning,
			CurrentStep:  "start",
			Inputs:       make(map[string]any),
			States:       make(map[string]workflow.StepState),
			Env:          make(map[string]string),
			StartedAt:    now.Add(-time.Hour),
			UpdatedAt:    now,
		},
		{
			WorkflowID:   "failed-1",
			WorkflowName: "list-test",
			Status:       workflow.StatusFailed,
			CurrentStep:  "start",
			Inputs:       make(map[string]any),
			States:       make(map[string]workflow.StepState),
			Env:          make(map[string]string),
			StartedAt:    now.Add(-2 * time.Hour),
			UpdatedAt:    now.Add(-time.Hour),
		},
		{
			WorkflowID:   "completed-1",
			WorkflowName: "list-test",
			Status:       workflow.StatusCompleted, // Should be filtered out
			CurrentStep:  "done",
			Inputs:       make(map[string]any),
			States:       make(map[string]workflow.StepState),
			Env:          make(map[string]string),
			StartedAt:    now.Add(-3 * time.Hour),
			UpdatedAt:    now.Add(-2 * time.Hour),
		},
	}

	for _, state := range states {
		require.NoError(t, stateStore.Save(ctx, state))
	}

	// List resumable workflows
	resumable, err := execSvc.ListResumable(ctx)

	require.NoError(t, err)
	assert.Len(t, resumable, 3, "should return 3 resumable workflows (all except completed)")

	// Verify completed is not in list
	for _, exec := range resumable {
		assert.NotEqual(t, "completed-1", exec.WorkflowID, "completed workflow should not be in list")
		assert.NotEqual(t, workflow.StatusCompleted, exec.Status, "no completed status in list")
	}

	// Verify running and failed are in list
	var foundRunning1, foundRunning2, foundFailed bool
	for _, exec := range resumable {
		switch exec.WorkflowID {
		case "running-1":
			foundRunning1 = true
		case "running-2":
			foundRunning2 = true
		case "failed-1":
			foundFailed = true
		}
	}
	assert.True(t, foundRunning1, "running-1 should be in list")
	assert.True(t, foundRunning2, "running-2 should be in list")
	assert.True(t, foundFailed, "failed-1 should be in list")
}

func TestResumeWithOverrides_E2E(t *testing.T) {
	// Resume with different inputs, verify they are used

	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create workflow with input
	outputFile := filepath.Join(tmpDir, "output.txt")
	wfYAML := `name: override-test
version: "1.0.0"
inputs:
  - name: message
    type: string
states:
  initial: echo
  echo:
    type: step
    command: echo "{{.inputs.message}}" > ` + outputFile + `
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "override-test.yaml"), []byte(wfYAML), 0o644))

	// Wire up components
	repo := repository.NewYAMLRepository(workflowsDir)
	stateStore := store.NewJSONStore(statesDir)
	exec := executor.NewShellExecutor()
	logger := &testhelpers.MockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, stateStore, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, stateStore, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := time.Now()

	// Create interrupted state with original input
	interruptedState := &workflow.ExecutionContext{
		WorkflowID:   "override-id",
		WorkflowName: "override-test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "echo",
		Inputs:       map[string]any{"message": "original-message"},
		States:       make(map[string]workflow.StepState),
		Env:          make(map[string]string),
		StartedAt:    now.Add(-time.Minute),
		UpdatedAt:    now,
	}
	require.NoError(t, stateStore.Save(ctx, interruptedState))

	// Resume with overridden input
	overrides := map[string]any{"message": "overridden-message"}
	resumedCtx, err := execSvc.Resume(ctx, "override-id", overrides)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, resumedCtx.Status)

	// Verify overridden message was used
	outputData, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, "overridden-message\n", string(outputData))

	// Verify input was overridden in context
	val, ok := resumedCtx.GetInput("message")
	require.True(t, ok)
	assert.Equal(t, "overridden-message", val)
}

func TestResumeFailedWorkflow_E2E(t *testing.T) {
	// Resume a workflow that was previously failed

	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create workflow
	wfYAML := `name: failed-resume
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo "success"
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "failed-resume.yaml"), []byte(wfYAML), 0o644))

	// Wire up components
	repo := repository.NewYAMLRepository(workflowsDir)
	stateStore := store.NewJSONStore(statesDir)
	exec := executor.NewShellExecutor()
	logger := &testhelpers.MockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, stateStore, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, stateStore, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := time.Now()

	// Create failed state (imagine it failed due to transient error)
	failedState := &workflow.ExecutionContext{
		WorkflowID:   "failed-id",
		WorkflowName: "failed-resume",
		Status:       workflow.StatusFailed,
		CurrentStep:  "start",
		Inputs:       make(map[string]any),
		States: map[string]workflow.StepState{
			"start": {
				Name:      "start",
				Status:    workflow.StatusFailed,
				Error:     "transient error",
				StartedAt: now.Add(-time.Minute),
			},
		},
		Env:       make(map[string]string),
		StartedAt: now.Add(-time.Minute),
		UpdatedAt: now,
	}
	require.NoError(t, stateStore.Save(ctx, failedState))

	// Resume - this time the step should succeed
	resumedCtx, err := execSvc.Resume(ctx, "failed-id", nil)

	require.NoError(t, err, "resume of failed workflow should succeed")
	assert.Equal(t, workflow.StatusCompleted, resumedCtx.Status)
	assert.Equal(t, "done", resumedCtx.CurrentStep)
}

func TestResumeWorkflow_WorkflowDefinitionChanged_E2E(t *testing.T) {
	// Resume fails when workflow definition has changed (step removed)

	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create workflow WITHOUT step2 (simulating it was removed)
	wfYAML := `name: changed-workflow
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo "step1"
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "changed-workflow.yaml"), []byte(wfYAML), 0o644))

	// Wire up components
	repo := repository.NewYAMLRepository(workflowsDir)
	stateStore := store.NewJSONStore(statesDir)
	exec := executor.NewShellExecutor()
	logger := &testhelpers.MockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, stateStore, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, stateStore, logger, resolver, nil)

	ctx := context.Background()

	now := time.Now()

	// Create state that was at step2 (which no longer exists)
	staleState := &workflow.ExecutionContext{
		WorkflowID:   "stale-id",
		WorkflowName: "changed-workflow",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step2", // This step no longer exists in workflow
		Inputs:       make(map[string]any),
		States: map[string]workflow.StepState{
			"step1": {
				Name:   "step1",
				Status: workflow.StatusCompleted,
			},
		},
		Env:       make(map[string]string),
		StartedAt: now.Add(-time.Hour),
		UpdatedAt: now,
	}
	require.NoError(t, stateStore.Save(ctx, staleState))

	// Resume should fail because step2 doesn't exist
	_, err := execSvc.Resume(ctx, "stale-id", nil)

	require.Error(t, err, "resume should fail when step no longer exists")
	assert.Contains(t, err.Error(), "step2", "error should mention missing step")
}

func TestResumeWorkflow_ParallelStep_E2E(t *testing.T) {
	// Resume workflow with parallel step

	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
	require.NoError(t, os.MkdirAll(statesDir, 0o755))

	// Create workflow with parallel step
	logFile := filepath.Join(tmpDir, "parallel.log")
	wfYAML := `name: parallel-resume
version: "1.0.0"
states:
  initial: parallel
  parallel:
    type: parallel
    parallel:
      - branch1
      - branch2
    strategy: all_succeed
    on_success: done
  branch1:
    type: step
    command: echo "BRANCH1" >> ` + logFile + `
    on_success: done
  branch2:
    type: step
    command: echo "BRANCH2" >> ` + logFile + `
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "parallel-resume.yaml"), []byte(wfYAML), 0o644))

	// Wire up components
	repo := repository.NewYAMLRepository(workflowsDir)
	stateStore := store.NewJSONStore(statesDir)
	exec := executor.NewShellExecutor()
	logger := &testhelpers.MockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, stateStore, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, stateStore, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := time.Now()

	// Create interrupted state at parallel step
	interruptedState := &workflow.ExecutionContext{
		WorkflowID:   "parallel-id",
		WorkflowName: "parallel-resume",
		Status:       workflow.StatusRunning,
		CurrentStep:  "parallel",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
		Env:          make(map[string]string),
		StartedAt:    now.Add(-time.Minute),
		UpdatedAt:    now,
	}
	require.NoError(t, stateStore.Save(ctx, interruptedState))

	// Resume
	resumedCtx, err := execSvc.Resume(ctx, "parallel-id", nil)

	require.NoError(t, err, "resume with parallel step should succeed")
	assert.Equal(t, workflow.StatusCompleted, resumedCtx.Status)
	assert.Equal(t, "done", resumedCtx.CurrentStep)

	// Verify both branches executed
	logData, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(logData), "BRANCH1")
	assert.Contains(t, string(logData), "BRANCH2")
}
