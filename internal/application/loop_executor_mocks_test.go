package application_test

import (
	"context"

	"github.com/awf-project/cli/pkg/interpolation"
)

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

// mockExpressionEvaluator implements ports.ExpressionEvaluator for testing
// C042: Updated to implement EvaluateBool and EvaluateInt methods
type mockExpressionEvaluator struct {
	boolResults map[string]bool
	intResults  map[string]int
	calls       []string
	err         error
}

func newMockExpressionEvaluator() *mockExpressionEvaluator {
	return &mockExpressionEvaluator{
		boolResults: make(map[string]bool),
		intResults:  make(map[string]int),
		calls:       make([]string, 0),
	}
}

func (m *mockExpressionEvaluator) EvaluateBool(expr string, ctx *interpolation.Context) (bool, error) {
	m.calls = append(m.calls, expr)
	if m.err != nil {
		return false, m.err
	}
	if result, ok := m.boolResults[expr]; ok {
		return result, nil
	}
	// Return false for unconfigured expressions
	return false, nil
}

func (m *mockExpressionEvaluator) EvaluateInt(expr string, ctx *interpolation.Context) (int, error) {
	m.calls = append(m.calls, expr)
	if m.err != nil {
		return 0, m.err
	}
	if result, ok := m.intResults[expr]; ok {
		return result, nil
	}
	// Return 0 for unconfigured expressions
	return 0, nil
}

// configurableMockResolver implements interpolation.Resolver with configurable results
type configurableMockResolver struct {
	results        map[string]string
	calls          []string
	err            error
	templateErrors map[string]error // per-template error injection
}

func newConfigurableMockResolver() *configurableMockResolver {
	return &configurableMockResolver{
		results:        make(map[string]string),
		calls:          make([]string, 0),
		templateErrors: make(map[string]error),
	}
}

func (m *configurableMockResolver) Resolve(template string, ctx *interpolation.Context) (string, error) {
	m.calls = append(m.calls, template)
	if m.err != nil {
		return "", m.err
	}
	if err, ok := m.templateErrors[template]; ok {
		return "", err
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
			Parent: intCtx.Loop.Parent,
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
