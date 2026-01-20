//go:build integration

package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/executor"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/internal/testutil"
)

// =============================================================================
// Secret Masking Integration Tests (C011 - US3)
// Tests validate that secret-prefixed variables are masked in logs:
// - Variables prefixed with SECRET_ are masked in stdout/stderr
// - Variables prefixed with API_KEY are masked in output
// - Variables prefixed with PASSWORD are masked in output
// - Non-secret variables remain visible
// - Secrets are masked in error messages
//
// Implementation Note:
// Tests use real ShellExecutor and capture output via logger to verify
// masking occurs at the infrastructure layer.
// =============================================================================

// TestSecretMasking_SECRET_Prefix_Integration verifies SECRET_* variables are masked in output
// Feature: C011 - Task T007
// Strategy: Run workflow with SECRET_* variable in command output, verify masked as ***
func TestSecretMasking_SECRET_Prefix_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name            string
		workflowFile    string
		inputs          map[string]any
		wantLogContains []string // Should find these masked values
		wantLogExcludes []string // Should NOT find these secret values
		wantErr         bool
	}{
		{
			name:         "SECRET_ prefix masked in command output",
			workflowFile: "secrets-masked.yaml",
			inputs: map[string]any{
				"SECRET_API_TOKEN": "super_secret_value_12345",
				"PUBLIC_VAR":       "visible_value",
			},
			wantLogContains: []string{
				"***",           // Masked secret placeholder
				"visible_value", // Public variable remains visible
			},
			wantLogExcludes: []string{
				"super_secret_value_12345", // Actual secret should be masked
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			outputFile := filepath.Join(tmpDir, "output.log")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			// Copy fixture to tmpDir
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflowFile)
			fixtureDest := filepath.Join(workflowsDir, tt.workflowFile)

			data, err := os.ReadFile(fixtureSource)
			require.NoError(t, err, "fixture file should exist")
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components with real executor and mock logger
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			// Set OUTPUT_FILE environment variable for workflow commands
			t.Setenv("OUTPUT_FILE", outputFile)

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow with secret variables
			workflowName := strings.TrimSuffix(tt.workflowFile, ".yaml")
			execCtx, err := svc.Run(ctx, workflowName, tt.inputs)

			// Assert - Layer 1: Error checking
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err, "workflow should complete successfully")
			}

			// Assert - Layer 2: Status verification
			require.NotNil(t, execCtx, "execution context should be returned")
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status, "workflow should complete")

			// Assert - Layer 3: Secret masking verification
			// Check step state output (masked by executor)
			stepState, ok := execCtx.GetStepState("print_vars")
			require.True(t, ok, "step state should exist")
			outputContent := stepState.Output

			// Verify masked placeholders exist
			for _, expectedSubstr := range tt.wantLogContains {
				assert.Contains(t, outputContent, expectedSubstr, "output should contain %s", expectedSubstr)
			}

			// Verify actual secrets are NOT present
			for _, secretValue := range tt.wantLogExcludes {
				assert.NotContains(t, outputContent, secretValue, "output should NOT contain secret: %s", secretValue)
			}

			// Additionally check logger captured logs
			logMessages := logger.GetMessages()
			var logLines []string
			for _, msg := range logMessages {
				logLines = append(logLines, msg.Msg)
			}
			logContent := strings.Join(logLines, "\n")

			for _, secretValue := range tt.wantLogExcludes {
				assert.NotContains(t, logContent, secretValue, "logger should NOT contain secret: %s", secretValue)
			}
		})
	}
}

