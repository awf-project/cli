package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/pkg/interpolation"
	"github.com/awf-project/awf/pkg/retry"
	"github.com/awf-project/awf/pkg/validation"
	"github.com/google/uuid"
)

// ConversationExecutor defines the interface for executing multi-turn conversations.
// This interface allows for dependency injection and testing with mocks.
type ConversationExecutor interface {
	ExecuteConversation(
		ctx context.Context,
		step *workflow.Step,
		config *workflow.ConversationConfig,
		execCtx *workflow.ExecutionContext,
		buildContext ContextBuilderFunc,
	) (*workflow.ConversationResult, error)
}

// ExecutionService orchestrates workflow execution.
type ExecutionService struct {
	workflowSvc       *WorkflowService
	executor          ports.CommandExecutor
	parallelExecutor  ports.ParallelExecutor
	store             ports.StateStore
	logger            ports.Logger
	resolver          interpolation.Resolver
	evaluator         ports.ExpressionEvaluator
	hookExecutor      *HookExecutor
	loopExecutor      *LoopExecutor
	stdoutWriter      io.Writer
	stderrWriter      io.Writer
	historySvc        *HistoryService
	templateSvc       *TemplateService
	operationProvider ports.OperationProvider
	agentRegistry     ports.AgentRegistry
	conversationMgr   ConversationExecutor // F033: Multi-turn conversation orchestration (interface for testability)
	outputLimiter     *OutputLimiter       // C019: Prevent OOM from unbounded output accumulation
}

// SetOutputWriters configures streaming output writers.
func (s *ExecutionService) SetOutputWriters(stdout, stderr io.Writer) {
	s.stdoutWriter = stdout
	s.stderrWriter = stderr
}

// SetTemplateService configures the template service for expanding template references.
func (s *ExecutionService) SetTemplateService(svc *TemplateService) {
	s.templateSvc = svc
}

// SetOperationProvider configures the plugin operation provider for F021.
// When set, operation-type steps can execute plugin-provided operations.
func (s *ExecutionService) SetOperationProvider(provider ports.OperationProvider) {
	s.operationProvider = provider
}

// SetAgentRegistry configures the agent registry for F039 agent step execution.
// When set, agent-type steps can execute AI provider operations.
func (s *ExecutionService) SetAgentRegistry(registry ports.AgentRegistry) {
	s.agentRegistry = registry
}

// SetEvaluator configures the expression evaluator for conditional transitions.
// When set, enables evaluation of "when" clauses in workflow transitions.
func (s *ExecutionService) SetEvaluator(evaluator ports.ExpressionEvaluator) {
	s.evaluator = evaluator
}

// SetConversationManager configures the conversation manager for F033 multi-turn conversations.
// When set, agent-type steps with mode: conversation can execute managed conversations.
// Accepts ConversationExecutor interface to allow dependency injection and testing with mocks.
func (s *ExecutionService) SetConversationManager(mgr ConversationExecutor) {
	s.conversationMgr = mgr
}

// NewExecutionService - historySvc can be nil to disable history recording.
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
		loopExecutor:     NewLoopExecutor(logger, nil, resolver),
		historySvc:       historySvc,
		outputLimiter:    NewOutputLimiter(workflow.DefaultOutputLimits()),
	}
}

// NewExecutionServiceWithEvaluator enables conditional transitions using the `when:` clause.
func NewExecutionServiceWithEvaluator(
	wfSvc *WorkflowService,
	executor ports.CommandExecutor,
	parallelExecutor ports.ParallelExecutor,
	store ports.StateStore,
	logger ports.Logger,
	resolver interpolation.Resolver,
	historySvc *HistoryService,
	evaluator ports.ExpressionEvaluator,
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
		loopExecutor:     NewLoopExecutor(logger, evaluator, resolver),
		historySvc:       historySvc,
		outputLimiter:    NewOutputLimiter(workflow.DefaultOutputLimits()),
	}
}

// Run executes a workflow by name with the given inputs.
func (s *ExecutionService) Run(
	ctx context.Context,
	workflowName string,
	inputs map[string]any,
) (*workflow.ExecutionContext, error) {
	return s.runWithCallStack(ctx, workflowName, inputs, nil)
}

// RunWithWorkflow executes a pre-loaded workflow with the given inputs.
// This avoids double-loading when the workflow has already been loaded (e.g., for input validation).
func (s *ExecutionService) RunWithWorkflow(
	ctx context.Context,
	wf *workflow.Workflow,
	inputs map[string]any,
) (*workflow.ExecutionContext, error) {
	return s.runWithCallStackAndWorkflow(ctx, "", wf, inputs, nil)
}

// runWithCallStack executes a workflow by name with an optional parent call stack.
// This is used internally by executeCallWorkflowStep to preserve circular detection.
func (s *ExecutionService) runWithCallStack(
	ctx context.Context,
	workflowName string,
	inputs map[string]any,
	parentCallStack []string,
) (*workflow.ExecutionContext, error) {
	return s.runWithCallStackAndWorkflow(ctx, workflowName, nil, inputs, parentCallStack)
}

