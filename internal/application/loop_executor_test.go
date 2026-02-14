package application_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/testutil/mocks"
)

//
// These tests verify the LoopExecutor implementation uses the
// ports.ExpressionEvaluator interface correctly (C042 DIP compliance).
//
// Coverage areas:
// - Constructor injection: ExpressionEvaluator passed via NewLoopExecutor
// - ParseItems: JSON array and comma-separated value parsing
// - BuildBodyStepIndices: Step name to index mapping with duplicate detection
// - PushLoopContext/PopLoopContext: Nested loop context management
//
// Related:
// - C042: Fix DIP Violations in Application Layer
// - Component T007: ResolveMaxIterations tests (in loop_executor_refactor_test.go)
// - Component T008: General LoopExecutor tests

//
// Note: mockLogger and mockResolver are defined in service_test.go
// and shared across all test files in the application_test package.

func TestLoopExecutor_NewLoopExecutor_RequiresExpressionEvaluator(t *testing.T) {
	logger := &mockLogger{}
	evaluator := mocks.NewMockExpressionEvaluator()
	resolver := newMockResolver()

	executor := application.NewLoopExecutor(logger, evaluator, resolver)

	require.NotNil(t, executor, "NewLoopExecutor should return non-nil instance")
}

func TestLoopExecutor_NewLoopExecutor_AcceptsPortsInterface(t *testing.T) {
	logger := &mockLogger{}

	// This test verifies compile-time compatibility:
	// If ExpressionEvaluator parameter type changed from ports.ExpressionEvaluator,
	// this would fail to compile
	evaluator := *mocks.NewMockExpressionEvaluator()
	resolver := newMockResolver()

	executor := application.NewLoopExecutor(logger, &evaluator, resolver)

	require.NotNil(t, executor)
}

