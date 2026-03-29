package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test SetStepTypeProvider Configuration
// Feature: C069 - Plugin Extensibility
// Component: T014 - Custom Step Type Execution

// TestSetStepTypeProvider verifies that the SetStepTypeProvider method
// correctly configures the step type provider dependency.
func TestSetStepTypeProvider_Configuration(t *testing.T) {
	tests := []struct {
		name     string
		provider ports.StepTypeProvider
	}{
		{
			name:     "sets provider when not nil",
			provider: newMockStepTypeProvider(),
		},
		{
			name:     "sets provider to nil",
			provider: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execSvc, _ := NewTestHarness(t).Build()

			execSvc.SetStepTypeProvider(tt.provider)

			// If provider is set, should use it for custom step types
			// If nil, should fall back to default behavior
			// We verify this indirectly through executeCustomStepType behavior
		})
	}
}

// TestExecuteCustomStepType_HappyPath verifies that custom step types
// are executed successfully when a provider is configured.
func TestExecuteCustomStepType_HappyPath(t *testing.T) {
	tests := []struct {
		name                   string
		customStepType         string
		stepConfig             map[string]any
		providerOutput         string
		providerExitCode       int
		expectedStatus         workflow.ExecutionStatus
		expectedNextStep       string
		shouldCompleteWithNext bool
	}{
		{
			name:                   "custom step executes and completes successfully",
			customStepType:         "custom.validate",
			stepConfig:             map[string]any{"rules": "strict"},
			providerOutput:         "validation passed",
			providerExitCode:       0,
			expectedStatus:         workflow.StatusCompleted,
			expectedNextStep:       "done",
			shouldCompleteWithNext: true,
		},
		{
			name:                   "custom step with zero exit code transitions to OnSuccess",
			customStepType:         "custom.transform",
			stepConfig:             map[string]any{"mode": "compress"},
			providerOutput:         "transformed data",
			providerExitCode:       0,
			expectedStatus:         workflow.StatusCompleted,
			expectedNextStep:       "done",
			shouldCompleteWithNext: true,
		},
		{
			name:                   "custom step with multiple config fields",
			customStepType:         "custom.process",
			stepConfig:             map[string]any{"timeout": 30, "retries": 3, "verbose": true},
			providerOutput:         "processing complete",
			providerExitCode:       0,
			expectedStatus:         workflow.StatusCompleted,
			expectedNextStep:       "done",
			shouldCompleteWithNext: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newMockStepTypeProvider()
			provider.setTypeSupported(tt.customStepType, true)
			provider.setExecuteResult(tt.customStepType, &ports.StepExecuteResult{
				Output:   tt.providerOutput,
				ExitCode: tt.providerExitCode,
				Data:     map[string]any{},
			})

			step := &workflow.Step{
				Name:      "custom",
				Type:      workflow.StepType(tt.customStepType),
				Config:    tt.stepConfig,
				OnSuccess: "done",
			}

			wf := &workflow.Workflow{
				Name:    "custom-step-test",
				Initial: "custom",
				Steps: map[string]*workflow.Step{
					"custom": step,
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			execSvc, _ := NewTestHarness(t).
				WithWorkflow("custom-step-test", wf).
				Build()
			execSvc.SetStepTypeProvider(provider)

			execCtx, err := execSvc.Run(context.Background(), "custom-step-test", nil)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, execCtx.Status)
			if tt.shouldCompleteWithNext {
				assert.Equal(t, tt.expectedNextStep, execCtx.CurrentStep)
			}

			// Verify step state contains provider output
			state, exists := execCtx.States["custom"]
			require.True(t, exists)
			assert.Equal(t, workflow.StatusCompleted, state.Status)
			assert.Equal(t, tt.providerExitCode, state.ExitCode)
			assert.Contains(t, state.Output, tt.providerOutput)
		})
	}
}

