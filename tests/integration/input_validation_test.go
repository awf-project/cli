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
// Input Validation Integration Tests (C011 - US2)
// Tests verify input validation rules work correctly at the CLI level:
// - Pattern validation (regex) rejects invalid inputs
// - Enum validation accepts only allowed values
// - Numeric min/max validation enforces bounds
// - Combined validation rules apply correctly
// - Validation error messages are accurate and helpful
//
// Implementation Note (ADR-004):
// Tests use ExecutionService.Run() with invalid inputs to verify full chain:
// CLI input → workflow parsing → validation → error reporting
// =============================================================================

// TestInputValidation_PatternMatch_Integration verifies regex pattern validation
// Feature: C011 - Component: input_validation_tests
// Strategy: Test valid and invalid pattern matches
func TestInputValidation_PatternMatch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name      string
		workflow  string
		inputName string
		value     string
		wantErr   bool
		errSubstr string // Expected substring in error message
	}{
		{
			name:      "valid email pattern matches",
			workflow:  "validation-patterns.yaml",
			inputName: "email",
			value:     "user@example.com",
			wantErr:   false,
		},
		{
			name:      "invalid email pattern rejected",
			workflow:  "validation-patterns.yaml",
			inputName: "email",
			value:     "not-an-email",
			wantErr:   true,
			errSubstr: "pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			statesDir := filepath.Join(tmpDir, "states")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
			require.NoError(t, os.MkdirAll(statesDir, 0o755))

			// Copy fixture to tmpDir
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflow)
			fixtureDest := filepath.Join(workflowsDir, tt.workflow)

			data, err := os.ReadFile(fixtureSource)
			require.NoError(t, err, "fixture file should exist: %s", fixtureSource)
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components using testutil pattern (C007)
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow with inputs
			workflowName := strings.TrimSuffix(tt.workflow, ".yaml")
			inputs := map[string]interface{}{
				tt.inputName: tt.value,
			}

			execCtx, err := svc.Run(ctx, workflowName, inputs)

			// Assert: Verify validation behavior (3-layer assertion pattern from C009)
			// Layer 1: Error presence
			if tt.wantErr {
				require.Error(t, err, "expected validation error")
				// Layer 3: Error details
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.errSubstr),
					"error should mention validation rule type")
			} else {
				// Layer 1: No error
				require.NoError(t, err, "valid input should not produce validation error")
				// Layer 2: Status
				require.NotNil(t, execCtx, "execution context should be returned")
				assert.Equal(t, workflow.StatusCompleted, execCtx.Status,
					"workflow should complete successfully")
			}
		})
	}
}

// TestInputValidation_EnumAllowedValues_Integration verifies enum validation
// Feature: C011 - Component: input_validation_tests
// Strategy: Test values in and out of allowed set
func TestInputValidation_EnumAllowedValues_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name      string
		workflow  string
		inputName string
		value     string
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "allowed enum value accepted",
			workflow:  "validation-enums.yaml",
			inputName: "environment",
			value:     "production",
			wantErr:   false,
		},
		{
			name:      "disallowed enum value rejected",
			workflow:  "validation-enums.yaml",
			inputName: "environment",
			value:     "invalid-env",
			wantErr:   true,
			errSubstr: "allowed values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			statesDir := filepath.Join(tmpDir, "states")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
			require.NoError(t, os.MkdirAll(statesDir, 0o755))

			// Copy fixture to tmpDir
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflow)
			fixtureDest := filepath.Join(workflowsDir, tt.workflow)

			data, err := os.ReadFile(fixtureSource)
			require.NoError(t, err, "fixture file should exist: %s", fixtureSource)
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components using testutil pattern (C007)
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow with inputs
			workflowName := strings.TrimSuffix(tt.workflow, ".yaml")
			inputs := map[string]interface{}{
				tt.inputName: tt.value,
			}

			execCtx, err := svc.Run(ctx, workflowName, inputs)

			// Assert: Verify validation behavior (3-layer assertion pattern from C009)
			// Layer 1: Error presence
			if tt.wantErr {
				require.Error(t, err, "expected validation error for disallowed enum value")
				// Layer 3: Error details
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.errSubstr),
					"error should mention allowed values constraint")
			} else {
				// Layer 1: No error
				require.NoError(t, err, "valid enum value should not produce validation error")
				// Layer 2: Status
				require.NotNil(t, execCtx, "execution context should be returned")
				assert.Equal(t, workflow.StatusCompleted, execCtx.Status,
					"workflow should complete successfully with valid enum")
			}
		})
	}
}

// TestInputValidation_NumericMinMax_Integration verifies min/max validation
// Feature: C011 - Component: input_validation_tests
// Strategy: Test values within and outside bounds
func TestInputValidation_NumericMinMax_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name      string
		workflow  string
		inputName string
		value     string
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "value within min/max range accepted",
			workflow:  "validation-numeric.yaml",
			inputName: "port",
			value:     "8080",
			wantErr:   false,
		},
		{
			name:      "value below minimum rejected",
			workflow:  "validation-numeric.yaml",
			inputName: "port",
			value:     "100",
			wantErr:   true,
			errSubstr: "minimum",
		},
		{
			name:      "value above maximum rejected",
			workflow:  "validation-numeric.yaml",
			inputName: "port",
			value:     "70000",
			wantErr:   true,
			errSubstr: "maximum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			statesDir := filepath.Join(tmpDir, "states")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
			require.NoError(t, os.MkdirAll(statesDir, 0o755))

			// Copy fixture to tmpDir
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflow)
			fixtureDest := filepath.Join(workflowsDir, tt.workflow)

			data, err := os.ReadFile(fixtureSource)
			require.NoError(t, err, "fixture file should exist: %s", fixtureSource)
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components using testutil pattern (C007)
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow with inputs
			workflowName := strings.TrimSuffix(tt.workflow, ".yaml")
			inputs := map[string]interface{}{
				tt.inputName: tt.value,
			}

			execCtx, err := svc.Run(ctx, workflowName, inputs)

			// Assert: Verify validation behavior (3-layer assertion pattern from C009)
			// Layer 1: Error presence
			if tt.wantErr {
				require.Error(t, err, "expected validation error for out-of-bounds value")
				// Layer 3: Error details
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.errSubstr),
					"error should mention min/max constraint violation")
			} else {
				// Layer 1: No error
				require.NoError(t, err, "value within bounds should not produce validation error")
				// Layer 2: Status
				require.NotNil(t, execCtx, "execution context should be returned")
				assert.Equal(t, workflow.StatusCompleted, execCtx.Status,
					"workflow should complete successfully with valid numeric value")
			}
		})
	}
}

