package application

import (
	"context"
	"fmt"

	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

// DryRunExecutor walks through a workflow without executing commands.
// It produces an execution plan showing what would happen.
type DryRunExecutor struct {
	wfSvc       *WorkflowService
	resolver    interpolation.Resolver
	evaluator   ports.ExpressionEvaluator
	templateSvc *TemplateService
	logger      ports.Logger
}

// NewDryRunExecutor creates a new DryRunExecutor with the required dependencies.
func NewDryRunExecutor(
	wfSvc *WorkflowService,
	resolver interpolation.Resolver,
	evaluator ports.ExpressionEvaluator,
	logger ports.Logger,
) *DryRunExecutor {
	return &DryRunExecutor{
		wfSvc:     wfSvc,
		resolver:  resolver,
		evaluator: evaluator,
		logger:    logger,
	}
}

// SetTemplateService sets the template service for workflow expansion.
func (e *DryRunExecutor) SetTemplateService(svc *TemplateService) {
	e.templateSvc = svc
}

// Execute performs a dry-run of the workflow with the given inputs.
// It returns an execution plan without running any commands.
func (e *DryRunExecutor) Execute(ctx context.Context, workflowName string, inputs map[string]any) (*workflow.DryRunPlan, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("dry run cancelled: %w", ctx.Err())
	default:
	}

	// Load workflow
	wf, err := e.wfSvc.GetWorkflow(ctx, workflowName)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow: %w", err)
	}

	// Expand templates if template service is available
	if e.templateSvc != nil {
		if err := e.templateSvc.ExpandWorkflow(ctx, wf); err != nil {
			return nil, fmt.Errorf("failed to expand templates: %w", err)
		}
	}

	return e.buildPlan(ctx, wf, inputs)
}

// buildPlan walks through the workflow and builds the execution plan.
func (e *DryRunExecutor) buildPlan(ctx context.Context, wf *workflow.Workflow, inputs map[string]any) (*workflow.DryRunPlan, error) {
	// Resolve inputs with defaults and validation
	resolvedInputs, err := e.resolveInputs(wf, inputs)
	if err != nil {
		return nil, err
	}

	// Build interpolation context with resolved inputs
	interpCtx := interpolation.NewContext()
	for name, input := range resolvedInputs {
		interpCtx.Inputs[name] = input.Value
	}
	interpCtx.Workflow.Name = wf.Name

	// Build plan by walking through all steps (breadth-first from initial)
	plan := &workflow.DryRunPlan{
		WorkflowName: wf.Name,
		Description:  wf.Description,
		Inputs:       resolvedInputs,
		Steps:        make([]workflow.DryRunStep, 0),
	}

	// Track visited steps to avoid duplicates
	visited := make(map[string]bool)
	queue := []string{wf.Initial}

	for len(queue) > 0 {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("dry run cancelled: %w", ctx.Err())
		default:
		}

		stepName := queue[0]
		queue = queue[1:]

		if visited[stepName] {
			continue
		}
		visited[stepName] = true

		step, ok := wf.Steps[stepName]
		if !ok {
			continue
		}

		// Build plan for this step
		dryRunStep, err := e.buildStepPlan(step, interpCtx)
		if err != nil {
			return nil, fmt.Errorf("step '%s': %w", stepName, err)
		}
		plan.Steps = append(plan.Steps, *dryRunStep)

		// Add next steps to queue based on transitions
		nextSteps := e.collectNextSteps(step)
		queue = append(queue, nextSteps...)
	}

	return plan, nil
}

// collectNextSteps gathers all possible next step names from a step's transitions.
func (e *DryRunExecutor) collectNextSteps(step *workflow.Step) []string {
	var nextSteps []string
	seen := make(map[string]bool)

	addIfNew := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			nextSteps = append(nextSteps, name)
		}
	}

	// From Transitions (conditional)
	for _, tr := range step.Transitions {
		addIfNew(tr.Goto)
	}

	// From legacy OnSuccess/OnFailure
	addIfNew(step.OnSuccess)
	addIfNew(step.OnFailure)

	// From parallel branches
	for _, branch := range step.Branches {
		addIfNew(branch)
	}

	// From loop body and OnComplete
	if step.Loop != nil {
		for _, bodyStep := range step.Loop.Body {
			addIfNew(bodyStep)
		}
		addIfNew(step.Loop.OnComplete)
	}

	return nextSteps
}

