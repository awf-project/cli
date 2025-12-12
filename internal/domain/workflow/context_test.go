package workflow_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
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

// =============================================================================
// F043: LoopContext JSON Serialization Tests
// =============================================================================

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
