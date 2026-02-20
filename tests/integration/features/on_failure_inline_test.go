//go:build integration

package features_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/awf/internal/application"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/internal/infrastructure/executor"
	"github.com/awf-project/awf/internal/infrastructure/repository"
	"github.com/awf-project/awf/internal/testutil/builders"
	"github.com/awf-project/awf/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F066: Inline Error Terminal Shorthand for on_failure Integration Tests
//
// Integration tests validate end-to-end behavior of inline on_failure:
// - Inline error objects with message and optional status
// - Message interpolation at runtime with {{states.*}}, {{inputs.*}}
// - String-form backward compatibility
// - Validation errors for missing/empty message
// - Parallel step inline error handling
//
// These tests complement unit tests (yaml_mapper_on_failure_test.go,
// execution_service_on_failure_inline_test.go) by exercising the full
// workflow lifecycle: parse → validate → execute.

// buildTestService constructs an ExecutionService wired with a YAML repository
// pointing to workflowsDir and in-memory mocks for store, executor, and logger.
func buildTestService(t *testing.T, workflowsDir string) *application.ExecutionService {
	t.Helper()
	repo := repository.NewYAMLRepository(workflowsDir)
	store := mocks.NewMockStateStore()
	exec := executor.NewShellExecutor()
	logger := mocks.NewMockLogger()

	return builders.NewExecutionServiceBuilder().
		WithWorkflowRepository(repo).
		WithStateStore(store).
		WithExecutor(exec).
		WithLogger(logger).
		Build()
}

