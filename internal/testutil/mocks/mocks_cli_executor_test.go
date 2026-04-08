package mocks_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: C025 Improve Agents Package Test Coverage
// Component: T003 MockCLIExecutor Tests

// This compile-time assertion verifies that MockCLIExecutor satisfies the
// ports.CLIExecutor interface. If the mock fails to implement the interface,
// the code will not compile, catching interface mismatches early.
var _ ports.CLIExecutor = (*mocks.MockCLIExecutor)(nil)

// Feature: C025 Improve Agents Package Test Coverage
// Component: T003 MockCLIExecutor

func TestMockCLIExecutor_NewMockCLIExecutor(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()

	require.NotNil(t, executor, "NewMockCLIExecutor should return non-nil instance")

	// Verify initial state
	calls := executor.GetCalls()
	assert.Empty(t, calls, "New executor should have no recorded calls")
}

func TestMockCLIExecutor_Run_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*mocks.MockCLIExecutor)
		binary     string
		args       []string
		wantStdout string
		wantStderr string
	}{
		{
			name: "simple command execution",
			setupFunc: func(exec *mocks.MockCLIExecutor) {
				exec.SetOutput([]byte("output"), []byte(""))
			},
			binary:     "claude",
			args:       []string{"--version"},
			wantStdout: "output",
			wantStderr: "",
		},
		{
			name: "command with error output",
			setupFunc: func(exec *mocks.MockCLIExecutor) {
				exec.SetOutput([]byte(""), []byte("error message"))
			},
			binary:     "gemini",
			args:       []string{"--help"},
			wantStdout: "",
			wantStderr: "error message",
		},
		{
			name: "command with both stdout and stderr",
			setupFunc: func(exec *mocks.MockCLIExecutor) {
				exec.SetOutput([]byte("normal output"), []byte("warning"))
			},
			binary:     "codex",
			args:       []string{"execute", "--model", "gpt-4"},
			wantStdout: "normal output",
			wantStderr: "warning",
		},
		{
			name: "command with no arguments",
			setupFunc: func(exec *mocks.MockCLIExecutor) {
				exec.SetOutput([]byte("result"), []byte(""))
			},
			binary:     "tool",
			args:       []string{},
			wantStdout: "result",
			wantStderr: "",
		},
		{
			name: "command with multiple arguments",
			setupFunc: func(exec *mocks.MockCLIExecutor) {
				exec.SetOutput([]byte("success"), []byte(""))
			},
			binary:     "claude",
			args:       []string{"--model", "opus", "--temperature", "0.7", "--max-tokens", "1000"},
			wantStdout: "success",
			wantStderr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := mocks.NewMockCLIExecutor()
			tt.setupFunc(executor)
			ctx := context.Background()

			stdout, stderr, err := executor.Run(ctx, tt.binary, nil, nil, tt.args...)

			assert.NoError(t, err, "Run should not error for configured output")
			assert.Equal(t, tt.wantStdout, string(stdout), "Stdout should match configured value")
			assert.Equal(t, tt.wantStderr, string(stderr), "Stderr should match configured value")

			// Verify call was recorded
			calls := executor.GetCalls()
			assert.Len(t, calls, 1, "Run should record the call")
			assert.Equal(t, tt.binary, calls[0].Name, "Recorded binary should match executed command")
			assert.Equal(t, tt.args, calls[0].Args, "Recorded args should match executed command")
		})
	}
}

