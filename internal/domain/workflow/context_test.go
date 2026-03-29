package workflow_test

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutionStatusString(t *testing.T) {
	statuses := []struct {
		status workflow.ExecutionStatus
		want   string
	}{
		{workflow.StatusPending, "pending"},
		{workflow.StatusRunning, "running"},
		{workflow.StatusCompleted, "completed"},
		{workflow.StatusFailed, "failed"},
		{workflow.StatusCancelled, "cancelled"},
	}

	for _, tt := range statuses {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("ExecutionStatus.String() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestNewExecutionContext(t *testing.T) {
	ctx := workflow.NewExecutionContext("test-workflow-123", "analyze-code")

	if ctx.WorkflowID != "test-workflow-123" {
		t.Errorf("expected WorkflowID 'test-workflow-123', got '%s'", ctx.WorkflowID)
	}
	if ctx.WorkflowName != "analyze-code" {
		t.Errorf("expected WorkflowName 'analyze-code', got '%s'", ctx.WorkflowName)
	}
	if ctx.Status != workflow.StatusPending {
		t.Errorf("expected Status 'pending', got '%s'", ctx.Status)
	}
	if ctx.Inputs == nil {
		t.Error("expected Inputs to be initialized")
	}
	if ctx.States == nil {
		t.Error("expected States to be initialized")
	}
	if ctx.Env == nil {
		t.Error("expected Env to be initialized")
	}
}

func TestExecutionContextSetInput(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "name")

	ctx.SetInput("file_path", "/tmp/test.py")
	ctx.SetInput("count", 42)

	val, ok := ctx.GetInput("file_path")
	if !ok {
		t.Error("expected input 'file_path' to exist")
	}
	if val != "/tmp/test.py" {
		t.Errorf("expected '/tmp/test.py', got '%v'", val)
	}

	valInt, ok := ctx.GetInput("count")
	if !ok {
		t.Error("expected input 'count' to exist")
	}
	if valInt != 42 {
		t.Errorf("expected 42, got '%v'", valInt)
	}

	_, ok = ctx.GetInput("nonexistent")
	if ok {
		t.Error("expected nonexistent input to not exist")
	}
}

func TestExecutionContextStepState(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "name")

	state := workflow.StepState{
		Name:      "validate",
		Status:    workflow.StatusCompleted,
		Output:    "valid",
		ExitCode:  0,
		Attempt:   1,
		StartedAt: time.Now().Add(-time.Second),
	}
	state.CompletedAt = time.Now()

	ctx.SetStepState("validate", state)

	retrieved, ok := ctx.GetStepState("validate")
	if !ok {
		t.Error("expected step state 'validate' to exist")
	}
	if retrieved.Output != "valid" {
		t.Errorf("expected output 'valid', got '%s'", retrieved.Output)
	}
	if retrieved.Status != workflow.StatusCompleted {
		t.Errorf("expected status 'completed', got '%s'", retrieved.Status)
	}
	if retrieved.Attempt != 1 {
		t.Errorf("expected attempt 1, got %d", retrieved.Attempt)
	}

	_, ok = ctx.GetStepState("nonexistent")
	if ok {
		t.Error("expected nonexistent step state to not exist")
	}
}

func TestExecutionContextUpdateTimestamp(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "name")
	initialUpdate := ctx.UpdatedAt

	time.Sleep(time.Millisecond)
	ctx.SetInput("key", "value")

	if !ctx.UpdatedAt.After(initialUpdate) {
		t.Error("expected UpdatedAt to be updated after SetInput")
	}
}

func TestStepStateFields(t *testing.T) {
	state := workflow.StepState{
		Name:        "test",
		Status:      workflow.StatusFailed,
		Output:      "stdout content",
		Stderr:      "error output",
		ExitCode:    1,
		Attempt:     3,
		Error:       "command failed",
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	if state.Name != "test" {
		t.Errorf("expected Name 'test', got '%s'", state.Name)
	}
	if state.Stderr != "error output" {
		t.Errorf("expected Stderr 'error output', got '%s'", state.Stderr)
	}
	if state.Error != "command failed" {
		t.Errorf("expected Error 'command failed', got '%s'", state.Error)
	}
}

func TestLoopContext_JSONSerialize_SingleLevel(t *testing.T) {
	ctx := &workflow.LoopContext{
		Item:   "test-item",
		Index:  2,
		First:  false,
		Last:   true,
		Length: 5,
		Parent: nil,
	}

	data, err := json.Marshal(ctx)
	require.NoError(t, err)

	var decoded workflow.LoopContext
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "test-item", decoded.Item)
	assert.Equal(t, 2, decoded.Index)
	assert.False(t, decoded.First)
	assert.True(t, decoded.Last)
	assert.Equal(t, 5, decoded.Length)
	assert.Nil(t, decoded.Parent)
}

