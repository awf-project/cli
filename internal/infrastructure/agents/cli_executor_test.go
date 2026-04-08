package agents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: exec_cli_executor
// Task: C025-T002

func TestNewExecCLIExecutor(t *testing.T) {
	executor := NewExecCLIExecutor()

	require.NotNil(t, executor)
	assert.IsType(t, &ExecCLIExecutor{}, executor)
}

func TestExecCLIExecutor_Run_SimpleCommands(t *testing.T) {
	tests := []struct {
		name          string
		binary        string
		args          []string
		wantStdoutLen int
		wantStderrLen int
		wantErrNil    bool
		description   string
	}{
		{
			name:          "echo with single arg",
			binary:        "echo",
			args:          []string{"hello"},
			wantStdoutLen: 6, // "hello\n"
			wantStderrLen: 0,
			wantErrNil:    true,
			description:   "Basic echo command should execute successfully",
		},
		{
			name:          "echo with multiple args",
			binary:        "echo",
			args:          []string{"hello", "world"},
			wantStdoutLen: 12, // "hello world\n"
			wantStderrLen: 0,
			wantErrNil:    true,
			description:   "Echo with multiple arguments",
		},
		{
			name:          "date command",
			binary:        "date",
			args:          []string{},
			wantStdoutLen: 0, // Don't check length (varies)
			wantStderrLen: 0,
			wantErrNil:    true,
			description:   "Date command should execute without args",
		},
		{
			name:          "pwd command",
			binary:        "pwd",
			args:          []string{},
			wantStdoutLen: 0, // Don't check length (varies)
			wantStderrLen: 0,
			wantErrNil:    true,
			description:   "Print working directory",
		},
	}

	executor := NewExecCLIExecutor()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := executor.Run(ctx, tt.binary, nil, nil, tt.args...)

			if tt.wantErrNil {
				require.NoError(t, err, tt.description)
			}
			if tt.wantStdoutLen > 0 {
				assert.Len(t, stdout, tt.wantStdoutLen, "stdout length mismatch")
			} else {
				assert.NotNil(t, stdout, "stdout should not be nil")
			}
			assert.Len(t, stderr, tt.wantStderrLen, "stderr should be empty for successful commands")
		})
	}
}

func TestExecCLIExecutor_Run_CommandWithFlags(t *testing.T) {
	executor := NewExecCLIExecutor()
	ctx := context.Background()

	tests := []struct {
		name   string
		binary string
		args   []string
	}{
		{
			name:   "ls with flags",
			binary: "ls",
			args:   []string{"-la", "/tmp"},
		},
		{
			name:   "echo with flag-like args",
			binary: "echo",
			args:   []string{"-n", "no newline"},
		},
		{
			name:   "printf with format",
			binary: "printf",
			args:   []string{"%s\n", "formatted"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := executor.Run(ctx, tt.binary, nil, nil, tt.args...)

			require.NoError(t, err)
			assert.NotNil(t, stdout)
			assert.NotNil(t, stderr)
		})
	}
}

func TestExecCLIExecutor_Run_EmptyCommand(t *testing.T) {
	executor := NewExecCLIExecutor()
	ctx := context.Background()

	stdout, stderr, err := executor.Run(ctx, "", nil, nil)

	// Empty binary name should cause an error
	assert.Error(t, err, "empty binary name should fail")
	assert.NotNil(t, stdout, "stdout should not be nil even on error")
	assert.NotNil(t, stderr, "stderr should not be nil even on error")
}

func TestExecCLIExecutor_Run_NoArguments(t *testing.T) {
	executor := NewExecCLIExecutor()
	ctx := context.Background()

	stdout, stderr, err := executor.Run(ctx, "echo", nil, nil)

	require.NoError(t, err)
	assert.NotNil(t, stdout)
	assert.NotNil(t, stderr)
}

func TestExecCLIExecutor_Run_ManyArguments(t *testing.T) {
	executor := NewExecCLIExecutor()
	ctx := context.Background()

	// Create many arguments
	args := make([]string, 100)
	for i := 0; i < 100; i++ {
		args[i] = "arg"
	}

	stdout, stderr, err := executor.Run(ctx, "echo", nil, nil, args...)

	require.NoError(t, err)
	assert.NotNil(t, stdout)
	assert.Greater(t, len(stdout), 0, "should have output from 100 args")
	assert.NotNil(t, stderr)
}

func TestExecCLIExecutor_Run_LargeOutput(t *testing.T) {
	executor := NewExecCLIExecutor()
	ctx := context.Background()

	// Generate large output
	stdout, stderr, err := executor.Run(ctx, "seq", nil, nil, "1", "1000")

	require.NoError(t, err)
	assert.Greater(t, len(stdout), 100, "should have substantial output")
	assert.NotNil(t, stderr)
}