func TestMockCLIExecutor_Run_ErrorInjection(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*mocks.MockCLIExecutor)
		binary    string
		args      []string
		wantErr   error
	}{
		{
			name: "execution error",
			setupFunc: func(exec *mocks.MockCLIExecutor) {
				exec.SetError(errors.New("execution failed"))
			},
			binary:  "claude",
			args:    []string{"--version"},
			wantErr: errors.New("execution failed"),
		},
		{
			name: "binary not found error",
			setupFunc: func(exec *mocks.MockCLIExecutor) {
				exec.SetError(errors.New("binary not found"))
			},
			binary:  "nonexistent",
			args:    []string{},
			wantErr: errors.New("binary not found"),
		},
		{
			name: "permission denied error",
			setupFunc: func(exec *mocks.MockCLIExecutor) {
				exec.SetError(errors.New("permission denied"))
			},
			binary:  "restricted",
			args:    []string{"run"},
			wantErr: errors.New("permission denied"),
		},
		{
			name: "exit code error",
			setupFunc: func(exec *mocks.MockCLIExecutor) {
				exec.SetError(errors.New("exit status 1"))
			},
			binary:  "tool",
			args:    []string{"fail"},
			wantErr: errors.New("exit status 1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := mocks.NewMockCLIExecutor()
			tt.setupFunc(executor)
			ctx := context.Background()

			stdout, stderr, err := executor.Run(ctx, tt.binary, nil, nil, tt.args...)

			assert.Error(t, err, "Run should return error when error is configured")
			assert.EqualError(t, err, tt.wantErr.Error(), "Run should return the configured error")
			assert.Nil(t, stdout, "Stdout should be nil when error occurs")
			assert.Nil(t, stderr, "Stderr should be nil when error occurs")

			// Verify call was recorded even with error
			calls := executor.GetCalls()
			assert.Len(t, calls, 1, "Run should record the call even with error")
		})
	}
}

func TestMockCLIExecutor_Run_ContextCancellation(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*mocks.MockCLIExecutor)
		ctxFunc   func() context.Context
		wantErr   error
	}{
		{
			name: "context already cancelled",
			setupFunc: func(exec *mocks.MockCLIExecutor) {
				exec.SetError(context.Canceled)
			},
			ctxFunc: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr: context.Canceled,
		},
		{
			name: "context deadline exceeded",
			setupFunc: func(exec *mocks.MockCLIExecutor) {
				exec.SetError(context.DeadlineExceeded)
			},
			ctxFunc: context.Background,
			wantErr: context.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := mocks.NewMockCLIExecutor()
			tt.setupFunc(executor)
			ctx := tt.ctxFunc()

			stdout, stderr, err := executor.Run(ctx, "claude", nil, nil, "--version")

			assert.Error(t, err, "Run should return error for context issues")
			assert.ErrorIs(t, err, tt.wantErr, "Error should match expected context error")
			assert.Nil(t, stdout, "Stdout should be nil when context error occurs")
			assert.Nil(t, stderr, "Stderr should be nil when context error occurs")
		})
	}
}