// runWithCallStackAndWorkflow executes a workflow with an optional parent call stack.
// If wf is nil, loads the workflow by name. Otherwise uses the provided workflow.
//
//nolint:gocognit // Complexity 35: main execution loop handles state machine transitions, error handling, and hook execution. Refactoring would split tightly-coupled state management.
func (s *ExecutionService) runWithCallStackAndWorkflow(
	ctx context.Context,
	workflowName string,
	wf *workflow.Workflow,
	inputs map[string]any,
	parentCallStack []string,
) (*workflow.ExecutionContext, error) {
	// load workflow if not provided
	if wf == nil {
		var err error
		wf, err = s.workflowSvc.GetWorkflow(ctx, workflowName)
		if err != nil {
			return nil, fmt.Errorf("load workflow: %w", err)
		}
	}

	// expand template references in workflow steps
	if s.templateSvc != nil {
		if err := s.templateSvc.ExpandWorkflow(ctx, wf); err != nil {
			return nil, fmt.Errorf("expand templates: %w", err)
		}
	}

	// initialize execution context
	execCtx := workflow.NewExecutionContext(uuid.New().String(), wf.Name)
	execCtx.Status = workflow.StatusRunning

	// Inherit parent call stack for circular detection in sub-workflows
	if len(parentCallStack) > 0 {
		execCtx.CallStack = make([]string, len(parentCallStack))
		copy(execCtx.CallStack, parentCallStack)
	}

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
	if err := s.hookExecutor.ExecuteHooks(ctx, wf.Hooks.WorkflowStart, intCtx, true); err != nil {
		execCtx.Status = workflow.StatusFailed
		s.checkpoint(ctx, execCtx)
		s.recordHistory(execCtx)
		return execCtx, fmt.Errorf("workflow_start hook failed: %w", err)
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
			// Check terminal status: failure or success (default)
			if step.Status == workflow.TerminalFailure {
				execCtx.Status = workflow.StatusFailed
				execErr = fmt.Errorf("workflow reached terminal failure state: %s", currentStep)
			} else {
				execCtx.Status = workflow.StatusCompleted
			}
			execCtx.CompletedAt = time.Now()
			s.checkpoint(ctx, execCtx)
			s.recordHistory(execCtx)
			s.logger.Info("workflow completed", "step", currentStep, "status", execCtx.Status)
			break
		}

		// execute step based on type
		var nextStep string
		var err error

		s.logger.Debug("executing step", "step", step.Name)

		switch step.Type {
		case workflow.StepTypeParallel:
			nextStep, err = s.executeParallelStep(ctx, wf, step, execCtx)
		case workflow.StepTypeForEach, workflow.StepTypeWhile:
			nextStep, err = s.executeLoopStep(ctx, wf, step, execCtx)
		case workflow.StepTypeOperation:
			nextStep, err = s.executePluginOperation(ctx, step, execCtx)
		case workflow.StepTypeCallWorkflow:
			nextStep, err = s.executeCallWorkflowStep(ctx, wf, step, execCtx)
		case workflow.StepTypeAgent:
			nextStep, err = s.executeAgentStep(ctx, step, execCtx)
		default:
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
			s.checkpoint(hookCtx, execCtx)
			s.recordHistory(execCtx)
			if err := s.hookExecutor.ExecuteHooks(hookCtx, wf.Hooks.WorkflowCancel, intCtx, false); err != nil {
				s.logger.Warn("workflow_cancel hook failed", "error", err)
			}
			return execCtx, execErr
		}

		// regular error - execute error hook
		intCtx.Error = &interpolation.ErrorData{
			Type:    classifyErrorType(execErr),
			Message: execErr.Error(),
			State:   execCtx.CurrentStep,
		}
		if err := s.hookExecutor.ExecuteHooks(hookCtx, wf.Hooks.WorkflowError, intCtx, false); err != nil {
			s.logger.Warn("workflow_error hook failed", "error", err)
		}
		return execCtx, execErr
	}

	// workflow completed successfully
	if err := s.hookExecutor.ExecuteHooks(hookCtx, wf.Hooks.WorkflowEnd, intCtx, false); err != nil {
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

	// T007: Prepare step execution (timeout, pre-hooks, interpolation)
	stepCtx, intCtx, cancel, err := s.prepareStepExecution(ctx, step, execCtx)
	if cancel != nil {
		defer cancel()
	}
	if err != nil {
		return "", err
	}

	// T007: Resolve command and directory
	cmd, err := s.resolveStepCommand(step, intCtx)
	if err != nil {
		return "", err
	}

	// T008: Execute command (with retry if configured)
	result, attempt, execErr := s.executeStepCommand(stepCtx, step, cmd)

	// T008: Record step state (timing, output, attempt)
	state := s.recordStepResult(step, startTime, result, attempt)

	// T009: Determine outcome and return next step
	if execErr != nil {
		return s.handleExecutionError(ctx, stepCtx, step, execCtx, &state, execErr)
	}

	if result.ExitCode != 0 {
		return s.handleNonZeroExit(stepCtx, step, execCtx, &state, result)
	}

	return s.handleSuccess(stepCtx, step, execCtx, &state)
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
			return s.evaluator.EvaluateBool(expr, intCtx)
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
	allStates := execCtx.GetAllStepStates()
	if len(allStates) > 0 {
		var lastState workflow.StepState
		var foundLast bool
		for name := range allStates {
			state := allStates[name]
			if !foundLast || state.CompletedAt.After(lastState.CompletedAt) {
				lastState = state
				foundLast = true
			}
		}
		if foundLast {
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

// buildLoopDataChain recursively converts domain LoopContext to interpolation LoopData.
// This enables {{.loop.Parent.*}} template access for nested loops.
// F043: Nested Loop Execution
func buildLoopDataChain(domainLoop *workflow.LoopContext) *interpolation.LoopData {
	if domainLoop == nil {
		return nil
	}
	return &interpolation.LoopData{
		Item:   domainLoop.Item,
		Index:  domainLoop.Index,
		First:  domainLoop.First,
		Last:   domainLoop.Last,
		Length: domainLoop.Length,
		Parent: buildLoopDataChain(domainLoop.Parent), // Recursive for parent chain
	}
}

// buildInterpolationContext converts ExecutionContext to interpolation.Context.
func (s *ExecutionService) buildInterpolationContext(
	execCtx *workflow.ExecutionContext,
) *interpolation.Context {
	// Convert step states - use thread-safe method
	allStates := execCtx.GetAllStepStates()
	states := make(map[string]interpolation.StepStateData, len(allStates))
	// Use explicit index iteration to avoid copying 184-byte StepState
	for name := range allStates {
		state := allStates[name]
		states[name] = interpolation.StepStateData{
			Output:     state.Output,
			Stderr:     state.Stderr,
			ExitCode:   state.ExitCode,
			Status:     state.Status.String(),
			Response:   state.Response,
			TokensUsed: state.TokensUsed,
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
		Error: nil, // Set in error hooks (F008)
	}

	// Include loop context if we're inside a loop (with parent chain for nested loops)
	intCtx.Loop = buildLoopDataChain(execCtx.CurrentLoop)

	return intCtx
}

// executeWithRetry executes a command with retry logic.
// Returns the final result, number of attempts made, and error.
func (s *ExecutionService) executeWithRetry(
	ctx context.Context,
	step *workflow.Step,
	cmd *ports.Command,
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
	retryer := retry.NewRetryer(&retryCfg, &retryLoggerAdapter{s.logger}, time.Now().UnixNano())

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
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Pre, intCtx, false); err != nil {
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
			switch {
			case branchResult.Error != nil:
				state.Status = workflow.StatusFailed
				state.Error = branchResult.Error.Error()
			case branchResult.ExitCode != 0:
				state.Status = workflow.StatusFailed
			default:
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
		if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
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
	if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
	}

	// Resolve next step using transitions (if defined) or fallback to OnSuccess
	return s.resolveNextStep(step, intCtx, true)
}

// executeLoopStep executes a for_each or while loop step.
//
//nolint:gocognit // Complexity 35: loop executor handles iteration, loop state, break/continue, and error cases. Complexity inherent to loop semantics.
func (s *ExecutionService) executeLoopStep(
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
	intCtx := s.buildInterpolationContext(execCtx)
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Pre, intCtx, false); err != nil {
		s.logger.Warn("pre-hook failed", "step", step.Name, "error", err)
	}

	// Create step executor callback that executes body steps
	// Supports nested loops: body steps can be loops themselves (F043)
	// F048: Updated to return (nextStep, error) to support transitions within loop body
	stepExecutor := func(ctx context.Context, stepName string, loopIntCtx *interpolation.Context) (string, error) {
		bodyStep, ok := wf.Steps[stepName]
		if !ok {
			return "", fmt.Errorf("body step not found: %s", stepName)
		}
		// Handle nested loops and special step types in body
		var nextStep string
		var err error
		switch bodyStep.Type {
		case workflow.StepTypeForEach, workflow.StepTypeWhile:
			nextStep, err = s.executeLoopStep(ctx, wf, bodyStep, execCtx)
		case workflow.StepTypeParallel:
			nextStep, err = s.executeParallelStep(ctx, wf, bodyStep, execCtx)
		case workflow.StepTypeCallWorkflow:
			nextStep, err = s.executeCallWorkflowStep(ctx, wf, bodyStep, execCtx)
		case workflow.StepTypeAgent:
			nextStep, err = s.executeAgentStep(ctx, bodyStep, execCtx)
		default:
			nextStep, err = s.executeStep(ctx, wf, bodyStep, execCtx)
		}
		if err != nil {
			return "", err
		}
		// Distinguish retry vs escape patterns:
		// - Retry: on_failure returns to THIS loop (step.Name) → continue loop
		// - Escape: on_failure transitions ELSEWHERE → propagate failure to break loop
		if nextStep != "" && nextStep != step.Name {
			// Step wanted to transition elsewhere while in failed state - escape pattern
			// Skip escape detection for continue_on_error steps (they intentionally proceed despite failure)
			if !bodyStep.ContinueOnError {
				if state, exists := execCtx.GetStepState(stepName); exists && state.Status == workflow.StatusFailed {
					if state.Error != "" {
						return "", fmt.Errorf("step %s failed: %s", stepName, state.Error)
					}
					return "", fmt.Errorf("step %s failed with exit code %d", stepName, state.ExitCode)
				}
			}
		}
		// F048: Return nextStep to enable transition handling in loop executor
		return nextStep, nil
	}

	// Execute loop based on type
	var result *workflow.LoopResult
	var err error

	s.logger.Info("executing loop step", "step", step.Name, "type", step.Loop.Type)

	if step.Type == workflow.StepTypeForEach {
		result, err = s.loopExecutor.ExecuteForEach(
			stepCtx, wf, step, execCtx, stepExecutor, s.buildInterpolationContext)
	} else {
		result, err = s.loopExecutor.ExecuteWhile(
			stepCtx, wf, step, execCtx, stepExecutor, s.buildInterpolationContext)
	}

	// Record loop step state
	loopState := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
	}

	if err != nil {
		// Check if the PARENT context was cancelled
		if ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
			loopState.Status = workflow.StatusFailed
			loopState.Error = err.Error()
			execCtx.SetStepState(step.Name, loopState)
			return "", fmt.Errorf("loop step %s: %w", step.Name, err)
		}

		loopState.Status = workflow.StatusFailed
		loopState.Error = err.Error()
		execCtx.SetStepState(step.Name, loopState)

		// Execute post-hooks even on failure
		intCtx = s.buildInterpolationContext(execCtx)
		if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
			s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
		}

		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		return "", err
	}

	// F048: Check if loop hit iteration limit with problematic patterns
	// While loops that run to MaxIterations with step failures or complex nesting
	// indicate potential infinite loop patterns that should be caught.
	if s.IsProblematicMaxIterationPattern(result, step, wf) {
		return s.HandleMaxIterationFailure(stepCtx, result, step, wf, execCtx, &loopState)
	}

	loopState.Status = workflow.StatusCompleted
	if result != nil {
		loopState.Output = fmt.Sprintf("completed %d iterations", result.TotalCount)
	}
	execCtx.SetStepState(step.Name, loopState)

	// Execute post-hooks
	intCtx = s.buildInterpolationContext(execCtx)
	if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
	}

	// F048 T007: Return nextStep from loop result if early exit occurred
	if result != nil && result.NextStep != "" {
		return result.NextStep, nil
	}

	return step.Loop.OnComplete, nil
}

