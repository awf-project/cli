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

// InteractiveExecutor executes workflows in step-by-step interactive mode.
// It pauses before each step, prompts for user action, and displays results.
type InteractiveExecutor struct {
	wfSvc            *WorkflowService
	executor         ports.CommandExecutor
	parallelExecutor ports.ParallelExecutor
	store            ports.StateStore
	logger           ports.Logger
	resolver         interpolation.Resolver
	evaluator        ExpressionEvaluator
	hookExecutor     *HookExecutor
	loopExecutor     *LoopExecutor
	templateSvc      *TemplateService
	prompt           ports.InteractivePrompt
	breakpoints      map[string]bool // steps to pause at (nil = pause at all steps)
	stdoutWriter     io.Writer
	stderrWriter     io.Writer
}

// NewInteractiveExecutor creates a new interactive executor.
func NewInteractiveExecutor(
	wfSvc *WorkflowService,
	executor ports.CommandExecutor,
	parallelExecutor ports.ParallelExecutor,
	store ports.StateStore,
	logger ports.Logger,
	resolver interpolation.Resolver,
	evaluator ExpressionEvaluator,
	prompt ports.InteractivePrompt,
) *InteractiveExecutor {
	return &InteractiveExecutor{
		wfSvc:            wfSvc,
		executor:         executor,
		parallelExecutor: parallelExecutor,
		store:            store,
		logger:           logger,
		resolver:         resolver,
		evaluator:        evaluator,
		hookExecutor:     NewHookExecutor(executor, logger, resolver),
		loopExecutor:     NewLoopExecutor(logger, evaluator, resolver),
		prompt:           prompt,
	}
}

// SetTemplateService configures the template service for workflow expansion.
func (e *InteractiveExecutor) SetTemplateService(svc *TemplateService) {
	e.templateSvc = svc
}

// SetBreakpoints sets specific steps to pause at.
// If breakpoints is nil or empty, all steps will pause.
func (e *InteractiveExecutor) SetBreakpoints(breakpoints []string) {
	if len(breakpoints) == 0 {
		e.breakpoints = nil
		return
	}
	e.breakpoints = make(map[string]bool)
	for _, bp := range breakpoints {
		e.breakpoints[bp] = true
	}
}

// SetOutputWriters configures streaming output writers.
func (e *InteractiveExecutor) SetOutputWriters(stdout, stderr io.Writer) {
	e.stdoutWriter = stdout
	e.stderrWriter = stderr
}

// Run executes the workflow in interactive mode.
// It returns the final execution context and any error.
func (e *InteractiveExecutor) Run(ctx context.Context, workflowName string, inputs map[string]any) (*workflow.ExecutionContext, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Load workflow
	wf, err := e.wfSvc.GetWorkflow(ctx, workflowName)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow: %w", err)
	}
	if wf == nil {
		return nil, fmt.Errorf("workflow '%s' not found", workflowName)
	}

	// Expand templates if template service is available
	if e.templateSvc != nil {
		if err := e.templateSvc.ExpandWorkflow(ctx, wf); err != nil {
			return nil, fmt.Errorf("failed to expand templates: %w", err)
		}
	}

	// Initialize execution context
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

	// Count steps for progress display
	totalSteps := len(wf.Steps)

	// Show interactive mode header
	e.prompt.ShowHeader(wf.Name)

	// Track execution state
	currentStep := wf.Initial
	stepIndex := 0
	var lastExecutedStep string
	hasRetry := false

	// Execution loop
	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return execCtx, ctx.Err()
		default:
		}

		step, ok := wf.Steps[currentStep]
		if !ok {
			execCtx.Status = workflow.StatusFailed
			return execCtx, fmt.Errorf("step not found: %s", currentStep)
		}

		execCtx.CurrentStep = currentStep
		stepIndex++

		// Terminal state - workflow complete
		if step.Type == workflow.StepTypeTerminal {
			execCtx.Status = workflow.StatusCompleted
			execCtx.CompletedAt = time.Now()
			e.checkpoint(ctx, execCtx)
			e.prompt.ShowCompleted(execCtx.Status)
			return execCtx, nil
		}

		// Build interpolation context for display
		interpCtx := e.buildInterpolationContext(execCtx)

		// Should we pause at this step?
		if e.shouldPause(currentStep) {
			// Show step details
			info := e.buildStepInfo(step, stepIndex, totalSteps, interpCtx)
			e.prompt.ShowStepDetails(info)

			// Prompt for action
			hasRetry = lastExecutedStep != ""
			action, err := e.handleInteractivePrompt(ctx, execCtx, interpCtx, hasRetry)
			if err != nil {
				return execCtx, err
			}

			// Handle action
			switch action {
			case workflow.ActionAbort:
				e.prompt.ShowAborted()
				execCtx.Status = workflow.StatusCancelled
				e.checkpoint(ctx, execCtx)
				return execCtx, nil

			case workflow.ActionSkip:
				e.prompt.ShowSkipped(step.Name, step.OnSuccess)
				currentStep = step.OnSuccess
				continue

			case workflow.ActionRetry:
				if lastExecutedStep != "" {
					// Re-execute the last step
					currentStep = lastExecutedStep
					stepIndex-- // Adjust index since we're repeating
					continue
				}

			case workflow.ActionContinue:
				// Fall through to execute
			}
		}

		// Execute step
		e.prompt.ShowExecuting(step.Name)
		var nextStep string
		var execErr error

		switch step.Type {
		case workflow.StepTypeParallel:
			nextStep, execErr = e.executeParallelStep(ctx, wf, step, execCtx)
		case workflow.StepTypeForEach, workflow.StepTypeWhile:
			nextStep, execErr = e.executeLoopStep(ctx, wf, step, execCtx)
		default:
			nextStep, execErr = e.executeStep(ctx, wf, step, execCtx)
		}

		// Get step state for display
		if state, exists := execCtx.States[step.Name]; exists {
			e.prompt.ShowStepResult(&state, nextStep)
		}

		if execErr != nil {
			// Check for context cancellation
			if errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded) {
				execCtx.Status = workflow.StatusCancelled
				e.prompt.ShowAborted()
				e.checkpoint(ctx, execCtx)
				return execCtx, execErr
			}

			execCtx.Status = workflow.StatusFailed
			e.prompt.ShowError(execErr)
			e.checkpoint(ctx, execCtx)
			return execCtx, execErr
		}

		lastExecutedStep = step.Name
		e.checkpoint(ctx, execCtx)
		currentStep = nextStep
	}
}

