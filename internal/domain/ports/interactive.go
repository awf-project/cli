package ports

import "github.com/awf-project/awf/internal/domain/workflow"

// StepPresenter defines the contract for displaying step lifecycle information.
// Implementations handle rendering of step execution progress and outcomes.
type StepPresenter interface {
	// ShowHeader displays the interactive mode header with workflow name.
	ShowHeader(workflowName string)

	// ShowStepDetails displays step information before execution.
	ShowStepDetails(info *workflow.InteractiveStepInfo)

	// ShowExecuting displays a message indicating step execution is in progress.
	ShowExecuting(stepName string)

	// ShowStepResult displays the outcome of step execution.
	ShowStepResult(state *workflow.StepState, nextStep string)
}

// StatusPresenter defines the contract for displaying terminal and outcome states.
// Implementations handle rendering of workflow completion, abortion, and errors.
type StatusPresenter interface {
	// ShowAborted displays a message indicating workflow was aborted.
	ShowAborted()

	// ShowSkipped displays a message indicating step was skipped.
	ShowSkipped(stepName string, nextStep string)

	// ShowCompleted displays a message indicating workflow completed.
	ShowCompleted(status workflow.ExecutionStatus)

	// ShowError displays an error message.
	ShowError(err error)
}

// UserInteraction defines the contract for interactive user input and context display.
// Implementations handle user prompts, input editing, and runtime context visualization.
type UserInteraction interface {
	// PromptAction prompts the user for an action and returns their choice.
	// hasRetry indicates whether the [r]etry option should be available.
	PromptAction(hasRetry bool) (workflow.InteractiveAction, error)

	// EditInput prompts the user to edit an input value.
	// Returns the new value and any error from parsing.
	EditInput(name string, current any) (any, error)

	// ShowContext displays the current runtime context (inputs, states).
	ShowContext(ctx *workflow.RuntimeContext)
}

// InteractivePrompt defines the composite contract for interactive mode user interaction.
// Implementations handle UI rendering and user input during step-by-step execution.
// This interface embeds StepPresenter, StatusPresenter, and UserInteraction for backward compatibility.
type InteractivePrompt interface {
	StepPresenter
	StatusPresenter
	UserInteraction
}