func TestLoopContext_JSONSerialize_NestedTwoLevels(t *testing.T) {
	outer := &workflow.LoopContext{
		Item:   "outer-item",
		Index:  0,
		First:  true,
		Last:   false,
		Length: 3,
		Parent: nil,
	}
	inner := &workflow.LoopContext{
		Item:   "inner-item",
		Index:  1,
		First:  false,
		Last:   true,
		Length: 2,
		Parent: outer,
	}

	data, err := json.Marshal(inner)
	require.NoError(t, err)

	var decoded workflow.LoopContext
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify inner
	assert.Equal(t, "inner-item", decoded.Item)
	assert.Equal(t, 1, decoded.Index)
	assert.False(t, decoded.First)
	assert.True(t, decoded.Last)
	assert.Equal(t, 2, decoded.Length)

	// Verify outer (parent)
	require.NotNil(t, decoded.Parent)
	assert.Equal(t, "outer-item", decoded.Parent.Item)
	assert.Equal(t, 0, decoded.Parent.Index)
	assert.True(t, decoded.Parent.First)
	assert.False(t, decoded.Parent.Last)
	assert.Equal(t, 3, decoded.Parent.Length)
	assert.Nil(t, decoded.Parent.Parent)
}

func TestLoopContext_JSONSerialize_TripleNesting(t *testing.T) {
	l1 := &workflow.LoopContext{
		Item: "L1", Index: 0, First: true, Last: true, Length: 1, Parent: nil,
	}
	l2 := &workflow.LoopContext{
		Item: "L2", Index: 1, First: false, Last: false, Length: 5, Parent: l1,
	}
	l3 := &workflow.LoopContext{
		Item: "L3", Index: 2, First: false, Last: true, Length: 3, Parent: l2,
	}

	data, err := json.Marshal(l3)
	require.NoError(t, err)

	var decoded workflow.LoopContext
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify L3
	assert.Equal(t, "L3", decoded.Item)

	// Verify L2
	require.NotNil(t, decoded.Parent)
	assert.Equal(t, "L2", decoded.Parent.Item)
	assert.Equal(t, 1, decoded.Parent.Index)
	assert.Equal(t, 5, decoded.Parent.Length)

	// Verify L1
	require.NotNil(t, decoded.Parent.Parent)
	assert.Equal(t, "L1", decoded.Parent.Parent.Item)

	// No L0
	assert.Nil(t, decoded.Parent.Parent.Parent)
}

func TestLoopContext_JSONSerialize_ComplexItems(t *testing.T) {
	tests := []struct {
		name string
		item any
	}{
		{name: "string", item: "test"},
		{name: "int", item: float64(42)}, // JSON numbers decode as float64
		{name: "float", item: 3.14},
		{name: "bool", item: true},
		{name: "nil", item: nil},
		{name: "slice", item: []any{"a", "b", "c"}},
		{name: "map", item: map[string]any{"key": "value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &workflow.LoopContext{
				Item:   tt.item,
				Index:  0,
				First:  true,
				Last:   true,
				Length: 1,
				Parent: nil,
			}

			data, err := json.Marshal(ctx)
			require.NoError(t, err)

			var decoded workflow.LoopContext
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.item, decoded.Item)
		})
	}
}

func TestLoopContext_JSONSerialize_WhileLoop(t *testing.T) {
	// While loops have Length = -1 and nil Item
	ctx := &workflow.LoopContext{
		Item:   nil,
		Index:  10,
		First:  false,
		Last:   false, // unknown for while
		Length: -1,    // sentinel for while loops
		Parent: nil,
	}

	data, err := json.Marshal(ctx)
	require.NoError(t, err)

	var decoded workflow.LoopContext
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Nil(t, decoded.Item)
	assert.Equal(t, 10, decoded.Index)
	assert.False(t, decoded.First)
	assert.False(t, decoded.Last)
	assert.Equal(t, -1, decoded.Length)
}

func TestLoopContext_JSONSerialize_MixedLoopTypes(t *testing.T) {
	// Outer: for_each loop
	forEachLoop := &workflow.LoopContext{
		Item:   "batch-1",
		Index:  0,
		First:  true,
		Last:   false,
		Length: 3,
		Parent: nil,
	}

	// Inner: while loop inside for_each
	whileLoop := &workflow.LoopContext{
		Item:   nil,
		Index:  5,
		First:  false,
		Last:   false,
		Length: -1,
		Parent: forEachLoop,
	}

	data, err := json.Marshal(whileLoop)
	require.NoError(t, err)

	var decoded workflow.LoopContext
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify inner (while)
	assert.Nil(t, decoded.Item)
	assert.Equal(t, 5, decoded.Index)
	assert.Equal(t, -1, decoded.Length)

	// Verify outer (for_each)
	require.NotNil(t, decoded.Parent)
	assert.Equal(t, "batch-1", decoded.Parent.Item)
	assert.Equal(t, 3, decoded.Parent.Length)
}

func TestLoopContext_JSONSerialize_DeepNesting(t *testing.T) {
	// Build a 5-level deep chain
	var prev *workflow.LoopContext
	for i := 0; i < 5; i++ {
		ctx := &workflow.LoopContext{
			Item:   i,
			Index:  i,
			First:  i == 0,
			Last:   i == 4,
			Length: 5,
			Parent: prev,
		}
		prev = ctx
	}

	// prev is now the innermost (L5)
	data, err := json.Marshal(prev)
	require.NoError(t, err)

	var decoded workflow.LoopContext
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify chain depth
	current := &decoded
	depth := 0
	for current != nil {
		depth++
		current = current.Parent
	}
	assert.Equal(t, 5, depth)
}

