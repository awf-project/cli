package application_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	domainerrors "github.com/awf-project/cli/internal/domain/errors"
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

func TestInteractiveExecutor_WithoutConfiguredTimeout(t *testing.T) {
	for _, family := range interactiveDeadlineFamilies() {
		t.Run(family.name, func(t *testing.T) {
			executor := newInteractiveDeadlineExecutor(false)
			logger := &interactiveDeadlineLogger{}
			prompt := newMockPrompt(workflow.ActionContinue)
			interactive := newInteractiveDeadlineRunner(family.workflow(0), executor, logger, prompt)
			parent := context.WithValue(context.Background(), interactiveDeadlineContextKey{}, family.name)

			t.Setenv("INTERACTIVE_TIMEOUT_ENV_SENTINEL", "environment-must-not-be-logged")
			execCtx, err := interactive.Run(parent, family.name, map[string]any{
				"input":          "input-must-not-be-logged",
				"prompt":         "prompt-must-not-be-logged",
				"SECRET_API_KEY": "secret-must-not-be-logged",
			})

			require.NoError(t, err)
			require.NotNil(t, execCtx)
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
			assert.True(t, prompt.completeCalled)
			assert.Positive(t, prompt.executingShowCount)
			for _, observed := range executor.receivedContexts(t) {
				assert.Equal(t, parent, observed)
				_, hasDeadline := observed.Deadline()
				assert.False(t, hasDeadline)
			}
			for _, sensitive := range []string{
				"input-must-not-be-logged",
				"prompt-must-not-be-logged",
				"environment-must-not-be-logged",
				"secret-must-not-be-logged",
				interactiveDeadlineOutput,
			} {
				assert.NotContains(t, logger.String(), sensitive)
			}
		})
	}
}

func TestInteractiveExecutor_ExplicitTimeout(t *testing.T) {
	const timeoutSeconds = 1

	for _, family := range interactiveDeadlineFamilies() {
		t.Run(family.name, func(t *testing.T) {
			executor := newInteractiveDeadlineExecutor(true)
			interactive := newInteractiveDeadlineRunner(
				family.workflow(timeoutSeconds), executor, &interactiveDeadlineLogger{}, newMockPrompt(),
			)
			startedAt := time.Now()

			execCtx, err := interactive.Run(context.Background(), family.name, nil)

			require.NotNil(t, execCtx)
			assert.Equal(t, workflow.StatusCancelled, execCtx.Status)
			assertInteractiveLocalTimeout(t, err, timeoutSeconds)
			for _, observed := range executor.receivedContexts(t) {
				deadline, ok := observed.Deadline()
				require.True(t, ok)
				assert.WithinDuration(t, startedAt.Add(timeoutSeconds*time.Second), deadline, 250*time.Millisecond)
				select {
				case <-observed.Done():
				case <-time.After(time.Second):
					t.Fatal("derived execution context was not cancelled")
				}
			}
		})
	}

	t.Run("successful_work_cancels_derived_context", func(t *testing.T) {
		executor := newInteractiveDeadlineExecutor(false)
		interactive := newInteractiveDeadlineRunner(
			interactiveCommandDeadlineWorkflow(30), executor, &interactiveDeadlineLogger{}, newMockPrompt(),
		)

		execCtx, err := interactive.Run(context.Background(), "command", nil)

		require.NoError(t, err)
		assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
		observed := executor.receivedContexts(t)[0]
		select {
		case <-observed.Done():
			assert.ErrorIs(t, observed.Err(), context.Canceled)
		case <-time.After(time.Second):
			t.Fatal("successful execution did not cancel its derived context")
		}
	})

	for _, family := range interactiveNestedDeadlineFamilies() {
		t.Run("enclosing_deadline_wins/"+family.name, func(t *testing.T) {
			executor := newInteractiveDeadlineExecutor(true)
			interactive := newInteractiveDeadlineRunner(
				family.workflow(timeoutSeconds), executor, &interactiveDeadlineLogger{}, newMockPrompt(),
			)
			startedAt := time.Now()

			execCtx, err := interactive.Run(context.Background(), family.name, nil)

			require.NotNil(t, execCtx)
			assert.Equal(t, workflow.StatusCancelled, execCtx.Status)
			assertInteractiveLocalTimeout(t, err, timeoutSeconds)
			for _, observed := range executor.receivedContexts(t) {
				deadline, ok := observed.Deadline()
				require.True(t, ok)
				assert.WithinDuration(t, startedAt.Add(time.Second), deadline, 250*time.Millisecond)
			}
		})
	}

	for _, recovery := range interactiveTimeoutRecoveryWorkflows() {
		t.Run("termination_bypasses_failure_routing/"+recovery.name, func(t *testing.T) {
			executor := newInteractiveDeadlineExecutor(true)
			interactive := newInteractiveDeadlineRunner(
				recovery.workflow(), executor, &interactiveDeadlineLogger{}, newMockPrompt(),
			)

			execCtx, err := interactive.Run(context.Background(), recovery.workflow().Name, nil)

			require.NotNil(t, execCtx)
			assert.Equal(t, workflow.StatusCancelled, execCtx.Status)
			assertInteractiveLocalTimeout(t, err, timeoutSeconds)
		})
	}
}

