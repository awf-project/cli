package diagram

// Diagram Test Helpers
// Shared test fixtures and utilities for diagram generation tests

import (
	"github.com/awf-project/cli/internal/domain/workflow"
)

// Helper function to create a simple test workflow
func createSimpleWorkflow() *workflow.Workflow {
	return &workflow.Workflow{
		Name:    "simple",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo start",
				OnSuccess: "end",
			},
			"end": {
				Name:   "end",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}
}
