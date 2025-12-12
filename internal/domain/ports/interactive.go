package ports

import "github.com/vanoix/awf/internal/domain/workflow"

// InteractivePrompt defines the contract for interactive mode user interaction.
// Implementations handle UI rendering and user input during step-by-step execution.
type InteractivePrompt interface {
	// ShowHeader displays the interactive mode header with workflow name.
	ShowHeader(workflowName string)

	// ShowStepDetails displays step information before execution.
	ShowStepDetails(info *workflow.InteractiveStepInfo)

	// PromptAction prompts the user for an action and returns their choice.
	// hasRetry indicates whether the [r]etry option should be available.
	PromptAction(hasRetry bool) (workflow.InteractiveAction, error)

	// ShowExecuting displays a message indicating step execution is in progress.
	ShowExecuting(stepName string)

	// ShowStepResult displays the outcome of step execution.
	ShowStepResult(state *workflow.StepState, nextStep string)

	// ShowContext displays the current runtime context (inputs, states).
	ShowContext(ctx *workflow.RuntimeContext)

	// EditInput prompts the user to edit an input value.
	// Returns the new value and any error from parsing.
	EditInput(name string, current any) (any, error)

	// ShowAborted displays a message indicating workflow was aborted.
	ShowAborted()

	// ShowSkipped displays a message indicating step was skipped.
	ShowSkipped(stepName string, nextStep string)

	// ShowCompleted displays a message indicating workflow completed.
	ShowCompleted(status workflow.ExecutionStatus)

	// ShowError displays an error message.
	ShowError(err error)
}
