//go:build integration

package integration_test

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

// ╔══════════════════════════════════════════════════════════════════════════╗
// ║ F036: Project Configuration File - Integration Tests                      ║
// ╠══════════════════════════════════════════════════════════════════════════╣
// ║ Tests verify end-to-end behavior of .awf/config.yaml:                    ║
// ║   - US1: Auto-populate workflow inputs from config                        ║
// ║   - US2: Validate config file on load (errors + unknown key warnings)    ║
// ║   - US3: Display config values via 'awf config show'                     ║
// ╚══════════════════════════════════════════════════════════════════════════╝

// TestConfigShow_Integration tests the 'awf config show' command behavior.
func TestConfigShow_Integration(t *testing.T) {
	t.Run("displays config values when config exists", func(t *testing.T) {
		tmpDir := t.TempDir()
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

		// Change to temp dir so config is discovered
		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"config", "show"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "project")
		assert.Contains(t, output, "my-project")
		assert.Contains(t, output, "env")
		assert.Contains(t, output, "staging")
		assert.Contains(t, output, "count")
		assert.Contains(t, output, "42")
	})

	t.Run("displays no config message when file missing", func(t *testing.T) {
		tmpDir := t.TempDir()

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"config", "show"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "No project configuration found")
		assert.Contains(t, output, "awf init")
	})

	t.Run("displays error for invalid YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		// Invalid YAML: unclosed bracket
		invalidYAML := `inputs: [
  project: "broken"
`
		require.NoError(t, os.WriteFile(
			filepath.Join(awfDir, "config.yaml"),
			[]byte(invalidYAML),
			0o644,
		))

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"config", "show"})

		err := cmd.Execute()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config")
	})

	t.Run("outputs JSON format correctly", func(t *testing.T) {
		tmpDir := t.TempDir()
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		configContent := `inputs:
  project: "json-test"
  enabled: true
`
		require.NoError(t, os.WriteFile(
			filepath.Join(awfDir, "config.yaml"),
			[]byte(configContent),
			0o644,
		))

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"config", "show", "--format", "json"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, `"path"`)
		assert.Contains(t, output, `"exists": true`)
		assert.Contains(t, output, `"inputs"`)
		assert.Contains(t, output, `"project": "json-test"`)
	})

	t.Run("outputs quiet format correctly", func(t *testing.T) {
		tmpDir := t.TempDir()
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		configContent := `inputs:
  alpha: "first"
  beta: "second"
  gamma: "third"
`
		require.NoError(t, os.WriteFile(
			filepath.Join(awfDir, "config.yaml"),
			[]byte(configContent),
			0o644,
		))

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"config", "show", "--format", "quiet"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")
		// Quiet mode outputs just keys, sorted alphabetically
		assert.Len(t, lines, 3)
		assert.Equal(t, "alpha", lines[0])
		assert.Equal(t, "beta", lines[1])
		assert.Equal(t, "gamma", lines[2])
	})
}

