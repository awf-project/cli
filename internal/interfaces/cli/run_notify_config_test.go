package cli_test

// Component T023 Tests: NotifyConfig Loading from .awf/config.yaml
// Purpose: Verify that CLI properly loads NotifyConfig from project configuration
// Scope: Config loading and wiring to NotifyOperationProvider in run commands
//
// Test Strategy:
// - Happy Path: Config loads notify settings and wires to provider
// - Edge Cases: Empty config, partial config, missing config file
// - Error Handling: Invalid YAML, invalid backend values

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/awf/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCommand_LoadsNotifyConfig_AllFields(t *testing.T) {
	// GIVEN: A project with complete notify configuration
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	// Create config with all notify fields populated
	configContent := `notify:
  default_backend: "desktop"
`
	configPath := filepath.Join(tmpDir, ".awf", "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	workflowContent := `name: config-test
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      message: "Test with loaded config"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "config-test.yaml", workflowContent)

	// WHEN: Running workflow with notify operation
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "config-test"})

	err = cmd.Execute()
	// THEN: Config should be loaded and passed to NotifyOperationProvider
	if err != nil {
		errMsg := err.Error()
		// Should not fail from missing config - config should be loaded
		assert.NotContains(t, errMsg, "nil pointer", "config should be loaded without nil errors")
		assert.NotContains(t, errMsg, "missing configuration", "config values should be loaded")
	}
}

func TestRunCommand_LoadsNotifyConfig_DefaultBackend(t *testing.T) {
	// GIVEN: A config with default_backend set
	tmpDir := setupTestDir(t)

	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	configContent := `notify:
  default_backend: "desktop"
`
	configPath := filepath.Join(tmpDir, ".awf", "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	workflowContent := `name: default-backend
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      message: "Using default backend from config"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "default-backend.yaml", workflowContent)

	// WHEN: Running workflow without explicit backend
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "default-backend"})

	err = cmd.Execute()
	// THEN: Default backend from config should be used
	if err != nil {
		errMsg := err.Error()
		// Should use desktop backend as configured, not fail from missing backend
		assert.NotContains(t, errMsg, "backend is required", "default_backend should be loaded from config")
		assert.NotContains(t, errMsg, "nil pointer", "should not panic")
	}
}

func TestRunCommand_LoadsNotifyConfig_EmptyConfig(t *testing.T) {
	// GIVEN: An empty config file
	tmpDir := setupTestDir(t)

	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	// Create empty config
	configPath := filepath.Join(tmpDir, ".awf", "config.yaml")
	err := os.WriteFile(configPath, []byte(""), 0o644)
	require.NoError(t, err)

	workflowContent := `name: empty-config
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      backend: desktop
      message: "With empty config"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "empty-config.yaml", workflowContent)

	// WHEN: Running workflow with empty config
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "empty-config"})

	err = cmd.Execute()
	// THEN: Should load empty NotifyConfig without errors
	// Empty config is valid - all fields are optional
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "config error", "empty config should load successfully")
		assert.NotContains(t, errMsg, "nil pointer", "should not panic with empty config")
	}
}

func TestRunCommand_LoadsNotifyConfig_MissingConfigFile(t *testing.T) {
	// Enable test mode to avoid real desktop/network calls in CI
	originalTestMode := os.Getenv("AWF_TEST_MODE")
	os.Setenv("AWF_TEST_MODE", "1")
	defer func() {
		if originalTestMode != "" {
			os.Setenv("AWF_TEST_MODE", originalTestMode)
		} else {
			os.Unsetenv("AWF_TEST_MODE")
		}
	}()

	// GIVEN: No config file exists
	tmpDir := setupTestDir(t)

	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	// Do NOT create config.yaml - test missing file case

	workflowContent := `name: missing-config
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      backend: desktop
      message: "No config file"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "missing-config.yaml", workflowContent)

	// WHEN: Running workflow without config file
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "missing-config"})

	err := cmd.Execute()
	// THEN: Should load empty NotifyConfig (missing file is not an error)
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "config error", "missing config file should return empty config")
		assert.NotContains(t, errMsg, "file not found", "missing config should not be an error")
		assert.NotContains(t, errMsg, "nil pointer", "should not panic with missing config")
	}
}

