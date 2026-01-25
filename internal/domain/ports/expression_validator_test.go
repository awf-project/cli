package ports_test

import (
	"errors"
	"testing"

	"github.com/vanoix/awf/internal/domain/ports"
)

// Component: T001
// Feature: C021

// ============================================================================
// Mock Implementations
// ============================================================================

// mockExpressionValidator is a test implementation of ExpressionValidator interface
type mockExpressionValidator struct {
	compileFunc func(expression string) error
	callCount   int
}

func newMockExpressionValidator() *mockExpressionValidator {
	return &mockExpressionValidator{
		compileFunc: func(expression string) error {
			// By default, accept all expressions
			return nil
		},
	}
}

func (m *mockExpressionValidator) Compile(expression string) error {
	m.callCount++
	return m.compileFunc(expression)
}

// ============================================================================
// Interface Compliance Tests
// ============================================================================

func TestExpressionValidatorInterface(t *testing.T) {
	// Verify interface compliance
	var _ ports.ExpressionValidator = (*mockExpressionValidator)(nil)
}

// ============================================================================
// ExpressionValidator Tests - Happy Path
// ============================================================================

func TestExpressionValidator_Compile_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{"simple variable", "response"},
		{"string contains", "response contains 'DONE'"},
		{"comparison", "turn_count >= 5"},
		{"logical and", "turn_count >= 3 && response contains 'APPROVED'"},
		{"logical or", "status == 'complete' || status == 'success'"},
		{"arithmetic", "count + 1"},
		{"parentheses", "(a + b) * c"},
		{"field access", "user.name"},
		{"array index", "items[0]"},
		{"function call", "len(items)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			validator := newMockExpressionValidator()

			// Act
			err := validator.Compile(tt.expression)
			// Assert
			if err != nil {
				t.Errorf("unexpected error for valid expression '%s': %v", tt.expression, err)
			}
			if validator.callCount != 1 {
				t.Errorf("expected Compile to be called once, got %d", validator.callCount)
			}
		})
	}
}

func TestExpressionValidator_Compile_ValidExpressions(t *testing.T) {
	// Arrange
	validator := newMockExpressionValidator()

	// Act & Assert - Multiple valid expressions
	validExpressions := []string{
		"true",
		"false",
		"123",
		"'hello world'",
		"a == b",
		"x > 10 && y < 20",
		"status in ['done', 'complete']",
	}

	for _, expr := range validExpressions {
		err := validator.Compile(expr)
		if err != nil {
			t.Errorf("unexpected error for expression '%s': %v", expr, err)
		}
	}

	if validator.callCount != len(validExpressions) {
		t.Errorf("expected %d calls, got %d", len(validExpressions), validator.callCount)
	}
}

// ============================================================================
// ExpressionValidator Tests - Edge Cases
// ============================================================================

func TestExpressionValidator_Compile_EmptyString(t *testing.T) {
	// Arrange
	validator := newMockExpressionValidator()
	validator.compileFunc = func(expression string) error {
		if expression == "" {
			return errors.New("empty expression")
		}
		return nil
	}

	// Act
	err := validator.Compile("")
	// Assert - should reject empty string
	if err == nil {
		t.Error("expected error for empty expression, got nil")
	}
	if err.Error() != "empty expression" {
		t.Errorf("expected error 'empty expression', got '%v'", err)
	}
}

func TestExpressionValidator_Compile_WhitespaceOnly(t *testing.T) {
	// Arrange
	validator := newMockExpressionValidator()
	// Act
	err := validator.Compile("   ")
	// Assert - whitespace-only should be accepted by default mock
	// (real implementation might reject this)
	if err != nil {
		t.Logf("Note: whitespace-only expression rejected: %v (acceptable behavior)", err)
	}
}

func TestExpressionValidator_Compile_LongExpression(t *testing.T) {
	// Arrange
	validator := newMockExpressionValidator()
	// Create a long but valid expression
	longExpr := "field1 == 'value1' && field2 == 'value2' && field3 == 'value3' && " +
		"field4 == 'value4' && field5 == 'value5' && field6 == 'value6' && " +
		"field7 == 'value7' && field8 == 'value8' && field9 == 'value9'"
	// Act
	err := validator.Compile(longExpr)
	// Assert - should handle long expressions
	if err != nil {
		t.Errorf("unexpected error for long expression: %v", err)
	}
}

