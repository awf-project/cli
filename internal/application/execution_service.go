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
	"github.com/vanoix/awf/pkg/retry"
	"github.com/vanoix/awf/pkg/validation"
)

// ExecutionService orchestrates workflow execution.
type ExecutionService struct {
	workflowSvc      *WorkflowService
	executor         ports.CommandExecutor
	parallelExecutor ports.ParallelExecutor
	store            ports.StateStore
	logger           ports.Logger
	resolver         interpolation.Resolver
	evaluator        ExpressionEvaluator
	hookExecutor     *HookExecutor
	stdoutWriter     io.Writer
	stderrWriter     io.Writer
	historySvc       *HistoryService
}

// ExpressionEvaluator evaluates conditional expressions.
type ExpressionEvaluator interface {
	Evaluate(expr string, ctx *interpolation.Context) (bool, error)
}

// SetOutputWriters configures streaming output writers.
func (s *ExecutionService) SetOutputWriters(stdout, stderr io.Writer) {
	s.stdoutWriter = stdout
	s.stderrWriter = stderr
}

// NewExecutionService creates a new execution service.
// historySvc can be nil to disable history recording.
func NewExecutionService(
	wfSvc *WorkflowService,
	executor ports.CommandExecutor,
	parallelExecutor ports.ParallelExecutor,
	store ports.StateStore,
	logger ports.Logger,
	resolver interpolation.Resolver,
	historySvc *HistoryService,
) *ExecutionService {
	return &ExecutionService{
		workflowSvc:      wfSvc,
		executor:         executor,
		parallelExecutor: parallelExecutor,
		store:            store,
		logger:           logger,
		resolver:         resolver,
		hookExecutor:     NewHookExecutor(executor, logger, resolver),
		historySvc:       historySvc,
	}
}