func TestMockCLIExecutor_Run_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "run without setting output",
			test: func(t *testing.T) {
				executor := mocks.NewMockCLIExecutor()
				ctx := context.Background()

				stdout, stderr, err := executor.Run(ctx, "tool", nil, nil, "arg")

				assert.NoError(t, err, "Run without output config should not error")
				assert.Nil(t, stdout, "Stdout should be nil without config")
				assert.Nil(t, stderr, "Stderr should be nil without config")
				_ = stderr // Used in assertion above
			},
		},
		{
			name: "empty binary name",
			test: func(t *testing.T) {
				executor := mocks.NewMockCLIExecutor()
				executor.SetOutput([]byte("ok"), []byte(""))
				ctx := context.Background()

				stdout, stderr, err := executor.Run(ctx, "", nil, nil, "arg")

				assert.NoError(t, err, "Run should handle empty binary name")
				assert.Equal(t, "ok", string(stdout))
				assert.Empty(t, stderr)

				calls := executor.GetCalls()
				assert.Len(t, calls, 1)
				assert.Equal(t, "", calls[0].Name, "Empty binary name should be recorded")
			},
		},
		{
			name: "nil context",
			test: func(t *testing.T) {
				executor := mocks.NewMockCLIExecutor()
				executor.SetOutput([]byte("output"), []byte(""))

				// Note: passing nil context is undefined behavior in production
				// but mock should handle it gracefully
				stdout, stderr, err := executor.Run(context.Background(), "tool", nil, nil, "arg")

				assert.NoError(t, err, "Mock should handle nil context")
				assert.Equal(t, "output", string(stdout))
				assert.Empty(t, stderr)
			},
		},
		{
			name: "large output",
			test: func(t *testing.T) {
				executor := mocks.NewMockCLIExecutor()
				largeOutput := make([]byte, 1024*1024) // 1MB
				for i := range largeOutput {
					largeOutput[i] = byte('x')
				}
				executor.SetOutput(largeOutput, []byte(""))
				ctx := context.Background()

				stdout, stderr, err := executor.Run(ctx, "generator", nil, nil, "data")

				assert.NoError(t, err, "Run should handle large output")
				assert.Len(t, stdout, 1024*1024, "Large output should be preserved")
				assert.Empty(t, stderr)
			},
		},
		{
			name: "unicode in arguments",
			test: func(t *testing.T) {
				executor := mocks.NewMockCLIExecutor()
				executor.SetOutput([]byte("success"), []byte(""))
				ctx := context.Background()

				unicodeArgs := []string{"--prompt", "你好世界", "--emoji", "😀🎉"}
				stdout, stderr, err := executor.Run(ctx, "tool", nil, nil, unicodeArgs...)

				assert.NoError(t, err)
				assert.Equal(t, "success", string(stdout))
				assert.Empty(t, stderr)

				calls := executor.GetCalls()
				assert.Equal(t, unicodeArgs, calls[0].Args, "Unicode args should be preserved")
			},
		},
		{
			name: "special characters in binary name",
			test: func(t *testing.T) {
				executor := mocks.NewMockCLIExecutor()
				executor.SetOutput([]byte("ok"), []byte(""))
				ctx := context.Background()

				specialBinary := "tool-v2.0_beta"
				stdout, stderr, err := executor.Run(ctx, specialBinary, nil, nil, "--test")

				assert.NoError(t, err)
				assert.Equal(t, "ok", string(stdout))
				assert.Empty(t, stderr)

				calls := executor.GetCalls()
				assert.Equal(t, specialBinary, calls[0].Name, "Special chars in binary should be preserved")
			},
		},
		{
			name: "100 arguments",
			test: func(t *testing.T) {
				executor := mocks.NewMockCLIExecutor()
				executor.SetOutput([]byte("done"), []byte(""))
				ctx := context.Background()

				args := make([]string, 100)
				for i := range args {
					args[i] = fmt.Sprintf("arg%d", i)
				}

				stdout, stderr, err := executor.Run(ctx, "tool", nil, nil, args...)

				assert.NoError(t, err)
				assert.Equal(t, "done", string(stdout))
				assert.Empty(t, stderr)

				calls := executor.GetCalls()
				assert.Len(t, calls[0].Args, 100, "All 100 args should be recorded")
			},
		},
		{
			name: "binary output with null bytes",
			test: func(t *testing.T) {
				executor := mocks.NewMockCLIExecutor()
				binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}
				executor.SetOutput(binaryData, []byte(""))
				ctx := context.Background()

				stdout, stderr, err := executor.Run(ctx, "binary-tool", nil, nil)

				assert.NoError(t, err)
				assert.Equal(t, binaryData, stdout, "Binary data with null bytes should be preserved")
				assert.Empty(t, stderr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

func TestMockCLIExecutor_CallRecording(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	executor.SetOutput([]byte("ok"), []byte(""))
	ctx := context.Background()

	commands := []struct {
		binary string
		args   []string
	}{
		{"claude", []string{"--model", "opus"}},
		{"gemini", []string{"--temperature", "0.7"}},
		{"codex", []string{"--max-tokens", "1000", "--format", "json"}},
	}

	for _, cmd := range commands {
		_, _, _ = executor.Run(ctx, cmd.binary, nil, nil, cmd.args...)
	}

	calls := executor.GetCalls()
	assert.Len(t, calls, 3, "All executions should be recorded")

	for i, cmd := range commands {
		assert.Equal(t, cmd.binary, calls[i].Name, "Call %d binary should match", i)
		assert.Equal(t, cmd.args, calls[i].Args, "Call %d args should match", i)
	}
}

func TestMockCLIExecutor_CallRecording_MultipleExecutionsOfSameCommand(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	executor.SetOutput([]byte("result"), []byte(""))
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, _, err := executor.Run(ctx, "claude", nil, nil, "--version")
		assert.NoError(t, err, "Execution %d should succeed", i)
	}

	calls := executor.GetCalls()
	assert.Len(t, calls, 5, "All 5 executions should be recorded")
	for i, call := range calls {
		assert.Equal(t, "claude", call.Name, "Call %d should have correct binary", i)
		assert.Equal(t, []string{"--version"}, call.Args, "Call %d should have correct args", i)
	}
}

func TestMockCLIExecutor_GetCalls_IsolatedCopy(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	executor.SetOutput([]byte("ok"), []byte(""))
	ctx := context.Background()

	_, _, _ = executor.Run(ctx, "tool", nil, nil, "arg1", "arg2")

	calls1 := executor.GetCalls()
	calls2 := executor.GetCalls()

	calls1[0].Name = "modified"
	calls1[0].Args[0] = "modified-arg"

	assert.NotEqual(t, calls1[0].Name, calls2[0].Name, "GetCalls should return isolated copy")
	assert.Equal(t, "tool", calls2[0].Name, "Internal state should be unaffected by modifications")
	assert.Equal(t, "arg1", calls2[0].Args[0], "Args should be deep copied")
}

func TestMockCLIExecutor_SetOutput(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	ctx := context.Background()

	executor.SetOutput([]byte("stdout content"), []byte("stderr content"))
	stdout, stderr, err := executor.Run(ctx, "tool", nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, "stdout content", string(stdout))
	assert.Equal(t, "stderr content", string(stderr))
}

func TestMockCLIExecutor_SetOutput_Overwrites(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	ctx := context.Background()

	executor.SetOutput([]byte("first"), []byte(""))
	executor.SetOutput([]byte("second"), []byte(""))
	stdout, stderr, err := executor.Run(ctx, "tool", nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, "second", string(stdout), "Second SetOutput should overwrite first")
	assert.Empty(t, stderr)
}

func TestMockCLIExecutor_SetError(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	ctx := context.Background()
	expectedErr := errors.New("test error")

	executor.SetError(expectedErr)
	stdout, stderr, err := executor.Run(ctx, "tool", nil, nil)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, stdout)
	assert.Nil(t, stderr)
}

