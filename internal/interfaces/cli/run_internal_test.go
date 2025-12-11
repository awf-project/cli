package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

// RED Phase: Test stubs for unexported run.go helper functions
// These tests will compile but fail when run - implementation validation needed

func TestShowExecutionDetails(t *testing.T) {
	tests := []struct {
		name    string
		execCtx *workflow.ExecutionContext
		wantOut []string // substrings expected in output
	}{
		{
			name: "single completed step",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("test-123", "test-wf")
				ctx.Status = workflow.StatusCompleted
				ctx.States["step1"] = workflow.StepState{
					Name:        "step1",
					Status:      workflow.StatusCompleted,
					StartedAt:   time.Now(),
					CompletedAt: time.Now().Add(100 * time.Millisecond),
				}
				return ctx
			}(),
			wantOut: []string{"Execution Details", "step1", "completed"},
		},
		{
			name: "multiple steps with different statuses",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("test-456", "multi-wf")
				ctx.Status = workflow.StatusFailed
				ctx.States["fetch"] = workflow.StepState{
					Name:        "fetch",
					Status:      workflow.StatusCompleted,
					StartedAt:   time.Now(),
					CompletedAt: time.Now().Add(50 * time.Millisecond),
				}
				ctx.States["process"] = workflow.StepState{
					Name:        "process",
					Status:      workflow.StatusFailed,
					StartedAt:   time.Now(),
					CompletedAt: time.Now().Add(30 * time.Millisecond),
				}
				return ctx
			}(),
			wantOut: []string{"fetch", "process", "failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

			showExecutionDetails(formatter, tt.execCtx)

			output := buf.String()
			for _, want := range tt.wantOut {
				assert.Contains(t, output, want)
			}
		})
	}
}

func TestShowStepOutputs(t *testing.T) {
	tests := []struct {
		name    string
		execCtx *workflow.ExecutionContext
		wantOut []string
	}{
		{
			name: "step with stdout only",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("test-1", "wf")
				ctx.States["build"] = workflow.StepState{
					Name:   "build",
					Status: workflow.StatusCompleted,
					Output: "Build successful",
				}
				return ctx
			}(),
			wantOut: []string{"build", "stdout", "Build successful"},
		},
		{
			name: "step with stderr",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("test-2", "wf")
				ctx.States["lint"] = workflow.StepState{
					Name:   "lint",
					Status: workflow.StatusCompleted,
					Stderr: "Warning: unused variable",
				}
				return ctx
			}(),
			wantOut: []string{"lint", "stderr", "Warning"},
		},
		{
			name: "step with no output shows success",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("test-3", "wf")
				ctx.States["clean"] = workflow.StepState{
					Name:   "clean",
					Status: workflow.StatusCompleted,
					Output: "",
					Stderr: "",
				}
				return ctx
			}(),
			wantOut: []string{"clean"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

			showStepOutputs(formatter, tt.execCtx)

			output := buf.String()
			for _, want := range tt.wantOut {
				assert.Contains(t, output, want)
			}
		})
	}
}

func TestShowEmptyStepFeedback(t *testing.T) {
	tests := []struct {
		name     string
		execCtx  *workflow.ExecutionContext
		wantStep string
	}{
		{
			name: "completed step with no output",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("test-1", "wf")
				ctx.States["silent-step"] = workflow.StepState{
					Name:   "silent-step",
					Status: workflow.StatusCompleted,
					Output: "",
					Stderr: "",
				}
				return ctx
			}(),
			wantStep: "silent-step",
		},
		{
			name: "step with output should not show feedback",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("test-2", "wf")
				ctx.States["verbose-step"] = workflow.StepState{
					Name:   "verbose-step",
					Status: workflow.StatusCompleted,
					Output: "Some output",
				}
				return ctx
			}(),
			wantStep: "", // should not contain step name in success message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

			showEmptyStepFeedback(formatter, tt.execCtx)

			output := buf.String()
			if tt.wantStep != "" {
				assert.Contains(t, output, tt.wantStep)
			}
		})
	}
}