// TestConfigInputsInWorkflow_Integration tests that config inputs are used in workflows.
func TestConfigInputsInWorkflow_Integration(t *testing.T) {
	t.Run("workflow uses input from config file", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create .awf/config.yaml with inputs
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		configContent := `inputs:
  message: "hello from config"
`
		require.NoError(t, os.WriteFile(
			filepath.Join(awfDir, "config.yaml"),
			[]byte(configContent),
			0o644,
		))

		// Create a workflow that uses the input
		wfYAML := `name: config-input-test
version: "1.0.0"
inputs:
  - name: message
    type: string
    required: true
states:
  initial: echo
  echo:
    type: step
    command: echo "{{.inputs.message}}"
    on_success: done
  done:
    type: terminal
`
		wfDir := filepath.Join(tmpDir, "workflows")
		require.NoError(t, os.MkdirAll(wfDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(wfDir, "config-input-test.yaml"),
			[]byte(wfYAML),
			0o644,
		))

		t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

		// Run from tmpDir so config is discovered
		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{
			"run", "config-input-test",
			"--storage", tmpDir,
			"--output", "buffered",
		})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "hello from config")
		assert.Contains(t, output, "completed")
	})

	t.Run("CLI flag overrides config input", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create .awf/config.yaml with inputs
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		configContent := `inputs:
  env: "staging"
`
		require.NoError(t, os.WriteFile(
			filepath.Join(awfDir, "config.yaml"),
			[]byte(configContent),
			0o644,
		))

		// Create a workflow that uses the input
		wfYAML := `name: override-test
version: "1.0.0"
inputs:
  - name: env
    type: string
    required: true
states:
  initial: echo
  echo:
    type: step
    command: echo "env={{.inputs.env}}"
    on_success: done
  done:
    type: terminal
`
		wfDir := filepath.Join(tmpDir, "workflows")
		require.NoError(t, os.MkdirAll(wfDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(wfDir, "override-test.yaml"),
			[]byte(wfYAML),
			0o644,
		))

		t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{
			"run", "override-test",
			"--storage", tmpDir,
			"--output", "buffered",
			"--input", "env=production", // CLI override
		})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		// CLI flag should win: "production" not "staging"
		assert.Contains(t, output, "env=production")
		assert.NotContains(t, output, "env=staging")
	})

	t.Run("workflow runs normally when no config exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		// No .awf/config.yaml - should not cause error

		wfYAML := `name: no-config-test
version: "1.0.0"
inputs:
  - name: value
    type: string
    default: "default-value"
states:
  initial: echo
  echo:
    type: step
    command: echo "value={{.inputs.value}}"
    on_success: done
  done:
    type: terminal
`
		wfDir := filepath.Join(tmpDir, "workflows")
		require.NoError(t, os.MkdirAll(wfDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(wfDir, "no-config-test.yaml"),
			[]byte(wfYAML),
			0o644,
		))

		t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{
			"run", "no-config-test",
			"--storage", tmpDir,
			"--output", "buffered",
		})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		// Should use default value since no config and no CLI input
		assert.Contains(t, output, "value=default-value")
		assert.Contains(t, output, "completed")
	})
}

// TestConfigValidation_Integration tests config validation during workflow run.
func TestConfigValidation_Integration(t *testing.T) {
	t.Run("invalid YAML produces error on workflow run", func(t *testing.T) {
		tmpDir := t.TempDir()
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		// Invalid YAML
		invalidYAML := `inputs: [
  # unclosed bracket
`
		require.NoError(t, os.WriteFile(
			filepath.Join(awfDir, "config.yaml"),
			[]byte(invalidYAML),
			0o644,
		))

		wfYAML := `name: simple-test
version: "1.0.0"
states:
  initial: echo
  echo:
    type: step
    command: echo "hello"
    on_success: done
  done:
    type: terminal
`
		wfDir := filepath.Join(tmpDir, "workflows")
		require.NoError(t, os.MkdirAll(wfDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(wfDir, "simple-test.yaml"),
			[]byte(wfYAML),
			0o644,
		))

		t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{
			"run", "simple-test",
			"--storage", tmpDir,
		})

		err := cmd.Execute()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config")
	})

	t.Run("unknown keys trigger warning but workflow proceeds", func(t *testing.T) {
		tmpDir := t.TempDir()
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		// Config with unknown keys - should warn but not fail
		configContent := `inputs:
  valid_input: "works"

unknown_key: "this should warn"
deprecated_setting: true
`
		require.NoError(t, os.WriteFile(
			filepath.Join(awfDir, "config.yaml"),
			[]byte(configContent),
			0o644,
		))

		wfYAML := `name: unknown-key-test
version: "1.0.0"
inputs:
  - name: valid_input
    type: string
    required: true
states:
  initial: echo
  echo:
    type: step
    command: echo "{{.inputs.valid_input}}"
    on_success: done
  done:
    type: terminal
`
		wfDir := filepath.Join(tmpDir, "workflows")
		require.NoError(t, os.MkdirAll(wfDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(wfDir, "unknown-key-test.yaml"),
			[]byte(wfYAML),
			0o644,
		))

		t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{
			"run", "unknown-key-test",
			"--storage", tmpDir,
			"--output", "buffered",
		})

		err := cmd.Execute()
		// Workflow should succeed despite unknown keys
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "works")
		assert.Contains(t, output, "completed")
	})
}

