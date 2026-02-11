package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

// MockLogger implements ports.Logger for testing.
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...any) {
	m.Called(msg, fields)
}

func (m *MockLogger) Info(msg string, fields ...any) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields ...any) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, fields ...any) {
	m.Called(msg, fields)
}

func (m *MockLogger) WithContext(ctx map[string]any) ports.Logger {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return m
	}
	return args.Get(0).(ports.Logger)
}

// MockExecutor implements ports.CommandExecutor for testing.
type MockExecutor struct {
	mock.Mock
}

func (m *MockExecutor) Execute(ctx context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	args := m.Called(ctx, cmd)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.CommandResult), args.Error(1)
}

// MockResolver implements interpolation.Resolver for testing.
type MockResolver struct {
	mock.Mock
}

func (m *MockResolver) Resolve(template string, ctx *interpolation.Context) (string, error) {
	args := m.Called(template, ctx)
	return args.String(0), args.Error(1)
}

func TestHookExecutor_ExecuteHooks_EmptyHook(t *testing.T) {
	logger := new(MockLogger)
	executor := new(MockExecutor)
	resolver := new(MockResolver)

	hookExec := NewHookExecutor(executor, logger, resolver)

	err := hookExec.ExecuteHooks(context.Background(), nil, nil, false)
	assert.NoError(t, err)

	err = hookExec.ExecuteHooks(context.Background(), workflow.Hook{}, nil, false)
	assert.NoError(t, err)
}

func TestHookExecutor_ExecuteHooks_LogAction(t *testing.T) {
	logger := new(MockLogger)
	executor := new(MockExecutor)
	resolver := new(MockResolver)

	hook := workflow.Hook{
		{Log: "Starting workflow..."},
	}

	resolver.On("Resolve", "Starting workflow...", mock.Anything).Return("Starting workflow...", nil)
	logger.On("Info", "Starting workflow...", mock.Anything).Return()

	hookExec := NewHookExecutor(executor, logger, resolver)
	err := hookExec.ExecuteHooks(context.Background(), hook, nil, false)

	require.NoError(t, err)
	logger.AssertCalled(t, "Info", "Starting workflow...", mock.Anything)
}

func TestHookExecutor_ExecuteHooks_CommandAction(t *testing.T) {
	logger := new(MockLogger)
	executor := new(MockExecutor)
	resolver := new(MockResolver)

	hook := workflow.Hook{
		{Command: "mkdir -p output"},
	}

	resolver.On("Resolve", "mkdir -p output", mock.Anything).Return("mkdir -p output", nil)
	executor.On("Execute", mock.Anything, mock.MatchedBy(func(cmd *ports.Command) bool {
		return cmd.Program == "mkdir -p output"
	})).Return(&ports.CommandResult{ExitCode: 0}, nil)
	logger.On("Debug", mock.Anything, mock.Anything).Return()

	hookExec := NewHookExecutor(executor, logger, resolver)
	err := hookExec.ExecuteHooks(context.Background(), hook, nil, false)

	require.NoError(t, err)
	executor.AssertExpectations(t)
}

func TestHookExecutor_ExecuteHooks_MultipleActions(t *testing.T) {
	logger := new(MockLogger)
	executor := new(MockExecutor)
	resolver := new(MockResolver)

	hook := workflow.Hook{
		{Log: "Step 1"},
		{Command: "echo hello"},
		{Log: "Step 2"},
	}

	resolver.On("Resolve", "Step 1", mock.Anything).Return("Step 1", nil)
	resolver.On("Resolve", "echo hello", mock.Anything).Return("echo hello", nil)
	resolver.On("Resolve", "Step 2", mock.Anything).Return("Step 2", nil)
	logger.On("Info", "Step 1", mock.Anything).Return()
	logger.On("Info", "Step 2", mock.Anything).Return()
	logger.On("Debug", mock.Anything, mock.Anything).Return()
	executor.On("Execute", mock.Anything, mock.MatchedBy(func(cmd *ports.Command) bool {
		return cmd.Program == "echo hello"
	})).Return(&ports.CommandResult{ExitCode: 0}, nil)

	hookExec := NewHookExecutor(executor, logger, resolver)
	err := hookExec.ExecuteHooks(context.Background(), hook, nil, false)

	require.NoError(t, err)
	logger.AssertCalled(t, "Info", "Step 1", mock.Anything)
	logger.AssertCalled(t, "Info", "Step 2", mock.Anything)
	executor.AssertExpectations(t)
}

func TestHookExecutor_ExecuteHooks_VariableInterpolation(t *testing.T) {
	logger := new(MockLogger)
	executor := new(MockExecutor)
	resolver := new(MockResolver)

	hook := workflow.Hook{
		{Log: "Processing {{inputs.file}}"},
	}

	intCtx := &interpolation.Context{
		Inputs: map[string]any{"file": "test.txt"},
	}

	resolver.On("Resolve", "Processing {{inputs.file}}", intCtx).
		Return("Processing test.txt", nil)
	logger.On("Info", "Processing test.txt", mock.Anything).Return()

	hookExec := NewHookExecutor(executor, logger, resolver)
	err := hookExec.ExecuteHooks(context.Background(), hook, intCtx, false)

	require.NoError(t, err)
	logger.AssertCalled(t, "Info", "Processing test.txt", mock.Anything)
}