// detectLoopPatterns examines loop iterations to detect step failures and complex step types.
// This helper extracts shared logic used by IsProblematicMaxIterationPattern and HandleMaxIterationFailure.
// Returns two booleans: hadFailures (true if any step failed), hasComplexSteps (true if loop contains while/foreach/parallel/callworkflow).
func (s *ExecutionService) detectLoopPatterns(result *workflow.LoopResult, wf *workflow.Workflow) (hadFailures, hasComplexSteps bool) {
	// Handle nil inputs gracefully
	if result == nil || result.Iterations == nil {
		return false, false
	}

	// Iterate through all iterations looking for failures and complex steps
	for _, iteration := range result.Iterations {
		// Skip iterations with nil step results
		if iteration.StepResults == nil {
			continue
		}

		// Check each step result in this iteration
		for stepName, stepState := range iteration.StepResults {
			// Check for failures
			if stepState != nil && stepState.Status == workflow.StatusFailed {
				hadFailures = true
			}

			// Check for complex step types - requires workflow definition
			if wf != nil && wf.Steps != nil {
				if stepDef, exists := wf.Steps[stepName]; exists {
					if isComplexStepType(stepDef.Type) {
						hasComplexSteps = true
					}
				}
			}

			// Early exit optimization: if both conditions found, no need to continue
			if hadFailures && hasComplexSteps {
				return true, true
			}
		}
	}

	return hadFailures, hasComplexSteps
}

