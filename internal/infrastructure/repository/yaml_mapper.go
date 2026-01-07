package repository

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/vanoix/awf/internal/domain/workflow"
)

// mapToDomain converts a yamlWorkflow to a domain Workflow.
func mapToDomain(filePath string, y *yamlWorkflow) (*workflow.Workflow, error) {
	wf := &workflow.Workflow{
		Name:        y.Name,
		Description: y.Description,
		Version:     y.Version,
		Author:      y.Author,
		Tags:        y.Tags,
		Env:         y.Env,
		Initial:     y.States.Initial,
		Inputs:      mapInputs(y.Inputs),
		Steps:       make(map[string]*workflow.Step),
		Hooks:       mapWorkflowHooks(y.Hooks),
	}

	// Map steps
	for name, step := range y.States.Steps {
		domainStep, err := mapStep(filePath, name, step)
		if err != nil {
			return nil, err
		}
		wf.Steps[name] = domainStep
	}

	return wf, nil
}

// mapInputs converts yamlInput slice to domain Input slice.
func mapInputs(inputs []yamlInput) []workflow.Input {
	result := make([]workflow.Input, len(inputs))
	for i, inp := range inputs {
		result[i] = workflow.Input{
			Name:        inp.Name,
			Type:        inp.Type,
			Description: inp.Description,
			Required:    inp.Required,
			Default:     inp.Default,
			Validation:  mapInputValidation(inp.Validation),
		}
	}
	return result
}

// mapInputValidation converts yamlInputValidation to domain InputValidation.
func mapInputValidation(v *yamlInputValidation) *workflow.InputValidation {
	if v == nil {
		return nil
	}
	return &workflow.InputValidation{
		Pattern:       v.Pattern,
		Enum:          v.Enum,
		Min:           v.Min,
		Max:           v.Max,
		FileExists:    v.FileExists,
		FileExtension: v.FileExtension,
	}
}

// mapStep converts yamlStep to domain Step.
func mapStep(filePath, name string, y yamlStep) (*workflow.Step, error) {
	stepType, err := parseStepType(y.Type)
	if err != nil {
		return nil, NewParseError(filePath, "states."+name+".type", err.Error())
	}

	step := &workflow.Step{
		Name:            name,
		Type:            stepType,
		Description:     y.Description,
		Command:         y.Command,
		Dir:             y.Dir,
		Operation:       y.Operation,
		OperationInputs: y.OperationInputs,
		Branches:        y.Parallel,
		Strategy:        y.Strategy,
		MaxConcurrent:   y.MaxConcurrent,
		OnSuccess:       y.OnSuccess,
		OnFailure:       y.OnFailure,
		Transitions:     mapTransitions(y.Transitions),
		DependsOn:       y.DependsOn,
		ContinueOnError: y.ContinueOnError,
		Retry:           mapRetry(y.Retry),
		Capture:         mapCapture(y.Capture),
		Hooks:           mapStepHooks(y.Hooks),
		Status:          workflow.TerminalStatus(y.Status),
		Loop:            mapLoopConfig(y),
		TemplateRef:     mapTemplateRef(y.UseTemplate, y.Parameters),
		CallWorkflow:    mapCallWorkflowFlat(y),
		Agent:           mapAgentConfigFlat(y),
	}

	// Parse timeout
	if y.Timeout != "" {
		timeout, err := parseDuration(y.Timeout)
		if err != nil {
			return nil, NewParseError(filePath, "states."+name+".timeout", "invalid duration: "+y.Timeout)
		}
		step.Timeout = int(timeout.Seconds())
	}

	return step, nil
}

// parseStepType converts string to StepType.
func parseStepType(s string) (workflow.StepType, error) {
	switch strings.ToLower(s) {
	case "step", "command":
		return workflow.StepTypeCommand, nil
	case "parallel":
		return workflow.StepTypeParallel, nil
	case "terminal":
		return workflow.StepTypeTerminal, nil
	case "for_each":
		return workflow.StepTypeForEach, nil
	case "while":
		return workflow.StepTypeWhile, nil
	case "operation":
		return workflow.StepTypeOperation, nil
	case "call_workflow":
		return workflow.StepTypeCallWorkflow, nil
	case "agent":
		return workflow.StepTypeAgent, nil
	default:
		return "", NewParseError("", "", "unknown step type: "+s)
	}
}