func TestLoopContext_JSONSerialize_ExecutionContext(t *testing.T) {
	// Test that ExecutionContext with LoopContext serializes correctly
	execCtx := workflow.NewExecutionContext("test-id", "test-wf")
	execCtx.CurrentLoop = &workflow.LoopContext{
		Item:   "current-item",
		Index:  2,
		First:  false,
		Last:   false,
		Length: 5,
		Parent: &workflow.LoopContext{
			Item:   "parent-item",
			Index:  1,
			First:  false,
			Last:   true,
			Length: 2,
			Parent: nil,
		},
	}

	data, err := json.Marshal(execCtx)
	require.NoError(t, err)

	var decoded workflow.ExecutionContext
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	require.NotNil(t, decoded.CurrentLoop)
	assert.Equal(t, "current-item", decoded.CurrentLoop.Item)
	assert.Equal(t, 2, decoded.CurrentLoop.Index)

	require.NotNil(t, decoded.CurrentLoop.Parent)
	assert.Equal(t, "parent-item", decoded.CurrentLoop.Parent.Item)
	assert.Nil(t, decoded.CurrentLoop.Parent.Parent)
}

func TestLoopContext_JSONSerialize_NilParent(t *testing.T) {
	// Verify nil Parent serializes correctly
	ctx := &workflow.LoopContext{
		Item:   "item",
		Index:  0,
		First:  true,
		Last:   true,
		Length: 1,
		Parent: nil,
	}

	data, err := json.Marshal(ctx)
	require.NoError(t, err)

	// Verify JSON doesn't contain "Parent":null (it should be omitted or null)
	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	// Parent should be nil/null in JSON
	parent, exists := raw["Parent"]
	if exists {
		assert.Nil(t, parent)
	}
}

func TestNewExecutionContext_CallStackInitialized(t *testing.T) {
	ctx := workflow.NewExecutionContext("test-id", "test-workflow")

	// CallStack should be nil (not initialized) for new contexts
	// This is fine - Go handles nil slices gracefully
	assert.Equal(t, 0, ctx.CallStackDepth())
	assert.False(t, ctx.IsInCallStack("any-workflow"))
}

func TestPushCallStack_SingleWorkflow(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "parent-workflow")

	ctx.PushCallStack("child-workflow")

	assert.Equal(t, 1, ctx.CallStackDepth())
	assert.True(t, ctx.IsInCallStack("child-workflow"))
	assert.False(t, ctx.IsInCallStack("other-workflow"))
}

func TestPushCallStack_MultipleWorkflows(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "root")

	ctx.PushCallStack("workflow-a")
	ctx.PushCallStack("workflow-b")
	ctx.PushCallStack("workflow-c")

	assert.Equal(t, 3, ctx.CallStackDepth())
	assert.True(t, ctx.IsInCallStack("workflow-a"))
	assert.True(t, ctx.IsInCallStack("workflow-b"))
	assert.True(t, ctx.IsInCallStack("workflow-c"))
}

func TestPushCallStack_UpdatesTimestamp(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "name")
	initialUpdate := ctx.UpdatedAt

	time.Sleep(time.Millisecond)
	ctx.PushCallStack("child-workflow")

	assert.True(t, ctx.UpdatedAt.After(initialUpdate), "UpdatedAt should be updated after PushCallStack")
}

func TestPopCallStack_RemovesLastWorkflow(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "root")
	ctx.PushCallStack("workflow-a")
	ctx.PushCallStack("workflow-b")

	ctx.PopCallStack()

	assert.Equal(t, 1, ctx.CallStackDepth())
	assert.True(t, ctx.IsInCallStack("workflow-a"))
	assert.False(t, ctx.IsInCallStack("workflow-b"))
}

func TestPopCallStack_EmptyStackDoesNotPanic(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "root")

	// Should not panic when popping from empty stack
	assert.NotPanics(t, func() {
		ctx.PopCallStack()
	})
	assert.Equal(t, 0, ctx.CallStackDepth())
}

func TestPopCallStack_MultiplePopsTillEmpty(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "root")
	ctx.PushCallStack("a")
	ctx.PushCallStack("b")
	ctx.PushCallStack("c")

	ctx.PopCallStack()
	assert.Equal(t, 2, ctx.CallStackDepth())

	ctx.PopCallStack()
	assert.Equal(t, 1, ctx.CallStackDepth())

	ctx.PopCallStack()
	assert.Equal(t, 0, ctx.CallStackDepth())

	// Extra pop should be safe
	ctx.PopCallStack()
	assert.Equal(t, 0, ctx.CallStackDepth())
}

func TestPopCallStack_UpdatesTimestamp(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "name")
	ctx.PushCallStack("child")

	time.Sleep(time.Millisecond)
	initialUpdate := ctx.UpdatedAt

	time.Sleep(time.Millisecond)
	ctx.PopCallStack()

	assert.True(t, ctx.UpdatedAt.After(initialUpdate), "UpdatedAt should be updated after PopCallStack")
}

func TestPopCallStack_EmptyStackDoesNotUpdateTimestamp(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "name")
	initialUpdate := ctx.UpdatedAt

	time.Sleep(time.Millisecond)
	ctx.PopCallStack() // no-op on empty stack

	assert.Equal(t, initialUpdate, ctx.UpdatedAt, "UpdatedAt should not change when popping empty stack")
}

