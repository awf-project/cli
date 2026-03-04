package workflow_test

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// B010: ValidStateProperties map missing JSON entry

func TestValidStateProperties_ContainsJSON(t *testing.T) {
	assert.True(t, workflow.ValidStateProperties["JSON"], "ValidStateProperties must include JSON (added by B010)")
}

func TestValidStateProperties_ExistingPropertiesUnchanged(t *testing.T) {
	for _, prop := range []string{"Output", "Stderr", "ExitCode", "Status", "Response", "TokensUsed"} {
		assert.True(t, workflow.ValidStateProperties[prop], "pre-existing property %q must remain valid", prop)
	}
}

func TestTemplateValidator_JSONPropertyPasses(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "json-prop-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo first",
				OnSuccess: "step2",
				OnFailure: "error",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{states.step1.JSON}}",
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			"error": {Name: "error", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure},
		},
	}

	result := workflow.NewTemplateValidator(w, newTestAnalyzer()).Validate()

	assert.False(t, result.HasErrors(), "{{states.step1.JSON}} must pass validation (B010)")
}

func TestTemplateValidator_LowercaseJSONSuggestsCorrection(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "lowercase-json-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo first",
				OnSuccess: "step2",
				OnFailure: "error",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{states.step1.json}}",
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			"error": {Name: "error", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure},
		},
	}

	result := workflow.NewTemplateValidator(w, newTestAnalyzer()).Validate()

	require.True(t, result.HasErrors(), "lowercase 'json' must fail validation")
	assert.Equal(t, workflow.ErrInvalidStateProperty, result.Errors[0].Code)
	assert.Contains(t, result.Errors[0].Message, "JSON", "error must suggest 'JSON' as correction")
}
