package ports

import "github.com/vanoix/awf/pkg/interpolation"

// ExpressionEvaluator defines the contract for evaluating runtime expressions.
// This port abstracts expression evaluation to maintain domain layer purity
// by avoiding direct dependencies on external expression libraries.
//
// Unlike ExpressionValidator (which validates syntax at compile-time),
// ExpressionEvaluator handles runtime evaluation with actual data context.
type ExpressionEvaluator interface {
	// EvaluateBool evaluates a boolean expression against the provided context.
	// Returns the boolean result of the expression, or an error if evaluation fails.
	// Example: "inputs.count > 5" → true/false
	EvaluateBool(expr string, ctx *interpolation.Context) (bool, error)

	// EvaluateInt evaluates an arithmetic expression against the provided context.
	// Returns the integer result of the expression, or an error if evaluation fails.
	// The result is coerced to int type regardless of the underlying numeric type.
	// Example: "20 / 4" → 5
	EvaluateInt(expr string, ctx *interpolation.Context) (int, error)
}
