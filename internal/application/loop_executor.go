package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

	// Check max iterations
	if len(items) > step.Loop.MaxIterations {
		return nil, fmt.Errorf("items count %d exceeds max_iterations %d",
			len(items), step.Loop.MaxIterations)
	}

	// Execute loop body for each item
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

	for i := 0; i < step.Loop.MaxIterations; i++ {
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
	if result.TotalCount >= step.Loop.MaxIterations {
		e.logger.Warn("while loop hit max iterations",
			"step", step.Name, "max", step.Loop.MaxIterations)
	}

	result.CompletedAt = time.Now()
	return result, nil
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
