package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/repository"
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

// TestResolvePromptFromPaths tests the multi-path prompt resolution function.
// This implements T006 - Update resolvePromptInput() to search multiple paths in priority order.
func TestResolvePromptFromPaths(t *testing.T) {
	// Base fixture path relative to project root
	fixtureBase := "tests/fixtures/prompts"

	tests := []struct {
		name         string
		relativePath string
		paths        []repository.SourcedPath
		wantContains string // substring expected in content
		wantErr      bool
		errContains  string
	}{
		// --- Happy path: Local-only prompt ---
		{
			name:         "finds prompt in local directory only",
			relativePath: "local-only.md",
			paths: []repository.SourcedPath{
				{Path: filepath.Join(fixtureBase, "local"), Source: repository.SourceLocal},
				{Path: filepath.Join(fixtureBase, "global"), Source: repository.SourceGlobal},
			},
			wantContains: "Source: local",
			wantErr:      false,
		},
		// --- Happy path: Global-only prompt ---
		{
			name:         "finds prompt in global directory when not in local",
			relativePath: "system.md",
			paths: []repository.SourcedPath{
				{Path: filepath.Join(fixtureBase, "local"), Source: repository.SourceLocal},
				{Path: filepath.Join(fixtureBase, "global"), Source: repository.SourceGlobal},
			},
			wantContains: "Source: global",
			wantErr:      false,
		},
		// --- Priority: Local overrides Global (US2) ---
		{
			name:         "local prompt takes precedence over global with same name",
			relativePath: "shared.md",
			paths: []repository.SourcedPath{
				{Path: filepath.Join(fixtureBase, "local"), Source: repository.SourceLocal},
				{Path: filepath.Join(fixtureBase, "global"), Source: repository.SourceGlobal},
			},
			wantContains: "local-shared-content", // Must be local version, not global
			wantErr:      false,
		},
		// --- Nested directories (FR-006) ---
		{
			name:         "supports nested directories in local path",
			relativePath: "nested/local-deep.md",
			paths: []repository.SourcedPath{
				{Path: filepath.Join(fixtureBase, "local"), Source: repository.SourceLocal},
				{Path: filepath.Join(fixtureBase, "global"), Source: repository.SourceGlobal},
			},
			wantContains: "local",
			wantErr:      false,
		},
		{
			name:         "supports nested directories in global path",
			relativePath: "nested/deep.md",
			paths: []repository.SourcedPath{
				{Path: filepath.Join(fixtureBase, "local"), Source: repository.SourceLocal},
				{Path: filepath.Join(fixtureBase, "global"), Source: repository.SourceGlobal},
			},
			wantContains: "deep",
			wantErr:      false,
		},
		// --- Error: File not found in any path ---
		{
			name:         "error when prompt not found in any path",
			relativePath: "nonexistent.md",
			paths: []repository.SourcedPath{
				{Path: filepath.Join(fixtureBase, "local"), Source: repository.SourceLocal},
				{Path: filepath.Join(fixtureBase, "global"), Source: repository.SourceGlobal},
			},
			wantErr:     true,
			errContains: "not found",
		},
		// --- Edge case: Empty paths slice ---
		{
			name:         "error when no paths provided",
			relativePath: "any.md",
			paths:        []repository.SourcedPath{},
			wantErr:      true,
			errContains:  "not found",
		},
		// --- Edge case: Paths don't exist on disk ---
		{
			name:         "error when path directories don't exist",
			relativePath: "test.md",
			paths: []repository.SourcedPath{
				{Path: "/nonexistent/path/1", Source: repository.SourceLocal},
				{Path: "/nonexistent/path/2", Source: repository.SourceGlobal},
			},
			wantErr:     true,
			errContains: "not found",
		},
		// --- Edge case: Only global path provided ---
		{
			name:         "finds prompt when only global path provided",
			relativePath: "system.md",
			paths: []repository.SourcedPath{
				{Path: filepath.Join(fixtureBase, "global"), Source: repository.SourceGlobal},
			},
			wantContains: "Source: global",
			wantErr:      false,
		},
		// --- Edge case: Only local path provided ---
		{
			name:         "finds prompt when only local path provided",
			relativePath: "local-only.md",
			paths: []repository.SourcedPath{
				{Path: filepath.Join(fixtureBase, "local"), Source: repository.SourceLocal},
			},
			wantContains: "Source: local",
			wantErr:      false,
		},
		// --- Priority order matters ---
		{
			name:         "first path in slice takes priority",
			relativePath: "shared.md",
			paths: []repository.SourcedPath{
				// Reversed order: global first, should return global content
				{Path: filepath.Join(fixtureBase, "global"), Source: repository.SourceGlobal},
				{Path: filepath.Join(fixtureBase, "local"), Source: repository.SourceLocal},
			},
			wantContains: "global-shared-content", // Global should win when listed first
			wantErr:      false,
		},
		// --- Edge case: First path doesn't exist but second does ---
		{
			name:         "falls through to second path when first doesn't exist",
			relativePath: "system.md",
			paths: []repository.SourcedPath{
				{Path: "/nonexistent/local", Source: repository.SourceLocal},
				{Path: filepath.Join(fixtureBase, "global"), Source: repository.SourceGlobal},
			},
			wantContains: "Source: global",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Change to project root for fixture paths to work
			origDir, err := os.Getwd()
			require.NoError(t, err)

			// Navigate to project root (3 levels up from internal/interfaces/cli)
			projectRoot := filepath.Join(origDir, "..", "..", "..")
			err = os.Chdir(projectRoot)
			require.NoError(t, err)
			defer func() { _ = os.Chdir(origDir) }()

			// Call the function under test
			content, err := resolvePromptFromPaths(tt.relativePath, tt.paths)

			if tt.wantErr {
				require.Error(t, err, "expected error but got none")
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains,
						"error message should contain %q", tt.errContains)
				}
			} else {
				require.NoError(t, err, "unexpected error: %v", err)
				assert.Contains(t, content, tt.wantContains,
					"content should contain %q", tt.wantContains)
			}
		})
	}
}