// isComplexStepType returns true if the step type represents a complex/nested structure
// that makes loop debugging difficult (while, foreach, parallel, call_workflow).
func isComplexStepType(stepType workflow.StepType) bool {
	switch stepType {
	case workflow.StepTypeWhile,
		workflow.StepTypeForEach,
		workflow.StepTypeParallel,
		workflow.StepTypeCallWorkflow:
		return true
	default:
		return false
	}
}

// shouldCheckLoopProblems determines if we should analyze a loop for problematic patterns.
// Returns true if the loop is a while loop with max iterations that completed by hitting the limit (didn't break early).
func (s *ExecutionService) shouldCheckLoopProblems(result *workflow.LoopResult, step *workflow.Step) bool {
	// Only check while loops with max iterations configured
	if result == nil || step.Type != workflow.StepTypeWhile || step.Loop.MaxIterations <= 0 {
		return false
	}

	// Check if loop completed by hitting max iterations (didn't break early)
	if result.TotalCount < step.Loop.MaxIterations || result.BrokeAt != -1 {
		return false
	}

	return true
}

// buildLoopFailureError constructs an error message based on detected loop patterns.
// It appends " with step failures" if failures occurred, or " with nested complexity" if complex steps exist.
func (s *ExecutionService) buildLoopFailureError(hadFailures, hasComplexSteps bool) string {
	errMsg := "loop reached maximum iterations"
	if hadFailures {
		errMsg += " with step failures"
	} else if hasComplexSteps {
		errMsg += " with nested complexity"
	}
	return errMsg
}

// executeLoopPostHooks executes post-failure hooks for a loop step.
// It logs a warning if hook execution fails but does not propagate the error.
func (s *ExecutionService) executeLoopPostHooks(ctx context.Context, step *workflow.Step, execCtx *workflow.ExecutionContext) {
	// Build interpolation context for hook execution
	intCtx := s.buildInterpolationContext(execCtx)

	// Execute post-hooks with failOnError to detect failures for logging purposes
	// (we still don't propagate the error to the caller)
	if err := s.hookExecutor.ExecuteHooks(ctx, step.Hooks.Post, intCtx, true); err != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", err)
	}
}

// prepareStepExecution sets up the execution context for a step, including timeout,
// interpolation context building, and pre-hook execution.
// Returns the step context (with timeout if configured) and interpolation context.
// NOTE: Caller is responsible for calling returned cancel function to prevent context leak.
func (s *ExecutionService) prepareStepExecution(
	ctx context.Context,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
) (stepCtx context.Context, intCtx *interpolation.Context, cancel context.CancelFunc, err error) {
	// apply step timeout
	stepCtx = ctx
	if step.Timeout > 0 {
		stepCtx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
	}

	// build interpolation context
	intCtx = s.buildInterpolationContext(execCtx)

	// execute pre-hooks
	if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Pre, intCtx, false); hookErr != nil {
		s.logger.Warn("pre-hook failed", "step", step.Name, "error", hookErr)
	}

	return stepCtx, intCtx, cancel, nil
}

// resolveStepCommand interpolates and resolves the step's command and directory,
// building a ports.Command struct ready for execution.
// Returns the fully resolved command or an interpolation error.
func (s *ExecutionService) resolveStepCommand(
	step *workflow.Step,
	intCtx *interpolation.Context,
) (*ports.Command, error) {
	// resolve command variables
	resolvedCmd, err := s.resolver.Resolve(step.Command, intCtx)
	if err != nil {
		return nil, fmt.Errorf("interpolate command: %w", err)
	}

	// resolve dir if specified
	resolvedDir := ""
	if step.Dir != "" {
		resolvedDir, err = s.resolver.Resolve(step.Dir, intCtx)
		if err != nil {
			return nil, fmt.Errorf("interpolate dir: %w", err)
		}
	}

	// build command with env for secret masking
	env := make(map[string]string)
	for k, v := range intCtx.Inputs {
		// Convert input values to strings for env map
		if strVal, ok := v.(string); ok {
			env[k] = strVal
		}
	}

	cmd := &ports.Command{
		Program: resolvedCmd,
		Dir:     resolvedDir,
		Env:     env,
		Timeout: step.Timeout,
		Stdout:  s.stdoutWriter,
		Stderr:  s.stderrWriter,
	}

	return cmd, nil
}

// executeStepCommand executes the command with retry logic if configured.
// Returns the command result, attempt number, and any execution error.
func (s *ExecutionService) executeStepCommand(
	ctx context.Context,
	step *workflow.Step,
	cmd *ports.Command,
) (*ports.CommandResult, int, error) {
	// Handle nil executor for testing (temporary for T010 RED phase tests)
	if s.executor == nil {
		return nil, 0, nil
	}

	if step.Retry != nil && step.Retry.MaxAttempts > 1 {
		return s.executeWithRetry(ctx, step, cmd)
	}

	result, err := s.executor.Execute(ctx, cmd)
	return result, 1, err
}

// recordStepResult builds a workflow.StepState from execution timing and command result.
// Returns the step state with populated fields (without Status, which is set by outcome handlers).
func (s *ExecutionService) recordStepResult(
	step *workflow.Step,
	startTime time.Time,
	result *ports.CommandResult,
	attempt int,
) workflow.StepState {
	state := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
		Attempt:     attempt,
	}

	if result != nil {
		// C019: Apply output limits to prevent OOM from large outputs
		limitResult, err := s.outputLimiter.Apply(result.Stdout, result.Stderr)
		if err != nil {
			// Log error but don't fail the step - store raw output
			s.logger.Error("Failed to apply output limits", map[string]interface{}{
				"step":  step.Name,
				"error": err.Error(),
			})
			state.Output = result.Stdout
			state.Stderr = result.Stderr
		} else {
			state.Output = limitResult.Output
			state.Stderr = limitResult.Stderr
			state.OutputPath = limitResult.OutputPath
			state.StderrPath = limitResult.StderrPath
			state.Truncated = limitResult.Truncated
		}
		state.ExitCode = result.ExitCode
	}

	return state
}