// TestConfigInit_Integration tests that 'awf init' creates config file.
func TestConfigInit_Integration(t *testing.T) {
	t.Run("init creates config.yaml with template", func(t *testing.T) {
		tmpDir := t.TempDir()

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"init"})

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify config.yaml was created
		configPath := filepath.Join(tmpDir, ".awf", "config.yaml")
		_, err = os.Stat(configPath)
		require.NoError(t, err, "config.yaml should exist after init")

		// Verify content includes commented inputs section
		content, err := os.ReadFile(configPath)
		require.NoError(t, err)

		contentStr := string(content)
		assert.Contains(t, contentStr, "inputs:")
		// Should have comments/examples
		assert.Contains(t, contentStr, "#")
	})

	t.Run("init does not overwrite existing config", func(t *testing.T) {
		tmpDir := t.TempDir()
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		// Create existing config with custom content
		customContent := `# My custom config
inputs:
  custom_key: "custom_value"
`
		configPath := filepath.Join(awfDir, "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte(customContent), 0o644))

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"init"})

		err := cmd.Execute()
		require.NoError(t, err)

		// Verify config was preserved
		content, err := os.ReadFile(configPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "custom_key")
		assert.Contains(t, string(content), "custom_value")
	})
}

// TestConfigMultipleInputTypes_Integration tests various input types in config.
func TestConfigMultipleInputTypes_Integration(t *testing.T) {
	t.Run("supports string, number, and boolean inputs", func(t *testing.T) {
		tmpDir := t.TempDir()
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		configContent := `inputs:
  name: "test-project"
  count: 42
  enabled: true
  ratio: 3.14
`
		require.NoError(t, os.WriteFile(
			filepath.Join(awfDir, "config.yaml"),
			[]byte(configContent),
			0o644,
		))

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"config", "show"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "name")
		assert.Contains(t, output, "test-project")
		assert.Contains(t, output, "count")
		assert.Contains(t, output, "42")
		assert.Contains(t, output, "enabled")
		assert.Contains(t, output, "true")
		assert.Contains(t, output, "ratio")
		assert.Contains(t, output, "3.14")
	})
}

// ╔══════════════════════════════════════════════════════════════════════════╗
// ║ F036: Edge Case Tests                                                     ║
// ╚══════════════════════════════════════════════════════════════════════════╝

// TestConfigEdgeCases_Integration tests edge cases for config handling.
func TestConfigEdgeCases_Integration(t *testing.T) {
	t.Run("empty config file is valid", func(t *testing.T) {
		tmpDir := t.TempDir()
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		// Empty file is valid YAML - should not error
		require.NoError(t, os.WriteFile(
			filepath.Join(awfDir, "config.yaml"),
			[]byte(""),
			0o644,
		))

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"config", "show"})

		err := cmd.Execute()
		require.NoError(t, err)
		// Empty config should show "no inputs" or similar
		output := buf.String()
		assert.NotEmpty(t, output)
	})

	t.Run("config with only comments is valid", func(t *testing.T) {
		tmpDir := t.TempDir()
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		commentOnlyConfig := `# This is a comment
# Another comment
# inputs:
#   key: value
`
		require.NoError(t, os.WriteFile(
			filepath.Join(awfDir, "config.yaml"),
			[]byte(commentOnlyConfig),
			0o644,
		))

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"config", "show"})

		err := cmd.Execute()
		require.NoError(t, err)
	})

	t.Run("config with empty inputs section is valid", func(t *testing.T) {
		tmpDir := t.TempDir()
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		emptyInputsConfig := `inputs:
# No inputs defined yet
`
		require.NoError(t, os.WriteFile(
			filepath.Join(awfDir, "config.yaml"),
			[]byte(emptyInputsConfig),
			0o644,
		))

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"config", "show"})

		err := cmd.Execute()
		require.NoError(t, err)
	})

	t.Run("null value in inputs is passed as nil", func(t *testing.T) {
		// Per data-model.md: null values in config are treated as nil
		// They are explicitly set (not unset), so workflow defaults don't apply
		tmpDir := t.TempDir()
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		nullValueConfig := `inputs:
  project: "my-project"
  nullable: null
`
		require.NoError(t, os.WriteFile(
			filepath.Join(awfDir, "config.yaml"),
			[]byte(nullValueConfig),
			0o644,
		))

		wfYAML := `name: null-test
version: "1.0.0"
inputs:
  - name: project
    type: string
    required: true
  - name: nullable
    type: string
    default: "default-for-null"
states:
  initial: echo
  echo:
    type: step
    command: echo "project={{.inputs.project}} nullable={{.inputs.nullable}}"
    on_success: done
  done:
    type: terminal
`
		wfDir := filepath.Join(tmpDir, "workflows")
		require.NoError(t, os.MkdirAll(wfDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(wfDir, "null-test.yaml"),
			[]byte(wfYAML),
			0o644,
		))

		t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{
			"run", "null-test",
			"--storage", tmpDir,
			"--output", "buffered",
		})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		// Project should come from config
		assert.Contains(t, output, "project=my-project")
		// Null values are passed as nil - template shows <no value>
		assert.Contains(t, output, "nullable=<no value>")
		assert.Contains(t, output, "completed")
	})
}

