//go:build integration

// Feature: C021
//
// Functional tests for ExpressionValidator port/adapter implementation (C021).
// These tests validate end-to-end expression validation behavior through the
// hexagonal architecture layers: domain ports, infrastructure adapters, and
// application wiring.
//
// Acceptance Criteria Coverage:
// - AC1: Domain layer has zero imports of github.com/expr-lang/expr
// - AC2: ExpressionValidator port interface exists in domain/ports
// - AC3: Infrastructure adapter implements port with compile-time check
// - AC4: All existing tests pass without modification
// - AC5: go list -deps ./internal/domain/... shows no external dependencies
// - AC6: No regressions in workflow validation functionality
//
// Test Strategy:
// - Happy Path: Valid expressions from conversation stop conditions
// - Edge Cases: Empty strings, unicode, special characters, extreme lengths
// - Error Handling: Invalid syntax returns clear error messages
// - Integration: Full workflow validation with conversation stop conditions
//
// Test Categories:
// - Happy Path: Normal expression syntax validation
// - Edge Cases: Boundary conditions and unusual inputs
// - Error Handling: Invalid expressions and error propagation
// - Integration: End-to-end workflow validation with real fixtures

package integration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/infrastructure/expression"
)

// =============================================================================
// Happy Path Tests: Valid Expression Syntax
// =============================================================================

func TestExpressionValidation_HappyPath_ValidExpressions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Given: Real ExprValidator adapter (not mocked)
	validator := expression.NewExprValidator()

	tests := []struct {
		name       string
		expression string
	}{
		{
			name:       "keyword_match",
			expression: "response contains 'COMPLETE'",
		},
		{
			name:       "turn_count_limit",
			expression: "turn_count >= 3",
		},
		{
			name:       "token_budget",
			expression: "total_tokens >= 4500",
		},
		{
			name:       "complex_and_condition",
			expression: "(turn_count >= 5 && response contains 'DONE') || total_tokens >= 10000",
		},
		{
			name:       "numeric_comparison",
			expression: "turn_count > 0 && turn_count < 100",
		},
		{
			name:       "string_contains",
			expression: "response contains 'success' || response contains 'done'",
		},
		{
			name:       "boolean_logic",
			expression: "turn_count >= 5 && total_tokens < 10000",
		},
		{
			name:       "nested_parentheses",
			expression: "((turn_count >= 3) || (total_tokens >= 5000)) && response contains 'END'",
		},
		{
			name:       "not_operator",
			expression: "!(response contains 'ERROR')",
		},
		{
			name:       "equality_check",
			expression: "turn_count == 10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When: Validating expression syntax
			err := validator.Compile(tt.expression)

			// Then: Expression compiles without error
			assert.NoError(t, err, "valid expression should compile: %s", tt.expression)
		})
	}
}

// =============================================================================
// Edge Cases: Boundary Conditions and Unusual Inputs
// =============================================================================

func TestExpressionValidation_EdgeCases_BoundaryConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	validator := expression.NewExprValidator()

	tests := []struct {
		name       string
		expression string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "empty_string",
			expression: "",
			wantErr:    false,
			errMsg:     "empty expression is valid (no stop condition)",
		},
		{
			name:       "whitespace_only",
			expression: "   ",
			wantErr:    false,
			errMsg:     "whitespace-only expression is valid (no stop condition)",
		},
		{
			name:       "unicode_content",
			expression: "response contains '完成'",
			wantErr:    false,
			errMsg:     "unicode in strings should be valid",
		},
		{
			name:       "unicode_operators",
			expression: "turn_count ≥ 3", // Using unicode ≥ instead of >=
			wantErr:    true,
			errMsg:     "unicode operators should fail",
		},
		{
			name:       "special_characters_in_string",
			expression: "response contains '!@#$%^&*()'",
			wantErr:    false,
			errMsg:     "special chars in quoted strings should be valid",
		},
		{
			name:       "escaped_quotes",
			expression: `response contains "He said \"done\""`,
			wantErr:    false,
			errMsg:     "escaped quotes should be valid",
		},
		{
			name:       "newlines_in_expression",
			expression: "turn_count >= 3\n&& total_tokens < 5000",
			wantErr:    false,
			errMsg:     "expressions with newlines should be valid",
		},
		{
			name:       "very_long_expression",
			expression: "turn_count >= 1 && turn_count >= 2 && turn_count >= 3 && turn_count >= 4 && turn_count >= 5 && turn_count >= 6 && turn_count >= 7 && turn_count >= 8 && turn_count >= 9 && turn_count >= 10",
			wantErr:    false,
			errMsg:     "long expressions should be valid",
		},
		{
			name:       "single_boolean",
			expression: "true",
			wantErr:    false,
			errMsg:     "literal boolean should be valid",
		},
		{
			name:       "single_number",
			expression: "42",
			wantErr:    false,
			errMsg:     "literal number should be valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Compile(tt.expression)

			if tt.wantErr {
				assert.Error(t, err, tt.errMsg)
			} else {
				assert.NoError(t, err, tt.errMsg)
			}
		})
	}
}

// =============================================================================
// Error Handling: Invalid Syntax and Error Messages
// =============================================================================