// handleExecutionError processes execution errors, distinguishing between workflow-level
// cancellation (propagate) and step-level failures (follow OnFailure/ContinueOnError).
// Updates state, executes post-hooks, and returns the next step or error.
func (s *ExecutionService) handleExecutionError(
	ctx context.Context,
	stepCtx context.Context,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
	state *workflow.StepState,
	execErr error,
) (string, error) {
	// Check if the PARENT context was cancelled (workflow-level cancellation)
	// This is different from step timeout (stepCtx deadline exceeded)
	// Step timeout should follow OnFailure, workflow cancellation should propagate
	// When workflow is cancelled, propagate error immediately regardless of execErr type
	if ctx.Err() != nil {
		state.Status = workflow.StatusFailed
		state.Error = execErr.Error()
		execCtx.SetStepState(step.Name, *state)
		return "", fmt.Errorf("step %s: %w", step.Name, execErr)
	}

	state.Status = workflow.StatusFailed
	state.Error = execErr.Error()
	execCtx.SetStepState(step.Name, *state)

	// execute post-hooks even on failure
	intCtx := s.buildInterpolationContext(execCtx)
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); err != nil {
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

// handleNonZeroExit processes non-zero exit codes, following OnFailure or ContinueOnError paths.
// Updates state, executes post-hooks, and returns the next step or error.
func (s *ExecutionService) handleNonZeroExit(
	stepCtx context.Context,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
	state *workflow.StepState,
	result *ports.CommandResult,
) (string, error) {
	state.Status = workflow.StatusFailed
	// Include stderr in error if available, otherwise just exit code
	if result.Stderr != "" {
		state.Error = result.Stderr
	} else {
		state.Error = fmt.Sprintf("exit code %d", result.ExitCode)
	}
	execCtx.SetStepState(step.Name, *state)

	// execute post-hooks even on failure
	intCtx := s.buildInterpolationContext(execCtx)
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); err != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", err)
	}

	// If continue_on_error is true, follow on_success path
	if step.ContinueOnError {
		return step.OnSuccess, nil
	}

	if step.OnFailure != "" {
		return step.OnFailure, nil
	}

	// Return error with stderr if available for better error classification
	if result.Stderr != "" {
		return "", fmt.Errorf("step %s: %s", step.Name, result.Stderr)
	}
	return "", fmt.Errorf("step %s: exit code %d", step.Name, result.ExitCode)
}

// handleSuccess processes successful step execution, executing post-hooks and resolving
// the next step via transitions or OnSuccess.
// Returns the next step name or error.
func (s *ExecutionService) handleSuccess(
	stepCtx context.Context,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
	state *workflow.StepState,
) (string, error) {
	state.Status = workflow.StatusCompleted
	execCtx.SetStepState(step.Name, *state)

	// execute post-hooks on success
	intCtx := s.buildInterpolationContext(execCtx)
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); err != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", err)
	}

	// Resolve next step using transitions (if defined) or fallback to OnSuccess
	return s.resolveNextStep(step, intCtx, true)
}

// stepExecutorAdapter adapts ExecutionService to the ports.StepExecutor interface.
// IsProblematicMaxIterationPattern checks if a loop hit max iterations with problematic patterns.
// Returns true if the loop completed by hitting max iterations AND has step failures or complex nesting.
func (s *ExecutionService) IsProblematicMaxIterationPattern(
	result *workflow.LoopResult,
	step *workflow.Step,
	wf *workflow.Workflow,
) bool {
	// Guard clause: only check while loops with max iterations configured
	if !s.shouldCheckLoopProblems(result, step) {
		return false
	}

	// Check for step failures or complex body steps using extracted helper
	hadFailures, hasComplexSteps := s.detectLoopPatterns(result, wf)

	return hadFailures || hasComplexSteps
}

