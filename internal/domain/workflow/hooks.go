package workflow

// Either Log or Command should be set, not both.
type HookAction struct {
	Log     string
	Command string
}

type Hook []HookAction

type WorkflowHooks struct {
	WorkflowStart  Hook
	WorkflowEnd    Hook
	WorkflowError  Hook
	WorkflowCancel Hook
}

type StepHooks struct {
	Pre  Hook
	Post Hook
}