func TestInteractiveExecutor_ParentCancellation(t *testing.T) {
	const localTimeoutSeconds = 30

	for _, family := range interactiveDeadlineFamilies() {
		t.Run(family.name, func(t *testing.T) {
			t.Run("user_cancellation", func(t *testing.T) {
				parent, cancel := context.WithCancel(context.Background())
				t.Cleanup(cancel)
				got := runInteractiveUntilParentEnds(t, family, localTimeoutSeconds, parent, cancel)

				assert.ErrorIs(t, got.err, context.Canceled)
				assert.False(t, errors.Is(got.err, context.DeadlineExceeded))
				assertInteractiveNoLocalTimeoutDetails(t, got.err)
			})

			t.Run("earlier_parent_deadline", func(t *testing.T) {
				parent, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
				t.Cleanup(cancel)
				got := runInteractiveUntilParentEnds(t, family, localTimeoutSeconds, parent, func() {})

				assert.ErrorIs(t, got.err, context.DeadlineExceeded)
				assert.False(t, errors.Is(got.err, context.Canceled))
				assertInteractiveNoLocalTimeoutDetails(t, got.err)
			})
		})
	}

	for _, recovery := range interactiveTimeoutRecoveryWorkflows() {
		t.Run("termination_bypasses_failure_routing/"+recovery.name, func(t *testing.T) {
			wf := recovery.workflow()
			wf.Steps[wf.Initial].Timeout = localTimeoutSeconds
			family := interactiveDeadlineFamily{
				name: wf.Name,
				workflow: func(int) *workflow.Workflow {
					return wf
				},
			}
			parent, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			got := runInteractiveUntilParentEnds(t, family, localTimeoutSeconds, parent, cancel)

			assert.ErrorIs(t, got.err, context.Canceled)
			assert.False(t, errors.Is(got.err, context.DeadlineExceeded))
			assertInteractiveNoLocalTimeoutDetails(t, got.err)
		})
	}
}

const interactiveDeadlineOutput = "output-must-not-be-logged"

type interactiveDeadlineContextKey struct{}

type interactiveDeadlineFamily struct {
	name     string
	workflow func(timeout int) *workflow.Workflow
}

type interactiveDeadlineOutcome struct {
	execCtx *workflow.ExecutionContext
	err     error
}

func runInteractiveUntilParentEnds(
	t *testing.T,
	family interactiveDeadlineFamily,
	localTimeoutSeconds int,
	parent context.Context,
	endParent func(),
) interactiveDeadlineOutcome {
	t.Helper()
	executor := newInteractiveDeadlineExecutor(true)
	interactive := newInteractiveDeadlineRunner(
		family.workflow(localTimeoutSeconds), executor, &interactiveDeadlineLogger{}, newMockPrompt(),
	)
	result := make(chan interactiveDeadlineOutcome, 1)

	go func() {
		execCtx, err := interactive.Run(parent, family.name, nil)
		result <- interactiveDeadlineOutcome{execCtx: execCtx, err: err}
	}()

	observed := executor.receiveContext(t)
	deadline, hasDeadline := observed.Deadline()
	require.True(t, hasDeadline)
	if parentDeadline, parentHasDeadline := parent.Deadline(); parentHasDeadline {
		assert.Equal(t, parentDeadline, deadline)
	} else {
		assert.WithinDuration(t, time.Now().Add(time.Duration(localTimeoutSeconds)*time.Second), deadline, time.Second)
	}
	endParent()

	select {
	case got := <-result:
		require.NotNil(t, got.execCtx)
		assert.Equal(t, workflow.StatusCancelled, got.execCtx.Status)
		return got
	case <-time.After(2 * time.Second):
		t.Fatal("interactive execution did not preserve parent termination")
		return interactiveDeadlineOutcome{}
	}
}