// buildStepPlan creates a DryRunStep from a workflow step.
func (e *DryRunExecutor) buildStepPlan(step *workflow.Step, interpCtx *interpolation.Context) (*workflow.DryRunStep, error) {
	dryRunStep := &workflow.DryRunStep{
		Name:            step.Name,
		Type:            step.Type,
		Description:     step.Description,
		Dir:             step.Dir,
		Timeout:         step.Timeout,
		ContinueOnError: step.ContinueOnError,
		Branches:        step.Branches,
		Strategy:        step.Strategy,
		MaxConcurrent:   step.MaxConcurrent,
		Status:          step.Status,
	}

	// Resolve command with variable interpolation
	if step.Command != "" {
		dryRunStep.Command = e.resolveCommand(step.Command, interpCtx)
	}

	// Build hooks
	dryRunStep.Hooks = e.buildHooks(step.Hooks, interpCtx)

	// Build transitions
	dryRunStep.Transitions = e.buildTransitions(step)

	// Build retry config
	if step.Retry != nil {
		dryRunStep.Retry = &workflow.DryRunRetry{
			MaxAttempts:    step.Retry.MaxAttempts,
			InitialDelayMs: step.Retry.InitialDelayMs,
			MaxDelayMs:     step.Retry.MaxDelayMs,
			Backoff:        step.Retry.Backoff,
			Multiplier:     step.Retry.Multiplier,
		}
	}

	// Build capture config
	if step.Capture != nil {
		dryRunStep.Capture = &workflow.DryRunCapture{
			Stdout:  step.Capture.Stdout,
			Stderr:  step.Capture.Stderr,
			MaxSize: step.Capture.MaxSize,
		}
	}

	// Build loop config
	if step.Loop != nil {
		dryRunStep.Loop = &workflow.DryRunLoop{
			Type:           string(step.Loop.Type),
			Items:          step.Loop.Items,
			Condition:      step.Loop.Condition,
			Body:           step.Loop.Body,
			MaxIterations:  step.Loop.MaxIterations,
			BreakCondition: step.Loop.BreakCondition,
			OnComplete:     step.Loop.OnComplete,
		}
	}

	// Build agent config (AC8: --dry-run shows resolved prompt without invoking)
	if step.Agent != nil {
		dryRunStep.Agent = e.buildAgentConfig(step.Agent, interpCtx)
	}

	return dryRunStep, nil
}

// resolveInputs validates and resolves input values with defaults.
func (e *DryRunExecutor) resolveInputs(wf *workflow.Workflow, inputs map[string]any) (map[string]workflow.DryRunInput, error) {
	result := make(map[string]workflow.DryRunInput)

	// Process defined inputs
	for _, inputDef := range wf.Inputs {
		dryRunInput := workflow.DryRunInput{
			Name:     inputDef.Name,
			Required: inputDef.Required,
		}

		// Check if value was provided
		switch {
		case inputs[inputDef.Name] != nil:
			dryRunInput.Value = inputs[inputDef.Name]
			dryRunInput.Default = false
		case inputDef.Default != nil:
			// Use default value
			dryRunInput.Value = inputDef.Default
			dryRunInput.Default = true
		case inputDef.Required:
			// Missing required input
			return nil, fmt.Errorf("missing required input: %s", inputDef.Name)
		default:
			// Optional with no default - skip
			continue
		}

		result[inputDef.Name] = dryRunInput
	}

	// Add any extra inputs not defined in the workflow
	for name, value := range inputs {
		if _, exists := result[name]; !exists {
			result[name] = workflow.DryRunInput{
				Name:     name,
				Value:    value,
				Default:  false,
				Required: false,
			}
		}
	}

	return result, nil
}

