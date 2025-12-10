package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

// ExecutionService orchestrates workflow execution.
type ExecutionService struct {
	workflowSvc  *WorkflowService
	executor     ports.CommandExecutor
	store        ports.StateStore
	logger       ports.Logger
	resolver     interpolation.Resolver
	hookExecutor *HookExecutor
	stdoutWriter io.Writer
	stderrWriter io.Writer
}

// SetOutputWriters configures streaming output writers.
func (s *ExecutionService) SetOutputWriters(stdout, stderr io.Writer) {
	s.stdoutWriter = stdout
	s.stderrWriter = stderr
}

// NewExecutionService creates a new execution service.
func NewExecutionService(
	wfSvc *WorkflowService,
	executor ports.CommandExecutor,
	store ports.StateStore,
	logger ports.Logger,
	resolver interpolation.Resolver,
) *ExecutionService {
	return &ExecutionService{
		workflowSvc:  wfSvc,
		executor:     executor,
		store:        store,
		logger:       logger,
		resolver:     resolver,
		hookExecutor: NewHookExecutor(executor, logger, resolver),
	}
}

// Run executes a workflow by name with the given inputs.
func (s *ExecutionService) Run(
	ctx context.Context,
	workflowName string,
	inputs map[string]any,
) (*workflow.ExecutionContext, error) {
	// load workflow
	wf, err := s.workflowSvc.GetWorkflow(ctx, workflowName)
	if err != nil {
		return nil, fmt.Errorf("load workflow: %w", err)
	}
	if wf == nil {
		return nil, fmt.Errorf("workflow not found: %s", workflowName)
	}

	// initialize execution context
	execCtx := workflow.NewExecutionContext(uuid.New().String(), wf.Name)
	execCtx.Status = workflow.StatusRunning

	// Apply default values for inputs not provided
	for _, inp := range wf.Inputs {
		if _, provided := inputs[inp.Name]; !provided && inp.Default != nil {
			execCtx.SetInput(inp.Name, inp.Default)
		}
	}
	// Then apply user-provided inputs (overriding defaults)
	for k, v := range inputs {
		execCtx.SetInput(k, v)
	}

	s.logger.Info("starting workflow", "workflow", wf.Name, "id", execCtx.WorkflowID)

	// execute workflow_start hooks
	intCtx := s.buildInterpolationContext(execCtx)
	if err := s.hookExecutor.ExecuteHooks(ctx, wf.Hooks.WorkflowStart, intCtx); err != nil {
		s.logger.Warn("workflow_start hook failed", "error", err)
	}

	// execution loop
	var execErr error
	currentStep := wf.Initial
	for {
		step, ok := wf.Steps[currentStep]
		if !ok {
			execCtx.Status = workflow.StatusFailed
			execErr = fmt.Errorf("step not found: %s", currentStep)
			break
		}

		execCtx.CurrentStep = currentStep

		// terminal state - done
		if step.Type == workflow.StepTypeTerminal {
			execCtx.Status = workflow.StatusCompleted
			execCtx.CompletedAt = time.Now()
			s.checkpoint(ctx, execCtx)
			s.logger.Info("workflow completed", "step", currentStep)
			break
		}

		// execute step
		s.logger.Debug("executing step", "step", step.Name)
		nextStep, err := s.executeStep(ctx, wf, step, execCtx)
		if err != nil {
			execCtx.Status = workflow.StatusFailed
			s.logger.Error("step failed", "step", step.Name, "error", err)
			s.checkpoint(ctx, execCtx)
			execErr = err
			break
		}

		// checkpoint after each step
		s.checkpoint(ctx, execCtx)

		currentStep = nextStep
	}

	// execute workflow hooks based on outcome
	// use background context for hooks since main ctx may be cancelled
	hookCtx := context.Background()
	intCtx = s.buildInterpolationContext(execCtx)

	if execErr != nil {
		// check if this was a cancellation (SIGINT/SIGTERM)
		if errors.Is(execErr, context.Canceled) || ctx.Err() == context.Canceled {
			execCtx.Status = workflow.StatusCancelled
			s.logger.Info("workflow cancelled", "workflow", wf.Name)
			if err := s.hookExecutor.ExecuteHooks(hookCtx, wf.Hooks.WorkflowCancel, intCtx); err != nil {
				s.logger.Warn("workflow_cancel hook failed", "error", err)
			}
			return execCtx, execErr
		}

		// regular error - execute error hook
		intCtx.Error = &interpolation.ErrorData{
			Message: execErr.Error(),
			State:   execCtx.CurrentStep,
		}
		if err := s.hookExecutor.ExecuteHooks(hookCtx, wf.Hooks.WorkflowError, intCtx); err != nil {
			s.logger.Warn("workflow_error hook failed", "error", err)
		}
		return execCtx, execErr
	}

	// workflow completed successfully
	if err := s.hookExecutor.ExecuteHooks(hookCtx, wf.Hooks.WorkflowEnd, intCtx); err != nil {
		s.logger.Warn("workflow_end hook failed", "error", err)
	}
	return execCtx, nil
}

