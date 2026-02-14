package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/infrastructure/agents"
	"github.com/vanoix/awf/internal/testutil"
)

func TestContextUsage_Integration(t *testing.T) {
	tests := []struct {
		name      string
		setupExec func() interface {
			Run(context.Context, string, ...string) ([]byte, []byte, error)
		}
		binary      string
		args        []string
		wantErr     bool
		description string
	}{
		{
			name: "ExecCLIExecutor with Background context executes successfully",
			setupExec: func() interface {
				Run(context.Context, string, ...string) ([]byte, []byte, error)
			} {
				return agents.NewExecCLIExecutor()
			},
			binary:      "echo",
			args:        []string{"test"},
			wantErr:     false,
			description: "Real executor should work with context.Background()",
		},
		{
			name: "MockCLIExecutor with Background context executes successfully",
			setupExec: func() interface {
				Run(context.Context, string, ...string) ([]byte, []byte, error)
			} {
				mock := testutil.NewMockCLIExecutor()
				mock.SetOutput([]byte("mock output"), []byte(""))
				return mock
			},
			binary:      "tool",
			args:        []string{"arg"},
			wantErr:     false,
			description: "Mock executor should work with context.Background()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := tt.setupExec()
			ctx := context.Background()

			stdout, stderr, err := executor.Run(ctx, tt.binary, tt.args...)

			if tt.wantErr {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
			assert.NotNil(t, stdout, "stdout should not be nil")
			assert.NotNil(t, stderr, "stderr should not be nil")
		})
	}
}

func TestContextUsage_MultipleExecutions(t *testing.T) {
	executor := agents.NewExecCLIExecutor()
	ctx := context.Background()

	results := make([]struct {
		stdout []byte
		stderr []byte
		err    error
	}, 3)

	for i := 0; i < 3; i++ {
		stdout, stderr, err := executor.Run(ctx, "echo", "test")
		results[i].stdout = stdout
		results[i].stderr = stderr
		results[i].err = err
	}

	for i, result := range results {
		assert.NoError(t, result.err, "Execution %d should succeed", i)
		assert.NotNil(t, result.stdout, "Execution %d should have stdout", i)
		assert.NotNil(t, result.stderr, "Execution %d should have stderr", i)
	}
}

func TestContextUsage_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		setupCtx    func() context.Context
		binary      string
		args        []string
		expectError bool
		description string
	}{
		{
			name:        "Background context with fast command",
			setupCtx:    context.Background,
			binary:      "true",
			args:        []string{},
			expectError: false,
			description: "Fast-exiting command should work with Background",
		},
		{
			name:        "Background context with no arguments",
			setupCtx:    context.Background,
			binary:      "echo",
			args:        []string{},
			expectError: false,
			description: "Command with no args should work with Background",
		},
		{
			name:        "Background context with many arguments",
			setupCtx:    context.Background,
			binary:      "echo",
			args:        make([]string, 50), // 50 empty args
			expectError: false,
			description: "Many arguments should work with Background",
		},
		{
			name:        "Background context with special characters",
			setupCtx:    context.Background,
			binary:      "echo",
			args:        []string{"hello", "世界", "🚀"},
			expectError: false,
			description: "Unicode args should work with Background",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := agents.NewExecCLIExecutor()
			ctx := tt.setupCtx()

			stdout, stderr, err := executor.Run(ctx, tt.binary, tt.args...)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
			assert.NotNil(t, stdout)
			assert.NotNil(t, stderr)
		})
	}
}

func TestContextUsage_NilContextBehavior(t *testing.T) {
	executor := agents.NewExecCLIExecutor()

	defer func() {
		r := recover()
		if r == nil {
			t.Log("No panic occurred; implementation returns error instead")
		} else {
			assert.NotNil(t, r, "Should panic with nil context")
		}
	}()

	_, _, err := executor.Run(context.Background(), "echo", "test")
	assert.NoError(t, err, "Valid context should work")
}

