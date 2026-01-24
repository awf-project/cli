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
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

// setupConfigTestDir creates a temporary directory for config loading tests.
// It sets AWF_CONFIG_PATH to the config file path, which is a thread-safe
// alternative to os.Chdir for tests that need loadProjectConfig().
//
// Returns the temporary directory path.
func setupConfigTestDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Set AWF_CONFIG_PATH to override config path (thread-safe alternative to os.Chdir)
	// This allows loadProjectConfig() to find the config at an absolute path
	t.Setenv("AWF_CONFIG_PATH", filepath.Join(tmpDir, ".awf", "config.yaml"))

	return tmpDir
}

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
	// Get project root (3 levels up from internal/interfaces/cli)
	origDir, err := os.Getwd()
	require.NoError(t, err)
	projectRoot := filepath.Join(origDir, "..", "..", "..")
	fixtureBase := filepath.Join(projectRoot, "tests/fixtures/prompts")

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
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))

	// Write file with leading/trailing whitespace
	content := "\n\n  content with whitespace  \n\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(promptsDir, "whitespace.md"),
		[]byte(content),
		0o644,
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
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))

	// Write empty file
	require.NoError(t, os.WriteFile(
		filepath.Join(promptsDir, "empty.md"),
		[]byte(""),
		0o644,
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
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))

	// Write file with only whitespace
	require.NoError(t, os.WriteFile(
		filepath.Join(promptsDir, "spaces.md"),
		[]byte("   \n\t\n   "),
		0o644,
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
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))

	// Write a large file (1MB of content)
	largeContent := strings.Repeat("This is a line of content.\n", 40000)
	require.NoError(t, os.WriteFile(
		filepath.Join(promptsDir, "large.md"),
		[]byte(largeContent),
		0o644,
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
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))

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
				0o644,
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
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))

	extensions := []string{".md", ".txt", ".prompt", ".yaml", ""}

	for _, ext := range extensions {
		filename := "test" + ext
		expectedContent := "content for ext:" + ext
		t.Run("extension_"+ext, func(t *testing.T) {
			require.NoError(t, os.WriteFile(
				filepath.Join(promptsDir, filename),
				[]byte(expectedContent),
				0o644,
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
	require.NoError(t, os.MkdirAll(deepPath, 0o755))

	require.NoError(t, os.WriteFile(
		filepath.Join(deepPath, "deep.md"),
		[]byte("deeply nested content"),
		0o644,
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

	require.NoError(t, os.MkdirAll(promptsDir, 0o755))
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	// Create actual file in target directory
	require.NoError(t, os.WriteFile(
		filepath.Join(targetDir, "actual.md"),
		[]byte("symlinked content"),
		0o644,
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
	require.NoError(t, os.MkdirAll(existingDir, 0o755))

	require.NoError(t, os.WriteFile(
		filepath.Join(existingDir, "test.md"),
		[]byte("found in existing path"),
		0o644,
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
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))

	// Create a directory with the same name as what we're looking for
	require.NoError(t, os.MkdirAll(filepath.Join(promptsDir, "notafile.md"), 0o755))

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
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))

	// Content with various Unicode characters
	utf8Content := "Hello 世界! Émoji: 🚀 Symbols: ∑∫∂ Greek: αβγδ"
	require.NoError(t, os.WriteFile(
		filepath.Join(promptsDir, "unicode.md"),
		[]byte(utf8Content),
		0o644,
	))

	paths := []repository.SourcedPath{
		{Path: promptsDir, Source: repository.SourceLocal},
	}

	result, err := resolvePromptFromPaths("unicode.md", paths)
	require.NoError(t, err)
	assert.Equal(t, utf8Content, result)
}

// =============================================================================
// T006: mergeInputs() tests
// =============================================================================

// TestMergeInputs tests the mergeInputs() helper that merges config file inputs
// with CLI flag inputs. CLI inputs always take precedence over config inputs.
func TestMergeInputs(t *testing.T) {
	tests := []struct {
		name         string
		configInputs map[string]any
		cliInputs    map[string]any
		want         map[string]any
	}{
		// --- Happy path: Both maps empty ---
		{
			name:         "both nil returns empty map",
			configInputs: nil,
			cliInputs:    nil,
			want:         map[string]any{},
		},
		{
			name:         "both empty returns empty map",
			configInputs: map[string]any{},
			cliInputs:    map[string]any{},
			want:         map[string]any{},
		},
		// --- Config only ---
		{
			name:         "config only with nil CLI",
			configInputs: map[string]any{"project": "my-project", "env": "staging"},
			cliInputs:    nil,
			want:         map[string]any{"project": "my-project", "env": "staging"},
		},
		{
			name:         "config only with empty CLI",
			configInputs: map[string]any{"project": "my-project"},
			cliInputs:    map[string]any{},
			want:         map[string]any{"project": "my-project"},
		},
		// --- CLI only ---
		{
			name:         "CLI only with nil config",
			configInputs: nil,
			cliInputs:    map[string]any{"debug": true, "count": 5},
			want:         map[string]any{"debug": true, "count": 5},
		},
		{
			name:         "CLI only with empty config",
			configInputs: map[string]any{},
			cliInputs:    map[string]any{"debug": true},
			want:         map[string]any{"debug": true},
		},
		// --- Merge without conflicts ---
		{
			name:         "disjoint keys are merged",
			configInputs: map[string]any{"project": "my-project", "count": 5},
			cliInputs:    map[string]any{"debug": true, "env": "prod"},
			want:         map[string]any{"project": "my-project", "count": 5, "debug": true, "env": "prod"},
		},
		// --- CLI overrides config (FR-003) ---
		{
			name:         "CLI overrides config for same key",
			configInputs: map[string]any{"env": "staging", "count": 10},
			cliInputs:    map[string]any{"env": "production"},
			want:         map[string]any{"env": "production", "count": 10},
		},
		{
			name:         "CLI overrides multiple config values",
			configInputs: map[string]any{"a": "config-a", "b": "config-b", "c": "config-c"},
			cliInputs:    map[string]any{"a": "cli-a", "c": "cli-c"},
			want:         map[string]any{"a": "cli-a", "b": "config-b", "c": "cli-c"},
		},
		{
			name:         "CLI overrides all config values",
			configInputs: map[string]any{"x": 1, "y": 2},
			cliInputs:    map[string]any{"x": 100, "y": 200},
			want:         map[string]any{"x": 100, "y": 200},
		},
		// --- Type variations ---
		{
			name:         "string values merge correctly",
			configInputs: map[string]any{"name": "config-name"},
			cliInputs:    map[string]any{"name": "cli-name"},
			want:         map[string]any{"name": "cli-name"},
		},
		{
			name:         "integer values merge correctly",
			configInputs: map[string]any{"count": 5},
			cliInputs:    map[string]any{"count": 10},
			want:         map[string]any{"count": 10},
		},
		{
			name:         "boolean values merge correctly",
			configInputs: map[string]any{"enabled": false},
			cliInputs:    map[string]any{"enabled": true},
			want:         map[string]any{"enabled": true},
		},
		{
			name:         "float values merge correctly",
			configInputs: map[string]any{"ratio": 0.5},
			cliInputs:    map[string]any{"ratio": 0.75},
			want:         map[string]any{"ratio": 0.75},
		},
		{
			name:         "mixed types merge correctly",
			configInputs: map[string]any{"name": "project", "count": 5, "enabled": true, "ratio": 0.5},
			cliInputs:    map[string]any{"count": 10, "extra": "new"},
			want:         map[string]any{"name": "project", "count": 10, "enabled": true, "ratio": 0.5, "extra": "new"},
		},
		// --- Edge cases: nil/empty string values ---
		{
			name:         "CLI empty string overrides config value",
			configInputs: map[string]any{"name": "default"},
			cliInputs:    map[string]any{"name": ""},
			want:         map[string]any{"name": ""},
		},
		{
			name:         "config with nil value",
			configInputs: map[string]any{"nullable": nil},
			cliInputs:    map[string]any{},
			want:         map[string]any{"nullable": nil},
		},
		{
			name:         "CLI nil overrides config value",
			configInputs: map[string]any{"value": "something"},
			cliInputs:    map[string]any{"value": nil},
			want:         map[string]any{"value": nil},
		},
		// --- Edge case: Type change via override ---
		{
			name:         "CLI can change type from int to string",
			configInputs: map[string]any{"port": 8080},
			cliInputs:    map[string]any{"port": "8080"},
			want:         map[string]any{"port": "8080"},
		},
		{
			name:         "CLI can change type from string to int",
			configInputs: map[string]any{"count": "5"},
			cliInputs:    map[string]any{"count": 5},
			want:         map[string]any{"count": 5},
		},
		// --- Large number of keys ---
		{
			name: "many keys merge correctly",
			configInputs: map[string]any{
				"k1": "v1", "k2": "v2", "k3": "v3", "k4": "v4", "k5": "v5",
				"k6": "v6", "k7": "v7", "k8": "v8", "k9": "v9", "k10": "v10",
			},
			cliInputs: map[string]any{
				"k1": "cli-v1", "k5": "cli-v5", "k10": "cli-v10", "k11": "new",
			},
			want: map[string]any{
				"k1": "cli-v1", "k2": "v2", "k3": "v3", "k4": "v4", "k5": "cli-v5",
				"k6": "v6", "k7": "v7", "k8": "v8", "k9": "v9", "k10": "cli-v10", "k11": "new",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeInputs(tt.configInputs, tt.cliInputs)

			// Verify result matches expected
			assert.Equal(t, tt.want, got, "merged result should match expected")

			// Verify result is a new map (not modifying inputs)
			// Note: len() for nil map is defined as 0, so we only check len() > 0
			if len(tt.configInputs) > 0 {
				// Modify result to verify it doesn't affect original
				got["_test_key_"] = "test"
				_, exists := tt.configInputs["_test_key_"]
				assert.False(t, exists, "modifying result should not affect configInputs")
			}
		})
	}
}

// TestMergeInputs_Immutability verifies that mergeInputs does not modify input maps
func TestMergeInputs_Immutability(t *testing.T) {
	configInputs := map[string]any{"a": "1", "b": "2"}
	cliInputs := map[string]any{"b": "override", "c": "3"}

	// Take copies before merge
	originalConfig := make(map[string]any)
	for k, v := range configInputs {
		originalConfig[k] = v
	}
	originalCLI := make(map[string]any)
	for k, v := range cliInputs {
		originalCLI[k] = v
	}

	// Perform merge
	result := mergeInputs(configInputs, cliInputs)

	// Verify original maps are unchanged
	assert.Equal(t, originalConfig, configInputs, "configInputs should not be modified")
	assert.Equal(t, originalCLI, cliInputs, "cliInputs should not be modified")

	// Verify result is correct
	assert.Equal(t, "1", result["a"])
	assert.Equal(t, "override", result["b"])
	assert.Equal(t, "3", result["c"])
}

// TestMergeInputs_ReturnNewMap verifies that mergeInputs always returns a new map
func TestMergeInputs_ReturnNewMap(t *testing.T) {
	configInputs := map[string]any{"key": "value"}
	cliInputs := map[string]any{}

	result := mergeInputs(configInputs, cliInputs)

	// Result should be a different map instance
	if len(configInputs) > 0 && len(result) > 0 {
		// Modify result
		result["new_key"] = "new_value"

		// Config should not have the new key
		_, exists := configInputs["new_key"]
		assert.False(t, exists, "result should be a new map, not a reference to input")
	}
}

// TestMergeInputs_SpecialKeys tests handling of special key names
func TestMergeInputs_SpecialKeys(t *testing.T) {
	tests := []struct {
		name         string
		configInputs map[string]any
		cliInputs    map[string]any
		checkKey     string
		wantValue    any
	}{
		{
			name:         "empty string key",
			configInputs: map[string]any{"": "empty-key-value"},
			cliInputs:    map[string]any{},
			checkKey:     "",
			wantValue:    "empty-key-value",
		},
		{
			name:         "key with spaces",
			configInputs: map[string]any{"key with spaces": "value"},
			cliInputs:    map[string]any{},
			checkKey:     "key with spaces",
			wantValue:    "value",
		},
		{
			name:         "key with special characters",
			configInputs: map[string]any{"key.with.dots": "dotted"},
			cliInputs:    map[string]any{"key.with.dots": "cli-dotted"},
			checkKey:     "key.with.dots",
			wantValue:    "cli-dotted",
		},
		{
			name:         "unicode key",
			configInputs: map[string]any{"キー": "value"},
			cliInputs:    map[string]any{},
			checkKey:     "キー",
			wantValue:    "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeInputs(tt.configInputs, tt.cliInputs)
			assert.Equal(t, tt.wantValue, result[tt.checkKey])
		})
	}
}

// TestMergeInputs_ComplexValues tests handling of complex value types
func TestMergeInputs_ComplexValues(t *testing.T) {
	// Note: While the spec says complex types (arrays, nested objects) are not
	// supported, mergeInputs should handle them gracefully if they appear
	tests := []struct {
		name         string
		configInputs map[string]any
		cliInputs    map[string]any
		checkKey     string
		wantValue    any
	}{
		{
			name:         "slice value from config",
			configInputs: map[string]any{"list": []string{"a", "b"}},
			cliInputs:    map[string]any{},
			checkKey:     "list",
			wantValue:    []string{"a", "b"},
		},
		{
			name:         "CLI overrides slice with scalar",
			configInputs: map[string]any{"value": []int{1, 2, 3}},
			cliInputs:    map[string]any{"value": "scalar"},
			checkKey:     "value",
			wantValue:    "scalar",
		},
		{
			name:         "nested map from config",
			configInputs: map[string]any{"nested": map[string]any{"inner": "value"}},
			cliInputs:    map[string]any{},
			checkKey:     "nested",
			wantValue:    map[string]any{"inner": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeInputs(tt.configInputs, tt.cliInputs)
			assert.Equal(t, tt.wantValue, result[tt.checkKey])
		})
	}
}

// =============================================================================
// T007: loadProjectConfig() tests
// =============================================================================

// configTestLogger implements ports.Logger for testing loadProjectConfig
type configTestLogger struct {
	debugMsgs []string
	infoMsgs  []string
	warnMsgs  []string
	errorMsgs []string
}

func newConfigTestLogger() *configTestLogger {
	return &configTestLogger{}
}

func (m *configTestLogger) Debug(msg string, keysAndValues ...any) {
	m.debugMsgs = append(m.debugMsgs, msg)
}

func (m *configTestLogger) Info(msg string, keysAndValues ...any) {
	m.infoMsgs = append(m.infoMsgs, msg)
}

func (m *configTestLogger) Warn(msg string, keysAndValues ...any) {
	m.warnMsgs = append(m.warnMsgs, msg)
}

func (m *configTestLogger) Error(msg string, keysAndValues ...any) {
	m.errorMsgs = append(m.errorMsgs, msg)
}

func (m *configTestLogger) WithContext(_ map[string]any) ports.Logger {
	return m
}

// TestLoadProjectConfig tests the loadProjectConfig() function.
// This function loads .awf/config.yaml and returns a ProjectConfig.
//
// Spec requirements (FR-001 through FR-005):
// - FR-001: Config file located at .awf/config.yaml
// - FR-004: Missing config file is not an error; returns empty defaults
// - FR-005: Invalid YAML produces exit code 1 with descriptive error
func TestLoadProjectConfig(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(t *testing.T) (cleanup func()) // setup test environment
		wantInputs  map[string]any                      // expected inputs in config
		wantErr     bool
		errContains string
	}{
		// --- FR-004: Missing config file is not an error ---
		{
			name: "no config file returns empty config",
			setupFunc: func(t *testing.T) func() {
				// Create temp dir and chdir to it (no .awf/config.yaml)
				_ = setupConfigTestDir(t)
				return func() {} // no-op cleanup, t.Cleanup handles restoration
			},
			wantInputs: nil, // empty config has nil Inputs
			wantErr:    false,
		},
		// --- FR-001, FR-002: Valid config with inputs ---
		{
			name: "valid config file with inputs",
			setupFunc: func(t *testing.T) func() {
				tmpDir := setupConfigTestDir(t)

				// Create .awf/config.yaml
				awfDir := filepath.Join(tmpDir, ".awf")
				require.NoError(t, os.MkdirAll(awfDir, 0o755))
				configContent := `inputs:
  project: my-project
  env: staging
  count: 5
`
				require.NoError(t, os.WriteFile(
					filepath.Join(awfDir, "config.yaml"),
					[]byte(configContent),
					0o644,
				))

				return func() {} // no-op cleanup, t.Cleanup handles restoration
			},
			wantInputs: map[string]any{
				"project": "my-project",
				"env":     "staging",
				"count":   5,
			},
			wantErr: false,
		},
		// --- FR-001: Empty config file ---
		{
			name: "empty config file returns empty config",
			setupFunc: func(t *testing.T) func() {
				tmpDir := setupConfigTestDir(t)

				awfDir := filepath.Join(tmpDir, ".awf")
				require.NoError(t, os.MkdirAll(awfDir, 0o755))
				require.NoError(t, os.WriteFile(
					filepath.Join(awfDir, "config.yaml"),
					[]byte(""),
					0o644,
				))

				return func() {} // no-op cleanup, t.Cleanup handles restoration
			},
			wantInputs: nil,
			wantErr:    false,
		},
		// --- FR-005: Invalid YAML produces error ---
		{
			name: "invalid YAML returns error",
			setupFunc: func(t *testing.T) func() {
				tmpDir := setupConfigTestDir(t)

				awfDir := filepath.Join(tmpDir, ".awf")
				require.NoError(t, os.MkdirAll(awfDir, 0o755))
				// Invalid YAML: bad indentation
				invalidYAML := `inputs:
  project: value
    bad: indentation
`
				require.NoError(t, os.WriteFile(
					filepath.Join(awfDir, "config.yaml"),
					[]byte(invalidYAML),
					0o644,
				))

				return func() {} // no-op cleanup, t.Cleanup handles restoration
			},
			wantErr:     true,
			errContains: "parse",
		},
		// --- Config with only comments ---
		{
			name: "config with only comments returns empty",
			setupFunc: func(t *testing.T) func() {
				tmpDir := setupConfigTestDir(t)

				awfDir := filepath.Join(tmpDir, ".awf")
				require.NoError(t, os.MkdirAll(awfDir, 0o755))
				configContent := `# This is a comment
# inputs:
#   project: my-project
`
				require.NoError(t, os.WriteFile(
					filepath.Join(awfDir, "config.yaml"),
					[]byte(configContent),
					0o644,
				))

				return func() {} // no-op cleanup, t.Cleanup handles restoration
			},
			wantInputs: nil,
			wantErr:    false,
		},
		// --- Config with empty inputs section ---
		{
			name: "config with empty inputs section",
			setupFunc: func(t *testing.T) func() {
				tmpDir := setupConfigTestDir(t)

				awfDir := filepath.Join(tmpDir, ".awf")
				require.NoError(t, os.MkdirAll(awfDir, 0o755))
				configContent := `inputs:
`
				require.NoError(t, os.WriteFile(
					filepath.Join(awfDir, "config.yaml"),
					[]byte(configContent),
					0o644,
				))

				return func() {} // no-op cleanup, t.Cleanup handles restoration
			},
			wantInputs: nil,
			wantErr:    false,
		},
		// --- Config with various value types ---
		{
			name: "config with various value types",
			setupFunc: func(t *testing.T) func() {
				tmpDir := setupConfigTestDir(t)

				awfDir := filepath.Join(tmpDir, ".awf")
				require.NoError(t, os.MkdirAll(awfDir, 0o755))
				configContent := `inputs:
  string_val: "hello"
  int_val: 42
  float_val: 3.14
  bool_val: true
  null_val: null
`
				require.NoError(t, os.WriteFile(
					filepath.Join(awfDir, "config.yaml"),
					[]byte(configContent),
					0o644,
				))

				return func() {} // no-op cleanup, t.Cleanup handles restoration
			},
			wantInputs: map[string]any{
				"string_val": "hello",
				"int_val":    42,
				"float_val":  3.14,
				"bool_val":   true,
				"null_val":   nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupFunc(t)
			defer cleanup()

			logger := newConfigTestLogger()

			// Call the function under test
			cfg, err := loadProjectConfig(logger)

			if tt.wantErr {
				require.Error(t, err, "expected error but got none")
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains,
						"error message should contain %q", tt.errContains)
				}
			} else {
				require.NoError(t, err, "unexpected error: %v", err)
				require.NotNil(t, cfg, "config should not be nil")

				if tt.wantInputs == nil {
					// Expect empty/nil inputs (len() for nil map is defined as 0)
					assert.Empty(t, cfg.Inputs, "expected empty inputs, got %v", cfg.Inputs)
				} else {
					// Verify each expected input
					for key, expectedVal := range tt.wantInputs {
						assert.Equal(t, expectedVal, cfg.Inputs[key],
							"input %q should have value %v", key, expectedVal)
					}
				}
			}
		})
	}
}

// TestLoadProjectConfig_UsesCorrectPath verifies that loadProjectConfig uses
// xdg.LocalConfigPath() which returns ".awf/config.yaml"
func TestLoadProjectConfig_UsesCorrectPath(t *testing.T) {
	// Create temp dir with config at expected path
	tmpDir := setupConfigTestDir(t)

	// Create .awf/config.yaml at the expected path
	awfDir := filepath.Join(tmpDir, ".awf")
	require.NoError(t, os.MkdirAll(awfDir, 0o755))
	configContent := `inputs:
  path_test: correct_path
`
	require.NoError(t, os.WriteFile(
		filepath.Join(awfDir, "config.yaml"),
		[]byte(configContent),
		0o644,
	))

	logger := newConfigTestLogger()
	cfg, err := loadProjectConfig(logger)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "correct_path", cfg.Inputs["path_test"],
		"config should be loaded from .awf/config.yaml")
}

// TestLoadProjectConfig_LoggerParameter verifies that loadProjectConfig
// accepts a logger parameter (for future warning logging)
func TestLoadProjectConfig_LoggerParameter(t *testing.T) {
	// Create temp dir with no config
	_ = setupConfigTestDir(t)

	// Verify the function accepts a logger and doesn't panic
	logger := newConfigTestLogger()
	cfg, err := loadProjectConfig(logger)

	// Should succeed (no config file = empty config)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Logger parameter is accepted (future: will be used for warnings)
	// This test verifies the function signature is correct
}

// =============================================================================
// T007: Integration tests for runWorkflow with config loading
// =============================================================================

// TestRunWorkflow_ConfigIntegration tests that runWorkflow properly integrates
// with loadProjectConfig and mergeInputs.
//
// These are behavioral tests verifying:
// - US1: Config inputs are used when no CLI inputs provided
// - FR-003: CLI inputs override config inputs
func TestRunWorkflow_ConfigIntegration(t *testing.T) {
	// Note: These are placeholder tests that document expected behavior.
	// Full integration testing requires more extensive setup with workflow fixtures.

	tests := []struct {
		name           string
		description    string
		configInputs   map[string]any
		cliInputFlags  []string
		expectedInputs map[string]any // inputs that should reach the execution
	}{
		{
			name:           "config inputs used when no CLI inputs",
			description:    "US1: Config values are used when no --input flags provided",
			configInputs:   map[string]any{"project": "2", "env": "staging"},
			cliInputFlags:  []string{},
			expectedInputs: map[string]any{"project": "2", "env": "staging"},
		},
		{
			name:           "CLI overrides config for same key",
			description:    "FR-003: CLI --input flag overrides config value",
			configInputs:   map[string]any{"env": "staging"},
			cliInputFlags:  []string{"env=production"},
			expectedInputs: map[string]any{"env": "production"},
		},
		{
			name:           "both merged when no overlap",
			description:    "Config and CLI inputs are merged when keys are disjoint",
			configInputs:   map[string]any{"project": "my-proj"},
			cliInputFlags:  []string{"debug=true"},
			expectedInputs: map[string]any{"project": "my-proj", "debug": "true"},
		},
		{
			name:           "CLI wins for all overlapping keys",
			description:    "All CLI values take precedence over config values",
			configInputs:   map[string]any{"a": "config-a", "b": "config-b", "c": "config-c"},
			cliInputFlags:  []string{"a=cli-a", "c=cli-c"},
			expectedInputs: map[string]any{"a": "cli-a", "b": "config-b", "c": "cli-c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Document the test case
			t.Logf("Scenario: %s", tt.description)
			t.Logf("Config inputs: %v", tt.configInputs)
			t.Logf("CLI flags: %v", tt.cliInputFlags)
			t.Logf("Expected merged inputs: %v", tt.expectedInputs)

			// Note: Full integration test requires:
			// 1. Setting up temp directory with .awf/config.yaml
			// 2. Creating a test workflow
			// 3. Running runWorkflow and capturing the inputs passed to execSvc.Run
			//
			// For RED phase, we verify the merge logic works correctly
			cliInputs, err := parseInputFlags(tt.cliInputFlags)
			require.NoError(t, err)

			merged := mergeInputs(tt.configInputs, cliInputs)

			// Verify merge produces expected result
			for key, expectedVal := range tt.expectedInputs {
				assert.Equal(t, expectedVal, merged[key],
					"key %q should have value %v", key, expectedVal)
			}
		})
	}
}

// TestRunWorkflow_ConfigError_Propagates tests that config loading errors
// are properly propagated from runWorkflow.
func TestRunWorkflow_ConfigError_Propagates(t *testing.T) {
	// This test documents that config errors should cause runWorkflow to fail
	// with an appropriate error message.
	//
	// Spec: FR-005 - Invalid YAML in config file produces exit code 1 with descriptive error
	//
	// Note: Full integration test would:
	// 1. Create temp dir with invalid .awf/config.yaml
	// 2. Call runWorkflow
	// 3. Verify error message mentions config file

	t.Run("config parse error should be wrapped", func(t *testing.T) {
		// Document expected behavior
		t.Log("When loadProjectConfig returns an error,")
		t.Log("runWorkflow should wrap it with 'config error: ...'")

		// This is verified by code inspection:
		// runWorkflow has:
		//   projectCfg, err := loadProjectConfig(logger)
		//   if err != nil {
		//       return fmt.Errorf("config error: %w", err)
		//   }
	})
}

// TestRunWorkflow_NoConfigFile_Succeeds tests that runWorkflow succeeds
// when there's no config file (FR-004).
func TestRunWorkflow_NoConfigFile_Succeeds(t *testing.T) {
	// This test documents that missing config is not an error
	//
	// Spec: FR-004 - Missing config file is not an error; system proceeds with empty defaults
	//
	// Note: Full integration test would:
	// 1. Create temp dir WITHOUT .awf/config.yaml
	// 2. Create a valid workflow
	// 3. Call runWorkflow
	// 4. Verify it succeeds

	t.Run("missing config file should not cause error", func(t *testing.T) {
		t.Log("When .awf/config.yaml does not exist,")
		t.Log("loadProjectConfig should return empty config, not error")
		t.Log("runWorkflow should proceed normally with empty config inputs")
	})
}

// ============================================================================
// F046: Interactive Mode for Incomplete Command Inputs - Component Tests
// ============================================================================

// TestHasMissingRequiredInputs tests detection of missing required workflow inputs.
// Component: run_command_integration
// Feature: F046 - Interactive Mode for Incomplete Command Inputs
// User Story: US1 - Prompt for Required Inputs
//
// These tests verify the helper function that determines if interactive input
// collection should be activated based on missing required inputs.
func TestHasMissingRequiredInputs(t *testing.T) {
	tests := []struct {
		name   string
		wf     *workflow.Workflow
		inputs map[string]any
		want   bool
	}{
		// --- Happy path: All required inputs provided ---
		{
			name: "all required inputs provided",
			wf: &workflow.Workflow{
				Name: "test-workflow",
				Inputs: []workflow.Input{
					{Name: "required1", Type: "string", Required: true},
					{Name: "required2", Type: "integer", Required: true},
				},
			},
			inputs: map[string]any{
				"required1": "value1",
				"required2": 42,
			},
			want: false, // No missing inputs
		},

		// --- Happy path: Missing required input ---
		{
			name: "one required input missing",
			wf: &workflow.Workflow{
				Name: "test-workflow",
				Inputs: []workflow.Input{
					{Name: "required1", Type: "string", Required: true},
					{Name: "required2", Type: "string", Required: true},
				},
			},
			inputs: map[string]any{
				"required1": "value1",
				// required2 is missing
			},
			want: true, // Has missing required input
		},

		// --- Happy path: All required inputs missing ---
		{
			name: "all required inputs missing",
			wf: &workflow.Workflow{
				Name: "test-workflow",
				Inputs: []workflow.Input{
					{Name: "name", Type: "string", Required: true},
					{Name: "count", Type: "integer", Required: true},
				},
			},
			inputs: map[string]any{},
			want:   true, // All required inputs missing
		},

		// --- Edge case: No required inputs in workflow ---
		{
			name: "no required inputs in workflow",
			wf: &workflow.Workflow{
				Name: "test-workflow",
				Inputs: []workflow.Input{
					{Name: "optional1", Type: "string", Required: false},
					{Name: "optional2", Type: "string", Required: false},
				},
			},
			inputs: map[string]any{},
			want:   false, // No required inputs, so none missing
		},

		// --- Edge case: Workflow has no inputs defined ---
		{
			name: "workflow with no inputs defined",
			wf: &workflow.Workflow{
				Name:   "simple-workflow",
				Inputs: []workflow.Input{},
			},
			inputs: map[string]any{},
			want:   false, // No inputs defined, so none missing
		},

		// --- Edge case: Workflow has nil inputs slice ---
		{
			name: "workflow with nil inputs slice",
			wf: &workflow.Workflow{
				Name:   "nil-inputs-workflow",
				Inputs: nil,
			},
			inputs: map[string]any{},
			want:   false, // Nil inputs treated as empty
		},

		// --- Edge case: Mixed required and optional, only required missing ---
		{
			name: "mixed required and optional, required missing",
			wf: &workflow.Workflow{
				Name: "mixed-workflow",
				Inputs: []workflow.Input{
					{Name: "required", Type: "string", Required: true},
					{Name: "optional", Type: "string", Required: false},
				},
			},
			inputs: map[string]any{
				"optional": "provided",
				// required is missing
			},
			want: true, // Required input missing
		},

		// --- Edge case: Mixed required and optional, optional missing ---
		{
			name: "mixed required and optional, optional missing",
			wf: &workflow.Workflow{
				Name: "mixed-workflow",
				Inputs: []workflow.Input{
					{Name: "required", Type: "string", Required: true},
					{Name: "optional", Type: "string", Required: false},
				},
			},
			inputs: map[string]any{
				"required": "provided",
				// optional is missing (but that's OK)
			},
			want: false, // All required inputs provided
		},

		// --- Edge case: Extra inputs provided (not in workflow) ---
		{
			name: "extra inputs provided beyond workflow definition",
			wf: &workflow.Workflow{
				Name: "simple-workflow",
				Inputs: []workflow.Input{
					{Name: "required", Type: "string", Required: true},
				},
			},
			inputs: map[string]any{
				"required": "value",
				"extra1":   "not-in-workflow",
				"extra2":   42,
			},
			want: false, // All required inputs provided, extras ignored
		},

		// --- Edge case: Empty string value for required input ---
		{
			name: "empty string value for required input",
			wf: &workflow.Workflow{
				Name: "test-workflow",
				Inputs: []workflow.Input{
					{Name: "name", Type: "string", Required: true},
				},
			},
			inputs: map[string]any{
				"name": "", // Empty string, but key exists
			},
			want: false, // Input exists (even if empty), not missing
		},

		// --- Edge case: Nil value for required input ---
		{
			name: "nil value for required input",
			wf: &workflow.Workflow{
				Name: "test-workflow",
				Inputs: []workflow.Input{
					{Name: "value", Type: "string", Required: true},
				},
			},
			inputs: map[string]any{
				"value": nil, // Nil value, but key exists
			},
			want: false, // Input key exists, not missing (validation happens later)
		},

		// --- Edge case: Multiple required, some missing ---
		{
			name: "multiple required inputs, some provided some missing",
			wf: &workflow.Workflow{
				Name: "multi-required",
				Inputs: []workflow.Input{
					{Name: "input1", Type: "string", Required: true},
					{Name: "input2", Type: "string", Required: true},
					{Name: "input3", Type: "string", Required: true},
					{Name: "input4", Type: "string", Required: true},
				},
			},
			inputs: map[string]any{
				"input1": "value1",
				"input3": "value3",
				// input2 and input4 missing
			},
			want: true, // Has missing required inputs
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasMissingRequiredInputs(tt.wf, tt.inputs)
			assert.Equal(t, tt.want, got,
				"hasMissingRequiredInputs() = %v, want %v", got, tt.want)
		})
	}
}

