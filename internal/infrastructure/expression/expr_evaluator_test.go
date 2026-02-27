package expression

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: T003
// Feature: C042

func TestExprEvaluator_InterfaceCompliance(t *testing.T) {
	// Verify ExprEvaluator implements ports.ExpressionEvaluator
	var _ ports.ExpressionEvaluator = (*ExprEvaluator)(nil)
}

func TestNewExprEvaluator(t *testing.T) {
	evaluator := NewExprEvaluator()

	require.NotNil(t, evaluator)
	assert.Implements(t, (*ports.ExpressionEvaluator)(nil), evaluator)
}

func TestExprEvaluator_EvaluateBool_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		context    *interpolation.Context
		want       bool
	}{
		{
			name:       "simple true literal",
			expression: "true",
			context:    &interpolation.Context{},
			want:       true,
		},
		{
			name:       "simple false literal",
			expression: "false",
			context:    &interpolation.Context{},
			want:       false,
		},
		{
			name:       "integer comparison - greater than true",
			expression: "inputs.count > 5",
			context: &interpolation.Context{
				Inputs: map[string]any{"count": 10},
			},
			want: true,
		},
		{
			name:       "integer comparison - greater than false",
			expression: "inputs.count > 5",
			context: &interpolation.Context{
				Inputs: map[string]any{"count": 3},
			},
			want: false,
		},
		{
			name:       "string equality - true",
			expression: "inputs.status == 'done'",
			context: &interpolation.Context{
				Inputs: map[string]any{"status": "done"},
			},
			want: true,
		},
		{
			name:       "string equality - false",
			expression: "inputs.status == 'done'",
			context: &interpolation.Context{
				Inputs: map[string]any{"status": "pending"},
			},
			want: false,
		},
		{
			name:       "string contains - true",
			expression: "inputs.message contains 'COMPLETE'",
			context: &interpolation.Context{
				Inputs: map[string]any{"message": "Task COMPLETE successfully"},
			},
			want: true,
		},
		{
			name:       "string contains - false",
			expression: "inputs.message contains 'COMPLETE'",
			context: &interpolation.Context{
				Inputs: map[string]any{"message": "Task pending"},
			},
			want: false,
		},
		{
			name:       "logical AND - true",
			expression: "inputs.count > 5 && inputs.status == 'done'",
			context: &interpolation.Context{
				Inputs: map[string]any{
					"count":  10,
					"status": "done",
				},
			},
			want: true,
		},
		{
			name:       "logical AND - false (first false)",
			expression: "inputs.count > 5 && inputs.status == 'done'",
			context: &interpolation.Context{
				Inputs: map[string]any{
					"count":  3,
					"status": "done",
				},
			},
			want: false,
		},
		{
			name:       "logical AND - false (second false)",
			expression: "inputs.count > 5 && inputs.status == 'done'",
			context: &interpolation.Context{
				Inputs: map[string]any{
					"count":  10,
					"status": "pending",
				},
			},
			want: false,
		},
		{
			name:       "logical OR - true (first true)",
			expression: "inputs.count > 5 || inputs.status == 'done'",
			context: &interpolation.Context{
				Inputs: map[string]any{
					"count":  10,
					"status": "pending",
				},
			},
			want: true,
		},
		{
			name:       "logical OR - true (second true)",
			expression: "inputs.count > 5 || inputs.status == 'done'",
			context: &interpolation.Context{
				Inputs: map[string]any{
					"count":  3,
					"status": "done",
				},
			},
			want: true,
		},
		{
			name:       "logical OR - false",
			expression: "inputs.count > 5 || inputs.status == 'done'",
			context: &interpolation.Context{
				Inputs: map[string]any{
					"count":  3,
					"status": "pending",
				},
			},
			want: false,
		},
		{
			name:       "nested parentheses",
			expression: "(inputs.count > 5 && inputs.ready) || inputs.force",
			context: &interpolation.Context{
				Inputs: map[string]any{
					"count": 10,
					"ready": true,
					"force": false,
				},
			},
			want: true,
		},
		{
			name:       "states access - exit code",
			expression: "states.build.ExitCode == 0",
			context: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"build": {ExitCode: 0},
				},
			},
			want: true,
		},
		{
			name:       "states access - output contains",
			expression: "states.test.Output contains 'PASS'",
			context: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"test": {Output: "All tests PASS"},
				},
			},
			want: true,
		},
		{
			name:       "env variable access",
			expression: "env.DEBUG == 'true'",
			context: &interpolation.Context{
				Env: map[string]string{"DEBUG": "true"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewExprEvaluator()

			result, err := evaluator.EvaluateBool(tt.expression, tt.context)

			require.NoError(t, err, "expression should evaluate without error")
			assert.Equal(t, tt.want, result, "expression result mismatch")
		})
	}
}

