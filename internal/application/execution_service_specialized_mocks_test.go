package application_test

import (
	"context"
	"fmt"
	"time"

	"github.com/vanoix/awf/internal/domain/plugin"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/pkg/interpolation"
)

// =============================================================================
// Shared Mock Executors and Evaluators for Execution Service Tests
// Feature: C008 - Test File Restructuring
// Component: extract_shared_mocks (T002)
// =============================================================================
//
// This file contains specialized mock implementations shared across multiple
// execution service test files. These mocks enable testing of:
// - Timeout behavior (timeoutMockExecutor)
// - Error handling (errorMockExecutor)
// - Retry logic (retryCountingExecutor)
// - Conditional expressions (mockEvaluator)
//
// Extracted from: execution_service_test.go and execution_service_retry_test.go
// Usage: Used by execution_service_test.go, execution_service_retry_test.go,
//        and future split test files (loop, parallel, hooks, core)
// =============================================================================

// timeoutMockExecutor simulates timeout behavior for testing context cancellation.
type timeoutMockExecutor struct {
	timeout time.Duration
}

func (m *timeoutMockExecutor) Execute(ctx context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	// simulate slow execution that gets cancelled
	select {
	case <-time.After(m.timeout):
		return &ports.CommandResult{ExitCode: -1}, context.DeadlineExceeded
	case <-ctx.Done():
		return &ports.CommandResult{ExitCode: -1}, fmt.Errorf("execution cancelled: %w", ctx.Err())
	}
}

// errorMockExecutor always returns an error, used for testing error handling paths.
type errorMockExecutor struct {
	err error
}

func (m *errorMockExecutor) Execute(ctx context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	return &ports.CommandResult{ExitCode: -1}, m.err
}

// retryCountingExecutor tracks execution count per command for testing retry logic.
// Supports configurable results for successive calls to simulate retry scenarios.
type retryCountingExecutor struct {
	calls       map[string]int
	results     map[string][]*ports.CommandResult // multiple results for successive calls
	defaultErr  error
	callHistory []string
}

func newRetryCountingExecutor() *retryCountingExecutor {
	return &retryCountingExecutor{
		calls:       make(map[string]int),
		results:     make(map[string][]*ports.CommandResult),
		callHistory: make([]string, 0),
	}
}

func (m *retryCountingExecutor) Execute(ctx context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	m.calls[cmd.Program]++
	m.callHistory = append(m.callHistory, cmd.Program)

	if results, ok := m.results[cmd.Program]; ok {
		idx := m.calls[cmd.Program] - 1
		if idx < len(results) {
			return results[idx], nil
		}
		// Return the last result for additional calls
		return results[len(results)-1], nil
	}

	if m.defaultErr != nil {
		return &ports.CommandResult{ExitCode: -1}, m.defaultErr
	}
	return &ports.CommandResult{ExitCode: 0, Stdout: "ok"}, nil
}

// conditionMockEvaluator implements ExpressionEvaluator for testing conditional expressions.
// Returns configured evaluation results for testing conditional step logic.
type conditionMockEvaluator struct {
	evaluations map[string]bool
}

func newConditionMockEvaluator() *conditionMockEvaluator {
	return &conditionMockEvaluator{
		evaluations: make(map[string]bool),
	}
}

func (m *conditionMockEvaluator) Evaluate(expr string, ctx *interpolation.Context) (bool, error) {
	if result, ok := m.evaluations[expr]; ok {
		return result, nil
	}
	// Default to false for unknown expressions
	return false, nil
}

// MockOperationProvider implements ports.OperationProvider for testing operation resolution.
// Tracks operation execution calls to verify interpolation behavior.
type MockOperationProvider struct {
	operations       map[string]*MockOperation
	ExecuteCallCount int
	LastOperation    string
	LastInputs       map[string]any
}

func (m *MockOperationProvider) SetOperation(name string, op *MockOperation) {
	if m.operations == nil {
		m.operations = make(map[string]*MockOperation)
	}
	m.operations[name] = op
}

func (m *MockOperationProvider) GetOperation(name string) (*plugin.OperationSchema, bool) {
	_, found := m.operations[name]
	if !found {
		return nil, false
	}
	// Return a basic schema for the operation
	return &plugin.OperationSchema{
		Name:        name,
		Description: "Mock operation for testing",
	}, found
}

func (m *MockOperationProvider) ListOperations() []*plugin.OperationSchema {
	schemas := make([]*plugin.OperationSchema, 0, len(m.operations))
	for name := range m.operations {
		schemas = append(schemas, &plugin.OperationSchema{
			Name:        name,
			Description: "Mock operation for testing",
		})
	}
	return schemas
}

func (m *MockOperationProvider) Execute(ctx context.Context, name string, inputs map[string]any) (*plugin.OperationResult, error) {
	m.ExecuteCallCount++
	m.LastOperation = name
	m.LastInputs = inputs

	op, found := m.operations[name]
	if !found {
		return nil, fmt.Errorf("operation not found: %s", name)
	}

	return op.Execute(ctx, inputs)
}

// MockOperation implements a mock operation for testing.
type MockOperation struct {
	ExecuteFunc func(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error)
}

func (m *MockOperation) Execute(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, inputs)
	}
	// Default success result
	return &plugin.OperationResult{
		Success: true,
		Outputs: make(map[string]any),
	}, nil
}