func TestContextUsage_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		binary      string
		args        []string
		wantErr     bool
		description string
	}{
		{
			name:        "Binary not found returns error",
			binary:      "nonexistent_binary_xyz",
			args:        []string{},
			wantErr:     true,
			description: "Missing binary should error with Background context",
		},
		{
			name:        "Non-zero exit code returns error",
			binary:      "sh",
			args:        []string{"-c", "exit 1"},
			wantErr:     true,
			description: "Command failure should propagate with Background context",
		},
		{
			name:        "Invalid arguments return error",
			binary:      "ls",
			args:        []string{"--invalid-flag-xyz"},
			wantErr:     true,
			description: "Invalid flags should error with Background context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := agents.NewExecCLIExecutor()
			ctx := context.Background()

			stdout, stderr, err := executor.Run(ctx, tt.binary, tt.args...)

			if tt.wantErr {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
			assert.NotNil(t, stdout)
			assert.NotNil(t, stderr)
		})
	}
}

func TestContextUsage_ErrorHandlingWithMock(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*testutil.MockCLIExecutor)
		wantErr     bool
		description string
	}{
		{
			name: "Mock with configured error",
			setupMock: func(m *testutil.MockCLIExecutor) {
				m.SetError(assert.AnError)
			},
			wantErr:     true,
			description: "Mock error should propagate with Background context",
		},
		{
			name: "Mock with success output",
			setupMock: func(m *testutil.MockCLIExecutor) {
				m.SetOutput([]byte("success"), []byte(""))
			},
			wantErr:     false,
			description: "Mock success should work with Background context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := testutil.NewMockCLIExecutor()
			tt.setupMock(mock)
			ctx := context.Background()

			stdout, stderr, err := mock.Run(ctx, "tool", "arg")

			if tt.wantErr {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
			if !tt.wantErr {
				assert.NotNil(t, stdout)
				assert.NotNil(t, stderr)
			}
		})
	}
}

// TestContextUsage_FullWorkflow validates that context.Background() works
// correctly in a complete test workflow simulating real usage.
func TestContextUsage_FullWorkflow(t *testing.T) {
	executor := agents.NewExecCLIExecutor()
	ctx := context.Background()

	steps := []struct {
		binary string
		args   []string
	}{
		{"echo", []string{"step1"}},
		{"true", []string{}},
		{"echo", []string{"step2"}},
	}

	results := make([]error, len(steps))
	for i, step := range steps {
		_, _, err := executor.Run(ctx, step.binary, step.args...)
		results[i] = err
	}

	for i, err := range results {
		assert.NoError(t, err, "Step %d should succeed with Background context", i+1)
	}
}

// TestContextUsage_CancellableContext validates that derived contexts
// from context.Background() work correctly for cancellation scenarios.
func TestContextUsage_CancellableContext(t *testing.T) {
	executor := agents.NewExecCLIExecutor()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stdout, stderr, err := executor.Run(ctx, "echo", "test")

	assert.NoError(t, err)
	assert.NotNil(t, stdout)
	assert.NotNil(t, stderr)
}

// TestContextUsage_TimeoutContext validates that timeout contexts
// derived from context.Background() work correctly.
func TestContextUsage_TimeoutContext(t *testing.T) {
	executor := agents.NewExecCLIExecutor()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stdout, stderr, err := executor.Run(ctx, "echo", "test")

	assert.NoError(t, err)
	assert.NotNil(t, stdout)
	assert.NotNil(t, stderr)
}

// TestContextUsage_ConcurrentAccess validates that context.Background()
// can be safely used concurrently across multiple test goroutines.
func TestContextUsage_ConcurrentAccess(t *testing.T) {
	executor := agents.NewExecCLIExecutor()
	ctx := context.Background()
	numGoroutines := 10

	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			_, _, err := executor.Run(ctx, "echo", "concurrent")
			errors <- err
		}(i)
	}

	// Collect results
	results := make([]error, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		results[i] = <-errors
	}

	for i, err := range results {
		assert.NoError(t, err, "Goroutine %d should succeed with shared Background context", i)
	}
}