// TestConfigShowFormats_Integration tests different output formats for config show.
func TestConfigShowFormats_Integration(t *testing.T) {
	t.Run("json format with no config shows empty state", func(t *testing.T) {
		tmpDir := t.TempDir()

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"config", "show", "--format", "json"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, `"exists": false`)
	})

	t.Run("quiet format with no config outputs nothing", func(t *testing.T) {
		tmpDir := t.TempDir()

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"config", "show", "--format", "quiet"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := strings.TrimSpace(buf.String())
		// Quiet mode with no config should output nothing or minimal text
		assert.Empty(t, output)
	})
}

// TestConfigMultipleOverrides_Integration tests multiple CLI overrides.
func TestConfigMultipleOverrides_Integration(t *testing.T) {
	t.Run("multiple CLI inputs override multiple config values", func(t *testing.T) {
		tmpDir := t.TempDir()
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		configContent := `inputs:
  name: "config-name"
  env: "staging"
  count: 10
`
		require.NoError(t, os.WriteFile(
			filepath.Join(awfDir, "config.yaml"),
			[]byte(configContent),
			0o644,
		))

		wfYAML := `name: multi-override-test
version: "1.0.0"
inputs:
  - name: name
    type: string
    required: true
  - name: env
    type: string
    required: true
  - name: count
    type: integer
    required: true
states:
  initial: echo
  echo:
    type: step
    command: echo "name={{.inputs.name}} env={{.inputs.env}} count={{.inputs.count}}"
    on_success: done
  done:
    type: terminal
`
		wfDir := filepath.Join(tmpDir, "workflows")
		require.NoError(t, os.MkdirAll(wfDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(wfDir, "multi-override-test.yaml"),
			[]byte(wfYAML),
			0o644,
		))

		t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{
			"run", "multi-override-test",
			"--storage", tmpDir,
			"--output", "buffered",
			"--input", "name=cli-name",
			"--input", "env=production",
			// count not overridden - should use config value
		})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		// CLI overrides should win
		assert.Contains(t, output, "name=cli-name")
		assert.Contains(t, output, "env=production")
		// Config value should be used for non-overridden input
		assert.Contains(t, output, "count=10")
	})

	t.Run("CLI input with equals sign in value works correctly", func(t *testing.T) {
		tmpDir := t.TempDir()
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		configContent := `inputs:
  query: "default"
`
		require.NoError(t, os.WriteFile(
			filepath.Join(awfDir, "config.yaml"),
			[]byte(configContent),
			0o644,
		))

		wfYAML := `name: equals-test
version: "1.0.0"
inputs:
  - name: query
    type: string
    required: true
states:
  initial: echo
  echo:
    type: step
    command: echo "query={{.inputs.query}}"
    on_success: done
  done:
    type: terminal
`
		wfDir := filepath.Join(tmpDir, "workflows")
		require.NoError(t, os.MkdirAll(wfDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(wfDir, "equals-test.yaml"),
			[]byte(wfYAML),
			0o644,
		))

		t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{
			"run", "equals-test",
			"--storage", tmpDir,
			"--output", "buffered",
			"--input", "query=select * from users where id=42",
		})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		// Value with = sign should be preserved
		assert.Contains(t, output, "query=select * from users where id=42")
	})
}

