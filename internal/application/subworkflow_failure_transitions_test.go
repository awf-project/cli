package application_test

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	infraexpr "github.com/awf-project/cli/internal/infrastructure/expression"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteCallWorkflowStep_FailureHonorsOrderedTransitions(t *testing.T) {
	for _, continueOnError := range []bool{false, true} {
		for _, verificationExit := range []int{0, 1} {
			name := "failure"
			if continueOnError {
				name = "continue"
			}
			if verificationExit == 0 {
				name += "_verified"
			} else {
				name += "_remaining"
			}
			t.Run(name, func(t *testing.T) {
				child := &workflow.Workflow{Name: "feedback", Initial: "failed", Steps: map[string]*workflow.Step{
					"failed": {Name: "failed", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure, Message: "invalid feedback"},
				}}
				parent := &workflow.Workflow{Name: "fix-errors", Initial: "verify", Steps: map[string]*workflow.Step{
					"verify": {Name: "verify", Type: workflow.StepTypeCommand, Command: "verify", OnSuccess: "call", OnFailure: "call"},
					"call": {
						Name: "call", Type: workflow.StepTypeCallWorkflow,
						CallWorkflow:    &workflow.CallWorkflowConfig{Workflow: "feedback"},
						ContinueOnError: continueOnError,
						OnFailure:       "wrong",
						Transitions: workflow.Transitions{
							{When: "states.verify.ExitCode == 0", Goto: "lint"},
							{Goto: "remaining"},
						},
					},
					"lint":      {Name: "lint", Type: workflow.StepTypeTerminal},
					"remaining": {Name: "remaining", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure, Message: "test errors remain"},
					"wrong":     {Name: "wrong", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure, Message: "wrong route"},
				}}
				service, _ := NewTestHarnessWithEvaluator(t, infraexpr.NewExprEvaluator()).
					WithWorkflow("feedback", child).WithWorkflow("fix-errors", parent).
					WithCommandResult("verify", &ports.CommandResult{ExitCode: verificationExit}).Build()
				result, err := service.RunWithWorkflowAndRunID(context.Background(), parent, nil, name)
				if verificationExit == 0 {
					require.NoError(t, err)
					assert.Equal(t, "lint", result.CurrentStep)
				} else {
					require.ErrorContains(t, err, "test errors remain")
					assert.Equal(t, "remaining", result.CurrentStep)
				}
				call, ok := result.GetStepState("call")
				require.True(t, ok)
				assert.Equal(t, workflow.StatusFailed, call.Status)
			})
		}
	}
}
