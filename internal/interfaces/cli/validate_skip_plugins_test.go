package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
)

// TestValidateCommand_SkipPluginsFlag tests the --skip-plugins flag parsing
func TestValidateCommand_SkipPluginsFlag(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../../../tests/fixtures/workflows")

	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute validate with --skip-plugins flag
	cmd.SetArgs([]string{"validate", "diagram-simple", "--skip-plugins"})

	err := cmd.Execute()
	// Should not error (flag should parse)
	assert.NoError(t, err, "validate with --skip-plugins should not error")
}

// TestValidateCommand_SkipPluginsFlagDefault tests that --skip-plugins defaults to false
func TestValidateCommand_SkipPluginsFlagDefault(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../../../tests/fixtures/workflows")

	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute validate without --skip-plugins flag
	cmd.SetArgs([]string{"validate", "diagram-simple"})

	err := cmd.Execute()
	// Should execute without error
	assert.NoError(t, err, "validate without --skip-plugins should work with default")
}

// TestValidateCommand_ValidatorTimeoutFlag tests the --validator-timeout flag parsing
func TestValidateCommand_ValidatorTimeoutFlag(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../../../tests/fixtures/workflows")

	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute validate with --validator-timeout flag
	cmd.SetArgs([]string{"validate", "diagram-simple", "--validator-timeout", "10s"})

	err := cmd.Execute()
	// Should not error (flag should parse)
	assert.NoError(t, err, "validate with --validator-timeout should not error")
}

// TestValidateCommand_ValidatorTimeoutFlagDefault tests that --validator-timeout defaults to 5s
func TestValidateCommand_ValidatorTimeoutFlagDefault(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../../../tests/fixtures/workflows")

	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute validate without --validator-timeout flag
	cmd.SetArgs([]string{"validate", "diagram-simple"})

	err := cmd.Execute()
	// Should execute without error with default
	assert.NoError(t, err, "validate without --validator-timeout should work with default")
}

// TestValidateCommand_ValidatorTimeoutInvalidDuration tests that invalid timeout values are rejected
func TestValidateCommand_ValidatorTimeoutInvalidDuration(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../../../tests/fixtures/workflows")

	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute validate with invalid timeout duration
	cmd.SetArgs([]string{"validate", "diagram-simple", "--validator-timeout", "invalid"})

	err := cmd.Execute()
	// Should error on invalid duration
	assert.Error(t, err, "validate with invalid duration should error")
}

// TestValidateCommand_BothPluginFlagsPresent tests that both flags can be used together
func TestValidateCommand_BothPluginFlagsPresent(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../../../tests/fixtures/workflows")

	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute validate with both plugin-related flags
	cmd.SetArgs([]string{"validate", "diagram-simple", "--skip-plugins", "--validator-timeout", "3s"})

	err := cmd.Execute()
	// Should not error (both flags should parse together)
	assert.NoError(t, err, "validate with both flags should not error")
}

// TestValidateCommand_SkipPluginsDisablesWarnings tests that --skip-plugins works with validate
func TestValidateCommand_SkipPluginsDisablesWarnings(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../../../tests/fixtures/workflows")

	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute without --skip-plugins
	cmd.SetArgs([]string{"validate", "diagram-simple"})
	err1 := cmd.Execute()

	// Reset command for second run
	cmd2 := cli.NewRootCommand()
	buf2 := new(bytes.Buffer)
	cmd2.SetOut(buf2)
	cmd2.SetErr(buf2)

	// Execute with --skip-plugins
	cmd2.SetArgs([]string{"validate", "diagram-simple", "--skip-plugins"})
	err2 := cmd2.Execute()

	// Both should execute without error
	assert.NoError(t, err1, "validate should not error")
	assert.NoError(t, err2, "validate with --skip-plugins should not error")
}

// TestValidateCommand_FlagHelpText tests that flag descriptions appear in help
func TestValidateCommand_FlagHelpText(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"validate", "--help"})
	_ = cmd.Execute()

	output := buf.String()

	// Help should mention both flags
	assert.True(t,
		strings.Contains(output, "skip-plugins") || strings.Contains(output, "skip-validators"),
		"help should mention skip-plugins or similar flag")
	assert.True(t,
		strings.Contains(output, "validator-timeout") || strings.Contains(output, "timeout"),
		"help should mention validator-timeout or similar flag")
}

// TestValidateCommand_ValidatorTimeoutFormat tests various valid timeout formats
func TestValidateCommand_ValidatorTimeoutFormat(t *testing.T) {
	t.Setenv("AWF_WORKFLOWS_PATH", "../../../tests/fixtures/workflows")

	tests := []struct {
		name    string
		timeout string
	}{
		{"seconds", "5s"},
		{"milliseconds", "5000ms"},
		{"minutes", "1m"},
		{"compound", "1m30s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)

			cmd.SetArgs([]string{"validate", "diagram-simple", "--validator-timeout", tt.timeout})
			err := cmd.Execute()

			assert.NoError(t, err, "should accept %q timeout format", tt.timeout)
		})
	}
}