// buildTransitions collects all possible transitions from a step.
func (e *DryRunExecutor) buildTransitions(step *workflow.Step) []workflow.DryRunTransition {
	// Preallocate: conditional transitions + legacy transitions (success/error)
	transitions := make([]workflow.DryRunTransition, 0, len(step.Transitions)+2)

	// Add conditional transitions first
	for _, tr := range step.Transitions {
		transitionType := "conditional"
		if tr.When == "" {
			transitionType = "default"
		}
		transitions = append(transitions, workflow.DryRunTransition{
			Condition: tr.When,
			Target:    tr.Goto,
			Type:      transitionType,
		})
	}

	// Add legacy transitions if no conditional transitions exist
	if len(step.Transitions) == 0 {
		if step.OnSuccess != "" {
			transitions = append(transitions, workflow.DryRunTransition{
				Target: step.OnSuccess,
				Type:   "success",
			})
		}
		if step.OnFailure != "" {
			transitions = append(transitions, workflow.DryRunTransition{
				Target: step.OnFailure,
				Type:   "failure",
			})
		}
	}

	// For loop steps, add the on_complete transition
	if step.Loop != nil && step.Loop.OnComplete != "" {
		// Check if not already added
		hasOnComplete := false
		for _, tr := range transitions {
			if tr.Target == step.Loop.OnComplete {
				hasOnComplete = true
				break
			}
		}
		if !hasOnComplete {
			transitions = append(transitions, workflow.DryRunTransition{
				Target: step.Loop.OnComplete,
				Type:   "success",
			})
		}
	}

	return transitions
}

// buildHooks converts step hooks to DryRunHooks.
func (e *DryRunExecutor) buildHooks(hooks workflow.StepHooks, interpCtx *interpolation.Context) workflow.DryRunHooks {
	result := workflow.DryRunHooks{}

	// Pre hooks
	for _, action := range hooks.Pre {
		hook := workflow.DryRunHook{}
		if action.Log != "" {
			hook.Type = "log"
			hook.Content = e.resolveCommand(action.Log, interpCtx)
		} else if action.Command != "" {
			hook.Type = "command"
			hook.Content = e.resolveCommand(action.Command, interpCtx)
		}
		result.Pre = append(result.Pre, hook)
	}

	// Post hooks
	for _, action := range hooks.Post {
		hook := workflow.DryRunHook{}
		if action.Log != "" {
			hook.Type = "log"
			hook.Content = e.resolveCommand(action.Log, interpCtx)
		} else if action.Command != "" {
			hook.Type = "command"
			hook.Content = e.resolveCommand(action.Command, interpCtx)
		}
		result.Post = append(result.Post, hook)
	}

	return result
}

// resolveCommand resolves template variables in a command string.
// For dry-run, we attempt to resolve inputs but leave states.* as placeholders.
func (e *DryRunExecutor) resolveCommand(cmd string, interpCtx *interpolation.Context) string {
	if e.resolver == nil {
		return cmd
	}

	resolved, err := e.resolver.Resolve(cmd, interpCtx)
	if err != nil {
		// If resolution fails, return the original command
		// This is acceptable for dry-run as we may have unresolvable {{states.*}} refs
		return cmd
	}
	return resolved
}

// buildAgentConfig builds dry-run agent configuration showing resolved prompt and CLI command.
func (e *DryRunExecutor) buildAgentConfig(agent *workflow.AgentConfig, interpCtx *interpolation.Context) *workflow.DryRunAgent {
	dryRunAgent := &workflow.DryRunAgent{
		Provider: agent.Provider,
		Timeout:  agent.Timeout,
		Options:  make(map[string]any),
	}

	// Resolve prompt template
	if agent.Prompt != "" {
		dryRunAgent.ResolvedPrompt = e.resolveCommand(agent.Prompt, interpCtx)
	}

	// Copy options
	for key, value := range agent.Options {
		dryRunAgent.Options[key] = value
	}

	// Build CLI command based on provider
	if agent.Command != "" {
		dryRunAgent.CLICommand = e.resolveCommand(agent.Command, interpCtx)
	}

	return dryRunAgent
}
