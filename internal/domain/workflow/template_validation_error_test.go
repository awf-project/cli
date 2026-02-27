package workflow_test

// C013: Domain test file splitting
// Source: internal/domain/workflow/template_validation_test.go
// Test count: 54 tests
// Focus: error.* namespace, hooks, general validation, loop expressions, and helpers
// Note: This file exceeds 600 lines per ADR-002 (logical cohesion over strict size limits)

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateValidator_ErrorRefInErrorHook_Valid(t *testing.T) {
	w := newTestWorkflow()
	w.Hooks = workflow.WorkflowHooks{
		WorkflowError: workflow.Hook{
			{Command: "echo Error: {{error.Message}} in {{error.State}}"},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "error references in workflow error hooks are valid")
}

func TestTemplateValidator_ErrorRefAllProperties_Valid(t *testing.T) {
	w := newTestWorkflow()
	w.Hooks = workflow.WorkflowHooks{
		WorkflowError: workflow.Hook{
			{Command: "echo {{error.Message}} {{error.State}} {{error.ExitCode}} {{error.Type}}"},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_ErrorRefOutsideErrorHook_Invalid(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{error.Message}}" // ERROR: not in error hook

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrErrorRefOutsideErrorHook, result.Errors[0].Code)
}

func TestTemplateValidator_ErrorRefInStartHook_Invalid(t *testing.T) {
	w := newTestWorkflow()
	w.Hooks = workflow.WorkflowHooks{
		WorkflowStart: workflow.Hook{
			{Command: "echo {{error.Message}}"}, // Invalid: WorkflowStart is not an error hook
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrErrorRefOutsideErrorHook, result.Errors[0].Code)
}

func TestTemplateValidator_ErrorRefInEndHook_Invalid(t *testing.T) {
	w := newTestWorkflow()
	w.Hooks = workflow.WorkflowHooks{
		WorkflowEnd: workflow.Hook{
			{Command: "echo {{error.Message}}"}, // Invalid: WorkflowEnd is not an error hook
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrErrorRefOutsideErrorHook, result.Errors[0].Code)
}

func TestTemplateValidator_InvalidErrorProperty(t *testing.T) {
	w := newTestWorkflow()
	w.Hooks = workflow.WorkflowHooks{
		WorkflowError: workflow.Hook{
			{Command: "echo {{error.invalid_property}}"},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrInvalidErrorProperty, result.Errors[0].Code)
}

func TestTemplateValidator_ValidStepPreHook(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Hooks = workflow.StepHooks{
		Pre: workflow.Hook{
			{Command: "echo Starting {{inputs.name}}"},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_ValidStepPostHook(t *testing.T) {
	w := newLinearWorkflow()
	w.Steps["step2"].Hooks = workflow.StepHooks{
		Post: workflow.Hook{
			{Command: "echo Completed with {{states.step1.Output}}"},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_InvalidRefInStepHook(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Hooks = workflow.StepHooks{
		Pre: workflow.Hook{
			{Command: "echo {{inputs.undefined}}"},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrUndefinedInput, result.Errors[0].Code)
}

func TestTemplateValidator_HookLogField(t *testing.T) {
	// Hooks can have Log instead of Command
	w := newTestWorkflow()
	w.Steps["start"].Hooks = workflow.StepHooks{
		Pre: workflow.Hook{
			{Log: "Processing {{inputs.name}}"},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_InvalidRefInHookLog(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Hooks = workflow.StepHooks{
		Pre: workflow.Hook{
			{Log: "Processing {{inputs.undefined}}"},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
}

func TestTemplateValidator_UnknownReferenceType(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{unknown.field}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrUnknownReferenceType, result.Errors[0].Code)
}

func TestTemplateValidator_TypoInNamespace(t *testing.T) {
	tests := []struct {
		name     string
		template string
	}{
		{"input instead of inputs", "{{input.name}}"},
		{"state instead of states", "{{state.step1.output}}"},
		{"workflows instead of workflow", "{{workflows.name}}"},
		{"environment instead of env", "{{environment.HOME}}"},
		{"errors instead of error", "{{errors.message}}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newTestWorkflow()
			w.Steps["start"].Command = "echo " + tt.template

			v := workflow.NewTemplateValidator(w, newTestAnalyzer())
			result := v.Validate()

			require.True(t, result.HasErrors())
			assert.Equal(t, workflow.ErrUnknownReferenceType, result.Errors[0].Code)
		})
	}
}

func TestTemplateValidator_AggregatesAllErrors(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "multi-error-test",
		Initial: "step1",
		Inputs:  []workflow.Input{{Name: "valid", Type: "string"}},
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{inputs.undefined}} {{states.nonexistent.Output}} {{workflow.bad}}",
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			"error": {
				Name:   "error",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.GreaterOrEqual(t, len(result.Errors), 3, "should collect all errors in single pass")
}

func TestTemplateValidator_AggregatesErrorsAcrossSteps(t *testing.T) {
	w := newLinearWorkflow()
	w.Steps["step1"].Command = "echo {{inputs.undefined1}}"
	w.Steps["step2"].Command = "echo {{inputs.undefined2}}"
	w.Steps["step3"].Command = "echo {{inputs.undefined3}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.GreaterOrEqual(t, len(result.Errors), 3)
}

func TestTemplateValidator_AggregatesErrorsAcrossFields(t *testing.T) {
	// Errors in different fields of the same step
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{inputs.undefined1}}"
	w.Steps["start"].Dir = "{{inputs.undefined2}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.GreaterOrEqual(t, len(result.Errors), 2)
}

func TestTemplateValidator_ValidDirFieldReference(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Dir = "{{context.WorkingDir}}/subdir"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_InvalidDirFieldReference(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Dir = "{{inputs.undefined}}/subdir"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
}

func TestTemplateValidator_TerminalStepsIgnored(t *testing.T) {
	// Terminal steps shouldn't have commands, but if they do, validate them
	w := newTestWorkflow()
	// Terminal steps don't have Command field used, so this is a no-op
	// but the validator should handle gracefully

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_ComplexRealWorldWorkflow(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "build-test-deploy",
		Initial: "checkout",
		Inputs: []workflow.Input{
			{Name: "repo", Type: "string", Required: true},
			{Name: "branch", Type: "string", Default: "main"},
			{Name: "environment", Type: "string", Required: true},
		},
		Steps: map[string]*workflow.Step{
			"checkout": {
				Name:      "checkout",
				Type:      workflow.StepTypeCommand,
				Command:   "git clone {{inputs.repo}} --branch {{inputs.branch}}",
				OnSuccess: "build",
				OnFailure: "error",
			},
			"build": {
				Name:      "build",
				Type:      workflow.StepTypeCommand,
				Command:   "make build",
				Dir:       "{{context.WorkingDir}}/repo",
				OnSuccess: "test",
				OnFailure: "error",
			},
			"test": {
				Name:      "test",
				Type:      workflow.StepTypeCommand,
				Command:   "make test",
				OnSuccess: "deploy",
				OnFailure: "error",
			},
			"deploy": {
				Name:      "deploy",
				Type:      workflow.StepTypeCommand,
				Command:   "deploy --env={{inputs.environment}} --version={{workflow.ID}}",
				OnSuccess: "notify",
				OnFailure: "error",
			},
			"notify": {
				Name:      "notify",
				Type:      workflow.StepTypeCommand,
				Command:   "curl -X POST -d 'Deployed {{inputs.repo}} to {{inputs.environment}}' {{env.SLACK_WEBHOOK}}",
				OnSuccess: "done",
				OnFailure: "done", // Notification failure shouldn't fail workflow
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			"error": {
				Name:   "error",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
		Hooks: workflow.WorkflowHooks{
			WorkflowStart: workflow.Hook{
				{Log: "Starting build for {{inputs.repo}}"},
			},
			WorkflowEnd: workflow.Hook{
				{Log: "Workflow {{workflow.Name}} completed in {{workflow.Duration}}"},
			},
			WorkflowError: workflow.Hook{
				{Command: "echo 'Error in {{error.State}}: {{error.Message}}' | mail -s 'Build Failed' team@example.com"},
			},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "real-world workflow should validate successfully")
}

func TestTemplateValidator_ErrorMessageContainsStepName(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{inputs.undefined}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Contains(t, result.Errors[0].Path, "start", "error path should contain step name")
}

func TestTemplateValidator_ErrorMessageContainsReference(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{inputs.undefined}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Contains(t, result.Errors[0].Message, "undefined", "error message should contain the reference")
}

func TestTemplateValidator_LoopExpressions_ValidMaxIterationsExpr(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.MaxIterationsExpr = "{{inputs.limit}}"
	w.Steps["loop_step"].Loop.MaxIterations = 0 // Dynamic takes precedence

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "valid input reference in MaxIterationsExpr should pass")
	assert.False(t, result.HasWarnings(), "valid input reference should not produce warnings")
}

func TestTemplateValidator_LoopExpressions_ValidMaxIterationsExprFromEnv(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.MaxIterationsExpr = "{{env.LOOP_LIMIT}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// Env variables are validated at runtime, not statically - should pass
	assert.False(t, result.HasErrors(), "env reference in MaxIterationsExpr should pass static validation")
}

func TestTemplateValidator_LoopExpressions_UndefinedInputInMaxIterationsExpr(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.MaxIterationsExpr = "{{inputs.undefined_var}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// Should emit a warning about potentially undefined variable
	require.True(t, result.HasWarnings() || result.HasErrors(),
		"undefined input in MaxIterationsExpr should be flagged")

	// Check that the issue mentions the undefined variable
	issues := result.AllIssues()
	require.NotEmpty(t, issues)

	found := false
	for _, issue := range issues {
		if issue.Code == workflow.ErrUndefinedLoopVariable || issue.Code == workflow.ErrUndefinedInput {
			found = true
			assert.Contains(t, issue.Message, "undefined_var",
				"warning should mention the undefined variable name")
			assert.Contains(t, issue.Path, "loop_step",
				"warning should mention the step name")
		}
	}
	assert.True(t, found, "should have found an undefined variable issue")
}

func TestTemplateValidator_LoopExpressions_ValidConditionWithInputs(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.Type = workflow.LoopTypeWhile
	w.Steps["loop_step"].Loop.Condition = "{{inputs.threshold}} > 0"
	w.Steps["loop_step"].Loop.Items = "" // Not needed for while loop

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "valid input reference in condition should pass")
}

func TestTemplateValidator_LoopExpressions_UndefinedInputInCondition(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.Type = workflow.LoopTypeWhile
	w.Steps["loop_step"].Loop.Condition = "{{inputs.undefined_count}} < {{inputs.threshold}}"
	w.Steps["loop_step"].Loop.Items = ""

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// Should flag the undefined input
	require.True(t, result.HasWarnings() || result.HasErrors(),
		"undefined input in condition should be flagged")

	issues := result.AllIssues()
	require.NotEmpty(t, issues)

	// Should mention undefined_count (threshold is defined)
	hasUndefinedCountIssue := false
	for _, issue := range issues {
		if issue.Code == workflow.ErrUndefinedLoopVariable || issue.Code == workflow.ErrUndefinedInput {
			if issue.Message != "" && (issue.Message == "undefined_count" ||
				issue.Message != "" && issue.Message[0:1] != "") {
				hasUndefinedCountIssue = true
			}
		}
	}
	_ = hasUndefinedCountIssue // Will be properly checked after implementation
}

func TestTemplateValidator_LoopExpressions_ValidItemsWithInputs(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.Items = "{{inputs.items_list}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "valid input reference in items should pass")
}

func TestTemplateValidator_LoopExpressions_UndefinedInputInItems(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.Items = "{{inputs.undefined_items}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasWarnings() || result.HasErrors(),
		"undefined input in items should be flagged")
}

func TestTemplateValidator_LoopExpressions_ValidBreakCondition(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.BreakCondition = "{{inputs.limit}} <= 0"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "valid input reference in break_condition should pass")
}

func TestTemplateValidator_LoopExpressions_UndefinedInputInBreakCondition(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.BreakCondition = "{{inputs.undefined_flag}} == true"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasWarnings() || result.HasErrors(),
		"undefined input in break_condition should be flagged")
}

func TestTemplateValidator_LoopExpressions_ArithmeticExpression(t *testing.T) {
	w := newLoopWorkflow()
	// Arithmetic expression with defined inputs
	w.Steps["loop_step"].Loop.MaxIterationsExpr = "{{inputs.limit * inputs.threshold}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// The validator should at minimum check that variables in expression exist
	// Both limit and threshold are defined, so should pass
	// Note: actual arithmetic validation happens at runtime
	assert.False(t, result.HasErrors(), "arithmetic expression with valid inputs should pass")
}

func TestTemplateValidator_LoopExpressions_ArithmeticExpressionWithUndefined(t *testing.T) {
	w := newLoopWorkflow()
	// Arithmetic expression with one undefined input
	w.Steps["loop_step"].Loop.MaxIterationsExpr = "{{inputs.limit + inputs.undefined_multiplier}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasWarnings() || result.HasErrors(),
		"arithmetic expression with undefined input should be flagged")
}

func TestTemplateValidator_LoopExpressions_StateReferenceInCondition(t *testing.T) {
	// Create a workflow where loop condition references previous step output
	w := &workflow.Workflow{
		Name:    "loop-with-state-ref",
		Initial: "setup",
		Inputs:  []workflow.Input{{Name: "target", Type: "integer", Default: 10}},
		Steps: map[string]*workflow.Step{
			"setup": {
				Name:      "setup",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 0",
				OnSuccess: "loop_step",
				OnFailure: "error",
			},
			"loop_step": {
				Name:      "loop_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo iteration",
				OnSuccess: "done",
				OnFailure: "error",
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeWhile,
					Condition:     "{{states.setup.Output}} < {{inputs.target}}",
					Body:          []string{"loop_body"},
					MaxIterations: 100,
					OnComplete:    "done",
				},
			},
			"loop_body": {
				Name:      "loop_body",
				Type:      workflow.StepTypeCommand,
				Command:   "echo body",
				OnSuccess: "loop_step",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			"error": {Name: "error", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// setup runs before loop_step, so the state reference should be valid
	assert.False(t, result.HasErrors(), "valid state reference in condition should pass")
}

func TestTemplateValidator_LoopExpressions_ForwardStateReferenceInCondition(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "loop-forward-ref",
		Initial: "loop_step",
		Inputs:  []workflow.Input{},
		Steps: map[string]*workflow.Step{
			"loop_step": {
				Name:      "loop_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo iteration",
				OnSuccess: "after_loop",
				OnFailure: "error",
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeWhile,
					Condition:     "{{states.after_loop.Output}} != 'done'", // Forward reference!
					Body:          []string{"loop_body"},
					MaxIterations: 10,
					OnComplete:    "after_loop",
				},
			},
			"loop_body": {
				Name:      "loop_body",
				Type:      workflow.StepTypeCommand,
				Command:   "echo body",
				OnSuccess: "loop_step",
				OnFailure: "error",
			},
			"after_loop": {
				Name:      "after_loop",
				Type:      workflow.StepTypeCommand,
				Command:   "echo done",
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			"error": {Name: "error", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// after_loop hasn't run yet - this should be flagged
	require.True(t, result.HasErrors() || result.HasWarnings(),
		"forward state reference in loop condition should be flagged")
}

func TestTemplateValidator_LoopExpressions_MultipleUndefinedInputs(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.MaxIterationsExpr = "{{inputs.undefined1}}"
	w.Steps["loop_step"].Loop.BreakCondition = "{{inputs.undefined2}} == true"
	w.Steps["loop_step"].Loop.Items = "{{inputs.undefined3}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// Should flag all undefined variables
	issues := result.AllIssues()
	undefinedCount := 0
	for _, issue := range issues {
		if issue.Code == workflow.ErrUndefinedLoopVariable || issue.Code == workflow.ErrUndefinedInput {
			undefinedCount++
		}
	}

	assert.GreaterOrEqual(t, undefinedCount, 3,
		"should flag all undefined variables, got %d", undefinedCount)
}

func TestTemplateValidator_LoopExpressions_NoLoop(t *testing.T) {
	// Step without loop - should not cause any issues
	w := newTestWorkflow()

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_LoopExpressions_EmptyExpressions(t *testing.T) {
	w := newLoopWorkflow()
	// All loop expressions are empty or use static values
	w.Steps["loop_step"].Loop.MaxIterationsExpr = ""
	w.Steps["loop_step"].Loop.MaxIterations = 10
	w.Steps["loop_step"].Loop.Items = "a,b,c" // Static, no interpolation

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "static loop config should pass")
}

func TestTemplateValidator_LoopExpressions_MixedValidAndInvalid(t *testing.T) {
	w := newLoopWorkflow()
	// One valid, one invalid
	w.Steps["loop_step"].Loop.MaxIterationsExpr = "{{inputs.limit}}"  // Valid
	w.Steps["loop_step"].Loop.BreakCondition = "{{inputs.undefined}}" // Invalid

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// Should flag the invalid one but not the valid one
	issues := result.AllIssues()
	undefinedCount := 0
	for _, issue := range issues {
		if issue.Code == workflow.ErrUndefinedLoopVariable || issue.Code == workflow.ErrUndefinedInput {
			undefinedCount++
			assert.Contains(t, issue.Message, "undefined",
				"should only flag the undefined variable")
		}
	}

	assert.Equal(t, 1, undefinedCount, "should only flag one undefined variable")
}

func TestTemplateValidator_LoopExpressions_WorkflowReference(t *testing.T) {
	w := newLoopWorkflow()
	// Using workflow property in loop expression (unusual but valid)
	w.Steps["loop_step"].Loop.BreakCondition = "{{workflow.Duration}} > 60"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "workflow reference in loop expression should be valid")
}

func TestTemplateValidator_LoopExpressions_InvalidWorkflowProperty(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.BreakCondition = "{{workflow.invalid_property}} > 0"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors() || result.HasWarnings(),
		"invalid workflow property in loop expression should be flagged")
}

func TestTemplateValidator_LoopExpressions_ContextReference(t *testing.T) {
	w := newLoopWorkflow()
	// Using context in loop (unusual but technically valid)
	w.Steps["loop_step"].Loop.Items = "{{context.WorkingDir}}/items"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "context reference in loop expression should be valid")
}

func TestTemplateValidator_LoopExpressions_ErrorPath(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.MaxIterationsExpr = "{{inputs.undefined}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	issues := result.AllIssues()
	require.NotEmpty(t, issues)

	// The error path should help identify the location
	for _, issue := range issues {
		if issue.Code == workflow.ErrUndefinedLoopVariable || issue.Code == workflow.ErrUndefinedInput {
			assert.Contains(t, issue.Path, "loop_step",
				"error path should contain step name for loop expressions")
		}
	}
}

func TestTemplateValidator_LoopExpressions_WarningLevel(t *testing.T) {
	// Undefined input variables in loop expressions should be warnings, not errors
	// because they could potentially come from env at runtime
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.MaxIterationsExpr = "{{inputs.maybe_from_env}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// Per the spec, undefined loop variables should be warnings since they could be from env
	// The exact behavior depends on implementation
	issues := result.AllIssues()
	require.NotEmpty(t, issues, "should produce at least one issue")
}

func TestTemplateValidator_LoopExpressions_WhileLoopWithItems(t *testing.T) {
	// While loop shouldn't have items, but if it does, still validate template references
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.Type = workflow.LoopTypeWhile
	w.Steps["loop_step"].Loop.Condition = "true"
	// Items shouldn't be used for while loop, but if set, validate it
	w.Steps["loop_step"].Loop.Items = "{{inputs.undefined}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// Should still validate the items field even if not used
	issues := result.AllIssues()
	hasUndefinedIssue := false
	for _, issue := range issues {
		if issue.Code == workflow.ErrUndefinedLoopVariable || issue.Code == workflow.ErrUndefinedInput {
			hasUndefinedIssue = true
		}
	}
	assert.True(t, hasUndefinedIssue, "should validate items field template references")
}

func TestTemplateValidator_LoopExpressions_ComplexExpression(t *testing.T) {
	w := newLoopWorkflow()
	// Complex expression with multiple references
	w.Steps["loop_step"].Loop.Condition = "{{inputs.limit}} > 0 && {{inputs.threshold}} < 100 && {{env.DEBUG}} == 'true'"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// All inputs are defined, env is validated at runtime
	assert.False(t, result.HasErrors(), "complex expression with valid references should pass")
}

func TestTemplateValidator_LoopExpressions_NestedTemplate(t *testing.T) {
	w := newLoopWorkflow()
	// Nested/escaped template syntax (edge case)
	w.Steps["loop_step"].Loop.Items = "{{inputs.items_list}},extra"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

// enqueueIfNotVisited Helper Tests (C003: Phase 3)

// TestEnqueueIfNotVisited_NotVisited verifies that unvisited states are added to queue.
// Feature: C003 - Reduce Graph Algorithm Cognitive Complexity
func TestEnqueueIfNotVisited_NotVisited(t *testing.T) {
	tests := []struct {
		name          string
		initialQueue  []string
		visited       map[string]bool
		state         string
		expectedQueue []string
	}{
		{
			name:          "add to empty queue",
			initialQueue:  []string{},
			visited:       map[string]bool{},
			state:         "step1",
			expectedQueue: []string{"step1"},
		},
		{
			name:          "add to non-empty queue",
			initialQueue:  []string{"existing"},
			visited:       map[string]bool{"existing": false},
			state:         "step2",
			expectedQueue: []string{"existing", "step2"},
		},
		{
			name:          "add when state not in visited map",
			initialQueue:  []string{"step1"},
			visited:       map[string]bool{"other": true},
			state:         "step2",
			expectedQueue: []string{"step1", "step2"},
		},
		{
			name:          "add when visited is false",
			initialQueue:  []string{"step1"},
			visited:       map[string]bool{"step2": false},
			state:         "step2",
			expectedQueue: []string{"step1", "step2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := make([]string, len(tt.initialQueue))
			copy(queue, tt.initialQueue)

			workflow.EnqueueIfNotVisited(&queue, tt.visited, tt.state)

			assert.Equal(t, tt.expectedQueue, queue, "queue should contain expected states")
		})
	}
}

// TestEnqueueIfNotVisited_AlreadyVisited verifies that visited states are not added to queue.
// Feature: C003 - Reduce Graph Algorithm Cognitive Complexity
func TestEnqueueIfNotVisited_AlreadyVisited(t *testing.T) {
	tests := []struct {
		name          string
		initialQueue  []string
		visited       map[string]bool
		state         string
		expectedQueue []string
	}{
		{
			name:          "skip already visited state",
			initialQueue:  []string{"step1"},
			visited:       map[string]bool{"step2": true},
			state:         "step2",
			expectedQueue: []string{"step1"},
		},
		{
			name:          "skip when queue would be empty",
			initialQueue:  []string{},
			visited:       map[string]bool{"step1": true},
			state:         "step1",
			expectedQueue: []string{},
		},
		{
			name:          "skip with multiple visited states",
			initialQueue:  []string{"step1", "step2"},
			visited:       map[string]bool{"step3": true, "step4": true},
			state:         "step3",
			expectedQueue: []string{"step1", "step2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := make([]string, len(tt.initialQueue))
			copy(queue, tt.initialQueue)

			workflow.EnqueueIfNotVisited(&queue, tt.visited, tt.state)

			assert.Equal(t, tt.expectedQueue, queue, "queue should remain unchanged for visited states")
		})
	}
}

// TestEnqueueIfNotVisited_EdgeCases verifies edge cases and boundary conditions.
// Feature: C003 - Reduce Graph Algorithm Cognitive Complexity
func TestEnqueueIfNotVisited_EdgeCases(t *testing.T) {
	t.Run("empty state name", func(t *testing.T) {
		queue := []string{"step1"}
		visited := map[string]bool{}

		workflow.EnqueueIfNotVisited(&queue, visited, "")

		// Empty state should still be added if not visited
		assert.Equal(t, []string{"step1", ""}, queue)
	})

	t.Run("nil visited map", func(t *testing.T) {
		queue := []string{"step1"}

		workflow.EnqueueIfNotVisited(&queue, nil, "step2")

		// With nil map, should treat as not visited and add
		assert.Equal(t, []string{"step1", "step2"}, queue)
	})

	t.Run("state with special characters", func(t *testing.T) {
		queue := []string{}
		visited := map[string]bool{}
		state := "step-with-dashes_and_underscores.123"

		workflow.EnqueueIfNotVisited(&queue, visited, state)

		assert.Equal(t, []string{state}, queue)
	})

	t.Run("multiple calls with same state alternating visited flag", func(t *testing.T) {
		queue := []string{}
		visited := map[string]bool{}

		// First call: not visited, should add
		workflow.EnqueueIfNotVisited(&queue, visited, "step1")
		assert.Equal(t, []string{"step1"}, queue)

		// Mark as visited
		visited["step1"] = true

		// Second call: visited, should not add
		workflow.EnqueueIfNotVisited(&queue, visited, "step1")
		assert.Equal(t, []string{"step1"}, queue, "should not add duplicate when visited")
	})
}

// TestEnqueueIfNotVisited_QueueModification verifies queue is modified in-place correctly.
// Feature: C003 - Reduce Graph Algorithm Cognitive Complexity
func TestEnqueueIfNotVisited_QueueModification(t *testing.T) {
	t.Run("queue pointer is modified", func(t *testing.T) {
		queue := []string{"initial"}
		visited := map[string]bool{}

		// Keep reference to original pointer
		queuePtr := &queue

		workflow.EnqueueIfNotVisited(queuePtr, visited, "new")

		// Verify the pointer was updated
		assert.Equal(t, []string{"initial", "new"}, *queuePtr)
		assert.Equal(t, []string{"initial", "new"}, queue)
	})

	t.Run("queue capacity grows as needed", func(t *testing.T) {
		queue := make([]string, 0, 2) // capacity of 2
		visited := map[string]bool{}

		workflow.EnqueueIfNotVisited(&queue, visited, "step1")
		workflow.EnqueueIfNotVisited(&queue, visited, "step2")
		workflow.EnqueueIfNotVisited(&queue, visited, "step3") // should grow capacity

		assert.Equal(t, []string{"step1", "step2", "step3"}, queue)
		assert.GreaterOrEqual(t, cap(queue), 3, "capacity should have grown")
	})
}