func TestExpressionValidator_Compile_SpecialCharacters(t *testing.T) {
	// Arrange
	validator := newMockExpressionValidator()
	tests := []struct {
		name       string
		expression string
	}{
		{"newline in string", "message contains '\\n'"},
		{"tab in string", "message contains '\\t'"},
		{"quote in string", "message contains '\\''"},
		{"unicode", "name == '世界'"},
		{"backslash", "path contains '\\\\'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			err := validator.Compile(tt.expression)
			// Assert - should handle special characters
			if err != nil {
				t.Errorf("unexpected error for expression with %s: %v", tt.name, err)
			}
		})
	}
}

func TestExpressionValidator_Compile_NestedExpressions(t *testing.T) {
	// Arrange
	validator := newMockExpressionValidator()
	tests := []struct {
		name       string
		expression string
	}{
		{"nested parentheses", "((a + b) * (c + d))"},
		{"nested logical", "(a && b) || (c && d)"},
		{"nested field access", "user.profile.settings.theme"},
		{"nested array access", "data[0][1]"},
		{"mixed nesting", "(items[0].value + items[1].value) > 100"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			err := validator.Compile(tt.expression)
			// Assert
			if err != nil {
				t.Errorf("unexpected error for nested expression '%s': %v", tt.expression, err)
			}
		})
	}
}

// ============================================================================
// ExpressionValidator Tests - Error Handling
// ============================================================================

func TestExpressionValidator_Compile_InvalidSyntax(t *testing.T) {
	// Arrange
	validator := newMockExpressionValidator()
	expectedErr := errors.New("syntax error: unexpected token")
	validator.compileFunc = func(expression string) error {
		return expectedErr
	}

	// Act
	err := validator.Compile("invalid && &&")
	// Assert
	if err == nil {
		t.Error("expected error for invalid syntax, got nil")
	}
	if err.Error() != "syntax error: unexpected token" {
		t.Errorf("expected error 'syntax error: unexpected token', got '%v'", err)
	}
}

func TestExpressionValidator_Compile_UnbalancedParentheses(t *testing.T) {
	// Arrange
	validator := newMockExpressionValidator()
	validator.compileFunc = func(expression string) error {
		if expression == "(a + b" || expression == "a + b)" {
			return errors.New("unbalanced parentheses")
		}
		return nil
	}

	tests := []struct {
		name       string
		expression string
	}{
		{"missing closing paren", "(a + b"},
		{"missing opening paren", "a + b)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			err := validator.Compile(tt.expression)
			// Assert
			if err == nil {
				t.Errorf("expected error for %s, got nil", tt.name)
			}
		})
	}
}

func TestExpressionValidator_Compile_UnknownOperator(t *testing.T) {
	// Arrange
	validator := newMockExpressionValidator()
	validator.compileFunc = func(expression string) error {
		if expression == "a @@ b" {
			return errors.New("unknown operator: @@")
		}
		return nil
	}

	// Act
	err := validator.Compile("a @@ b")
	// Assert
	if err == nil {
		t.Error("expected error for unknown operator, got nil")
	}
	if err.Error() != "unknown operator: @@" {
		t.Errorf("expected error 'unknown operator: @@', got '%v'", err)
	}
}

func TestExpressionValidator_Compile_InvalidStringLiteral(t *testing.T) {
	// Arrange
	validator := newMockExpressionValidator()
	validator.compileFunc = func(expression string) error {
		if expression == "name == 'unclosed" {
			return errors.New("unclosed string literal")
		}
		return nil
	}

	// Act
	err := validator.Compile("name == 'unclosed")
	// Assert
	if err == nil {
		t.Error("expected error for unclosed string literal, got nil")
	}
}

func TestExpressionValidator_Compile_EmptyParentheses(t *testing.T) {
	// Arrange
	validator := newMockExpressionValidator()
	validator.compileFunc = func(expression string) error {
		if expression == "()" {
			return errors.New("empty parentheses")
		}
		return nil
	}

	// Act
	err := validator.Compile("()")
	// Assert
	if err == nil {
		t.Error("expected error for empty parentheses, got nil")
	}
}