// NewExecutionServiceWithEvaluator creates a new execution service with expression evaluator.
// This enables conditional transitions using the `when:` clause.
func NewExecutionServiceWithEvaluator(
	wfSvc *WorkflowService,
	executor ports.CommandExecutor,
	parallelExecutor ports.ParallelExecutor,
	store ports.StateStore,
	logger ports.Logger,
	resolver interpolation.Resolver,
	historySvc *HistoryService,
	evaluator ExpressionEvaluator,
) *ExecutionService {
	return &ExecutionService{
		workflowSvc:      wfSvc,
		executor:         executor,
		parallelExecutor: parallelExecutor,
		store:            store,
		logger:           logger,
		resolver:         resolver,
		evaluator:        evaluator,
		hookExecutor:     NewHookExecutor(executor, logger, resolver),
		historySvc:       historySvc,
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

	// Validate inputs against definitions
	if err := s.validateInputs(execCtx.Inputs, wf.Inputs); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
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
			s.recordHistory(execCtx)
			s.logger.Info("workflow completed", "step", currentStep)
			break
		}

		// execute step based on type
		var nextStep string
		var err error

		s.logger.Debug("executing step", "step", step.Name)

		if step.Type == workflow.StepTypeParallel {
			nextStep, err = s.executeParallelStep(ctx, wf, step, execCtx)
		} else {
			nextStep, err = s.executeStep(ctx, wf, step, execCtx)
		}

		if err != nil {
			execCtx.Status = workflow.StatusFailed
			s.logger.Error("step failed", "step", step.Name, "error", err)
			s.checkpoint(ctx, execCtx)
			s.recordHistory(execCtx)
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
		// check if this was a cancellation (SIGINT/SIGTERM) or timeout
		if errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded) ||
			ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
			execCtx.Status = workflow.StatusCancelled
			s.logger.Info("workflow cancelled", "workflow", wf.Name)
			s.recordHistory(execCtx)
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

	// execute (with retry if configured)
	var result *ports.CommandResult
	var execErr error
	var attempt int

	if step.Retry != nil && step.Retry.MaxAttempts > 1 {
		result, attempt, execErr = s.executeWithRetry(stepCtx, step, cmd)
	} else {
		attempt = 1
		result, execErr = s.executor.Execute(stepCtx, cmd)
	}

	// record step state
	state := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
		Attempt:     attempt,
	}

	if result != nil {
		state.Output = result.Stdout
		state.Stderr = result.Stderr
		state.ExitCode = result.ExitCode
	}

	// determine outcome
	if execErr != nil {
		// Check if the PARENT context was cancelled (workflow-level cancellation)
		// This is different from step timeout (stepCtx deadline exceeded)
		// Step timeout should follow OnFailure, workflow cancellation should propagate
		if ctx.Err() != nil && (errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded)) {
			state.Status = workflow.StatusFailed
			state.Error = execErr.Error()
			execCtx.SetStepState(step.Name, state)
			return "", fmt.Errorf("step %s: %w", step.Name, execErr)
		}

		state.Status = workflow.StatusFailed
		state.Error = execErr.Error()
		execCtx.SetStepState(step.Name, state)

		// execute post-hooks even on failure
		intCtx = s.buildInterpolationContext(execCtx)
		if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx); err != nil {
			s.logger.Warn("post-hook failed", "step", step.Name, "error", err)
		}

		// If continue_on_error is true, follow on_success path
		if step.ContinueOnError {
			return step.OnSuccess, nil
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

		// If continue_on_error is true, follow on_success path
		if step.ContinueOnError {
			return step.OnSuccess, nil
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

	// Resolve next step using transitions (if defined) or fallback to OnSuccess
	return s.resolveNextStep(step, intCtx, true)
}

// resolveNextStep determines the next step using conditional transitions or legacy OnSuccess/OnFailure.
// If step has Transitions defined, evaluates them in order (first match wins).
// If no transition matches and no default, falls back to OnSuccess/OnFailure based on success parameter.
func (s *ExecutionService) resolveNextStep(
	step *workflow.Step,
	intCtx *interpolation.Context,
	success bool,
) (string, error) {
	// If transitions are defined, evaluate them first
	if len(step.Transitions) > 0 && s.evaluator != nil {
		evalFunc := func(expr string) (bool, error) {
			return s.evaluator.Evaluate(expr, intCtx)
		}

		nextStep, found, err := step.Transitions.EvaluateFirstMatch(evalFunc)
		if err != nil {
			return "", fmt.Errorf("evaluate transitions: %w", err)
		}
		if found {
			s.logger.Debug("transition matched", "step", step.Name, "next", nextStep)
			return nextStep, nil
		}
		// No transition matched, fall through to legacy handling
		s.logger.Debug("no transition matched, using legacy", "step", step.Name)
	}

	// Legacy fallback: use OnSuccess/OnFailure
	if success {
		return step.OnSuccess, nil
	}
	return step.OnFailure, nil
}

// checkpoint saves the current execution state.
// Failures are logged but not fatal - execution continues.
func (s *ExecutionService) checkpoint(ctx context.Context, execCtx *workflow.ExecutionContext) {
	if err := s.store.Save(ctx, execCtx); err != nil {
		s.logger.Warn("checkpoint failed", "workflow_id", execCtx.WorkflowID, "error", err)
	}
}

// recordHistory saves execution to history when workflow reaches terminal state.
// Failures are logged but not fatal - execution continues.
func (s *ExecutionService) recordHistory(execCtx *workflow.ExecutionContext) {
	if s.historySvc == nil {
		return
	}

	// Only record terminal states
	if execCtx.Status != workflow.StatusCompleted &&
		execCtx.Status != workflow.StatusFailed &&
		execCtx.Status != workflow.StatusCancelled {
		return
	}

	// Map ExecutionContext to ExecutionRecord
	record := &workflow.ExecutionRecord{
		ID:           execCtx.WorkflowID,
		WorkflowID:   execCtx.WorkflowID,
		WorkflowName: execCtx.WorkflowName,
		Status:       execCtx.Status.String(),
		StartedAt:    execCtx.StartedAt,
		CompletedAt:  execCtx.CompletedAt,
	}

	// Calculate duration
	if !execCtx.CompletedAt.IsZero() {
		record.DurationMs = execCtx.CompletedAt.Sub(execCtx.StartedAt).Milliseconds()
	}

	// Find last executed step for exit code and error
	if len(execCtx.States) > 0 {
		var lastState *workflow.StepState
		for _, state := range execCtx.States {
			stateCopy := state
			if lastState == nil || state.CompletedAt.After(lastState.CompletedAt) {
				lastState = &stateCopy
			}
		}
		if lastState != nil {
			record.ExitCode = lastState.ExitCode
			if lastState.Error != "" {
				record.ErrorMessage = lastState.Error
			}
		}
	}

	// Record to history store (use background context to avoid cancellation)
	ctx := context.Background()
	if err := s.historySvc.Record(ctx, record); err != nil {
		s.logger.Warn("failed to record history", "workflow_id", execCtx.WorkflowID, "error", err)
	} else {
		s.logger.Debug("recorded execution history", "workflow_id", execCtx.WorkflowID, "status", record.Status)
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

// executeWithRetry executes a command with retry logic.
// Returns the final result, number of attempts made, and error.
func (s *ExecutionService) executeWithRetry(
	ctx context.Context,
	step *workflow.Step,
	cmd ports.Command,
) (*ports.CommandResult, int, error) {
	// Convert domain RetryConfig to retry.Config
	retryCfg := retry.Config{
		MaxAttempts:        step.Retry.MaxAttempts,
		InitialDelay:       time.Duration(step.Retry.InitialDelayMs) * time.Millisecond,
		MaxDelay:           time.Duration(step.Retry.MaxDelayMs) * time.Millisecond,
		Strategy:           retry.Strategy(step.Retry.Backoff),
		Multiplier:         step.Retry.Multiplier,
		Jitter:             step.Retry.Jitter,
		RetryableExitCodes: step.Retry.RetryableExitCodes,
	}

	// Create retryer with current time as seed for random jitter
	retryer := retry.NewRetryer(retryCfg, &retryLoggerAdapter{s.logger}, time.Now().UnixNano())

	var result *ports.CommandResult
	var execErr error

	for attempt := 1; attempt <= retryCfg.MaxAttempts; attempt++ {
		result, execErr = s.executor.Execute(ctx, cmd)

		// Determine exit code
		exitCode := 0
		if result != nil {
			exitCode = result.ExitCode
		}
		if execErr != nil && exitCode == 0 {
			// Execution error without exit code (e.g., context cancelled)
			// Don't retry on context errors
			if errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded) {
				return result, attempt, execErr
			}
			exitCode = 1 // Treat as generic failure
		}

		// Success - no retry needed
		if execErr == nil && exitCode == 0 {
			return result, attempt, nil
		}

		// Check if we should retry
		if !retryer.ShouldRetry(exitCode, attempt) {
			s.logger.Debug("not retrying",
				"step", step.Name,
				"attempt", attempt,
				"exit_code", exitCode,
				"max_attempts", retryCfg.MaxAttempts,
			)
			return result, attempt, execErr
		}

		// Log retry
		s.logger.Info("retrying step",
			"step", step.Name,
			"attempt", attempt,
			"exit_code", exitCode,
			"max_attempts", retryCfg.MaxAttempts,
		)

		// Wait before next attempt
		if err := retryer.Wait(ctx, attempt); err != nil {
			// Context cancelled during wait
			return result, attempt, err
		}
	}

	return result, retryCfg.MaxAttempts, execErr
}

// retryLoggerAdapter adapts ports.Logger to retry.Logger interface.
type retryLoggerAdapter struct {
	logger ports.Logger
}

func (a *retryLoggerAdapter) Debug(msg string, keysAndValues ...any) {
	a.logger.Debug(msg, keysAndValues...)
}

func (a *retryLoggerAdapter) Info(msg string, keysAndValues ...any) {
	a.logger.Info(msg, keysAndValues...)
}

// executeParallelStep executes a parallel step using the ParallelExecutor.
func (s *ExecutionService) executeParallelStep(
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

	// build interpolation context for hooks
	intCtx := s.buildInterpolationContext(execCtx)

	// execute pre-hooks
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Pre, intCtx); err != nil {
		s.logger.Warn("pre-hook failed", "step", step.Name, "error", err)
	}

	// build parallel config
	config := workflow.ParallelConfig{
		Strategy:      workflow.ParseParallelStrategy(step.Strategy),
		MaxConcurrent: step.MaxConcurrent,
	}

	// create step executor adapter
	adapter := &stepExecutorAdapter{
		execSvc: s,
	}

	// execute parallel branches
	s.logger.Info("executing parallel step", "step", step.Name, "branches", step.Branches, "strategy", config.Strategy)
	result, err := s.parallelExecutor.Execute(stepCtx, wf, step.Branches, config, execCtx, adapter)

	// copy branch results to execCtx.States so they're available for interpolation
	if result != nil {
		for branchName, branchResult := range result.Results {
			state := workflow.StepState{
				Name:        branchName,
				StartedAt:   branchResult.StartedAt,
				CompletedAt: branchResult.CompletedAt,
				Output:      branchResult.Output,
				Stderr:      branchResult.Stderr,
				ExitCode:    branchResult.ExitCode,
			}
			if branchResult.Error != nil {
				state.Status = workflow.StatusFailed
				state.Error = branchResult.Error.Error()
			} else if branchResult.ExitCode != 0 {
				state.Status = workflow.StatusFailed
			} else {
				state.Status = workflow.StatusCompleted
			}
			execCtx.SetStepState(branchName, state)
		}
	}

	// record parallel step state
	parallelState := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
	}

	if err != nil {
		// Check if the PARENT context was cancelled
		if ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
			parallelState.Status = workflow.StatusFailed
			parallelState.Error = err.Error()
			execCtx.SetStepState(step.Name, parallelState)
			return "", fmt.Errorf("parallel step %s: %w", step.Name, err)
		}

		parallelState.Status = workflow.StatusFailed
		parallelState.Error = err.Error()
		execCtx.SetStepState(step.Name, parallelState)

		// execute post-hooks even on failure
		intCtx = s.buildInterpolationContext(execCtx)
		if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx); hookErr != nil {
			s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
		}

		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		return "", fmt.Errorf("parallel step %s: %w", step.Name, err)
	}

	parallelState.Status = workflow.StatusCompleted
	execCtx.SetStepState(step.Name, parallelState)

	// execute post-hooks on success
	intCtx = s.buildInterpolationContext(execCtx)
	if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx); hookErr != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
	}

	// Resolve next step using transitions (if defined) or fallback to OnSuccess
	return s.resolveNextStep(step, intCtx, true)
}