func TestIsInCallStack_EmptyStackReturnsFalse(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "root")

	assert.False(t, ctx.IsInCallStack("any-workflow"))
	assert.False(t, ctx.IsInCallStack(""))
}

func TestIsInCallStack_ExactMatchRequired(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "root")
	ctx.PushCallStack("my-workflow")

	assert.True(t, ctx.IsInCallStack("my-workflow"))
	assert.False(t, ctx.IsInCallStack("my-workflow-extended"))
	assert.False(t, ctx.IsInCallStack("my"))
	assert.False(t, ctx.IsInCallStack("MY-WORKFLOW")) // case sensitive
}

func TestIsInCallStack_FindsAnyPositionInStack(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "root")
	ctx.PushCallStack("first")
	ctx.PushCallStack("second")
	ctx.PushCallStack("third")

	// All positions should be found
	assert.True(t, ctx.IsInCallStack("first"))
	assert.True(t, ctx.IsInCallStack("second"))
	assert.True(t, ctx.IsInCallStack("third"))
}

func TestCallStackDepth_EmptyStack(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "root")
	assert.Equal(t, 0, ctx.CallStackDepth())
}

func TestCallStackDepth_AfterPushAndPop(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "root")

	ctx.PushCallStack("a")
	assert.Equal(t, 1, ctx.CallStackDepth())

	ctx.PushCallStack("b")
	assert.Equal(t, 2, ctx.CallStackDepth())

	ctx.PopCallStack()
	assert.Equal(t, 1, ctx.CallStackDepth())

	ctx.PopCallStack()
	assert.Equal(t, 0, ctx.CallStackDepth())
}

func TestCallStackDepth_MaxDepthScenario(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "root")

	// Push up to MaxCallStackDepth
	for i := 0; i < workflow.MaxCallStackDepth; i++ {
		ctx.PushCallStack("workflow-" + string(rune('a'+i)))
	}

	assert.Equal(t, workflow.MaxCallStackDepth, ctx.CallStackDepth())
}

func TestCallStack_CircularDetectionWorkflow(t *testing.T) {
	// Simulates: parent → child-a → child-b → (attempt to call parent again)
	ctx := workflow.NewExecutionContext("id", "parent-workflow")

	// Enter parent
	ctx.PushCallStack("parent-workflow")
	assert.Equal(t, 1, ctx.CallStackDepth())

	// Enter child-a
	ctx.PushCallStack("child-a")
	assert.Equal(t, 2, ctx.CallStackDepth())

	// Enter child-b
	ctx.PushCallStack("child-b")
	assert.Equal(t, 3, ctx.CallStackDepth())

	// Attempt to call parent again - should detect circular reference
	assert.True(t, ctx.IsInCallStack("parent-workflow"), "should detect circular call to parent-workflow")
	assert.True(t, ctx.IsInCallStack("child-a"), "should detect circular call to child-a")

	// child-c is not in stack, so it's safe to call
	assert.False(t, ctx.IsInCallStack("child-c"))

	// Unwind the stack
	ctx.PopCallStack() // exit child-b
	ctx.PopCallStack() // exit child-a
	ctx.PopCallStack() // exit parent

	assert.Equal(t, 0, ctx.CallStackDepth())
	assert.False(t, ctx.IsInCallStack("parent-workflow"))
}

func TestCallStack_PushPopPushSameWorkflow(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "root")

	ctx.PushCallStack("workflow-a")
	assert.True(t, ctx.IsInCallStack("workflow-a"))

	ctx.PopCallStack()
	assert.False(t, ctx.IsInCallStack("workflow-a"))

	// Push again after pop - should work
	ctx.PushCallStack("workflow-a")
	assert.True(t, ctx.IsInCallStack("workflow-a"))
	assert.Equal(t, 1, ctx.CallStackDepth())
}

func TestCallStack_JSONSerialize(t *testing.T) {
	ctx := workflow.NewExecutionContext("test-id", "test-wf")
	ctx.PushCallStack("parent")
	ctx.PushCallStack("child-a")
	ctx.PushCallStack("child-b")

	data, err := json.Marshal(ctx)
	require.NoError(t, err)

	var decoded workflow.ExecutionContext
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify call stack is preserved
	assert.Equal(t, 3, decoded.CallStackDepth())
	assert.True(t, decoded.IsInCallStack("parent"))
	assert.True(t, decoded.IsInCallStack("child-a"))
	assert.True(t, decoded.IsInCallStack("child-b"))
}

func TestCallStack_JSONSerialize_EmptyStack(t *testing.T) {
	ctx := workflow.NewExecutionContext("test-id", "test-wf")

	data, err := json.Marshal(ctx)
	require.NoError(t, err)

	var decoded workflow.ExecutionContext
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, 0, decoded.CallStackDepth())
}

func TestCallStack_StackOrderPreserved(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "root")

	workflows := []string{"first", "second", "third", "fourth"}
	for _, w := range workflows {
		ctx.PushCallStack(w)
	}

	// Pop in reverse order and verify
	for i := len(workflows) - 1; i >= 0; i-- {
		assert.True(t, ctx.IsInCallStack(workflows[i]))
		ctx.PopCallStack()
		assert.False(t, ctx.IsInCallStack(workflows[i]))
	}
}

// Component: step_state_extension
// Feature: F033