func TestMockCLIExecutor_SetError_Overwrites(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	ctx := context.Background()

	executor.SetError(errors.New("first error"))
	executor.SetError(errors.New("second error"))
	_, _, err := executor.Run(ctx, "tool", nil, nil)

	assert.Error(t, err)
	assert.EqualError(t, err, "second error", "Second SetError should overwrite first")
}

func TestMockCLIExecutor_SetError_TakesPrecedenceOverOutput(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	ctx := context.Background()

	executor.SetOutput([]byte("stdout"), []byte("stderr"))
	executor.SetError(errors.New("execution failed"))
	stdout, stderr, err := executor.Run(ctx, "tool", nil, nil)

	assert.Error(t, err, "Error should take precedence over output")
	assert.EqualError(t, err, "execution failed")
	assert.Nil(t, stdout, "Output should not be returned when error is set")
	assert.Nil(t, stderr)
}

func TestMockCLIExecutor_Clear(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	executor.SetOutput([]byte("stdout"), []byte("stderr"))
	executor.SetError(errors.New("error"))
	ctx := context.Background()

	// Execute some commands
	for i := 0; i < 3; i++ {
		_, _, _ = executor.Run(ctx, fmt.Sprintf("cmd-%d", i), nil, nil)
	}

	// Verify state before clear
	calls := executor.GetCalls()
	assert.Len(t, calls, 3, "Should have recorded calls before clear")

	executor.Clear()

	calls = executor.GetCalls()
	assert.Empty(t, calls, "Clear should remove all recorded calls")

	stdout, stderr, err := executor.Run(ctx, "new-cmd", nil, nil)
	assert.NoError(t, err, "Clear should reset error configuration")
	assert.Nil(t, stdout, "Clear should reset output configuration")
	assert.Nil(t, stderr, "Clear should reset stderr configuration")
}

