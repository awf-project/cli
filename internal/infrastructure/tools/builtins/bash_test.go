package builtins_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/executor"
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

type contextObservingExecutor struct {
	delegate ports.CommandExecutor
	contexts chan context.Context
}

func (e *contextObservingExecutor) Execute(ctx context.Context, command *ports.Command) (*ports.CommandResult, error) {
	select {
	case e.contexts <- ctx:
	default:
	}
	return e.delegate.Execute(ctx, command)
}

func TestBash_WithoutTimeoutUsesCallerContext(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process-group lifecycle assertions require POSIX signals")
	}

	privateValues := []string{
		"PRIVATE_BASH_CANCELLATION_COMMAND",
		"PRIVATE_TOOL_OUTPUT",
		"PRIVATE_PROMPT",
		"PRIVATE_REQUEST_BODY",
		"PRIVATE_AUTHORIZATION_HEADER",
		"PRIVATE_PLUGIN_CONFIGURATION",
		"PRIVATE_TOKEN",
		"PRIVATE_CREDENTIAL",
	}
	tmpDir := t.TempDir()
	readyPath := filepath.Join(tmpDir, "ready")
	pidPath := filepath.Join(tmpDir, "child.pid")
	observingExec := &contextObservingExecutor{
		delegate: executor.NewShellExecutor(),
		contexts: make(chan context.Context, 1),
	}
	provider := builtins.NewProvider(builtins.WithExecutor(observingExec))
	callerCtx, cancel := context.WithCancel(context.WithValue(context.Background(), bashContextKey{}, "operation-123"))
	defer cancel()

	resultCh := make(chan bashExecution, 1)
	go func() {
		result, err := provider.CallTool(callerCtx, "Bash", map[string]any{
			"command": bashBlockingCommand(privateValues, pidPath, readyPath),
		})
		resultCh <- bashExecution{result: result, err: err}
	}()

	require.Eventually(t, func() bool {
		_, err := os.Stat(readyPath)
		return err == nil
	}, 2*time.Second, 10*time.Millisecond, "child process did not reach the synchronized blocker")

	observedCtx := <-observingExec.contexts
	require.Same(t, callerCtx, observedCtx)
	_, hasDeadline := observedCtx.Deadline()
	assert.False(t, hasDeadline)
	assert.Equal(t, "operation-123", observedCtx.Value(bashContextKey{}))

	childPID := readBashChildPID(t, pidPath)
	cancel()

	execution := awaitBashExecution(t, resultCh)
	require.Error(t, execution.err)
	assert.ErrorIs(t, execution.err, context.Canceled)
	assert.Nil(t, execution.result)
	for _, sensitive := range privateValues {
		assert.NotContains(t, execution.err.Error(), sensitive)
	}
	assertBashChildTerminated(t, childPID)
}

type bashContextKey struct{}

func TestBash_TimeoutSeconds_WrapsContext(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process-group lifecycle assertions require POSIX signals")
	}

	const timeoutSecs = 1
	privateValues := []string{
		"PRIVATE_BASH_TIMEOUT_COMMAND",
		"PRIVATE_TOOL_OUTPUT",
		"PRIVATE_PROMPT",
		"PRIVATE_REQUEST_BODY",
		"PRIVATE_AUTHORIZATION_HEADER",
		"PRIVATE_PLUGIN_CONFIGURATION",
		"PRIVATE_TOKEN",
		"PRIVATE_CREDENTIAL",
	}
	tmpDir := t.TempDir()
	readyPath := filepath.Join(tmpDir, "ready")
	pidPath := filepath.Join(tmpDir, "child.pid")
	observingExec := &contextObservingExecutor{
		delegate: executor.NewShellExecutor(),
		contexts: make(chan context.Context, 1),
	}
	provider := builtins.NewProvider(builtins.WithExecutor(observingExec))
	callerCtx := context.WithValue(context.Background(), bashContextKey{}, "operation-456")
	before := time.Now()
	resultCh := make(chan bashExecution, 1)
	go func() {
		result, err := provider.CallTool(callerCtx, "Bash", map[string]any{
			"command":         bashBlockingCommand(privateValues, pidPath, readyPath),
			"timeout_seconds": timeoutSecs,
		})
		resultCh <- bashExecution{result: result, err: err}
	}()

	require.Eventually(t, func() bool {
		_, err := os.Stat(readyPath)
		return err == nil
	}, 2*time.Second, 10*time.Millisecond, "child process did not reach the synchronized blocker")

	observedCtx := <-observingExec.contexts
	assert.Equal(t, "operation-456", observedCtx.Value(bashContextKey{}))
	deadline, ok := observedCtx.Deadline()
	require.True(t, ok, "context must have a deadline when timeout_seconds is set")
	expectedMin := before.Add(time.Duration(timeoutSecs) * time.Second)
	expectedMax := before.Add(time.Duration(timeoutSecs)*time.Second + 500*time.Millisecond)

	assert.True(t, deadline.After(before),
		"deadline must be after call start; got deadline=%v, before=%v", deadline, before)
	assert.True(t, deadline.Before(expectedMax),
		"deadline must be before now+timeout+tolerance; got deadline=%v, expectedMax=%v", deadline, expectedMax)
	assert.True(t, !deadline.Before(expectedMin),
		"deadline must be at least now+timeout; got deadline=%v, expectedMin=%v", deadline, expectedMin)

	childPID := readBashChildPID(t, pidPath)
	execution := awaitBashExecution(t, resultCh)
	require.Error(t, execution.err)
	assert.ErrorIs(t, execution.err, context.DeadlineExceeded)
	assert.Nil(t, execution.result)
	for _, sensitive := range privateValues {
		assert.NotContains(t, execution.err.Error(), sensitive)
	}
	assertBashChildTerminated(t, childPID)
}

type bashExecution struct {
	result *ports.ToolResult
	err    error
}

func bashBlockingCommand(privateValues []string, pidPath, readyPath string) string {
	quotedValues := make([]string, 0, len(privateValues))
	for _, value := range privateValues {
		quotedValues = append(quotedValues, fmt.Sprintf("%q", value))
	}
	return fmt.Sprintf(
		"printf '%%s' %s >/dev/null; sleep 30 & child=$!; printf '%%s' \"$child\" > %q; touch %q; wait \"$child\"",
		strings.Join(quotedValues, " "),
		pidPath,
		readyPath,
	)
}

func readBashChildPID(t *testing.T, pidPath string) int {
	t.Helper()
	pidBytes, err := os.ReadFile(pidPath)
	require.NoError(t, err)
	childPID, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	require.NoError(t, err)
	return childPID
}

func awaitBashExecution(t *testing.T, resultCh <-chan bashExecution) bashExecution {
	t.Helper()
	select {
	case execution := <-resultCh:
		return execution
	case <-time.After(2 * time.Second):
		t.Fatal("Bash command did not return after context termination")
		return bashExecution{}
	}
}

func assertBashChildTerminated(t *testing.T, childPID int) {
	t.Helper()
	assert.Eventually(t, func() bool {
		err := syscall.Kill(childPID, 0)
		return errors.Is(err, syscall.ESRCH)
	}, time.Second, 10*time.Millisecond, "descendant process %d survived context termination", childPID)
}