// TestSecretMasking_API_KEY_Prefix_Integration verifies API_KEY* variables are masked in output
// Feature: C011 - Task T007
// Strategy: Run workflow with API_KEY* variable, verify masked in logs
func TestSecretMasking_API_KEY_Prefix_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name            string
		workflowFile    string
		inputs          map[string]any
		wantLogContains []string
		wantLogExcludes []string
		wantErr         bool
	}{
		{
			name:         "API_KEY prefix masked in command output",
			workflowFile: "secrets-masked.yaml",
			inputs: map[string]any{
				"API_KEY_OPENAI": "sk-proj-abc123def456",
				"PUBLIC_VAR":     "visible_api_name",
			},
			wantLogContains: []string{
				"***",
				"visible_api_name",
			},
			wantLogExcludes: []string{
				"sk-proj-abc123def456",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			outputFile := filepath.Join(tmpDir, "output.log")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			// Copy fixture
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflowFile)
			fixtureDest := filepath.Join(workflowsDir, tt.workflowFile)
			data, err := os.ReadFile(fixtureSource)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			t.Setenv("OUTPUT_FILE", outputFile)

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow
			workflowName := strings.TrimSuffix(tt.workflowFile, ".yaml")
			execCtx, err := svc.Run(ctx, workflowName, tt.inputs)

			// Assert - Layer 1: Error
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// Assert - Layer 2: Status
			require.NotNil(t, execCtx)
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

			// Assert - Layer 3: Secret masking
			stepState, ok := execCtx.GetStepState("print_vars")
			require.True(t, ok, "step state should exist")
			outputContent := stepState.Output

			for _, expectedSubstr := range tt.wantLogContains {
				assert.Contains(t, outputContent, expectedSubstr)
			}

			for _, secretValue := range tt.wantLogExcludes {
				assert.NotContains(t, outputContent, secretValue, "API_KEY should be masked")
			}

			// Additionally check logger
			logMessages := logger.GetMessages()
			var logLines []string
			for _, msg := range logMessages {
				logLines = append(logLines, msg.Msg)
			}
			logContent := strings.Join(logLines, "\n")
			for _, secretValue := range tt.wantLogExcludes {
				assert.NotContains(t, logContent, secretValue, "logger should NOT contain secret")
			}
		})
	}
}

// TestSecretMasking_PASSWORD_Prefix_Integration verifies PASSWORD* variables are masked in output
// Feature: C011 - Task T007
// Strategy: Run workflow with PASSWORD* variable, verify masked in logs
func TestSecretMasking_PASSWORD_Prefix_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name            string
		workflowFile    string
		inputs          map[string]any
		wantLogContains []string
		wantLogExcludes []string
		wantErr         bool
	}{
		{
			name:         "PASSWORD prefix masked in command output",
			workflowFile: "secrets-masked.yaml",
			inputs: map[string]any{
				"PASSWORD_DB": "admin_pass_987654",
				"USERNAME":    "visible_user",
			},
			wantLogContains: []string{
				"***",
				"visible_user",
			},
			wantLogExcludes: []string{
				"admin_pass_987654",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			outputFile := filepath.Join(tmpDir, "output.log")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			// Copy fixture
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflowFile)
			fixtureDest := filepath.Join(workflowsDir, tt.workflowFile)
			data, err := os.ReadFile(fixtureSource)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			t.Setenv("OUTPUT_FILE", outputFile)

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow
			workflowName := strings.TrimSuffix(tt.workflowFile, ".yaml")
			execCtx, err := svc.Run(ctx, workflowName, tt.inputs)

			// Assert - Layer 1: Error
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// Assert - Layer 2: Status
			require.NotNil(t, execCtx)
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

			// Assert - Layer 3: Secret masking
			stepState, ok := execCtx.GetStepState("print_vars")
			require.True(t, ok, "step state should exist")
			outputContent := stepState.Output

			for _, expectedSubstr := range tt.wantLogContains {
				assert.Contains(t, outputContent, expectedSubstr)
			}

			for _, secretValue := range tt.wantLogExcludes {
				assert.NotContains(t, outputContent, secretValue, "PASSWORD should be masked")
			}

			// Additionally check logger
			logMessages := logger.GetMessages()
			var logLines []string
			for _, msg := range logMessages {
				logLines = append(logLines, msg.Msg)
			}
			logContent := strings.Join(logLines, "\n")
			for _, secretValue := range tt.wantLogExcludes {
				assert.NotContains(t, logContent, secretValue, "logger should NOT contain secret")
			}
		})
	}
}

