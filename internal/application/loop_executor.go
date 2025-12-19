package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

// StepExecutorFunc executes a step by name within the current loop context.
type StepExecutorFunc func(ctx context.Context, stepName string, intCtx *interpolation.Context) error

// ContextBuilderFunc builds an interpolation context from execution context.
type ContextBuilderFunc func(execCtx *workflow.ExecutionContext) *interpolation.Context

// LoopExecutor executes for_each and while loop constructs.
type LoopExecutor struct {
	logger    ports.Logger
	evaluator ExpressionEvaluator
	resolver  interpolation.Resolver
}

// NewLoopExecutor creates a new LoopExecutor.
func NewLoopExecutor(
	logger ports.Logger,
	evaluator ExpressionEvaluator,
	resolver interpolation.Resolver,
) *LoopExecutor {
	return &LoopExecutor{
		logger:    logger,
		evaluator: evaluator,
		resolver:  resolver,
	}
}

// PushLoopContext sets a new loop context, linking to existing as parent.
// This enables nested loops by preserving outer loop state.
// F043: Nested Loop Execution
func (e *LoopExecutor) PushLoopContext(
	execCtx *workflow.ExecutionContext,
	item any,
	index int,
	first, last bool,
	length int,
) {
	newCtx := &workflow.LoopContext{
		Item:   item,
		Index:  index,
		First:  first,
		Last:   last,
		Length: length,
		Parent: execCtx.CurrentLoop, // Link to outer loop (nil if top-level)
	}
	execCtx.CurrentLoop = newCtx
}

// PopLoopContext restores the parent loop context.
// Returns the popped context for inspection if needed.
// F043: Nested Loop Execution
func (e *LoopExecutor) PopLoopContext(execCtx *workflow.ExecutionContext) *workflow.LoopContext {
	if execCtx.CurrentLoop == nil {
		return nil
	}
	popped := execCtx.CurrentLoop
	execCtx.CurrentLoop = execCtx.CurrentLoop.Parent
	return popped
}

// ExecuteForEach iterates over items and executes body steps for each.
func (e *LoopExecutor) ExecuteForEach(
	ctx context.Context,
	wf *workflow.Workflow,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
	stepExecutor StepExecutorFunc,
	buildContext ContextBuilderFunc,
) (*workflow.LoopResult, error) {
	result := workflow.NewLoopResult()

	// Resolve items expression
	intCtx := buildContext(execCtx)
	itemsStr, err := e.resolver.Resolve(step.Loop.Items, intCtx)
	if err != nil {
		return nil, fmt.Errorf("resolve items: %w", err)
	}

	// Parse items
	items, err := e.ParseItems(itemsStr)
	if err != nil {
		return nil, fmt.Errorf("parse items: %w", err)
	}

	// Determine max iterations: use dynamic expression if set, otherwise static value
	// F037: Dynamic Variable Interpolation in Loops
	maxIterations := step.Loop.MaxIterations
	if step.Loop.IsMaxIterationsDynamic() {
		maxIterations, err = e.ResolveMaxIterations(step.Loop.MaxIterationsExpr, intCtx)
		if err != nil {
			return nil, fmt.Errorf("resolve max_iterations: %w", err)
		}
	}

	// F037: max_iterations limits execution to first N items (not a validation)
	// Slice items to respect max_iterations limit
	if len(items) > maxIterations {
		items = items[:maxIterations]
	}

	// Execute loop body for each item (limited by max_iterations)
	for i, item := range items {
		// Check context cancellation before starting iteration
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		iterResult := workflow.IterationResult{
			Index:       i,
			Item:        item,
			StepResults: make(map[string]*workflow.StepState),
			StartedAt:   time.Now(),
		}

		// Set loop context on execCtx so executeStep can access it
		// Use push to enable nested loops (preserves parent context)
		e.PushLoopContext(execCtx, item, i, i == 0, i == len(items)-1, len(items))

		// Set loop context for this iteration (for callbacks)
		intCtx = buildContext(execCtx)
		intCtx.Loop = &interpolation.LoopData{
			Item:   item,
			Index:  i,
			First:  i == 0,
			Last:   i == len(items)-1,
			Length: len(items),
		}

		// Execute body steps
		var iterErr error
		for _, bodyStepName := range step.Loop.Body {
			if err := stepExecutor(ctx, bodyStepName, intCtx); err != nil {
				iterErr = err
				break
			}
			// Capture step state
			if state, ok := execCtx.GetStepState(bodyStepName); ok {
				stateCopy := state
				iterResult.StepResults[bodyStepName] = &stateCopy
			}
		}

		iterResult.Error = iterErr
		iterResult.CompletedAt = time.Now()
		result.Iterations = append(result.Iterations, iterResult)
		result.TotalCount++

		// Check break condition after iteration completes
		if step.Loop.BreakCondition != "" && e.evaluator != nil {
			intCtx = buildContext(execCtx)
			shouldBreak, err := e.evaluator.Evaluate(step.Loop.BreakCondition, intCtx)
			if err != nil {
				e.logger.Warn("break condition evaluation failed", "error", err)
			} else if shouldBreak {
				result.BrokeAt = i
				// Pop context before breaking to restore parent
				e.PopLoopContext(execCtx)
				break
			}
		}

		// Pop loop context at end of iteration (restores parent for nested loops)
		e.PopLoopContext(execCtx)

		// Check for iteration error
		if iterErr != nil {
			return result, iterErr
		}
	}

	result.CompletedAt = time.Now()
	return result, nil
}