func TestStepState_ConversationFields_NilByDefault(t *testing.T) {
	// Happy path: New StepState should have nil conversation fields
	state := workflow.StepState{
		Name:   "test-step",
		Status: workflow.StatusPending,
	}

	assert.Nil(t, state.Conversation, "Conversation should be nil for non-conversation steps")
	assert.Equal(t, 0, state.TokensUsed, "TokensUsed should default to 0")
	assert.Nil(t, state.ContextWindowState, "ContextWindowState should be nil by default")
}

func TestStepState_ConversationFields_SetConversation(t *testing.T) {
	// Happy path: Set conversation state on StepState
	state := workflow.StepState{
		Name:   "agent-step",
		Status: workflow.StatusRunning,
	}

	conversation := &workflow.ConversationState{
		Turns: []workflow.Turn{
			{Role: workflow.TurnRoleSystem, Content: "You are a helpful assistant", Tokens: 10},
			{Role: workflow.TurnRoleUser, Content: "Hello", Tokens: 5},
		},
		TotalTurns:  2,
		TotalTokens: 15,
	}

	state.Conversation = conversation
	state.TokensUsed = 15

	assert.NotNil(t, state.Conversation)
	assert.Equal(t, 2, state.Conversation.TotalTurns)
	assert.Equal(t, 15, state.TokensUsed)
}

func TestStepState_ConversationFields_SetContextWindowState(t *testing.T) {
	// Happy path: Set context window state on StepState
	state := workflow.StepState{
		Name:   "agent-step",
		Status: workflow.StatusCompleted,
	}

	contextWindow := &workflow.ContextWindowState{
		Strategy:        workflow.StrategySlidingWindow,
		TruncationCount: 2,
		TurnsDropped:    5,
		TokensDropped:   1200,
		LastTruncatedAt: 8,
	}

	state.ContextWindowState = contextWindow

	assert.NotNil(t, state.ContextWindowState)
	assert.Equal(t, workflow.StrategySlidingWindow, state.ContextWindowState.Strategy)
	assert.Equal(t, 2, state.ContextWindowState.TruncationCount)
	assert.Equal(t, 5, state.ContextWindowState.TurnsDropped)
	assert.Equal(t, 1200, state.ContextWindowState.TokensDropped)
	assert.Equal(t, 8, state.ContextWindowState.LastTruncatedAt)
}

func TestStepState_ConversationFields_CompleteConversationMode(t *testing.T) {
	// Happy path: Full conversation mode step with all fields populated
	state := workflow.StepState{
		Name:        "refine-code",
		Status:      workflow.StatusCompleted,
		Output:      "APPROVED",
		StartedAt:   time.Now().Add(-5 * time.Second),
		CompletedAt: time.Now(),
		Response: map[string]any{
			"status": "approved",
			"issues": 0,
		},
		Conversation: &workflow.ConversationState{
			Turns: []workflow.Turn{
				{Role: workflow.TurnRoleSystem, Content: "You are a code reviewer", Tokens: 50},
				{Role: workflow.TurnRoleUser, Content: "Review this code...", Tokens: 500},
				{Role: workflow.TurnRoleAssistant, Content: "I found these issues...", Tokens: 800},
				{Role: workflow.TurnRoleUser, Content: "Fix the issues", Tokens: 20},
				{Role: workflow.TurnRoleAssistant, Content: "APPROVED", Tokens: 600},
			},
			TotalTurns:  5,
			TotalTokens: 1970,
			StoppedBy:   workflow.StopReasonCondition,
		},
		TokensUsed: 17000,
		ContextWindowState: &workflow.ContextWindowState{
			Strategy:        workflow.StrategySlidingWindow,
			TruncationCount: 1,
			TurnsDropped:    2,
			TokensDropped:   300,
			LastTruncatedAt: 3,
		},
	}

	assert.Equal(t, "refine-code", state.Name)
	assert.Equal(t, workflow.StatusCompleted, state.Status)
	assert.NotNil(t, state.Conversation)
	assert.Equal(t, 5, state.Conversation.TotalTurns)
	assert.Equal(t, workflow.StopReasonCondition, state.Conversation.StoppedBy)
	assert.Equal(t, 17000, state.TokensUsed)
	assert.NotNil(t, state.ContextWindowState)
	assert.Equal(t, 1, state.ContextWindowState.TruncationCount)
}

