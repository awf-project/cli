package application_test

import (
	"context"

	"github.com/vanoix/awf/pkg/interpolation"
)

// =============================================================================
// Shared Mock Implementations for Loop Executor Tests
// =============================================================================
//
// This file contains mock implementations used across multiple loop executor
// test files. Extracted during C014 test file splitting to prevent duplication
// and ensure consistency.
//
// Mocks:
// - mockExpressionEvaluator: Configurable boolean expression evaluator
// - configurableMockResolver: Configurable string template resolver
// - stepExecutorRecorder: Records step executions for verification
// - counterExpressionEvaluator: Returns true for first N calls, then false

// mockExpressionEvaluator implements ExpressionEvaluator for testing
type mockExpressionEvaluator struct {
	results map[string]bool
	calls   []string
	err     error
}

func newMockExpressionEvaluator() *mockExpressionEvaluator {
	return &mockExpressionEvaluator{
		results: make(map[string]bool),
		calls:   make([]string, 0),
	}
}

func (m *mockExpressionEvaluator) Evaluate(expr string, ctx *interpolation.Context) (bool, error) {
	m.calls = append(m.calls, expr)
	if m.err != nil {
		return false, m.err
	}
	if result, ok := m.results[expr]; ok {
		return result, nil
	}
	return false, nil
}

// configurableMockResolver implements interpolation.Resolver with configurable results
type configurableMockResolver struct {
	results map[string]string
	calls   []string
	err     error
}

func newConfigurableMockResolver() *configurableMockResolver {
	return &configurableMockResolver{
		results: make(map[string]string),
		calls:   make([]string, 0),
	}
}

func (m *configurableMockResolver) Resolve(template string, ctx *interpolation.Context) (string, error) {
	m.calls = append(m.calls, template)
	if m.err != nil {
		return "", m.err
	}
	if result, ok := m.results[template]; ok {
		return result, nil
	}
	// Default: return template unchanged
	return template, nil
}

// stepExecutorRecorder records step executions for verification
// F048: Updated to support new StepExecutorFunc signature
type stepExecutorRecorder struct {
	executions  []stepExecution
	results     map[string]error
	transitions map[string]string // F048: Map of stepName -> nextStep for transition testing
}

type stepExecution struct {
	stepName string
	loopData *interpolation.LoopData
}

func newStepExecutorRecorder() *stepExecutorRecorder {
	return &stepExecutorRecorder{
		executions:  make([]stepExecution, 0),
		results:     make(map[string]error),
		transitions: make(map[string]string),
	}
}

// F048: Updated to return (nextStep, error)
func (r *stepExecutorRecorder) execute(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
	exec := stepExecution{stepName: stepName}
	if intCtx.Loop != nil {
		exec.loopData = &interpolation.LoopData{
			Item:   intCtx.Loop.Item,
			Index:  intCtx.Loop.Index,
			First:  intCtx.Loop.First,
			Last:   intCtx.Loop.Last,
			Length: intCtx.Loop.Length,
		}
	}
	r.executions = append(r.executions, exec)

	if err, ok := r.results[stepName]; ok {
		return "", err
	}

	// F048: Return transition if configured for this step
	if nextStep, ok := r.transitions[stepName]; ok {
		return nextStep, nil
	}

	return "", nil
}