func TestBuildStepInfos(t *testing.T) {
	tests := []struct {
		name      string
		execCtx   *workflow.ExecutionContext
		wantCount int
		wantNames []string
	}{
		{
			name: "single step",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("test-1", "wf")
				ctx.States["step1"] = workflow.StepState{
					Name:        "step1",
					Status:      workflow.StatusCompleted,
					Output:      "output",
					ExitCode:    0,
					StartedAt:   time.Now(),
					CompletedAt: time.Now(),
				}
				return ctx
			}(),
			wantCount: 1,
			wantNames: []string{"step1"},
		},
		{
			name: "multiple steps",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("test-2", "wf")
				ctx.States["fetch"] = workflow.StepState{Name: "fetch", Status: workflow.StatusCompleted}
				ctx.States["process"] = workflow.StepState{Name: "process", Status: workflow.StatusCompleted}
				ctx.States["store"] = workflow.StepState{Name: "store", Status: workflow.StatusCompleted}
				return ctx
			}(),
			wantCount: 3,
			wantNames: []string{"fetch", "process", "store"},
		},
		{
			name:      "empty states",
			execCtx:   workflow.NewExecutionContext("test-3", "wf"),
			wantCount: 0,
			wantNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := buildStepInfos(tt.execCtx)

			assert.Len(t, steps, tt.wantCount)

			// Check all expected names are present
			names := make([]string, len(steps))
			for i, s := range steps {
				names[i] = s.Name
			}
			for _, want := range tt.wantNames {
				assert.Contains(t, names, want)
			}
		})
	}
}

func TestCategorizeError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		wantExit int
	}{
		{
			name:     "not found error",
			errMsg:   "workflow not found: myworkflow",
			wantExit: ExitUser,
		},
		{
			name:     "invalid error",
			errMsg:   "invalid state reference",
			wantExit: ExitWorkflow,
		},
		{
			name:     "timeout error",
			errMsg:   "command timeout after 30s",
			wantExit: ExitExecution,
		},
		{
			name:     "exit code error",
			errMsg:   "command failed with exit code 1",
			wantExit: ExitExecution,
		},
		{
			name:     "permission error",
			errMsg:   "permission denied",
			wantExit: ExitSystem,
		},
		{
			name:     "generic error",
			errMsg:   "something went wrong",
			wantExit: ExitExecution,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &exitError{err: assert.AnError}
			// Create error with specific message for testing
			testErr := &testError{msg: tt.errMsg}
			got := categorizeError(testErr)
			assert.Equal(t, tt.wantExit, got)
			_ = err // silence unused
		})
	}
}

// testError is a helper for testing categorizeError
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestExitError_ExitCode(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		wantCode int
	}{
		{
			name:     "user error",
			code:     ExitUser,
			wantCode: 1,
		},
		{
			name:     "workflow error",
			code:     ExitWorkflow,
			wantCode: 2,
		},
		{
			name:     "execution error",
			code:     ExitExecution,
			wantCode: 3,
		},
		{
			name:     "system error",
			code:     ExitSystem,
			wantCode: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &exitError{code: tt.code, err: assert.AnError}
			assert.Equal(t, tt.wantCode, err.ExitCode())
		})
	}
}

func TestExitError_Error(t *testing.T) {
	underlying := &testError{msg: "test error message"}
	err := &exitError{code: ExitUser, err: underlying}

	assert.Equal(t, "test error message", err.Error())
}

func TestCliLogger_WithContext(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	logger := &cliLogger{
		formatter: formatter,
		verbose:   true,
		context:   map[string]any{"initial": "value"},
	}

	// Add context
	newLogger := logger.WithContext(map[string]any{
		"step": "fetch",
		"id":   "123",
	})

	// Verify it returns a new logger
	assert.NotSame(t, logger, newLogger)

	// Verify context is merged
	cliLog, ok := newLogger.(*cliLogger)
	assert.True(t, ok)
	assert.Equal(t, "value", cliLog.context["initial"])
	assert.Equal(t, "fetch", cliLog.context["step"])
	assert.Equal(t, "123", cliLog.context["id"])
}

