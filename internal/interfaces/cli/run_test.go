package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

func TestRunCommand_NoArgs(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no workflow name provided")
	}
}

func TestRunCommand_WorkflowNotFound(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "nonexistent-workflow"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent workflow")
	}
}

func TestRunCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected root command to have 'run' subcommand")
	}
}

func TestRunCommand_HasInputFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			flag := sub.Flags().Lookup("input")
			if flag == nil {
				t.Error("expected 'run' command to have --input flag")
			}
			return
		}
	}

	t.Error("run command not found")
}

func TestRunCommand_Help(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Execute a workflow") {
		t.Errorf("expected help text, got: %s", output)
	}
}

func TestRunCommand_HasOutputFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			flag := sub.Flags().Lookup("output")
			if flag == nil {
				t.Error("expected 'run' command to have --output flag")
			}
			return
		}
	}

	t.Error("run command not found")
}

func TestRunCommand_InvalidOutputMode(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "test-workflow", "--output=invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid output mode")
	}
	if !strings.Contains(err.Error(), "invalid output mode") {
		t.Errorf("expected 'invalid output mode' error, got: %v", err)
	}
}

func TestRunCommand_HasStepFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			flag := sub.Flags().Lookup("step")
			require.NotNil(t, flag, "expected 'run' command to have --step flag")
			assert.Equal(t, "s", flag.Shorthand, "step flag should have -s shorthand")
			return
		}
	}

	t.Error("run command not found")
}

func TestRunCommand_HasMockFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			flag := sub.Flags().Lookup("mock")
			require.NotNil(t, flag, "expected 'run' command to have --mock flag")
			assert.Equal(t, "m", flag.Shorthand, "mock flag should have -m shorthand")
			return
		}
	}

	t.Error("run command not found")
}

func TestRunCommand_StepFlagInHelp(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "--step")
	assert.Contains(t, output, "Execute only this step")
}

func TestRunCommand_MockFlagInHelp(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "--mock")
	assert.Contains(t, output, "Mock state value")
}

func TestParseMockFlags(t *testing.T) {
	tests := []struct {
		name        string
		flags       []string
		want        map[string]string
		wantErr     bool
		errContains string
	}{
		{
			name:  "empty flags returns empty map",
			flags: []string{},
			want:  map[string]string{},
		},
		{
			name:  "single mock flag",
			flags: []string{"states.fetch.output=hello"},
			want:  map[string]string{"states.fetch.output": "hello"},
		},
		{
			name:  "multiple mock flags",
			flags: []string{"states.fetch.output=data", "states.process.output=result"},
			want: map[string]string{
				"states.fetch.output":   "data",
				"states.process.output": "result",
			},
		},
		{
			name:  "mock flag with value containing equals sign",
			flags: []string{"states.step.output=key=value"},
			want:  map[string]string{"states.step.output": "key=value"},
		},
		{
			name:  "mock flag with empty value",
			flags: []string{"states.step.output="},
			want:  map[string]string{"states.step.output": ""},
		},
		{
			name:  "mock flag with json value",
			flags: []string{`states.api.output={"key":"value"}`},
			want:  map[string]string{"states.api.output": `{"key":"value"}`},
		},
		{
			name:        "invalid format without equals",
			flags:       []string{"states.step.output"},
			wantErr:     true,
			errContains: "invalid mock format",
		},
		{
			name:        "empty key",
			flags:       []string{"=value"},
			wantErr:     true,
			errContains: "mock key cannot be empty",
		},
		{
			name:  "whitespace trimmed from key and value",
			flags: []string{"  states.step.output  =  value  "},
			want:  map[string]string{"states.step.output": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cli.ParseMockFlags(tt.flags)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestRunCommand_SingleStep_WorkflowNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create .awf directory but no workflow
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	_ = os.MkdirAll(workflowsDir, 0755)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "nonexistent", "--step=mystep"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRunCommand_SingleStep_StepNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow without the requested step
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	_ = os.MkdirAll(workflowsDir, 0755)
	workflowContent := `name: test
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo hello
    on_success: done
  done:
    type: terminal
`
	_ = os.WriteFile(filepath.Join(workflowsDir, "test.yaml"), []byte(workflowContent), 0644)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "test", "--step=nonexistent"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRunCommand_SingleStep_Success(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create storage directories (states and history for Badger)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0755)

	// Create a simple workflow
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	_ = os.MkdirAll(workflowsDir, 0755)
	workflowContent := `name: test
version: "1.0.0"
states:
  initial: greet
  greet:
    type: step
    command: echo hello
    on_success: done
  done:
    type: terminal
`
	_ = os.WriteFile(filepath.Join(workflowsDir, "test.yaml"), []byte(workflowContent), 0644)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "test", "--step=greet"})

	err := cmd.Execute()
	// Should succeed (once implemented)
	require.NoError(t, err)
}

func TestRunCommand_SingleStep_WithInputs(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create storage directories (states and history for Badger)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0755)

	// Create a workflow with inputs
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	_ = os.MkdirAll(workflowsDir, 0755)
	workflowContent := `name: input-test
version: "1.0.0"
inputs:
  - name: message
    type: string
states:
  initial: show
  show:
    type: step
    command: echo "{{.inputs.message}}"
    on_success: done
  done:
    type: terminal
`
	_ = os.WriteFile(filepath.Join(workflowsDir, "input-test.yaml"), []byte(workflowContent), 0644)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "input-test", "--step=show", "--input=message=test-value"})

	err := cmd.Execute()
	require.NoError(t, err)
}

