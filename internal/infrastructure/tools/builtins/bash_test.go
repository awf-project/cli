package builtins_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/tools/builtins"
	"github.com/awf-project/cli/internal/testutil/mocks"
)

// TestBash_HappyPath_CommandExecution verifies happy path execution.
// Acceptance: Provider.CallTool(ctx, "Bash", {"command": "echo hi"}) invokes Executor.Execute
// and returns combined Stdout + Stderr in Content[0].Text with IsError: false.
func TestBash_HappyPath_CommandExecution(t *testing.T) {
	mockExec := mocks.NewMockCommandExecutor()
	mockExec.SetCommandResult("echo hi", &ports.CommandResult{
		Stdout:   "hi\n",
		Stderr:   "",
		ExitCode: 0,
	})

	provider := builtins.NewProvider(builtins.WithExecutor(mockExec))
	result, err := provider.CallTool(context.Background(), "Bash", map[string]any{
		"command": "echo hi",
	})

	require.NoError(t, err, "CallTool should return nil error on successful execution")
	require.NotNil(t, result, "CallTool should return non-nil result")
	assert.False(t, result.IsError, "IsError should be false for successful command")
	assert.Len(t, result.Content, 1, "result should contain exactly one content block")
	assert.Equal(t, "hi\n", result.Content[0].Text, "text should contain stdout")
}

// TestBash_NonZeroExitCode_ReturnsIsError verifies exit code error handling.
// Acceptance: Bash returns IsError: true when result.ExitCode != 0 (text contains exit code + stderr).
func TestBash_NonZeroExitCode_ReturnsIsError(t *testing.T) {
	mockExec := mocks.NewMockCommandExecutor()
	mockExec.SetCommandResult("failing_command", &ports.CommandResult{
		Stdout:   "",
		Stderr:   "boom",
		ExitCode: 2,
	})

	provider := builtins.NewProvider(builtins.WithExecutor(mockExec))
	result, err := provider.CallTool(context.Background(), "Bash", map[string]any{
		"command": "failing_command",
	})

	require.NoError(t, err, "CallTool should return nil error; exit code failure is IsError, not Go error")
	require.NotNil(t, result, "CallTool should return non-nil result")
	assert.True(t, result.IsError, "IsError should be true on non-zero exit code")
	assert.Len(t, result.Content, 1, "result should contain exactly one content block")
	text := result.Content[0].Text
	assert.Contains(t, text, "exit code 2", "text should contain formatted exit code")
	assert.Contains(t, text, "boom", "text should contain stderr")
}

// TestBash_ExecutorSpawnFailure_ReturnsGoError verifies Go error on executor failure.
// Acceptance: Bash returns Go error when Executor.Execute itself returns an error
// (spawn failure, context cancelled).
func TestBash_ExecutorSpawnFailure_ReturnsGoError(t *testing.T) {
	mockExec := mocks.NewMockCommandExecutor()
	expectedErr := errors.New("spawn failed")
	mockExec.SetExecuteError(expectedErr)

	provider := builtins.NewProvider(builtins.WithExecutor(mockExec))
	result, err := provider.CallTool(context.Background(), "Bash", map[string]any{
		"command": "ls",
	})

	assert.Error(t, err, "CallTool should return error on executor failure")
	assert.Nil(t, result, "result should be nil when executor returns error")
	assert.ErrorIs(t, err, expectedErr, "returned error should be the executor error")
}

// TestBash_WithCwd_PassesDirectoryToExecutor verifies cwd parameter handling.
// Acceptance: Bash schema includes optional cwd string; handler passes to Command.Dir.
func TestBash_WithCwd_PassesDirectoryToExecutor(t *testing.T) {
	mockExec := mocks.NewMockCommandExecutor()
	mockExec.SetCommandResult("ls", &ports.CommandResult{
		Stdout:   "file.txt\n",
		Stderr:   "",
		ExitCode: 0,
	})

	provider := builtins.NewProvider(builtins.WithExecutor(mockExec))
	result, err := provider.CallTool(context.Background(), "Bash", map[string]any{
		"command": "ls",
		"cwd":     "/tmp",
	})

	require.NoError(t, err, "CallTool should succeed with cwd")
	require.NotNil(t, result, "result should not be nil")
	assert.False(t, result.IsError, "IsError should be false for successful command")

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1, "executor should be called exactly once")
	assert.Equal(t, "/tmp", calls[0].Dir, "Dir should be set to provided cwd")
	assert.Equal(t, "ls", calls[0].Program, "Program should be the command")
	assert.False(t, calls[0].IsScriptFile, "IsScriptFile should be false for shell commands")
}

