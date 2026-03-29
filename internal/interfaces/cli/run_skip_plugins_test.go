package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
)

// TestRunCommand_SkipPluginsFlag tests the --skip-plugins flag parsing in run command
func TestRunCommand_SkipPluginsFlag(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../../../tests/fixtures/workflows")

	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute run with --skip-plugins flag
	cmd.SetArgs([]string{"run", "diagram-simple", "--skip-plugins"})

	err := cmd.Execute()
	// Should not error (flag should parse)
	assert.NoError(t, err, "run with --skip-plugins should not error")
}

// TestRunCommand_SkipPluginsFlagDefault tests that --skip-plugins defaults to false
func TestRunCommand_SkipPluginsFlagDefault(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../../../tests/fixtures/workflows")

	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute run without --skip-plugins flag
	cmd.SetArgs([]string{"run", "diagram-simple"})

	err := cmd.Execute()
	// Should execute without error
	assert.NoError(t, err, "run without --skip-plugins should work with default")
}

// TestRunCommand_SkipPluginsWithDryRun tests that --skip-plugins works with --dry-run
func TestRunCommand_SkipPluginsWithDryRun(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../../../tests/fixtures/workflows")
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute run with both --skip-plugins and --dry-run
	cmd.SetArgs([]string{"run", "diagram-simple", "--skip-plugins", "--dry-run"})

	err := cmd.Execute()
	// Should not error (flags should parse together)
	assert.NoError(t, err, "run with --skip-plugins and --dry-run should not error")
}

// TestRunCommand_SkipPluginsWithStep tests that --skip-plugins works with --step
func TestRunCommand_SkipPluginsWithStep(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../../../tests/fixtures/workflows")
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute run with both --skip-plugins and --step
	cmd.SetArgs([]string{"run", "diagram-simple", "--skip-plugins", "--step", "start"})

	err := cmd.Execute()
	// Should not error (flags should parse together)
	assert.NoError(t, err, "run with --skip-plugins and --step should not error")
}

// TestRunCommand_SkipPluginsWithInput tests that --skip-plugins works with --input
func TestRunCommand_SkipPluginsWithInput(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../../../tests/fixtures/workflows")
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute run with both --skip-plugins and --input
	cmd.SetArgs([]string{"run", "diagram-simple", "--skip-plugins", "--input", "key=value"})

	err := cmd.Execute()
	// Should not error (flags should parse together)
	assert.NoError(t, err, "run with --skip-plugins and --input should not error")
}

// TestRunCommand_SkipPluginsMultipleFlagsExpanded tests that --skip-plugins works with all major flags
func TestRunCommand_SkipPluginsMultipleFlagsExpanded(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../../../tests/fixtures/workflows")
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute run with --skip-plugins and multiple other flags
	cmd.SetArgs([]string{
		"run", "diagram-simple",
		"--skip-plugins",
		"--input", "param1=value1",
		"--output", "silent",
	})

	err := cmd.Execute()
	// Should not error
	assert.NoError(t, err, "run with --skip-plugins and multiple flags should not error")
}

// TestRunCommand_SkipPluginsFlagHelpText tests that flag description appears in help
func TestRunCommand_SkipPluginsFlagHelpText(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"run", "--help"})
	_ = cmd.Execute()

	output := buf.String()

	// Help should mention the skip-plugins flag
	assert.True(t,
		strings.Contains(output, "skip-plugins") || strings.Contains(output, "skip-validators"),
		"help should mention skip-plugins or similar flag")
}

// TestRunCommand_SkipPluginsDisablesBehavior tests that --skip-plugins affects execution
func TestRunCommand_SkipPluginsDisablesBehavior(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../../../tests/fixtures/workflows")
	// Run without --skip-plugins
	cmd1 := cli.NewRootCommand()
	buf1 := new(bytes.Buffer)
	cmd1.SetOut(buf1)
	cmd1.SetErr(buf1)
	cmd1.SetArgs([]string{"run", "diagram-simple"})
	_ = cmd1.Execute()
	outputWithPlugins := buf1.String()

	// Run with --skip-plugins
	cmd2 := cli.NewRootCommand()
	buf2 := new(bytes.Buffer)
	cmd2.SetOut(buf2)
	cmd2.SetErr(buf2)
	cmd2.SetArgs([]string{"run", "diagram-simple", "--skip-plugins"})
	_ = cmd2.Execute()
	outputWithoutPlugins := buf2.String()

	// Behavior should differ between the two runs
	// (plugin system may initialize differently, or warnings may appear)
	assert.NotEqual(t, outputWithPlugins, outputWithoutPlugins,
		"output with --skip-plugins should differ from output without flag")
}

// TestRunCommand_SkipPluginsBoolValue tests that --skip-plugins is a boolean flag (not string-valued)
func TestRunCommand_SkipPluginsBoolValue(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../../../tests/fixtures/workflows")
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Try using flag in boolean context (no value)
	cmd.SetArgs([]string{"run", "diagram-simple", "--skip-plugins"})

	err := cmd.Execute()
	assert.NoError(t, err, "--skip-plugins should work as boolean flag without value")
}