// ExecuteWhile repeats body steps while condition is true.
func (e *LoopExecutor) ExecuteWhile(
	ctx context.Context,
	wf *workflow.Workflow,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
	stepExecutor StepExecutorFunc,
	buildContext ContextBuilderFunc,
) (*workflow.LoopResult, error) {
	result := workflow.NewLoopResult()

	// Determine max iterations: use dynamic expression if set, otherwise static value
	// F037: Dynamic Variable Interpolation in Loops
	intCtx := buildContext(execCtx)
	maxIterations := step.Loop.MaxIterations
	if step.Loop.IsMaxIterationsDynamic() {
		var err error
		maxIterations, err = e.ResolveMaxIterations(step.Loop.MaxIterationsExpr, intCtx)
		if err != nil {
			return nil, fmt.Errorf("resolve max_iterations: %w", err)
		}
	}

	for i := 0; i < maxIterations; i++ {
		// Check context cancellation before starting iteration
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		// Set loop context on execCtx so executeStep can access it
		// Use push to enable nested loops (preserves parent context)
		// While loops don't have an item, so we pass nil
		e.PushLoopContext(execCtx, nil, i, i == 0, false, -1)

		// Evaluate while condition
		intCtx := buildContext(execCtx)

		shouldContinue, err := e.evaluator.Evaluate(step.Loop.Condition, intCtx)
		if err != nil {
			// Pop context before returning on error
			e.PopLoopContext(execCtx)
			return result, fmt.Errorf("evaluate condition: %w", err)
		}

		if !shouldContinue {
			// Pop context before breaking when condition is false
			e.PopLoopContext(execCtx)
			break
		}

		iterResult := workflow.IterationResult{
			Index:       i,
			StepResults: make(map[string]*workflow.StepState),
			StartedAt:   time.Now(),
		}

		// Execute body steps
		var iterErr error
		for _, bodyStepName := range step.Loop.Body {
			if err := stepExecutor(ctx, bodyStepName, intCtx); err != nil {
				iterErr = err
				break
			}
			// Capture step state
			if state, ok := execCtx.GetStepState(bodyStepName); ok {
				stateCopy := state
				iterResult.StepResults[bodyStepName] = &stateCopy
			}
		}

		iterResult.Error = iterErr
		iterResult.CompletedAt = time.Now()
		result.Iterations = append(result.Iterations, iterResult)
		result.TotalCount++

		// Check break condition after iteration completes
		if step.Loop.BreakCondition != "" && e.evaluator != nil {
			intCtx = buildContext(execCtx)
			shouldBreak, err := e.evaluator.Evaluate(step.Loop.BreakCondition, intCtx)
			if err != nil {
				e.logger.Warn("break condition evaluation failed", "error", err)
			} else if shouldBreak {
				result.BrokeAt = i
				// Pop context before breaking to restore parent
				e.PopLoopContext(execCtx)
				break
			}
		}

		// Pop loop context at end of iteration (restores parent for nested loops)
		e.PopLoopContext(execCtx)

		// Check for iteration error
		if iterErr != nil {
			return result, iterErr
		}
	}

	// Check if we hit max iterations without condition becoming false
	if result.TotalCount >= maxIterations {
		e.logger.Warn("while loop hit max iterations",
			"step", step.Name, "max", maxIterations)
	}

	result.CompletedAt = time.Now()
	return result, nil
}

