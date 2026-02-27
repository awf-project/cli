//go:build integration

package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "config" {
			found = true
			break
		}
	}

	assert.True(t, found, "expected root command to have 'config' subcommand")
}

func TestConfigShowCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()
	configCmd, _, err := cmd.Find([]string{"config"})
	require.NoError(t, err)

	found := false
	for _, sub := range configCmd.Commands() {
		if sub.Name() == "show" {
			found = true
			break
		}
	}

	assert.True(t, found, "expected 'config' command to have 'show' subcommand")
}

func TestConfigShowCommand_Help(t *testing.T) {
	cmd := cli.NewRootCommand()

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "show", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "config")
	assert.Contains(t, output, "show")
	assert.Contains(t, output, ".awf/config.yaml")
}

func TestConfigShowCommand_NoExtraArgs(t *testing.T) {
	cmd := cli.NewRootCommand()

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "show", "extra-arg"})

	err := cmd.Execute()
	assert.Error(t, err, "expected error when extra argument provided")
}

func TestConfigShow_ValidConfig_DisplaysAllInputs(t *testing.T) {
	// US3 Acceptance: Given a valid config file, when I run `awf config show`,
	// then all configured inputs are displayed
	tmpDir := setupInitTestDir(t)

	// Create .awf/config.yaml with 3 inputs (per spec independent test)
	awfDir := filepath.Join(tmpDir, ".awf")
	require.NoError(t, os.MkdirAll(awfDir, 0o755))

	configContent := `inputs:
  project: "my-project"
  env: "staging"
  count: 42
`
	require.NoError(t, os.WriteFile(
		filepath.Join(awfDir, "config.yaml"),
		[]byte(configContent),
		0o644,
	))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "show"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	// All 3 inputs should be displayed
	assert.Contains(t, output, "project")
	assert.Contains(t, output, "my-project")
	assert.Contains(t, output, "env")
	assert.Contains(t, output, "staging")
	assert.Contains(t, output, "count")
	assert.Contains(t, output, "42")
}

func TestConfigShow_NoConfigFile_DisplaysMessage(t *testing.T) {
	// US3 Acceptance: Given no config file, when I run `awf config show`,
	// then a message indicates no project config found
	tmpDir := setupInitTestDir(t)

	// Isolate from global config
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Don't create any .awf/config.yaml

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "show"})

	err := cmd.Execute()
	// Should not error - missing config is not an error (FR-004)
	require.NoError(t, err)

	output := out.String()
	// Should indicate no config found
	assert.True(t,
		strings.Contains(output, "No") || strings.Contains(output, "not found") || strings.Contains(output, "no config"),
		"expected message indicating no config found, got: %s", output,
	)
}

func TestConfigShow_EmptyInputs_DisplaysPath(t *testing.T) {
	// Config file exists but has empty inputs section
	tmpDir := setupInitTestDir(t)

	awfDir := filepath.Join(tmpDir, ".awf")
	require.NoError(t, os.MkdirAll(awfDir, 0o755))

	configContent := `# Empty config
inputs: {}
`
	require.NoError(t, os.WriteFile(
		filepath.Join(awfDir, "config.yaml"),
		[]byte(configContent),
		0o644,
	))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "show"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	// Should show config file path even if no inputs
	assert.Contains(t, output, ".awf/config.yaml")
}

func TestConfigShow_JSONFormat(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	awfDir := filepath.Join(tmpDir, ".awf")
	require.NoError(t, os.MkdirAll(awfDir, 0o755))

	configContent := `inputs:
  project: "test"
  enabled: true
`
	require.NoError(t, os.WriteFile(
		filepath.Join(awfDir, "config.yaml"),
		[]byte(configContent),
		0o644,
	))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "show", "--format", "json"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should be valid JSON
	var result cli.ConfigShowOutput
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should be valid JSON: %s", output)

	// Should contain expected fields
	assert.True(t, result.Exists, "exists should be true when config file exists")
	assert.Contains(t, result.Path, "config.yaml")
	assert.Equal(t, "test", result.Inputs["project"])
	assert.True(t, result.Inputs["enabled"].(bool))
}

func TestConfigShow_JSONFormat_NoConfig(t *testing.T) {
	_ = setupInitTestDir(t)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "show", "--format", "json"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	var result cli.ConfigShowOutput
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should be valid JSON: %s", output)

	assert.False(t, result.Exists, "exists should be false when no config file")
	assert.Empty(t, result.Inputs, "inputs should be empty when no config file")
}

func TestConfigShow_QuietFormat(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	awfDir := filepath.Join(tmpDir, ".awf")
	require.NoError(t, os.MkdirAll(awfDir, 0o755))

	configContent := `inputs:
  key1: "value1"
  key2: "value2"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(awfDir, "config.yaml"),
		[]byte(configContent),
		0o644,
	))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "show", "--format", "quiet"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	// Quiet mode: just input names, one per line
	assert.GreaterOrEqual(t, len(lines), 2, "should have at least 2 lines for 2 inputs")
}

func TestConfigShow_TableFormat(t *testing.T) {
	tmpDir := setupInitTestDir(t)

	awfDir := filepath.Join(tmpDir, ".awf")
	require.NoError(t, os.MkdirAll(awfDir, 0o755))

	configContent := `inputs:
  project: "test"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(awfDir, "config.yaml"),
		[]byte(configContent),
		0o644,
	))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "show", "--format", "table"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	// Table should have headers
	assert.True(t,
		strings.Contains(output, "KEY") || strings.Contains(output, "NAME") || strings.Contains(output, "INPUT"),
		"table should have column headers",
	)
	assert.True(t,
		strings.Contains(output, "VALUE") || strings.Contains(output, "project"),
		"table should show input values",
	)
}

