package workflow

import "time"

// ExecutionStatus represents the status of a workflow execution.
type ExecutionStatus string

const (
	StatusPending   ExecutionStatus = "pending"
	StatusRunning   ExecutionStatus = "running"
	StatusCompleted ExecutionStatus = "completed"
	StatusFailed    ExecutionStatus = "failed"
	StatusCancelled ExecutionStatus = "cancelled"
)

func (s ExecutionStatus) String() string {
	return string(s)
}

// StepState holds the execution state of a single step.
type StepState struct {
	Name        string
	Status      ExecutionStatus
	Output      string
	Stderr      string
	ExitCode    int
	Attempt     int
	Error       string
	StartedAt   time.Time
	CompletedAt time.Time
	Response    map[string]any // parsed JSON response from agent steps
	Tokens      int            // token usage from agent steps (deprecated, use TokensUsed)

	// F033: Conversation mode fields
	Conversation       *ConversationState  // conversation history and state (nil for non-conversation steps)
	TokensUsed         int                 // total tokens used in conversation mode
	ContextWindowState *ContextWindowState // context window management state (nil if not applicable)
}

// LoopContext holds the current loop iteration state.
type LoopContext struct {
	Item   any          // current item value (for_each)
	Index  int          // 0-based iteration index
	First  bool         // true on first iteration
	Last   bool         // true on last iteration (for_each only)
	Length int          // total items count (for_each only, -1 for while)
	Parent *LoopContext // reference to enclosing loop (F043: nested loops)
}

// ExecutionContext holds the runtime state of a workflow execution.
type ExecutionContext struct {
	WorkflowID   string
	WorkflowName string
	Status       ExecutionStatus
	CurrentStep  string
	Inputs       map[string]any
	States       map[string]StepState
	Env          map[string]string
	StartedAt    time.Time
	UpdatedAt    time.Time
	CompletedAt  time.Time
	CurrentLoop  *LoopContext // current loop iteration context (nil when not in a loop)
	CallStack    []string     // active workflow names for circular detection (F023)
}

// NewExecutionContext creates a new execution context.
func NewExecutionContext(workflowID, workflowName string) *ExecutionContext {
	now := time.Now()
	return &ExecutionContext{
		WorkflowID:   workflowID,
		WorkflowName: workflowName,
		Status:       StatusPending,
		Inputs:       make(map[string]any),
		States:       make(map[string]StepState),
		Env:          make(map[string]string),
		StartedAt:    now,
		UpdatedAt:    now,
	}
}

// SetInput sets an input value.
func (c *ExecutionContext) SetInput(key string, value any) {
	c.Inputs[key] = value
	c.UpdatedAt = time.Now()
}

// GetInput retrieves an input value.
func (c *ExecutionContext) GetInput(key string) (any, bool) {
	val, ok := c.Inputs[key]
	return val, ok
}

// SetStepState sets the state of a step.
func (c *ExecutionContext) SetStepState(stepName string, state StepState) {
	c.States[stepName] = state
	c.UpdatedAt = time.Now()
}

// GetStepState retrieves the state of a step.
func (c *ExecutionContext) GetStepState(stepName string) (StepState, bool) {
	state, ok := c.States[stepName]
	return state, ok
}

// PushCallStack adds a workflow name to the call stack.
// Used when entering a sub-workflow to track the call chain.
func (c *ExecutionContext) PushCallStack(workflowName string) {
	c.CallStack = append(c.CallStack, workflowName)
	c.UpdatedAt = time.Now()
}

// PopCallStack removes the last workflow name from the call stack.
// Used when exiting a sub-workflow. Does nothing if stack is empty.
func (c *ExecutionContext) PopCallStack() {
	if len(c.CallStack) > 0 {
		c.CallStack = c.CallStack[:len(c.CallStack)-1]
		c.UpdatedAt = time.Now()
	}
}

// IsInCallStack checks if a workflow name is already in the call stack.
// Used to detect circular workflow calls.
func (c *ExecutionContext) IsInCallStack(workflowName string) bool {
	for _, name := range c.CallStack {
		if name == workflowName {
			return true
		}
	}
	return false
}

// CallStackDepth returns the current depth of the call stack.
// Used to enforce maximum nesting depth for sub-workflows.
func (c *ExecutionContext) CallStackDepth() int {
	return len(c.CallStack)
}