// TestResolvePromptFromPaths_ContentTrimming verifies whitespace handling
func TestResolvePromptFromPaths_ContentTrimming(t *testing.T) {
	// Create temp directory with a prompt file containing whitespace
	tmpDir := t.TempDir()
	promptsDir := filepath.Join(tmpDir, "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0755))

	// Write file with leading/trailing whitespace
	content := "\n\n  content with whitespace  \n\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(promptsDir, "whitespace.md"),
		[]byte(content),
		0644,
	))

	paths := []repository.SourcedPath{
		{Path: promptsDir, Source: repository.SourceLocal},
	}

	result, err := resolvePromptFromPaths("whitespace.md", paths)
	require.NoError(t, err)

	// Content should be trimmed
	assert.Equal(t, "content with whitespace", result,
		"content should have leading/trailing whitespace trimmed")
}

// TestResolvePromptFromPaths_EmptyFile tests handling of empty prompt files
func TestResolvePromptFromPaths_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	promptsDir := filepath.Join(tmpDir, "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0755))

	// Write empty file
	require.NoError(t, os.WriteFile(
		filepath.Join(promptsDir, "empty.md"),
		[]byte(""),
		0644,
	))

	paths := []repository.SourcedPath{
		{Path: promptsDir, Source: repository.SourceLocal},
	}

	result, err := resolvePromptFromPaths("empty.md", paths)
	require.NoError(t, err)
	assert.Empty(t, result, "empty file should return empty string")
}

