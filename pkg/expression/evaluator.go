package expression

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/awf-project/awf/pkg/interpolation"
	"github.com/expr-lang/expr"
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

func NewExprEvaluator() *ExprEvaluator {
	return &ExprEvaluator{}
}

func (e *ExprEvaluator) Evaluate(exprStr string, ctx *interpolation.Context) (bool, error) {
	if exprStr == "" {
		return false, errors.New("empty expression")
	}

	exprStr = preprocessExpression(exprStr)
	env := BuildExprContext(ctx)

	env["has_prefix"] = strings.HasPrefix
	env["has_suffix"] = strings.HasSuffix

	program, err := expr.Compile(exprStr,
		expr.Env(env),
		expr.AsBool(),
		expr.AllowUndefinedVariables())
	if err != nil {
		return false, fmt.Errorf("compile expression: %w", err)
	}

	result, err := expr.Run(program, env)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "invalid operation: <nil>") &&
			(strings.Contains(errMsg, "inputs.") || strings.Contains(errMsg, "env.")) {
			return false, nil
		}
		if strings.Contains(errMsg, "cannot fetch") && strings.Contains(errMsg, "states.") {
			return false, nil
		}
		return false, fmt.Errorf("evaluate expression: %w", err)
	}

	boolResult, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("expression did not return boolean, got %T", result)
	}

	return boolResult, nil
}

// preprocessExpression transforms function-style calls to expr operators
// Examples:
//
//	contains(haystack, needle) -> haystack contains needle
//	has_prefix(a, b) -> has_prefix(a, b)  [kept as function]
//	has_suffix(a, b) -> has_suffix(a, b)  [kept as function]
func preprocessExpression(exprStr string) string {
	containsRe := regexp.MustCompile(`contains\(([^,]+),\s*([^)]+)\)`)
	exprStr = containsRe.ReplaceAllString(exprStr, `$1 contains $2`)
	return exprStr
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

	if ctx.Inputs != nil {
		inputs := make(map[string]any, len(ctx.Inputs))
		for k, v := range ctx.Inputs {
			inputs[k] = coerceValue(v)
		}
		result["inputs"] = inputs
	}

	if ctx.States != nil {
		states := make(map[string]any, len(ctx.States))
		for k, v := range ctx.States {
			states[k] = map[string]any{
				"Output":     v.Output,
				"Stderr":     v.Stderr,
				"ExitCode":   v.ExitCode,
				"Status":     v.Status,
				"Response":   v.Response,
				"TokensUsed": v.TokensUsed,
			}
		}
		result["states"] = states
	}

	if ctx.Env != nil {
		env := make(map[string]any, len(ctx.Env))
		for k, v := range ctx.Env {
			env[k] = v
		}
		result["env"] = env
	}

	workflow := map[string]any{
		"ID":           ctx.Workflow.ID,
		"Name":         ctx.Workflow.Name,
		"CurrentState": ctx.Workflow.CurrentState,
		"Duration":     ctx.Workflow.Duration(),
	}
	result["workflow"] = workflow

	if ctx.Loop != nil {
		loop := buildLoopContext(ctx.Loop)
		result["loop"] = loop
	}

	if ctx.Error != nil {
		errorData := buildErrorContext(ctx.Error)
		result["error"] = errorData
	}

	contextData := buildSystemContext(&ctx.Context)
	result["context"] = contextData

	return result
}

// buildLoopContext creates the loop namespace map with PascalCase keys.
// Recursively handles nested loops via Parent field.
func buildLoopContext(loop *interpolation.LoopData) map[string]any {
	ctx := map[string]any{
		"Index":  loop.Index,
		"Index1": loop.Index1(),
		"Item":   loop.Item,
		"Length": loop.Length,
		"First":  loop.First,
		"Last":   loop.Last,
		"Parent": nil,
	}

	if loop.Parent != nil {
		ctx["Parent"] = buildLoopContext(loop.Parent)
	}

	return ctx
}

// buildErrorContext creates the error namespace map with PascalCase keys.
// Stub implementation - returns zero values.
func buildErrorContext(err *interpolation.ErrorData) map[string]any {
	if err == nil {
		return map[string]any{
			"Message":  "",
			"State":    "",
			"ExitCode": 0,
			"Type":     "",
		}
	}
	return map[string]any{
		"Message":  err.Message,
		"State":    err.State,
		"ExitCode": err.ExitCode,
		"Type":     err.Type,
	}
}

// buildSystemContext creates the context namespace map with PascalCase keys.
// Returns empty strings for nil context, ensuring context.* is always available.
func buildSystemContext(ctx *interpolation.ContextData) map[string]any {
	if ctx == nil {
		return map[string]any{
			"WorkingDir": "",
			"User":       "",
			"Hostname":   "",
		}
	}
	return map[string]any{
		"WorkingDir": ctx.WorkingDir,
		"User":       ctx.User,
		"Hostname":   ctx.Hostname,
	}
}

// coerceValue attempts to convert string values to their native types:
// "true"/"false" -> bool, numeric strings -> int or float.
func coerceValue(v any) any {
	str, ok := v.(string)
	if !ok {
		return v
	}

	if str != "" {
		if str == "true" || str == "True" || str == "TRUE" {
			return true
		}
		if str == "false" || str == "False" || str == "FALSE" {
			return false
		}
	}

	if i, err := strconv.ParseInt(str, 10, 64); err == nil {
		return i
	}

	if f, err := strconv.ParseFloat(str, 64); err == nil {
		return f
	}

	return v
}