// TestSecretMasking_NonSecrets_Integration verifies non-secret variables remain visible
// Feature: C011 - Task T007
// Strategy: Run workflow with mixed secret/non-secret vars, verify only secrets masked
func TestSecretMasking_NonSecrets_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name            string
		workflowFile    string
		inputs          map[string]any
		wantLogContains []string
		wantLogExcludes []string
		wantErr         bool
	}{
		{
			name:         "non-secret variables remain visible",
			workflowFile: "secrets-masked.yaml",
			inputs: map[string]any{
				"SECRET_TOKEN": "hidden_value",
				"PUBLIC_VAR":   "visible_value",
				"NORMAL_VAR":   "also_visible",
				"CONFIG_VAR":   "config_visible",
			},
			wantLogContains: []string{
				"visible_value",
				"also_visible",
				"config_visible",
				"***", // Secret placeholder
			},
			wantLogExcludes: []string{
				"hidden_value", // Only secret should be masked
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			outputFile := filepath.Join(tmpDir, "output.log")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			// Copy fixture
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflowFile)
			fixtureDest := filepath.Join(workflowsDir, tt.workflowFile)
			data, err := os.ReadFile(fixtureSource)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			t.Setenv("OUTPUT_FILE", outputFile)

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow with mixed variables
			workflowName := strings.TrimSuffix(tt.workflowFile, ".yaml")
			execCtx, err := svc.Run(ctx, workflowName, tt.inputs)

			// Assert - Layer 1: Error
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// Assert - Layer 2: Status
			require.NotNil(t, execCtx)
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

			// Assert - Layer 3: Verify selective masking
			stepState, ok := execCtx.GetStepState("print_vars")
			require.True(t, ok, "step state should exist")
			outputContent := stepState.Output

			// Public variables should be visible
			for _, visibleValue := range tt.wantLogContains {
				assert.Contains(t, outputContent, visibleValue, "public var should be visible: %s", visibleValue)
			}

			// Secrets should be masked
			for _, secretValue := range tt.wantLogExcludes {
				assert.NotContains(t, outputContent, secretValue, "secret should be masked")
			}

			// Additionally check logger
			logMessages := logger.GetMessages()
			var logLines []string
			for _, msg := range logMessages {
				logLines = append(logLines, msg.Msg)
			}
			logContent := strings.Join(logLines, "\n")
			for _, secretValue := range tt.wantLogExcludes {
				assert.NotContains(t, logContent, secretValue, "logger should NOT contain secret")
			}
		})
	}
}