// mapRetry converts yamlRetry to domain RetryConfig.
func mapRetry(y *yamlRetry) *workflow.RetryConfig {
	if y == nil {
		return nil
	}

	initialDelayMs := 0
	if y.InitialDelay != "" {
		if d, err := parseDuration(y.InitialDelay); err == nil {
			initialDelayMs = int(d.Milliseconds())
		}
	}

	maxDelayMs := 0
	if y.MaxDelay != "" {
		if d, err := parseDuration(y.MaxDelay); err == nil {
			maxDelayMs = int(d.Milliseconds())
		}
	}

	return &workflow.RetryConfig{
		MaxAttempts:        y.MaxAttempts,
		InitialDelayMs:     initialDelayMs,
		MaxDelayMs:         maxDelayMs,
		Backoff:            y.Backoff,
		Multiplier:         y.Multiplier,
		Jitter:             y.Jitter,
		RetryableExitCodes: y.RetryableExitCodes,
	}
}

// mapCapture converts yamlCapture to domain CaptureConfig.
func mapCapture(y *yamlCapture) *workflow.CaptureConfig {
	if y == nil {
		return nil
	}
	return &workflow.CaptureConfig{
		Stdout:   y.Stdout,
		Stderr:   y.Stderr,
		MaxSize:  y.MaxSize,
		Encoding: y.Encoding,
	}
}

// mapLoopConfig converts yamlStep loop fields to domain LoopConfig.
func mapLoopConfig(y yamlStep) *workflow.LoopConfig {
	// Check if this is a loop step
	hasItems := y.Items != nil
	hasWhile := y.While != ""

	if !hasItems && !hasWhile {
		return nil
	}

	var loopType workflow.LoopType
	var items string

	if hasItems {
		loopType = workflow.LoopTypeForEach
		switch v := y.Items.(type) {
		case string:
			items = v
		case []any:
			// Convert to JSON string for later parsing
			b, _ := json.Marshal(v)
			items = string(b)
		}
	} else {
		loopType = workflow.LoopTypeWhile
	}

	var maxIter int
	var maxIterExpr string
	var maxIterExplicitlySet bool
	switch v := y.MaxIterations.(type) {
	case int:
		maxIter = v
		// Zero is treated as "use default", so don't mark as explicitly set
		if v != 0 {
			maxIterExplicitlySet = true
		}
	case string:
		// Dynamic expression - store for runtime resolution
		// Empty string is treated as unset
		if v != "" {
			maxIterExpr = v
			maxIterExplicitlySet = true
		}
	case nil:
		// Not set - use default
	default:
		// Unexpected type - use default (validation will catch invalid types)
	}
	if maxIter == 0 && maxIterExpr == "" && !maxIterExplicitlySet {
		maxIter = workflow.DefaultMaxIterations
	}

	return &workflow.LoopConfig{
		Type:                       loopType,
		Items:                      items,
		Condition:                  y.While,
		Body:                       y.Body,
		MaxIterations:              maxIter,
		MaxIterationsExpr:          maxIterExpr,
		MaxIterationsExplicitlySet: maxIterExplicitlySet,
		BreakCondition:             y.BreakWhen,
		OnComplete:                 y.OnComplete,
	}
}

// mapWorkflowHooks converts yamlWorkflowHooks to domain WorkflowHooks.
func mapWorkflowHooks(y *yamlWorkflowHooks) workflow.WorkflowHooks {
	if y == nil {
		return workflow.WorkflowHooks{}
	}
	return workflow.WorkflowHooks{
		WorkflowStart:  mapHook(y.WorkflowStart),
		WorkflowEnd:    mapHook(y.WorkflowEnd),
		WorkflowError:  mapHook(y.WorkflowError),
		WorkflowCancel: mapHook(y.WorkflowCancel),
	}
}