// HandleMaxIterationFailure handles the case when a loop hits max iterations with problematic patterns.
// If OnComplete is configured, completes successfully via that transition (e.g., retry patterns).
// Otherwise, treats as failure and returns error or transitions via OnFailure.
func (s *ExecutionService) HandleMaxIterationFailure(
	ctx context.Context,
	result *workflow.LoopResult,
	step *workflow.Step,
	wf *workflow.Workflow,
	execCtx *workflow.ExecutionContext,
	loopState *workflow.StepState,
) (string, error) {
	// Use extracted helper to determine pattern type
	hadFailures, hasComplexSteps := s.detectLoopPatterns(result, wf)

	// If OnComplete is configured, treat max iterations as successful completion
	// This supports retry patterns where on_failure loops back until max iterations
	if step.Loop != nil && step.Loop.OnComplete != "" {
		loopState.Status = workflow.StatusCompleted
		if result != nil {
			loopState.Output = fmt.Sprintf("completed %d iterations", result.TotalCount)
		}
		execCtx.SetStepState(step.Name, *loopState)

		// Use extracted helper to execute post-hooks
		s.executeLoopPostHooks(ctx, step, execCtx)

		return step.Loop.OnComplete, nil
	}

	// No OnComplete configured - treat as failure
	loopState.Status = workflow.StatusFailed

	// Use extracted helper to build error message
	errMsg := s.buildLoopFailureError(hadFailures, hasComplexSteps)
	loopState.Error = errMsg
	execCtx.SetStepState(step.Name, *loopState)

	// Use extracted helper to execute post-hooks
	s.executeLoopPostHooks(ctx, step, execCtx)

	if step.OnFailure != "" {
		return step.OnFailure, nil
	}
	return "", fmt.Errorf("while loop %s: %s", step.Name, errMsg)
}

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

	// Execute the step using the appropriate execution method based on step type
	var err error
	switch step.Type {
	case workflow.StepTypeAgent:
		_, err = a.execSvc.executeAgentStep(ctx, step, execCtx)
	case workflow.StepTypeParallel:
		_, err = a.execSvc.executeParallelStep(ctx, wf, step, execCtx)
	case workflow.StepTypeForEach, workflow.StepTypeWhile:
		_, err = a.execSvc.executeLoopStep(ctx, wf, step, execCtx)
	case workflow.StepTypeOperation:
		_, err = a.execSvc.executePluginOperation(ctx, step, execCtx)
	case workflow.StepTypeCallWorkflow:
		_, err = a.execSvc.executeCallWorkflowStep(ctx, wf, step, execCtx)
	default:
		_, err = a.execSvc.executeStep(ctx, wf, step, execCtx)
	}

	result.CompletedAt = time.Now()

	// Get the step state that was recorded by the execution method
	if state, exists := execCtx.GetStepState(stepName); exists {
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
	if err := s.hookExecutor.ExecuteHooks(ctx, wf.Hooks.WorkflowStart, intCtx, true); err != nil {
		execCtx.Status = workflow.StatusFailed
		s.checkpoint(ctx, execCtx)
		s.recordHistory(execCtx)
		return execCtx, fmt.Errorf("workflow_start hook failed: %w", err)
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
			// Check terminal status: failure or success (default)
			if step.Status == workflow.TerminalFailure {
				execCtx.Status = workflow.StatusFailed
				execErr = fmt.Errorf("workflow reached terminal failure state: %s", currentStep)
			} else {
				execCtx.Status = workflow.StatusCompleted
			}
			execCtx.CompletedAt = time.Now()
			s.checkpoint(ctx, execCtx)
			s.recordHistory(execCtx)
			s.logger.Info("workflow completed", "step", currentStep, "status", execCtx.Status)
			break
		}

		// execute step based on type
		var nextStep string
		var err error

		s.logger.Debug("executing step", "step", step.Name)

		switch step.Type {
		case workflow.StepTypeParallel:
			nextStep, err = s.executeParallelStep(ctx, wf, step, execCtx)
		case workflow.StepTypeForEach, workflow.StepTypeWhile:
			nextStep, err = s.executeLoopStep(ctx, wf, step, execCtx)
		case workflow.StepTypeOperation:
			nextStep, err = s.executePluginOperation(ctx, step, execCtx)
		case workflow.StepTypeCallWorkflow:
			nextStep, err = s.executeCallWorkflowStep(ctx, wf, step, execCtx)
		case workflow.StepTypeAgent:
			nextStep, err = s.executeAgentStep(ctx, step, execCtx)
		default:
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
			s.checkpoint(hookCtx, execCtx)
			s.recordHistory(execCtx)
			if err := s.hookExecutor.ExecuteHooks(hookCtx, wf.Hooks.WorkflowCancel, intCtx, false); err != nil {
				s.logger.Warn("workflow_cancel hook failed", "error", err)
			}
			return execCtx, execErr
		}

		// regular error - execute error hook
		intCtx.Error = &interpolation.ErrorData{
			Type:    classifyErrorType(execErr),
			Message: execErr.Error(),
			State:   execCtx.CurrentStep,
		}
		if err := s.hookExecutor.ExecuteHooks(hookCtx, wf.Hooks.WorkflowError, intCtx, false); err != nil {
			s.logger.Warn("workflow_error hook failed", "error", err)
		}
		return execCtx, execErr
	}

	// workflow completed successfully
	if err := s.hookExecutor.ExecuteHooks(hookCtx, wf.Hooks.WorkflowEnd, intCtx, false); err != nil {
		s.logger.Warn("workflow_end hook failed", "error", err)
	}
	return execCtx, nil
}

// ErrNoOperationProvider is returned when an operation step is executed without a configured provider.
var ErrNoOperationProvider = errors.New("operation provider not configured")

// executePluginOperation executes a plugin-provided operation step.
// F021: Plugin System - Executes operations registered by plugins.
// Returns ErrNoOperationProvider if no operation provider is configured.
//
//nolint:gocognit // Complexity 31: plugin operation executor handles validation, resolution, state management. Plugin interface requires comprehensive error handling.
func (s *ExecutionService) executePluginOperation(
	ctx context.Context,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
) (string, error) {
	startTime := time.Now()

	// Validate provider is configured
	if s.operationProvider == nil {
		return "", fmt.Errorf("step %s: %w", step.Name, ErrNoOperationProvider)
	}

	// Validate operation exists
	_, found := s.operationProvider.GetOperation(step.Operation)
	if !found {
		return "", fmt.Errorf("step %s: operation %q not found", step.Name, step.Operation)
	}

	// Apply step timeout
	stepCtx := ctx
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
		defer cancel()
	}

	// Build interpolation context
	intCtx := s.buildInterpolationContext(execCtx)

	// Execute pre-hooks
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Pre, intCtx, false); err != nil {
		s.logger.Warn("pre-hook failed", "step", step.Name, "error", err)
	}

	// Resolve operation inputs via interpolation
	resolvedInputs, err := s.resolveOperationInputs(step.OperationInputs, intCtx)
	if err != nil {
		return "", fmt.Errorf("step %s: resolve inputs: %w", step.Name, err)
	}

	// Execute the operation
	s.logger.Debug("executing plugin operation", "step", step.Name, "operation", step.Operation)
	result, execErr := s.operationProvider.Execute(stepCtx, step.Operation, resolvedInputs)

	// Record step state
	state := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
		Attempt:     1,
	}

	// Serialize operation outputs to step output and Response (for interpolation)
	if result != nil && result.Outputs != nil {
		state.Output = s.serializeOperationOutputs(result.Outputs)
		state.Response = result.Outputs
	}

	// Handle execution error (e.g., context cancelled, provider error)
	if execErr != nil {
		// Check if parent context was cancelled (workflow-level cancellation)
		if ctx.Err() != nil && (errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded)) {
			state.Status = workflow.StatusFailed
			state.Error = execErr.Error()
			execCtx.SetStepState(step.Name, state)
			return "", fmt.Errorf("step %s: %w", step.Name, execErr)
		}

		state.Status = workflow.StatusFailed
		state.Error = execErr.Error()
		s.logger.Warn("operation failed", "step", step.Name, "operation", step.Operation, "error", execErr)
		execCtx.SetStepState(step.Name, state)

		// Execute post-hooks even on failure
		intCtx = s.buildInterpolationContext(execCtx)
		if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
			s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
		}

		if step.ContinueOnError {
			return step.OnSuccess, nil
		}
		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		return "", fmt.Errorf("step %s: %w", step.Name, execErr)
	}

	// Handle operation failure (Success=false in result)
	if result != nil && !result.Success {
		state.Status = workflow.StatusFailed
		if result.Error != "" {
			state.Error = result.Error
		}
		s.logger.Warn("operation returned failure", "step", step.Name, "operation", step.Operation, "error", result.Error)
		execCtx.SetStepState(step.Name, state)

		// Execute post-hooks even on failure
		intCtx = s.buildInterpolationContext(execCtx)
		if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
			s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
		}

		if step.ContinueOnError {
			return step.OnSuccess, nil
		}
		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		errMsg := "operation failed"
		if result.Error != "" {
			errMsg = result.Error
		}
		return "", fmt.Errorf("step %s: %s", step.Name, errMsg)
	}

	// Success
	state.Status = workflow.StatusCompleted
	execCtx.SetStepState(step.Name, state)

	// Execute post-hooks on success
	intCtx = s.buildInterpolationContext(execCtx)
	if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
	}

	// Resolve next step using transitions or OnSuccess
	return s.resolveNextStep(step, intCtx, true)
}