// ResolveMaxIterations resolves a dynamic max_iterations expression to an integer.
// It performs template interpolation first ({{var}} substitution), then evaluates
// any arithmetic expressions, and finally validates the result is within bounds.
// Returns error if the expression cannot be resolved, is not a valid integer,
// or the value is outside the allowed range (1-10000).
// F037: Dynamic Variable Interpolation in Loops
func (e *LoopExecutor) ResolveMaxIterations(expr string, ctx *interpolation.Context) (int, error) {
	// Step 1: Validate non-empty expression
	if expr == "" {
		return 0, fmt.Errorf("max_iterations expression is empty")
	}

	// Step 2: Resolve {{var}} placeholders using template resolver
	resolved, err := e.resolver.Resolve(expr, ctx)
	if err != nil {
		return 0, fmt.Errorf("resolve max_iterations expression: %w", err)
	}

	// Step 3: Trim whitespace (command output often has trailing newlines)
	resolved = strings.TrimSpace(resolved)

	// Step 4: Try to parse as integer directly first
	value, err := e.parseMaxIterationsValue(resolved)
	if err != nil {
		return 0, err
	}

	// Step 5: Validate bounds (1 ≤ n ≤ MaxAllowedIterations)
	if value < 1 {
		return 0, fmt.Errorf("max_iterations value %d must be at least 1", value)
	}
	if value > workflow.MaxAllowedIterations {
		return 0, fmt.Errorf("max_iterations value %d exceeds maximum allowed limit of %d",
			value, workflow.MaxAllowedIterations)
	}

	return value, nil
}

// parseMaxIterationsValue attempts to parse the resolved string as an integer.
// If direct parsing fails, it evaluates it as an arithmetic expression.
func (e *LoopExecutor) parseMaxIterationsValue(resolved string) (int, error) {
	// Try direct integer parse first (most common case)
	if i, err := strconv.Atoi(resolved); err == nil {
		return i, nil
	}

	// Check for arithmetic operators - if present, evaluate as expression
	if strings.ContainsAny(resolved, "+-*/") {
		return e.evaluateArithmeticExpression(resolved)
	}

	// Not a valid integer and no arithmetic operators
	return 0, fmt.Errorf("max_iterations value %q is invalid: must be an integer", resolved)
}

// evaluateArithmeticExpression evaluates a simple arithmetic expression
// using the expr-lang/expr library.
func (e *LoopExecutor) evaluateArithmeticExpression(exprStr string) (int, error) {
	// Use expr-lang/expr for arithmetic evaluation
	program, err := expr.Compile(exprStr)
	if err != nil {
		return 0, fmt.Errorf("max_iterations expression %q is invalid: %w", exprStr, err)
	}

	result, err := expr.Run(program, nil)
	if err != nil {
		return 0, fmt.Errorf("max_iterations expression %q evaluation failed: %w", exprStr, err)
	}

	// Convert result to int
	switch v := result.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		// Check if it's a whole number
		if v != float64(int(v)) {
			return 0, fmt.Errorf("max_iterations value %q is invalid: must be an integer, got %v", exprStr, v)
		}
		return int(v), nil
	default:
		return 0, fmt.Errorf("max_iterations expression %q returned unexpected type %T", exprStr, result)
	}
}

// ParseItems converts items string to slice.
// Used by ExecuteForEach to parse the items expression result.
// Supports JSON arrays and comma-separated values.
func (e *LoopExecutor) ParseItems(itemsStr string) ([]any, error) {
	// Try JSON array first
	var items []any
	if err := json.Unmarshal([]byte(itemsStr), &items); err == nil {
		return items, nil
	}

	// Fallback: comma-separated strings
	parts := strings.Split(itemsStr, ",")
	items = make([]any, len(parts))
	for i, p := range parts {
		items[i] = strings.TrimSpace(p)
	}
	return items, nil
}