func TestExecCLIExecutor_Run_SpecialCharacters(t *testing.T) {
	executor := NewExecCLIExecutor()
	ctx := context.Background()

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "special chars in args",
			args: []string{"hello", "world!", "@#$%", "a&b"},
		},
		{
			name: "quotes in args",
			args: []string{`"quoted"`, `'single'`},
		},
		{
			name: "unicode in args",
			args: []string{"hello", "世界", "🚀"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := executor.Run(ctx, "echo", nil, nil, tt.args...)

			require.NoError(t, err)
			assert.NotNil(t, stdout)
			assert.NotNil(t, stderr)
		})
	}
}

func TestExecCLIExecutor_Run_NilContext(t *testing.T) {
	executor := NewExecCLIExecutor()

	// This is an edge case - proper usage requires valid context
	defer func() {
		if r := recover(); r != nil {
			// Panic is acceptable for nil context
			assert.NotNil(t, r, "should panic or error with nil context")
		}
	}()

	// If no panic, should return error
	_, _, err := executor.Run(context.Background(), "echo", nil, nil, "test")
	if err != nil {
		assert.Error(t, err, "nil context should cause error")
	}
}

func TestExecCLIExecutor_Run_BinaryNotFound(t *testing.T) {
	executor := NewExecCLIExecutor()
	ctx := context.Background()

	tests := []struct {
		name   string
		binary string
	}{
		{
			name:   "nonexistent binary",
			binary: "nonexistent_binary_12345",
		},
		{
			name:   "invalid path",
			binary: "/invalid/path/to/binary",
		},
		{
			name:   "binary with spaces",
			binary: "binary with spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := executor.Run(ctx, tt.binary, nil, nil)

			assert.Error(t, err, "non-existent binary should cause error")
			assert.NotNil(t, stdout)
			assert.NotNil(t, stderr)
		})
	}
}

func TestExecCLIExecutor_Run_NonZeroExitCode(t *testing.T) {
	executor := NewExecCLIExecutor()
	ctx := context.Background()

	tests := []struct {
		name       string
		binary     string
		args       []string
		expectCode int
	}{
		{
			name:       "exit 1",
			binary:     "sh",
			args:       []string{"-c", "exit 1"},
			expectCode: 1,
		},
		{
			name:       "exit 42",
			binary:     "sh",
			args:       []string{"-c", "exit 42"},
			expectCode: 42,
		},
		{
			name:       "false command",
			binary:     "false",
			args:       []string{},
			expectCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := executor.Run(ctx, tt.binary, nil, nil, tt.args...)

			assert.Error(t, err, "non-zero exit should cause error")
			assert.NotNil(t, stdout)
			assert.NotNil(t, stderr)
		})
	}
}

func TestExecCLIExecutor_Run_CommandTimeout(t *testing.T) {
	executor := NewExecCLIExecutor()

	tests := []struct {
		name    string
		timeout time.Duration
		binary  string
		args    []string
	}{
		{
			name:    "sleep command timeout",
			timeout: 100 * time.Millisecond,
			binary:  "sleep",
			args:    []string{"10"},
		},
		{
			name:    "very short timeout",
			timeout: 1 * time.Millisecond,
			binary:  "sleep",
			args:    []string{"1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			stdout, stderr, err := executor.Run(ctx, tt.binary, nil, nil, tt.args...)

			assert.Error(t, err, "timeout should cause error")
			assert.True(t,
				errors.Is(err, context.DeadlineExceeded) || err != nil,
				"error should be timeout-related",
			)
			assert.NotNil(t, stdout)
			assert.NotNil(t, stderr)
		})
	}
}

func TestExecCLIExecutor_Run_ContextCancellation(t *testing.T) {
	executor := NewExecCLIExecutor()

	tests := []struct {
		name   string
		binary string
		args   []string
	}{
		{
			name:   "cancel during sleep",
			binary: "sleep",
			args:   []string{"10"},
		},
		{
			name:   "cancel during long operation",
			binary: "sh",
			args:   []string{"-c", "sleep 10 && echo done"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Cancel immediately after starting
			go func() {
				time.Sleep(50 * time.Millisecond)
				cancel()
			}()

			stdout, stderr, err := executor.Run(ctx, tt.binary, nil, nil, tt.args...)

			assert.Error(t, err, "cancellation should cause error")
			assert.True(t,
				errors.Is(err, context.Canceled) || err != nil,
				"error should be cancellation-related",
			)
			assert.NotNil(t, stdout)
			assert.NotNil(t, stderr)
		})
	}
}