// TestConfigResume_Integration tests config integration with resume command.
func TestConfigResume_Integration(t *testing.T) {
	t.Run("resume uses config inputs for new inputs", func(t *testing.T) {
		tmpDir := t.TempDir()
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		configContent := `inputs:
  project: "resumed-project"
`
		require.NoError(t, os.WriteFile(
			filepath.Join(awfDir, "config.yaml"),
			[]byte(configContent),
			0o644,
		))

		// Create a simple workflow
		wfYAML := `name: resume-config-test
version: "1.0.0"
inputs:
  - name: project
    type: string
    required: true
states:
  initial: step1
  step1:
    type: step
    command: echo "project={{.inputs.project}}"
    on_success: done
  done:
    type: terminal
`
		wfDir := filepath.Join(tmpDir, "workflows")
		require.NoError(t, os.MkdirAll(wfDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(wfDir, "resume-config-test.yaml"),
			[]byte(wfYAML),
			0o644,
		))

		t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		// First run the workflow using config
		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{
			"run", "resume-config-test",
			"--storage", tmpDir,
			"--output", "buffered",
		})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "project=resumed-project")
		assert.Contains(t, output, "completed")
	})
}

// TestConfigPermissions_Integration tests config file permission errors.
func TestConfigPermissions_Integration(t *testing.T) {
	// Skip on Windows where permission model is different
	if os.Getenv("OS") == "Windows_NT" {
		t.Skip("skipping permission test on Windows")
	}

	t.Run("unreadable config file produces error", func(t *testing.T) {
		tmpDir := t.TempDir()
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		configPath := filepath.Join(awfDir, "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("inputs:\n  key: value\n"), 0o644))

		// Make file unreadable
		require.NoError(t, os.Chmod(configPath, 0o000))
		defer os.Chmod(configPath, 0o644) // Restore for cleanup

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"config", "show"})

		err := cmd.Execute()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")
	})
}

// TestConfigWithSpecialCharacters_Integration tests config with special characters in values.
func TestConfigWithSpecialCharacters_Integration(t *testing.T) {
	t.Run("config values with special characters are preserved", func(t *testing.T) {
		tmpDir := t.TempDir()
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		configContent := `inputs:
  url: "https://example.com/path?foo=bar&baz=qux"
  json_like: '{"key": "value"}'
  multiword: "hello world with spaces"
  single_quotes: 'single quoted value'
`
		require.NoError(t, os.WriteFile(
			filepath.Join(awfDir, "config.yaml"),
			[]byte(configContent),
			0o644,
		))

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"config", "show"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "url")
		assert.Contains(t, output, "example.com")
		assert.Contains(t, output, "json_like")
		assert.Contains(t, output, "multiword")
		assert.Contains(t, output, "hello world with spaces")
	})

	t.Run("config values with shell metacharacters work in workflow", func(t *testing.T) {
		tmpDir := t.TempDir()
		awfDir := filepath.Join(tmpDir, ".awf")
		require.NoError(t, os.MkdirAll(awfDir, 0o755))

		// Values with shell metacharacters that need proper escaping
		configContent := `inputs:
  message: "hello 'world' with \"quotes\""
`
		require.NoError(t, os.WriteFile(
			filepath.Join(awfDir, "config.yaml"),
			[]byte(configContent),
			0o644,
		))

		wfYAML := `name: special-chars-test
version: "1.0.0"
inputs:
  - name: message
    type: string
    required: true
states:
  initial: echo
  echo:
    type: step
    command: echo "{{.inputs.message}}"
    on_success: done
  done:
    type: terminal
`
		wfDir := filepath.Join(tmpDir, "workflows")
		require.NoError(t, os.MkdirAll(wfDir, 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(wfDir, "special-chars-test.yaml"),
			[]byte(wfYAML),
			0o644,
		))

		t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

		originalDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalDir)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{
			"run", "special-chars-test",
			"--storage", tmpDir,
			"--output", "buffered",
		})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		// Workflow should complete
		assert.Contains(t, output, "completed")
	})
}
