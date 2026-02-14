package application_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/testutil"
	"github.com/vanoix/awf/pkg/interpolation"
)

//
// These tests verify the refactored LoopExecutor implementation that uses
// ports.ExpressionEvaluator interface instead of direct expr-lang dependency.
//
// Coverage areas:
// - Happy path: Normal arithmetic expression evaluation via port
// - Edge cases: Direct integer parsing, whitespace trimming, boundary values
// - Error handling: Evaluator errors, invalid expressions, out-of-bounds values
//
// Related:
// - C042: Fix DIP Violations in Application Layer
// - Component T007: Refactor LoopExecutor to use ports.ExpressionEvaluator

func TestLoopExecutor_ResolveMaxIterations_DelegatesArithmeticToEvaluator(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Configure evaluator to return 5 for the arithmetic expression
	evaluator.SetIntResult(5, nil)

	// Resolver converts "{{inputs.total / inputs.batch}}" → "20 / 4"
	resolver.results["{{inputs.total / inputs.batch}}"] = "20 / 4"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["total"] = "20"
	ctx.Inputs["batch"] = "4"

	result, err := exec.ResolveMaxIterations("{{inputs.total / inputs.batch}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 5, result)
}

func TestLoopExecutor_ResolveMaxIterations_DirectIntegerParsing(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "42"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "42"

	result, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 42, result)
}

func TestLoopExecutor_ResolveMaxIterations_WhitespaceTrimming(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Simulate command output with trailing newline
	resolver.results["{{steps.count.output}}"] = "  10  \n"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	result, err := exec.ResolveMaxIterations("{{steps.count.output}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 10, result)
}

func TestLoopExecutor_ResolveMaxIterations_ComplexArithmetic(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression: (2 + 3) * 4 = 20
	evaluator.SetIntResult(20, nil)
	resolver.results["{{(inputs.a + inputs.b) * inputs.c}}"] = "(2 + 3) * 4"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	result, err := exec.ResolveMaxIterations("{{(inputs.a + inputs.b) * inputs.c}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 20, result)
}

func TestLoopExecutor_ResolveMaxIterations_MinimumValue(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.min}}"] = "1"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	result, err := exec.ResolveMaxIterations("{{inputs.min}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result)
}