// stepExecutorAdapter adapts ExecutionService to the ports.StepExecutor interface.
type stepExecutorAdapter struct {
	execSvc *ExecutionService
}

// ExecuteStep implements ports.StepExecutor.
func (a *stepExecutorAdapter) ExecuteStep(
	ctx context.Context,
	wf *workflow.Workflow,
	stepName string,
	execCtx *workflow.ExecutionContext,
) (*workflow.BranchResult, error) {
	step, ok := wf.Steps[stepName]
	if !ok {
		return nil, fmt.Errorf("step not found: %s", stepName)
	}

	startTime := time.Now()
	result := &workflow.BranchResult{
		Name:      stepName,
		StartedAt: startTime,
	}

	// Execute the step using the existing executeStep method
	_, err := a.execSvc.executeStep(ctx, wf, step, execCtx)

	result.CompletedAt = time.Now()

	// Get the step state that was recorded by executeStep
	if state, exists := execCtx.States[stepName]; exists {
		result.Output = state.Output
		result.Stderr = state.Stderr
		result.ExitCode = state.ExitCode
		if state.Error != "" {
			result.Error = errors.New(state.Error)
		}
	}

	if err != nil {
		result.Error = err
	}

	return result, err
}

// validateInputs converts workflow.Input to validation.Input and validates.
func (s *ExecutionService) validateInputs(inputs map[string]any, defs []workflow.Input) error {
	valDefs := make([]validation.Input, len(defs))
	for i, d := range defs {
		valDefs[i] = validation.Input{
			Name:     d.Name,
			Type:     d.Type,
			Required: d.Required,
		}
		if d.Validation != nil {
			valDefs[i].Validation = &validation.Rules{
				Pattern:       d.Validation.Pattern,
				Enum:          d.Validation.Enum,
				Min:           d.Validation.Min,
				Max:           d.Validation.Max,
				FileExists:    d.Validation.FileExists,
				FileExtension: d.Validation.FileExtension,
			}
		}
	}
	return validation.ValidateInputs(inputs, valDefs)
}