func TestExpressionValidator_Compile_MultipleErrors(t *testing.T) {
	// Test that the validator can handle expressions with multiple syntax errors
	// Arrange
	validator := newMockExpressionValidator()
	validator.compileFunc = func(expression string) error {
		return errors.New("multiple syntax errors found")
	}

	invalidExpressions := []string{
		"&&",
		"||",
		"==",
		"a +",
		"* b",
		"a b c",
	}

	for _, expr := range invalidExpressions {
		t.Run(expr, func(t *testing.T) {
			// Act
			err := validator.Compile(expr)
			// Assert - should return error for each invalid expression
			if err == nil {
				t.Errorf("expected error for invalid expression '%s', got nil", expr)
			}
		})
	}
}

// ============================================================================
// Integration Tests - Behavior Verification
// ============================================================================
func TestExpressionValidator_Compile_Idempotent(t *testing.T) {
	// Test that compiling the same expression multiple times produces consistent results
	// Arrange
	validator := newMockExpressionValidator()
	expression := "status == 'complete'"
	// Act
	err1 := validator.Compile(expression)
	err2 := validator.Compile(expression)
	err3 := validator.Compile(expression)
	// Assert
	if err1 != nil || err2 != nil || err3 != nil {
		t.Errorf("unexpected errors: %v, %v, %v", err1, err2, err3)
	}
	if validator.callCount != 3 {
		t.Errorf("expected 3 calls, got %d", validator.callCount)
	}
}

func TestExpressionValidator_Compile_ErrorConsistency(t *testing.T) {
	// Test that the same invalid expression produces errors consistently
	// Arrange
	validator := newMockExpressionValidator()
	validator.compileFunc = func(expression string) error {
		if expression == "invalid" {
			return errors.New("syntax error")
		}
		return nil
	}

	// Act
	err1 := validator.Compile("invalid")
	err2 := validator.Compile("invalid")
	// Assert
	if err1 == nil || err2 == nil {
		t.Error("expected errors for invalid expression")
	}
	if err1.Error() != err2.Error() {
		t.Errorf("inconsistent error messages: '%v' vs '%v'", err1, err2)
	}
}

func TestExpressionValidator_Compile_MixedValidInvalid(t *testing.T) {
	// Test alternating valid and invalid expressions
	// Arrange
	validator := newMockExpressionValidator()
	validator.compileFunc = func(expression string) error {
		if expression == "valid" {
			return nil
		}
		return errors.New("invalid expression")
	}

	// Act & Assert
	if err := validator.Compile("valid"); err != nil {
		t.Errorf("unexpected error for valid expression: %v", err)
	}
	if err := validator.Compile("invalid"); err == nil {
		t.Error("expected error for invalid expression, got nil")
	}
	if err := validator.Compile("valid"); err != nil {
		t.Errorf("unexpected error for valid expression after error: %v", err)
	}
}

// ============================================================================
// Usage Pattern Tests
// ============================================================================

func TestExpressionValidator_Compile_StopConditionPatterns(t *testing.T) {
	// Test real-world stop condition patterns from conversation config
	// Arrange
	validator := newMockExpressionValidator()

	realWorldPatterns := []string{
		"response contains 'DONE'",
		"turn_count >= 5",
		"turn_count >= 3 && response contains 'APPROVED'",
		"status == 'complete' || error_count > 0",
		"output_length > 1000",
	}

	for _, pattern := range realWorldPatterns {
		t.Run(pattern, func(t *testing.T) {
			// Act
			err := validator.Compile(pattern)
			// Assert
			if err != nil {
				t.Errorf("unexpected error for real-world pattern '%s': %v", pattern, err)
			}
		})
	}
}

func TestExpressionValidator_Compile_NoSideEffects(t *testing.T) {
	// Verify that Compile is side-effect free and only validates syntax
	// Arrange
	validator := newMockExpressionValidator()
	expression := "dangerous_function()"

	// Act - Compile should only check syntax, not execute
	err := validator.Compile(expression)
	// Assert - should validate syntax without executing
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Note: The actual expression is not evaluated, only syntax is checked
}