func TestConfigShow_AllInputTypes(t *testing.T) {
	// Test that all YAML types are displayed correctly
	tmpDir := setupInitTestDir(t)

	awfDir := filepath.Join(tmpDir, ".awf")
	require.NoError(t, os.MkdirAll(awfDir, 0o755))

	configContent := `inputs:
  string_val: "hello"
  int_val: 42
  float_val: 3.14
  bool_true: true
  bool_false: false
`
	require.NoError(t, os.WriteFile(
		filepath.Join(awfDir, "config.yaml"),
		[]byte(configContent),
		0o644,
	))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "show"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "string_val")
	assert.Contains(t, output, "hello")
	assert.Contains(t, output, "int_val")
	assert.Contains(t, output, "42")
	assert.Contains(t, output, "float_val")
	assert.Contains(t, output, "3.14")
	assert.Contains(t, output, "bool_true")
	assert.Contains(t, output, "true")
	assert.Contains(t, output, "bool_false")
	assert.Contains(t, output, "false")
}

func TestConfigShow_InvalidYAML_ReturnsError(t *testing.T) {
	// FR-005: Invalid YAML in config file produces exit code 1 with descriptive error
	tmpDir := setupInitTestDir(t)

	awfDir := filepath.Join(tmpDir, ".awf")
	require.NoError(t, os.MkdirAll(awfDir, 0o755))

	// Invalid YAML syntax
	invalidContent := `inputs:
  key: value
  bad_indent:
- this breaks
    yaml: parsing
`
	require.NoError(t, os.WriteFile(
		filepath.Join(awfDir, "config.yaml"),
		[]byte(invalidContent),
		0o644,
	))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "show"})

	err := cmd.Execute()
	assert.Error(t, err, "should return error for invalid YAML")

	// Error message should be descriptive
	errOutput := out.String()
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	combined := errOutput + errMsg
	assert.True(t,
		strings.Contains(combined, "YAML") || strings.Contains(combined, "yaml") ||
			strings.Contains(combined, "parse") || strings.Contains(combined, "config"),
		"error should mention YAML parsing issue, got: %s", combined,
	)
}

func TestConfigShow_ConfigPathDisplayed(t *testing.T) {
	// FR-001: Config file located at .awf/config.yaml
	tmpDir := setupInitTestDir(t)

	awfDir := filepath.Join(tmpDir, ".awf")
	require.NoError(t, os.MkdirAll(awfDir, 0o755))

	configContent := `inputs:
  test: "value"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(awfDir, "config.yaml"),
		[]byte(configContent),
		0o644,
	))

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "show"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	// Should show the config file path
	assert.Contains(t, output, ".awf/config.yaml")
}

func TestConfigShow_HintToRunInit(t *testing.T) {
	// When no config exists, suggest running awf init
	_ = setupInitTestDir(t)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "show"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	// Should hint to run awf init
	assert.Contains(t, output, "init", "should suggest running 'awf init'")
}

func TestConfigShow_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		configContent  string
		createConfig   bool
		expectError    bool
		expectContains []string
		expectNotEmpty bool
	}{
		{
			name:           "single input",
			configContent:  "inputs:\n  key: value\n",
			createConfig:   true,
			expectContains: []string{"key", "value"},
		},
		{
			name:           "multiple inputs",
			configContent:  "inputs:\n  a: 1\n  b: 2\n  c: 3\n",
			createConfig:   true,
			expectContains: []string{"a", "b", "c"},
		},
		{
			name:           "no config file",
			createConfig:   false,
			expectContains: []string{"No", "config"},
		},
		{
			name:          "empty file",
			configContent: "",
			createConfig:  true,
			// Should handle gracefully - empty config is valid
		},
		{
			name:           "only comments",
			configContent:  "# This is a comment\n# Another comment\n",
			createConfig:   true,
			expectContains: []string{".awf/config.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := setupInitTestDir(t)

			if tt.createConfig {
				awfDir := filepath.Join(tmpDir, ".awf")
				require.NoError(t, os.MkdirAll(awfDir, 0o755))
				require.NoError(t, os.WriteFile(
					filepath.Join(awfDir, "config.yaml"),
					[]byte(tt.configContent),
					0o644,
				))
			}

			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"config", "show"})

			err := cmd.Execute()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			output := out.String()
			for _, expected := range tt.expectContains {
				assert.Contains(t, output, expected,
					"expected output to contain %q", expected)
			}

			if tt.expectNotEmpty {
				assert.NotEmpty(t, output)
			}
		})
	}
}

func TestConfigShowOutput_JSONMarshaling(t *testing.T) {
	output := cli.ConfigShowOutput{
		Path:   ".awf/config.yaml",
		Exists: true,
		Inputs: map[string]any{
			"key1": "value1",
			"key2": 42,
		},
	}

	data, err := json.Marshal(output)
	require.NoError(t, err)

	var parsed cli.ConfigShowOutput
	require.NoError(t, json.Unmarshal(data, &parsed))

	assert.Equal(t, output.Path, parsed.Path)
	assert.Equal(t, output.Exists, parsed.Exists)
	assert.Equal(t, "value1", parsed.Inputs["key1"])
}

func TestConfigShowOutput_EmptyInputsOmitted(t *testing.T) {
	output := cli.ConfigShowOutput{
		Path:   ".awf/config.yaml",
		Exists: false,
		Inputs: nil,
	}

	data, err := json.Marshal(output)
	require.NoError(t, err)

	// Inputs should be omitted when nil (omitempty tag)
	assert.NotContains(t, string(data), "inputs")
}