func TestExprEvaluator_EvaluateBool_EmptyExpression(t *testing.T) {
	evaluator := NewExprEvaluator()
	ctx := &interpolation.Context{}

	result, err := evaluator.EvaluateBool("", ctx)

	assert.Error(t, err, "empty expression should return error")
	assert.False(t, result, "empty expression should return false")
}

func TestExprEvaluator_EvaluateBool_NilContext(t *testing.T) {
	evaluator := NewExprEvaluator()

	result, err := evaluator.EvaluateBool("true", nil)

	// pkg/expression handles nil context by creating empty maps
	require.NoError(t, err)
	assert.True(t, result)
}

func TestExprEvaluator_EvaluateBool_MissingInputs(t *testing.T) {
	evaluator := NewExprEvaluator()
	ctx := &interpolation.Context{}

	result, err := evaluator.EvaluateBool("inputs.count > 5", ctx)

	// Missing inputs should evaluate to false without error (per pkg/expression behavior)
	require.NoError(t, err)
	assert.False(t, result)
}

func TestExprEvaluator_EvaluateBool_MissingStateKey(t *testing.T) {
	evaluator := NewExprEvaluator()
	ctx := &interpolation.Context{
		States: map[string]interpolation.StepStateData{
			"build": {ExitCode: 0},
		},
	}

	result, err := evaluator.EvaluateBool("states.nonexistent.ExitCode == 0", ctx)

	// Missing state keys should evaluate to false without error
	require.NoError(t, err)
	assert.False(t, result)
}

