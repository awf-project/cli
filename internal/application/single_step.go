package application

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

// SingleStepResult holds the result of executing a single step.
type SingleStepResult struct {
	StepName    string
	Output      string
	Stderr      string
	ExitCode    int
	Status      workflow.ExecutionStatus
	Error       string
	StartedAt   time.Time
	CompletedAt time.Time
}

// ExecuteSingleStep executes a single step from a workflow in isolation.
// It bypasses the state machine and directly runs the specified step.
// Mocked states can be provided to simulate dependencies on previous steps.
func (s *ExecutionService) ExecuteSingleStep(
	ctx context.Context,
	workflowName string,
	stepName string,
	inputs map[string]any,
	mocks map[string]string,
) (*SingleStepResult, error) {
	// Load workflow
	wf, err := s.workflowSvc.GetWorkflow(ctx, workflowName)
	if err != nil {
		return nil, fmt.Errorf("load workflow: %w", err)
	}
	if wf == nil {
		return nil, fmt.Errorf("workflow not found: %s", workflowName)
	}

	// expand template references in workflow steps
	if s.templateSvc != nil {
		if err := s.templateSvc.ExpandWorkflow(ctx, wf); err != nil {
			return nil, fmt.Errorf("expand templates: %w", err)
		}
	}

	// Find step
	step, ok := wf.Steps[stepName]
	if !ok {
		return nil, fmt.Errorf("step not found: %s", stepName)
	}

	// Terminal steps cannot be executed
	if step.Type == workflow.StepTypeTerminal {
		return nil, fmt.Errorf("cannot execute terminal step: %s", stepName)
	}

	startTime := time.Now()
	result := &SingleStepResult{
		StepName:  stepName,
		StartedAt: startTime,
	}

	// Build interpolation context with inputs and mocked states
	intCtx := s.buildSingleStepInterpolationContext(wf.Name, inputs, mocks)

	// Apply step timeout
	stepCtx := ctx
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
		defer cancel()
	}

	// Execute pre-hooks
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Pre, intCtx); err != nil {
		s.logger.Warn("pre-hook failed", "step", step.Name, "error", err)
	}

	// Resolve command variables
	resolvedCmd, err := s.resolver.Resolve(step.Command, intCtx)
	if err != nil {
		result.CompletedAt = time.Now()
		result.Status = workflow.StatusFailed
		result.Error = fmt.Sprintf("interpolate command: %s", err)
		return result, fmt.Errorf("interpolate command: %w", err)
	}

	// Resolve dir if specified
	resolvedDir := ""
	if step.Dir != "" {
		resolvedDir, err = s.resolver.Resolve(step.Dir, intCtx)
		if err != nil {
			result.CompletedAt = time.Now()
			result.Status = workflow.StatusFailed
			result.Error = fmt.Sprintf("interpolate dir: %s", err)
			return result, fmt.Errorf("interpolate dir: %w", err)
		}
	}

	// Build and execute command
	cmd := ports.Command{
		Program: resolvedCmd,
		Dir:     resolvedDir,
		Timeout: step.Timeout,
		Stdout:  s.stdoutWriter,
		Stderr:  s.stderrWriter,
	}

	cmdResult, execErr := s.executor.Execute(stepCtx, cmd)

	result.CompletedAt = time.Now()

	if cmdResult != nil {
		result.Output = cmdResult.Stdout
		result.Stderr = cmdResult.Stderr
		result.ExitCode = cmdResult.ExitCode
	}

	// Execute post-hooks (always, even on failure)
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx); err != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", err)
	}

	// Determine outcome
	if execErr != nil {
		result.Status = workflow.StatusFailed
		result.Error = execErr.Error()
		return result, nil // Return result without error - let caller decide
	}

	if cmdResult != nil && cmdResult.ExitCode != 0 {
		result.Status = workflow.StatusFailed
		return result, nil // Return result without error - let caller decide
	}

	result.Status = workflow.StatusCompleted
	return result, nil
}

// buildSingleStepInterpolationContext creates an interpolation context for single step execution.
// It populates inputs and mocked states.
func (s *ExecutionService) buildSingleStepInterpolationContext(
	workflowName string,
	inputs map[string]any,
	mocks map[string]string,
) *interpolation.Context {
	// Parse mocked states from the mocks map
	// Format: "states.step_name.output" -> value
	states := make(map[string]interpolation.StepStateData)
	for key, value := range mocks {
		if strings.HasPrefix(key, "states.") {
			parts := strings.SplitN(key, ".", 3)
			if len(parts) >= 3 {
				stepName := parts[1]
				field := parts[2]

				// Get or create state entry
				state, exists := states[stepName]
				if !exists {
					state = interpolation.StepStateData{
						Status: "completed", // Mocked states are assumed completed
					}
				}

				// Set the field value
				switch field {
				case "output":
					state.Output = value
				case "stderr":
					state.Stderr = value
				case "status":
					state.Status = value
				}

				states[stepName] = state
			}
		}
	}

	// Get runtime context
	wd, _ := os.Getwd()
	hostname, _ := os.Hostname()

	// Build environment map
	env := make(map[string]string)
	for _, e := range os.Environ() {
		if parts := strings.SplitN(e, "=", 2); len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}

	return &interpolation.Context{
		Inputs: inputs,
		States: states,
		Workflow: interpolation.WorkflowData{
			ID:        uuid.New().String(),
			Name:      workflowName,
			StartedAt: time.Now(),
		},
		Env: env,
		Context: interpolation.ContextData{
			WorkingDir: wd,
			User:       os.Getenv("USER"),
			Hostname:   hostname,
		},
	}
}
