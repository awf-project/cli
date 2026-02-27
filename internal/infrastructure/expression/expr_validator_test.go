package expression

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: T003
// Feature: C021

func TestExprValidator_InterfaceCompliance(t *testing.T) {
	// Verify ExprValidator implements ports.ExpressionValidator
	var _ ports.ExpressionValidator = (*ExprValidator)(nil)
}

func TestNewExprValidator(t *testing.T) {
	validator := NewExprValidator()

	require.NotNil(t, validator)
	assert.IsType(t, &ExprValidator{}, validator)
}

func TestExprValidator_Compile_ValidExpressions(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{
			name:       "simple contains",
			expression: "response contains 'DONE'",
		},
		{
			name:       "turn count comparison",
			expression: "turn_count >= 5",
		},
		{
			name:       "token comparison",
			expression: "total_tokens > 50000",
		},
		{
			name:       "logical AND",
			expression: "turn_count >= 3 && response contains 'APPROVED'",
		},
		{
			name:       "logical OR",
			expression: "turn_count >= 5 || total_tokens > 10000",
		},
		{
			name:       "complex nested expression",
			expression: "(turn_count >= 5 || total_tokens > 10000) && response contains 'COMPLETE'",
		},
		{
			name:       "equality check",
			expression: "status == 'finished'",
		},
		{
			name:       "inequality check",
			expression: "status != 'pending'",
		},
		{
			name:       "less than",
			expression: "count < 100",
		},
		{
			name:       "less than or equal",
			expression: "count <= 100",
		},
		{
			name:       "greater than",
			expression: "count > 0",
		},
		{
			name:       "greater than or equal",
			expression: "count >= 0",
		},
		{
			name:       "multiple AND conditions",
			expression: "a > 0 && b > 0 && c > 0",
		},
		{
			name:       "multiple OR conditions",
			expression: "status == 'done' || status == 'complete' || status == 'finished'",
		},
		{
			name:       "mixed AND/OR with parentheses",
			expression: "(a > 0 && b > 0) || (c > 0 && d > 0)",
		},
		{
			name:       "arithmetic expression",
			expression: "total_tokens + remaining_tokens > 100000",
		},
		{
			name:       "string concatenation check",
			expression: "prefix + suffix == 'complete'",
		},
		{
			name:       "contains with double quotes",
			expression: `response contains "DONE"`,
		},
		{
			name:       "ternary-like expression",
			expression: "count > 10 ? 'high' : 'low'",
		},
		{
			name:       "array/list membership",
			expression: "'error' in status_list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewExprValidator()

			err := validator.Compile(tt.expression)

			require.NoError(t, err, "expression should compile successfully: %s", tt.expression)
		})
	}
}

func TestExprValidator_Compile_EmptyExpression(t *testing.T) {
	validator := NewExprValidator()

	err := validator.Compile("")

	// Empty expression should be valid (no early exit condition)
	require.NoError(t, err)
}

func TestExprValidator_Compile_WhitespaceOnly(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{"single space", " "},
		{"multiple spaces", "   "},
		{"tabs", "\t\t"},
		{"newlines", "\n\n"},
		{"mixed whitespace", " \t\n "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewExprValidator()

			err := validator.Compile(tt.expression)

			// Whitespace-only should be treated as empty (valid)
			require.NoError(t, err)
		})
	}
}

func TestExprValidator_Compile_ExpressionWithLeadingTrailingWhitespace(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{"leading space", " count > 5"},
		{"trailing space", "count > 5 "},
		{"both leading and trailing", "  count > 5  "},
		{"leading tab", "\tcount > 5"},
		{"trailing newline", "count > 5\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewExprValidator()

			err := validator.Compile(tt.expression)

			require.NoError(t, err)
		})
	}
}

func TestExprValidator_Compile_MultilineExpression(t *testing.T) {
	validator := NewExprValidator()
	expression := `turn_count >= 5 &&
		total_tokens > 10000 &&
		response contains 'COMPLETE'`

	err := validator.Compile(expression)

	require.NoError(t, err)
}

func TestExprValidator_Compile_VeryLongExpression(t *testing.T) {
	validator := NewExprValidator()
	// Build a very long but valid expression
	expression := "(a > 0 && b > 0) || (c > 0 && d > 0) || (e > 0 && f > 0) || (g > 0 && h > 0) || (i > 0 && j > 0) || (k > 0 && l > 0) || (m > 0 && n > 0) || (o > 0 && p > 0)"

	err := validator.Compile(expression)

	require.NoError(t, err)
}

func TestExprValidator_Compile_DeeplyNestedParentheses(t *testing.T) {
	validator := NewExprValidator()
	expression := "((((a > 0))))"

	err := validator.Compile(expression)

	require.NoError(t, err)
}

func TestExprValidator_Compile_NumericLiterals(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{"integer", "count == 42"},
		{"negative integer", "count == -42"},
		{"float", "ratio == 3.14"},
		{"negative float", "ratio == -3.14"},
		{"scientific notation", "value == 1e6"},
		{"zero", "count == 0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewExprValidator()

			err := validator.Compile(tt.expression)

			require.NoError(t, err)
		})
	}
}