func TestExprEvaluator_EvaluateBool_TypeCoercion_StringBoolean(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"lowercase true", "true", true},
		{"uppercase true", "TRUE", true},
		{"titlecase true", "True", true},
		{"lowercase false", "false", false},
		{"uppercase false", "FALSE", false},
		{"titlecase false", "False", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewExprEvaluator()
			ctx := &interpolation.Context{
				Inputs: map[string]any{"flag": tt.value},
			}

			result, err := evaluator.EvaluateBool("inputs.flag", ctx)

			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestExprEvaluator_EvaluateBool_TypeCoercion_StringNumbers(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		expression string
		want       bool
	}{
		{
			name:       "integer string comparison",
			value:      "10",
			expression: "inputs.count > 5",
			want:       true,
		},
		{
			name:       "float string comparison",
			value:      "3.14",
			expression: "inputs.ratio > 3.0",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewExprEvaluator()
			ctx := &interpolation.Context{
				Inputs: map[string]any{"count": tt.value, "ratio": tt.value},
			}

			result, err := evaluator.EvaluateBool(tt.expression, ctx)

			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestExprEvaluator_EvaluateBool_InvalidSyntax(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{"unclosed quote", "inputs.status == 'done"},
		{"unclosed parenthesis", "(inputs.count > 5"},
		{"invalid operator", "inputs.count >< 5"},
		{"missing operand", "inputs.count >"},
		{"double operators", "inputs.count >> 5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewExprEvaluator()
			ctx := &interpolation.Context{}

			result, err := evaluator.EvaluateBool(tt.expression, ctx)

			require.Error(t, err, "invalid syntax should return error")
			assert.False(t, result, "error should return false")
		})
	}
}

func TestExprEvaluator_EvaluateBool_NonBooleanResult(t *testing.T) {
	evaluator := NewExprEvaluator()
	ctx := &interpolation.Context{
		Inputs: map[string]any{"count": 10},
	}

	result, err := evaluator.EvaluateBool("inputs.count + 5", ctx)

	require.Error(t, err, "non-boolean result should return error")
	assert.Contains(t, err.Error(), "boolean", "error should mention type mismatch")
	assert.False(t, result)
}

func TestExprEvaluator_EvaluateBool_LoopConditions(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		context    *interpolation.Context
		want       bool
	}{
		{
			name:       "loop continue condition - count limit",
			expression: "loop.Index < 10",
			context: &interpolation.Context{
				Loop: &interpolation.LoopData{
					Index: 5,
				},
			},
			want: true,
		},
		{
			name:       "loop break condition - status check",
			expression: "states.process.Status == 'complete'",
			context: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"process": {Status: "complete"},
				},
			},
			want: true,
		},
		{
			name:       "loop item condition",
			expression: "loop.Item contains 'test'",
			context: &interpolation.Context{
				Loop: &interpolation.LoopData{
					Item: "test-file.txt",
				},
			},
			want: true,
		},
		{
			name:       "first iteration check",
			expression: "loop.First",
			context: &interpolation.Context{
				Loop: &interpolation.LoopData{
					First: true,
				},
			},
			want: true,
		},
		{
			name:       "last iteration check",
			expression: "loop.Last",
			context: &interpolation.Context{
				Loop: &interpolation.LoopData{
					Last: true,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewExprEvaluator()

			result, err := evaluator.EvaluateBool(tt.expression, tt.context)

			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestExprEvaluator_EvaluateInt_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		context    *interpolation.Context
		want       int
	}{
		{
			name:       "integer literal",
			expression: "42",
			context:    &interpolation.Context{},
			want:       42,
		},
		{
			name:       "simple addition",
			expression: "10 + 5",
			context:    &interpolation.Context{},
			want:       15,
		},
		{
			name:       "simple subtraction",
			expression: "10 - 3",
			context:    &interpolation.Context{},
			want:       7,
		},
		{
			name:       "simple multiplication",
			expression: "6 * 7",
			context:    &interpolation.Context{},
			want:       42,
		},
		{
			name:       "simple division",
			expression: "20 / 4",
			context:    &interpolation.Context{},
			want:       5,
		},
		{
			name:       "loop max_iterations - division",
			expression: "20 / 4",
			context:    &interpolation.Context{},
			want:       5,
		},
		{
			name:       "loop max_iterations - from input",
			expression: "inputs.max_iterations",
			context: &interpolation.Context{
				Inputs: map[string]any{"max_iterations": 10},
			},
			want: 10,
		},
		{
			name:       "loop max_iterations - calculated from input",
			expression: "inputs.total / inputs.batch_size",
			context: &interpolation.Context{
				Inputs: map[string]any{
					"total":      100,
					"batch_size": 10,
				},
			},
			want: 10,
		},
		{
			name:       "complex arithmetic",
			expression: "(10 + 5) * 2 - 3",
			context:    &interpolation.Context{},
			want:       27,
		},
		{
			name:       "modulo operator",
			expression: "17 % 5",
			context:    &interpolation.Context{},
			want:       2,
		},
		{
			name:       "negative numbers",
			expression: "-10 + 5",
			context:    &interpolation.Context{},
			want:       -5,
		},
		{
			name:       "state value access",
			expression: "states.counter.ExitCode",
			context: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"counter": {ExitCode: 42},
				},
			},
			want: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewExprEvaluator()

			result, err := evaluator.EvaluateInt(tt.expression, tt.context)

			require.NoError(t, err, "expression should evaluate without error")
			assert.Equal(t, tt.want, result, "expression result mismatch")
		})
	}
}

