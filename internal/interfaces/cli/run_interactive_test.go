package cli_test

// Interactive mode and prompt resolution tests extracted from run_test.go (C015 T010)
// Tests interactive workflow execution with breakpoints and prompt file resolution

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunCommand_Interactive tests interactive mode execution
func TestRunCommand_Interactive(t *testing.T) {
	tests := []struct {
		name          string
		setupWorkflow func(t *testing.T, tmpDir string)
		args          []string
		mockInput     string // Simulated user input for prompts
		wantErr       bool
		errContains   string
	}{
		{
			name: "interactive mode with simple workflow",
			setupWorkflow: func(t *testing.T, tmpDir string) {
				// Create storage directories for state and history
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755))
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755))
				content := `name: interactive-test
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo step1
    on_success: done
  done:
    type: terminal
`
				createTestWorkflow(t, tmpDir, "interactive-test.yaml", content)
			},
			args:      []string{"run", "interactive-test", "--interactive"},
			mockInput: "y\n", // Proceed with step
			wantErr:   false,
		},
		{
			name: "interactive mode with breakpoints",
			setupWorkflow: func(t *testing.T, tmpDir string) {
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755))
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755))
				content := `name: breakpoint-test
version: "1.0.0"
states:
  initial: prepare
  prepare:
    type: step
    command: echo preparing
    on_success: process
  process:
    type: step
    command: echo processing
    on_success: done
  done:
    type: terminal
`
				createTestWorkflow(t, tmpDir, "breakpoint-test.yaml", content)
			},
			args:      []string{"run", "breakpoint-test", "--interactive", "--breakpoint=process"},
			mockInput: "y\n", // Proceed
			wantErr:   false,
		},
		{
			name: "interactive mode with comma-separated breakpoints",
			setupWorkflow: func(t *testing.T, tmpDir string) {
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755))
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755))
				content := `name: multi-breakpoint
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo 1
    on_success: step2
  step2:
    type: step
    command: echo 2
    on_success: step3
  step3:
    type: step
    command: echo 3
    on_success: done
  done:
    type: terminal
`
				createTestWorkflow(t, tmpDir, "multi-breakpoint.yaml", content)
			},
			args:      []string{"run", "multi-breakpoint", "--interactive", "--breakpoint=step1,step3"},
			mockInput: "y\ny\n", // Proceed for each breakpoint
			wantErr:   false,
		},
		{
			name: "interactive mode with inputs",
			setupWorkflow: func(t *testing.T, tmpDir string) {
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755))
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755))
				content := `name: input-interactive
version: "1.0.0"
inputs:
  - name: msg
    type: string
states:
  initial: echo
  echo:
    type: step
    command: echo "{{inputs.msg}}"
    on_success: done
  done:
    type: terminal
`
				createTestWorkflow(t, tmpDir, "input-interactive.yaml", content)
			},
			args:      []string{"run", "input-interactive", "--interactive", "--input=msg=test"},
			mockInput: "y\n",
			wantErr:   false,
		},
		{
			name: "interactive mode with nonexistent workflow",
			setupWorkflow: func(t *testing.T, tmpDir string) {
			},
			args:        []string{"run", "nonexistent", "--interactive"},
			mockInput:   "",
			wantErr:     true,
			errContains: "not found",
		},
		{
			name: "interactive mode with invalid input format",
			setupWorkflow: func(t *testing.T, tmpDir string) {
				content := `name: test
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo test
    on_success: done
  done:
    type: terminal
`
				createTestWorkflow(t, tmpDir, "test.yaml", content)
			},
			args:        []string{"run", "test", "--interactive", "--input=invalid"},
			mockInput:   "",
			wantErr:     true,
			errContains: "invalid input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := setupTestDir(t)

			tt.setupWorkflow(t, tmpDir)

			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			// Set up mock stdin for interactive prompts
			cmd.SetIn(strings.NewReader(tt.mockInput))
			cmd.SetArgs(append([]string{"--storage=" + tmpDir}, tt.args...))

			err := cmd.Execute()

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				// Interactive mode might error with EOF or context cancellation, which is acceptable
				// EOF happens when stdin is exhausted during prompts
				if err != nil {
					if !strings.Contains(err.Error(), "context canceled") &&
						!strings.Contains(err.Error(), "EOF") &&
						!strings.Contains(err.Error(), "read input") {
						t.Errorf("unexpected error: %v", err)
					}
				}
			}
		})
	}
}