func TestStepState_ConversationFields_JSONSerialization(t *testing.T) {
	// Edge case: Verify conversation fields serialize/deserialize correctly
	original := workflow.StepState{
		Name:   "agent-step",
		Status: workflow.StatusCompleted,
		Output: "Final response",
		Conversation: &workflow.ConversationState{
			Turns: []workflow.Turn{
				{Role: workflow.TurnRoleUser, Content: "Hello", Tokens: 5},
				{Role: workflow.TurnRoleAssistant, Content: "Hi there", Tokens: 10},
			},
			TotalTurns:  2,
			TotalTokens: 15,
			StoppedBy:   workflow.StopReasonMaxTurns,
		},
		TokensUsed: 15,
		ContextWindowState: &workflow.ContextWindowState{
			Strategy:        workflow.StrategySlidingWindow,
			TruncationCount: 0,
			TurnsDropped:    0,
			TokensDropped:   0,
			LastTruncatedAt: 0,
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded workflow.StepState
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Name, decoded.Name)
	assert.NotNil(t, decoded.Conversation)
	assert.Equal(t, 2, decoded.Conversation.TotalTurns)
	assert.Equal(t, 15, decoded.TokensUsed)
	assert.NotNil(t, decoded.ContextWindowState)
	assert.Equal(t, workflow.StrategySlidingWindow, decoded.ContextWindowState.Strategy)
}

func TestStepState_ConversationFields_ExecutionContextIntegration(t *testing.T) {
	// Happy path: StepState with conversation fields stored in ExecutionContext
	ctx := workflow.NewExecutionContext("test-id", "test-workflow")

	state := workflow.StepState{
		Name:   "agent-conversation",
		Status: workflow.StatusCompleted,
		Output: "Done",
		Conversation: &workflow.ConversationState{
			Turns: []workflow.Turn{
				{Role: workflow.TurnRoleUser, Content: "Task", Tokens: 10},
				{Role: workflow.TurnRoleAssistant, Content: "Done", Tokens: 20},
			},
			TotalTurns:  2,
			TotalTokens: 30,
		},
		TokensUsed: 30,
	}

	ctx.SetStepState("agent-conversation", state)

	retrieved, ok := ctx.GetStepState("agent-conversation")
	require.True(t, ok)
	assert.NotNil(t, retrieved.Conversation)
	assert.Equal(t, 30, retrieved.TokensUsed)
	assert.Equal(t, 2, retrieved.Conversation.TotalTurns)
}

func TestStepState_ConversationFields_NilConversationSerialization(t *testing.T) {
	// Edge case: StepState with nil Conversation should serialize correctly
	state := workflow.StepState{
		Name:         "shell-step",
		Status:       workflow.StatusCompleted,
		Output:       "success",
		Conversation: nil,
		TokensUsed:   0,
	}

	data, err := json.Marshal(state)
	require.NoError(t, err)

	var decoded workflow.StepState
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Nil(t, decoded.Conversation)
	assert.Equal(t, 0, decoded.TokensUsed)
	assert.Nil(t, decoded.ContextWindowState)
}

func TestStepState_ConversationFields_LargeTokenCounts(t *testing.T) {
	// Edge case: Handle large token counts
	state := workflow.StepState{
		Name:       "long-conversation",
		Status:     workflow.StatusCompleted,
		TokensUsed: 150000, // Large conversation
		Conversation: &workflow.ConversationState{
			Turns:       make([]workflow.Turn, 100), // Many turns
			TotalTurns:  100,
			TotalTokens: 150000,
			StoppedBy:   workflow.StopReasonMaxTokens,
		},
	}

	assert.Equal(t, 150000, state.TokensUsed)
	assert.Equal(t, 100, state.Conversation.TotalTurns)
}

func TestStepState_ConversationFields_EmptyConversation(t *testing.T) {
	// Edge case: Empty conversation (zero turns)
	state := workflow.StepState{
		Name:   "empty-conversation",
		Status: workflow.StatusFailed,
		Error:  "No turns executed",
		Conversation: &workflow.ConversationState{
			Turns:       []workflow.Turn{},
			TotalTurns:  0,
			TotalTokens: 0,
		},
		TokensUsed: 0,
	}

	assert.NotNil(t, state.Conversation)
	assert.Empty(t, state.Conversation.Turns)
	assert.Equal(t, 0, state.TokensUsed)
}

func TestStepState_ConversationFields_MultipleStrategies(t *testing.T) {
	// Edge case: Test different context window strategies
	strategies := []workflow.ContextWindowStrategy{
		workflow.StrategyNone,
		workflow.StrategySlidingWindow,
		workflow.StrategySummarize,
		workflow.StrategyTruncateMiddle,
	}

	for _, strategy := range strategies {
		t.Run(string(strategy), func(t *testing.T) {
			state := workflow.StepState{
				Name: "strategy-test",
				ContextWindowState: &workflow.ContextWindowState{
					Strategy:        strategy,
					TruncationCount: 1,
					TurnsDropped:    3,
					TokensDropped:   500,
				},
			}

			assert.NotNil(t, state.ContextWindowState)
			assert.Equal(t, strategy, state.ContextWindowState.Strategy)
		})
	}
}

func TestStepState_ConversationFields_StopReasons(t *testing.T) {
	// Edge case: Test all stop reasons
	stopReasons := []workflow.StopReason{
		workflow.StopReasonCondition,
		workflow.StopReasonMaxTurns,
		workflow.StopReasonMaxTokens,
		workflow.StopReasonError,
	}

	for _, reason := range stopReasons {
		t.Run(string(reason), func(t *testing.T) {
			state := workflow.StepState{
				Name: "stop-reason-test",
				Conversation: &workflow.ConversationState{
					Turns:      []workflow.Turn{{Role: workflow.TurnRoleUser, Content: "test", Tokens: 1}},
					TotalTurns: 1,
					StoppedBy:  reason,
				},
			}

			assert.Equal(t, reason, state.Conversation.StoppedBy)
		})
	}
}

func TestStepState_ConversationFields_MixedStepsInContext(t *testing.T) {
	// Edge case: ExecutionContext with mixed step types (conversation and non-conversation)
	ctx := workflow.NewExecutionContext("test-id", "mixed-workflow")

	// Regular shell step (no conversation)
	shellStep := workflow.StepState{
		Name:         "shell-command",
		Status:       workflow.StatusCompleted,
		Output:       "success",
		ExitCode:     0,
		Conversation: nil,
		TokensUsed:   0,
	}

	// Agent conversation step
	agentStep := workflow.StepState{
		Name:   "agent-conversation",
		Status: workflow.StatusCompleted,
		Output: "AI response",
		Conversation: &workflow.ConversationState{
			Turns:       []workflow.Turn{{Role: workflow.TurnRoleUser, Content: "test", Tokens: 10}},
			TotalTurns:  1,
			TotalTokens: 10,
		},
		TokensUsed: 10,
	}

	ctx.SetStepState("shell-command", shellStep)
	ctx.SetStepState("agent-conversation", agentStep)

	// Verify both types stored correctly
	shell, ok := ctx.GetStepState("shell-command")
	require.True(t, ok)
	assert.Nil(t, shell.Conversation)
	assert.Equal(t, 0, shell.TokensUsed)

	agent, ok := ctx.GetStepState("agent-conversation")
	require.True(t, ok)
	assert.NotNil(t, agent.Conversation)
	assert.Equal(t, 10, agent.TokensUsed)
}

func TestStepState_ConversationFields_ContextWindowNoTruncation(t *testing.T) {
	// Edge case: ContextWindowState with no truncation applied
	state := workflow.StepState{
		Name: "no-truncation",
		ContextWindowState: &workflow.ContextWindowState{
			Strategy:        workflow.StrategySlidingWindow,
			TruncationCount: 0,
			TurnsDropped:    0,
			TokensDropped:   0,
			LastTruncatedAt: 0,
		},
	}

	assert.NotNil(t, state.ContextWindowState)
	assert.Equal(t, 0, state.ContextWindowState.TruncationCount)
}

func TestStepState_ConversationFields_MaxTruncation(t *testing.T) {
	// Edge case: Heavy truncation scenario
	state := workflow.StepState{
		Name: "heavy-truncation",
		ContextWindowState: &workflow.ContextWindowState{
			Strategy:        workflow.StrategySlidingWindow,
			TruncationCount: 50,
			TurnsDropped:    200,
			TokensDropped:   50000,
			LastTruncatedAt: 250,
		},
	}

	assert.Equal(t, 50, state.ContextWindowState.TruncationCount)
	assert.Equal(t, 200, state.ContextWindowState.TurnsDropped)
	assert.Equal(t, 50000, state.ContextWindowState.TokensDropped)
}

func TestStepState_ConversationFields_FailedConversation(t *testing.T) {
	// Error handling: Failed conversation step
	state := workflow.StepState{
		Name:   "failed-conversation",
		Status: workflow.StatusFailed,
		Error:  "API timeout",
		Conversation: &workflow.ConversationState{
			Turns: []workflow.Turn{
				{Role: workflow.TurnRoleUser, Content: "Request", Tokens: 10},
			},
			TotalTurns:  1,
			TotalTokens: 10,
			StoppedBy:   workflow.StopReasonError,
		},
		TokensUsed: 10,
	}

	assert.Equal(t, workflow.StatusFailed, state.Status)
	assert.Equal(t, "API timeout", state.Error)
	assert.NotNil(t, state.Conversation)
	assert.Equal(t, workflow.StopReasonError, state.Conversation.StoppedBy)
}

func TestStepState_ConversationFields_ZeroTokensUsed(t *testing.T) {
	// Edge case: Conversation with zero tokens (error scenario)
	state := workflow.StepState{
		Name:       "zero-tokens",
		Status:     workflow.StatusFailed,
		TokensUsed: 0,
		Conversation: &workflow.ConversationState{
			Turns:       []workflow.Turn{},
			TotalTurns:  0,
			TotalTokens: 0,
		},
	}

	assert.Equal(t, 0, state.TokensUsed)
	assert.NotNil(t, state.Conversation)
	assert.Equal(t, 0, state.Conversation.TotalTokens)
}

// C069: Tests for StepState.Data field (structured output from custom step types)

func TestStepState_Data_SetAndGet(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		wantKeys []string
	}{
		{
			name: "empty data map",
			data: map[string]any{},
		},
		{
			name:     "single string value",
			data:     map[string]any{"result": "success"},
			wantKeys: []string{"result"},
		},
		{
			name: "multiple types",
			data: map[string]any{
				"status":  "completed",
				"count":   42,
				"ratio":   3.14,
				"enabled": true,
			},
			wantKeys: []string{"status", "count", "ratio", "enabled"},
		},
		{
			name: "nested structures",
			data: map[string]any{
				"config": map[string]any{
					"timeout": 30,
					"retries": 3,
				},
				"items": []any{1, 2, 3},
			},
			wantKeys: []string{"config", "items"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := workflow.StepState{
				Name: "test-step",
				Data: tt.data,
			}

			assert.Equal(t, tt.data, state.Data)
			for _, key := range tt.wantKeys {
				assert.Contains(t, state.Data, key)
			}
		})
	}
}