// shouldPause returns true if execution should pause at the given step.
func (e *InteractiveExecutor) shouldPause(stepName string) bool {
	if e.breakpoints == nil {
		return true // pause at all steps when no breakpoints set
	}
	return e.breakpoints[stepName]
}

// buildStepInfo creates step information for display.
func (e *InteractiveExecutor) buildStepInfo(
	step *workflow.Step,
	index int,
	total int,
	interpCtx *interpolation.Context,
) *workflow.InteractiveStepInfo {
	return &workflow.InteractiveStepInfo{
		Name:        step.Name,
		Index:       index,
		Total:       total,
		Step:        step,
		Command:     e.resolveCommand(step.Command, interpCtx),
		Transitions: e.formatTransitions(step),
	}
}

// resolveCommand resolves template variables in a command string.
func (e *InteractiveExecutor) resolveCommand(cmd string, interpCtx *interpolation.Context) string {
	if e.resolver == nil {
		return cmd
	}
	resolved, err := e.resolver.Resolve(cmd, interpCtx)
	if err != nil {
		return cmd
	}
	return resolved
}

// formatTransitions creates human-readable transition descriptions.
func (e *InteractiveExecutor) formatTransitions(step *workflow.Step) []string {
	var transitions []string

	// Add conditional transitions first
	for _, tr := range step.Transitions {
		desc := fmt.Sprintf("→ when '%s': %s", tr.When, tr.Goto)
		if tr.When == "" {
			desc = fmt.Sprintf("→ default: %s", tr.Goto)
		}
		transitions = append(transitions, desc)
	}

	// Add legacy transitions if no conditional ones
	if len(step.Transitions) == 0 {
		if step.OnSuccess != "" {
			transitions = append(transitions, fmt.Sprintf("→ on_success: %s", step.OnSuccess))
		}
		if step.OnFailure != "" {
			transitions = append(transitions, fmt.Sprintf("→ on_failure: %s", step.OnFailure))
		}
	}

	// Loop completion
	if step.Loop != nil && step.Loop.OnComplete != "" {
		transitions = append(transitions, fmt.Sprintf("→ on_complete: %s", step.Loop.OnComplete))
	}

	return transitions
}

// handleInteractivePrompt handles the interactive prompt loop, including inspect and edit.
func (e *InteractiveExecutor) handleInteractivePrompt(
	ctx context.Context,
	execCtx *workflow.ExecutionContext,
	interpCtx *interpolation.Context,
	hasRetry bool,
) (workflow.InteractiveAction, error) {
	for {
		action, err := e.prompt.PromptAction(hasRetry)
		if err != nil {
			return workflow.ActionAbort, err
		}

		switch action {
		case workflow.ActionInspect:
			e.prompt.ShowContext(interpCtx)
			continue

		case workflow.ActionEdit:
			if err := e.handleEditInput(execCtx, interpCtx); err != nil {
				e.prompt.ShowError(err)
			}
			continue

		default:
			return action, nil
		}
	}
}

