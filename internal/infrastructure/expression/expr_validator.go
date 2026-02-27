package expression

import (
	"fmt"
	"strings"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/expr-lang/expr"
)

// ExprValidator implements ports.ExpressionValidator using the expr-lang/expr library.
// Provides expression syntax validation by wrapping expr.Compile().
type ExprValidator struct{}

// NewExprValidator creates a new ExprValidator instance.
func NewExprValidator() ports.ExpressionValidator {
	return &ExprValidator{}
}

// Compile validates the syntax of an expression string.
// Returns nil if the expression is syntactically valid, error otherwise.
// Empty or whitespace-only expressions are considered valid (no stop condition).
func (v *ExprValidator) Compile(expression string) error {
	// Empty or whitespace-only expressions are valid (no early exit condition)
	if strings.TrimSpace(expression) == "" {
		return nil
	}

	_, err := expr.Compile(expression, expr.AllowUndefinedVariables(), expr.DisableAllBuiltins())
	if err != nil {
		return fmt.Errorf("expression compilation failed: %w", err)
	}
	return nil
}

// Compile-time verification that ExprValidator implements ports.ExpressionValidator
var _ ports.ExpressionValidator = (*ExprValidator)(nil)
