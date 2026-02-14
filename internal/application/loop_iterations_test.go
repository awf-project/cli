package application_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/pkg/interpolation"
)

//
// These tests verify the dynamic max_iterations resolution functionality
// for loop execution, including:
//
// - Simple integer literals and environment variables
// - Arithmetic expressions (addition, subtraction, multiplication, division, modulo)
// - Complex nested arithmetic expressions
// - Boundary conditions (min=1, max=100,000)
// - Error cases (zero, negative, exceeds max, non-integer, missing variables)
// - Whitespace trimming and step output resolution
// - Table-driven test suite covering all scenarios
//
// Shared mock implementations are in loop_executor_mocks_test.go

func TestLoopExecutor_ResolveMaxIterations_SimpleInteger(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Configure resolver to return the literal value
	resolver.results["{{inputs.limit}}"] = "5"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "5"

	result, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 5, result)
}

func TestLoopExecutor_ResolveMaxIterations_FromEnvVariable(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{env.LOOP_LIMIT}}"] = "10"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Env["LOOP_LIMIT"] = "10"

	result, err := exec.ResolveMaxIterations("{{env.LOOP_LIMIT}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 10, result)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticAddition(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to "2 + 3"
	resolver.results["{{inputs.a + inputs.b}}"] = "2 + 3"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["a"] = "2"
	ctx.Inputs["b"] = "3"

	result, err := exec.ResolveMaxIterations("{{inputs.a + inputs.b}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 5, result)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticMultiplication(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression: pages * retries_per_page
	resolver.results["{{inputs.pages * inputs.retries_per_page}}"] = "3 * 2"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["pages"] = "3"
	ctx.Inputs["retries_per_page"] = "2"

	result, err := exec.ResolveMaxIterations("{{inputs.pages * inputs.retries_per_page}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 6, result)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticSubtraction(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to "10 - 3"
	resolver.results["{{inputs.total - inputs.offset}}"] = "10 - 3"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["total"] = "10"
	ctx.Inputs["offset"] = "3"

	result, err := exec.ResolveMaxIterations("{{inputs.total - inputs.offset}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 7, result)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticDivision(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to "20 / 4" (exact integer division)
	resolver.results["{{inputs.total / inputs.batch_size}}"] = "20 / 4"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["total"] = "20"
	ctx.Inputs["batch_size"] = "4"

	result, err := exec.ResolveMaxIterations("{{inputs.total / inputs.batch_size}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 5, result)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticModulo(t *testing.T) {
	// NOTE: FR-006 only requires +, -, *, / operators.
	// Modulo (%) is NOT in the spec, but expr-lang supports it.
	// The implementation currently doesn't recognize % as an arithmetic operator
	// because it's not in strings.ContainsAny("+-*/").
	// This test verifies that % expressions are NOT currently supported.
	// If modulo support is added later, update this test.
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to "17 % 5" - but % is not recognized as operator
	resolver.results["{{inputs.total % inputs.divisor}}"] = "17 % 5"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["total"] = "17"
	ctx.Inputs["divisor"] = "5"

	_, err := exec.ResolveMaxIterations("{{inputs.total % inputs.divisor}}", ctx)

	// Modulo is not supported per FR-006, should error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticComplexExpression(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to "(2 + 3) * 4"
	resolver.results["{{(inputs.a + inputs.b) * inputs.c}}"] = "(2 + 3) * 4"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["a"] = "2"
	ctx.Inputs["b"] = "3"
	ctx.Inputs["c"] = "4"

	result, err := exec.ResolveMaxIterations("{{(inputs.a + inputs.b) * inputs.c}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 20, result)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticDivisionByZero(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to "10 / 0"
	resolver.results["{{inputs.total / inputs.divisor}}"] = "10 / 0"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["total"] = "10"
	ctx.Inputs["divisor"] = "0"

	_, err := exec.ResolveMaxIterations("{{inputs.total / inputs.divisor}}", ctx)

	// Division by zero should return error (expr-lang behavior may vary)
	// If it returns infinity or NaN, it will fail the integer conversion
	require.Error(t, err)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticNonWholeNumber(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to "7 / 2" = 3.5 (non-whole number)
	// C042: Infrastructure evaluator converts float to int (3.5 → 3)
	resolver.results["{{inputs.a / inputs.b}}"] = "7 / 2"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["a"] = "7"
	ctx.Inputs["b"] = "2"

	result, err := exec.ResolveMaxIterations("{{inputs.a / inputs.b}}", ctx)

	// C042: Float division results are automatically converted to int
	require.NoError(t, err)
	assert.Equal(t, 3, result)
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticInvalidSyntax(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to truly invalid syntax (unclosed parenthesis)
	resolver.results["{{inputs.expr}}"] = "2 + (3 * 4"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["expr"] = "2 + (3 * 4"

	_, err := exec.ResolveMaxIterations("{{inputs.expr}}", ctx)

	// C042: Infrastructure evaluator provides specific syntax error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "evaluation failed")
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticNegativeResult(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to "3 - 10" = -7
	resolver.results["{{inputs.a - inputs.b}}"] = "3 - 10"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["a"] = "3"
	ctx.Inputs["b"] = "10"

	_, err := exec.ResolveMaxIterations("{{inputs.a - inputs.b}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be at least 1")
}

func TestLoopExecutor_ResolveMaxIterations_ArithmeticExceedsMax(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Expression resolves to "5000 * 3" = 15000 (exceeds max 10000)
	resolver.results["{{inputs.a * inputs.b}}"] = "5000 * 3"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["a"] = "5000"
	ctx.Inputs["b"] = "3"

	_, err := exec.ResolveMaxIterations("{{inputs.a * inputs.b}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

func TestLoopExecutor_ResolveMaxIterations_BoundaryMin(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "1"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "1"

	result, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, result)
}

func TestLoopExecutor_ResolveMaxIterations_BoundaryMax(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "10000"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "10000"

	result, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 10000, result)
}

func TestLoopExecutor_ResolveMaxIterations_ErrorZero(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "0"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "0"

	_, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be at least 1")
}

func TestLoopExecutor_ResolveMaxIterations_ErrorNegative(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "-5"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "-5"

	_, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be at least 1")
}

func TestLoopExecutor_ResolveMaxIterations_ErrorExceedsMax(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "10001"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "10001"

	_, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

func TestLoopExecutor_ResolveMaxIterations_ErrorNonInteger(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "not_a_number"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "not_a_number"

	_, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestLoopExecutor_ResolveMaxIterations_ErrorFloat(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{inputs.limit}}"] = "3.14"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "3.14"

	_, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.Error(t, err)
	// Could be "invalid syntax" or custom message about integers
}

func TestLoopExecutor_ResolveMaxIterations_ErrorMissingVariable(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Resolver returns error for missing variable
	resolver.err = errors.New("undefined variable: inputs.undefined_var")

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	_, err := exec.ResolveMaxIterations("{{inputs.undefined_var}}", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "undefined variable")
}

func TestLoopExecutor_ResolveMaxIterations_ErrorEmptyExpression(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()

	_, err := exec.ResolveMaxIterations("", ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestLoopExecutor_ResolveMaxIterations_FromStepOutput(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	resolver.results["{{states.count.output}}"] = "7"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.States["count"] = interpolation.StepStateData{
		Output:   "7",
		ExitCode: 0,
		Status:   "completed",
	}

	result, err := exec.ResolveMaxIterations("{{states.count.output}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 7, result)
}

func TestLoopExecutor_ResolveMaxIterations_TrimWhitespace(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Resolved value has whitespace (e.g., from command output)
	resolver.results["{{inputs.limit}}"] = "  42  \n"

	exec := application.NewLoopExecutor(logger, evaluator, resolver)

	ctx := interpolation.NewContext()
	ctx.Inputs["limit"] = "  42  \n"

	result, err := exec.ResolveMaxIterations("{{inputs.limit}}", ctx)

	require.NoError(t, err)
	assert.Equal(t, 42, result)
}

func TestLoopExecutor_ResolveMaxIterations_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		expr          string
		resolveResult string
		resolveErr    error
		wantValue     int
		wantErr       bool
		errContains   string
	}{
		{
			name:          "valid integer from input",
			expr:          "{{inputs.count}}",
			resolveResult: "5",
			wantValue:     5,
			wantErr:       false,
		},
		{
			name:          "valid min boundary",
			expr:          "{{inputs.min}}",
			resolveResult: "1",
			wantValue:     1,
			wantErr:       false,
		},
		{
			name:          "valid max boundary",
			expr:          "{{inputs.max}}",
			resolveResult: "10000",
			wantValue:     10000,
			wantErr:       false,
		},
		{
			name:          "large valid value",
			expr:          "{{inputs.large}}",
			resolveResult: "9999",
			wantValue:     9999,
			wantErr:       false,
		},
		{
			name:          "zero is invalid",
			expr:          "{{inputs.zero}}",
			resolveResult: "0",
			wantErr:       true,
			errContains:   "at least 1",
		},
		{
			name:          "negative is invalid",
			expr:          "{{inputs.neg}}",
			resolveResult: "-10",
			wantErr:       true,
			errContains:   "at least 1",
		},
		{
			name:          "exceeds max limit",
			expr:          "{{inputs.huge}}",
			resolveResult: "100000",
			wantErr:       true,
			errContains:   "exceeds",
		},
		{
			name:          "non-numeric string",
			expr:          "{{inputs.text}}",
			resolveResult: "hello",
			wantErr:       true,
			errContains:   "invalid",
		},
		{
			name:          "float value",
			expr:          "{{inputs.float}}",
			resolveResult: "5.5",
			wantErr:       true,
		},
		{
			name:       "resolver error",
			expr:       "{{inputs.missing}}",
			resolveErr: errors.New("variable not found"),
			wantErr:    true,
		},
		{
			name:        "empty expression",
			expr:        "",
			wantErr:     true,
			errContains: "empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			evaluator := newMockExpressionEvaluator()
			resolver := newConfigurableMockResolver()

			if tt.resolveErr != nil {
				resolver.err = tt.resolveErr
			} else {
				resolver.results[tt.expr] = tt.resolveResult
			}

			exec := application.NewLoopExecutor(logger, evaluator, resolver)
			ctx := interpolation.NewContext()

			result, err := exec.ResolveMaxIterations(tt.expr, ctx)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantValue, result)
			}
		})
	}
}
