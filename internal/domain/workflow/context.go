package workflow

import (
	"sync"
	"time"
)

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
	JSON        any            // parsed JSON output when output_format: json is specified (map[string]any or []any)
	// F033: Conversation mode fields
	Conversation       *ConversationState  // conversation history and state (nil for non-conversation steps)
	TokensUsed         int                 // total tokens used in conversation mode
	ContextWindowState *ContextWindowState // context window management state (nil if not applicable)

	// C019: Output streaming fields for memory management
	OutputPath string // Path to temp file if output was streamed (empty if in-memory)
	StderrPath string // Path to temp file if stderr was streamed (empty if in-memory)
	Truncated  bool   // True if output was truncated (not streamed)
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
// Thread-safe for concurrent access during parallel execution.
type ExecutionContext struct {
	mu           sync.RWMutex // protects concurrent map access
	WorkflowID   string
	WorkflowName string
	Status       ExecutionStatus
	CurrentStep  string
	ExitCode     int // process exit code propagated from terminal steps (FR-004)
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
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Inputs[key] = value
	c.UpdatedAt = time.Now()
}

// GetInput retrieves an input value.
func (c *ExecutionContext) GetInput(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.Inputs[key]
	return val, ok
}

// SetStepState sets the state of a step.
func (c *ExecutionContext) SetStepState(stepName string, state StepState) { //nolint:gocritic // hugeParam: StepState passed by value to avoid pointer indirection overhead
	c.mu.Lock()
	defer c.mu.Unlock()
	c.States[stepName] = state
	c.UpdatedAt = time.Now()
}

// GetStepState retrieves the state of a step.
func (c *ExecutionContext) GetStepState(stepName string) (StepState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	state, ok := c.States[stepName]
	return state, ok
}

// GetAllStepStates returns a copy of all step states in a thread-safe manner.
func (c *ExecutionContext) GetAllStepStates() map[string]StepState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Return a copy to prevent concurrent access to the map
	states := make(map[string]StepState, len(c.States))
	for k, v := range c.States { //nolint:gocritic // rangeValCopy: defensive copy required for thread-safety, consistent with value-semantic design (see SetStepState)
		states[k] = v
	}
	return states
}

// PushCallStack adds a workflow name to the call stack.
// Used when entering a sub-workflow to track the call chain.
func (c *ExecutionContext) PushCallStack(workflowName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CallStack = append(c.CallStack, workflowName)
	c.UpdatedAt = time.Now()
}

// PopCallStack removes the last workflow name from the call stack.
// Used when exiting a sub-workflow. Does nothing if stack is empty.
func (c *ExecutionContext) PopCallStack() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.CallStack) > 0 {
		c.CallStack = c.CallStack[:len(c.CallStack)-1]
		c.UpdatedAt = time.Now()
	}
}

// IsInCallStack checks if a workflow name is already in the call stack.
// Used to detect circular workflow calls.
func (c *ExecutionContext) IsInCallStack(workflowName string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
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
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.CallStack)
}