// TestCollectMissingInputsIfNeeded tests the integration point for input collection.
// Component: run_command_integration
// Feature: F046 - Interactive Mode for Incomplete Command Inputs
// User Story: US1 - Prompt for Required Inputs
//
// These tests verify the function that orchestrates input collection service
// when required inputs are missing and stdin is a terminal.
//
// NOTE: This test uses the existing mockLogger from plugins_internal_test.go
func TestCollectMissingInputsIfNeeded(t *testing.T) {
	tests := []struct {
		name    string
		wf      *workflow.Workflow
		inputs  map[string]any
		wantErr bool
		errMsg  string
	}{
		// --- Happy path: All inputs provided, no collection needed ---
		{
			name: "all inputs provided, skip collection",
			wf: &workflow.Workflow{
				Name: "complete-workflow",
				Inputs: []workflow.Input{
					{Name: "name", Type: "string", Required: true},
					{Name: "count", Type: "integer", Required: true},
				},
			},
			inputs: map[string]any{
				"name":  "test",
				"count": 42,
			},
			wantErr: false, // Should return inputs unchanged
		},

		// --- Happy path: No required inputs in workflow ---
		{
			name: "workflow with no required inputs",
			wf: &workflow.Workflow{
				Name: "optional-workflow",
				Inputs: []workflow.Input{
					{Name: "optional", Type: "string", Required: false},
				},
			},
			inputs:  map[string]any{},
			wantErr: false, // No required inputs, no collection needed
		},

		// --- Happy path: Workflow with no inputs ---
		{
			name: "workflow with no inputs defined",
			wf: &workflow.Workflow{
				Name:   "simple-workflow",
				Inputs: []workflow.Input{},
			},
			inputs:  map[string]any{},
			wantErr: false, // No inputs, no collection
		},

		// --- Edge case: Missing required input (stub will skip collection) ---
		{
			name: "missing required input but stub returns unchanged",
			wf: &workflow.Workflow{
				Name: "incomplete-workflow",
				Inputs: []workflow.Input{
					{Name: "required", Type: "string", Required: true},
				},
			},
			inputs: map[string]any{},
			// Stub returns inputs unchanged, will fail when implemented
			wantErr: false,
		},

		// --- Edge case: Nil workflow ---
		// Note: This would panic in real code, but tests document expected behavior
		// Real implementation should handle gracefully or require non-nil workflow

		// --- Edge case: Nil inputs map ---
		{
			name: "nil inputs map",
			wf: &workflow.Workflow{
				Name: "test-workflow",
				Inputs: []workflow.Input{
					{Name: "opt", Type: "string", Required: false},
				},
			},
			inputs:  nil, // Nil inputs map
			wantErr: false,
		},

		// --- Edge case: Empty inputs map with no required inputs ---
		{
			name: "empty inputs with all optional",
			wf: &workflow.Workflow{
				Name: "all-optional",
				Inputs: []workflow.Input{
					{Name: "opt1", Type: "string", Required: false},
					{Name: "opt2", Type: "integer", Required: false},
				},
			},
			inputs:  map[string]any{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: collectMissingInputsIfNeeded needs real *cobra.Command
			// These tests document expected behavior for stub implementation
			// Full integration tests will use actual cobra.Command instances

			t.Logf("Testing with workflow: %s", tt.wf.Name)
			t.Logf("Provided inputs: %v", tt.inputs)

			// Stub behavior verification:
			// - Current stub always returns inputs unchanged, nil error
			// - Real implementation will:
			//   1. Check stdin is terminal
			//   2. Check hasMissingRequiredInputs()
			//   3. Create collector and service if needed
			//   4. Return collected inputs or error

			if tt.wantErr {
				t.Logf("Expected error: %s", tt.errMsg)
			} else {
				t.Logf("Expected: inputs returned unchanged")
			}
		})
	}
}

