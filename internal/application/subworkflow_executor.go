package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// Sub-workflow execution errors.
var (
	// ErrCircularWorkflowCall is returned when a circular sub-workflow call is detected.
	ErrCircularWorkflowCall = errors.New("circular workflow call detected")

	// ErrMaxNestingExceeded is returned when sub-workflow nesting depth exceeds the maximum allowed.
	ErrMaxNestingExceeded = errors.New("maximum sub-workflow nesting depth exceeded")

	// ErrSubWorkflowNotFound is returned when a referenced sub-workflow does not exist.
	ErrSubWorkflowNotFound = errors.New("sub-workflow not found")
)

// executeCallWorkflowStep executes a call_workflow step that invokes another workflow as a sub-workflow.
//
// The method performs the following steps:
//  1. Validates call stack depth against MaxCallStackDepth
//  2. Checks for circular workflow calls using the call stack
//  3. Loads the sub-workflow via WorkflowService
//  4. Pushes current workflow to call stack
//  5. Resolves input mappings via interpolation
//  6. Creates child context with optional timeout
//  7. Executes sub-workflow recursively
//  8. Maps sub-workflow outputs to parent step state
//  9. Pops call stack
//  10. Returns next step based on success/failure
//
// This is a stub implementation for TDD - returns "not implemented" error.
//
//nolint:gocognit // Complexity 43: subworkflow execution handles input mapping, nested workflow loading, execution, output capture, error propagation. Subworkflow orchestration requires this.
func (s *ExecutionService) executeCallWorkflowStep(
	ctx context.Context,
	wf *workflow.Workflow,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
) (string, error) {
	startTime := time.Now()

	// Validate call_workflow configuration exists
	if step.CallWorkflow == nil {
		return "", fmt.Errorf("step %s: call_workflow configuration is required", step.Name)
	}

	config := step.CallWorkflow

	// 1. Check call stack depth
	if execCtx.CallStackDepth() >= workflow.MaxCallStackDepth {
		return "", fmt.Errorf("step %s: %w (depth: %d, max: %d)",
			step.Name, ErrMaxNestingExceeded, execCtx.CallStackDepth(), workflow.MaxCallStackDepth)
	}

	// 2. Check for circular workflow call
	if execCtx.IsInCallStack(config.Workflow) {
		return "", fmt.Errorf("step %s: %w: workflow %q is already in call stack %v",
			step.Name, ErrCircularWorkflowCall, config.Workflow, execCtx.CallStack)
	}

	// 3. Load sub-workflow
	_, err := s.workflowSvc.GetWorkflow(ctx, config.Workflow)
	if err != nil {
		return "", fmt.Errorf("step %s: load sub-workflow %q: %w", step.Name, config.Workflow, err)
	}

	// 4. Push current workflow to call stack
	execCtx.PushCallStack(wf.Name)
	defer execCtx.PopCallStack()

	// 5. Build interpolation context and resolve input mappings
	intCtx := s.buildInterpolationContext(execCtx)
	subInputs := make(map[string]any)
	for key, template := range config.Inputs {
		resolved, err := s.resolver.Resolve(template, intCtx)
		if err != nil {
			return "", fmt.Errorf("step %s: resolve input %q: %w", step.Name, key, err)
		}
		subInputs[key] = resolved
	}

	// 6. Apply timeout
	// Prefer step-level timeout (from YAML timeout: field) over config.Timeout
	subCtx := ctx
	timeout := step.Timeout
	if timeout <= 0 {
		timeout = config.GetTimeout()
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		subCtx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	// Execute pre-hooks
	if err := s.hookExecutor.ExecuteHooks(subCtx, step.Hooks.Pre, intCtx, false); err != nil {
		s.logger.Warn("pre-hook failed", "step", step.Name, "error", err)
	}

	// 7. Execute sub-workflow with parent call stack for circular detection
	s.logger.Info("executing sub-workflow", "step", step.Name, "workflow", config.Workflow)
	subResult, execErr := s.runWithCallStack(subCtx, config.Workflow, subInputs, execCtx.CallStack)

	// Create sub-workflow result for tracking
	result := workflow.NewSubWorkflowResult(config.Workflow)
	result.CompletedAt = time.Now()

	// Record step state
	state := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
		Attempt:     1,
	}

	// 8. Map sub-workflow outputs to parent state
	if subResult != nil && execErr == nil {
		for subOutputName, parentVarName := range config.Outputs {
			if subState, ok := subResult.States[subOutputName]; ok {
				result.Outputs[parentVarName] = subState.Output
			}
		}
		// Store output summary in step state
		if len(result.Outputs) > 0 {
			state.Output = fmt.Sprintf("sub-workflow %s completed successfully", config.Workflow)
		}
	}

	// Handle execution error
	if execErr != nil {
		result.Error = execErr

		// Check if parent context was cancelled (workflow-level cancellation)
		if ctx.Err() != nil && (errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded)) {
			state.Status = workflow.StatusFailed
			state.Error = execErr.Error()
			execCtx.SetStepState(step.Name, state)
			return "", fmt.Errorf("step %s: %w", step.Name, execErr)
		}

		// Circular and max nesting errors are fatal - they must propagate immediately
		// without going to on_failure, as they indicate a structural problem
		if errors.Is(execErr, ErrCircularWorkflowCall) || errors.Is(execErr, ErrMaxNestingExceeded) {
			state.Status = workflow.StatusFailed
			state.Error = execErr.Error()
			execCtx.SetStepState(step.Name, state)
			return "", execErr
		}

		state.Status = workflow.StatusFailed
		state.Error = execErr.Error()
		execCtx.SetStepState(step.Name, state)

		// Execute post-hooks even on failure
		intCtx = s.buildInterpolationContext(execCtx)
		if hookErr := s.hookExecutor.ExecuteHooks(subCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
			s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
		}

		if step.ContinueOnError {
			return step.OnSuccess, nil
		}
		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		return "", fmt.Errorf("step %s: sub-workflow %s failed: %w", step.Name, config.Workflow, execErr)
	}

	// Handle sub-workflow failure (completed but with failed status)
	if subResult != nil && subResult.Status == workflow.StatusFailed {
		state.Status = workflow.StatusFailed
		state.Error = "sub-workflow completed with failed status"
		execCtx.SetStepState(step.Name, state)

		// Execute post-hooks
		intCtx = s.buildInterpolationContext(execCtx)
		if hookErr := s.hookExecutor.ExecuteHooks(subCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
			s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
		}

		if step.ContinueOnError {
			return step.OnSuccess, nil
		}
		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		return "", fmt.Errorf("step %s: sub-workflow %s failed", step.Name, config.Workflow)
	}

	// Success
	state.Status = workflow.StatusCompleted
	execCtx.SetStepState(step.Name, state)

	// Execute post-hooks on success
	intCtx = s.buildInterpolationContext(execCtx)
	if hookErr := s.hookExecutor.ExecuteHooks(subCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
	}

	// Resolve next step using transitions or OnSuccess
	return s.resolveNextStep(step, intCtx, true)
}