// TestResolvePromptFromPaths_WhitespaceOnlyFile tests handling of whitespace-only files
func TestResolvePromptFromPaths_WhitespaceOnlyFile(t *testing.T) {
	tmpDir := t.TempDir()
	promptsDir := filepath.Join(tmpDir, "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0755))

	// Write file with only whitespace
	require.NoError(t, os.WriteFile(
		filepath.Join(promptsDir, "spaces.md"),
		[]byte("   \n\t\n   "),
		0644,
	))

	paths := []repository.SourcedPath{
		{Path: promptsDir, Source: repository.SourceLocal},
	}

	result, err := resolvePromptFromPaths("spaces.md", paths)
	require.NoError(t, err)
	assert.Empty(t, result, "whitespace-only file should return empty string after trimming")
}

// TestResolvePromptFromPaths_LargeFile tests handling of large prompt files
func TestResolvePromptFromPaths_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	promptsDir := filepath.Join(tmpDir, "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0755))

	// Write a large file (1MB of content)
	largeContent := strings.Repeat("This is a line of content.\n", 40000)
	require.NoError(t, os.WriteFile(
		filepath.Join(promptsDir, "large.md"),
		[]byte(largeContent),
		0644,
	))

	paths := []repository.SourcedPath{
		{Path: promptsDir, Source: repository.SourceLocal},
	}

	result, err := resolvePromptFromPaths("large.md", paths)
	require.NoError(t, err)
	assert.Contains(t, result, "This is a line of content",
		"large file content should be readable")
}

// TestResolvePromptFromPaths_SpecialCharactersInFilename tests filenames with special chars
func TestResolvePromptFromPaths_SpecialCharactersInFilename(t *testing.T) {
	tmpDir := t.TempDir()
	promptsDir := filepath.Join(tmpDir, "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0755))

	tests := []struct {
		name     string
		filename string
	}{
		{"filename with spaces", "my prompt.md"},
		{"filename with dashes", "my-prompt.md"},
		{"filename with underscores", "my_prompt.md"},
		{"filename with dots", "my.prompt.v2.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write file
			require.NoError(t, os.WriteFile(
				filepath.Join(promptsDir, tt.filename),
				[]byte("special char content"),
				0644,
			))

			paths := []repository.SourcedPath{
				{Path: promptsDir, Source: repository.SourceLocal},
			}

			result, err := resolvePromptFromPaths(tt.filename, paths)
			require.NoError(t, err)
			assert.Contains(t, result, "special char content")
		})
	}
}

// TestResolvePromptFromPaths_DifferentFileExtensions tests various file extensions
func TestResolvePromptFromPaths_DifferentFileExtensions(t *testing.T) {
	tmpDir := t.TempDir()
	promptsDir := filepath.Join(tmpDir, "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0755))

	extensions := []string{".md", ".txt", ".prompt", ".yaml", ""}

	for _, ext := range extensions {
		filename := "test" + ext
		expectedContent := "content for ext:" + ext
		t.Run("extension_"+ext, func(t *testing.T) {
			require.NoError(t, os.WriteFile(
				filepath.Join(promptsDir, filename),
				[]byte(expectedContent),
				0644,
			))

			paths := []repository.SourcedPath{
				{Path: promptsDir, Source: repository.SourceLocal},
			}

			result, err := resolvePromptFromPaths(filename, paths)
			require.NoError(t, err)
			assert.Contains(t, result, expectedContent)
		})
	}
}

// TestResolvePromptFromPaths_DeeplyNested tests deeply nested directory structures
func TestResolvePromptFromPaths_DeeplyNested(t *testing.T) {
	tmpDir := t.TempDir()
	deepPath := filepath.Join(tmpDir, "prompts", "a", "b", "c", "d", "e")
	require.NoError(t, os.MkdirAll(deepPath, 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(deepPath, "deep.md"),
		[]byte("deeply nested content"),
		0644,
	))

	paths := []repository.SourcedPath{
		{Path: filepath.Join(tmpDir, "prompts"), Source: repository.SourceLocal},
	}

	result, err := resolvePromptFromPaths("a/b/c/d/e/deep.md", paths)
	require.NoError(t, err)
	assert.Contains(t, result, "deeply nested content")
}

// TestResolvePromptFromPaths_SymlinkHandling tests behavior with symlinks
func TestResolvePromptFromPaths_SymlinkHandling(t *testing.T) {
	tmpDir := t.TempDir()
	promptsDir := filepath.Join(tmpDir, "prompts")
	targetDir := filepath.Join(tmpDir, "target")

	require.NoError(t, os.MkdirAll(promptsDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	// Create actual file in target directory
	require.NoError(t, os.WriteFile(
		filepath.Join(targetDir, "actual.md"),
		[]byte("symlinked content"),
		0644,
	))

	// Create symlink in prompts directory
	symlinkPath := filepath.Join(promptsDir, "linked.md")
	err := os.Symlink(filepath.Join(targetDir, "actual.md"), symlinkPath)
	if err != nil {
		t.Skip("symlinks not supported on this system")
	}

	paths := []repository.SourcedPath{
		{Path: promptsDir, Source: repository.SourceLocal},
	}

	result, err := resolvePromptFromPaths("linked.md", paths)
	require.NoError(t, err)
	assert.Contains(t, result, "symlinked content")
}

// TestResolvePromptFromPaths_MultiplePathsWithPartialExistence tests mixed existing/non-existing paths
func TestResolvePromptFromPaths_MultiplePathsWithPartialExistence(t *testing.T) {
	tmpDir := t.TempDir()
	existingDir := filepath.Join(tmpDir, "existing")
	require.NoError(t, os.MkdirAll(existingDir, 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(existingDir, "test.md"),
		[]byte("found in existing path"),
		0644,
	))

	paths := []repository.SourcedPath{
		{Path: filepath.Join(tmpDir, "nonexistent1"), Source: repository.SourceLocal},
		{Path: filepath.Join(tmpDir, "nonexistent2"), Source: repository.SourceLocal},
		{Path: existingDir, Source: repository.SourceGlobal},
		{Path: filepath.Join(tmpDir, "nonexistent3"), Source: repository.SourceLocal},
	}

	result, err := resolvePromptFromPaths("test.md", paths)
	require.NoError(t, err)
	assert.Contains(t, result, "found in existing path")
}

// TestResolvePromptFromPaths_FileInDirectoryNotFile tests that directories are not treated as files
func TestResolvePromptFromPaths_FileInDirectoryNotFile(t *testing.T) {
	tmpDir := t.TempDir()
	promptsDir := filepath.Join(tmpDir, "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0755))

	// Create a directory with the same name as what we're looking for
	require.NoError(t, os.MkdirAll(filepath.Join(promptsDir, "notafile.md"), 0755))

	paths := []repository.SourcedPath{
		{Path: promptsDir, Source: repository.SourceLocal},
	}

	_, err := resolvePromptFromPaths("notafile.md", paths)
	// Should error because it's a directory, not a file
	require.Error(t, err)
}

// TestResolvePromptFromPaths_UTF8Content tests handling of UTF-8 content
func TestResolvePromptFromPaths_UTF8Content(t *testing.T) {
	tmpDir := t.TempDir()
	promptsDir := filepath.Join(tmpDir, "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0755))

	// Content with various Unicode characters
	utf8Content := "Hello 世界! Émoji: 🚀 Symbols: ∑∫∂ Greek: αβγδ"
	require.NoError(t, os.WriteFile(
		filepath.Join(promptsDir, "unicode.md"),
		[]byte(utf8Content),
		0644,
	))

	paths := []repository.SourcedPath{
		{Path: promptsDir, Source: repository.SourceLocal},
	}

	result, err := resolvePromptFromPaths("unicode.md", paths)
	require.NoError(t, err)
	assert.Equal(t, utf8Content, result)
}