// Resume continues an interrupted workflow execution from where it left off.
// It loads persisted state, validates resumability, merges input overrides,
// and continues execution from CurrentStep while skipping completed steps.
func (s *ExecutionService) Resume(
	ctx context.Context,
	workflowID string,
	inputOverrides map[string]any,
) (*workflow.ExecutionContext, error) {
	// 1. Load state
	execCtx, err := s.store.Load(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}
	if execCtx == nil {
		return nil, fmt.Errorf("workflow execution not found: %s", workflowID)
	}

	// 2. Validate resumable (not completed)
	if execCtx.Status == workflow.StatusCompleted {
		return nil, fmt.Errorf("workflow already completed, cannot resume")
	}

	// 3. Load workflow definition
	wf, err := s.workflowSvc.GetWorkflow(ctx, execCtx.WorkflowName)
	if err != nil {
		return nil, fmt.Errorf("load workflow: %w", err)
	}
	if wf == nil {
		return nil, fmt.Errorf("workflow '%s' not found", execCtx.WorkflowName)
	}

	// 4. Validate current step exists
	if _, exists := wf.Steps[execCtx.CurrentStep]; !exists {
		return nil, fmt.Errorf("cannot resume: step '%s' no longer exists in workflow", execCtx.CurrentStep)
	}

	// 5. Merge input overrides
	for k, v := range inputOverrides {
		execCtx.SetInput(k, v)
	}

	// 6. Reset status to running
	execCtx.Status = workflow.StatusRunning

	// 7. Execute from current step
	s.logger.Info("resuming workflow", "id", workflowID, "from", execCtx.CurrentStep)

	// execute workflow_start hooks (on resume we might want these again)
	intCtx := s.buildInterpolationContext(execCtx)
	if err := s.hookExecutor.ExecuteHooks(ctx, wf.Hooks.WorkflowStart, intCtx); err != nil {
		s.logger.Warn("workflow_start hook failed", "error", err)
	}

	// Continue execution from current step
	return s.executeFromStep(ctx, wf, execCtx)
}