// TestExecuteCustomStepType_NoProviderConfigured verifies that custom steps
// fail gracefully when no provider is configured.
func TestExecuteCustomStepType_NoProviderConfigured(t *testing.T) {
	tests := []struct {
		name           string
		customStepType string
		expectedError  string
	}{
		{
			name:           "custom step without provider returns error",
			customStepType: "custom.unknown",
			expectedError:  "step type provider not configured",
		},
		{
			name:           "another custom step without provider fails",
			customStepType: "custom.validate",
			expectedError:  "step type provider not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &workflow.Step{
				Name:      "custom",
				Type:      workflow.StepType(tt.customStepType),
				OnSuccess: "done",
			}

			wf := &workflow.Workflow{
				Name:    "no-provider-test",
				Initial: "custom",
				Steps: map[string]*workflow.Step{
					"custom": step,
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			// Do NOT set provider
			execSvc, _ := NewTestHarness(t).
				WithWorkflow("no-provider-test", wf).
				Build()

			execCtx, err := execSvc.Run(context.Background(), "no-provider-test", nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
			assert.Equal(t, workflow.StatusFailed, execCtx.Status)
		})
	}
}

// TestExecuteCustomStepType_NonZeroExitCode verifies that custom steps
// with non-zero exit codes follow OnFailure paths.
func TestExecuteCustomStepType_NonZeroExitCode(t *testing.T) {
	tests := []struct {
		name               string
		exitCode           int
		hasOnFailure       bool
		onFailureTarget    string
		hasContinueOnError bool
	}{
		{
			name:            "non-zero exit with OnFailure transitions to failure step",
			exitCode:        1,
			hasOnFailure:    true,
			onFailureTarget: "failure",
		},
		{
			name:               "non-zero exit with ContinueOnError transitions to OnSuccess",
			exitCode:           1,
			hasContinueOnError: true,
		},
		{
			name:     "non-zero exit without OnFailure propagates error",
			exitCode: 127,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newMockStepTypeProvider()
			provider.setTypeSupported("custom.process", true)
			provider.setExecuteResult("custom.process", &ports.StepExecuteResult{
				Output:   "process failed",
				ExitCode: tt.exitCode,
			})

			step := &workflow.Step{
				Name:      "custom",
				Type:      "custom.process",
				OnSuccess: "done",
			}

			if tt.hasOnFailure {
				step.OnFailure = tt.onFailureTarget
			}
			if tt.hasContinueOnError {
				step.ContinueOnError = true
			}

			wf := &workflow.Workflow{
				Name:    "exit-code-test",
				Initial: "custom",
				Steps: map[string]*workflow.Step{
					"custom": step,
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
					"failure": {
						Name: "failure",
						Type: workflow.StepTypeTerminal,
					},
					"recover": {
						Name: "recover",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			execSvc, _ := NewTestHarness(t).
				WithWorkflow("exit-code-test", wf).
				Build()
			execSvc.SetStepTypeProvider(provider)

			execCtx, err := execSvc.Run(context.Background(), "exit-code-test", nil)

			if tt.hasOnFailure || tt.hasContinueOnError {
				// handleNonZeroExit evaluates transitions (F068):
				// OnFailure routes to failure step, ContinueOnError routes to OnSuccess
				require.NoError(t, err)
				assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
			} else {
				// No transition configured — error propagates
				require.Error(t, err)
				assert.Equal(t, workflow.StatusFailed, execCtx.Status)
			}
		})
	}
}

// TestExecuteCustomStepType_ProviderError verifies that provider execution errors
// are handled correctly and propagated through the workflow.
func TestExecuteCustomStepType_ProviderError(t *testing.T) {
	tests := []struct {
		name          string
		providerError error
		hasOnFailure  bool
	}{
		{
			name:          "provider returns error",
			providerError: errors.New("plugin execution failed"),
		},
		{
			name:          "provider error with OnFailure transition",
			providerError: errors.New("timeout"),
			hasOnFailure:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newMockStepTypeProvider()
			provider.setTypeSupported("custom.execute", true)
			provider.setExecuteError("custom.execute", tt.providerError)

			step := &workflow.Step{
				Name:      "custom",
				Type:      "custom.execute",
				OnSuccess: "done",
			}

			if tt.hasOnFailure {
				step.OnFailure = "failure"
			}

			wf := &workflow.Workflow{
				Name:    "provider-error-test",
				Initial: "custom",
				Steps: map[string]*workflow.Step{
					"custom": step,
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
					"failure": {
						Name: "failure",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			execSvc, _ := NewTestHarness(t).
				WithWorkflow("provider-error-test", wf).
				Build()
			execSvc.SetStepTypeProvider(provider)

			execCtx, err := execSvc.Run(context.Background(), "provider-error-test", nil)

			// Tests should fail when stub is in place
			require.Error(t, err)
			assert.Equal(t, workflow.StatusFailed, execCtx.Status)
		})
	}
}

// TestExecuteCustomStepType_UnsupportedType verifies that steps with types
// not supported by the provider are handled appropriately.
func TestExecuteCustomStepType_UnsupportedType(t *testing.T) {
	tests := []struct {
		name             string
		stepType         string
		providerSupports string
	}{
		{
			name:             "provider does not support requested step type",
			stepType:         "custom.unknown",
			providerSupports: "custom.supported",
		},
		{
			name:             "empty step type not supported",
			stepType:         "",
			providerSupports: "custom.valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newMockStepTypeProvider()
			provider.setTypeSupported(tt.providerSupports, true)
			provider.setTypeSupported(tt.stepType, false)

			step := &workflow.Step{
				Name:      "custom",
				Type:      workflow.StepType(tt.stepType),
				OnSuccess: "done",
			}

			wf := &workflow.Workflow{
				Name:    "unsupported-type-test",
				Initial: "custom",
				Steps: map[string]*workflow.Step{
					"custom": step,
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			execSvc, _ := NewTestHarness(t).
				WithWorkflow("unsupported-type-test", wf).
				Build()
			execSvc.SetStepTypeProvider(provider)

			execCtx, err := execSvc.Run(context.Background(), "unsupported-type-test", nil)

			require.Error(t, err)
			assert.Equal(t, workflow.StatusFailed, execCtx.Status)
		})
	}
}

// mockStepTypeProvider is a test double for ports.StepTypeProvider
type mockStepTypeProvider struct {
	supportedTypes map[string]bool
	results        map[string]*ports.StepExecuteResult
	errors         map[string]error
}

func newMockStepTypeProvider() *mockStepTypeProvider {
	return &mockStepTypeProvider{
		supportedTypes: make(map[string]bool),
		results:        make(map[string]*ports.StepExecuteResult),
		errors:         make(map[string]error),
	}
}

func (m *mockStepTypeProvider) HasStepType(typeName string) bool {
	supported, exists := m.supportedTypes[typeName]
	return exists && supported
}

func (m *mockStepTypeProvider) ExecuteStep(ctx context.Context, req ports.StepExecuteRequest) (ports.StepExecuteResult, error) {
	if err, exists := m.errors[req.StepType]; exists {
		return ports.StepExecuteResult{}, err
	}

	if result, exists := m.results[req.StepType]; exists {
		return *result, nil
	}

	return ports.StepExecuteResult{}, errors.New("step type not found")
}

func (m *mockStepTypeProvider) setTypeSupported(typeName string, supported bool) {
	m.supportedTypes[typeName] = supported
}

func (m *mockStepTypeProvider) setExecuteResult(typeName string, result *ports.StepExecuteResult) {
	m.results[typeName] = result
}

func (m *mockStepTypeProvider) setExecuteError(typeName string, err error) {
	m.errors[typeName] = err
}