func TestLoopExecutor_ResolveMaxIterations_MaximumValue(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.max}}"] = "10000"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	result, err := exec.ResolveMaxIterations("{{inputs.max}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 10000, result)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticWithAddition(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	evaluator.SetIntResult(8, nil)
	resolver.results["{{inputs.a + inputs.b}}"] = "5 + 3"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	result, err := exec.ResolveMaxIterations("{{inputs.a + inputs.b}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 8, result)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticWithSubtraction(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	evaluator.SetIntResult(7, nil)
	resolver.results["{{inputs.total - inputs.offset}}"] = "10 - 3"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	result, err := exec.ResolveMaxIterations("{{inputs.total - inputs.offset}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 7, result)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticWithMultiplication(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	evaluator.SetIntResult(15, nil)
	resolver.results["{{inputs.pages * inputs.items}}"] = "3 * 5"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	result, err := exec.ResolveMaxIterations("{{inputs.pages * inputs.items}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 15, result)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticWithDivision(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	evaluator.SetIntResult(4, nil)
	resolver.results["{{inputs.total / inputs.divisor}}"] = "12 / 3"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	result, err := exec.ResolveMaxIterations("{{inputs.total / inputs.divisor}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 4, result)
}

func TestLoopExecutor_ResolveMaxIterations_EmptyExpression(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	_, err := exec.ResolveMaxIterations("", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestLoopExecutor_ResolveMaxIterations_ResolverError(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.err = errors.New("missing variable")

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	_, err := exec.ResolveMaxIterations("{{inputs.missing}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve max_iterations expression")
}

func TestLoopExecutor_ResolveMaxIterations_EvaluatorError(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression contains arithmetic, so evaluator will be called
	evaluator.SetIntResult(0, errors.New("division by zero"))
	resolver.results["{{inputs.total / inputs.zero}}"] = "10 / 0"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	_, err := exec.ResolveMaxIterations("{{inputs.total / inputs.zero}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "evaluation failed")
	assert.Contains(t, err.Error(), "division by zero")
}

func TestLoopExecutor_ResolveMaxIterations_InvalidInteger(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.invalid}}"] = "abc123"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	_, err := exec.ResolveMaxIterations("{{inputs.invalid}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestLoopExecutor_ResolveMaxIterations_ZeroValue(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	evaluator.SetIntResult(0, nil)
	resolver.results["{{inputs.total - inputs.total}}"] = "5 - 5"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	_, err := exec.ResolveMaxIterations("{{inputs.total - inputs.total}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be at least 1")
}

func TestLoopExecutor_ResolveMaxIterations_NegativeValue(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	evaluator.SetIntResult(-5, nil)
	resolver.results["{{inputs.a - inputs.b}}"] = "3 - 8"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	_, err := exec.ResolveMaxIterations("{{inputs.a - inputs.b}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be at least 1")
}

func TestLoopExecutor_ResolveMaxIterations_ExceedsMaximum(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	evaluator.SetIntResult(100001, nil)
	resolver.results["{{inputs.huge * inputs.multiplier}}"] = "10000 * 11"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	_, err := exec.ResolveMaxIterations("{{inputs.huge * inputs.multiplier}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed limit")
}

func TestLoopExecutor_EvaluateArithmeticExpression_DelegatesToEvaluatorPort(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Set up custom function to verify delegation
	called := false
	evaluator.SetEvaluateIntFunc(func(expr string, ctx *interpolation.Context) (int, error) {
		called = true
		assert.Equal(t, "20 / 4", expr)
		return 5, nil
	})

	resolver.results["{{inputs.expr}}"] = "20 / 4"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	result, err := exec.ResolveMaxIterations("{{inputs.expr}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 5, result)
	assert.True(t, called, "EvaluateInt should have been called on the port")
}

func TestLoopExecutor_EvaluateArithmeticExpression_ReceivesEmptyContext(t *testing.T) {
	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Capture context passed to evaluator
	var capturedCtx *interpolation.Context
	evaluator.SetEvaluateIntFunc(func(expr string, ctx *interpolation.Context) (int, error) {
		capturedCtx = ctx
		return 10, nil
	})

	resolver.results["{{inputs.expr}}"] = "5 + 5"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["foo"] = "bar" // Add data to verify it's NOT passed to evaluator

	_, err := exec.ResolveMaxIterations("{{inputs.expr}}", ctx)

	require.NoError(t, err)
	require.NotNil(t, capturedCtx)
	assert.Empty(t, capturedCtx.Inputs, "Arithmetic evaluation should use empty context")
}

func TestLoopExecutor_NoDirectExprLangDependency(t *testing.T) {
	// This test verifies the refactoring goal: LoopExecutor should not
	// directly depend on expr-lang. All expression evaluation should go
	// through the ports.ExpressionEvaluator interface.
	//
	// This is a documentation test - the actual verification is done via:
	// 1. Code review (no expr-lang import in loop_executor.go)
	// 2. Architecture tests (C042 compliance test)
	// 3. Compilation success (if interface not used, code won't compile)

	logger := &mockLogger{}
	evaluator := testutil.NewMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	evaluator.SetIntResult(42, nil)
	resolver.results["{{expr}}"] = "6 * 7"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	result, err := exec.ResolveMaxIterations("{{expr}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 42, result)
}

func TestLoopExecutor_ResolveMaxIterations_Refactored_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		expression     string
		resolvedValue  string
		evaluatorInt   int
		evaluatorErr   error
		resolverErr    error
		expectedResult int
		expectedError  string
	}{
		{
			name:           "direct integer - small",
			expression:     "{{inputs.count}}",
			resolvedValue:  "5",
			expectedResult: 5,
		},
		{
			name:           "direct integer - large",
			expression:     "{{inputs.count}}",
			resolvedValue:  "9999",
			expectedResult: 9999,
		},
		{
			name:           "arithmetic - addition",
			expression:     "{{a + b}}",
			resolvedValue:  "10 + 5",
			evaluatorInt:   15,
			expectedResult: 15,
		},
		{
			name:           "arithmetic - subtraction",
			expression:     "{{a - b}}",
			resolvedValue:  "20 - 8",
			evaluatorInt:   12,
			expectedResult: 12,
		},
		{
			name:           "arithmetic - multiplication",
			expression:     "{{a * b}}",
			resolvedValue:  "6 * 7",
			evaluatorInt:   42,
			expectedResult: 42,
		},
		{
			name:           "arithmetic - division",
			expression:     "{{a / b}}",
			resolvedValue:  "100 / 20",
			evaluatorInt:   5,
			expectedResult: 5,
		},
		{
			name:           "whitespace handling",
			expression:     "{{output}}",
			resolvedValue:  "  42  \n",
			expectedResult: 42,
		},
		{
			name:          "evaluator error",
			expression:    "{{expr}}",
			resolvedValue: "1 / 0",
			evaluatorErr:  errors.New("division by zero"),
			expectedError: "division by zero",
		},
		{
			name:          "resolver error",
			expression:    "{{missing}}",
			resolverErr:   errors.New("variable not found"),
			expectedError: "resolve max_iterations expression",
		},
		{
			name:          "invalid integer",
			expression:    "{{text}}",
			resolvedValue: "abc123",
			expectedError: "invalid",
		},
		{
			name:           "boundary - minimum (1)",
			expression:     "{{min}}",
			resolvedValue:  "1",
			expectedResult: 1,
		},
		{
			name:           "boundary - maximum (10000)",
			expression:     "{{max}}",
			resolvedValue:  "10000",
			expectedResult: 10000,
		},
		{
			name:          "boundary - zero",
			expression:    "{{zero}}",
			resolvedValue: "0 + 0",
			evaluatorInt:  0,
			expectedError: "must be at least 1",
		},
		{
			name:          "boundary - negative",
			expression:    "{{neg}}",
			resolvedValue: "5 - 10",
			evaluatorInt:  -5,
			expectedError: "must be at least 1",
		},
		{
			name:          "boundary - exceeds max",
			expression:    "{{huge}}",
			resolvedValue: "1000 * 1000",
			evaluatorInt:  1000000,
			expectedError: "exceeds maximum allowed limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			evaluator := testutil.NewMockExpressionEvaluator()
			resolver := newConfigurableMockResolver()

			if tt.resolverErr != nil {
				resolver.err = tt.resolverErr
			} else {
				resolver.results[tt.expression] = tt.resolvedValue
			}

			if tt.evaluatorErr != nil {
				evaluator.SetIntResult(0, tt.evaluatorErr)
			} else if tt.evaluatorInt != 0 {
				evaluator.SetIntResult(tt.evaluatorInt, nil)
			}

			exec := application.NewLoopExecutor(logger, evaluator, resolver)
			ctx := interpolation.NewContext()

			result, err := exec.ResolveMaxIterations(tt.expression, ctx)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}