func TestExprEvaluator_EvaluateInt_EmptyExpression(t *testing.T) {
	evaluator := NewExprEvaluator()
	ctx := &interpolation.Context{}

	result, err := evaluator.EvaluateInt("", ctx)

	assert.Error(t, err, "empty expression should return error")
	assert.Equal(t, 0, result, "error should return zero value")
}

func TestExprEvaluator_EvaluateInt_NilContext(t *testing.T) {
	evaluator := NewExprEvaluator()

	result, err := evaluator.EvaluateInt("42", nil)

	require.NoError(t, err)
	assert.Equal(t, 42, result)
}

func TestExprEvaluator_EvaluateInt_TypeCoercion_Int64(t *testing.T) {
	evaluator := NewExprEvaluator()
	ctx := &interpolation.Context{
		Inputs: map[string]any{"count": int64(999999)},
	}

	result, err := evaluator.EvaluateInt("inputs.count", ctx)

	require.NoError(t, err)
	assert.Equal(t, 999999, result)
}

func TestExprEvaluator_EvaluateInt_TypeCoercion_Float64(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		context    *interpolation.Context
		want       int
	}{
		{
			name:       "float to int - truncation",
			expression: "inputs.ratio",
			context: &interpolation.Context{
				Inputs: map[string]any{"ratio": 3.14},
			},
			want: 3,
		},
		{
			name:       "float division result",
			expression: "10 / 3",
			context:    &interpolation.Context{},
			want:       3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewExprEvaluator()

			result, err := evaluator.EvaluateInt(tt.expression, tt.context)

			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestExprEvaluator_EvaluateInt_TypeCoercion_StringNumber(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int
	}{
		{"positive integer", "42", 42},
		{"negative integer", "-10", -10},
		{"zero", "0", 0},
		{"large number", "999999", 999999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewExprEvaluator()
			ctx := &interpolation.Context{
				Inputs: map[string]any{"count": tt.value},
			}

			result, err := evaluator.EvaluateInt("inputs.count", ctx)

			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestExprEvaluator_EvaluateInt_DivisionByZero(t *testing.T) {
	evaluator := NewExprEvaluator()
	ctx := &interpolation.Context{}

	result, err := evaluator.EvaluateInt("10 / 0", ctx)

	require.Error(t, err, "division by zero should return error")
	assert.Equal(t, 0, result)
}

func TestExprEvaluator_EvaluateInt_Overflow(t *testing.T) {
	evaluator := NewExprEvaluator()
	ctx := &interpolation.Context{}

	result, err := evaluator.EvaluateInt("999999999 * 999999999", ctx)

	// Implementation may handle overflow differently - verify it doesn't panic
	// Either succeeds with wrapped value or returns error
	if err == nil {
		assert.NotEqual(t, 0, result, "should have some result")
	} else {
		assert.Equal(t, 0, result, "error should return zero")
	}
}

func TestExprEvaluator_EvaluateInt_InvalidSyntax(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{"unclosed parenthesis", "(10 + 5"},
		{"invalid operator", "10 ++ 5"},
		{"missing operand", "10 +"},
		{"double operators", "10 ** 5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewExprEvaluator()
			ctx := &interpolation.Context{}

			result, err := evaluator.EvaluateInt(tt.expression, ctx)

			require.Error(t, err, "invalid syntax should return error")
			assert.Equal(t, 0, result, "error should return zero")
		})
	}
}

func TestExprEvaluator_EvaluateInt_NonNumericResult(t *testing.T) {
	evaluator := NewExprEvaluator()
	ctx := &interpolation.Context{
		Inputs: map[string]any{"message": "hello"},
	}

	result, err := evaluator.EvaluateInt("inputs.message", ctx)

	require.Error(t, err, "non-numeric result should return error")
	assert.Equal(t, 0, result)
}

func TestExprEvaluator_EvaluateInt_BooleanExpression(t *testing.T) {
	evaluator := NewExprEvaluator()
	ctx := &interpolation.Context{
		Inputs: map[string]any{"count": 10},
	}

	result, err := evaluator.EvaluateInt("inputs.count > 5", ctx)

	require.Error(t, err, "boolean expression should return error")
	assert.Equal(t, 0, result)
}

func TestExprEvaluator_EvaluateInt_LoopMaxIterations(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		context    *interpolation.Context
		want       int
	}{
		{
			name:       "direct integer literal",
			expression: "10",
			context:    &interpolation.Context{},
			want:       10,
		},
		{
			name:       "from input variable",
			expression: "inputs.max_iterations",
			context: &interpolation.Context{
				Inputs: map[string]any{"max_iterations": 5},
			},
			want: 5,
		},
		{
			name:       "calculated from inputs - division",
			expression: "inputs.total / inputs.batch_size",
			context: &interpolation.Context{
				Inputs: map[string]any{
					"total":      100,
					"batch_size": 10,
				},
			},
			want: 10,
		},
		{
			name:       "calculated from inputs - addition",
			expression: "inputs.base + inputs.extra",
			context: &interpolation.Context{
				Inputs: map[string]any{
					"base":  10,
					"extra": 5,
				},
			},
			want: 15,
		},
		{
			name:       "from state exit code",
			expression: "states.counter.ExitCode",
			context: &interpolation.Context{
				States: map[string]interpolation.StepStateData{
					"counter": {ExitCode: 7},
				},
			},
			want: 7,
		},
		{
			name:       "complex calculation",
			expression: "(inputs.pages * inputs.items_per_page) / inputs.parallel_tasks",
			context: &interpolation.Context{
				Inputs: map[string]any{
					"pages":          10,
					"items_per_page": 20,
					"parallel_tasks": 4,
				},
			},
			want: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewExprEvaluator()

			result, err := evaluator.EvaluateInt(tt.expression, tt.context)

			require.NoError(t, err, "loop max_iterations expression should evaluate")
			assert.Equal(t, tt.want, result, "max_iterations mismatch")
		})
	}
}

