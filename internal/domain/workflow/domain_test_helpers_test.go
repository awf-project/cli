package workflow_test

// C013: Domain test file splitting - Shared test infrastructure
// This file contains shared test utilities used across multiple test files in the workflow domain package.

import (
	"fmt"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
)

// testAnalyzer: Template analysis helper for tests

// testAnalyzer wraps pkg/interpolation for testing template validation.
type testAnalyzer struct{}

func newTestAnalyzer() *testAnalyzer {
	return &testAnalyzer{}
}

func (a *testAnalyzer) ExtractReferences(template string) ([]workflow.TemplateReference, error) {
	refs, err := interpolation.ExtractReferences(template)
	if err != nil {
		return nil, fmt.Errorf("extracting references from %q: %w", template, err)
	}

	result := make([]workflow.TemplateReference, len(refs))
	for i, ref := range refs {
		result[i] = workflow.TemplateReference{
			Type:      convertRefType(ref.Type),
			Namespace: ref.Namespace,
			Path:      ref.Path,
			Property:  ref.Property,
			Raw:       ref.Raw,
		}
	}
	return result, nil
}

// convertRefType converts interpolation.ReferenceType to workflow.ReferenceType
func convertRefType(t interpolation.ReferenceType) workflow.ReferenceType {
	switch t {
	case interpolation.TypeInputs:
		return workflow.TypeInputs
	case interpolation.TypeStates:
		return workflow.TypeStates
	case interpolation.TypeWorkflow:
		return workflow.TypeWorkflow
	case interpolation.TypeEnv:
		return workflow.TypeEnv
	case interpolation.TypeError:
		return workflow.TypeError
	case interpolation.TypeContext:
		return workflow.TypeContext
	case interpolation.TypeLoop:
		return workflow.TypeLoop
	default:
		return workflow.TypeUnknown
	}
}

// newTestWorkflow creates a basic test workflow with start, done, and error states
func newTestWorkflow() *workflow.Workflow {
	return &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Inputs: []workflow.Input{
			{Name: "name", Type: "string", Required: true},
			{Name: "count", Type: "integer", Default: 1},
		},
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{inputs.name}}",
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
}

// newLinearWorkflow creates a linear workflow with three sequential steps
func newLinearWorkflow() *workflow.Workflow {
	return &workflow.Workflow{
		Name:    "linear-workflow",
		Initial: "step1",
		Inputs: []workflow.Input{
			{Name: "input1", Type: "string"},
		},
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo step1",
				OnSuccess: "step2",
				OnFailure: "error",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo step2",
				OnSuccess: "step3",
				OnFailure: "error",
			},
			"step3": {
				Name:      "step3",
				Type:      workflow.StepTypeCommand,
				Command:   "echo step3",
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
}

// newLoopWorkflow creates a workflow with loop configuration for testing
func newLoopWorkflow() *workflow.Workflow {
	return &workflow.Workflow{
		Name:    "loop-workflow",
		Initial: "loop_step",
		Inputs: []workflow.Input{
			{Name: "limit", Type: "integer", Default: 5},
			{Name: "threshold", Type: "integer", Default: 10},
			{Name: "items_list", Type: "string", Default: "a,b,c"},
		},
		Steps: map[string]*workflow.Step{
			"loop_step": {
				Name:      "loop_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo iteration",
				OnSuccess: "done",
				OnFailure: "error",
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         "{{inputs.items_list}}",
					Body:          []string{"loop_body"},
					MaxIterations: 10,
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
}

// containsString checks if string s contains substr
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// indexOf returns the index of item in slice, or -1 if not found
func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}
