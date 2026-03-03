package cli_test

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

// interactiveConfigWorkflow is a minimal workflow with two required inputs.
// The echo step outputs key=value pairs so tests can assert which values were used.
const interactiveConfigWorkflow = `name: interactive-config-test
description: Test config input merging in interactive mode
inputs:
  - name: api_key
    type: string
    required: true
    description: API key
  - name: model
    type: string
    required: true
    description: Model name
states:
  initial: run
  run:
    type: step
    command: echo "api_key={{inputs.api_key}} model={{inputs.model}}"
    on_success: done
  done:
    type: terminal
    status: success
`

// TestRunInteractive_ConfigInputsPreFillExecution verifies that an input pre-filled in
// config.yaml is not prompted for and its value is used during interactive execution.
func TestRunInteractive_ConfigInputsPreFillExecution(t *testing.T) {
	tmpDir := setupTestDir(t)

	configPath := writeConfigFile(t, tmpDir, map[string]string{
		"api_key": "sk-from-config",
	})
	t.Setenv("AWF_CONFIG_PATH", configPath)
	createTestWorkflow(t, tmpDir, "interactive-config-test.yaml", interactiveConfigWorkflow)

	// "c\n" continues through the interactive step; model is provided via --input.
	// api_key must come from config.yaml without prompting.
	stdin := strings.NewReader("c\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run", "interactive-config-test",
		"--interactive",
		"--input", "model=gpt-4",
	})

	err := cmd.Execute()
	require.NoError(t, err, "interactive run should succeed when api_key is pre-filled by config.yaml")

	output := stdout.String()
	assert.Contains(t, output, "sk-from-config", "api_key value from config.yaml must appear in output")
	assert.Contains(t, output, "gpt-4", "model value from --input must appear in output")
}

// TestRunInteractive_CLIInputsOverrideConfig verifies that CLI --input values take
// precedence over config.yaml values when the same key appears in both.
func TestRunInteractive_CLIInputsOverrideConfig(t *testing.T) {
	tmpDir := setupTestDir(t)

	configPath := writeConfigFile(t, tmpDir, map[string]string{
		"api_key": "sk-config",
		"model":   "gpt-config",
	})
	t.Setenv("AWF_CONFIG_PATH", configPath)
	createTestWorkflow(t, tmpDir, "interactive-config-test.yaml", interactiveConfigWorkflow)

	stdin := strings.NewReader("c\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run", "interactive-config-test",
		"--interactive",
		"--input", "api_key=sk-cli",
		"--input", "model=gpt-cli",
	})

	err := cmd.Execute()
	require.NoError(t, err, "interactive run should succeed")

	output := stdout.String()
	assert.Contains(t, output, "sk-cli", "CLI api_key must override config.yaml value")
	assert.Contains(t, output, "gpt-cli", "CLI model must override config.yaml value")
	assert.NotContains(t, output, "sk-config", "config api_key must not appear when CLI overrides it")
	assert.NotContains(t, output, "gpt-config", "config model must not appear when CLI overrides it")
}

// TestRunInteractive_BothInputsFromConfig verifies that when all required inputs are
// supplied by config.yaml, interactive mode runs without any --input flags or prompts.
func TestRunInteractive_BothInputsFromConfig(t *testing.T) {
	tmpDir := setupTestDir(t)

	configPath := writeConfigFile(t, tmpDir, map[string]string{
		"api_key": "sk-test",
		"model":   "gpt-4",
	})
	t.Setenv("AWF_CONFIG_PATH", configPath)
	createTestWorkflow(t, tmpDir, "interactive-config-test.yaml", interactiveConfigWorkflow)

	stdin := strings.NewReader("c\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run", "interactive-config-test",
		"--interactive",
	})

	err := cmd.Execute()
	require.NoError(t, err, "interactive run should succeed when all inputs are provided by config.yaml")

	output := stdout.String()
	assert.Contains(t, output, "sk-test", "api_key from config.yaml must appear in output")
	assert.Contains(t, output, "gpt-4", "model from config.yaml must appear in output")
}

// TestRunInteractive_NoConfigFile_NoRegression verifies that interactive mode works
// normally when no config.yaml exists and all inputs are provided via --input flags.
func TestRunInteractive_NoConfigFile_NoRegression(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Point to a config path that does not exist → loadProjectConfig returns empty config
	t.Setenv("AWF_CONFIG_PATH", filepath.Join(tmpDir, ".awf", "config.yaml"))
	createTestWorkflow(t, tmpDir, "interactive-config-test.yaml", interactiveConfigWorkflow)

	stdin := strings.NewReader("c\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run", "interactive-config-test",
		"--interactive",
		"--input", "api_key=sk-cli",
		"--input", "model=gpt-4",
	})

	err := cmd.Execute()
	require.NoError(t, err, "interactive run should succeed when no config.yaml but all inputs via --input")

	output := stdout.String()
	assert.Contains(t, output, "sk-cli", "api_key from --input must appear in output")
	assert.Contains(t, output, "gpt-4", "model from --input must appear in output")
}

// TestRunInteractive_InvalidConfig_ReturnsConfigError verifies that a malformed
// config.yaml causes interactive mode to fail with a "config error:" prefix.
func TestRunInteractive_InvalidConfig_ReturnsConfigError(t *testing.T) {
	tmpDir := setupTestDir(t)

	awfDir := filepath.Join(tmpDir, ".awf")
	require.NoError(t, os.MkdirAll(awfDir, 0o755))
	configPath := filepath.Join(awfDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("inputs: [\ninvalid yaml:::\n"), 0o644))
	t.Setenv("AWF_CONFIG_PATH", configPath)
	createTestWorkflow(t, tmpDir, "interactive-config-test.yaml", interactiveConfigWorkflow)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run", "interactive-config-test",
		"--interactive",
		"--input", "api_key=sk-cli",
		"--input", "model=gpt-4",
	})

	err := cmd.Execute()
	require.Error(t, err, "interactive run should fail with invalid config.yaml")
	assert.Contains(t, err.Error(), "config error", "error must be prefixed with 'config error'")
}
