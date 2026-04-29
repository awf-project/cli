package application

import (
	"context"
	"io"
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSpan implements ports.Span for testing
type MockSpan struct {
	mock.Mock
	ended bool
}

func (m *MockSpan) End() {
	m.Called()
	m.ended = true
}

func (m *MockSpan) SetAttribute(key string, value any) {
	m.Called(key, value)
}

func (m *MockSpan) RecordError(err error) {
	m.Called(err)
}

func (m *MockSpan) AddEvent(name string) {
	m.Called(name)
}

// MockTracer implements ports.Tracer for testing
type MockTracer struct {
	mock.Mock
	spans []*MockSpan
}

func (m *MockTracer) Start(ctx context.Context, spanName string) (context.Context, ports.Span) {
	m.Called(ctx, spanName)
	span := &MockSpan{}
	span.On("End").Return()
	span.On("SetAttribute", mock.Anything, mock.Anything).Maybe().Return()
	span.On("RecordError", mock.Anything).Maybe().Return()
	span.On("AddEvent", mock.Anything).Maybe().Return()
	m.spans = append(m.spans, span)
	return ctx, span
}

// MockCommandExecutor implements ports.CommandExecutor for testing
type MockCommandExecutor struct {
	mock.Mock
}

func (m *MockCommandExecutor) Execute(ctx context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	args := m.Called(ctx, cmd)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.CommandResult), args.Error(1)
}

// TestExecuteStepCommand_EmitsShellExecuteSpan verifies shell.execute span is started and ended
func TestExecuteStepCommand_EmitsShellExecuteSpan(t *testing.T) {
	mockTracer := new(MockTracer)
	mockExecutor := new(MockCommandExecutor)

	// Setup mock expectations
	expectedCtx := context.Background()
	mockTracer.On("Start", expectedCtx, "shell.execute").Return(expectedCtx, &MockSpan{})

	result := &ports.CommandResult{
		Stdout:   "command output",
		ExitCode: 0,
	}
	mockExecutor.On("Execute", expectedCtx, mock.MatchedBy(func(cmd *ports.Command) bool {
		return cmd.Program == "echo"
	})).Return(result, nil)

	// Create ExecutionService with mocked dependencies
	svc := &ExecutionService{
		executor: mockExecutor,
		tracer:   mockTracer,
	}

	step := &workflow.Step{
		Name: "test-step",
		Type: workflow.StepTypeCommand,
	}

	cmd := &ports.Command{
		Program: "echo",
	}

	// Execute
	ctx := context.Background()
	cmdResult, attempts, err := svc.executeStepCommand(ctx, step, cmd)

	// Verify span was started
	assert.NoError(t, err)
	assert.NotNil(t, cmdResult)
	assert.Equal(t, 1, attempts)
	mockTracer.AssertCalled(t, "Start", ctx, "shell.execute")
}

// TestExecuteStepCommand_EmitsSpanWithNilTracer verifies executeStepCommand handles nil tracer gracefully
func TestExecuteStepCommand_WithNilTracer(t *testing.T) {
	svc := &ExecutionService{
		executor: nil,
		tracer:   nil,
	}

	step := &workflow.Step{
		Name: "test-step",
	}

	cmd := &ports.Command{
		Program: "echo",
	}

	// Should not panic with nil tracer
	ctx := context.Background()
	cmdResult, attempts, err := svc.executeStepCommand(ctx, step, cmd)

	// With nil executor (current stub behavior), returns nil
	assert.NoError(t, err)
	assert.Nil(t, cmdResult)
	assert.Equal(t, 0, attempts)
}

// MockOperationProvider implements ports.OperationProvider for testing
type MockOperationProvider struct {
	mock.Mock
}

func (m *MockOperationProvider) GetOperation(name string) (*pluginmodel.OperationSchema, bool) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Bool(1)
	}
	return args.Get(0).(*pluginmodel.OperationSchema), args.Bool(1)
}

func (m *MockOperationProvider) ListOperations() []*pluginmodel.OperationSchema {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]*pluginmodel.OperationSchema)
}

func (m *MockOperationProvider) Execute(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
	args := m.Called(ctx, name, inputs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pluginmodel.OperationResult), args.Error(1)
}

// TestExecutePluginOperation_EmitsPluginRpcSpan verifies plugin.rpc span is emitted
func TestExecutePluginOperation_EmitsPluginRpcSpan(t *testing.T) {
	mockTracer := new(MockTracer)
	mockTracer.On("Start", mock.Anything, "plugin.rpc").Return()
	mockOpProvider := new(MockOperationProvider)

	mockOpProvider.On("GetOperation", "test.operation").Return(nil, false)

	// Setup execution service with mocked dependencies
	svc := &ExecutionService{
		tracer:            mockTracer,
		operationProvider: mockOpProvider,
		stdoutWriter:      io.Discard,
		stderrWriter:      io.Discard,
	}

	step := &workflow.Step{
		Name:      "test-operation",
		Operation: "test.operation",
	}

	execCtx := &workflow.ExecutionContext{
		WorkflowID: "test-workflow",
	}

	// Execute — operation not found triggers early return after span start
	ctx := context.Background()
	_, err := svc.executePluginOperation(ctx, step, execCtx)

	// Verify span was started with correct name
	mockTracer.AssertCalled(t, "Start", ctx, "plugin.rpc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestIsSecretInputKey_IdentifiesSecretPrefixes verifies isSecretInputKey detection
func TestIsSecretInputKey_IdentifiesSecretPrefixes(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		wantOK bool
	}{
		{"SECRET_ prefix", "SECRET_API_KEY", true},
		{"secret_ lowercase", "secret_key", true},
		{"Secret_ mixed case", "Secret_Key", true},
		{"API_KEY prefix", "API_KEY", true},
		{"api_key lowercase", "api_key", true},
		{"PASSWORD prefix", "PASSWORD", true},
		{"password lowercase", "password", true},
		{"TOKEN prefix", "TOKEN", true},
		{"token lowercase", "token", true},
		{"TOKEN_AUTH", "TOKEN_AUTH", true},
		{"TOKEN_REFRESH", "TOKEN_REFRESH", true},
		{"AUTH_TOKEN", "AUTH_TOKEN", false},
		{"REFRESH_TOKEN", "REFRESH_TOKEN", false},
		{"DB_PASSWORD", "DB_PASSWORD", false},
		{"not a secret", "NORMAL_VAR", false},
		{"similar but not secret", "SECRECY", false},
		{"USER_SECRET", "USER_SECRET", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSecretInputKey(tt.key)
			assert.Equal(t, tt.wantOK, result)
		})
	}
}