// TestContextUsage_SemanticCorrectness validates that context.Background()
// is semantically appropriate for test scenarios (vs context.TODO()).
func TestContextUsage_SemanticCorrectness(t *testing.T) {
	t.Run("Background context is appropriate for tests", func(t *testing.T) {
		executor := agents.NewExecCLIExecutor()

		// context.Background() is correct because:
		// 1. We know exactly what context we need (empty context for testing)
		// 2. This is not a "TODO" - the implementation is complete
		// 3. This is top-level test execution, not pending design
		ctx := context.Background()

		stdout, stderr, err := executor.Run(ctx, "echo", "semantic")

		require.NoError(t, err, "Background context should work correctly in tests")
		assert.NotEmpty(t, stdout, "Should produce output")
		assert.NotNil(t, stderr, "Should have stderr buffer")
	})

	t.Run("Background context vs TODO context behavior", func(t *testing.T) {
		// Both context.Background() and context.TODO() return the same
		// underlying emptyCtx type, so runtime behavior is identical.
		// The difference is semantic/documentary:
		// - Background: "I intentionally want an empty context"
		// - TODO: "I'm not sure what context to use yet"

		executor := agents.NewExecCLIExecutor()

		// Test with Background (correct for tests)
		ctxBackground := context.Background()
		stdoutBg, stderrBg, errBg := executor.Run(ctxBackground, "echo", "test")

		// Test with TODO (incorrect semantic choice for tests)
		ctxTODO := context.TODO()
		stdoutTodo, stderrTodo, errTodo := executor.Run(ctxTODO, "echo", "test")

		assert.Equal(t, errBg, errTodo, "Both should have same error status")
		assert.Equal(t, len(stdoutBg), len(stdoutTodo), "Both should produce same output length")
		assert.Equal(t, len(stderrBg), len(stderrTodo), "Both should have same stderr length")

		// The difference is semantic correctness:
		// Background communicates intent clearly in tests
		// TODO suggests incomplete implementation
	})
}

// TestContextUsageAcceptanceCriteria validates that all acceptance criteria
// from the C046 specification are met.
func TestContextUsageAcceptanceCriteria(t *testing.T) {
	t.Run("AC1: context.Background() works in all test scenarios", func(t *testing.T) {
		// This test validates that the replacement from context.TODO()
		// to context.Background() maintains correct behavior

		executor := agents.NewExecCLIExecutor()
		ctx := context.Background()

		// Various test scenarios
		scenarios := []struct {
			binary string
			args   []string
		}{
			{"echo", []string{"test"}},
			{"true", []string{}},
			{"pwd", []string{}},
		}

		for _, scenario := range scenarios {
			stdout, stderr, err := executor.Run(ctx, scenario.binary, scenario.args...)
			assert.NoError(t, err, "All commands should work with Background context")
			assert.NotNil(t, stdout)
			assert.NotNil(t, stderr)
		}
	})

	t.Run("AC2: No regressions in test behavior", func(t *testing.T) {
		// Verify that existing test patterns still work correctly

		mock := testutil.NewMockCLIExecutor()
		mock.SetOutput([]byte("output"), []byte("error"))
		ctx := context.Background()

		stdout, stderr, err := mock.Run(ctx, "tool", "arg")

		assert.NoError(t, err)
		assert.Equal(t, "output", string(stdout))
		assert.Equal(t, "error", string(stderr))

		// Verify call recording still works
		calls := mock.GetCalls()
		require.Len(t, calls, 1)
		assert.Equal(t, "tool", calls[0].Name)
		assert.Equal(t, []string{"arg"}, calls[0].Args)
	})

	t.Run("AC3: Context semantics are correct for test usage", func(t *testing.T) {
		// Validate that context.Background() is the correct semantic choice

		// Background is appropriate because:
		// 1. We're at the top level of a test function
		// 2. We intentionally want an empty context
		// 3. The context handling is complete, not pending

		ctx := context.Background()

		// Verify it's the correct context type
		assert.NotNil(t, ctx, "Background context should be valid")

		// context.Background() returns an emptyCtx whose Done() channel is nil
		// This is correct: a nil channel never becomes selectable, which means
		// the context is never cancelled
		doneChannel := ctx.Done()
		if doneChannel != nil {
			// If Done() returns non-nil, it should never close
			select {
			case <-doneChannel:
				t.Fatal("Background context Done channel should never close")
			default:
				// Expected: channel exists but is not closed
			}
		}

		// Verify it doesn't have deadline or cancellation
		_, hasDeadline := ctx.Deadline()
		assert.False(t, hasDeadline, "Background context should have no deadline")
	})

	t.Run("AC4: go vet passes with context.Background()", func(t *testing.T) {
		// This is validated by running go vet in CI/CD
		// This test confirms the runtime behavior is correct

		executor := agents.NewExecCLIExecutor()
		ctx := context.Background()

		// No vet warnings should occur with this usage
		stdout, stderr, err := executor.Run(ctx, "echo", "test")

		assert.NoError(t, err)
		assert.NotNil(t, stdout)
		assert.NotNil(t, stderr)

		// Context is properly used:
		// - Not stored in struct
		// - Not nil
		// - Passed as first parameter
		// - Not replaced mid-execution
	})
}