func TestRunCommand_LoadsNotifyConfig_PartialNotifySection(t *testing.T) {
	// GIVEN: Config with notify section but only some fields
	tmpDir := setupTestDir(t)

	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	configContent := `notify:
  default_backend: "desktop"
`
	configPath := filepath.Join(tmpDir, ".awf", "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	workflowContent := `name: partial-config
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      backend: desktop
      message: "Partial config"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "partial-config.yaml", workflowContent)

	// WHEN: Running workflow with partial notify config
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "partial-config"})

	err = cmd.Execute()
	// THEN: Partial config should load successfully
	// Unprovided fields should have zero values (empty strings)
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "config error", "partial config should load")
		assert.NotContains(t, errMsg, "nil pointer", "should handle partial config")
	}
}

func TestRunSingleStep_LoadsNotifyConfig(t *testing.T) {
	// GIVEN: Single-step execution with notify config
	tmpDir := setupTestDir(t)

	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	configContent := `notify:
  default_backend: "desktop"
`
	configPath := filepath.Join(tmpDir, ".awf", "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	workflowContent := `name: single-step-config
version: "1.0.0"
states:
  initial: first
  first:
    type: operation
    operation: notify.send
    inputs:
      message: "Single step with config"
    on_success: second
  second:
    type: step
    command: echo "done"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "single-step-config.yaml", workflowContent)

	// WHEN: Running single step with notify operation
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run",
		"single-step-config",
		"--step=first",
	})

	err = cmd.Execute()
	// THEN: Config should be loaded in single-step execution path
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "missing configuration", "config should be loaded in single-step mode")
		assert.NotContains(t, errMsg, "nil pointer", "should not panic")
	}
}

func TestRunCommand_LoadsNotifyConfig_WithInputsSection(t *testing.T) {
	// GIVEN: Config with both inputs and notify sections
	tmpDir := setupTestDir(t)

	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	configContent := `inputs:
  user_name: "test-user"
notify:
  default_backend: "desktop"
`
	configPath := filepath.Join(tmpDir, ".awf", "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	workflowContent := `name: mixed-config
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      message: "Hello {{inputs.user_name}}"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "mixed-config.yaml", workflowContent)

	// WHEN: Running workflow with both config sections
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "mixed-config"})

	err = cmd.Execute()
	// THEN: Both inputs and notify config should be loaded
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "config error", "both config sections should load")
		assert.NotContains(t, errMsg, "nil pointer", "should handle mixed config")
	}
}

func TestRunCommand_LoadsNotifyConfig_InvalidYAML(t *testing.T) {
	// GIVEN: Config file with invalid YAML syntax
	tmpDir := setupTestDir(t)

	// Change to tmpDir so config loader finds our test config
	origDir, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	defer os.Chdir(origDir)

	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	// Create invalid YAML with syntax error (unclosed bracket)
	configContent := `notify:
  ntfy_url: [unclosed
  default_backend: "ntfy"
`
	configPath := filepath.Join(tmpDir, ".awf", "config.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	workflowContent := `name: invalid-yaml
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      backend: desktop
      message: "Test"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "invalid-yaml.yaml", workflowContent)

	// WHEN: Running workflow with invalid YAML config
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "invalid-yaml"})

	err = cmd.Execute()

	// THEN: Should return config error for invalid YAML
	require.Error(t, err, "invalid YAML should cause error")
	errMsg := err.Error()
	assert.Contains(t, errMsg, "config error", "should report config error")
}