// TestBash_CombinedStdoutStderr_InContent verifies output combination.
// Acceptance: CallTool returns combined Stdout + Stderr in Content[0].Text.
func TestBash_CombinedStdoutStderr_InContent(t *testing.T) {
	mockExec := mocks.NewMockCommandExecutor()
	mockExec.SetCommandResult("mixed_command", &ports.CommandResult{
		Stdout:   "output line\n",
		Stderr:   "error line\n",
		ExitCode: 0,
	})

	provider := builtins.NewProvider(builtins.WithExecutor(mockExec))
	result, err := provider.CallTool(context.Background(), "Bash", map[string]any{
		"command": "mixed_command",
	})

	require.NoError(t, err, "CallTool should return nil error")
	require.NotNil(t, result, "result should not be nil")
	assert.False(t, result.IsError, "IsError should be false on exit code 0")
	text := result.Content[0].Text
	assert.Contains(t, text, "output line", "text should contain stdout")
	assert.Contains(t, text, "error line", "text should contain stderr")
}

// TestBash_MissingCommand_ReturnsError verifies schema validation.
// Acceptance: Bash schema requires "command" string.
func TestBash_MissingCommand_ReturnsError(t *testing.T) {
	provider := builtins.NewProvider()
	result, err := provider.CallTool(context.Background(), "Bash", map[string]any{
		"cwd": "/tmp",
	})

	assert.Error(t, err, "CallTool should return error when required command is missing")
	assert.Nil(t, result, "result should be nil")
	assert.Contains(t, err.Error(), "missing required argument", "error should mention missing argument")
}

// TestBash_NoExecutor_ReturnsIsError verifies that calling Bash when no executor
// is configured returns a graceful ToolResult with IsError:true instead of panicking.
// The schema validation occurs before the executor nil check, so we must use a
// provider created with NewProvider() (no WithExecutor) and pass a valid command.
func TestBash_NoExecutor_ReturnsIsError(t *testing.T) {
	// Provider without WithExecutor: executor field is nil.
	provider := builtins.NewProvider()
	result, err := provider.CallTool(context.Background(), "Bash", map[string]any{
		"command": "echo hello",
	})

	require.NoError(t, err, "CallTool should return nil error (not a Go error) when executor is nil")
	require.NotNil(t, result, "result should not be nil when executor is nil")
	assert.True(t, result.IsError, "IsError should be true when executor is not configured")
	require.Len(t, result.Content, 1, "result should contain exactly one content block")
	assert.Contains(t, result.Content[0].Text, "no executor configured",
		"error text should mention missing executor")
}

// ctxCapturingExecutor is a test-only executor that captures the context it receives.
type ctxCapturingExecutor struct {
	capturedCtx context.Context
	result      *ports.CommandResult
}

func (e *ctxCapturingExecutor) Execute(ctx context.Context, _ *ports.Command) (*ports.CommandResult, error) {
	e.capturedCtx = ctx
	if e.result != nil {
		return e.result, nil
	}
	return &ports.CommandResult{Stdout: "", Stderr: "", ExitCode: 0}, nil
}

// TestBash_TimeoutSeconds_WrapsContext verifies that Bash honors the
// timeout_seconds parameter by wrapping ctx with context.WithTimeout before
// calling Execute. The captured context must have a deadline approximately
// equal to now + timeout_seconds (tolerance: 500ms).
func TestBash_TimeoutSeconds_WrapsContext(t *testing.T) {
	const timeoutSecs = 1

	capturingExec := &ctxCapturingExecutor{
		result: &ports.CommandResult{Stdout: "ok", Stderr: "", ExitCode: 0},
	}

	provider := builtins.NewProvider(builtins.WithExecutor(capturingExec))

	before := time.Now()
	_, err := provider.CallTool(context.Background(), "Bash", map[string]any{
		"command":         "true",
		"timeout_seconds": timeoutSecs,
	})
	require.NoError(t, err, "CallTool should succeed with timeout_seconds")
	after := time.Now()

	require.NotNil(t, capturingExec.capturedCtx, "executor must have been called")

	deadline, ok := capturingExec.capturedCtx.Deadline()
	require.True(t, ok, "context must have a deadline when timeout_seconds is set")

	// Deadline must be in the future relative to the call start, and within now+timeout+tolerance.
	expectedMin := before.Add(time.Duration(timeoutSecs) * time.Second)
	expectedMax := after.Add(time.Duration(timeoutSecs)*time.Second + 500*time.Millisecond)

	assert.True(t, deadline.After(before),
		"deadline must be after call start; got deadline=%v, before=%v", deadline, before)
	assert.True(t, deadline.Before(expectedMax),
		"deadline must be before now+timeout+tolerance; got deadline=%v, expectedMax=%v", deadline, expectedMax)
	assert.True(t, !deadline.Before(expectedMin),
		"deadline must be at least now+timeout; got deadline=%v, expectedMin=%v", deadline, expectedMin)
}