func TestExprEvaluator_Consistency_MultipleEvaluations(t *testing.T) {
	// Verify that multiple evaluations produce consistent results
	evaluator := NewExprEvaluator()
	ctx := &interpolation.Context{
		Inputs: map[string]any{"count": 10},
	}

	result1, err1 := evaluator.EvaluateBool("inputs.count > 5", ctx)
	result2, err2 := evaluator.EvaluateBool("inputs.count > 5", ctx)
	result3, err3 := evaluator.EvaluateBool("inputs.count > 5", ctx)

	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NoError(t, err3)
	assert.Equal(t, result1, result2)
	assert.Equal(t, result2, result3)
}

func TestExprEvaluator_Consistency_NoSideEffects(t *testing.T) {
	// Verify that evaluation doesn't modify context
	evaluator := NewExprEvaluator()
	ctx := &interpolation.Context{
		Inputs: map[string]any{"count": 10},
	}
	originalCount := ctx.Inputs["count"]

	_, _ = evaluator.EvaluateBool("inputs.count > 5", ctx)
	_, _ = evaluator.EvaluateInt("inputs.count + 5", ctx)

	assert.Equal(t, originalCount, ctx.Inputs["count"], "context should not be modified")
}

func TestExprEvaluator_Consistency_MultipleInstances(t *testing.T) {
	// Verify that multiple evaluator instances work independently
	evaluator1 := NewExprEvaluator()
	evaluator2 := NewExprEvaluator()
	ctx := &interpolation.Context{
		Inputs: map[string]any{"count": 10},
	}

	result1, err1 := evaluator1.EvaluateInt("inputs.count + 5", ctx)
	result2, err2 := evaluator2.EvaluateInt("inputs.count + 5", ctx)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, result1, result2)
	assert.Equal(t, 15, result1)
}