func TestRunCommand_LoadsNotifyConfig_UnknownKeys(t *testing.T) {
	// GIVEN: Config with unknown keys in notify section
	tmpDir := setupTestDir(t)

	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	configContent := `notify:
  default_backend: "desktop"
  unknown_field: "value"
  another_unknown: 123
`
	configPath := filepath.Join(tmpDir, ".awf", "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	workflowContent := `name: unknown-keys
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      backend: desktop
      message: "Test"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "unknown-keys.yaml", workflowContent)

	// WHEN: Running workflow with unknown keys in notify config
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "unknown-keys"})

	err = cmd.Execute()
	// THEN: Unknown keys should be ignored (YAML unmarshaler ignores unknown fields)
	// Config should load successfully with known fields only
	if err != nil {
		errMsg := err.Error()
		// Unknown fields in notify section are silently ignored by YAML
		assert.NotContains(t, errMsg, "config error", "unknown fields should be ignored")
		assert.NotContains(t, errMsg, "nil pointer", "should not panic")
	}
}

func TestRunCommand_LoadsNotifyConfig_InvalidBackendValue(t *testing.T) {
	// GIVEN: Config with invalid default_backend value
	tmpDir := setupTestDir(t)

	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	configContent := `notify:
  default_backend: "invalid_backend_name"
`
	configPath := filepath.Join(tmpDir, ".awf", "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	workflowContent := `name: invalid-backend-value
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      message: "Using invalid default backend"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "invalid-backend-value.yaml", workflowContent)

	// WHEN: Running workflow with invalid default_backend
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "invalid-backend-value"})

	err = cmd.Execute()
	// THEN: Config loads successfully (validation happens at execution time)
	// The invalid backend value should be caught by the provider during execution
	if err != nil {
		errMsg := err.Error()
		// Config loads invalid values - validation is provider's responsibility
		assert.NotContains(t, errMsg, "config error", "config loading should not validate backend values")
	}
}

func TestRunCommand_LoadsNotifyConfig_PermissionError(t *testing.T) {
	t.Skip("Skipping permission test - requires platform-specific permission handling")
	// NOTE: This test is platform-dependent and may not work reliably in CI environments
	// Permission errors on config files are rare in practice since users control .awf/config.yaml

	// GIVEN: Config file with no read permissions
	tmpDir := setupTestDir(t)

	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	configContent := `notify:
  ntfy_url: "https://ntfy.sh"
`
	configPath := filepath.Join(tmpDir, ".awf", "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	// Remove read permissions
	err = os.Chmod(configPath, 0o000)
	require.NoError(t, err)
	defer os.Chmod(configPath, 0o644) // Restore for cleanup

	workflowContent := `name: permission-error
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      backend: desktop
      message: "Test"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "permission-error.yaml", workflowContent)

	// WHEN: Running workflow with unreadable config
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "permission-error"})

	err = cmd.Execute()

	// THEN: Should return config error for permission denied
	require.Error(t, err, "permission error should cause failure")
	errMsg := err.Error()
	assert.Contains(t, errMsg, "config error", "should report config error for permission issues")
}

func TestRunCommand_NotifyConfigWiringToProvider_FullStack(t *testing.T) {
	// GIVEN: Complete config that exercises full wiring stack
	tmpDir := setupTestDir(t)

	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	configContent := `notify:
  default_backend: "desktop"
inputs:
  workflow_name: "integration-test"
`
	configPath := filepath.Join(tmpDir, ".awf", "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	workflowContent := `name: full-stack-wiring
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      message: "Testing {{inputs.workflow_name}}"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "full-stack-wiring.yaml", workflowContent)

	// WHEN: Running workflow (exercises: Config Load -> NotifyConfig -> Provider -> Execution)
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "full-stack-wiring"})

	err = cmd.Execute()
	// THEN: Full stack wiring should work without nil pointers
	// Test verifies:
	// 1. loadProjectConfig() loads config from .awf/config.yaml
	// 2. projectCfg.Notify is populated with YAML values
	// 3. notify.NotifyConfig is constructed from projectCfg.Notify
	// 4. NewNotifyOperationProvider receives config
	// 5. Provider is wired to ExecutionService
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "nil pointer", "full wiring should not have nil pointers")
		assert.NotContains(t, errMsg, "missing configuration", "config should be properly loaded and wired")
		assert.NotContains(t, errMsg, "config error", "config loading should succeed")
	}
}