// ErrNoAgentRegistry is returned when an agent step is executed without a configured registry.
var ErrNoAgentRegistry = errors.New("agent registry not configured")

// executeAgentStep executes an AI agent step.
// F039: Agent Step Type - Executes AI provider operations via CLI interfaces.
// Returns ErrNoAgentRegistry if no agent registry is configured.
//
//nolint:gocognit // Complexity 39: agent step execution handles conversation/single modes, retries, context windows, token management. Inherent to agent orchestration.
func (s *ExecutionService) executeAgentStep(
	ctx context.Context,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
) (string, error) {
	startTime := time.Now()

	// Validate registry is configured
	if s.agentRegistry == nil {
		// Record failed state before returning error to maintain consistent state tracking
		state := workflow.StepState{
			Name:        step.Name,
			StartedAt:   startTime,
			CompletedAt: time.Now(),
			Status:      workflow.StatusFailed,
			Error:       ErrNoAgentRegistry.Error(),
			Attempt:     1,
		}
		execCtx.SetStepState(step.Name, state)
		return "", fmt.Errorf("step %s: %w", step.Name, ErrNoAgentRegistry)
	}

	// Validate agent config exists
	if step.Agent == nil {
		return "", fmt.Errorf("step %s: agent configuration missing", step.Name)
	}

	// Apply step timeout (use agent timeout if step timeout not set)
	stepCtx := ctx
	timeout := step.Timeout
	if timeout == 0 && step.Agent.Timeout > 0 {
		timeout = step.Agent.Timeout
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	// Build interpolation context
	intCtx := s.buildInterpolationContext(execCtx)

	// Execute pre-hooks
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Pre, intCtx, false); err != nil {
		s.logger.Warn("pre-hook failed", "step", step.Name, "error", err)
	}

	// F033: Route to conversation execution if mode is "conversation"
	if step.Agent.Mode == "conversation" {
		return s.executeConversationStep(stepCtx, step, execCtx)
	}

	// Resolve prompt via interpolation
	resolvedPrompt, err := s.resolver.Resolve(step.Agent.Prompt, intCtx)
	if err != nil {
		return "", fmt.Errorf("step %s: resolve prompt: %w", step.Name, err)
	}

	// Get provider from registry
	provider, err := s.agentRegistry.Get(step.Agent.Provider)
	if err != nil {
		return "", fmt.Errorf("step %s: %w", step.Name, err)
	}

	// Audit log if dangerouslySkipPermissions is enabled
	if step.Agent.Options != nil {
		if skipPerms, ok := step.Agent.Options["dangerouslySkipPermissions"].(bool); ok && skipPerms {
			s.logger.Warn("dangerouslySkipPermissions enabled",
				"workflow", execCtx.WorkflowName,
				"step", step.Name,
				"provider", step.Agent.Provider,
				"timestamp", time.Now().Format(time.RFC3339))
		}
	}

	// Execute the agent
	s.logger.Debug("executing agent step", "step", step.Name, "provider", step.Agent.Provider)
	result, execErr := provider.Execute(stepCtx, resolvedPrompt, step.Agent.Options)

	// Record step state
	state := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
		Attempt:     1,
	}

	// Populate state from result
	if result != nil {
		state.Output = result.Output
		// AC5: JSON auto-parsed to states.step_name.Response
		state.Response = result.Response
		// AC6: Token usage in states.step_name.tokens_used
		state.TokensUsed = result.Tokens
	}

	// Handle execution error (e.g., context cancelled, provider error)
	if execErr != nil {
		// Check if parent context was cancelled (workflow-level cancellation)
		if ctx.Err() != nil && (errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded)) {
			state.Status = workflow.StatusFailed
			state.Error = execErr.Error()
			if result != nil {
				state.Response = result.Response
				state.TokensUsed = result.Tokens
			}
			execCtx.SetStepState(step.Name, state)
			return "", fmt.Errorf("step %s: %w", step.Name, execErr)
		}

		state.Status = workflow.StatusFailed
		state.Error = execErr.Error()
		if result != nil {
			state.Response = result.Response
			state.TokensUsed = result.Tokens
		}
		execCtx.SetStepState(step.Name, state)

		// Execute post-hooks even on failure
		intCtx = s.buildInterpolationContext(execCtx)
		if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
			s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
		}

		if step.ContinueOnError {
			return step.OnSuccess, nil
		}
		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		return "", fmt.Errorf("step %s: %w", step.Name, execErr)
	}

	// Handle agent result error
	if result != nil && result.Error != nil {
		state.Status = workflow.StatusFailed
		state.Error = result.Error.Error()
		execCtx.SetStepState(step.Name, state)

		// Execute post-hooks even on failure
		intCtx = s.buildInterpolationContext(execCtx)
		if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
			s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
		}

		if step.ContinueOnError {
			return step.OnSuccess, nil
		}
		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		return "", fmt.Errorf("step %s: agent error: %w", step.Name, result.Error)
	}

	// Success
	state.Status = workflow.StatusCompleted
	execCtx.SetStepState(step.Name, state)

	// Execute post-hooks on success
	intCtx = s.buildInterpolationContext(execCtx)
	if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
	}

	// Resolve next step using transitions or OnSuccess
	return s.resolveNextStep(step, intCtx, true)
}