// ListResumable returns all workflow executions that can be resumed.
// A workflow is resumable if its status is not completed.
func (s *ExecutionService) ListResumable(ctx context.Context) ([]*workflow.ExecutionContext, error) {
	ids, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}

	var resumable []*workflow.ExecutionContext
	for _, id := range ids {
		execCtx, err := s.store.Load(ctx, id)
		if err != nil || execCtx == nil {
			continue
		}
		// Only include non-completed executions
		if execCtx.Status != workflow.StatusCompleted {
			resumable = append(resumable, execCtx)
		}
	}
	return resumable, nil
}

// executeFromStep continues workflow execution from the specified starting step.
// It handles the execution loop, hooks, and state transitions.
func (s *ExecutionService) executeFromStep(
	ctx context.Context,
	wf *workflow.Workflow,
	execCtx *workflow.ExecutionContext,
) (*workflow.ExecutionContext, error) {
	var execErr error
	currentStep := execCtx.CurrentStep

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
			s.recordHistory(execCtx)
			s.logger.Info("workflow completed", "step", currentStep)
			break
		}

		// execute step based on type
		var nextStep string
		var err error

		s.logger.Debug("executing step", "step", step.Name)

		if step.Type == workflow.StepTypeParallel {
			nextStep, err = s.executeParallelStep(ctx, wf, step, execCtx)
		} else {
			nextStep, err = s.executeStep(ctx, wf, step, execCtx)
		}

		if err != nil {
			execCtx.Status = workflow.StatusFailed
			s.logger.Error("step failed", "step", step.Name, "error", err)
			s.checkpoint(ctx, execCtx)
			s.recordHistory(execCtx)
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
	intCtx := s.buildInterpolationContext(execCtx)

	if execErr != nil {
		// check if this was a cancellation (SIGINT/SIGTERM) or timeout
		if errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded) ||
			ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
			execCtx.Status = workflow.StatusCancelled
			s.logger.Info("workflow cancelled", "workflow", wf.Name)
			s.recordHistory(execCtx)
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