// TestOnFailureInline_BasicMessage verifies inline on_failure with message only
// Scenario: US1 - Inline error object on on_failure
// Strategy: Create minimal workflow with inline on_failure, trigger failure, verify error message
func TestOnFailureInline_BasicMessage(t *testing.T) {
	tests := []struct {
		name            string
		workflowYAML    string
		expectedMessage string
		expectErr       bool
	}{
		{
			name: "inline message only, default failure status",
			workflowYAML: `
name: test-inline-message
version: "1.0.0"
states:
  initial: failing_step
  failing_step:
    type: step
    command: exit 1
    on_failure:
      message: "Custom error message"
`,
			expectedMessage: "Custom error message",
			expectErr:       true,
		},
		{
			name: "inline message with special characters",
			workflowYAML: `
name: test-inline-special
version: "1.0.0"
states:
  initial: failing_step
  failing_step:
    type: step
    command: exit 1
    on_failure:
      message: "Error: deployment failed (see logs)"
`,
			expectedMessage: "Error: deployment failed (see logs)",
			expectErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			workflowPath := filepath.Join(workflowsDir, "test.yaml")
			require.NoError(t, os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0o644))

			svc := buildTestService(t, workflowsDir)

			_, err := svc.Run(ctx, "test", nil)

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedMessage)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestOnFailureInline_MessageWithStatus verifies inline on_failure with message and status
// Scenario: US1 - Inline error object with optional status field
// Strategy: Create workflow with inline on_failure specifying exit code, verify status propagated
func TestOnFailureInline_MessageWithStatus(t *testing.T) {
	tests := []struct {
		name             string
		workflowYAML     string
		expectedMessage  string
		expectedExitCode int
		expectErr        bool
	}{
		{
			name: "inline message with exit code 1",
			workflowYAML: `
name: test-inline-status-1
version: "1.0.0"
states:
  initial: failing_step
  failing_step:
    type: step
    command: exit 1
    on_failure:
      message: "Build failed"
      status: 1
`,
			expectedMessage:  "Build failed",
			expectedExitCode: 1,
			expectErr:        true,
		},
		{
			name: "inline message with exit code 3",
			workflowYAML: `
name: test-inline-status-3
version: "1.0.0"
states:
  initial: failing_step
  failing_step:
    type: step
    command: exit 1
    on_failure:
      message: "Deploy failed"
      status: 3
`,
			expectedMessage:  "Deploy failed",
			expectedExitCode: 3,
			expectErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			workflowPath := filepath.Join(workflowsDir, "test.yaml")
			require.NoError(t, os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0o644))

			svc := buildTestService(t, workflowsDir)

			execCtx, err := svc.Run(ctx, "test", nil)

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedMessage)
				assert.Equal(t, tt.expectedExitCode, execCtx.ExitCode,
					"ExitCode from inline on_failure status must propagate to execution context")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestOnFailureInline_MessageInterpolation verifies message template interpolation at runtime
// Scenario: US1, FR-003 - Message field supports template interpolation
// Strategy: Create workflow with {{states.*}}, {{inputs.*}} in message, verify substitution
func TestOnFailureInline_MessageInterpolation(t *testing.T) {
	tests := []struct {
		name            string
		workflowYAML    string
		inputs          map[string]any
		expectedMessage string
		expectErr       bool
	}{
		{
			name: "interpolate inputs in message",
			workflowYAML: `
name: test-inline-interp-inputs
version: "1.0.0"
states:
  initial: failing_step
  failing_step:
    type: step
    command: exit 1
    on_failure:
      message: "Operation {{inputs.operation}} failed"
`,
			inputs:          map[string]any{"operation": "deploy"},
			expectedMessage: "Operation deploy failed",
			expectErr:       true,
		},
		{
			name: "interpolate multiple variables in message",
			workflowYAML: `
name: test-inline-interp-multi
version: "1.0.0"
states:
  initial: failing_step
  failing_step:
    type: step
    command: exit 1
    on_failure:
      message: "{{inputs.component}} failed: expected {{inputs.expected}}"
`,
			inputs:          map[string]any{"component": "API", "expected": "success"},
			expectedMessage: "API failed: expected success",
			expectErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			workflowPath := filepath.Join(workflowsDir, "test.yaml")
			require.NoError(t, os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0o644))

			svc := buildTestService(t, workflowsDir)

			_, err := svc.Run(ctx, "test", tt.inputs)

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedMessage)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestOnFailureInline_StringFormBackwardCompat verifies string-form on_failure still works
// Scenario: US2 - Named terminal reference backward compatibility
// Strategy: Create workflow with string-form on_failure referencing named terminal
func TestOnFailureInline_StringFormBackwardCompat(t *testing.T) {
	tests := []struct {
		name            string
		workflowYAML    string
		expectedMessage string
		expectErr       bool
	}{
		{
			name: "string-form on_failure references named terminal",
			workflowYAML: `
name: test-inline-compat-string
version: "1.0.0"
states:
  initial: failing_step
  failing_step:
    type: step
    command: exit 1
    on_failure: error_handler
  error_handler:
    type: terminal
    message: "Handled via named terminal"
    status: failure
`,
			expectedMessage: "Handled via named terminal",
			expectErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			workflowPath := filepath.Join(workflowsDir, "test.yaml")
			require.NoError(t, os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0o644))

			svc := buildTestService(t, workflowsDir)

			_, err := svc.Run(ctx, "test", nil)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestOnFailureInline_ValidateInvalidMessage verifies validation rejects invalid inline errors
// Scenario: US3, FR-006 - awf validate reports errors for invalid inline objects
// Strategy: Run awf validate on workflows with missing/empty message, verify error
func TestOnFailureInline_ValidateInvalidMessage(t *testing.T) {
	tests := []struct {
		name            string
		workflowYAML    string
		expectedError   string
		shouldFailValid bool
	}{
		{
			name: "missing message field rejected",
			workflowYAML: `
name: test-inline-no-message
version: "1.0.0"
states:
  initial: failing_step
  failing_step:
    type: step
    command: exit 1
    on_failure:
      status: 3
`,
			expectedError:   "required field missing",
			shouldFailValid: true,
		},
		{
			name: "empty message field rejected",
			workflowYAML: `
name: test-inline-empty-message
version: "1.0.0"
states:
  initial: failing_step
  failing_step:
    type: step
    command: exit 1
    on_failure:
      message: ""
      status: 1
`,
			expectedError:   "non-empty string",
			shouldFailValid: true,
		},
		{
			name: "empty object rejected",
			workflowYAML: `
name: test-inline-empty-object
version: "1.0.0"
states:
  initial: failing_step
  failing_step:
    type: step
    command: exit 1
    on_failure: {}
`,
			expectedError:   "required field missing",
			shouldFailValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			workflowPath := filepath.Join(workflowsDir, "test.yaml")
			require.NoError(t, os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0o644))

			repo := repository.NewYAMLRepository(workflowsDir)

			_, err := repo.Load(ctx, "test")

			if tt.shouldFailValid {
				require.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.expectedError))
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestOnFailureInline_ParallelBranch verifies inline error in parallel step branches
// Scenario: US4, FR-001 - Inline error objects work in parallel step branches
// Strategy: Create parallel step with inline error in branch, verify behavior
func TestOnFailureInline_ParallelBranch(t *testing.T) {
	tests := []struct {
		name            string
		workflowYAML    string
		expectedMessage string
		expectErr       bool
	}{
		{
			name: "parallel branch with inline error on best_effort strategy",
			workflowYAML: `
name: test-inline-parallel
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    strategy: best_effort
    items:
      - name: branch_a
        type: step
        command: exit 0
      - name: branch_b
        type: step
        command: exit 1
        on_failure:
          message: "Branch B failed"
`,
			expectedMessage: "Branch B failed",
			expectErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			workflowPath := filepath.Join(workflowsDir, "test.yaml")
			require.NoError(t, os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0o644))

			svc := buildTestService(t, workflowsDir)

			_, err := svc.Run(ctx, "test", nil)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestOnFailureInline_SynthesizedTerminalExists verifies synthesized terminal injection
// Scenario: ADR-001 - Inline errors synthesize anonymous terminal at parse time
// Strategy: Load workflow, verify synthesized terminal exists in Steps map
func TestOnFailureInline_SynthesizedTerminalExists(t *testing.T) {
	tests := []struct {
		name          string
		workflowYAML  string
		originalSteps int
		synthesized   string
	}{
		{
			name: "inline error synthesizes __inline_error_ terminal",
			workflowYAML: `
name: test-inline-synth
version: "1.0.0"
states:
  initial: failing_step
  failing_step:
    type: step
    command: exit 1
    on_failure:
      message: "Error"
`,
			originalSteps: 1,
			synthesized:   "__inline_error_failing_step",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			workflowPath := filepath.Join(workflowsDir, "test.yaml")
			require.NoError(t, os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0o644))

			repo := repository.NewYAMLRepository(workflowsDir)

			wf, err := repo.Load(ctx, "test")
			require.NoError(t, err)

			assert.Greater(t, len(wf.Steps), tt.originalSteps, "synthesized terminal should be injected")

			if tt.synthesized != "" {
				_, exists := wf.Steps[tt.synthesized]
				assert.True(t, exists, "synthesized terminal %s should exist in Steps map", tt.synthesized)

				synthesized := wf.Steps[tt.synthesized]
				assert.Equal(t, workflow.StepTypeTerminal, synthesized.Type)
				assert.NotEmpty(t, synthesized.Message, "synthesized terminal should have message")
			}
		})
	}
}