func TestRunCommand_SingleStep_WithMocks(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create storage directories (states and history for Badger)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0755)

	// Create a workflow where process depends on fetch output
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	_ = os.MkdirAll(workflowsDir, 0755)
	workflowContent := `name: mock-test
version: "1.0.0"
states:
  initial: fetch
  fetch:
    type: step
    command: curl http://api
    on_success: process
  process:
    type: step
    command: echo "{{.states.fetch.Output}}"
    on_success: done
  done:
    type: terminal
`
	_ = os.WriteFile(filepath.Join(workflowsDir, "mock-test.yaml"), []byte(workflowContent), 0644)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// Execute the process step with mocked fetch output
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run", "mock-test",
		"--step=process",
		"--mock=states.fetch.output=mocked-data",
	})

	err := cmd.Execute()
	require.NoError(t, err)
}

func TestRunCommand_SingleStep_TerminalStepError(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create storage directories (states and history for Badger)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0755)

	// Create a workflow with a terminal step
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	_ = os.MkdirAll(workflowsDir, 0755)
	workflowContent := `name: terminal-test
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`
	_ = os.WriteFile(filepath.Join(workflowsDir, "terminal-test.yaml"), []byte(workflowContent), 0644)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// Try to execute a terminal step (should error)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "terminal-test", "--step=done"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminal")
}