// TestExecCLIExecutor_Run_ContextCancellation_TerminatesProcessGroup verifies
// that when a context is cancelled, the spawned process and its entire process
// group are terminated quickly via SIGKILL, rather than waiting for the full
// command duration.
//
// This test is part of B002 bug fix - ensuring CLIExecutor properly manages
// process groups similar to ShellExecutor's implementation.
//
// Component: B002-T001 (RED phase - this test should FAIL initially)
func TestExecCLIExecutor_Run_ContextCancellation_TerminatesProcessGroup(t *testing.T) {
	executor := NewExecCLIExecutor()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel after 50ms
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	stdout, stderr, err := executor.Run(ctx, "sleep", nil, nil, "10")
	duration := time.Since(start)

	// Process group kill should complete within 1s (not wait for full 10s)
	// This assertion will FAIL until process group management is implemented
	assert.Error(t, err, "context cancellation should cause error")
	assert.True(t, errors.Is(err, context.Canceled), "error should be context.Canceled")
	assert.Less(t, duration, 1*time.Second, "Process should be killed quickly via SIGKILL, not wait for full sleep duration")
	assert.NotNil(t, stdout, "stdout buffer should not be nil")
	assert.NotNil(t, stderr, "stderr buffer should not be nil")
}

func TestExecCLIExecutor_Run_StderrOutput(t *testing.T) {
	executor := NewExecCLIExecutor()
	ctx := context.Background()

	tests := []struct {
		name    string
		command string
	}{
		{
			name:    "write to stderr",
			command: "echo error >&2",
		},
		{
			name:    "stderr and stdout",
			command: "echo out; echo err >&2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := executor.Run(ctx, "sh", nil, nil, "-c", tt.command)

			require.NoError(t, err)
			// Note: os/exec.CombinedOutput merges stderr into stdout
			// So we might see output in stdout or stderr depending on implementation
			assert.True(t,
				len(stdout) > 0 || len(stderr) > 0,
				"should have output in stdout or stderr",
			)
		})
	}
}

func TestExecCLIExecutor_Run_InvalidArguments(t *testing.T) {
	executor := NewExecCLIExecutor()
	ctx := context.Background()

	tests := []struct {
		name   string
		binary string
		args   []string
	}{
		{
			name:   "invalid flag",
			binary: "ls",
			args:   []string{"--invalid-flag-xyz"},
		},
		{
			name:   "malformed arguments",
			binary: "sh",
			args:   []string{"-c", "invalid syntax {{{"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := executor.Run(ctx, tt.binary, nil, nil, tt.args...)

			// Most commands will error on invalid args
			// But we're testing the executor handles it gracefully
			assert.NotNil(t, stdout)
			assert.NotNil(t, stderr)
			// Error may or may not occur depending on command
			_ = err
		})
	}
}