// executeConversationStep orchestrates a multi-turn agent conversation following F033 spec.
// It delegates to ConversationManager which handles:
// - Turn iteration with conversation history
// - Context window management (token counting, truncation strategies)
// - Stop condition evaluation (expression-based or max limits)
// - Conversation state persistence in step state
//
// Flow:
//  1. Validate conversation manager is configured
//  2. Extract conversation config from step.Agent.Conversation
//  3. Delegate to ConversationManager.ExecuteConversation
//  4. Map ConversationResult to StepState
//  5. Execute hooks and resolve next step
//
// F051: T009 - Implement delegation to ConversationManager
func (s *ExecutionService) executeConversationStep(
	ctx context.Context,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
) (string, error) {
	startTime := time.Now()

	// 1. Validate conversation manager is configured
	if s.conversationMgr == nil {
		return "", errors.New("conversation manager not configured")
	}

	// 2. Validate agent config exists
	if step.Agent == nil {
		return "", errors.New("agent config is nil")
	}

	// 3. Validate conversation config exists
	if step.Agent.Conversation == nil {
		return "", errors.New("conversation config is nil")
	}

	// 4. Create buildContext closure for interpolation
	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return s.buildInterpolationContext(ec)
	}

	// 5. Delegate to ConversationManager
	s.logger.Debug("executing conversation step", "step", step.Name, "provider", step.Agent.Provider)
	result, err := s.conversationMgr.ExecuteConversation(
		ctx,
		step,
		step.Agent.Conversation,
		execCtx,
		buildContext,
	)

	// 6. Map ConversationResult to StepState
	state := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
		Attempt:     1,
	}

	if result != nil {
		state.Output = result.Output
		state.Response = result.Response
		state.TokensUsed = result.TokensTotal
		state.Conversation = result.State
	}

	// 7. Handle execution error
	if err != nil {
		state.Status = workflow.StatusFailed
		state.Error = err.Error()
		execCtx.SetStepState(step.Name, state)
		return "", fmt.Errorf("conversation execution failed: %w", err)
	}

	// 8. Mark as completed
	state.Status = workflow.StatusCompleted
	execCtx.SetStepState(step.Name, state)

	// 9. Execute post-hooks on success
	intCtx := s.buildInterpolationContext(execCtx)
	if hookErr := s.hookExecutor.ExecuteHooks(ctx, step.Hooks.Post, intCtx, false); hookErr != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
	}

	// 10. Resolve next step using transitions or OnSuccess
	return s.resolveNextStep(step, intCtx, true)
}

// resolveOperationInputs resolves all string values in operation inputs via interpolation.
func (s *ExecutionService) resolveOperationInputs(
	inputs map[string]any,
	intCtx *interpolation.Context,
) (map[string]any, error) {
	if inputs == nil {
		return nil, nil
	}

	resolved := make(map[string]any, len(inputs))
	for key, value := range inputs {
		resolvedValue, err := s.resolveOperationValue(value, intCtx)
		if err != nil {
			return nil, fmt.Errorf("input %q: %w", key, err)
		}
		resolved[key] = resolvedValue
	}
	return resolved, nil
}

// resolveOperationValue recursively resolves interpolation in operation input values.
func (s *ExecutionService) resolveOperationValue(value any, intCtx *interpolation.Context) (any, error) {
	switch v := value.(type) {
	case string:
		return s.resolver.Resolve(v, intCtx)
	case map[string]any:
		resolved := make(map[string]any, len(v))
		for k, val := range v {
			resolvedVal, err := s.resolveOperationValue(val, intCtx)
			if err != nil {
				return nil, err
			}
			resolved[k] = resolvedVal
		}
		return resolved, nil
	case []any:
		resolved := make([]any, len(v))
		for i, val := range v {
			resolvedVal, err := s.resolveOperationValue(val, intCtx)
			if err != nil {
				return nil, err
			}
			resolved[i] = resolvedVal
		}
		return resolved, nil
	default:
		// Non-string primitives (int, bool, etc.) pass through unchanged
		return value, nil
	}
}

// serializeOperationOutputs converts operation outputs to a string for step state.
func (s *ExecutionService) serializeOperationOutputs(outputs map[string]any) string {
	if len(outputs) == 0 {
		return ""
	}
	// Simple key=value format for now
	var result strings.Builder
	first := true
	for k, v := range outputs {
		if !first {
			result.WriteString("\n")
		}
		result.WriteString(fmt.Sprintf("%s=%v", k, v))
		first = false
	}
	return result.String()
}

// classifyErrorType categorizes errors into types matching CLI exit code taxonomy.
// Returns: "execution", "workflow", "user", or "system"
func classifyErrorType(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "terminal failure"):
		return "workflow"
	case strings.Contains(errStr, "step not found"), strings.Contains(errStr, "invalid state"):
		return "workflow"
	case strings.Contains(errStr, "cycle detected"):
		return "workflow"
	case strings.Contains(errStr, "exit code"):
		return "execution"
	case strings.Contains(errStr, "timeout"), strings.Contains(errStr, "context deadline"):
		return "execution"
	case strings.Contains(errStr, "command failed"):
		return "execution"
	case strings.Contains(errStr, "not found"), strings.Contains(errStr, "missing"):
		return "user"
	case strings.Contains(errStr, "invalid input"), strings.Contains(errStr, "validation"):
		return "user"
	case strings.Contains(errStr, "permission"), strings.Contains(errStr, "access denied"):
		return "system"
	case strings.Contains(errStr, "IO error"), strings.Contains(errStr, "file system"):
		return "system"
	default:
		return "execution"
	}
}
