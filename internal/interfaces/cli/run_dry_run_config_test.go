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

func writeConfigFile(t *testing.T, tmpDir string, inputs map[string]string) string {
	t.Helper()
	awfDir := filepath.Join(tmpDir, ".awf")
	require.NoError(t, os.MkdirAll(awfDir, 0o755))

	var sb strings.Builder
	sb.WriteString("inputs:\n")
	for k, v := range inputs {
		sb.WriteString("  " + k + ": \"" + v + "\"\n")
	}
	configPath := filepath.Join(awfDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(sb.String()), 0o644))
	return configPath
}

// dryRunWorkflow is a minimal workflow with two required inputs.
const dryRunWorkflow = `name: dry-run-config-test
description: Test config input merging in dry-run
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
    command: echo "{{.inputs.api_key}} {{.inputs.model}}"
    on_success: done
  done:
    type: terminal
    status: success
`

// TestRunDryRun_ConfigInputsPreFillPlan verifies that inputs from config.yaml appear
// in the dry-run plan when not supplied via CLI.
func TestRunDryRun_ConfigInputsPreFillPlan(t *testing.T) {
	tmpDir := setupTestDir(t)

	configPath := writeConfigFile(t, tmpDir, map[string]string{
		"api_key": "sk-from-config",
	})
	t.Setenv("AWF_CONFIG_PATH", configPath)
	createTestWorkflow(t, tmpDir, "dry-run-config-test.yaml", dryRunWorkflow)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run", "dry-run-config-test",
		"--dry-run",
		"--input", "model=gpt-4",
	})

	err := cmd.Execute()
	require.NoError(t, err, "dry-run should succeed when api_key is provided by config.yaml")

	output := buf.String()
	assert.Contains(t, output, "sk-from-config", "dry-run plan must include api_key value from config.yaml")
	assert.Contains(t, output, "gpt-4", "dry-run plan must include model value from CLI --input")
}

// TestRunDryRun_CLIInputsOverrideConfig verifies that CLI --input values take precedence
// over config.yaml values when the same key is provided in both.
func TestRunDryRun_CLIInputsOverrideConfig(t *testing.T) {
	tmpDir := setupTestDir(t)

	configPath := writeConfigFile(t, tmpDir, map[string]string{
		"api_key": "sk-config",
		"model":   "gpt-config",
	})
	t.Setenv("AWF_CONFIG_PATH", configPath)
	createTestWorkflow(t, tmpDir, "dry-run-config-test.yaml", dryRunWorkflow)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run", "dry-run-config-test",
		"--dry-run",
		"--input", "api_key=sk-cli",
		"--input", "model=gpt-cli",
	})

	err := cmd.Execute()
	require.NoError(t, err, "dry-run should succeed")

	output := buf.String()
	assert.Contains(t, output, "sk-cli", "CLI api_key must override config.yaml value")
	assert.Contains(t, output, "gpt-cli", "CLI model must override config.yaml value")
	assert.NotContains(t, output, "sk-config", "config api_key must not appear when CLI overrides it")
	assert.NotContains(t, output, "gpt-config", "config model must not appear when CLI overrides it")
}

// TestRunDryRun_BothInputsFromConfig verifies that when all required inputs are supplied
// by config.yaml, dry-run succeeds without any --input flags.
func TestRunDryRun_BothInputsFromConfig(t *testing.T) {
	tmpDir := setupTestDir(t)

	configPath := writeConfigFile(t, tmpDir, map[string]string{
		"api_key": "sk-test",
		"model":   "gpt-4",
	})
	t.Setenv("AWF_CONFIG_PATH", configPath)
	createTestWorkflow(t, tmpDir, "dry-run-config-test.yaml", dryRunWorkflow)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run", "dry-run-config-test",
		"--dry-run",
	})

	err := cmd.Execute()
	require.NoError(t, err, "dry-run should succeed when all inputs are provided by config.yaml")

	output := buf.String()
	assert.Contains(t, output, "sk-test", "api_key from config.yaml must appear in dry-run plan")
	assert.Contains(t, output, "gpt-4", "model from config.yaml must appear in dry-run plan")
}

// TestRunDryRun_NoConfigFile_NormalDryRun verifies that dry-run works normally when no
// config.yaml exists and all inputs are provided via --input flags (no regression).
func TestRunDryRun_NoConfigFile_NormalDryRun(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Point to a config path that does not exist (triggers "file not found" path → empty config)
	t.Setenv("AWF_CONFIG_PATH", filepath.Join(tmpDir, ".awf", "config.yaml"))
	createTestWorkflow(t, tmpDir, "dry-run-config-test.yaml", dryRunWorkflow)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run", "dry-run-config-test",
		"--dry-run",
		"--input", "api_key=sk-cli",
		"--input", "model=gpt-4",
	})

	err := cmd.Execute()
	require.NoError(t, err, "dry-run should succeed when no config.yaml exists but all inputs are via --input")

	output := buf.String()
	assert.Contains(t, output, "sk-cli", "api_key from --input must appear in dry-run plan")
	assert.Contains(t, output, "gpt-4", "model from --input must appear in dry-run plan")
}

// TestRunDryRun_InvalidConfig_ReturnsConfigError verifies that a malformed config.yaml
// causes dry-run to fail with a "config error:" prefix.
func TestRunDryRun_InvalidConfig_ReturnsConfigError(t *testing.T) {
	tmpDir := setupTestDir(t)

	awfDir := filepath.Join(tmpDir, ".awf")
	require.NoError(t, os.MkdirAll(awfDir, 0o755))
	configPath := filepath.Join(awfDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("inputs: [\ninvalid yaml:::\n"), 0o644))
	t.Setenv("AWF_CONFIG_PATH", configPath)
	createTestWorkflow(t, tmpDir, "dry-run-config-test.yaml", dryRunWorkflow)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run", "dry-run-config-test",
		"--dry-run",
		"--input", "api_key=sk-cli",
		"--input", "model=gpt-4",
	})

	err := cmd.Execute()
	require.Error(t, err, "dry-run should fail with invalid config.yaml")
	assert.Contains(t, err.Error(), "config error", "error should be prefixed with 'config error'")
}
