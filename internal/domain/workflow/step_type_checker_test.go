package workflow_test

// C069: StepTypeChecker parameter tests
// Tests: Step.Validate() with StepTypeChecker for custom step type acceptance,
// Workflow.Validate() with StepTypeChecker parameter passing

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
)

// TestStepValidate_UnknownType_NilChecker validates backward compatibility:
// Unknown step types are rejected when checker is nil
func TestStepValidate_UnknownType_NilChecker(t *testing.T) {
	step := workflow.Step{
		Name: "custom_step",
		Type: "custom_type", // Not a built-in type
	}

	err := step.Validate(nil, nil)

	assert.Error(t, err)
	assert.ErrorContains(t, err, "unknown step type")
}

// TestStepValidate_UnknownType_CheckerAccepts validates happy path:
// Unknown step types are accepted when checker returns true
func TestStepValidate_UnknownType_CheckerAccepts(t *testing.T) {
	step := workflow.Step{
		Name: "custom_step",
		Type: "custom_plugin_step",
	}

	// Checker that accepts the custom type
	checker := func(typeName string) bool {
		return typeName == "custom_plugin_step"
	}

	err := step.Validate(nil, checker)

	assert.NoError(t, err)
}

// TestStepValidate_UnknownType_CheckerRejects validates error path:
// Unknown step types are rejected when checker returns false
func TestStepValidate_UnknownType_CheckerRejects(t *testing.T) {
	step := workflow.Step{
		Name: "custom_step",
		Type: "unknown_custom_type",
	}

	// Checker that only accepts specific types
	checker := func(typeName string) bool {
		return typeName == "approved_custom_type"
	}

	err := step.Validate(nil, checker)

	assert.Error(t, err)
	assert.ErrorContains(t, err, "unknown step type")
}

// TestStepValidate_BuiltinType_CheckerIgnored validates built-in types
// work correctly regardless of checker value (checker is only consulted for unknown types)
func TestStepValidate_BuiltinType_CheckerIgnored(t *testing.T) {
	step := workflow.Step{
		Name:    "builtin_step",
		Type:    workflow.StepTypeCommand,
		Command: "echo hello",
	}

	// Checker that rejects everything (should be ignored for built-in types)
	rejectingChecker := func(typeName string) bool {
		return false
	}

	err := step.Validate(nil, rejectingChecker)

	assert.NoError(t, err)
}

// TestWorkflowValidate_WithCustomStepType_CheckerAccepts validates workflow validation
// passes the checker correctly, allowing custom step types
func TestWorkflowValidate_WithCustomStepType_CheckerAccepts(t *testing.T) {
	wf := workflow.Workflow{
		Name:    "test_workflow",
		Initial: "custom_step",
		Steps: map[string]*workflow.Step{
			"custom_step": {
				Name: "custom_step",
				Type: "custom_type", // Custom type from plugin
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	// Checker that accepts custom types
	checker := func(typeName string) bool {
		return typeName == "custom_type"
	}

	err := wf.Validate(nil, checker)

	assert.NoError(t, err)
}

// TestWorkflowValidate_WithCustomStepType_CheckerRejects validates workflow validation
// properly rejects custom step types when checker returns false
func TestWorkflowValidate_WithCustomStepType_CheckerRejects(t *testing.T) {
	wf := workflow.Workflow{
		Name:    "test_workflow",
		Initial: "custom_step",
		Steps: map[string]*workflow.Step{
			"custom_step": {
				Name: "custom_step",
				Type: "unknown_type",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	// Checker that only accepts specific types
	checker := func(typeName string) bool {
		return typeName == "approved_type"
	}

	err := wf.Validate(nil, checker)

	assert.Error(t, err)
	assert.ErrorContains(t, err, "unknown step type")
}

// TestWorkflowValidate_WithCustomStepType_NilChecker validates backward compatibility:
// workflow validation rejects custom step types when checker is nil
func TestWorkflowValidate_WithCustomStepType_NilChecker(t *testing.T) {
	wf := workflow.Workflow{
		Name:    "test_workflow",
		Initial: "custom_step",
		Steps: map[string]*workflow.Step{
			"custom_step": {
				Name: "custom_step",
				Type: "custom_type",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	err := wf.Validate(nil, nil)

	assert.Error(t, err)
	assert.ErrorContains(t, err, "unknown step type")
}

// TestWorkflowValidate_MultipleSteps_WithChecker validates that checker is applied
// consistently to all steps in workflow
func TestWorkflowValidate_MultipleSteps_WithChecker(t *testing.T) {
	wf := workflow.Workflow{
		Name:    "multi_step_workflow",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      "custom_type_a",
				OnSuccess: "step2",
			},
			"step2": {
				Name:      "step2",
				Type:      "custom_type_b",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	// Checker that accepts both custom types
	checker := func(typeName string) bool {
		return typeName == "custom_type_a" || typeName == "custom_type_b"
	}

	err := wf.Validate(nil, checker)

	assert.NoError(t, err)
}

// TestWorkflowValidate_FirstStepCustomType_SecondStepUnknown validates checker
// correctly rejects when one custom type is unknown
func TestWorkflowValidate_FirstStepCustomType_SecondStepUnknown(t *testing.T) {
	wf := workflow.Workflow{
		Name:    "mixed_workflow",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      "known_custom_type",
				OnSuccess: "step2",
			},
			"step2": {
				Name:      "step2",
				Type:      "unknown_custom_type",
				OnSuccess: "end",
			},
			"end": {
				Name: "end",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	// Checker that only accepts one custom type
	checker := func(typeName string) bool {
		return typeName == "known_custom_type"
	}

	err := wf.Validate(nil, checker)

	assert.Error(t, err)
	assert.ErrorContains(t, err, "unknown step type")
}

// TestStepValidate_CustomType_WithRetryConfig validates that custom step types
// can have retry config validated correctly
func TestStepValidate_CustomType_WithRetryConfig(t *testing.T) {
	step := workflow.Step{
		Name: "custom_with_retry",
		Type: "custom_type",
		Retry: &workflow.RetryConfig{
			MaxAttempts: 3,
		},
	}

	checker := func(typeName string) bool {
		return typeName == "custom_type"
	}

	err := step.Validate(nil, checker)

	assert.NoError(t, err)
}

// TestStepValidate_CustomType_WithInvalidRetry validates that retry config
// validation still applies to custom step types
func TestStepValidate_CustomType_WithInvalidRetry(t *testing.T) {
	step := workflow.Step{
		Name: "custom_with_bad_retry",
		Type: "custom_type",
		Retry: &workflow.RetryConfig{
			MaxAttempts: -1, // Invalid: must be >= 1
		},
	}

	checker := func(typeName string) bool {
		return typeName == "custom_type"
	}

	err := step.Validate(nil, checker)

	assert.Error(t, err)
	assert.ErrorContains(t, err, "max_attempts must be >= 1")
}

// TestStepValidate_CustomType_CheckerPanicDoesNotOccur validates checker function
// is called and its result is used (controls behavior without panicking)
func TestStepValidate_CustomType_CheckerCalled(t *testing.T) {
	step := workflow.Step{
		Name: "custom_step",
		Type: "my_custom_type",
	}

	checkerCalled := false
	checker := func(typeName string) bool {
		checkerCalled = true
		return typeName == "my_custom_type"
	}

	err := step.Validate(nil, checker)

	assert.NoError(t, err)
	assert.True(t, checkerCalled, "checker should have been called")
}
