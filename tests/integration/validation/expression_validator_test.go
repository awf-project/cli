//go:build integration

// Feature: C021
//
// Standalone functional tests for ExpressionValidator port/adapter implementation.
// Tests run independently to avoid conflicts with broken integration test suite.

package validation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/infrastructure/expression"
)

// TestExprValidator_HappyPath validates normal expression syntax
func TestExprValidator_HappyPath(t *testing.T) {
	validator := expression.NewExprValidator()

	validExpressions := []string{
		"response contains 'COMPLETE'",
		"turn_count >= 3",
		"total_tokens >= 4500",
		"(turn_count >= 5 && response contains 'DONE') || total_tokens >= 10000",
	}

	for _, expr := range validExpressions {
		err := validator.Compile(expr)
		assert.NoError(t, err, "valid expression should compile: %s", expr)
	}
}

// TestExprValidator_EdgeCases validates boundary conditions
func TestExprValidator_EdgeCases(t *testing.T) {
	validator := expression.NewExprValidator()

	tests := []struct {
		expr    string
		wantErr bool
	}{
		{"", false},                                     // empty is valid (no condition)
		{"   ", false},                                  // whitespace is valid (no condition)
		{"response contains '完成'", false},               // unicode in strings
		{"response contains '!@#$%^&*()'", false},       // special chars
		{`response contains "He said \"done\""`, false}, // escaped quotes
	}

	for _, tt := range tests {
		err := validator.Compile(tt.expr)
		if tt.wantErr {
			assert.Error(t, err, "should fail: %s", tt.expr)
		} else {
			assert.NoError(t, err, "should succeed: %s", tt.expr)
		}
	}
}

// TestExprValidator_ErrorHandling validates invalid syntax detection
func TestExprValidator_ErrorHandling(t *testing.T) {
	validator := expression.NewExprValidator()

	invalidExpressions := []string{
		"(turn_count >= 3",            // unclosed paren
		"turn_count &&",               // missing operand
		"response contains 'unclosed", // unclosed quote
	}

	for _, expr := range invalidExpressions {
		err := validator.Compile(expr)
		assert.Error(t, err, "invalid expression should fail: %s", expr)
	}
}

// TestExprValidator_PortCompliance verifies interface implementation
func TestExprValidator_PortCompliance(t *testing.T) {
	var _ ports.ExpressionValidator = (*expression.ExprValidator)(nil)

	validator := expression.NewExprValidator()
	require.NotNil(t, validator)

	err := validator.Compile("turn_count >= 3")
	assert.NoError(t, err)
}

// TestExprValidator_ConversationFixtureExpressions validates real workflow expressions
func TestExprValidator_ConversationFixtureExpressions(t *testing.T) {
	validator := expression.NewExprValidator()

	// From conversation-stop-condition.yaml
	fixtureExpressions := map[string]string{
		"keyword_match":      "response contains 'COMPLETE'",
		"turn_count_limit":   "turn_count >= 3",
		"token_budget":       "total_tokens >= 4500",
		"complex_expression": "(turn_count >= 5 && response contains 'DONE') || total_tokens >= 10000",
	}

	for stepName, expr := range fixtureExpressions {
		err := validator.Compile(expr)
		assert.NoError(t, err, "fixture expression should be valid for step %s: %s", stepName, expr)
	}
}

// TestExprValidator_Concurrent validates thread-safety
func TestExprValidator_Concurrent(t *testing.T) {
	validator := expression.NewExprValidator()

	expressions := []string{
		"turn_count >= 3",
		"total_tokens >= 5000",
		"response contains 'DONE'",
	}

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			expr := expressions[idx%len(expressions)]
			err := validator.Compile(expr)
			assert.NoError(t, err, "concurrent validation should work: %s", expr)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
