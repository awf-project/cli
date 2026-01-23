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
// Returns the next step name (if transition matched) and any error encountered.
// F048: Support transitions within loop body
type StepExecutorFunc func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error)

// ContextBuilderFunc builds an interpolation context from execution context.
type ContextBuilderFunc func(execCtx *workflow.ExecutionContext) *interpolation.Context

// loopExitState tracks loop exit state with target step.
// Used internally by ExecuteForEach and ExecuteWhile to manage transitions.
type loopExitState struct {
	shouldExit bool
	targetStep string
}

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
//
//nolint:gocognit // Complexity 40: forEach executor manages iteration over collections, loop state, break/continue jumps, parallel execution. Loop semantics require comprehensive handling.
func (e *LoopExecutor) ExecuteForEach(
	ctx context.Context,
	wf *workflow.Workflow,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
	stepExecutor StepExecutorFunc,
	buildContext ContextBuilderFunc,
) (*workflow.LoopResult, error) {
	// Validate loop body for duplicates before starting execution
	// F048 PR-67: Reject duplicate step names to prevent configuration errors
	if _, err := e.BuildBodyStepIndices(step.Loop.Body); err != nil {
		return nil, fmt.Errorf("invalid loop body: %w", err)
	}

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
	// F048 T006: Use default if MaxIterations is 0
	if maxIterations == 0 {
		if step.Loop.MaxIterationsExplicitlySet {
			return nil, fmt.Errorf("max_iterations cannot be zero")
		}
		maxIterations = workflow.DefaultMaxIterations
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
			return result, fmt.Errorf("loop cancelled: %w", ctx.Err())
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
		// F048 T004: Build step name → index map for transition support
		// Note: body already validated for duplicates at function start
		bodyStepIndices, err := e.BuildBodyStepIndices(step.Loop.Body)
		if err != nil {
			// Should never happen since we validated at function start
			// But handle defensively to prevent panic
			e.PopLoopContext(execCtx)
			return result, fmt.Errorf("build body step indices: %w", err)
		}

		var iterErr error
		exitState := loopExitState{} // F048 T006/T007: Track loop exit state
		for bodyIdx := 0; bodyIdx < len(step.Loop.Body); bodyIdx++ {
			bodyStepName := step.Loop.Body[bodyIdx]

			// F048: StepExecutorFunc now returns (nextStep, error)
			nextStep, err := stepExecutor(ctx, bodyStepName, intCtx)
			if err != nil {
				iterErr = err
				break
			}

			// F048 T005: Evaluate transition after step execution
			shouldBreak, newIdx := e.evaluateBodyTransition(nextStep, bodyStepIndices, step.Loop.Body, bodyIdx, step.Name, wf.Steps)

			// Capture step state
			if state, ok := execCtx.GetStepState(bodyStepName); ok {
				stateCopy := state
				iterResult.StepResults[bodyStepName] = &stateCopy
			}

			// F048 T005: Handle transition result
			if shouldBreak {
				// Early exit from loop body (transition to step outside loop)
				// F048 T006: Set flag to exit entire foreach loop, not just this iteration
				// F048 T007: Capture nextStep for early exit transition
				exitState.targetStep = nextStep
				exitState.shouldExit = true
				break
			}

			// F048 T006: Handle intra-body jump (only when newIdx >= 0, meaning jump detected)
			if newIdx >= 0 {
				adjustedIdx := e.handleIntraBodyJump(newIdx, bodyIdx)
				bodyIdx = adjustedIdx
			}
		}

		iterResult.Error = iterErr
		iterResult.CompletedAt = time.Now()
		result.Iterations = append(result.Iterations, iterResult)
		result.TotalCount++

		// C019: Apply rolling window memory management
		if step.Loop.MemoryConfig != nil && step.Loop.MemoryConfig.MaxRetainedIterations > 0 {
			maxRetained := step.Loop.MemoryConfig.MaxRetainedIterations
			if len(result.Iterations) > maxRetained {
				// Prune oldest iterations, keeping only the last N
				pruneCount := len(result.Iterations) - maxRetained
				result.Iterations = result.Iterations[pruneCount:]
				result.PrunedCount += pruneCount
			}
		}

		// F048 T006: Check if we should exit the loop due to external transition
		if exitState.shouldExit {
			// F048 T007: Set nextStep in result for early exit
			result.NextStep = exitState.targetStep
			// Pop context before breaking to restore parent
			e.PopLoopContext(execCtx)
			break
		}

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
//
//nolint:gocognit // Complexity 42: while loop executor handles condition evaluation, iteration limits, break/continue, loop state tracking. Inherent to while loop semantics.
func (e *LoopExecutor) ExecuteWhile(
	ctx context.Context,
	wf *workflow.Workflow,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
	stepExecutor StepExecutorFunc,
	buildContext ContextBuilderFunc,
) (*workflow.LoopResult, error) {
	// Validate loop body for duplicates before starting execution
	// F048 PR-67: Reject duplicate step names to prevent configuration errors
	if _, err := e.BuildBodyStepIndices(step.Loop.Body); err != nil {
		return nil, fmt.Errorf("invalid loop body: %w", err)
	}

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
	// F048 T006: Use default if MaxIterations is 0
	if maxIterations == 0 {
		if step.Loop.MaxIterationsExplicitlySet {
			return nil, fmt.Errorf("max_iterations cannot be zero")
		}
		maxIterations = workflow.DefaultMaxIterations
	}

	for i := 0; i < maxIterations; i++ {
		// Check context cancellation before starting iteration
		if ctx.Err() != nil {
			return result, fmt.Errorf("loop cancelled: %w", ctx.Err())
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
		// F048 T004: Build step name → index map for transition support
		// Note: body already validated for duplicates at function start
		bodyStepIndices, err := e.BuildBodyStepIndices(step.Loop.Body)
		if err != nil {
			// Should never happen since we validated at function start
			// But handle defensively to prevent panic
			e.PopLoopContext(execCtx)
			return result, fmt.Errorf("build body step indices: %w", err)
		}

		var iterErr error
		exitState := loopExitState{} // F048 T006/T007: Track loop exit state
		for bodyIdx := 0; bodyIdx < len(step.Loop.Body); bodyIdx++ {
			bodyStepName := step.Loop.Body[bodyIdx]

			// F048: StepExecutorFunc now returns (nextStep, error)
			nextStep, err := stepExecutor(ctx, bodyStepName, intCtx)
			if err != nil {
				iterErr = err
				break
			}

			// F048 T005: Evaluate transition after step execution
			shouldBreak, newIdx := e.evaluateBodyTransition(nextStep, bodyStepIndices, step.Loop.Body, bodyIdx, step.Name, wf.Steps)

			// Capture step state
			if state, ok := execCtx.GetStepState(bodyStepName); ok {
				stateCopy := state
				iterResult.StepResults[bodyStepName] = &stateCopy
			}

			// F048 T005: Handle transition result
			if shouldBreak {
				// Early exit from loop body (transition to step outside loop)
				// F048 T006: Set flag to exit entire while loop, not just this iteration
				// F048 T007: Capture nextStep for early exit transition
				exitState.targetStep = nextStep
				exitState.shouldExit = true
				break
			}

			// F048 T006: Handle intra-body jump (only when newIdx >= 0, meaning jump detected)
			if newIdx >= 0 {
				adjustedIdx := e.handleIntraBodyJump(newIdx, bodyIdx)
				bodyIdx = adjustedIdx
			}
		}

		iterResult.Error = iterErr
		iterResult.CompletedAt = time.Now()
		result.Iterations = append(result.Iterations, iterResult)
		result.TotalCount++

		// C019: Apply rolling window memory management
		if step.Loop.MemoryConfig != nil && step.Loop.MemoryConfig.MaxRetainedIterations > 0 {
			maxRetained := step.Loop.MemoryConfig.MaxRetainedIterations
			if len(result.Iterations) > maxRetained {
				// Prune oldest iterations, keeping only the last N
				pruneCount := len(result.Iterations) - maxRetained
				result.Iterations = result.Iterations[pruneCount:]
				result.PrunedCount += pruneCount
			}
		}

		// F048 T006: Check if we should exit the loop due to external transition
		if exitState.shouldExit {
			// F048 T007: Set nextStep in result for early exit
			result.NextStep = exitState.targetStep
			// Pop context before breaking to restore parent
			e.PopLoopContext(execCtx)
			break
		}

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
func (e *LoopExecutor) ResolveMaxIterations(maxIterExpr string, ctx *interpolation.Context) (int, error) {
	// Step 1: Validate non-empty expression
	if maxIterExpr == "" {
		return 0, fmt.Errorf("max_iterations expression is empty")
	}

	// Step 2: Resolve {{var}} placeholders using template resolver
	resolved, err := e.resolver.Resolve(maxIterExpr, ctx)
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

// BuildBodyStepIndices creates a map from step name to index in the body slice.
// Used by F048 to support transitions within loop bodies by enabling jump-to-index logic.
// Returns a map where keys are step names and values are their positions in the body array,
// or an error if duplicate step names are detected (to prevent silent configuration errors).
// F048 T004: Body Step Index Mapping
//
// INTERNAL: This method is exported for testing purposes only.
func (e *LoopExecutor) BuildBodyStepIndices(body []string) (map[string]int, error) {
	indices := make(map[string]int)
	seen := make(map[string]int)
	for i, stepName := range body {
		if prevIdx, exists := seen[stepName]; exists {
			return nil, fmt.Errorf("duplicate step '%s' in loop body at indices %d and %d", stepName, prevIdx, i)
		}
		indices[stepName] = i
		seen[stepName] = i
	}
	return indices, nil
}

// evaluateBodyTransition evaluates the transition result after a loop body step execution.
// Returns (shouldBreak, newIdx) where:
//   - shouldBreak: true if transition targets step outside loop body (early exit)
//   - newIdx: target step index if transition targets step within body, -1 if no transition or sequential
//
// F048 T005: Transition Evaluation in Loop Body
func (e *LoopExecutor) evaluateBodyTransition(
	nextStep string,
	bodyStepIndices map[string]int,
	body []string,
	currentIdx int,
	loopStepName string,
	workflowSteps map[string]*workflow.Step,
) (shouldBreak bool, newIdx int) {
	// Case 1: No transition - sequential execution (most common case)
	if nextStep == "" {
		return false, -1
	}

	// Case 2: Intra-body jump - transition to step within loop body
	if targetIdx, exists := bodyStepIndices[nextStep]; exists {
		e.logger.Info("loop intra-body transition",
			"target", nextStep,
			"target_index", targetIdx)
		return false, targetIdx
	}

	// Case 3: Retry pattern - transition to loop step itself (ADR-004)
	if nextStep == loopStepName {
		e.logger.Info("loop retry pattern detected",
			"loop", loopStepName,
			"current_index", currentIdx)
		return false, -1
	}

	// Case 4: Invalid target - target doesn't exist in workflow (ADR-005)
	if _, exists := workflowSteps[nextStep]; !exists {
		e.logger.Warn("transition target not found in workflow, continuing sequential",
			"target", nextStep,
			"current_index", currentIdx)
		return false, -1
	}

	// Case 5: Early exit - target exists in workflow but not in body (ADR-003)
	e.logger.Info("loop early exit",
		"target", nextStep,
		"current_index", currentIdx,
		"body_size", len(body))
	return true, -1
}

// handleIntraBodyJump adjusts the loop body index to jump to a target step within the loop body.
// Returns the new index value that should be assigned to bodyIdx in the loop.
// The returned value accounts for the loop's increment by subtracting 1.
//
// Parameters:
//   - newIdx: target step index within body (must be >= 0, caller ensures this)
//   - currentIdx: current position in the loop body
//
// Returns:
//   - adjustedIdx: the index to assign to bodyIdx (can be -1 for jump to index 0)
//
// F048 T006: Intra-Body Jump Handling in ExecuteWhile
func (e *LoopExecutor) handleIntraBodyJump(newIdx, currentIdx int) int {
	// Intra-body jump - adjust index to compensate for loop increment
	// The for loop will increment bodyIdx after this assignment, so we subtract 1
	// to land on the target index after the increment.
	// Example: To jump to index 3, we return 2, then loop increments to 3
	// Special case: To jump to index 0, we return -1, then loop increments to 0
	adjustedIdx := newIdx - 1
	e.logger.Info("loop intra-body jump",
		"from_index", currentIdx,
		"to_index", newIdx,
		"adjusted_index", adjustedIdx)
	return adjustedIdx
}
