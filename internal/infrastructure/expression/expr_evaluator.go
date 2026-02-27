package expression

import (
	"fmt"
	"math"
	"regexp"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/pkg/expression"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/expr-lang/expr"
)

// ExprEvaluator implements ports.ExpressionEvaluator using the expr-lang/expr library.
// Delegates to pkg/expression.ExprEvaluator for context building and evaluation logic.
type ExprEvaluator struct {
	pkgEvaluator *expression.ExprEvaluator
}

// NewExprEvaluator creates a new ExprEvaluator instance.
func NewExprEvaluator() ports.ExpressionEvaluator {
	return &ExprEvaluator{
		pkgEvaluator: expression.NewExprEvaluator(),
	}
}

// EvaluateBool evaluates a boolean expression against the provided context.
// Returns the boolean result of the expression, or an error if evaluation fails.
func (e *ExprEvaluator) EvaluateBool(exprStr string, ctx *interpolation.Context) (bool, error) {
	// Delegate to pkg/expression.ExprEvaluator
	result, err := e.pkgEvaluator.Evaluate(exprStr, ctx)
	if err != nil {
		// Check if the error is about type mismatch and enhance the message
		errMsg := err.Error()
		if regexp.MustCompile(`invalid operation.*bool\(int\)`).MatchString(errMsg) {
			return false, fmt.Errorf("expression did not return boolean, got numeric value")
		}
		return false, fmt.Errorf("evaluate bool expression: %w", err)
	}
	return result, nil
}

// EvaluateInt evaluates an arithmetic expression against the provided context.
// Returns the integer result of the expression, or an error if evaluation fails.
// The result is coerced to int type regardless of the underlying numeric type.
func (e *ExprEvaluator) EvaluateInt(exprStr string, ctx *interpolation.Context) (int, error) {
	if exprStr == "" {
		return 0, fmt.Errorf("empty expression")
	}

	// Validate syntax patterns that should be considered invalid
	// even though expr-lang may accept them
	if err := validateIntExpressionSyntax(exprStr); err != nil {
		return 0, err
	}

	// Build the context map for the expression evaluator
	env := expression.BuildExprContext(ctx)

	// Compile the expression without AsInt to get proper type info
	program, err := expr.Compile(exprStr,
		expr.Env(env),
		expr.AllowUndefinedVariables())
	if err != nil {
		return 0, fmt.Errorf("compile expression: %w", err)
	}

	// Run the expression
	result, err := expr.Run(program, env)
	if err != nil {
		return 0, fmt.Errorf("evaluate expression: %w", err)
	}

	// Coerce result to int and check for errors
	switch v := result.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		// Check for division by zero and other invalid operations
		if math.IsInf(v, 0) {
			return 0, fmt.Errorf("division by zero")
		}
		if math.IsNaN(v) {
			return 0, fmt.Errorf("invalid arithmetic operation")
		}
		return int(v), nil
	case bool:
		// Boolean expressions are not valid for integer evaluation
		return 0, fmt.Errorf("expression returned boolean, expected numeric value")
	default:
		return 0, fmt.Errorf("expression did not return numeric value, got %T", result)
	}
}

// validateIntExpressionSyntax checks for patterns that should be considered
// invalid syntax for integer expressions, even if expr-lang might accept them.
func validateIntExpressionSyntax(exprStr string) error {
	// Check for double plus (unary plus applied twice - confusing)
	matched, err := regexp.MatchString(`\+\+`, exprStr)
	if err != nil {
		return fmt.Errorf("validate expression syntax: %w", err)
	}
	if matched {
		return fmt.Errorf("compile expression: invalid operator ++")
	}

	// Check for power operator (** should not be used for integer expressions)
	matched, err = regexp.MatchString(`\*\*`, exprStr)
	if err != nil {
		return fmt.Errorf("validate expression syntax: %w", err)
	}
	if matched {
		return fmt.Errorf("compile expression: invalid operator **")
	}

	return nil
}

// Compile-time verification that ExprEvaluator implements ports.ExpressionEvaluator
var _ ports.ExpressionEvaluator = (*ExprEvaluator)(nil)
