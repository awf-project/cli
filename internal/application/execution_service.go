package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// ExecutionService orchestrates workflow execution.
type ExecutionService struct {
	workflowSvc *WorkflowService
	executor    ports.CommandExecutor
	store       ports.StateStore
	logger      ports.Logger
}

// NewExecutionService creates a new execution service.
func NewExecutionService(
	wfSvc *WorkflowService,
	executor ports.CommandExecutor,
	store ports.StateStore,
	logger ports.Logger,
) *ExecutionService {
	return &ExecutionService{
		workflowSvc: wfSvc,
		executor:    executor,
		store:       store,
		logger:      logger,
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
	for k, v := range inputs {
		execCtx.SetInput(k, v)
	}

	s.logger.Info("starting workflow", "workflow", wf.Name, "id", execCtx.WorkflowID)

	// execution loop
	currentStep := wf.Initial
	for {
		step, ok := wf.Steps[currentStep]
		if !ok {
			execCtx.Status = workflow.StatusFailed
			return execCtx, fmt.Errorf("step not found: %s", currentStep)
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
		nextStep, err := s.executeStep(ctx, step, execCtx)
		if err != nil {
			execCtx.Status = workflow.StatusFailed
			s.logger.Error("step failed", "step", step.Name, "error", err)
			s.checkpoint(ctx, execCtx)
			return execCtx, err
		}

		// checkpoint after each step
		s.checkpoint(ctx, execCtx)

		currentStep = nextStep
	}

	return execCtx, nil
}

// executeStep executes a single step and returns the next step name.
func (s *ExecutionService) executeStep(
	ctx context.Context,
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

	// build command
	cmd := ports.Command{
		Program: step.Command,
		Timeout: step.Timeout,
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

		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		return "", fmt.Errorf("step %s: %w", step.Name, execErr)
	}

	if result.ExitCode != 0 {
		state.Status = workflow.StatusFailed
		execCtx.SetStepState(step.Name, state)

		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		return "", fmt.Errorf("step %s: exit code %d", step.Name, result.ExitCode)
	}

	state.Status = workflow.StatusCompleted
	execCtx.SetStepState(step.Name, state)
	return step.OnSuccess, nil
}

// checkpoint saves the current execution state.
// Failures are logged but not fatal - execution continues.
func (s *ExecutionService) checkpoint(ctx context.Context, execCtx *workflow.ExecutionContext) {
	if err := s.store.Save(ctx, execCtx); err != nil {
		s.logger.Warn("checkpoint failed", "workflow_id", execCtx.WorkflowID, "error", err)
	}
}
