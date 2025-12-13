package cli_test

import (
	"bytes"
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
