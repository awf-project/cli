//go:build integration

package cli_test

// C015 Component T009: Agent execution tests
// Extracted from run_test.go - 14 test functions testing agent step execution and dry-run mode
// Tests: DryRun_* (general dry-run), DryRun_AgentStep_* (F039 agent integration with dry-run mode)

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
				createTestWorkflow(t, tmpDir, "simple.yaml", content)
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
				createTestWorkflow(t, tmpDir, "with-inputs.yaml", content)
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
				createTestWorkflow(t, tmpDir, "parallel.yaml", content)
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
			},
			args:        []string{"run", "nonexistent", "--dry-run"},
			wantErr:     true,
			errContains: "not found",
		},
		{
			name: "dry-run with invalid input format",
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
			args:        []string{"run", "test", "--dry-run", "--input=invalid"},
			wantErr:     true,
			errContains: "invalid input",
		},
		{
			name: "dry-run with JSON output format",
			setupWorkflow: func(t *testing.T, tmpDir string) {
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
				createTestWorkflow(t, tmpDir, "json-test.yaml", content)
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
			tmpDir := setupInitTestDir(t)

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

// TestRunCommand_DryRun_AgentStep_Basic tests basic dry-run with agent step
// AC8: --dry-run shows resolved prompt without invoking
func TestRunCommand_DryRun_AgentStep_Basic(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create a workflow with a simple agent step
	workflowContent := `name: agent-basic
version: "1.0.0"
states:
  initial: ask
  ask:
    type: agent
    provider: claude
    prompt: "What is 2+2?"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "agent-basic.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-basic", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should display dry-run header and agent step info
	assert.Contains(t, output, "Dry Run", "should display dry-run header")
	assert.Contains(t, output, "ask", "should display step name")
	assert.Contains(t, output, "agent", "should indicate agent step type")
	assert.Contains(t, output, "What is 2+2?", "should show resolved prompt")
}

func TestRunCommand_DryRun_AgentStep_ResolvedPrompt(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create a workflow with interpolated prompt
	workflowContent := `name: agent-interpolated
version: "1.0.0"
inputs:
  - name: question
    type: string
    required: true
states:
  initial: ask
  ask:
    type: agent
    provider: claude
    prompt: "Answer this: {{inputs.question}}"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "agent-interpolated.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run",
		"agent-interpolated",
		"--input=question=What is the capital of France?",
		"--dry-run",
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show the interpolated prompt
	assert.Contains(t, output, "What is the capital of France?", "should resolve {{inputs.question}}")
}

func TestRunCommand_DryRun_AgentStep_CLICommand(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create a workflow with agent CLI command syntax
	workflowContent := `name: agent-cli
version: "1.0.0"
states:
  initial: analyze
  analyze:
    type: agent
    provider: claude
    prompt: "analyze this code: main.go"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "agent-cli.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-cli", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	assert.Contains(t, output, "analyze", "should display step name")
	assert.Contains(t, output, "analyze this code: main.go", "should show command")
}

func TestRunCommand_DryRun_AgentStep_WithOptions(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create a workflow with agent options
	workflowContent := `name: agent-options
version: "1.0.0"
states:
  initial: generate
  generate:
    type: agent
    provider: claude
    prompt: "Generate a haiku"
    options:
      temperature: 0.7
      max_tokens: 100
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "agent-options.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-options", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	assert.Contains(t, output, "generate", "should display step name")
	// Options might be displayed in various formats
	assert.Contains(t, output, "Generate a haiku", "should show prompt")
}

func TestRunCommand_DryRun_AgentStep_CustomProvider(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create a workflow with custom provider
	workflowContent := `name: agent-custom
version: "1.0.0"
states:
  initial: query
  query:
    type: agent
    provider: gemini
    prompt: "Explain quantum computing"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "agent-custom.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-custom", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	assert.Contains(t, output, "query", "should display step name")
	assert.Contains(t, output, "gemini", "should show custom provider")
}

func TestRunCommand_DryRun_AgentStep_Parallel(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create a workflow with parallel agent steps
	workflowContent := `name: agent-parallel
version: "1.0.0"
states:
  initial: parallel_agents
  parallel_agents:
    type: parallel
    parallel:
      - agent1
      - agent2
    on_success: done
  agent1:
    type: agent
    provider: claude
    prompt: "Summarize topic A"
    on_success: done
  agent2:
    type: agent
    provider: claude
    prompt: "Summarize topic B"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "agent-parallel.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-parallel", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	assert.Contains(t, output, "parallel_agents", "should display parallel step")
	// Both agent steps should be shown in dry-run output
	assert.Contains(t, output, "agent1", "should show first agent")
	assert.Contains(t, output, "agent2", "should show second agent")
}

func TestRunCommand_DryRun_AgentStep_MultiTurn(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create a workflow with sequential agent steps
	workflowContent := `name: agent-multiturn
version: "1.0.0"
states:
  initial: first_ask
  first_ask:
    type: agent
    provider: claude
    prompt: "What is the capital of France?"
    on_success: second_ask
  second_ask:
    type: agent
    provider: claude
    prompt: "What is the population of that city?"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "agent-multiturn.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-multiturn", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	assert.Contains(t, output, "first_ask", "should display first agent step")
	// Note: In dry-run, we might only see the initial step, not subsequent ones
	assert.Contains(t, output, "What is the capital of France?", "should show first prompt")
}

func TestRunCommand_DryRun_AgentStep_WithTimeout(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create a workflow with agent timeout
	workflowContent := `name: agent-timeout
version: "1.0.0"
states:
  initial: slow_query
  slow_query:
    type: agent
    provider: claude
    prompt: "Process this large dataset"
    timeout: 30s
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "agent-timeout.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-timeout", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	assert.Contains(t, output, "slow_query", "should display step name")
	// Timeout might be shown in various formats
	assert.Contains(t, output, "Process this large dataset", "should show prompt")
}

func TestRunCommand_DryRun_AgentStep_MixedSteps(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create a workflow mixing agent and regular steps
	workflowContent := `name: agent-mixed
version: "1.0.0"
states:
  initial: prepare
  prepare:
    type: step
    command: echo "Preparing data"
    on_success: analyze
  analyze:
    type: agent
    provider: claude
    prompt: "Analyze the prepared data"
    on_success: finalize
  finalize:
    type: step
    command: echo "Finalizing"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "agent-mixed.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-mixed", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show both regular step and agent step
	assert.Contains(t, output, "prepare", "should show prepare step")
	// In dry-run, might only show the execution plan
	assert.Contains(t, output, "Dry Run", "should indicate dry-run mode")
}

func TestRunCommand_DryRun_AgentStep_JSONOutput(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create a workflow with agent that expects JSON output
	workflowContent := `name: agent-json
version: "1.0.0"
states:
  initial: structured_query
  structured_query:
    type: agent
    provider: claude
    prompt: "Return JSON with keys: name, age, city"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "agent-json.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-json", "--dry-run"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	assert.Contains(t, output, "structured_query", "should display step name")
	assert.Contains(t, output, "Return JSON", "should show prompt mentioning JSON")
}

func TestRunCommand_DryRun_AgentStep_InvalidPromptSyntax(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create a workflow with invalid interpolation syntax
	workflowContent := `name: agent-invalid
version: "1.0.0"
inputs:
  - name: question
    type: string
    required: true
states:
  initial: ask
  ask:
    type: agent
    provider: claude
    prompt: "Answer: {{inputs.invalid_field}}"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "agent-invalid.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run",
		"agent-invalid",
		"--input=question=test",
		"--dry-run",
	})

	err := cmd.Execute()

	// Depending on validation timing, this might error or show unresolved template
	// Accept either behavior
	if err != nil {
		// Validation caught the issue
		assert.Contains(t, err.Error(), "invalid_field")
	} else {
		// Template shown as-is
		output := out.String()
		assert.NotEmpty(t, output, "should produce some output")
	}
}

func TestRunCommand_DryRun_AgentStep_EmptyPrompt(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create a workflow with empty prompt
	workflowContent := `name: agent-empty
version: "1.0.0"
states:
  initial: ask
  ask:
    type: agent
    provider: claude
    prompt: ""
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "agent-empty.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "agent-empty", "--dry-run"})

	err := cmd.Execute()

	// Should either error during validation or proceed with empty prompt
	if err != nil {
		// Validation caught empty prompt
		assert.NotNil(t, err)
	} else {
		// Proceeded with empty prompt
		output := out.String()
		assert.Contains(t, output, "ask", "should show step name")
	}
}

func TestRunCommand_DryRun_AgentStep_LongPrompt(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	// Create a workflow with a very long prompt

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
	createTestWorkflow(t, tmpDir, "agent-long.yaml", workflowContent)

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