func TestStepState_Data_Modification(t *testing.T) {
	state := workflow.StepState{
		Name: "test-step",
		Data: make(map[string]any),
	}

	state.Data["output"] = "value1"
	assert.Equal(t, "value1", state.Data["output"])

	state.Data["output"] = "updated"
	assert.Equal(t, "updated", state.Data["output"])

	delete(state.Data, "output")
	_, exists := state.Data["output"]
	assert.False(t, exists)
}

func TestStepState_Data_ComplexValues(t *testing.T) {
	state := workflow.StepState{
		Name: "test-step",
		Data: map[string]any{
			"timestamp": time.Now(),
			"metadata": map[string]any{
				"version": "1.0",
				"cached":  true,
			},
			"results": []any{"pass", "fail", "skip"},
		},
	}

	assert.NotNil(t, state.Data["timestamp"])
	metadata := state.Data["metadata"].(map[string]any)
	assert.Equal(t, "1.0", metadata["version"])

	results := state.Data["results"].([]any)
	assert.Len(t, results, 3)
}

func TestStepState_Data_NilMap(t *testing.T) {
	state := workflow.StepState{
		Name: "test-step",
		Data: nil,
	}

	assert.Nil(t, state.Data)
}

func TestExecutionContext_SetStepState_WithData(t *testing.T) {
	ctx := workflow.NewExecutionContext("wf-123", "test-workflow")

	state := workflow.StepState{
		Name:   "step-1",
		Status: workflow.StatusCompleted,
		Output: "success",
		Data: map[string]any{
			"result": "done",
			"count":  42,
		},
	}

	ctx.SetStepState("step-1", state)

	retrieved, exists := ctx.GetStepState("step-1")
	require.True(t, exists)
	assert.Equal(t, "step-1", retrieved.Name)
	assert.Equal(t, "success", retrieved.Output)
	assert.Equal(t, map[string]any{"result": "done", "count": 42}, retrieved.Data)
}

