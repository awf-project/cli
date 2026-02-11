// Package expression provides expression evaluation and validation adapters.
//
// This package implements the domain ExpressionEvaluator and ExpressionValidator
// ports using the expr-lang/expr library for safe template expression parsing
// and evaluation in workflow conditions.
//
// Key Types:
//
//	ExprEvaluator    - Evaluates boolean and integer expressions in workflow conditions
//	ExprValidator    - Compiles and validates expression syntax at workflow load time
//
// Usage Example:
//
//	evaluator := expression.NewExprEvaluator()
//	result, err := evaluator.EvaluateBool("status == 'success'", data)
//	if err != nil {
//	    // Handle evaluation error
//	}
//	// Use result for conditional branching
//
//	validator := expression.NewExprValidator()
//	if err := validator.Compile("count > 10"); err != nil {
//	    // Handle validation error
//	}
package expression
