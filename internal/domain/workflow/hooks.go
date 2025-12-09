package workflow

// HookAction represents a single action in a hook.
// Either Log or Command should be set, not both.
type HookAction struct {
	Log     string // log message to output
	Command string // shell command to execute
}

// Hook represents a list of actions to execute.
type Hook []HookAction

// WorkflowHooks defines hooks at workflow level.
type WorkflowHooks struct {
	WorkflowStart  Hook // before workflow execution
	WorkflowEnd    Hook // after successful completion
	WorkflowError  Hook // on workflow error
	WorkflowCancel Hook // on user cancellation (Ctrl-C)
}

// StepHooks defines hooks at step level.
type StepHooks struct {
	Pre  Hook // before step execution
	Post Hook // after step execution
}
