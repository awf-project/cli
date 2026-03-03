//go:build integration

// Bug: B007
package features_test

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

// ╔══════════════════════════════════════════════════════════════════════════╗
// ║ B007: Interactive Input Prompt Ignores config.yaml Values                ║
// ╠══════════════════════════════════════════════════════════════════════════╣
// ║ Tests verify that config.yaml inputs are merged before interactive and   ║
// ║ dry-run execution, so pre-configured values are not prompted again.      ║
// ║   - US1: Config inputs pre-fill interactive mode (--interactive)         ║
// ║   - US2: Config inputs pre-fill dry-run mode (--dry-run)                 ║
// ╚══════════════════════════════════════════════════════════════════════════╝

// setupB007Env creates a tmpDir with .awf/ and returns the absolute path to
// the fixture workflows directory. Must be called before os.Chdir so the
// relative fixture path resolves correctly.
func setupB007Env(t *testing.T) (tmpDir, fixturesDir string) {
	t.Helper()

	// Resolve fixture path before any chdir
	abs, err := filepath.Abs("../../fixtures/workflows")
	require.NoError(t, err)
	fixturesDir = abs

	tmpDir = t.TempDir()
	awfDir := filepath.Join(tmpDir, ".awf")
	require.NoError(t, os.MkdirAll(awfDir, 0o755))
	return tmpDir, fixturesDir
}

func writeB007Config(t *testing.T, tmpDir string, inputs map[string]string) {
	t.Helper()
	var sb strings.Builder
	sb.WriteString("inputs:\n")
	for k, v := range inputs {
		sb.WriteString("  " + k + ": \"" + v + "\"\n")
	}
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, ".awf", "config.yaml"),
		[]byte(sb.String()),
		0o644,
	))
}

// US1-AC1: Config pre-fills api_key; interactive mode should not prompt for it.
//
// Given: config.yaml has api_key pre-filled
// When: awf run --interactive without --input api_key
// Then: workflow echoes api_key from config (not empty)
func TestB007_ConfigPrefillsInteractiveMode_Integration(t *testing.T) {
	tmpDir, fixturesDir := setupB007Env(t)
	writeB007Config(t, tmpDir, map[string]string{"api_key": "sk-test"})

	originalDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(originalDir)

	t.Setenv("AWF_WORKFLOWS_PATH", fixturesDir)

	// Only model provided via CLI; api_key should come from config.
	// "c\n" continues through the single echo step.
	stdin := strings.NewReader("c\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"run", "config-interactive-test",
		"--interactive",
		"--input", "model=gpt-4",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err, "workflow should complete when api_key is pre-filled by config")

	output := stdout.String()
	assert.Contains(t, output, "api_key=sk-test", "api_key should be pre-filled from config.yaml")
	assert.Contains(t, output, "model=gpt-4", "model should come from --input flag")
}

// US1-AC2: CLI flag overrides config value in interactive mode.
//
// Given: config.yaml has api_key=sk-config
// When: awf run --interactive --input api_key=sk-cli
// Then: api_key=sk-cli (CLI wins over config)
func TestB007_CLIOverridesConfigInInteractiveMode_Integration(t *testing.T) {
	tmpDir, fixturesDir := setupB007Env(t)
	writeB007Config(t, tmpDir, map[string]string{"api_key": "sk-config"})

	originalDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(originalDir)

	t.Setenv("AWF_WORKFLOWS_PATH", fixturesDir)

	stdin := strings.NewReader("c\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"run", "config-interactive-test",
		"--interactive",
		"--input", "api_key=sk-cli",
		"--input", "model=gpt-4",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err, "workflow should complete with CLI override")

	output := stdout.String()
	assert.Contains(t, output, "api_key=sk-cli", "CLI --input should override config.yaml value")
	assert.NotContains(t, output, "sk-config", "config value must not appear when CLI overrides it")
}

// US1-AC3: Both inputs from config; interactive mode needs only step confirmations.
//
// Given: config.yaml has both api_key and model
// When: awf run --interactive with no --input flags
// Then: workflow echoes both values from config
func TestB007_BothInputsFromConfig_Interactive_Integration(t *testing.T) {
	tmpDir, fixturesDir := setupB007Env(t)
	writeB007Config(t, tmpDir, map[string]string{
		"api_key": "sk-test",
		"model":   "gpt-4",
	})

	originalDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(originalDir)

	t.Setenv("AWF_WORKFLOWS_PATH", fixturesDir)

	stdin := strings.NewReader("c\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"run", "config-interactive-test",
		"--interactive",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err, "workflow should complete when both inputs are in config")

	output := stdout.String()
	assert.Contains(t, output, "api_key=sk-test", "api_key should come from config")
	assert.Contains(t, output, "model=gpt-4", "model should come from config")
}

// US2-AC1: Config pre-fills api_key in dry-run; plan should show config value.
//
// Given: config.yaml has api_key pre-filled
// When: awf run --dry-run without --input api_key
// Then: dry-run plan shows api_key from config
func TestB007_ConfigPrefillsDryRun_Integration(t *testing.T) {
	tmpDir, fixturesDir := setupB007Env(t)
	writeB007Config(t, tmpDir, map[string]string{"api_key": "sk-test"})

	originalDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(originalDir)

	t.Setenv("AWF_WORKFLOWS_PATH", fixturesDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"run", "config-interactive-test",
		"--dry-run",
		"--input", "model=gpt-4",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err, "dry-run should succeed when api_key is pre-filled by config")

	output := buf.String()
	assert.Contains(t, output, "sk-test", "dry-run plan should include api_key value from config.yaml")
}

// US2-AC2: No config file; all inputs via CLI (no regression).
//
// Given: no config.yaml exists
// When: awf run --interactive with both inputs via --input flags
// Then: workflow runs normally using CLI values
func TestB007_NoConfigAllInputsViaCLI_NoRegression_Integration(t *testing.T) {
	tmpDir, fixturesDir := setupB007Env(t)
	// No .awf/config.yaml written

	originalDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(originalDir)

	t.Setenv("AWF_WORKFLOWS_PATH", fixturesDir)

	stdin := strings.NewReader("c\n")

	cmd := cli.NewRootCommand()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(stdin)
	cmd.SetArgs([]string{
		"run", "config-interactive-test",
		"--interactive",
		"--input", "api_key=sk-test",
		"--input", "model=gpt-4",
		"--storage", tmpDir,
	})

	err := cmd.Execute()
	require.NoError(t, err, "workflow should work without config.yaml when all inputs provided via CLI")

	output := stdout.String()
	assert.Contains(t, output, "api_key=sk-test", "api_key from --input should be used")
	assert.Contains(t, output, "model=gpt-4", "model from --input should be used")
}