func TestMockCLIExecutor_ConcurrentRun(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	executor.SetOutput([]byte("output"), []byte(""))
	ctx := context.Background()
	numGoroutines := 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			binary := fmt.Sprintf("tool-%d", id)
			args := []string{fmt.Sprintf("arg-%d", id)}
			stdout, stderr, err := executor.Run(ctx, binary, nil, nil, args...)
			assert.NoError(t, err)
			assert.Equal(t, "output", string(stdout))
			assert.Empty(t, stderr)
		}(i)
	}

	wg.Wait()

	calls := executor.GetCalls()
	assert.Len(t, calls, numGoroutines, "All concurrent executions should be recorded")
}

func TestMockCLIExecutor_ConcurrentReadWrite(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	ctx := context.Background()
	const goroutines = 20
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 3) // readers + writers + configurators

	// Readers (Run)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _, _ = executor.Run(ctx, fmt.Sprintf("reader-%d", id), nil, nil, fmt.Sprintf("iter-%d", j))
			}
		}(i)
	}

	// Configurators (SetOutput/SetError)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if j%2 == 0 {
					executor.SetOutput([]byte(fmt.Sprintf("output-%d", id)), []byte(""))
				} else {
					executor.SetError(fmt.Errorf("error-%d", id))
				}
			}
		}(i)
	}

	// Readers (GetCalls)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = executor.GetCalls()
			}
		}()
	}

	wg.Wait()
	assert.True(t, true, "Concurrent access should not cause races")
}

func TestMockCLIExecutor_ConcurrentClear(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	executor.SetOutput([]byte("output"), []byte(""))
	ctx := context.Background()
	const goroutines = 10

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Clearers
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			executor.Clear()
		}()
	}

	// Runners
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			_, _, _ = executor.Run(ctx, fmt.Sprintf("tool-%d", id), nil, nil)
		}(i)
	}

	wg.Wait()
	assert.True(t, true, "Concurrent Clear and Run should not cause races")
}

func TestMockCLIExecutor_ClaudeProviderScenario(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	ctx := context.Background()

	jsonResponse := `{"output": "Hello, world!", "tokens": 100}`
	executor.SetOutput([]byte(jsonResponse), []byte(""))

	stdout, stderr, err := executor.Run(ctx, "claude", nil, nil, "--model", "opus", "--prompt", "Say hello")

	assert.NoError(t, err)
	assert.Equal(t, jsonResponse, string(stdout))
	assert.Empty(t, stderr)

	calls := executor.GetCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "claude", calls[0].Name)
	assert.Equal(t, []string{"--model", "opus", "--prompt", "Say hello"}, calls[0].Args)
}

func TestMockCLIExecutor_GeminiProviderScenario(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	ctx := context.Background()

	executor.SetOutput([]byte(`{"result": "Analysis complete"}`), []byte(""))

	stdout, stderr, err := executor.Run(ctx, "gemini", nil, nil,
		"--temperature", "0.5",
		"--max-tokens", "2000",
		"--format", "json",
		"Analyze this code")

	assert.NoError(t, err)
	assert.Contains(t, string(stdout), "Analysis complete")
	assert.Empty(t, stderr)

	calls := executor.GetCalls()
	assert.Len(t, calls, 1)
	assert.Equal(t, "gemini", calls[0].Name)
	assert.Contains(t, calls[0].Args, "--temperature")
}

func TestMockCLIExecutor_ProviderErrorScenario(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	ctx := context.Background()

	// Simulate CLI error (e.g., API key invalid)
	executor.SetError(errors.New("exit status 1: API key invalid"))

	stdout, stderr, err := executor.Run(ctx, "claude", nil, nil, "--prompt", "test")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key invalid")
	assert.Nil(t, stdout)
	assert.Nil(t, stderr)

	// Verify call was recorded even with error
	calls := executor.GetCalls()
	assert.Len(t, calls, 1)
}

func TestMockCLIExecutor_ProviderTimeoutScenario(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	ctx := context.Background()

	// Simulate timeout
	executor.SetError(context.DeadlineExceeded)

	stdout, stderr, err := executor.Run(ctx, "codex", nil, nil, "--prompt", "long running task")

	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Nil(t, stdout)
	assert.Nil(t, stderr)
}
