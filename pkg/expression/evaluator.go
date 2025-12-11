package expression

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/expr-lang/expr"
	"github.com/vanoix/awf/pkg/interpolation"
)

// Evaluator evaluates conditional expressions against a context.
type Evaluator interface {
	// Evaluate parses and evaluates the expression against the context.
	// Returns true if the condition is met, false otherwise.
	// Returns an error for invalid expressions or evaluation failures.
	Evaluate(expr string, ctx *interpolation.Context) (bool, error)
}

// ExprEvaluator implements Evaluator using expr-lang/expr library.
type ExprEvaluator struct{}

// NewExprEvaluator creates a new ExprEvaluator.
func NewExprEvaluator() *ExprEvaluator {
	return &ExprEvaluator{}
}

// Evaluate evaluates the expression against the interpolation context.
func (e *ExprEvaluator) Evaluate(exprStr string, ctx *interpolation.Context) (bool, error) {
	if exprStr == "" {
		return false, errors.New("empty expression")
	}

	// Build the context map for the expression evaluator
	env := BuildExprContext(ctx)

	// Compile the expression with the environment
	program, err := expr.Compile(exprStr, expr.Env(env), expr.AsBool())
	if err != nil {
		return false, fmt.Errorf("compile expression: %w", err)
	}

	// Run the expression
	result, err := expr.Run(program, env)
	if err != nil {
		return false, fmt.Errorf("evaluate expression: %w", err)
	}

	// Ensure result is bool
	boolResult, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("expression did not return boolean, got %T", result)
	}

	return boolResult, nil
}

// BuildExprContext converts an interpolation.Context to a map for expression evaluation.
// The map structure allows dot-access patterns like inputs.name, states.step.exit_code.
func BuildExprContext(ctx *interpolation.Context) map[string]any {
	result := map[string]any{
		"inputs":   make(map[string]any),
		"states":   make(map[string]any),
		"env":      make(map[string]any),
		"workflow": make(map[string]any),
	}

	if ctx == nil {
		return result
	}

	// Map inputs with type coercion for string numbers and booleans
	if ctx.Inputs != nil {
		inputs := make(map[string]any, len(ctx.Inputs))
		for k, v := range ctx.Inputs {
			inputs[k] = coerceValue(v)
		}
		result["inputs"] = inputs
	}

	// Map states (step results)
	if ctx.States != nil {
		states := make(map[string]any, len(ctx.States))
		for k, v := range ctx.States {
			states[k] = map[string]any{
				"output":    v.Output,
				"stderr":    v.Stderr,
				"exit_code": v.ExitCode,
				"status":    v.Status,
			}
		}
		result["states"] = states
	}

	// Map env
	if ctx.Env != nil {
		env := make(map[string]any, len(ctx.Env))
		for k, v := range ctx.Env {
			env[k] = v
		}
		result["env"] = env
	}

	// Map workflow metadata
	workflow := map[string]any{
		"id":            ctx.Workflow.ID,
		"name":          ctx.Workflow.Name,
		"current_state": ctx.Workflow.CurrentState,
	}
	result["workflow"] = workflow

	return result
}

// coerceValue attempts to convert string values to their native types.
// - "true"/"false" -> bool
// - numeric strings -> int or float
func coerceValue(v any) any {
	str, ok := v.(string)
	if !ok {
		return v
	}

	// Try boolean
	if str == "true" {
		return true
	}
	if str == "false" {
		return false
	}

	// Try integer
	if i, err := strconv.ParseInt(str, 10, 64); err == nil {
		return int(i)
	}

	// Try float
	if f, err := strconv.ParseFloat(str, 64); err == nil {
		return f
	}

	return v
}