// handleEditInput prompts for input name and edits it.
func (e *InteractiveExecutor) handleEditInput(
	execCtx *workflow.ExecutionContext,
	interpCtx *interpolation.Context,
) error {
	// For simplicity, prompt for the first available input
	if len(execCtx.Inputs) == 0 {
		return fmt.Errorf("no inputs available to edit")
	}

	// Get the first input key (in real implementation, would prompt for which input)
	for name, current := range execCtx.Inputs {
		newValue, err := e.prompt.EditInput(name, current)
		if err != nil {
			return err
		}
		execCtx.SetInput(name, newValue)
		interpCtx.Inputs[name] = newValue
		break
	}

	return nil
}

// checkpoint saves the current execution state.
func (e *InteractiveExecutor) checkpoint(ctx context.Context, execCtx *workflow.ExecutionContext) {
	if err := e.store.Save(ctx, execCtx); err != nil {
		e.logger.Warn("checkpoint failed", "workflow_id", execCtx.WorkflowID, "error", err)
	}
}

// buildInterpolationContext converts ExecutionContext to interpolation.Context.
func (e *InteractiveExecutor) buildInterpolationContext(
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

	// Build environment map
	env := make(map[string]string)
	for _, envStr := range os.Environ() {
		if parts := strings.SplitN(envStr, "=", 2); len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	for k, v := range execCtx.Env {
		env[k] = v
	}

	intCtx := &interpolation.Context{
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
	}

	// Include loop context if we're inside a loop
	intCtx.Loop = buildLoopDataChain(execCtx.CurrentLoop)

	return intCtx
}

// executeStep executes a single command step.
func (e *InteractiveExecutor) executeStep(
	ctx context.Context,
	wf *workflow.Workflow,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
) (string, error) {
	startTime := time.Now()

	// Apply step timeout
	stepCtx := ctx
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
		defer cancel()
	}

	// Build interpolation context
	intCtx := e.buildInterpolationContext(execCtx)

	// Execute pre-hooks
	if err := e.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Pre, intCtx); err != nil {
		e.logger.Warn("pre-hook failed", "step", step.Name, "error", err)
	}

	// Resolve command variables
	resolvedCmd, err := e.resolver.Resolve(step.Command, intCtx)
	if err != nil {
		return "", fmt.Errorf("interpolate command: %w", err)
	}

	// Resolve dir if specified
	resolvedDir := ""
	if step.Dir != "" {
		resolvedDir, err = e.resolver.Resolve(step.Dir, intCtx)
		if err != nil {
			return "", fmt.Errorf("interpolate dir: %w", err)
		}
	}

	// Build command
	cmd := ports.Command{
		Program: resolvedCmd,
		Dir:     resolvedDir,
		Timeout: step.Timeout,
		Stdout:  e.stdoutWriter,
		Stderr:  e.stderrWriter,
	}

	// Execute command
	result, execErr := e.executor.Execute(stepCtx, cmd)

	// Record step state
	state := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
		Attempt:     1,
	}

	if result != nil {
		state.Output = result.Stdout
		state.Stderr = result.Stderr
		state.ExitCode = result.ExitCode
	}

	// Determine outcome
	if execErr != nil {
		state.Status = workflow.StatusFailed
		state.Error = execErr.Error()
		execCtx.SetStepState(step.Name, state)

		// Execute post-hooks even on failure
		intCtx = e.buildInterpolationContext(execCtx)
		if err := e.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx); err != nil {
			e.logger.Warn("post-hook failed", "step", step.Name, "error", err)
		}

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

		// Execute post-hooks
		intCtx = e.buildInterpolationContext(execCtx)
		if err := e.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx); err != nil {
			e.logger.Warn("post-hook failed", "step", step.Name, "error", err)
		}

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

	// Execute post-hooks
	intCtx = e.buildInterpolationContext(execCtx)
	if err := e.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx); err != nil {
		e.logger.Warn("post-hook failed", "step", step.Name, "error", err)
	}

	return e.resolveNextStep(step, intCtx, true)
}

// resolveNextStep determines the next step using transitions or legacy OnSuccess/OnFailure.
func (e *InteractiveExecutor) resolveNextStep(
	step *workflow.Step,
	intCtx *interpolation.Context,
	success bool,
) (string, error) {
	// If transitions are defined, evaluate them first
	if len(step.Transitions) > 0 && e.evaluator != nil {
		evalFunc := func(expr string) (bool, error) {
			return e.evaluator.Evaluate(expr, intCtx)
		}

		nextStep, found, err := step.Transitions.EvaluateFirstMatch(evalFunc)
		if err != nil {
			return "", fmt.Errorf("evaluate transitions: %w", err)
		}
		if found {
			return nextStep, nil
		}
	}

	// Legacy fallback
	if success {
		return step.OnSuccess, nil
	}
	return step.OnFailure, nil
}