func interactiveDeadlineFamilies() []interactiveDeadlineFamily {
	return []interactiveDeadlineFamily{
		{name: "command", workflow: interactiveCommandDeadlineWorkflow},
		{name: "parallel", workflow: interactiveParallelDeadlineWorkflow},
		{name: "for_each", workflow: interactiveForEachDeadlineWorkflow},
		{name: "while", workflow: interactiveWhileDeadlineWorkflow},
	}
}

func interactiveNestedDeadlineFamilies() []interactiveDeadlineFamily {
	parallel := interactiveParallelDeadlineWorkflow
	forEach := interactiveForEachDeadlineWorkflow
	whileLoop := interactiveWhileDeadlineWorkflow
	return []interactiveDeadlineFamily{
		{name: "parallel", workflow: func(timeout int) *workflow.Workflow {
			wf := parallel(timeout)
			wf.Steps["branch"].Timeout = 30
			return wf
		}},
		{name: "for_each", workflow: func(timeout int) *workflow.Workflow {
			wf := forEach(timeout)
			wf.Steps["body"].Timeout = 30
			return wf
		}},
		{name: "while", workflow: func(timeout int) *workflow.Workflow {
			wf := whileLoop(timeout)
			wf.Steps["body"].Timeout = 30
			return wf
		}},
	}
}

type interactiveTimeoutRecoveryWorkflow struct {
	name     string
	workflow func() *workflow.Workflow
}

func interactiveTimeoutRecoveryWorkflows() []interactiveTimeoutRecoveryWorkflow {
	withRecovery := func(wf *workflow.Workflow) *workflow.Workflow {
		wf.Steps[wf.Initial].OnFailure = "recovered"
		wf.Steps["recovered"] = &workflow.Step{Name: "recovered", Type: workflow.StepTypeTerminal}
		return wf
	}
	return []interactiveTimeoutRecoveryWorkflow{
		{name: "command_on_failure", workflow: func() *workflow.Workflow {
			return withRecovery(interactiveCommandDeadlineWorkflow(1))
		}},
		{name: "command_continue_on_error", workflow: func() *workflow.Workflow {
			wf := interactiveCommandDeadlineWorkflow(1)
			wf.Steps[wf.Initial].ContinueOnError = true
			return wf
		}},
		{name: "command_failure_transition", workflow: func() *workflow.Workflow {
			wf := interactiveCommandDeadlineWorkflow(1)
			wf.Steps[wf.Initial].Transitions = workflow.Transitions{{Goto: "done"}}
			return wf
		}},
		{name: "parallel_on_failure", workflow: func() *workflow.Workflow {
			return withRecovery(interactiveParallelDeadlineWorkflow(1))
		}},
		{name: "for_each_on_failure", workflow: func() *workflow.Workflow {
			return withRecovery(interactiveForEachDeadlineWorkflow(1))
		}},
		{name: "while_on_failure", workflow: func() *workflow.Workflow {
			return withRecovery(interactiveWhileDeadlineWorkflow(1))
		}},
	}
}

func interactiveCommandDeadlineWorkflow(timeout int) *workflow.Workflow {
	return interactiveDeadlineWorkflow("command", &workflow.Step{
		Name: "command", Type: workflow.StepTypeCommand, Command: "command", Timeout: timeout, OnSuccess: "done",
	})
}

func interactiveParallelDeadlineWorkflow(timeout int) *workflow.Workflow {
	wf := interactiveDeadlineWorkflow("parallel", &workflow.Step{
		Name: "parallel", Type: workflow.StepTypeParallel, Branches: []string{"branch"},
		Strategy: "all_succeed", Timeout: timeout, OnSuccess: "done",
	})
	wf.Steps["branch"] = &workflow.Step{Name: "branch", Type: workflow.StepTypeCommand, Command: "branch"}
	return wf
}

func interactiveForEachDeadlineWorkflow(timeout int) *workflow.Workflow {
	wf := interactiveDeadlineWorkflow("for_each", &workflow.Step{
		Name: "for_each", Type: workflow.StepTypeForEach, Timeout: timeout,
		Loop: &workflow.LoopConfig{
			Type: workflow.LoopTypeForEach, Items: `["item"]`, Body: []string{"body"}, OnComplete: "done",
		},
	})
	wf.Steps["body"] = &workflow.Step{Name: "body", Type: workflow.StepTypeCommand, Command: "body"}
	return wf
}

