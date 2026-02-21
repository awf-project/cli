package application

import (
	"context"
	"fmt"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/pkg/interpolation"
)

// DryRunExecutor walks through a workflow without executing commands.
// It produces an execution plan showing what would happen.
type DryRunExecutor struct {
	wfSvc       *WorkflowService
	resolver    interpolation.Resolver
	evaluator   ports.ExpressionEvaluator
	templateSvc *TemplateService
	logger      ports.Logger
	awfPaths    map[string]string // F063: XDG directory paths injected from interfaces layer
}

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

func (e *DryRunExecutor) SetTemplateService(svc *TemplateService) {
	e.templateSvc = svc
}

// SetAWFPaths configures the AWF XDG directory paths for F063 template interpolation.
func (e *DryRunExecutor) SetAWFPaths(paths map[string]string) {
	e.awfPaths = paths
}

func (e *DryRunExecutor) Execute(ctx context.Context, workflowName string, inputs map[string]any) (*workflow.DryRunPlan, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("dry run cancelled: %w", ctx.Err())
	default:
	}

	wf, err := e.wfSvc.GetWorkflow(ctx, workflowName)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow: %w", err)
	}

	if e.templateSvc != nil {
		if err := e.templateSvc.ExpandWorkflow(ctx, wf); err != nil {
			return nil, fmt.Errorf("failed to expand templates: %w", err)
		}
	}

	return e.buildPlan(ctx, wf, inputs)
}

func (e *DryRunExecutor) buildPlan(ctx context.Context, wf *workflow.Workflow, inputs map[string]any) (*workflow.DryRunPlan, error) {
	resolvedInputs, err := e.resolveInputs(wf, inputs)
	if err != nil {
		return nil, err
	}

	interpCtx := interpolation.NewContext()
	for name, input := range resolvedInputs {
		interpCtx.Inputs[name] = input.Value
	}
	interpCtx.Workflow.Name = wf.Name

	// Populate AWF context with XDG directory paths (F063)
	// Paths are injected via SetAWFPaths() to avoid infrastructure import in application layer
	if e.awfPaths != nil {
		interpCtx.AWF = e.awfPaths
	} else {
		interpCtx.AWF = map[string]string{}
	}

	plan := &workflow.DryRunPlan{
		WorkflowName: wf.Name,
		Description:  wf.Description,
		Inputs:       resolvedInputs,
		Steps:        make([]workflow.DryRunStep, 0),
	}

	visited := make(map[string]bool)
	queue := []string{wf.Initial}

	for len(queue) > 0 {
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

		dryRunStep, err := e.buildStepPlan(ctx, step, wf, interpCtx)
		if err != nil {
			return nil, fmt.Errorf("step '%s': %w", stepName, err)
		}
		plan.Steps = append(plan.Steps, *dryRunStep)

		nextSteps := e.collectNextSteps(step)
		queue = append(queue, nextSteps...)
	}

	return plan, nil
}

func (e *DryRunExecutor) collectNextSteps(step *workflow.Step) []string {
	var nextSteps []string
	seen := make(map[string]bool)

	addIfNew := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			nextSteps = append(nextSteps, name)
		}
	}

	for _, tr := range step.Transitions {
		addIfNew(tr.Goto)
	}

	addIfNew(step.OnSuccess)
	addIfNew(step.OnFailure)

	for _, branch := range step.Branches {
		addIfNew(branch)
	}

	if step.Loop != nil {
		for _, bodyStep := range step.Loop.Body {
			addIfNew(bodyStep)
		}
		addIfNew(step.Loop.OnComplete)
	}

	return nextSteps
}