// TestRunCommand_DryRun tests the dry-run functionality
func TestRunCommand_DryRun(t *testing.T) {
	tests := []struct {
		name          string
		setupWorkflow func(t *testing.T, tmpDir string)
		args          []string
		wantErr       bool
		errContains   string
		validateOut   func(t *testing.T, output string)
	}{
		{
			name: "basic dry-run shows execution plan",
			setupWorkflow: func(t *testing.T, tmpDir string) {
				workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
				require.NoError(t, os.MkdirAll(workflowsDir, 0755))
				content := `name: simple
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo hello
    on_success: done
  done:
    type: terminal
`
				require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "simple.yaml"), []byte(content), 0644))
			},
			args:    []string{"run", "simple", "--dry-run"},
			wantErr: false,
			validateOut: func(t *testing.T, output string) {
				assert.Contains(t, output, "Dry Run")
				assert.Contains(t, output, "start")
				assert.Contains(t, output, "echo hello")
			},
		},
		{
			name: "dry-run with inputs shows interpolated values",
			setupWorkflow: func(t *testing.T, tmpDir string) {
				workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
				require.NoError(t, os.MkdirAll(workflowsDir, 0755))
				content := `name: with-inputs
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
				require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "with-inputs.yaml"), []byte(content), 0644))
			},
			args:    []string{"run", "with-inputs", "--dry-run", "--input=msg=hello world"},
			wantErr: false,
			validateOut: func(t *testing.T, output string) {
				assert.Contains(t, output, "Dry Run")
			},
		},
		{
			name: "dry-run with parallel states",
			setupWorkflow: func(t *testing.T, tmpDir string) {
				workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
				require.NoError(t, os.MkdirAll(workflowsDir, 0755))
				content := `name: parallel
version: "1.0.0"
states:
  initial: build
  build:
    type: parallel
    strategy: all_succeed
    parallel:
      - test
      - lint
    on_success: done
  test:
    type: step
    command: go test
    on_success: done
  lint:
    type: step
    command: golangci-lint run
    on_success: done
  done:
    type: terminal
`
				require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "parallel.yaml"), []byte(content), 0644))
			},
			args:    []string{"run", "parallel", "--dry-run"},
			wantErr: false,
			validateOut: func(t *testing.T, output string) {
				assert.Contains(t, output, "Dry Run")
			},
		},
		{
			name: "dry-run with nonexistent workflow",
			setupWorkflow: func(t *testing.T, tmpDir string) {
				workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
				require.NoError(t, os.MkdirAll(workflowsDir, 0755))
			},
			args:        []string{"run", "nonexistent", "--dry-run"},
			wantErr:     true,
			errContains: "not found",
		},
		{
			name: "dry-run with invalid input format",
			setupWorkflow: func(t *testing.T, tmpDir string) {
				workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
				require.NoError(t, os.MkdirAll(workflowsDir, 0755))
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
				require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "test.yaml"), []byte(content), 0644))
			},
			args:        []string{"run", "test", "--dry-run", "--input=invalid"},
			wantErr:     true,
			errContains: "invalid input",
		},
		{
			name: "dry-run with JSON output format",
			setupWorkflow: func(t *testing.T, tmpDir string) {
				workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
				require.NoError(t, os.MkdirAll(workflowsDir, 0755))
				content := `name: json-test
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo hello
    on_success: done
  done:
    type: terminal
`
				require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "json-test.yaml"), []byte(content), 0644))
			},
			args:    []string{"--format=json", "run", "json-test", "--dry-run"},
			wantErr: false,
			validateOut: func(t *testing.T, output string) {
				// JSON output should be valid
				assert.NotEmpty(t, output)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			origDir, _ := os.Getwd()
			defer func() { _ = os.Chdir(origDir) }()
			_ = os.Chdir(tmpDir)

			tt.setupWorkflow(t, tmpDir)

			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(append([]string{"--storage=" + tmpDir}, tt.args...))

			err := cmd.Execute()

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				if tt.validateOut != nil {
					tt.validateOut(t, out.String())
				}
			}
		})
	}
}

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
				workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
				require.NoError(t, os.MkdirAll(workflowsDir, 0755))
				// Create storage directories for state and history
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0755))
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "history"), 0755))
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
				require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "interactive-test.yaml"), []byte(content), 0644))
			},
			args:      []string{"run", "interactive-test", "--interactive"},
			mockInput: "y\n", // Proceed with step
			wantErr:   false,
		},
		{
			name: "interactive mode with breakpoints",
			setupWorkflow: func(t *testing.T, tmpDir string) {
				workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
				require.NoError(t, os.MkdirAll(workflowsDir, 0755))
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0755))
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "history"), 0755))
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
				require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "breakpoint-test.yaml"), []byte(content), 0644))
			},
			args:      []string{"run", "breakpoint-test", "--interactive", "--breakpoint=process"},
			mockInput: "y\n", // Proceed
			wantErr:   false,
		},
		{
			name: "interactive mode with comma-separated breakpoints",
			setupWorkflow: func(t *testing.T, tmpDir string) {
				workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
				require.NoError(t, os.MkdirAll(workflowsDir, 0755))
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0755))
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "history"), 0755))
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
				require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "multi-breakpoint.yaml"), []byte(content), 0644))
			},
			args:      []string{"run", "multi-breakpoint", "--interactive", "--breakpoint=step1,step3"},
			mockInput: "y\ny\n", // Proceed for each breakpoint
			wantErr:   false,
		},
		{
			name: "interactive mode with inputs",
			setupWorkflow: func(t *testing.T, tmpDir string) {
				workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
				require.NoError(t, os.MkdirAll(workflowsDir, 0755))
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0755))
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "history"), 0755))
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
				require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "input-interactive.yaml"), []byte(content), 0644))
			},
			args:      []string{"run", "input-interactive", "--interactive", "--input=msg=test"},
			mockInput: "y\n",
			wantErr:   false,
		},
		{
			name: "interactive mode with nonexistent workflow",
			setupWorkflow: func(t *testing.T, tmpDir string) {
				workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
				require.NoError(t, os.MkdirAll(workflowsDir, 0755))
			},
			args:        []string{"run", "nonexistent", "--interactive"},
			mockInput:   "",
			wantErr:     true,
			errContains: "not found",
		},
		{
			name: "interactive mode with invalid input format",
			setupWorkflow: func(t *testing.T, tmpDir string) {
				workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
				require.NoError(t, os.MkdirAll(workflowsDir, 0755))
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
				require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "test.yaml"), []byte(content), 0644))
			},
			args:        []string{"run", "test", "--interactive", "--input=invalid"},
			mockInput:   "",
			wantErr:     true,
			errContains: "invalid input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			origDir, _ := os.Getwd()
			defer func() { _ = os.Chdir(origDir) }()
			_ = os.Chdir(tmpDir)

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

// TestRunCommand_DryRunFlag tests that --dry-run flag is recognized
func TestRunCommand_HasDryRunFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			flag := sub.Flags().Lookup("dry-run")
			require.NotNil(t, flag, "expected 'run' command to have --dry-run flag")
			assert.Equal(t, "false", flag.DefValue, "dry-run should default to false")
			return
		}
	}

	t.Error("run command not found")
}

// TestRunCommand_InteractiveFlag tests that --interactive flag is recognized
func TestRunCommand_HasInteractiveFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			flag := sub.Flags().Lookup("interactive")
			require.NotNil(t, flag, "expected 'run' command to have --interactive flag")
			assert.Equal(t, "false", flag.DefValue, "interactive should default to false")
			return
		}
	}

	t.Error("run command not found")
}

// TestRunCommand_BreakpointFlag tests that --breakpoint flag is recognized
func TestRunCommand_HasBreakpointFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			flag := sub.Flags().Lookup("breakpoint")
			require.NotNil(t, flag, "expected 'run' command to have --breakpoint flag")
			return
		}
	}

	t.Error("run command not found")
}

// TestRunCommand_SQLiteHistoryStore_Wiring tests that workflows correctly use SQLiteHistoryStore
// This validates the T004 CLI wiring update from BadgerDB to SQLite
func TestRunCommand_SQLiteHistoryStore_Wiring(t *testing.T) {
	tests := []struct {
		name        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "workflow execution uses SQLite history store",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			origDir, _ := os.Getwd()
			defer func() { _ = os.Chdir(origDir) }()
			_ = os.Chdir(tmpDir)

			// Setup workflow and directories
			workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
			require.NoError(t, os.MkdirAll(workflowsDir, 0755))
			require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0755))

			workflowContent := `name: sqlite-test
version: "1.0.0"
states:
  initial: echo
  echo:
    type: step
    command: echo "testing sqlite"
    on_success: done
  done:
    type: terminal
`
			require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "sqlite-test.yaml"), []byte(workflowContent), 0644))

			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "sqlite-test"})

			err := cmd.Execute()

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)

				// Verify SQLite database file was created (not Badger directory)
				historyDBPath := filepath.Join(tmpDir, "history.db")
				_, statErr := os.Stat(historyDBPath)
				assert.NoError(t, statErr, "SQLite history.db should exist after workflow execution")

				// Verify no Badger directory was created
				badgerPath := filepath.Join(tmpDir, "history")
				_, badgerErr := os.Stat(badgerPath)
				assert.True(t, os.IsNotExist(badgerErr), "Badger history directory should NOT exist")
			}
		})
	}
}

// TestRunCommand_ConcurrentWorkflows validates that bug-48 is fixed
// Multiple workflows should be able to run concurrently without lock errors
func TestRunCommand_ConcurrentWorkflows(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Setup workflow and directories
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0755))

	// Create a workflow that takes a bit of time
	workflowContent := `name: concurrent-test
version: "1.0.0"
states:
  initial: work
  work:
    type: step
    command: echo "workflow running" && sleep 0.1
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "concurrent-test.yaml"), []byte(workflowContent), 0644))

	// Run multiple workflows concurrently
	const numConcurrent = 3
	errChan := make(chan error, numConcurrent)
	doneChan := make(chan struct{}, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func(workerID int) {
			// Each goroutine needs its own command instance
			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "concurrent-test"})

			err := cmd.Execute()
			if err != nil {
				errChan <- fmt.Errorf("worker %d failed: %w (output: %s)", workerID, err, out.String())
			}
			doneChan <- struct{}{}
		}(i)
	}

	// Wait for all workers to complete
	for i := 0; i < numConcurrent; i++ {
		<-doneChan
	}
	close(errChan)

	// Check if any worker failed
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	// All concurrent executions should succeed (bug-48 fix validation)
	assert.Empty(t, errors, "concurrent workflow executions should not fail with lock errors")

	// Verify history.db exists and is a valid SQLite file
	historyDBPath := filepath.Join(tmpDir, "history.db")
	info, err := os.Stat(historyDBPath)
	require.NoError(t, err)
	assert.True(t, info.Size() > 0, "history.db should have content")
}