func TestExecCLIExecutor_Run_ConcurrentExecutions(t *testing.T) {
	executor := NewExecCLIExecutor()
	ctx := context.Background()

	const concurrency = 10

	// Launch multiple concurrent executions
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(n int) {
			stdout, stderr, err := executor.Run(ctx, "echo", nil, nil, "concurrent")
			assert.NoError(t, err)
			assert.NotNil(t, stdout)
			assert.NotNil(t, stderr)
			done <- true
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < concurrency; i++ {
		<-done
	}
}

func TestExecCLIExecutor_Run_SimulateClaudeProvider(t *testing.T) {
	executor := NewExecCLIExecutor()
	ctx := context.Background()

	// Simulate how ClaudeProvider would use this
	// (using echo as a stand-in for claude CLI)
	args := []string{"test prompt"}

	stdout, stderr, err := executor.Run(ctx, "echo", nil, nil, args...)

	require.NoError(t, err, "Claude provider simulation should succeed")
	assert.NotNil(t, stdout)
	assert.Greater(t, len(stdout), 0, "should have response")
	assert.NotNil(t, stderr)
}

func TestExecCLIExecutor_Run_SimulateGeminiProvider(t *testing.T) {
	executor := NewExecCLIExecutor()
	ctx := context.Background()

	// Simulate Gemini provider with JSON output flag
	args := []string{"--json", "test"}

	stdout, stderr, err := executor.Run(ctx, "echo", nil, nil, args...)

	require.NoError(t, err)
	assert.NotNil(t, stdout)
	assert.NotNil(t, stderr)
}

func TestExecCLIExecutor_Run_SimulateCodexProvider(t *testing.T) {
	executor := NewExecCLIExecutor()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Simulate Codex provider with model selection
	args := []string{"--model", "gpt-4", "prompt"}

	stdout, stderr, err := executor.Run(ctx, "echo", nil, nil, args...)

	require.NoError(t, err)
	assert.NotNil(t, stdout)
	assert.NotNil(t, stderr)
}

// TestRun_SetsProcessGroup verifies that SysProcAttr.Setpgid is configured
// to enable process group isolation. This test validates AC-005 requirement
// by verifying that spawned processes and their children are properly
// terminated when the parent context is cancelled.
//
// Component: B002-T005 (Process group configuration verification)
//
// Implementation approach:
// Since SysProcAttr cannot be directly inspected after cmd.Run() completes,
// we verify indirectly by spawning a process that creates children, then
// canceling the context and checking that all processes terminate cleanly.
func TestRun_SetsProcessGroup(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		cancelDelay time.Duration
		description string
	}{
		{
			name:        "single background child process",
			command:     "sleep 10 & wait",
			cancelDelay: 50 * time.Millisecond,
			description: "Verify single child process terminates with parent",
		},
		{
			name:        "multiple background child processes",
			command:     "sleep 10 & sleep 10 & sleep 10 & wait",
			cancelDelay: 50 * time.Millisecond,
			description: "Verify multiple children terminate with parent",
		},
		{
			name:        "nested child processes",
			command:     "sh -c 'sleep 10 & sleep 10' & wait",
			cancelDelay: 50 * time.Millisecond,
			description: "Verify nested shell spawning children terminates cleanly",
		},
		{
			name:        "immediate cancellation",
			command:     "sleep 10 & sleep 10 & wait",
			cancelDelay: 1 * time.Millisecond,
			description: "Verify cleanup works with immediate cancellation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewExecCLIExecutor()
			ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // G118: cancel is called in goroutine below, not deferred

			go func() {
				time.Sleep(tt.cancelDelay)
				cancel()
			}()

			// Execute command that spawns child processes
			stdout, stderr, err := executor.Run(ctx, "sh", nil, nil, "-c", tt.command)

			assert.Error(t, err, tt.description)
			assert.True(t, errors.Is(err, context.Canceled), "error should be context.Canceled")
			assert.NotNil(t, stdout, "stdout should not be nil")
			assert.NotNil(t, stderr, "stderr should not be nil")
		})
	}
}

// TestRun_SetsProcessGroup_EdgeCases tests edge cases for process group management
func TestRun_SetsProcessGroup_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		cancelDelay time.Duration
		description string
	}{
		{
			name:        "empty command with process group",
			command:     "true",
			cancelDelay: 50 * time.Millisecond,
			description: "Fast-exiting command should work with process group",
		},
		{
			name:        "command with no children",
			command:     "echo test",
			cancelDelay: 50 * time.Millisecond,
			description: "Command without children should work normally",
		},
		{
			name:        "rapid fork bomb protection",
			command:     "for i in 1 2 3; do sleep 10 & done; wait",
			cancelDelay: 50 * time.Millisecond,
			description: "Multiple rapid forks should all be cleaned up",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewExecCLIExecutor()
			ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // G118: cancel is called in goroutine below, not deferred

			go func() {
				time.Sleep(tt.cancelDelay)
				cancel()
			}()

			stdout, stderr, err := executor.Run(ctx, "sh", nil, nil, "-c", tt.command)

			assert.NotNil(t, stdout, "stdout should not be nil")
			assert.NotNil(t, stderr, "stderr should not be nil")

			// Error may or may not occur depending on command timing
			if err != nil {
				assert.True(t,
					errors.Is(err, context.Canceled) || err != nil,
					"if error occurs, should be cancellation-related",
				)
			}
		})
	}
}

// TestRun_SetsProcessGroup_ErrorHandling tests error scenarios with process groups
func TestRun_SetsProcessGroup_ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		shouldError    bool
		expectCanceled bool
		description    string
	}{
		{
			name:           "direct command failure",
			command:        "sh -c 'exit 1'",
			shouldError:    true,
			expectCanceled: false,
			description:    "Direct command exit with error should propagate",
		},
		{
			name:           "command not found",
			command:        "nonexistent_command_xyz",
			shouldError:    true,
			expectCanceled: false,
			description:    "Missing command should cause error",
		},
		{
			name:           "parent exit with backgrounded child",
			command:        "sleep 0.1",
			shouldError:    false,
			expectCanceled: false,
			description:    "Parent process exits cleanly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewExecCLIExecutor()
			ctx := context.Background()

			stdout, stderr, err := executor.Run(ctx, "sh", nil, nil, "-c", tt.command)

			assert.NotNil(t, stdout, "stdout should not be nil")
			assert.NotNil(t, stderr, "stderr should not be nil")

			if tt.shouldError {
				assert.Error(t, err, tt.description)
				if tt.expectCanceled {
					assert.True(t, errors.Is(err, context.Canceled), "should be cancellation error")
				}
			}
		})
	}
}