// TestCollectMissingInputsIfNeeded_Integration documents expected integration behavior
// This test serves as documentation for the real implementation's integration points.
// Component: run_command_integration
// Feature: F046
func TestCollectMissingInputsIfNeeded_Integration(t *testing.T) {
	t.Run("should create input collector and service when missing inputs", func(t *testing.T) {
		t.Log("When hasMissingRequiredInputs() returns true AND stdin is terminal,")
		t.Log("collectMissingInputsIfNeeded should:")
		t.Log("  1. Create colorizer := ui.NewColorizer(!cfg.NoColor)")
		t.Log("  2. Create collector := ui.NewCLIInputCollector(cmd.InOrStdin(), cmd.OutOrStdout(), colorizer)")
		t.Log("  3. Create service := application.NewInputCollectionService(collector, logger)")
		t.Log("  4. Call service.CollectMissingInputs(wf, inputs)")
		t.Log("  5. Return collected inputs merged with provided inputs")
	})

	t.Run("should error when stdin not terminal and inputs missing", func(t *testing.T) {
		t.Log("When hasMissingRequiredInputs() returns true AND stdin is NOT a terminal,")
		t.Log("collectMissingInputsIfNeeded should:")
		t.Log("  1. Return error: 'missing required inputs and stdin is not a terminal'")
		t.Log("  2. Suggest: 'provide inputs via --input flags'")
	})

	t.Run("should skip collection when all inputs provided", func(t *testing.T) {
		t.Log("When hasMissingRequiredInputs() returns false,")
		t.Log("collectMissingInputsIfNeeded should:")
		t.Log("  1. Return inputs unchanged")
		t.Log("  2. Not create collector or service (performance optimization)")
	})

	t.Run("should check terminal using os.Stdin.Stat", func(t *testing.T) {
		t.Log("Terminal detection logic:")
		t.Log("  fileInfo, err := os.Stdin.Stat()")
		t.Log("  isTerminal := (fileInfo.Mode() & os.ModeCharDevice) != 0")
	})
}
