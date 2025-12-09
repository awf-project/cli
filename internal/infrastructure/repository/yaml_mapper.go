package repository

import (
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
		Branches:        y.Parallel,
		Strategy:        y.Strategy,
		MaxConcurrent:   y.MaxConcurrent,
		OnSuccess:       y.OnSuccess,
		OnFailure:       y.OnFailure,
		DependsOn:       y.DependsOn,
		ContinueOnError: y.ContinueOnError,
		Retry:           mapRetry(y.Retry),
		Capture:         mapCapture(y.Capture),
		Hooks:           mapStepHooks(y.Hooks),
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

// parseDuration parses a duration string like "30s", "2m", "1h".
// Also supports integer-only strings as seconds.
func parseDuration(s string) (time.Duration, error) {
	// Try standard Go duration format
	d, err := time.ParseDuration(s)
	if err == nil {
		return d, nil
	}

	// Try as plain integer (seconds)
	if secs, err := strconv.Atoi(s); err == nil {
		return time.Duration(secs) * time.Second, nil
	}

	return 0, err
}