func TestExpressionValidation_ErrorHandling_InvalidSyntax(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	validator := expression.NewExprValidator()

	tests := []struct {
		name        string
		expression  string
		wantErr     bool
		errContains string
	}{
		{
			name:        "unclosed_parenthesis",
			expression:  "(turn_count >= 3",
			wantErr:     true,
			errContains: "unexpected token EOF",
		},
		{
			name:        "invalid_operator",
			expression:  "turn_count >> 3",
			wantErr:     true,
			errContains: "",
		},
		{
			name:        "undefined_variable",
			expression:  "undefined_var >= 3",
			wantErr:     false, // Note: expr.Compile only checks syntax, not variable existence
			errContains: "",
		},
		{
			name:        "malformed_comparison",
			expression:  "turn_count >= >= 3",
			wantErr:     true,
			errContains: "",
		},
		{
			name:        "missing_operand",
			expression:  "turn_count &&",
			wantErr:     true,
			errContains: "unexpected token EOF",
		},
		{
			name:        "invalid_string_literal",
			expression:  "response contains 'unclosed",
			wantErr:     true,
			errContains: "",
		},
		{
			name:        "mismatched_quotes",
			expression:  `response contains "mixed'`,
			wantErr:     true,
			errContains: "",
		},
		{
			name:        "division_by_zero",
			expression:  "turn_count / 0",
			wantErr:     false, // Syntax is valid, runtime error would occur during evaluation
			errContains: "",
		},
		{
			name:        "invalid_function_call",
			expression:  "contains(response, 'DONE')",
			wantErr:     true, // Function calls are disabled via DisableAllBuiltins()
			errContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Compile(tt.expression)

			if tt.wantErr {
				assert.Error(t, err, "expression should fail validation: %s", tt.expression)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err, "expression should be valid: %s", tt.expression)
			}
		})
	}
}

// =============================================================================
// Integration: Port Interface Compliance
// =============================================================================

func TestExpressionValidation_Integration_PortCompliance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Given: ExprValidator must implement ports.ExpressionValidator interface
	var _ ports.ExpressionValidator = (*expression.ExprValidator)(nil)

	// When: Creating validator instance
	validator := expression.NewExprValidator()
	require.NotNil(t, validator, "validator should be instantiated")

	// Then: Interface method is callable
	err := validator.Compile("turn_count >= 3")
	assert.NoError(t, err, "interface method should work")
}

// =============================================================================
// Integration: Real-World Conversation Stop Conditions
// =============================================================================

func TestExpressionValidation_Integration_ConversationStopConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	validator := expression.NewExprValidator()

	// Test cases derived from conversation-stop-condition.yaml fixture
	tests := []struct {
		name       string
		condition  string
		stepName   string
		wantErr    bool
	}{
		{
			name:      "keyword_match_step",
			condition: "response contains 'COMPLETE'",
			stepName:  "keyword_match",
			wantErr:   false,
		},
		{
			name:      "turn_count_limit_step",
			condition: "turn_count >= 3",
			stepName:  "turn_count_limit",
			wantErr:   false,
		},
		{
			name:      "token_budget_step",
			condition: "total_tokens >= 4500",
			stepName:  "token_budget",
			wantErr:   false,
		},
		{
			name:      "complex_expression_step",
			condition: "(turn_count >= 5 && response contains 'DONE') || total_tokens >= 10000",
			stepName:  "complex_expression",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When: Validating stop condition from real workflow
			err := validator.Compile(tt.condition)

			// Then: Condition validates according to expectations
			if tt.wantErr {
				assert.Error(t, err, "stop condition should fail for step %s", tt.stepName)
			} else {
				assert.NoError(t, err, "stop condition should be valid for step %s", tt.stepName)
			}
		})
	}
}

// =============================================================================
// Integration: Validator Factory Function
// =============================================================================

func TestExpressionValidation_Integration_ValidatorFactory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// When: Creating multiple validator instances
	validator1 := expression.NewExprValidator()
	validator2 := expression.NewExprValidator()

	// Then: Both instances work independently
	require.NotNil(t, validator1)
	require.NotNil(t, validator2)

	err1 := validator1.Compile("turn_count >= 3")
	err2 := validator2.Compile("total_tokens >= 5000")

	assert.NoError(t, err1)
	assert.NoError(t, err2)
}

// =============================================================================
// Integration: Error Message Clarity
// =============================================================================

func TestExpressionValidation_Integration_ErrorMessageClarity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	validator := expression.NewExprValidator()

	tests := []struct {
		name           string
		expression     string
		wantErr        bool
		errorShouldBe  string
		errorMentions  []string
	}{
		{
			name:          "unclosed_parenthesis_clear_message",
			expression:    "(turn_count >= 3",
			wantErr:       true,
			errorMentions: []string{"unexpected", "EOF"},
		},
		{
			name:          "missing_operand_clear_message",
			expression:    "turn_count &&",
			wantErr:       true,
			errorMentions: []string{"unexpected", "EOF"},
		},
		{
			name:       "valid_expression_no_error",
			expression: "turn_count >= 3",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Compile(tt.expression)

			if tt.wantErr {
				require.Error(t, err, "expression should produce error")
				errMsg := err.Error()
				for _, mention := range tt.errorMentions {
					assert.Contains(t, errMsg, mention,
						"error message should mention '%s': got %q", mention, errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// Integration: Concurrent Validation
// =============================================================================

func TestExpressionValidation_Integration_ConcurrentValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	validator := expression.NewExprValidator()

	// When: Multiple goroutines validate expressions concurrently
	expressions := []string{
		"turn_count >= 3",
		"total_tokens >= 5000",
		"response contains 'DONE'",
		"(turn_count >= 5 && response contains 'END')",
		"turn_count > 0 && turn_count < 100",
	}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			expr := expressions[idx%len(expressions)]
			err := validator.Compile(expr)
			assert.NoError(t, err, "concurrent validation should work for: %s", expr)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