// executeParallelStep executes a parallel step.
func (e *InteractiveExecutor) executeParallelStep(
	ctx context.Context,
	wf *workflow.Workflow,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
) (string, error) {
	startTime := time.Now()

	// Apply step timeout
	stepCtx := ctx
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
		defer cancel()
	}

	// Build interpolation context
	intCtx := e.buildInterpolationContext(execCtx)

	// Execute pre-hooks
	if err := e.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Pre, intCtx); err != nil {
		e.logger.Warn("pre-hook failed", "step", step.Name, "error", err)
	}

	// Build parallel config
	config := workflow.ParallelConfig{
		Strategy:      workflow.ParseParallelStrategy(step.Strategy),
		MaxConcurrent: step.MaxConcurrent,
	}

	// Create step executor adapter
	adapter := &interactiveStepExecutorAdapter{
		execSvc: e,
	}

	// Execute parallel branches
	result, err := e.parallelExecutor.Execute(stepCtx, wf, step.Branches, config, execCtx, adapter)

	// Copy branch results to execCtx.States
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

	// Record parallel step state
	parallelState := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
	}

	if err != nil {
		parallelState.Status = workflow.StatusFailed
		parallelState.Error = err.Error()
		execCtx.SetStepState(step.Name, parallelState)

		// Execute post-hooks
		intCtx = e.buildInterpolationContext(execCtx)
		if hookErr := e.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx); hookErr != nil {
			e.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
		}

		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		return "", fmt.Errorf("parallel step %s: %w", step.Name, err)
	}

	parallelState.Status = workflow.StatusCompleted
	execCtx.SetStepState(step.Name, parallelState)

	// Execute post-hooks
	intCtx = e.buildInterpolationContext(execCtx)
	if hookErr := e.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx); hookErr != nil {
		e.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
	}

	return e.resolveNextStep(step, intCtx, true)
}

// executeLoopStep executes a for_each or while loop step.
func (e *InteractiveExecutor) executeLoopStep(
	ctx context.Context,
	wf *workflow.Workflow,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
) (string, error) {
	startTime := time.Now()

	// Apply step timeout
	stepCtx := ctx
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
		defer cancel()
	}

	// Execute pre-hooks
	intCtx := e.buildInterpolationContext(execCtx)
	if err := e.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Pre, intCtx); err != nil {
		e.logger.Warn("pre-hook failed", "step", step.Name, "error", err)
	}

	// Create step executor callback
	stepExecutor := func(ctx context.Context, stepName string, loopIntCtx *interpolation.Context) error {
		bodyStep, ok := wf.Steps[stepName]
		if !ok {
			return fmt.Errorf("body step not found: %s", stepName)
		}
		var err error
		switch bodyStep.Type {
		case workflow.StepTypeForEach, workflow.StepTypeWhile:
			_, err = e.executeLoopStep(ctx, wf, bodyStep, execCtx)
		case workflow.StepTypeParallel:
			_, err = e.executeParallelStep(ctx, wf, bodyStep, execCtx)
		default:
			_, err = e.executeStep(ctx, wf, bodyStep, execCtx)
		}
		return err
	}

	// Execute loop
	var result *workflow.LoopResult
	var err error

	if step.Type == workflow.StepTypeForEach {
		result, err = e.loopExecutor.ExecuteForEach(
			stepCtx, wf, step, execCtx, stepExecutor, e.buildInterpolationContext)
	} else {
		result, err = e.loopExecutor.ExecuteWhile(
			stepCtx, wf, step, execCtx, stepExecutor, e.buildInterpolationContext)
	}

	// Record loop step state
	loopState := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
	}

	if err != nil {
		loopState.Status = workflow.StatusFailed
		loopState.Error = err.Error()
		execCtx.SetStepState(step.Name, loopState)

		// Execute post-hooks
		intCtx = e.buildInterpolationContext(execCtx)
		if hookErr := e.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx); hookErr != nil {
			e.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
		}

		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		return "", err
	}

	loopState.Status = workflow.StatusCompleted
	if result != nil {
		loopState.Output = fmt.Sprintf("completed %d iterations", result.TotalCount)
	}
	execCtx.SetStepState(step.Name, loopState)

	// Execute post-hooks
	intCtx = e.buildInterpolationContext(execCtx)
	if hookErr := e.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx); hookErr != nil {
		e.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
	}

	return step.Loop.OnComplete, nil
}

// interactiveStepExecutorAdapter adapts InteractiveExecutor to the ports.StepExecutor interface.
type interactiveStepExecutorAdapter struct {
	execSvc *InteractiveExecutor
}

// ExecuteStep implements ports.StepExecutor.
func (a *interactiveStepExecutorAdapter) ExecuteStep(
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

	_, err := a.execSvc.executeStep(ctx, wf, step, execCtx)
	result.CompletedAt = time.Now()

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