func TestLoopExecutor_ParseItems_JSONArray(t *testing.T) {
	logger := &mockLogger{}
	evaluator := mocks.NewMockExpressionEvaluator()
	resolver := newMockResolver()
	executor := application.NewLoopExecutor(logger, evaluator, resolver)

	tests := []struct {
		name     string
		input    string
		expected []any
		wantErr  bool
	}{
		{
			name:     "EmptyArray",
			input:    "[]",
			expected: []any{},
			wantErr:  false,
		},
		{
			name:     "StringArray",
			input:    `["a", "b", "c"]`,
			expected: []any{"a", "b", "c"},
			wantErr:  false,
		},
		{
			name:     "NumberArray",
			input:    `[1, 2, 3]`,
			expected: []any{float64(1), float64(2), float64(3)}, // JSON numbers are float64
			wantErr:  false,
		},
		{
			name:     "MixedArray",
			input:    `["apple", 42, true]`,
			expected: []any{"apple", float64(42), true},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.ParseItems(tt.input)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestLoopExecutor_ParseItems_CommaSeparated(t *testing.T) {
	logger := &mockLogger{}
	evaluator := mocks.NewMockExpressionEvaluator()
	resolver := newMockResolver()
	executor := application.NewLoopExecutor(logger, evaluator, resolver)

	tests := []struct {
		name     string
		input    string
		expected []any
	}{
		{
			name:     "SingleItem",
			input:    "apple",
			expected: []any{"apple"},
		},
		{
			name:     "MultipleItems",
			input:    "apple,banana,cherry",
			expected: []any{"apple", "banana", "cherry"},
		},
		{
			name:     "ItemsWithWhitespace",
			input:    " apple , banana , cherry ",
			expected: []any{"apple", "banana", "cherry"},
		},
		{
			name:     "EmptyString",
			input:    "",
			expected: []any{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.ParseItems(tt.input)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoopExecutor_ParseItems_PrefersJSONOverCommaSeparated(t *testing.T) {
	logger := &mockLogger{}
	evaluator := mocks.NewMockExpressionEvaluator()
	resolver := newMockResolver()
	executor := application.NewLoopExecutor(logger, evaluator, resolver)

	result, err := executor.ParseItems(`["a,b", "c,d"]`)

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "a,b", result[0])
	assert.Equal(t, "c,d", result[1])
}

func TestLoopExecutor_BuildBodyStepIndices_HappyPath(t *testing.T) {
	logger := &mockLogger{}
	evaluator := mocks.NewMockExpressionEvaluator()
	resolver := newMockResolver()
	executor := application.NewLoopExecutor(logger, evaluator, resolver)

	body := []string{"step_a", "step_b", "step_c"}

	indices, err := executor.BuildBodyStepIndices(body)

	require.NoError(t, err)
	assert.Len(t, indices, 3)
	assert.Equal(t, 0, indices["step_a"])
	assert.Equal(t, 1, indices["step_b"])
	assert.Equal(t, 2, indices["step_c"])
}

func TestLoopExecutor_BuildBodyStepIndices_EmptyBody(t *testing.T) {
	logger := &mockLogger{}
	evaluator := mocks.NewMockExpressionEvaluator()
	resolver := newMockResolver()
	executor := application.NewLoopExecutor(logger, evaluator, resolver)

	body := []string{}

	indices, err := executor.BuildBodyStepIndices(body)

	require.NoError(t, err)
	assert.Empty(t, indices)
}

func TestLoopExecutor_BuildBodyStepIndices_DuplicateStepName(t *testing.T) {
	logger := &mockLogger{}
	evaluator := mocks.NewMockExpressionEvaluator()
	resolver := newMockResolver()
	executor := application.NewLoopExecutor(logger, evaluator, resolver)

	// Body contains duplicate step name
	body := []string{"step_a", "step_b", "step_a"}

	indices, err := executor.BuildBodyStepIndices(body)

	require.Error(t, err)
	assert.Nil(t, indices)
	assert.Contains(t, err.Error(), "duplicate step")
	assert.Contains(t, err.Error(), "step_a")
}

func TestLoopExecutor_BuildBodyStepIndices_MultipleDuplicates(t *testing.T) {
	logger := &mockLogger{}
	evaluator := mocks.NewMockExpressionEvaluator()
	resolver := newMockResolver()
	executor := application.NewLoopExecutor(logger, evaluator, resolver)

	// Body contains first duplicate encountered
	body := []string{"step_a", "step_b", "step_a", "step_c", "step_b"}

	indices, err := executor.BuildBodyStepIndices(body)

	require.Error(t, err)
	assert.Nil(t, indices)
	assert.Contains(t, err.Error(), "duplicate step")
}

func TestLoopExecutor_PushLoopContext_CreatesNewContext(t *testing.T) {
	logger := &mockLogger{}
	evaluator := mocks.NewMockExpressionEvaluator()
	resolver := newMockResolver()
	executor := application.NewLoopExecutor(logger, evaluator, resolver)

	execCtx := &workflow.ExecutionContext{
		CurrentLoop: nil,
	}

	executor.PushLoopContext(execCtx, "item1", 0, true, false, 3)

	require.NotNil(t, execCtx.CurrentLoop)
	assert.Equal(t, "item1", execCtx.CurrentLoop.Item)
	assert.Equal(t, 0, execCtx.CurrentLoop.Index)
	assert.True(t, execCtx.CurrentLoop.First)
	assert.False(t, execCtx.CurrentLoop.Last)
	assert.Equal(t, 3, execCtx.CurrentLoop.Length)
	assert.Nil(t, execCtx.CurrentLoop.Parent, "First loop should have no parent")
}

func TestLoopExecutor_PushLoopContext_NestedLoops(t *testing.T) {
	logger := &mockLogger{}
	evaluator := mocks.NewMockExpressionEvaluator()
	resolver := newMockResolver()
	executor := application.NewLoopExecutor(logger, evaluator, resolver)

	execCtx := &workflow.ExecutionContext{}

	executor.PushLoopContext(execCtx, "outer_item", 0, true, true, 1)
	outerContext := execCtx.CurrentLoop

	executor.PushLoopContext(execCtx, "inner_item", 0, true, false, 2)
	innerContext := execCtx.CurrentLoop

	require.NotNil(t, innerContext)
	assert.Equal(t, "inner_item", innerContext.Item)
	assert.Equal(t, outerContext, innerContext.Parent, "Inner loop should link to outer loop")

	assert.Equal(t, "outer_item", innerContext.Parent.Item)
}

func TestLoopExecutor_PopLoopContext_RestoresParent(t *testing.T) {
	logger := &mockLogger{}
	evaluator := mocks.NewMockExpressionEvaluator()
	resolver := newMockResolver()
	executor := application.NewLoopExecutor(logger, evaluator, resolver)

	execCtx := &workflow.ExecutionContext{}

	// Push outer, then inner
	executor.PushLoopContext(execCtx, "outer", 0, true, true, 1)
	outerContext := execCtx.CurrentLoop
	executor.PushLoopContext(execCtx, "inner", 0, true, true, 1)

	popped := executor.PopLoopContext(execCtx)

	require.NotNil(t, popped)
	assert.Equal(t, "inner", popped.Item)

	assert.Equal(t, outerContext, execCtx.CurrentLoop)
	assert.Equal(t, "outer", execCtx.CurrentLoop.Item)
}

func TestLoopExecutor_PopLoopContext_WithNilContext(t *testing.T) {
	logger := &mockLogger{}
	evaluator := mocks.NewMockExpressionEvaluator()
	resolver := newMockResolver()
	executor := application.NewLoopExecutor(logger, evaluator, resolver)

	execCtx := &workflow.ExecutionContext{
		CurrentLoop: nil,
	}

	popped := executor.PopLoopContext(execCtx)

	assert.Nil(t, popped)
	assert.Nil(t, execCtx.CurrentLoop)
}

func TestLoopExecutor_PopLoopContext_ReturnsToNil(t *testing.T) {
	logger := &mockLogger{}
	evaluator := mocks.NewMockExpressionEvaluator()
	resolver := newMockResolver()
	executor := application.NewLoopExecutor(logger, evaluator, resolver)

	execCtx := &workflow.ExecutionContext{}

	// Push single loop
	executor.PushLoopContext(execCtx, "item", 0, true, true, 1)

	popped := executor.PopLoopContext(execCtx)

	require.NotNil(t, popped)
	assert.Nil(t, execCtx.CurrentLoop)
}

func TestLoopExecutor_ParseItems_EdgeCases(t *testing.T) {
	logger := &mockLogger{}
	evaluator := mocks.NewMockExpressionEvaluator()
	resolver := newMockResolver()
	executor := application.NewLoopExecutor(logger, evaluator, resolver)

	tests := []struct {
		name     string
		input    string
		expected []any
	}{
		{
			name:     "SingleComma",
			input:    ",",
			expected: []any{"", ""},
		},
		{
			name:     "MultipleCommas",
			input:    ",,",
			expected: []any{"", "", ""},
		},
		{
			name:     "WhitespaceOnly",
			input:    "   ",
			expected: []any{""},
		},
		{
			name:     "SpecialCharacters",
			input:    "@#$,%^&,*()_",
			expected: []any{"@#$", "%^&", "*()_"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.ParseItems(tt.input)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoopExecutor_BuildBodyStepIndices_LargeBody(t *testing.T) {
	logger := &mockLogger{}
	evaluator := mocks.NewMockExpressionEvaluator()
	resolver := newMockResolver()
	executor := application.NewLoopExecutor(logger, evaluator, resolver)

	// Create 100 unique step names
	body := make([]string, 100)
	for i := 0; i < 100; i++ {
		body[i] = "step_" + string(rune('a'+i%26)) + string(rune('0'+i/26))
	}

	indices, err := executor.BuildBodyStepIndices(body)

	require.NoError(t, err)
	assert.Len(t, indices, 100)

	// Verify first and last indices
	assert.Equal(t, 0, indices["step_a0"])
	assert.Equal(t, 99, indices[body[99]])
}

func TestLoopExecutor_PushLoopContext_MultipleNestingLevels(t *testing.T) {
	logger := &mockLogger{}
	evaluator := mocks.NewMockExpressionEvaluator()
	resolver := newMockResolver()
	executor := application.NewLoopExecutor(logger, evaluator, resolver)

	execCtx := &workflow.ExecutionContext{}

	executor.PushLoopContext(execCtx, "level1", 0, true, true, 1)
	level1 := execCtx.CurrentLoop

	executor.PushLoopContext(execCtx, "level2", 0, true, true, 1)
	level2 := execCtx.CurrentLoop

	executor.PushLoopContext(execCtx, "level3", 0, true, true, 1)
	level3 := execCtx.CurrentLoop

	require.NotNil(t, level3)
	assert.Equal(t, "level3", level3.Item)
	assert.Equal(t, level2, level3.Parent)
	assert.Equal(t, level1, level3.Parent.Parent)
	assert.Nil(t, level3.Parent.Parent.Parent)
}