// executeStep executes a single step and returns the next step name.
func (s *ExecutionService) executeStep(
	ctx context.Context,
	wf *workflow.Workflow,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
) (string, error) {
	startTime := time.Now()

	// apply step timeout
	stepCtx := ctx
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
		defer cancel()
	}

	// build interpolation context
	intCtx := s.buildInterpolationContext(execCtx)

	// execute pre-hooks
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Pre, intCtx); err != nil {
		s.logger.Warn("pre-hook failed", "step", step.Name, "error", err)
	}

	// resolve command variables
	resolvedCmd, err := s.resolver.Resolve(step.Command, intCtx)
	if err != nil {
		return "", fmt.Errorf("interpolate command: %w", err)
	}

	// resolve dir if specified
	resolvedDir := ""
	if step.Dir != "" {
		resolvedDir, err = s.resolver.Resolve(step.Dir, intCtx)
		if err != nil {
			return "", fmt.Errorf("interpolate dir: %w", err)
		}
	}

	// build command
	cmd := ports.Command{
		Program: resolvedCmd,
		Dir:     resolvedDir,
		Timeout: step.Timeout,
		Stdout:  s.stdoutWriter,
		Stderr:  s.stderrWriter,
	}

	// execute
	result, execErr := s.executor.Execute(stepCtx, cmd)

	// record step state
	state := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
	}

	if result != nil {
		state.Output = result.Stdout
		state.Stderr = result.Stderr
		state.ExitCode = result.ExitCode
	}

	// determine outcome
	if execErr != nil {
		state.Status = workflow.StatusFailed
		state.Error = execErr.Error()
		execCtx.SetStepState(step.Name, state)

		// execute post-hooks even on failure
		intCtx = s.buildInterpolationContext(execCtx)
		if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx); err != nil {
			s.logger.Warn("post-hook failed", "step", step.Name, "error", err)
		}

		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		return "", fmt.Errorf("step %s: %w", step.Name, execErr)
	}

	if result.ExitCode != 0 {
		state.Status = workflow.StatusFailed
		execCtx.SetStepState(step.Name, state)

		// execute post-hooks even on failure
		intCtx = s.buildInterpolationContext(execCtx)
		if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx); err != nil {
			s.logger.Warn("post-hook failed", "step", step.Name, "error", err)
		}

		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		return "", fmt.Errorf("step %s: exit code %d", step.Name, result.ExitCode)
	}

	state.Status = workflow.StatusCompleted
	execCtx.SetStepState(step.Name, state)

	// execute post-hooks on success
	intCtx = s.buildInterpolationContext(execCtx)
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx); err != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", err)
	}

	return step.OnSuccess, nil
}

// checkpoint saves the current execution state.
// Failures are logged but not fatal - execution continues.
func (s *ExecutionService) checkpoint(ctx context.Context, execCtx *workflow.ExecutionContext) {
	if err := s.store.Save(ctx, execCtx); err != nil {
		s.logger.Warn("checkpoint failed", "workflow_id", execCtx.WorkflowID, "error", err)
	}
}

// buildInterpolationContext converts ExecutionContext to interpolation.Context.
func (s *ExecutionService) buildInterpolationContext(
	execCtx *workflow.ExecutionContext,
) *interpolation.Context {
	// Convert step states
	states := make(map[string]interpolation.StepStateData, len(execCtx.States))
	for name, state := range execCtx.States {
		states[name] = interpolation.StepStateData{
			Output:   state.Output,
			Stderr:   state.Stderr,
			ExitCode: state.ExitCode,
			Status:   state.Status.String(),
		}
	}

	// Get runtime context
	wd, _ := os.Getwd()
	hostname, _ := os.Hostname()

	// Build environment map (merge execCtx.Env with os env)
	env := make(map[string]string)
	for _, e := range os.Environ() {
		if parts := strings.SplitN(e, "=", 2); len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	for k, v := range execCtx.Env {
		env[k] = v // Override with workflow-specific env
	}

	return &interpolation.Context{
		Inputs: execCtx.Inputs,
		States: states,
		Workflow: interpolation.WorkflowData{
			ID:           execCtx.WorkflowID,
			Name:         execCtx.WorkflowName,
			CurrentState: execCtx.CurrentStep,
			StartedAt:    execCtx.StartedAt,
		},
		Env: env,
		Context: interpolation.ContextData{
			WorkingDir: wd,
			User:       os.Getenv("USER"),
			Hostname:   hostname,
		},
		Error: nil, // Set in error hooks (F008)
	}
}