func TestExprValidator_Compile_StringLiterals(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{"single quotes", "status == 'done'"},
		{"double quotes", `status == "done"`},
		{"empty string single quotes", "status == ''"},
		{"empty string double quotes", `status == ""`},
		{"string with spaces", "status == 'not done'"},
		{"string with special chars", "status == 'done!'"},
		{"string with escaped quote", `message == "It's done"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewExprValidator()

			err := validator.Compile(tt.expression)

			require.NoError(t, err)
		})
	}
}

func TestExprValidator_Compile_BooleanLiterals(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{"true literal", "is_complete == true"},
		{"false literal", "is_complete == false"},
		{"true standalone", "true"},
		{"false standalone", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewExprValidator()

			err := validator.Compile(tt.expression)

			require.NoError(t, err)
		})
	}
}

func TestExprValidator_Compile_InvalidSyntax(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		reason     string
	}{
		{
			name:       "missing closing quote",
			expression: "response contains 'DONE",
			reason:     "unterminated string literal",
		},
		{
			name:       "missing opening quote",
			expression: "response contains DONE'",
			reason:     "unterminated string literal",
		},
		{
			name:       "unclosed parenthesis",
			expression: "(count > 5",
			reason:     "missing closing parenthesis",
		},
		{
			name:       "extra closing parenthesis",
			expression: "count > 5)",
			reason:     "unexpected closing parenthesis",
		},
		{
			name:       "invalid operator",
			expression: "count >< 5",
			reason:     "invalid comparison operator",
		},
		{
			name:       "missing right operand",
			expression: "count >",
			reason:     "incomplete expression",
		},
		{
			name:       "missing left operand",
			expression: "> 5",
			reason:     "incomplete expression",
		},
		{
			name:       "double operators",
			expression: "count >> 5",
			reason:     "invalid operator sequence",
		},
		{
			name:       "invalid variable name",
			expression: "123count > 5",
			reason:     "variable cannot start with digit",
		},
		{
			name:       "unclosed string in complex expression",
			expression: "turn_count >= 3 && response contains 'APPROVED",
			reason:     "unterminated string in complex expression",
		},
		{
			name:       "mismatched quotes",
			expression: `status == 'done"`,
			reason:     "mismatched quote types",
		},
		{
			name:       "invalid character",
			expression: "count @ 5",
			reason:     "invalid character in expression",
		},
		{
			name:       "missing operator between operands",
			expression: "count 5",
			reason:     "missing operator",
		},
		{
			name:       "empty parentheses",
			expression: "()",
			reason:     "empty expression in parentheses",
		},
		{
			name:       "consecutive operators",
			expression: "count && || 5",
			reason:     "consecutive logical operators",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewExprValidator()

			err := validator.Compile(tt.expression)

			require.Error(t, err, "expression should fail to compile: %s (reason: %s)", tt.expression, tt.reason)
			assert.NotEmpty(t, err.Error(), "error message should not be empty")
		})
	}
}

func TestExprValidator_Compile_RealWorldStopConditions(t *testing.T) {
	// These are actual stop condition patterns used in workflows
	tests := []struct {
		name       string
		expression string
	}{
		{
			name:       "simple turn limit",
			expression: "turn_count >= 10",
		},
		{
			name:       "token budget exhausted",
			expression: "total_tokens > 100000",
		},
		{
			name:       "user confirmation received",
			expression: "response contains 'CONFIRMED'",
		},
		{
			name:       "error detected",
			expression: "response contains 'ERROR' || response contains 'FAILED'",
		},
		{
			name:       "task complete with confirmation",
			expression: "turn_count >= 3 && response contains 'COMPLETE'",
		},
		{
			name:       "budget or completion",
			expression: "total_tokens > 50000 || response contains 'DONE'",
		},
		{
			name:       "multi-condition early exit",
			expression: "(turn_count >= 5 || total_tokens > 10000) && response contains 'COMPLETE'",
		},
		{
			name:       "complex approval workflow",
			expression: "(turn_count >= 3 && response contains 'APPROVED') || (turn_count >= 10 && response contains 'PENDING')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewExprValidator()

			err := validator.Compile(tt.expression)

			require.NoError(t, err, "real-world stop condition should compile: %s", tt.expression)
		})
	}
}

func TestExprValidator_Compile_Idempotency(t *testing.T) {
	// Verify that compiling the same expression multiple times produces consistent results
	validator := NewExprValidator()
	expression := "turn_count >= 5 && response contains 'DONE'"

	for i := range 5 {
		err := validator.Compile(expression)

		require.NoError(t, err, "iteration %d should compile successfully", i)
	}
}

func TestExprValidator_Compile_MultipleValidators(t *testing.T) {
	// Verify that multiple validator instances work independently
	validator1 := NewExprValidator()
	validator2 := NewExprValidator()
	expression := "count > 5"

	err1 := validator1.Compile(expression)
	err2 := validator2.Compile(expression)

	require.NoError(t, err1)
	require.NoError(t, err2)
}

func TestExprValidator_Compile_DifferentExpressionsSequentially(t *testing.T) {
	// Verify that validator can handle different expressions in sequence
	validator := NewExprValidator()
	expressions := []string{
		"count > 5",
		"status == 'done'",
		"turn_count >= 10",
		"response contains 'COMPLETE'",
	}

	for _, expr := range expressions {
		err := validator.Compile(expr)
		require.NoError(t, err, "expression should compile: %s", expr)
	}
}

func TestExprValidator_Compile_NoSideEffects(t *testing.T) {
	// Verify that Compile only validates, doesn't evaluate or modify state
	validator := NewExprValidator()
	expression := "count > 5"

	err1 := validator.Compile(expression)
	err2 := validator.Compile(expression)

	require.NoError(t, err1)
	require.NoError(t, err2)
}

func TestExprValidator_Compile_DoesNotEvaluate(t *testing.T) {
	// Verify that Compile doesn't require variable values (only checks syntax)
	validator := NewExprValidator()
	// Expression with undefined variables - should compile (syntax valid)
	expression := "some_undefined_variable > 5"

	err := validator.Compile(expression)

	require.NoError(t, err, "syntax validation should not require variable definitions")
}