func (e *DryRunExecutor) buildStepPlan(ctx context.Context, step *workflow.Step, wf *workflow.Workflow, interpCtx *interpolation.Context) (*workflow.DryRunStep, error) {
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

	commandToResolve := step.Command
	if step.ScriptFile != "" {
		loadedScript, err := loadScriptFile(ctx, step.ScriptFile, wf, interpCtx)
		if err != nil {
			return nil, fmt.Errorf("load script file: %w", err)
		}
		commandToResolve = loadedScript
		dryRunStep.ScriptFile = step.ScriptFile
	}

	if commandToResolve != "" {
		dryRunStep.Command = e.resolveCommand(commandToResolve, interpCtx)
	}

	dryRunStep.Hooks = e.buildHooks(step.Hooks, interpCtx)
	dryRunStep.Transitions = e.buildTransitions(step)

	if step.Retry != nil {
		dryRunStep.Retry = &workflow.DryRunRetry{
			MaxAttempts:    step.Retry.MaxAttempts,
			InitialDelayMs: step.Retry.InitialDelayMs,
			MaxDelayMs:     step.Retry.MaxDelayMs,
			Backoff:        step.Retry.Backoff,
			Multiplier:     step.Retry.Multiplier,
		}
	}

	if step.Capture != nil {
		dryRunStep.Capture = &workflow.DryRunCapture{
			Stdout:  step.Capture.Stdout,
			Stderr:  step.Capture.Stderr,
			MaxSize: step.Capture.MaxSize,
		}
	}

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

	if step.Agent != nil {
		var err error
		dryRunStep.Agent, err = e.buildAgentConfig(ctx, step.Agent, wf, interpCtx)
		if err != nil {
			return nil, fmt.Errorf("build agent config: %w", err)
		}
	}

	return dryRunStep, nil
}

func (e *DryRunExecutor) resolveInputs(wf *workflow.Workflow, inputs map[string]any) (map[string]workflow.DryRunInput, error) {
	result := make(map[string]workflow.DryRunInput)

	for _, inputDef := range wf.Inputs {
		dryRunInput := workflow.DryRunInput{
			Name:     inputDef.Name,
			Required: inputDef.Required,
		}

		switch {
		case inputs[inputDef.Name] != nil:
			dryRunInput.Value = inputs[inputDef.Name]
			dryRunInput.Default = false
		case inputDef.Default != nil:
			dryRunInput.Value = inputDef.Default
			dryRunInput.Default = true
		case inputDef.Required:
			return nil, fmt.Errorf("missing required input: %s", inputDef.Name)
		default:
			continue
		}

		result[inputDef.Name] = dryRunInput
	}

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

func (e *DryRunExecutor) buildTransitions(step *workflow.Step) []workflow.DryRunTransition {
	transitions := make([]workflow.DryRunTransition, 0, len(step.Transitions)+2)

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

	if step.Loop != nil && step.Loop.OnComplete != "" {
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

func (e *DryRunExecutor) buildHooks(hooks workflow.StepHooks, interpCtx *interpolation.Context) workflow.DryRunHooks {
	result := workflow.DryRunHooks{}

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

// NOTE: For dry-run, we attempt to resolve inputs but leave states.* as placeholders if unresolvable.
func (e *DryRunExecutor) resolveCommand(cmd string, interpCtx *interpolation.Context) string {
	if e.resolver == nil {
		return cmd
	}

	resolved, err := e.resolver.Resolve(cmd, interpCtx)
	if err != nil {
		return cmd
	}
	return resolved
}

func (e *DryRunExecutor) buildAgentConfig(ctx context.Context, agent *workflow.AgentConfig, wf *workflow.Workflow, interpCtx *interpolation.Context) (*workflow.DryRunAgent, error) {
	dryRunAgent := &workflow.DryRunAgent{
		Provider:     agent.Provider,
		Timeout:      agent.Timeout,
		OutputFormat: agent.OutputFormat,
		Options:      make(map[string]any),
	}

	promptToResolve := agent.Prompt
	if agent.PromptFile != "" {
		loadedPrompt, err := loadPromptFile(ctx, agent.PromptFile, wf, interpCtx)
		if err != nil {
			return nil, fmt.Errorf("load prompt file: %w", err)
		}
		promptToResolve = loadedPrompt
	}
	if promptToResolve != "" {
		dryRunAgent.ResolvedPrompt = e.resolveCommand(promptToResolve, interpCtx)
	}

	for key, value := range agent.Options {
		dryRunAgent.Options[key] = value
	}

	return dryRunAgent, nil
}