func TestExecutionContext_GetStepState_DataAccess(t *testing.T) {
	ctx := workflow.NewExecutionContext("wf-456", "workflow")

	state := workflow.StepState{
		Name: "step-2",
		Data: map[string]any{
			"message": "test message",
			"active":  true,
		},
	}

	ctx.SetStepState("step-2", state)

	retrieved, _ := ctx.GetStepState("step-2")
	assert.Equal(t, "test message", retrieved.Data["message"])
	assert.Equal(t, true, retrieved.Data["active"])
}

func TestExecutionContext_ConcurrentDataAccess(t *testing.T) {
	ctx := workflow.NewExecutionContext("wf-789", "concurrent-test")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			state := workflow.StepState{
				Name: "step-concurrent",
				Data: map[string]any{
					"index": index,
					"value": index * 10,
				},
			}

			ctx.SetStepState("step-concurrent", state)

			retrieved, exists := ctx.GetStepState("step-concurrent")
			assert.True(t, exists)
			assert.NotNil(t, retrieved.Data)
		}(i)
	}

	wg.Wait()
}

func TestExecutionContext_GetAllStepStates_WithData(t *testing.T) {
	ctx := workflow.NewExecutionContext("wf-abc", "multi-step")

	states := map[string]workflow.StepState{
		"step-1": {
			Name: "step-1",
			Data: map[string]any{"result": "pass"},
		},
		"step-2": {
			Name: "step-2",
			Data: map[string]any{"result": "fail"},
		},
		"step-3": {
			Name: "step-3",
			Data: map[string]any{"result": "skip"},
		},
	}

	for name, state := range states {
		ctx.SetStepState(name, state)
	}

	allStates := ctx.GetAllStepStates()
	assert.Len(t, allStates, 3)

	for name, expectedState := range states {
		retrieved, exists := allStates[name]
		assert.True(t, exists)
		assert.Equal(t, expectedState.Data, retrieved.Data)
	}
}

func TestStepState_Data_TypeAssertion(t *testing.T) {
	state := workflow.StepState{
		Name: "test-step",
		Data: map[string]any{
			"string_val": "test",
			"int_val":    42,
			"float_val":  3.14,
			"bool_val":   true,
		},
	}

	strVal, ok := state.Data["string_val"].(string)
	assert.True(t, ok)
	assert.Equal(t, "test", strVal)

	intVal, ok := state.Data["int_val"].(int)
	assert.True(t, ok)
	assert.Equal(t, 42, intVal)

	floatVal, ok := state.Data["float_val"].(float64)
	assert.True(t, ok)
	assert.Equal(t, 3.14, floatVal)

	boolVal, ok := state.Data["bool_val"].(bool)
	assert.True(t, ok)
	assert.True(t, boolVal)
}

func TestExecutionContext_MultipleSteps_DataIsolation(t *testing.T) {
	ctx := workflow.NewExecutionContext("wf-isolation", "isolation-test")

	step1 := workflow.StepState{
		Name: "step-1",
		Data: map[string]any{"unique": "data-1"},
	}
	step2 := workflow.StepState{
		Name: "step-2",
		Data: map[string]any{"unique": "data-2"},
	}

	ctx.SetStepState("step-1", step1)
	ctx.SetStepState("step-2", step2)

	retrieved1, _ := ctx.GetStepState("step-1")
	retrieved2, _ := ctx.GetStepState("step-2")

	assert.Equal(t, "data-1", retrieved1.Data["unique"])
	assert.Equal(t, "data-2", retrieved2.Data["unique"])
}