// TestInputValidation_CombinedRules_Integration verifies multiple validation rules
// Feature: C011 - Component: input_validation_tests
// Strategy: Test inputs that must satisfy multiple constraints
func TestInputValidation_CombinedRules_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name      string
		workflow  string
		inputs    map[string]string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "all combined rules satisfied",
			workflow: "validation-patterns.yaml",
			inputs: map[string]string{
				"email": "test@example.com",
				"url":   "https://example.com",
			},
			wantErr: false,
		},
		{
			name:     "one of combined rules fails",
			workflow: "validation-patterns.yaml",
			inputs: map[string]string{
				"email": "test@example.com",
				"url":   "not-a-url",
			},
			wantErr:   true,
			errSubstr: "pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			statesDir := filepath.Join(tmpDir, "states")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
			require.NoError(t, os.MkdirAll(statesDir, 0o755))

			// Copy fixture to tmpDir
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflow)
			fixtureDest := filepath.Join(workflowsDir, tt.workflow)

			data, err := os.ReadFile(fixtureSource)
			require.NoError(t, err, "fixture file should exist: %s", fixtureSource)
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components using testutil pattern (C007)
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow with multiple inputs
			workflowName := strings.TrimSuffix(tt.workflow, ".yaml")

			// Convert string inputs to interface{} map
			inputs := make(map[string]interface{})
			for k, v := range tt.inputs {
				inputs[k] = v
			}

			execCtx, err := svc.Run(ctx, workflowName, inputs)

			// Assert: Verify validation behavior (3-layer assertion pattern from C009)
			// Layer 1: Error presence
			if tt.wantErr {
				require.Error(t, err, "expected validation error when one rule fails")
				// Layer 3: Error details
				assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.errSubstr),
					"error should mention the failed validation rule")
			} else {
				// Layer 1: No error
				require.NoError(t, err, "all valid inputs should pass combined validation")
				// Layer 2: Status
				require.NotNil(t, execCtx, "execution context should be returned")
				assert.Equal(t, workflow.StatusCompleted, execCtx.Status,
					"workflow should complete successfully with all valid inputs")
			}
		})
	}
}

// TestInputValidation_ErrorMessages_Integration verifies validation error messages
// Feature: C011 - Component: input_validation_tests
// Strategy: Validate error messages include field name, rule type, and constraint
func TestInputValidation_ErrorMessages_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name        string
		workflow    string
		inputName   string
		value       string
		wantSubstrs []string // All must appear in error message
	}{
		{
			name:      "pattern error mentions field and rule",
			workflow:  "validation-patterns.yaml",
			inputName: "email",
			value:     "invalid",
			wantSubstrs: []string{
				"email",
				"pattern",
			},
		},
		{
			name:      "enum error mentions allowed values",
			workflow:  "validation-enums.yaml",
			inputName: "environment",
			value:     "bad-env",
			wantSubstrs: []string{
				"environment",
				"allowed",
			},
		},
		{
			name:      "min/max error mentions bounds",
			workflow:  "validation-numeric.yaml",
			inputName: "port",
			value:     "99",
			wantSubstrs: []string{
				"port",
				"minimum",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Setup test environment
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")
			statesDir := filepath.Join(tmpDir, "states")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))
			require.NoError(t, os.MkdirAll(statesDir, 0o755))

			// Copy fixture to tmpDir
			fixtureSource := filepath.Join("../fixtures/workflows", tt.workflow)
			fixtureDest := filepath.Join(workflowsDir, tt.workflow)

			data, err := os.ReadFile(fixtureSource)
			require.NoError(t, err, "fixture file should exist: %s", fixtureSource)
			require.NoError(t, os.WriteFile(fixtureDest, data, 0o644))

			// Wire up components using testutil pattern (C007)
			repo := repository.NewYAMLRepository(workflowsDir)
			store := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			logger := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(store).
				WithExecutor(exec).
				WithLogger(logger).
				Build()

			// Act: Execute workflow with invalid input to trigger validation error
			workflowName := strings.TrimSuffix(tt.workflow, ".yaml")
			inputs := map[string]interface{}{
				tt.inputName: tt.value,
			}

			_, err = svc.Run(ctx, workflowName, inputs)

			// Assert: Verify error message contains all expected substrings (Layer 3 detail)
			require.Error(t, err, "invalid input should produce validation error")

			errMsg := strings.ToLower(err.Error())
			for _, substr := range tt.wantSubstrs {
				assert.Contains(t, errMsg, strings.ToLower(substr),
					"error message should contain '%s'", substr)
			}
		})
	}
}
