package ports_test

import (
	"context"
	"errors"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
)

// Component: cli_executor_port
// Task: C025-T001

// mockCLIExecutor is a test implementation of CLIExecutor interface
type mockCLIExecutor struct {
	runFunc    func(ctx context.Context, name string, args ...string) (stdout, stderr []byte, err error)
	runCalled  int
	lastCalled struct {
		name string
		args []string
	}
}

func newMockCLIExecutor() *mockCLIExecutor {
	return &mockCLIExecutor{
		runFunc: func(ctx context.Context, name string, args ...string) (stdout, stderr []byte, err error) {
			return []byte("mock stdout"), []byte(""), nil
		},
	}
}

func (m *mockCLIExecutor) Run(ctx context.Context, name string, args ...string) (stdout, stderr []byte, err error) {
	m.runCalled++
	m.lastCalled.name = name
	m.lastCalled.args = args
	return m.runFunc(ctx, name, args...)
}

func TestCLIExecutorInterface(t *testing.T) {
	// Verify interface compliance
	var _ ports.CLIExecutor = (*mockCLIExecutor)(nil)
}

func TestCLIExecutor_Run_HappyPath(t *testing.T) {
	tests := []struct {
		name           string
		binaryName     string
		args           []string
		expectedStdout string
		expectedStderr string
	}{
		{
			name:           "simple command no args",
			binaryName:     "echo",
			args:           []string{},
			expectedStdout: "mock stdout",
			expectedStderr: "",
		},
		{
			name:           "command with single arg",
			binaryName:     "claude",
			args:           []string{"--version"},
			expectedStdout: "mock stdout",
			expectedStderr: "",
		},
		{
			name:           "command with multiple args",
			binaryName:     "claude",
			args:           []string{"-p", "hello world", "--model", "sonnet"},
			expectedStdout: "mock stdout",
			expectedStderr: "",
		},
		{
			name:           "command with flags and values",
			binaryName:     "gemini",
			args:           []string{"--json", "--temperature", "0.7"},
			expectedStdout: "mock stdout",
			expectedStderr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockCLIExecutor()
			ctx := context.Background()

			stdout, stderr, err := mock.Run(ctx, tt.binaryName, tt.args...)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if string(stdout) != tt.expectedStdout {
				t.Errorf("expected stdout '%s', got '%s'", tt.expectedStdout, string(stdout))
			}
			if string(stderr) != tt.expectedStderr {
				t.Errorf("expected stderr '%s', got '%s'", tt.expectedStderr, string(stderr))
			}
			if mock.runCalled != 1 {
				t.Errorf("expected Run to be called once, was called %d times", mock.runCalled)
			}
			if mock.lastCalled.name != tt.binaryName {
				t.Errorf("expected binary name '%s', got '%s'", tt.binaryName, mock.lastCalled.name)
			}
		})
	}
}

func TestCLIExecutor_Run_WithStderr(t *testing.T) {
	mock := newMockCLIExecutor()
	mock.runFunc = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		return []byte("output"), []byte("warning message"), nil
	}
	ctx := context.Background()

	stdout, stderr, err := mock.Run(ctx, "test-binary", "--flag")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if string(stdout) != "output" {
		t.Errorf("expected stdout 'output', got '%s'", string(stdout))
	}
	if string(stderr) != "warning message" {
		t.Errorf("expected stderr 'warning message', got '%s'", string(stderr))
	}
}

func TestCLIExecutor_Run_EmptyBinaryName(t *testing.T) {
	mock := newMockCLIExecutor()
	mock.runFunc = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		if name == "" {
			return nil, nil, errors.New("binary name cannot be empty")
		}
		return []byte("output"), []byte(""), nil
	}
	ctx := context.Background()

	_, _, err := mock.Run(ctx, "", "--arg")

	if err == nil {
		t.Error("expected error for empty binary name, got nil")
	}
	if err.Error() != "binary name cannot be empty" {
		t.Errorf("expected error 'binary name cannot be empty', got '%v'", err)
	}
}

func TestCLIExecutor_Run_NoArgs(t *testing.T) {
	mock := newMockCLIExecutor()
	ctx := context.Background()

	stdout, stderr, err := mock.Run(ctx, "test-binary")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(mock.lastCalled.args) != 0 {
		t.Errorf("expected no args, got %d args", len(mock.lastCalled.args))
	}
	if stdout == nil {
		t.Error("expected stdout to be non-nil")
	}
	if stderr == nil {
		t.Error("expected stderr to be non-nil")
	}
}

func TestCLIExecutor_Run_EmptyStdout(t *testing.T) {
	mock := newMockCLIExecutor()
	mock.runFunc = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		return []byte(""), []byte(""), nil
	}
	ctx := context.Background()

	stdout, stderr, err := mock.Run(ctx, "test-binary")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(stdout) != 0 {
		t.Errorf("expected empty stdout, got %d bytes", len(stdout))
	}
	if len(stderr) != 0 {
		t.Errorf("expected empty stderr, got %d bytes", len(stderr))
	}
}

func TestCLIExecutor_Run_LargeOutput(t *testing.T) {
	mock := newMockCLIExecutor()
	largeOutput := make([]byte, 1024*1024) // 1MB
	for i := range largeOutput {
		largeOutput[i] = 'A'
	}
	mock.runFunc = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		return largeOutput, []byte(""), nil
	}
	ctx := context.Background()

	stdout, _, err := mock.Run(ctx, "test-binary")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(stdout) != len(largeOutput) {
		t.Errorf("expected stdout length %d, got %d", len(largeOutput), len(stdout))
	}
}