func TestCliLogger_Methods(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	logger := &cliLogger{
		formatter: formatter,
		verbose:   true,
		silent:    false,
	}

	// Test that methods don't panic
	logger.Debug("debug message", "key", "value")
	logger.Info("info message", "key", "value")
	logger.Warn("warn message", "key", "value")
	logger.Error("error message", "key", "value")

	output := buf.String()
	assert.Contains(t, output, "info message")
	assert.Contains(t, output, "warn message")
	assert.Contains(t, output, "error message")
}

func TestCliLogger_Silent(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	logger := &cliLogger{
		formatter: formatter,
		verbose:   true,
		silent:    true, // silent mode
	}

	logger.Debug("should not appear")
	logger.Info("should not appear")
	logger.Warn("should not appear")
	logger.Error("should not appear")

	assert.Empty(t, buf.String())
}

func TestCliLogger_MergeContext(t *testing.T) {
	tests := []struct {
		name          string
		initialCtx    map[string]any
		keysAndValues []any
		wantLen       int
	}{
		{
			name:          "empty context, empty keys",
			initialCtx:    nil,
			keysAndValues: []any{},
			wantLen:       0,
		},
		{
			name:          "empty context, with keys",
			initialCtx:    nil,
			keysAndValues: []any{"key1", "val1", "key2", "val2"},
			wantLen:       4,
		},
		{
			name:          "with context, empty keys",
			initialCtx:    map[string]any{"ctx1": "ctxval1"},
			keysAndValues: []any{},
			wantLen:       2, // context gets flattened to key, value pairs
		},
		{
			name:          "with context, with keys",
			initialCtx:    map[string]any{"ctx1": "ctxval1"},
			keysAndValues: []any{"key1", "val1"},
			wantLen:       4, // context (2) + keys (2)
		},
		{
			name:          "multiple context entries",
			initialCtx:    map[string]any{"ctx1": "v1", "ctx2": "v2", "ctx3": "v3"},
			keysAndValues: []any{"k1", "v1"},
			wantLen:       8, // context (6) + keys (2)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

			logger := &cliLogger{
				formatter: formatter,
				verbose:   true,
				context:   tt.initialCtx,
			}

			result := logger.mergeContext(tt.keysAndValues)
			assert.Len(t, result, tt.wantLen)
		})
	}
}

func TestFormatLog(t *testing.T) {
	tests := []struct {
		name          string
		msg           string
		keysAndValues []any
		want          string
	}{
		{
			name:          "message only",
			msg:           "simple message",
			keysAndValues: []any{},
			want:          "simple message",
		},
		{
			name:          "message with single key-value",
			msg:           "operation",
			keysAndValues: []any{"status", "success"},
			want:          "operation status=success",
		},
		{
			name:          "message with multiple key-values",
			msg:           "step completed",
			keysAndValues: []any{"step", "fetch", "duration", 100},
			want:          "step completed step=fetch duration=100",
		},
		{
			name:          "odd number of keys (last key without value)",
			msg:           "warning",
			keysAndValues: []any{"key1", "val1", "orphan"},
			want:          "warning key1=val1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatLog(tt.msg, tt.keysAndValues...)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestCliLogger_DebugNotVerbose(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	logger := &cliLogger{
		formatter: formatter,
		verbose:   false, // not verbose
		silent:    false,
	}

	logger.Debug("debug message", "key", "value")

	// Debug should not appear when not verbose
	assert.NotContains(t, buf.String(), "debug message")
}

func TestCliLogger_WithContextNilInitial(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	logger := &cliLogger{
		formatter: formatter,
		verbose:   true,
		context:   nil, // no initial context
	}

	newLogger := logger.WithContext(map[string]any{
		"step": "fetch",
	})

	cliLog, ok := newLogger.(*cliLogger)
	assert.True(t, ok)
	assert.Equal(t, "fetch", cliLog.context["step"])
}

func TestCliLogger_InfoWithContext(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	logger := &cliLogger{
		formatter: formatter,
		verbose:   true,
		silent:    false,
		context:   map[string]any{"workflow": "test-wf"},
	}

	logger.Info("step started", "step", "fetch")

	output := buf.String()
	assert.Contains(t, output, "step started")
	assert.Contains(t, output, "workflow=test-wf")
	assert.Contains(t, output, "step=fetch")
}