func TestHookExecutor_ExecuteHooks_CommandFailure_ContinueByDefault(t *testing.T) {
	logger := new(MockLogger)
	executor := new(MockExecutor)
	resolver := new(MockResolver)

	hook := workflow.Hook{
		{Command: "failing-command"},
		{Log: "After failure"},
	}

	resolver.On("Resolve", "failing-command", mock.Anything).Return("failing-command", nil)
	resolver.On("Resolve", "After failure", mock.Anything).Return("After failure", nil)
	executor.On("Execute", mock.Anything, mock.MatchedBy(func(cmd *ports.Command) bool {
		return cmd.Program == "failing-command"
	})).Return(nil, errors.New("command failed"))
	logger.On("Debug", mock.Anything, mock.Anything).Return()
	logger.On("Warn", mock.Anything, mock.Anything).Return()
	logger.On("Info", "After failure", mock.Anything).Return()

	hookExec := NewHookExecutor(executor, logger, resolver)
	err := hookExec.ExecuteHooks(context.Background(), hook, nil, false)

	require.NoError(t, err) // continues despite failure
	logger.AssertCalled(t, "Warn", mock.Anything, mock.Anything)
	logger.AssertCalled(t, "Info", "After failure", mock.Anything)
}

func TestHookExecutor_ExecuteHooks_CommandFailure_FailOnError(t *testing.T) {
	logger := new(MockLogger)
	executor := new(MockExecutor)
	resolver := new(MockResolver)

	hook := workflow.Hook{
		{Command: "failing-command"},
		{Log: "Should not run"},
	}

	resolver.On("Resolve", "failing-command", mock.Anything).Return("failing-command", nil)
	executor.On("Execute", mock.Anything, mock.MatchedBy(func(cmd *ports.Command) bool {
		return cmd.Program == "failing-command"
	})).Return(nil, errors.New("command failed"))
	logger.On("Debug", mock.Anything, mock.Anything).Return()

	hookExec := NewHookExecutor(executor, logger, resolver)
	err := hookExec.ExecuteHooks(context.Background(), hook, nil, true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "command failed")
	// "Should not run" log should NOT be called
	logger.AssertNotCalled(t, "Info", "Should not run", mock.Anything)
}

func TestHookExecutor_ExecuteHooks_NonZeroExitCode(t *testing.T) {
	logger := new(MockLogger)
	executor := new(MockExecutor)
	resolver := new(MockResolver)

	hook := workflow.Hook{
		{Command: "exit 1"},
	}

	resolver.On("Resolve", "exit 1", mock.Anything).Return("exit 1", nil)
	executor.On("Execute", mock.Anything, mock.MatchedBy(func(cmd *ports.Command) bool {
		return cmd.Program == "exit 1"
	})).Return(&ports.CommandResult{ExitCode: 1, Stderr: "error"}, nil)
	logger.On("Debug", mock.Anything, mock.Anything).Return()
	logger.On("Warn", mock.Anything, mock.Anything).Return()

	hookExec := NewHookExecutor(executor, logger, resolver)
	err := hookExec.ExecuteHooks(context.Background(), hook, nil, false)

	require.NoError(t, err) // continues by default
	logger.AssertCalled(t, "Warn", mock.Anything, mock.Anything)
}

func TestHookExecutor_ExecuteHooks_InterpolationError(t *testing.T) {
	logger := new(MockLogger)
	executor := new(MockExecutor)
	resolver := new(MockResolver)

	hook := workflow.Hook{
		{Log: "{{undefined.var}}"},
	}

	resolver.On("Resolve", "{{undefined.var}}", mock.Anything).
		Return("", errors.New("undefined variable"))
	logger.On("Warn", mock.Anything, mock.Anything).Return()

	hookExec := NewHookExecutor(executor, logger, resolver)
	err := hookExec.ExecuteHooks(context.Background(), hook, nil, false)

	require.NoError(t, err) // continues by default
	logger.AssertCalled(t, "Warn", mock.Anything, mock.Anything)
}

func TestHookExecutor_ExecuteHooks_ContextCancellation(t *testing.T) {
	logger := new(MockLogger)
	executor := new(MockExecutor)
	resolver := new(MockResolver)

	hook := workflow.Hook{
		{Command: "long-running"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	resolver.On("Resolve", "long-running", mock.Anything).Return("long-running", nil)
	executor.On("Execute", mock.Anything, mock.MatchedBy(func(cmd *ports.Command) bool {
		return cmd.Program == "long-running"
	})).Return(nil, context.Canceled)
	logger.On("Debug", mock.Anything, mock.Anything).Return()

	hookExec := NewHookExecutor(executor, logger, resolver)
	err := hookExec.ExecuteHooks(ctx, hook, nil, false)

	// Context cancellation should propagate
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}