func TestCLIExecutor_Run_ExecutionError(t *testing.T) {
	tests := []struct {
		name          string
		mockError     error
		expectedError string
	}{
		{
			name:          "binary not found",
			mockError:     errors.New("executable file not found in $PATH"),
			expectedError: "executable file not found in $PATH",
		},
		{
			name:          "permission denied",
			mockError:     errors.New("permission denied"),
			expectedError: "permission denied",
		},
		{
			name:          "exit code 1",
			mockError:     errors.New("exit status 1"),
			expectedError: "exit status 1",
		},
		{
			name:          "generic execution error",
			mockError:     errors.New("command failed"),
			expectedError: "command failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockCLIExecutor()
			mock.runFunc = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
				return nil, []byte("error output"), tt.mockError
			}
			ctx := context.Background()

			stdout, stderr, err := mock.Run(ctx, "test-binary")

			if err == nil {
				t.Error("expected error, got nil")
			}
			if err.Error() != tt.expectedError {
				t.Errorf("expected error '%s', got '%s'", tt.expectedError, err.Error())
			}
			if stdout != nil {
				t.Errorf("expected nil stdout on error, got %d bytes", len(stdout))
			}
			if string(stderr) != "error output" {
				t.Errorf("expected stderr 'error output', got '%s'", string(stderr))
			}
		})
	}
}

func TestCLIExecutor_Run_ContextCancellation(t *testing.T) {
	mock := newMockCLIExecutor()
	mock.runFunc = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
			return []byte("output"), []byte(""), nil
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, _, err := mock.Run(ctx, "test-binary")

	if err == nil {
		t.Error("expected error for cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got: %v", err)
	}
}

func TestCLIExecutor_Run_ContextDeadline(t *testing.T) {
	mock := newMockCLIExecutor()
	mock.runFunc = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
			return []byte("output"), []byte(""), nil
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 0) // Already expired
	defer cancel()

	_, _, err := mock.Run(ctx, "test-binary")

	if err == nil {
		t.Error("expected error for expired context, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded error, got: %v", err)
	}
}

func TestCLIExecutor_Run_ErrorWithStderr(t *testing.T) {
	mock := newMockCLIExecutor()
	mock.runFunc = func(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
		return nil, []byte("detailed error message"), errors.New("command failed")
	}
	ctx := context.Background()

	stdout, stderr, err := mock.Run(ctx, "test-binary")

	if err == nil {
		t.Error("expected error, got nil")
	}
	if stdout != nil {
		t.Errorf("expected nil stdout on error, got %d bytes", len(stdout))
	}
	if string(stderr) != "detailed error message" {
		t.Errorf("expected stderr 'detailed error message', got '%s'", string(stderr))
	}
}

func TestCLIExecutor_Run_ArgumentOrder(t *testing.T) {
	mock := newMockCLIExecutor()
	ctx := context.Background()
	expectedArgs := []string{"--flag1", "value1", "--flag2", "value2"}

	_, _, _ = mock.Run(ctx, "test-binary", expectedArgs...)

	if len(mock.lastCalled.args) != len(expectedArgs) {
		t.Errorf("expected %d args, got %d", len(expectedArgs), len(mock.lastCalled.args))
	}
	for i, arg := range expectedArgs {
		if mock.lastCalled.args[i] != arg {
			t.Errorf("arg[%d]: expected '%s', got '%s'", i, arg, mock.lastCalled.args[i])
		}
	}
}

func TestCLIExecutor_Run_ArgumentsWithSpaces(t *testing.T) {
	mock := newMockCLIExecutor()
	ctx := context.Background()
	argsWithSpaces := []string{"-p", "hello world", "--data", "value with spaces"}

	_, _, _ = mock.Run(ctx, "test-binary", argsWithSpaces...)

	if len(mock.lastCalled.args) != len(argsWithSpaces) {
		t.Errorf("expected %d args, got %d", len(argsWithSpaces), len(mock.lastCalled.args))
	}
	for i, arg := range argsWithSpaces {
		if mock.lastCalled.args[i] != arg {
			t.Errorf("arg[%d]: expected '%s', got '%s'", i, arg, mock.lastCalled.args[i])
		}
	}
}

func TestCLIExecutor_Run_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"quotes", []string{`--text="quoted value"`}},
		{"newlines", []string{"--text", "line1\nline2"}},
		{"tabs", []string{"--text", "col1\tcol2"}},
		{"unicode", []string{"--text", "Hello 世界"}},
		{"symbols", []string{"--text", "!@#$%^&*()"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockCLIExecutor()
			ctx := context.Background()

			_, _, err := mock.Run(ctx, "test-binary", tt.args...)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(mock.lastCalled.args) != len(tt.args) {
				t.Errorf("expected %d args, got %d", len(tt.args), len(mock.lastCalled.args))
			}
		})
	}
}

func TestCLIExecutor_Run_MultipleCallsTracking(t *testing.T) {
	mock := newMockCLIExecutor()
	ctx := context.Background()

	_, _, _ = mock.Run(ctx, "first-binary", "arg1")
	_, _, _ = mock.Run(ctx, "second-binary", "arg2", "arg3")
	_, _, _ = mock.Run(ctx, "third-binary")

	if mock.runCalled != 3 {
		t.Errorf("expected 3 calls, got %d", mock.runCalled)
	}
	if mock.lastCalled.name != "third-binary" {
		t.Errorf("expected last called binary 'third-binary', got '%s'", mock.lastCalled.name)
	}
	if len(mock.lastCalled.args) != 0 {
		t.Errorf("expected no args in last call, got %d", len(mock.lastCalled.args))
	}
}