// mapStepHooks converts yamlStepHooks to domain StepHooks.
func mapStepHooks(y *yamlStepHooks) workflow.StepHooks {
	if y == nil {
		return workflow.StepHooks{}
	}
	return workflow.StepHooks{
		Pre:  mapHook(y.Pre),
		Post: mapHook(y.Post),
	}
}

// mapHook converts yamlHookAction slice to domain Hook.
func mapHook(actions []yamlHookAction) workflow.Hook {
	if len(actions) == 0 {
		return nil
	}
	hook := make(workflow.Hook, len(actions))
	for i, a := range actions {
		hook[i] = workflow.HookAction{
			Log:     a.Log,
			Command: a.Command,
		}
	}
	return hook
}

// mapTransitions converts yamlTransition slice to domain Transitions.
func mapTransitions(transitions []yamlTransition) workflow.Transitions {
	if len(transitions) == 0 {
		return nil
	}
	result := make(workflow.Transitions, len(transitions))
	for i, t := range transitions {
		result[i] = workflow.Transition{
			When: t.When,
			Goto: t.Goto,
		}
	}
	return result
}

// parseDuration parses a duration string like "30s", "2m", "1h".
// Also supports integer-only strings as seconds.
func parseDuration(s string) (time.Duration, error) {
	// Try standard Go duration format
	d, parseErr := time.ParseDuration(s)
	if parseErr == nil {
		return d, nil
	}

	// Try as plain integer (seconds)
	secs, atoiErr := strconv.Atoi(s)
	if atoiErr == nil {
		return time.Duration(secs) * time.Second, nil
	}

	return 0, parseErr
}

// mapTemplate converts yamlTemplate to domain Template.
func mapTemplate(filePath string, y *yamlTemplate) (*workflow.Template, error) {
	t := &workflow.Template{
		Name:       y.Name,
		Parameters: mapTemplateParams(y.Parameters),
		States:     make(map[string]*workflow.Step),
	}

	// Map states
	for name, step := range y.States.Steps {
		domainStep, err := mapStep(filePath, name, step)
		if err != nil {
			return nil, err
		}
		t.States[name] = domainStep
	}

	// Validate template
	if err := t.Validate(); err != nil {
		return nil, NewParseError(filePath, "template", err.Error())
	}

	return t, nil
}

// mapTemplateParams converts yamlTemplateParam slice to domain.
func mapTemplateParams(params []yamlTemplateParam) []workflow.TemplateParam {
	result := make([]workflow.TemplateParam, len(params))
	for i, p := range params {
		result[i] = workflow.TemplateParam{
			Name:     p.Name,
			Required: p.Required,
			Default:  p.Default,
		}
	}
	return result
}

// mapTemplateRef converts use_template + parameters to WorkflowTemplateRef.
func mapTemplateRef(useTemplate string, parameters map[string]any) *workflow.WorkflowTemplateRef {
	if useTemplate == "" {
		return nil
	}
	return &workflow.WorkflowTemplateRef{
		TemplateName: useTemplate,
		Parameters:   parameters,
	}
}

// mapCallWorkflowFlat converts flat yamlStep fields to domain CallWorkflowConfig.
func mapCallWorkflowFlat(y yamlStep) *workflow.CallWorkflowConfig {
	if y.Workflow == "" {
		return nil
	}
	return &workflow.CallWorkflowConfig{
		Workflow: y.Workflow,
		Inputs:   y.CallInputs,
		Outputs:  y.CallOutputs,
		// Timeout is handled separately via step.Timeout
	}
}

// mapAgentConfigFlat maps flat agent fields from yamlStep to AgentConfig.
// Returns nil if no agent provider is specified.
func mapAgentConfigFlat(y yamlStep) *workflow.AgentConfig {
	if y.Provider == "" {
		return nil
	}
	return &workflow.AgentConfig{
		Provider: y.Provider,
		Prompt:   y.Prompt,
		Options:  y.Options,
		// Timeout is handled separately via step.Timeout
		Command: "", // Not supported in flat structure - stub
	}
}