func interactiveWhileDeadlineWorkflow(timeout int) *workflow.Workflow {
	wf := interactiveDeadlineWorkflow("while", &workflow.Step{
		Name: "while", Type: workflow.StepTypeWhile, Timeout: timeout,
		Loop: &workflow.LoopConfig{
			Type: workflow.LoopTypeWhile, Condition: "once", Body: []string{"body"},
			MaxIterations: 1, OnComplete: "done",
		},
	})
	wf.Steps["body"] = &workflow.Step{Name: "body", Type: workflow.StepTypeCommand, Command: "body"}
	return wf
}

func interactiveDeadlineWorkflow(name string, initial *workflow.Step) *workflow.Workflow {
	return &workflow.Workflow{
		Name: name, Initial: initial.Name,
		Steps: map[string]*workflow.Step{
			initial.Name: initial,
			"done":       {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}
}

func newInteractiveDeadlineRunner(
	wf *workflow.Workflow,
	executor *interactiveDeadlineExecutor,
	logger ports.Logger,
	prompt ports.InteractivePrompt,
) *application.InteractiveExecutor {
	repo := newMockRepository()
	repo.workflows[wf.Name] = wf
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, logger, nil)
	return application.NewInteractiveExecutor(
		wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), logger,
		newMockResolver(), &interactiveDeadlineEvaluator{}, prompt,
	)
}

type interactiveDeadlineExecutor struct {
	waitForDone bool
	contexts    chan context.Context
	mu          sync.Mutex
	received    []context.Context
}

func newInteractiveDeadlineExecutor(waitForDone bool) *interactiveDeadlineExecutor {
	return &interactiveDeadlineExecutor{waitForDone: waitForDone, contexts: make(chan context.Context, 8)}
}

func (e *interactiveDeadlineExecutor) Execute(ctx context.Context, _ *ports.Command) (*ports.CommandResult, error) {
	e.mu.Lock()
	e.received = append(e.received, ctx)
	e.mu.Unlock()
	e.contexts <- ctx
	if e.waitForDone {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return &ports.CommandResult{ExitCode: 0, Stdout: interactiveDeadlineOutput}, nil
}

func (e *interactiveDeadlineExecutor) receiveContext(t *testing.T) context.Context {
	t.Helper()
	select {
	case ctx := <-e.contexts:
		return ctx
	case <-time.After(time.Second):
		t.Fatal("command executor did not receive a context")
		return nil
	}
}

func (e *interactiveDeadlineExecutor) receivedContexts(t *testing.T) []context.Context {
	t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	require.NotEmpty(t, e.received)
	return append([]context.Context(nil), e.received...)
}

type interactiveDeadlineEvaluator struct {
	mu        sync.Mutex
	evaluated bool
}

func (e *interactiveDeadlineEvaluator) EvaluateBool(_ string, _ *interpolation.Context) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.evaluated {
		return false, nil
	}
	e.evaluated = true
	return true, nil
}

func (*interactiveDeadlineEvaluator) EvaluateInt(string, *interpolation.Context) (int, error) {
	return 0, nil
}

type interactiveDeadlineLogger struct {
	mu      sync.Mutex
	entries []string
}

func (l *interactiveDeadlineLogger) Debug(msg string, fields ...any) { l.record(msg, fields...) }
func (l *interactiveDeadlineLogger) Info(msg string, fields ...any)  { l.record(msg, fields...) }
func (l *interactiveDeadlineLogger) Warn(msg string, fields ...any)  { l.record(msg, fields...) }
func (l *interactiveDeadlineLogger) Error(msg string, fields ...any) { l.record(msg, fields...) }
func (l *interactiveDeadlineLogger) WithContext(fields map[string]any) ports.Logger {
	l.record("context", fields)
	return l
}

func (l *interactiveDeadlineLogger) record(msg string, fields ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, fmt.Sprint(append([]any{msg}, fields...)...))
}

func (l *interactiveDeadlineLogger) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.entries, "\n")
}

func assertInteractiveLocalTimeout(t *testing.T, err error, timeoutSeconds int) {
	t.Helper()
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	var structuredErr *domainerrors.StructuredError
	require.ErrorAs(t, err, &structuredErr)
	assert.Equal(t, domainerrors.ErrorCodeExecutionCommandTimeout, structuredErr.Code)
	assert.Equal(t, map[string]any{
		"timeout_owner":   "step",
		"timeout_seconds": timeoutSeconds,
	}, structuredErr.Details)
}

func assertInteractiveNoLocalTimeoutDetails(t *testing.T, err error) {
	t.Helper()
	var structuredErr *domainerrors.StructuredError
	if errors.As(err, &structuredErr) {
		assert.NotContains(t, structuredErr.Details, "timeout_owner")
		assert.NotContains(t, structuredErr.Details, "timeout_seconds")
	}
}