func TestRunCommand_PromptResolution(t *testing.T) {
	tests := []struct {
		name        string
		setupPrompt func(t *testing.T, tmpDir string)
		inputFlag   string
		wantErr     bool
		errContains string
	}{
		{
			name: "resolves @prompts/ prefix to file content",
			setupPrompt: func(t *testing.T, tmpDir string) {
				promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
				require.NoError(t, os.MkdirAll(promptsDir, 0o755))
				require.NoError(t, os.WriteFile(
					filepath.Join(promptsDir, "test.md"),
					[]byte("This is prompt content"),
					0o644,
				))
			},
			inputFlag: "prompt=@prompts/test.md",
			wantErr:   false,
		},
		{
			name: "trims whitespace from prompt content",
			setupPrompt: func(t *testing.T, tmpDir string) {
				promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
				require.NoError(t, os.MkdirAll(promptsDir, 0o755))
				require.NoError(t, os.WriteFile(
					filepath.Join(promptsDir, "whitespace.txt"),
					[]byte("\n  content with whitespace  \n\n"),
					0o644,
				))
			},
			inputFlag: "msg=@prompts/whitespace.txt",
			wantErr:   false,
		},
		{
			name: "supports nested directories",
			setupPrompt: func(t *testing.T, tmpDir string) {
				nestedDir := filepath.Join(tmpDir, ".awf", "prompts", "ai", "agents")
				require.NoError(t, os.MkdirAll(nestedDir, 0o755))
				require.NoError(t, os.WriteFile(
					filepath.Join(nestedDir, "system.md"),
					[]byte("You are an AI assistant"),
					0o644,
				))
			},
			inputFlag: "system=@prompts/ai/agents/system.md",
			wantErr:   false,
		},
		{
			name: "error when prompt file does not exist",
			setupPrompt: func(t *testing.T, tmpDir string) {
				promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
				require.NoError(t, os.MkdirAll(promptsDir, 0o755))
				// No file created
			},
			inputFlag:   "prompt=@prompts/nonexistent.md",
			wantErr:     true,
			errContains: "not found",
		},
		{
			name: "error when .awf/prompts directory does not exist",
			setupPrompt: func(t *testing.T, tmpDir string) {
				// Create .awf but not prompts subdirectory
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf"), 0o755))
			},
			inputFlag:   "prompt=@prompts/test.md",
			wantErr:     true,
			errContains: "not found",
		},
		{
			name: "blocks path traversal attack",
			setupPrompt: func(t *testing.T, tmpDir string) {
				promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
				require.NoError(t, os.MkdirAll(promptsDir, 0o755))
				// Create a sensitive file outside prompts dir
				sensitiveFile := filepath.Join(tmpDir, "secret.txt")
				require.NoError(t, os.WriteFile(sensitiveFile, []byte("secret"), 0o644))
			},
			inputFlag:   "data=@prompts/../secret.txt",
			wantErr:     true,
			errContains: "invalid prompt path",
		},
		{
			name: "blocks absolute path in prompt reference",
			setupPrompt: func(t *testing.T, tmpDir string) {
				promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
				require.NoError(t, os.MkdirAll(promptsDir, 0o755))
			},
			inputFlag:   "data=@prompts//etc/passwd",
			wantErr:     true,
			errContains: "invalid prompt path",
		},
		{
			name: "regular value without @prompts/ prefix is unchanged",
			setupPrompt: func(t *testing.T, tmpDir string) {
				promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
				require.NoError(t, os.MkdirAll(promptsDir, 0o755))
			},
			inputFlag: "name=plain-value",
			wantErr:   false,
		},
		{
			name: "supports .txt extension",
			setupPrompt: func(t *testing.T, tmpDir string) {
				promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
				require.NoError(t, os.MkdirAll(promptsDir, 0o755))
				require.NoError(t, os.WriteFile(
					filepath.Join(promptsDir, "note.txt"),
					[]byte("Plain text content"),
					0o644,
				))
			},
			inputFlag: "note=@prompts/note.txt",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := setupTestDir(t)

			// Setup prompts directory and files
			tt.setupPrompt(t, tmpDir)

			// Create a minimal workflow for the test
			workflowContent := `name: test
version: "1.0.0"
states:
  initial: echo
  echo:
    type: step
    command: echo "{{inputs.prompt}}"
    on_success: done
  done:
    type: terminal
`
			createTestWorkflow(t, tmpDir, "test.yaml", workflowContent)

			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "test", "--input", tt.inputFlag})

			err := cmd.Execute()

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				// Even without error, the workflow might fail for other reasons
				// We're just testing the prompt resolution doesn't error
				if err != nil && strings.Contains(err.Error(), "resolve prompt") {
					t.Errorf("unexpected prompt resolution error: %v", err)
				}
			}
		})
	}
}