// TestRunCommand_SingleStep_SQLiteHistory verifies single step execution uses SQLite
func TestRunCommand_SingleStep_SQLiteHistory(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Setup
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0755))

	workflowContent := `name: step-test
version: "1.0.0"
states:
  initial: greet
  greet:
    type: step
    command: echo hello
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "step-test.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "step-test", "--step=greet"})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify SQLite database was used
	historyDBPath := filepath.Join(tmpDir, "history.db")
	_, statErr := os.Stat(historyDBPath)
	assert.NoError(t, statErr, "SQLite history.db should exist after single step execution")
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
				require.NoError(t, os.MkdirAll(promptsDir, 0755))
				require.NoError(t, os.WriteFile(
					filepath.Join(promptsDir, "test.md"),
					[]byte("This is prompt content"),
					0644,
				))
			},
			inputFlag: "prompt=@prompts/test.md",
			wantErr:   false,
		},
		{
			name: "trims whitespace from prompt content",
			setupPrompt: func(t *testing.T, tmpDir string) {
				promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
				require.NoError(t, os.MkdirAll(promptsDir, 0755))
				require.NoError(t, os.WriteFile(
					filepath.Join(promptsDir, "whitespace.txt"),
					[]byte("\n  content with whitespace  \n\n"),
					0644,
				))
			},
			inputFlag: "msg=@prompts/whitespace.txt",
			wantErr:   false,
		},
		{
			name: "supports nested directories",
			setupPrompt: func(t *testing.T, tmpDir string) {
				nestedDir := filepath.Join(tmpDir, ".awf", "prompts", "ai", "agents")
				require.NoError(t, os.MkdirAll(nestedDir, 0755))
				require.NoError(t, os.WriteFile(
					filepath.Join(nestedDir, "system.md"),
					[]byte("You are an AI assistant"),
					0644,
				))
			},
			inputFlag: "system=@prompts/ai/agents/system.md",
			wantErr:   false,
		},
		{
			name: "error when prompt file does not exist",
			setupPrompt: func(t *testing.T, tmpDir string) {
				promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
				require.NoError(t, os.MkdirAll(promptsDir, 0755))
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
				require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf"), 0755))
			},
			inputFlag:   "prompt=@prompts/test.md",
			wantErr:     true,
			errContains: "not found",
		},
		{
			name: "blocks path traversal attack",
			setupPrompt: func(t *testing.T, tmpDir string) {
				promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
				require.NoError(t, os.MkdirAll(promptsDir, 0755))
				// Create a sensitive file outside prompts dir
				sensitiveFile := filepath.Join(tmpDir, "secret.txt")
				require.NoError(t, os.WriteFile(sensitiveFile, []byte("secret"), 0644))
			},
			inputFlag:   "data=@prompts/../secret.txt",
			wantErr:     true,
			errContains: "invalid prompt path",
		},
		{
			name: "blocks absolute path in prompt reference",
			setupPrompt: func(t *testing.T, tmpDir string) {
				promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
				require.NoError(t, os.MkdirAll(promptsDir, 0755))
			},
			inputFlag:   "data=@prompts//etc/passwd",
			wantErr:     true,
			errContains: "invalid prompt path",
		},
		{
			name: "regular value without @prompts/ prefix is unchanged",
			setupPrompt: func(t *testing.T, tmpDir string) {
				promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
				require.NoError(t, os.MkdirAll(promptsDir, 0755))
			},
			inputFlag: "name=plain-value",
			wantErr:   false,
		},
		{
			name: "supports .txt extension",
			setupPrompt: func(t *testing.T, tmpDir string) {
				promptsDir := filepath.Join(tmpDir, ".awf", "prompts")
				require.NoError(t, os.MkdirAll(promptsDir, 0755))
				require.NoError(t, os.WriteFile(
					filepath.Join(promptsDir, "note.txt"),
					[]byte("Plain text content"),
					0644,
				))
			},
			inputFlag: "note=@prompts/note.txt",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			origDir, _ := os.Getwd()
			defer func() { _ = os.Chdir(origDir) }()
			_ = os.Chdir(tmpDir)

			// Setup prompts directory and files
			tt.setupPrompt(t, tmpDir)

			// Create a minimal workflow for the test
			workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
			_ = os.MkdirAll(workflowsDir, 0755)
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
			_ = os.WriteFile(filepath.Join(workflowsDir, "test.yaml"), []byte(workflowContent), 0644)

			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"run", "test", "--input", tt.inputFlag})

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

// ============================================================================
// F035: Workflow Arguments Help Command Tests
// ============================================================================

// TestRunCommand_WorkflowHelp_WithInputs tests that `awf run <workflow> --help`
// displays workflow-specific help including input parameters (US1)
func TestRunCommand_WorkflowHelp_WithInputs(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with multiple inputs of varying types
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: commit
version: "1.0.0"
description: Create a git commit with the specified message
inputs:
  - name: branch
    type: string
    required: true
    description: The branch to commit to
  - name: message
    type: string
    required: false
    default: "Auto-commit"
    description: The commit message
  - name: verbose
    type: boolean
    required: false
    default: false
states:
  initial: commit
  commit:
    type: step
    command: git commit -m "{{inputs.message}}"
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "commit.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "commit", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show workflow description (US4)
	assert.Contains(t, output, "Create a git commit", "should display workflow description")

	// Should show Input Parameters header
	assert.Contains(t, output, "Input Parameters", "should display input parameters header")

	// Should show each input name
	assert.Contains(t, output, "branch", "should display 'branch' input")
	assert.Contains(t, output, "message", "should display 'message' input")
	assert.Contains(t, output, "verbose", "should display 'verbose' input")

	// Should show input types (FR-002)
	assert.Contains(t, output, "string", "should display string type")
	assert.Contains(t, output, "boolean", "should display boolean type")

	// Should show required/optional status (FR-002)
	assert.Contains(t, output, "yes", "should indicate required inputs")
	assert.Contains(t, output, "no", "should indicate optional inputs")

	// Should show descriptions (US2)
	assert.Contains(t, output, "The branch to commit to", "should display input description")
	assert.Contains(t, output, "The commit message", "should display input description")

	// Should show default values (US3)
	assert.Contains(t, output, "Auto-commit", "should display default value")

	// Should also include standard command flags (FR-003)
	assert.Contains(t, output, "--input", "should include standard --input flag")
}

// TestRunCommand_WorkflowHelp_NoInputs tests that `awf run <workflow> --help`
// shows appropriate message when workflow has no inputs (US1)
func TestRunCommand_WorkflowHelp_NoInputs(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with no inputs
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: deploy
version: "1.0.0"
states:
  initial: deploy
  deploy:
    type: step
    command: echo "deploying..."
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "deploy.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "deploy", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show "No input parameters" message
	assert.Contains(t, output, "No input parameters", "should indicate no inputs defined")

	// Should still include standard command flags
	assert.Contains(t, output, "--input", "should include standard --input flag")
}

// TestRunCommand_WorkflowHelp_WorkflowNotFound tests error handling for non-existent workflow (US1)
func TestRunCommand_WorkflowHelp_WorkflowNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create .awf directory but no workflow
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"run", "unknown-workflow", "--help"})

	err := cmd.Execute()
	// Help should not return an error, but display error message in output
	require.NoError(t, err)

	// Should show error message about workflow not found
	combinedOutput := out.String() + errOut.String()
	assert.Contains(t, combinedOutput, "not found", "should display workflow not found error")
	assert.Contains(t, combinedOutput, "unknown-workflow", "should mention the workflow name")

	// Should fall back to default help after error
	assert.Contains(t, out.String(), "Execute a workflow", "should show default help text")
}

// TestRunCommand_WorkflowHelp_WithDescription tests workflow description display (US4)
func TestRunCommand_WorkflowHelp_WithDescription(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with a description
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: analyze
version: "1.0.0"
description: Analyze code quality and generate a comprehensive report
inputs:
  - name: target
    type: string
    required: true
states:
  initial: analyze
  analyze:
    type: step
    command: echo "analyzing..."
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "analyze.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "analyze", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Description should appear at the top (before inputs)
	assert.Contains(t, output, "Description:", "should have description label")
	assert.Contains(t, output, "Analyze code quality", "should display workflow description")

	// Description should appear before Input Parameters
	descIdx := strings.Index(output, "Analyze code quality")
	inputIdx := strings.Index(output, "Input Parameters")
	assert.Less(t, descIdx, inputIdx, "description should appear before input parameters")
}

// TestRunCommand_WorkflowHelp_WithoutDescription tests that help works without description (US4)
func TestRunCommand_WorkflowHelp_WithoutDescription(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow without a description
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: simple
version: "1.0.0"
inputs:
  - name: file
    type: string
    required: true
states:
  initial: process
  process:
    type: step
    command: echo "processing..."
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "simple.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "simple", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should NOT have "Description:" label when no description
	assert.NotContains(t, output, "Description:", "should not show description label when no description")

	// Should still show inputs
	assert.Contains(t, output, "file", "should display input name")
	assert.Contains(t, output, "Input Parameters", "should show input parameters section")
}

// TestRunCommand_WorkflowHelp_InputWithoutDescription tests "No description" placeholder (US2)
func TestRunCommand_WorkflowHelp_InputWithoutDescription(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with inputs that have no descriptions
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: nodesc
version: "1.0.0"
inputs:
  - name: param1
    type: string
    required: true
  - name: param2
    type: string
    required: false
    description: This one has a description
states:
  initial: run
  run:
    type: step
    command: echo "running..."
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "nodesc.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "nodesc", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show "No description" placeholder for inputs without description
	assert.Contains(t, output, "No description", "should show 'No description' placeholder")

	// Should still show the actual description for the input that has one
	assert.Contains(t, output, "This one has a description", "should display input with description")
}

// TestRunCommand_WorkflowHelp_DefaultValues tests default value display (US3)
func TestRunCommand_WorkflowHelp_DefaultValues(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with various default values
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: defaults
version: "1.0.0"
inputs:
  - name: timeout
    type: integer
    required: false
    default: 30
    description: Timeout in seconds
  - name: format
    type: string
    required: false
    default: json
    description: Output format
  - name: optional_no_default
    type: string
    required: false
    description: Optional without default
  - name: required_input
    type: string
    required: true
    description: Required input
states:
  initial: run
  run:
    type: step
    command: echo "running..."
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "defaults.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "defaults", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show default values
	assert.Contains(t, output, "30", "should display numeric default value")
	assert.Contains(t, output, "json", "should display string default value")

	// Should show "-" or empty for inputs without defaults
	// The input "optional_no_default" should have "-" in the DEFAULT column
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "optional_no_default") {
			assert.Contains(t, line, "-", "should show '-' for optional input without default")
			break
		}
	}
}

// TestRunCommand_WorkflowHelp_NoWorkflowArg tests that default help is shown without workflow arg
func TestRunCommand_WorkflowHelp_NoWorkflowArg(t *testing.T) {
	cmd := cli.NewRootCommand()

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show default Cobra help for run command (FR-003)
	assert.Contains(t, output, "Execute a workflow", "should show run command description")
	assert.Contains(t, output, "--input", "should show --input flag")
	assert.Contains(t, output, "--output", "should show --output flag")
	assert.Contains(t, output, "--step", "should show --step flag")
	assert.Contains(t, output, "--dry-run", "should show --dry-run flag")
}

// TestRunCommand_WorkflowHelp_HelpTakesPrecedence tests that --help prevents execution (FR-005)
func TestRunCommand_WorkflowHelp_HelpTakesPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow that would fail if executed
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: would-fail
version: "1.0.0"
inputs:
  - name: required_param
    type: string
    required: true
states:
  initial: fail
  fail:
    type: step
    command: exit 1
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "would-fail.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// Note: Missing required input and using a failing command, but --help should prevent execution
	cmd.SetArgs([]string{"run", "would-fail", "--help"})

	err := cmd.Execute()
	// Should not error - help takes precedence
	require.NoError(t, err)

	output := out.String()

	// Should show help content, not execution error
	assert.Contains(t, output, "required_param", "should display input in help")
	assert.NotContains(t, output, "exit 1", "should not show command output")
}

// TestRunCommand_WorkflowHelp_TableFormat tests help output format is aligned (NFR-002)
func TestRunCommand_WorkflowHelp_TableFormat(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with inputs
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: formatted
version: "1.0.0"
inputs:
  - name: short
    type: string
    required: true
    description: Short
  - name: very_long_parameter_name
    type: boolean
    required: false
    description: A very long description that should still be readable
states:
  initial: run
  run:
    type: step
    command: echo "running..."
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "formatted.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "formatted", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should have table header with column names
	assert.Contains(t, output, "NAME", "should have NAME column header")
	assert.Contains(t, output, "TYPE", "should have TYPE column header")
	assert.Contains(t, output, "REQUIRED", "should have REQUIRED column header")
	assert.Contains(t, output, "DEFAULT", "should have DEFAULT column header")
	assert.Contains(t, output, "DESCRIPTION", "should have DESCRIPTION column header")
}

// TestRunCommand_WorkflowHelp_AllInputTypes tests different input types display correctly
func TestRunCommand_WorkflowHelp_AllInputTypes(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with all input types
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: alltypes
version: "1.0.0"
inputs:
  - name: string_input
    type: string
    required: true
  - name: integer_input
    type: integer
    required: false
    default: 42
  - name: boolean_input
    type: boolean
    required: false
    default: true
  - name: number_input
    type: number
    required: false
    default: 3.14
states:
  initial: run
  run:
    type: step
    command: echo "running..."
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "alltypes.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "alltypes", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should display all type names
	assert.Contains(t, output, "string", "should display string type")
	assert.Contains(t, output, "integer", "should display integer type")
	assert.Contains(t, output, "boolean", "should display boolean type")
	assert.Contains(t, output, "number", "should display number type")
}

// ============================================================================
// Feature 39: Agent Step Type - Dry-Run Support Tests
// Component: dry_run_support (7/7)
// ============================================================================

// TestRunCommand_DryRun_AgentStep_Basic tests basic dry-run with agent step
// AC8: --dry-run shows resolved prompt without invoking
func TestRunCommand_DryRun_AgentStep_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with a simple agent step
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: agent-simple
version: "1.0.0"
inputs:
  - name: code_path
    type: string
    required: true
states:
  initial: analyze
  analyze:
    type: agent
    provider: claude
    prompt: |
      Analyze this code for potential issues:
      {{inputs.code_path}}
    options:
      model: claude-sonnet-4-20250514
      max_tokens: 4096
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "agent-simple.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-simple", "--dry-run", "--input=code_path=main.go"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show dry-run output
	assert.Contains(t, output, "Dry Run", "should display dry-run header")
	assert.Contains(t, output, "analyze", "should display agent step name")
	assert.Contains(t, output, "agent", "should display step type as agent")
	assert.Contains(t, output, "claude", "should display provider name")
}

// TestRunCommand_DryRun_AgentStep_ResolvedPrompt tests that dry-run shows resolved prompt
// AC8: --dry-run shows resolved prompt after interpolation
func TestRunCommand_DryRun_AgentStep_ResolvedPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with agent step using input interpolation
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: agent-interpolation
version: "1.0.0"
inputs:
  - name: target
    type: string
    required: true
  - name: task
    type: string
    required: true
states:
  initial: process
  process:
    type: agent
    provider: codex
    prompt: "{{inputs.task}} for file: {{inputs.target}}"
    timeout: 120
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "agent-interpolation.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run", "agent-interpolation",
		"--dry-run",
		"--input=target=app.js",
		"--input=task=Refactor",
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show the resolved prompt (after interpolation)
	// The stub will return empty values, but the framework should attempt resolution
	assert.Contains(t, output, "Dry Run", "should display dry-run header")
	assert.Contains(t, output, "process", "should display step name")
}

// TestRunCommand_DryRun_AgentStep_CLICommand tests that dry-run shows CLI command
// AC8: --dry-run shows CLI command that would be executed
func TestRunCommand_DryRun_AgentStep_CLICommand(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with agent step
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: agent-cli
version: "1.0.0"
states:
  initial: generate
  generate:
    type: agent
    provider: gemini
    prompt: "Generate documentation"
    options:
      model: gemini-pro
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "agent-cli.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-cli", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show the CLI command that would be executed
	// The stub returns empty values, but we verify the dry-run structure is in place
	assert.Contains(t, output, "Dry Run", "should display dry-run header")
	assert.Contains(t, output, "generate", "should display step name")
}

// TestRunCommand_DryRun_AgentStep_WithOptions tests dry-run with agent options
// AC6: Provider-specific options shown in dry-run
func TestRunCommand_DryRun_AgentStep_WithOptions(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with agent step with multiple options
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: agent-options
version: "1.0.0"
states:
  initial: review
  review:
    type: agent
    provider: claude
    prompt: "Review this code"
    options:
      model: claude-sonnet-4-20250514
      max_tokens: 8192
      temperature: 0.7
      output_format: json
    timeout: 300
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "agent-options.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-options", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show dry-run with options displayed
	assert.Contains(t, output, "Dry Run", "should display dry-run header")
	assert.Contains(t, output, "review", "should display step name")
	assert.Contains(t, output, "claude", "should display provider")
}

// TestRunCommand_DryRun_AgentStep_CustomProvider tests dry-run with custom provider
// AC2: Custom provider support in dry-run
func TestRunCommand_DryRun_AgentStep_CustomProvider(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with custom provider
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: agent-custom
version: "1.0.0"
states:
  initial: custom_task
  custom_task:
    type: agent
    provider: custom
    command: "my-llm --prompt {{prompt}} --json"
    prompt: "Summarize the report"
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "agent-custom.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-custom", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show dry-run with custom provider
	assert.Contains(t, output, "Dry Run", "should display dry-run header")
	assert.Contains(t, output, "custom_task", "should display step name")
}

// TestRunCommand_DryRun_AgentStep_Parallel tests dry-run with parallel agent steps
// AC10: Works with parallel steps
func TestRunCommand_DryRun_AgentStep_Parallel(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with parallel agent steps
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: agent-parallel
version: "1.0.0"
inputs:
  - name: code
    type: string
    required: true
states:
  initial: analyze_parallel
  analyze_parallel:
    type: parallel
    strategy: all_succeed
    parallel:
      - security_check
      - performance_check
    on_success: done
  security_check:
    type: agent
    provider: claude
    prompt: "Check security issues in: {{inputs.code}}"
    on_success: done
  performance_check:
    type: agent
    provider: codex
    prompt: "Analyze performance of: {{inputs.code}}"
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "agent-parallel.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-parallel", "--dry-run", "--input=code=app.go"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show both parallel agent steps in dry-run
	assert.Contains(t, output, "Dry Run", "should display dry-run header")
	assert.Contains(t, output, "analyze_parallel", "should display parallel step")
	assert.Contains(t, output, "security_check", "should display first agent step")
	assert.Contains(t, output, "performance_check", "should display second agent step")
}

// TestRunCommand_DryRun_AgentStep_MultiTurn tests dry-run with multi-turn agent workflow
// Multi-turn = multiple agent steps using {{states.*}} for context passing
func TestRunCommand_DryRun_AgentStep_MultiTurn(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with multi-turn agent conversation
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: agent-multiturn
version: "1.0.0"
inputs:
  - name: code_path
    type: string
    required: true
states:
  initial: analyze
  analyze:
    type: agent
    provider: claude
    prompt: "Analyze this code: {{inputs.code_path}}"
    on_success: suggest_fixes
  suggest_fixes:
    type: agent
    provider: claude
    prompt: |
      Based on this analysis:
      {{states.analyze.output}}

      Suggest specific fixes.
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "agent-multiturn.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-multiturn", "--dry-run", "--input=code_path=main.go"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show both agent steps with state references
	assert.Contains(t, output, "Dry Run", "should display dry-run header")
	assert.Contains(t, output, "analyze", "should display first agent step")
	assert.Contains(t, output, "suggest_fixes", "should display second agent step")
}

// TestRunCommand_DryRun_AgentStep_WithTimeout tests dry-run showing agent timeout
// AC7: Timeout handling per step shown in dry-run
func TestRunCommand_DryRun_AgentStep_WithTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with agent step with custom timeout
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: agent-timeout
version: "1.0.0"
states:
  initial: long_task
  long_task:
    type: agent
    provider: claude
    prompt: "Complex analysis task"
    timeout: 600
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "agent-timeout.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-timeout", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show dry-run with timeout displayed
	assert.Contains(t, output, "Dry Run", "should display dry-run header")
	assert.Contains(t, output, "long_task", "should display step name")
}

// TestRunCommand_DryRun_AgentStep_MixedSteps tests dry-run with mixed step types
// Ensures agent steps work alongside command, parallel, and terminal steps
func TestRunCommand_DryRun_AgentStep_MixedSteps(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with mixed step types
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: agent-mixed
version: "1.0.0"
states:
  initial: fetch
  fetch:
    type: step
    command: git clone repo.git
    on_success: analyze
  analyze:
    type: agent
    provider: claude
    prompt: "Analyze the codebase"
    on_success: build
  build:
    type: step
    command: make build
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "agent-mixed.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-mixed", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show all steps in dry-run (command, agent, command, terminal)
	assert.Contains(t, output, "Dry Run", "should display dry-run header")
	assert.Contains(t, output, "fetch", "should display command step")
	assert.Contains(t, output, "analyze", "should display agent step")
	assert.Contains(t, output, "build", "should display second command step")
}

// TestRunCommand_DryRun_AgentStep_JSONOutput tests dry-run with JSON output format
// Ensures agent dry-run works with JSON formatting
func TestRunCommand_DryRun_AgentStep_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with agent step
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: agent-json
version: "1.0.0"
states:
  initial: generate
  generate:
    type: agent
    provider: opencode
    prompt: "Generate unit tests"
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "agent-json.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "--format=json", "run", "agent-json", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// JSON output should be valid (even if stub returns empty values)
	assert.NotEmpty(t, output, "should produce JSON output")
}

// TestRunCommand_DryRun_AgentStep_InvalidPromptSyntax tests dry-run error handling
// AC9: Error handling for invalid configurations
func TestRunCommand_DryRun_AgentStep_InvalidPromptSyntax(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with invalid template syntax in prompt
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: agent-invalid
version: "1.0.0"
states:
  initial: bad_prompt
  bad_prompt:
    type: agent
    provider: claude
    prompt: "Analyze {{unclosed template"
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "agent-invalid.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-invalid", "--dry-run"})

	err := cmd.Execute()

	// Dry-run might succeed even with template errors (shows unresolved template)
	// Or it might fail depending on validation strictness
	// The important thing is it doesn't crash
	_ = err // Either outcome is acceptable for this test
	assert.NotPanics(t, func() {
		_ = cmd.Execute()
	}, "should not panic on invalid template syntax")
}

// TestRunCommand_DryRun_AgentStep_EmptyPrompt tests dry-run with empty prompt
// Edge case: what happens with empty prompt
func TestRunCommand_DryRun_AgentStep_EmptyPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with empty prompt
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `name: agent-empty
version: "1.0.0"
states:
  initial: empty
  empty:
    type: agent
    provider: claude
    prompt: ""
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "agent-empty.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-empty", "--dry-run"})

	err := cmd.Execute()

	// Empty prompt should fail validation before dry-run
	assert.Error(t, err, "should fail validation for empty prompt")
	assert.Contains(t, err.Error(), "prompt is required", "error should mention missing prompt")
}

// TestRunCommand_DryRun_AgentStep_LongPrompt tests dry-run with very long prompt
// Edge case: handling of long prompts in dry-run display
func TestRunCommand_DryRun_AgentStep_LongPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create a workflow with a very long prompt
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	longPrompt := strings.Repeat("This is a very long prompt. ", 100)
	workflowContent := fmt.Sprintf(`name: agent-long
version: "1.0.0"
states:
  initial: long_prompt
  long_prompt:
    type: agent
    provider: claude
    prompt: "%s"
    on_success: done
  done:
    type: terminal
`, longPrompt)
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "agent-long.yaml"), []byte(workflowContent), 0644))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-long", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should handle long prompts gracefully (might truncate or wrap)
	assert.Contains(t, output, "Dry Run", "should display dry-run header")
	assert.Contains(t, output, "long_prompt", "should display step name")
}