// TestSecretMasking_ErrorOutput_Integration verifies secrets masked in stderr and error messages
// Feature: C011 - Task T007
// Strategy: Run workflow that fails with secret in error output, verify masked
func TestSecretMasking_ErrorOutput_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name            string
		workflowFile    string
		inputs          map[string]any
		wantErrContains string   // Error message should contain this
		wantErrExcludes []string // Error message should NOT contain secrets
		wantErr         bool
	}{
		{
			name:         "secrets masked in error output",
			workflowFile: "secrets-in-errors.yaml",
			inputs: map[string]any{
				"SECRET_API_KEY": "confidential_key_xyz789",
			},
			wantErrContains: "***", // Masked secret in error
			wantErrExcludes: []string{
				"confidential_key_xyz789", // Actual secret should not appear
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			errorFile := filepath.Join(tmpDir, "error.log")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			// Copy fixture
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflowFile)
			fixtureDest := filepath.Join(workflowsDir, tt.workflowFile)
			data, err := os.ReadFile(fixtureSource)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			t.Setenv("ERROR_FILE", errorFile)

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute failing workflow with secret
			workflowName := strings.TrimSuffix(tt.workflowFile, ".yaml")
			execCtx, err := svc.Run(ctx, workflowName, tt.inputs)

			// Assert - Layer 1: Error checking
			if tt.wantErr {
				require.Error(t, err, "workflow should fail")
			}

			// Assert - Layer 2: Status verification
			require.NotNil(t, execCtx)
			assert.Equal(t, workflow.StatusFailed, execCtx.Status)

			// Assert - Layer 3: Secret masking in error output
			// Check step stderr (masked by executor)
			stepState, ok := execCtx.GetStepState("fail_with_secret")
			require.True(t, ok, "step state should exist")
			stderrContent := stepState.Stderr

			assert.Contains(t, stderrContent, tt.wantErrContains, "stderr should contain masked placeholder")

			for _, secretValue := range tt.wantErrExcludes {
				assert.NotContains(t, stderrContent, secretValue, "stderr should NOT contain secret")
			}

			// Check logger captured logs
			logMessages := logger.GetMessages()
			var logLines []string
			for _, msg := range logMessages {
				logLines = append(logLines, msg.Msg)
			}
			logContent := strings.Join(logLines, "\n")

			for _, secretValue := range tt.wantErrExcludes {
				assert.NotContains(t, logContent, secretValue, "logger should NOT contain secret in error logs")
			}
		})
	}
}

// TestSecretMasking_CaseInsensitive_Integration verifies secret prefixes work regardless of case
// Feature: C011 - Task T007 (Edge case)
// Strategy: Test secret_, api_key_, password_ (lowercase) also get masked
func TestSecretMasking_CaseInsensitive_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name            string
		workflowYAML    string // Inline YAML for custom scenarios
		inputs          map[string]any
		wantLogExcludes []string
		wantErr         bool
	}{
		{
			name: "lowercase secret_ prefix also masked",
			workflowYAML: `name: secrets-case-test
version: "1.0.0"
inputs:
  - name: secret_token
    type: string
    required: true
  - name: api_key_service
    type: string
    required: true
states:
  initial: print_vars
  print_vars:
    type: step
    command: |
      echo "Token: {{.inputs.secret_token}}"
      echo "API: {{.inputs.api_key_service}}"
    on_success: done
  done:
    type: terminal
`,
			inputs: map[string]any{
				"secret_token":    "lowercase_secret_123",
				"api_key_service": "lowercase_api_456",
			},
			wantLogExcludes: []string{
				"lowercase_secret_123",
				"lowercase_api_456",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment with inline YAML
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			outputFile := filepath.Join(tmpDir, "output.log")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			// Write inline workflow
			workflowPath := filepath.Join(workflowsDir, "test-case.yaml")
			require.NoError(t, os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0o644))

			// Wire up components
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			t.Setenv("OUTPUT_FILE", outputFile)

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow
			execCtx, err := svc.Run(ctx, "test-case", tt.inputs)

			// Assert - Layer 1: Error
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// Assert - Layer 2: Status
			require.NotNil(t, execCtx)
			assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

			// Assert - Layer 3: Case-insensitive masking
			stepState, ok := execCtx.GetStepState("print_vars")
			require.True(t, ok, "step state should exist")
			outputContent := stepState.Output

			for _, secretValue := range tt.wantLogExcludes {
				assert.NotContains(t, outputContent, secretValue, "lowercase secret prefix should also be masked")
			}

			// Additionally check logger
			logMessages := logger.GetMessages()
			var logLines []string
			for _, msg := range logMessages {
				logLines = append(logLines, msg.Msg)
			}
			logContent := strings.Join(logLines, "\n")
			for _, secretValue := range tt.wantLogExcludes {
				assert.NotContains(t, logContent, secretValue, "logger should NOT contain secret")
			}
		})
	}
}
