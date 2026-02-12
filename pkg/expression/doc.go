// Package expression provides boolean expression evaluation for workflow conditions.
//
// This package implements the public API for evaluating conditional expressions
// in workflow transitions, loop conditions, and agent stop conditions. It uses
// the expr-lang/expr library for expression parsing and evaluation.
//
// Key features:
//   - Boolean expression evaluation against interpolation context
//   - Type coercion for string numbers and booleans
//   - Helper functions for string operations (has_prefix, has_suffix, contains)
//   - Graceful handling of missing keys (evaluates to false, not error)
//
// # Core Components
//
// ## Evaluator Interface (evaluator.go)
//
// Evaluator defines the contract for expression evaluation:
//   - Evaluate: Parse and evaluate expression, return bool result
//   - ExprEvaluator: Implementation using expr-lang/expr library
//
// ## Expression Context (evaluator.go)
//
// BuildExprContext converts interpolation.Context to expr-compatible map:
//   - inputs: Map of workflow input parameters (with type coercion)
//   - states: Map of step results with PascalCase properties (Output, Stderr, ExitCode, Status, Response, TokensUsed)
//   - workflow: Workflow metadata with PascalCase properties (ID, Name, CurrentState, Duration)
//   - env: Environment variables
//   - loop: Loop context (Index, Index1, Item, Length, First, Last, Parent)
//   - error: Error context (Message, State, ExitCode, Type)
//   - context: Runtime context (WorkingDir, User, Hostname)
//
// ## Type Coercion (evaluator.go)
//
// Automatic type conversion for input values:
//   - "true"/"false" -> bool (case-insensitive)
//   - Numeric strings -> int64 or float64
//   - Preserves native types (don't convert numbers to strings)
//
// ## Expression Preprocessing (evaluator.go)
//
// Transform function-style syntax to expr operators:
//   - contains(haystack, needle) -> haystack contains needle
//   - has_prefix(a, b) -> has_prefix(a, b) (kept as function)
//   - has_suffix(a, b) -> has_suffix(a, b) (kept as function)
//
// # Expression Syntax
//
// Expressions use expr-lang syntax with AWF context namespaces:
//
// ## Comparison Operators
//
//	inputs.count > 10
//	inputs.count == 5
//	inputs.enabled == true
//	states.step.ExitCode == 0
//	states.step.Status == "completed"
//
// ## Logical Operators
//
//	inputs.debug == true && inputs.verbose == true
//	inputs.mode == "dev" || inputs.mode == "test"
//	!(inputs.skip == true)
//
// ## String Operations
//
//	states.step.Output contains "success"
//	has_prefix(states.step.Output, "ERROR:")
//	has_suffix(inputs.file, ".yaml")
//
// ## Numeric Operations
//
//	inputs.count >= 1 && inputs.count <= 10
//	states.step.ExitCode != 0
//	workflow.Duration contains "5m"
//
// ## Missing Keys Behavior
//
// Missing keys evaluate to false without error:
//
//	inputs.nonexistent == true          # false (input not provided)
//	env.MISSING_VAR == "value"          # false (env var not set)
//	states.not_run.Output == "x"        # false (step not executed yet)
//
// # Usage Examples
//
// ## Basic Evaluation
//
// Evaluate simple boolean expression:
//
//	evaluator := expression.NewExprEvaluator()
//	ctx := interpolation.NewContext()
//	ctx.Inputs["count"] = "5"
//
//	result, err := evaluator.Evaluate("inputs.count > 3", ctx)
//	// result: true
//
// ## State Transitions
//
// Conditional workflow transitions based on step results:
//
//	ctx.States["check_status"] = interpolation.StepStateData{
//	    ExitCode: 0,
//	    Output:   "OK",
//	    Status:   "completed",
//	}
//
//	evaluator.Evaluate("states.check_status.ExitCode == 0", ctx)
//	// result: true
//
//	evaluator.Evaluate("states.check_status.Output contains 'OK'", ctx)
//	// result: true
//
// ## Type Coercion
//
// Automatic type conversion for string inputs:
//
//	ctx.Inputs["count"] = "42"        // string
//	ctx.Inputs["enabled"] = "true"    // string
//
//	evaluator.Evaluate("inputs.count > 40", ctx)
//	// result: true (string "42" coerced to int)
//
//	evaluator.Evaluate("inputs.enabled == true", ctx)
//	// result: true (string "true" coerced to bool)
//
// ## Loop Conditions
//
// Evaluate loop continuation conditions:
//
//	ctx.Loop = &interpolation.LoopData{
//	    Index:  5,
//	    Item:   "test",
//	    Length: 10,
//	}
//
//	evaluator.Evaluate("loop.Index < loop.Length", ctx)
//	// result: true
//
// ## Agent Stop Conditions
//
// Check if agent conversation should stop:
//
//	ctx.States["agent_step"] = interpolation.StepStateData{
//	    Output:   "Task completed. DONE",
//	    Response: map[string]any{"status": "success"},
//	}
//
//	evaluator.Evaluate("states.agent_step.Output contains 'DONE'", ctx)
//	// result: true
//
// ## Nested Loops (F043)
//
// Access parent loop context:
//
//	ctx.Loop = &interpolation.LoopData{
//	    Index: 2,
//	    Item:  "child",
//	    Parent: &interpolation.LoopData{
//	        Index: 1,
//	        Item:  "parent",
//	    },
//	}
//
//	evaluator.Evaluate("loop.Parent.Index == 1", ctx)
//	// result: true
//
// ## Error Context Conditions
//
// Conditional error handling:
//
//	ctx.Error = &interpolation.ErrorData{
//	    Message:  "Connection timeout",
//	    State:    "api_call",
//	    ExitCode: 7,
//	    Type:     "network",
//	}
//
//	evaluator.Evaluate("error.ExitCode == 7", ctx)
//	// result: true
//
//	evaluator.Evaluate("has_prefix(error.Message, 'Connection')", ctx)
//	// result: true
//
// # Workflow Integration
//
// ## Conditional Transitions (workflow transitions field)
//
//	transitions:
//	  - condition: "{{states.check.ExitCode}} == 0"
//	    target: "success"
//	  - condition: "{{states.check.ExitCode}} == 404"
//	    target: "not_found"
//	  - condition: "true"  # default fallback
//	    target: "error"
//
// ## Loop Conditions (while loops)
//
//	loop:
//	  type: while
//	  condition: "{{loop.Index}} < 10"
//	  body:
//	    - step_name
//
// ## Agent Stop Conditions (conversation mode)
//
//	agent:
//	  mode: conversation
//	  conversation:
//	    stop_conditions:
//	      - "{{states.agent.Output}} contains 'DONE'"
//	      - "{{loop.Index}} >= 10"
//
// # Error Handling
//
// ## Compilation Errors
//
// Invalid syntax returns compile error:
//
//	evaluator.Evaluate("inputs.count >>>", ctx)
//	// error: "compile expression: syntax error"
//
// ## Type Errors
//
// Non-boolean result returns error:
//
//	evaluator.Evaluate("inputs.count + 5", ctx)
//	// error: "expression did not return boolean, got int64"
//
// ## Graceful Missing Key Handling
//
// Missing keys evaluate to false (not error):
//
//	evaluator.Evaluate("inputs.missing == 'value'", ctx)
//	// result: false, error: nil
//
//	evaluator.Evaluate("states.not_run.Output == 'x'", ctx)
//	// result: false, error: nil
//
// # Helper Functions
//
// Built-in string helper functions:
//
//	has_prefix(str, prefix)    # strings.HasPrefix
//	has_suffix(str, suffix)    # strings.HasSuffix
//	contains(haystack, needle) # preprocessed to "haystack contains needle"
//
// # Property Name Casing (F050)
//
// PascalCase properties (uppercase first letter):
//   - states.step.Output, states.step.Stderr, states.step.ExitCode, states.step.Status, states.step.Response, states.step.TokensUsed
//   - workflow.ID, workflow.Name, workflow.CurrentState, workflow.Duration
//   - loop.Index, loop.Index1, loop.Item, loop.First, loop.Last, loop.Length, loop.Parent
//   - error.Message, error.State, error.ExitCode, error.Type
//   - context.WorkingDir, context.User, context.Hostname
//
// # Design Principles
//
// ## Public API Surface
//
// This is a public package (pkg/) for external consumers:
//   - Stable API with semantic versioning
//   - Clean Evaluator interface for easy mocking
//   - Depends on pkg/interpolation (public) for Context types
//
// ## Type Safety
//
// Strong typing with automatic coercion:
//   - Input strings coerced to native types (int, float, bool)
//   - PascalCase property names enforce type structure
//   - Boolean result type enforced at compile time
//
// ## Graceful Degradation
//
// Missing keys don't fail expressions:
//   - Allows forward references to steps not yet executed
//   - Simplifies conditional logic (no existence checks needed)
//   - expr library's AllowUndefinedVariables option enabled
//
// ## Expression Power
//
// expr-lang provides rich expression capabilities:
//   - Arithmetic: +, -, *, /, %
//   - Comparison: ==, !=, <, >, <=, >=
//   - Logical: &&, ||, !
//   - String operators: contains, in
//   - Array/map access: arr[0], map["key"]
//
// # Related Documentation
//
// See also:
//   - pkg/interpolation: Template variable resolution (provides Context types)
//   - internal/domain/workflow: Workflow transition and condition types
//   - internal/infrastructure/expression: Infrastructure adapter implementing expression compilation port
//   - docs/conditional-transitions.md: Workflow transition syntax
//   - External: https://expr-lang.org/ - expr-lang documentation
package expression
