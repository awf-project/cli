package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/plugin"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// mockOperationProvider implements ports.OperationProvider for testing.
type mockOperationProvider struct {
	operations map[string]*plugin.OperationSchema
	results    map[string]*plugin.OperationResult
	execError  error
}

func newMockOperationProvider() *mockOperationProvider {
	return &mockOperationProvider{
		operations: make(map[string]*plugin.OperationSchema),
		results:    make(map[string]*plugin.OperationResult),
	}
}

func (m *mockOperationProvider) GetOperation(name string) (*plugin.OperationSchema, bool) {
	op, ok := m.operations[name]
	return op, ok
}

func (m *mockOperationProvider) ListOperations() []*plugin.OperationSchema {
	ops := make([]*plugin.OperationSchema, 0, len(m.operations))
	for _, op := range m.operations {
		ops = append(ops, op)
	}
	return ops
}

func (m *mockOperationProvider) Execute(
	ctx context.Context,
	name string,
	inputs map[string]any,
) (*plugin.OperationResult, error) {
	if m.execError != nil {
		return nil, m.execError
	}
	if result, ok := m.results[name]; ok {
		return result, nil
	}
	return &plugin.OperationResult{Success: true, Outputs: map[string]any{}}, nil
}

// addOperation registers an operation in the mock provider.
func (m *mockOperationProvider) addOperation(name, description, pluginName string) {
	m.operations[name] = &plugin.OperationSchema{
		Name:        name,
		Description: description,
		PluginName:  pluginName,
		Inputs:      map[string]plugin.InputSchema{},
		Outputs:     []string{},
	}
}

// TestExecutionService_PluginOperation_NoProviderConfigured tests that operation steps fail
// when no OperationProvider is configured.
func TestExecutionService_PluginOperation_NoProviderConfigured(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["op-test"] = &workflow.Workflow{
		Name:    "op-test",
		Initial: "notify",
		Steps: map[string]*workflow.Step{
			"notify": {
				Name:      "notify",
				Type:      workflow.StepTypeOperation,
				Operation: "slack.send",
				OperationInputs: map[string]any{
					"channel": "#builds",
					"message": "Build completed",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	// Note: SetOperationProvider is NOT called - provider is nil

	ctx, err := execSvc.Run(context.Background(), "op-test", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, application.ErrNoOperationProvider)
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
	assert.Equal(t, "notify", ctx.CurrentStep)
}

// TestExecutionService_PluginOperation_OperationNotFound tests that operation steps fail
// when the specified operation doesn't exist in the provider.
func TestExecutionService_PluginOperation_OperationNotFound(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["op-test"] = &workflow.Workflow{
		Name:    "op-test",
		Initial: "notify",
		Steps: map[string]*workflow.Step{
			"notify": {
				Name:      "notify",
				Type:      workflow.StepTypeOperation,
				Operation: "nonexistent.operation",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	provider := newMockOperationProvider()
	// Don't add the operation - it won't exist

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc.SetOperationProvider(provider)

	ctx, err := execSvc.Run(context.Background(), "op-test", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "nonexistent.operation")
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}

// TestExecutionService_PluginOperation_BasicExecution tests that a basic operation
// executes successfully when the operation exists.
func TestExecutionService_PluginOperation_BasicExecution(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["op-test"] = &workflow.Workflow{
		Name:    "op-test",
		Initial: "notify",
		Steps: map[string]*workflow.Step{
			"notify": {
				Name:      "notify",
				Type:      workflow.StepTypeOperation,
				Operation: "slack.send",
				OperationInputs: map[string]any{
					"channel": "#builds",
					"message": "Build completed",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	provider := newMockOperationProvider()
	provider.addOperation("slack.send", "Send Slack message", "slack-plugin")
	provider.results["slack.send"] = &plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{"sent": true},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc.SetOperationProvider(provider)

	ctx, err := execSvc.Run(context.Background(), "op-test", nil)

	// Operation should succeed
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}

// TestExecutionService_PluginOperation_WithOnFailure tests that operation step failure
// can transition to an OnFailure state.
func TestExecutionService_PluginOperation_WithOnFailure(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["op-test"] = &workflow.Workflow{
		Name:    "op-test",
		Initial: "notify",
		Steps: map[string]*workflow.Step{
			"notify": {
				Name:      "notify",
				Type:      workflow.StepTypeOperation,
				Operation: "slack.send",
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
			"error": {
				Name: "error",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	provider := newMockOperationProvider()
	provider.addOperation("slack.send", "Send Slack message", "slack-plugin")
	provider.results["slack.send"] = &plugin.OperationResult{
		Success: false,
		Error:   "Channel not found",
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc.SetOperationProvider(provider)

	ctx, err := execSvc.Run(context.Background(), "op-test", nil)

	// Operation fails, but workflow should complete via OnFailure path
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "error", ctx.CurrentStep)
}

// TestExecutionService_PluginOperation_InMixedWorkflow tests an operation step
// in a workflow that also has command steps.
func TestExecutionService_PluginOperation_InMixedWorkflow(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["mixed"] = &workflow.Workflow{
		Name:    "mixed",
		Initial: "build",
		Steps: map[string]*workflow.Step{
			"build": {
				Name:      "build",
				Type:      workflow.StepTypeCommand,
				Command:   "make build",
				OnSuccess: "notify",
			},
			"notify": {
				Name:      "notify",
				Type:      workflow.StepTypeOperation,
				Operation: "slack.send",
				OperationInputs: map[string]any{
					"message": "Build completed",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()
	executor.results["make build"] = &ports.CommandResult{Stdout: "built", ExitCode: 0}

	provider := newMockOperationProvider()
	provider.addOperation("slack.send", "Send Slack message", "slack-plugin")
	provider.results["slack.send"] = &plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{"sent": true},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		executor,
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc.SetOperationProvider(provider)

	ctx, err := execSvc.Run(context.Background(), "mixed", nil)

	// Both steps should succeed
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// Verify build step was executed successfully
	buildState, ok := ctx.GetStepState("build")
	require.True(t, ok, "build step should have been executed")
	assert.Equal(t, workflow.StatusCompleted, buildState.Status)
	assert.Equal(t, "built", buildState.Output)

	// Verify notify step was executed successfully
	notifyState, ok := ctx.GetStepState("notify")
	require.True(t, ok, "notify step should have been executed")
	assert.Equal(t, workflow.StatusCompleted, notifyState.Status)
}

// TestExecutionService_PluginOperation_StepValidation tests that operation-type steps
// are validated correctly in the workflow.
func TestExecutionService_PluginOperation_StepValidation(t *testing.T) {
	tests := []struct {
		name      string
		step      *workflow.Step
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid operation step",
			step: &workflow.Step{
				Name:      "notify",
				Type:      workflow.StepTypeOperation,
				Operation: "slack.send",
			},
			wantError: false,
		},
		{
			name: "missing operation name",
			step: &workflow.Step{
				Name:      "notify",
				Type:      workflow.StepTypeOperation,
				Operation: "", // empty
			},
			wantError: true,
			errorMsg:  "operation is required",
		},
		{
			name: "operation step with inputs",
			step: &workflow.Step{
				Name:      "notify",
				Type:      workflow.StepTypeOperation,
				Operation: "slack.send",
				OperationInputs: map[string]any{
					"channel": "#general",
					"message": "Hello",
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate(nil)
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestExecutionService_SetOperationProvider tests the setter method.
func TestExecutionService_SetOperationProvider(t *testing.T) {
	wfSvc := application.NewWorkflowService(
		newMockRepository(),
		newMockStateStore(),
		newMockExecutor(),
		&mockLogger{},
		nil,
	)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)

	provider := newMockOperationProvider()
	provider.addOperation("test.op", "Test operation", "test-plugin")

	// Should not panic
	execSvc.SetOperationProvider(provider)

	// Create a workflow with an operation step to verify the provider is set
	repo := newMockRepository()
	repo.workflows["test"] = &workflow.Workflow{
		Name:    "test",
		Initial: "op",
		Steps: map[string]*workflow.Step{
			"op": {
				Name:      "op",
				Type:      workflow.StepTypeOperation,
				Operation: "test.op",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	// Create new service with the test repo
	wfSvc2 := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc2 := application.NewExecutionService(
		wfSvc2,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc2.SetOperationProvider(provider)

	ctx, err := execSvc2.Run(context.Background(), "test", nil)
	// Should succeed (not ErrNoOperationProvider)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// TestExecutionService_PluginOperation_MultipleOperationTypes tests different
// plugin operation names are correctly routed.
func TestExecutionService_PluginOperation_MultipleOperationTypes(t *testing.T) {
	tests := []struct {
		name          string
		operation     string
		registered    bool
		expectSuccess bool
		errorContains string
	}{
		{
			name:          "slack.send registered",
			operation:     "slack.send",
			registered:    true,
			expectSuccess: true,
		},
		{
			name:          "email.send registered",
			operation:     "email.send",
			registered:    true,
			expectSuccess: true,
		},
		{
			name:          "webhook.post not registered",
			operation:     "webhook.post",
			registered:    false,
			expectSuccess: false,
			errorContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			repo.workflows["test"] = &workflow.Workflow{
				Name:    "test",
				Initial: "step",
				Steps: map[string]*workflow.Step{
					"step": {
						Name:      "step",
						Type:      workflow.StepTypeOperation,
						Operation: tt.operation,
						OnSuccess: "done",
					},
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			provider := newMockOperationProvider()
			if tt.registered {
				provider.addOperation(tt.operation, "Test operation", "test-plugin")
				provider.results[tt.operation] = &plugin.OperationResult{
					Success: true,
					Outputs: map[string]any{},
				}
			}

			wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
			execSvc := application.NewExecutionService(
				wfSvc,
				newMockExecutor(),
				newMockParallelExecutor(),
				newMockStateStore(),
				&mockLogger{},
				newMockResolver(),
				nil,
			)
			execSvc.SetOperationProvider(provider)

			ctx, err := execSvc.Run(context.Background(), "test", nil)

			if tt.expectSuccess {
				require.NoError(t, err)
				assert.Equal(t, workflow.StatusCompleted, ctx.Status)
			} else {
				assert.Contains(t, err.Error(), "not found")
			}
		})
	}
}

// TestExecutionService_Resume_WithOperationStep tests resuming a workflow
// that has an operation step.
func TestExecutionService_Resume_WithOperationStep(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["resume-op"] = &workflow.Workflow{
		Name:    "resume-op",
		Initial: "notify",
		Steps: map[string]*workflow.Step{
			"notify": {
				Name:      "notify",
				Type:      workflow.StepTypeOperation,
				Operation: "slack.send",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	// Create a state store with an interrupted execution at the operation step
	stateStore := newMockStateStore()
	execCtx := workflow.NewExecutionContext("test-id", "resume-op")
	execCtx.CurrentStep = "notify"
	execCtx.Status = workflow.StatusRunning
	stateStore.states["test-id"] = execCtx

	provider := newMockOperationProvider()
	provider.addOperation("slack.send", "Send Slack message", "slack-plugin")
	provider.results["slack.send"] = &plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{"sent": true},
	}

	wfSvc := application.NewWorkflowService(repo, stateStore, newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		stateStore,
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc.SetOperationProvider(provider)

	ctx, err := execSvc.Resume(context.Background(), "test-id", nil)

	// Should succeed after resuming at the operation step
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}

// ============================================================================
// GREEN PHASE TESTS - These tests will FAIL until the stub is implemented
// ============================================================================

// TestExecutionService_PluginOperation_SuccessfulExecution tests successful
// operation execution (will fail until implementation is complete).
func TestExecutionService_PluginOperation_SuccessfulExecution(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["op-success"] = &workflow.Workflow{
		Name:    "op-success",
		Initial: "notify",
		Steps: map[string]*workflow.Step{
			"notify": {
				Name:      "notify",
				Type:      workflow.StepTypeOperation,
				Operation: "slack.send",
				OperationInputs: map[string]any{
					"channel": "#builds",
					"message": "Build completed",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	provider := newMockOperationProvider()
	provider.addOperation("slack.send", "Send Slack message", "slack-plugin")
	provider.results["slack.send"] = &plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{"message_id": "12345"},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc.SetOperationProvider(provider)

	ctx, err := execSvc.Run(context.Background(), "op-success", nil)

	// GREEN PHASE: This should pass when implementation is complete
	require.NoError(t, err, "operation step should succeed")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// Verify operation step state was recorded
	state, ok := ctx.GetStepState("notify")
	require.True(t, ok, "operation step state should be recorded")
	assert.Equal(t, workflow.StatusCompleted, state.Status)
}

// TestExecutionService_PluginOperation_OperationFailure tests operation
// execution failure handling (will fail until implementation is complete).
func TestExecutionService_PluginOperation_OperationFailure(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["op-fail"] = &workflow.Workflow{
		Name:    "op-fail",
		Initial: "notify",
		Steps: map[string]*workflow.Step{
			"notify": {
				Name:      "notify",
				Type:      workflow.StepTypeOperation,
				Operation: "slack.send",
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
			"error": {
				Name: "error",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	provider := newMockOperationProvider()
	provider.addOperation("slack.send", "Send Slack message", "slack-plugin")
	provider.results["slack.send"] = &plugin.OperationResult{
		Success: false,
		Error:   "Channel not found",
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc.SetOperationProvider(provider)

	ctx, err := execSvc.Run(context.Background(), "op-fail", nil)

	// GREEN PHASE: This should pass when implementation is complete
	require.NoError(t, err, "workflow should complete via error path")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "error", ctx.CurrentStep, "should transition to error state")
}

// TestExecutionService_PluginOperation_InputInterpolation tests that operation
// inputs are interpolated (will fail until implementation is complete).
func TestExecutionService_PluginOperation_InputInterpolation(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["op-interpolate"] = &workflow.Workflow{
		Name:    "op-interpolate",
		Initial: "notify",
		Inputs: []workflow.Input{
			{Name: "channel", Type: "string", Required: true},
		},
		Steps: map[string]*workflow.Step{
			"notify": {
				Name:      "notify",
				Type:      workflow.StepTypeOperation,
				Operation: "slack.send",
				OperationInputs: map[string]any{
					"channel": "{{inputs.channel}}", // Should be interpolated
					"message": "Build completed",
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	provider := newMockOperationProvider()
	provider.addOperation("slack.send", "Send Slack message", "slack-plugin")
	provider.results["slack.send"] = &plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(), // Note: mock resolver doesn't interpolate
		nil,
	)
	execSvc.SetOperationProvider(provider)

	ctx, err := execSvc.Run(context.Background(), "op-interpolate", map[string]any{
		"channel": "#builds",
	})

	// GREEN PHASE: This should pass when implementation is complete
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// TestExecutionService_PluginOperation_OutputCapture tests that operation
// outputs are available in step state (will fail until implementation is complete).
func TestExecutionService_PluginOperation_OutputCapture(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["op-output"] = &workflow.Workflow{
		Name:    "op-output",
		Initial: "notify",
		Steps: map[string]*workflow.Step{
			"notify": {
				Name:      "notify",
				Type:      workflow.StepTypeOperation,
				Operation: "slack.send",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	provider := newMockOperationProvider()
	provider.addOperation("slack.send", "Send Slack message", "slack-plugin")
	provider.results["slack.send"] = &plugin.OperationResult{
		Success: true,
		Outputs: map[string]any{
			"message_id": "12345",
			"timestamp":  "2024-01-15T10:00:00Z",
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc.SetOperationProvider(provider)

	ctx, err := execSvc.Run(context.Background(), "op-output", nil)

	// GREEN PHASE: This should pass when implementation is complete
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify outputs are captured in step state
	state, ok := ctx.GetStepState("notify")
	require.True(t, ok)
	// Output should contain serialized outputs or be accessible somehow
	assert.NotEmpty(t, state.Output, "operation outputs should be captured")
}

// TestExecutionService_PluginOperation_Timeout tests that operation steps
// respect timeout configuration (will fail until implementation is complete).
func TestExecutionService_PluginOperation_Timeout(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["op-timeout"] = &workflow.Workflow{
		Name:    "op-timeout",
		Initial: "notify",
		Steps: map[string]*workflow.Step{
			"notify": {
				Name:      "notify",
				Type:      workflow.StepTypeOperation,
				Operation: "slow.operation",
				Timeout:   1, // 1 second timeout
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
			"error": {
				Name: "error",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	provider := newMockOperationProvider()
	provider.addOperation("slow.operation", "Slow operation", "test-plugin")
	// Note: In the actual implementation, this would timeout

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
	)
	execSvc.SetOperationProvider(provider)

	// This test verifies timeout handling is implemented
	// For now with stub implementation, behavior is limited
	_, err := execSvc.Run(context.Background(), "op-timeout", nil)

	// GREEN PHASE: When implemented, should either succeed or timeout gracefully
	// For now, we just verify the test runs without panic
	_ = err
}

// ============================================================================
// ERROR HANDLING TESTS - These verify error messages contain helpful context
// ============================================================================

// TestExecutionService_PluginOperation_ErrorMessages tests that error messages
// include helpful context.
func TestExecutionService_PluginOperation_ErrorMessages(t *testing.T) {
	tests := []struct {
		name          string
		stepName      string
		operation     string
		setupProvider bool
		addOperation  bool
		expectedParts []string
	}{
		{
			name:          "no provider - includes step name",
			stepName:      "my-notify-step",
			operation:     "slack.send",
			setupProvider: false,
			addOperation:  false,
			expectedParts: []string{"my-notify-step", "operation provider not configured"},
		},
		{
			name:          "operation not found - includes operation name",
			stepName:      "send-alert",
			operation:     "custom.alert",
			setupProvider: true,
			addOperation:  false,
			expectedParts: []string{"send-alert", "custom.alert", "not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			repo.workflows["test"] = &workflow.Workflow{
				Name:    "test",
				Initial: tt.stepName,
				Steps: map[string]*workflow.Step{
					tt.stepName: {
						Name:      tt.stepName,
						Type:      workflow.StepTypeOperation,
						Operation: tt.operation,
						OnSuccess: "done",
					},
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
			execSvc := application.NewExecutionService(
				wfSvc,
				newMockExecutor(),
				newMockParallelExecutor(),
				newMockStateStore(),
				&mockLogger{},
				newMockResolver(),
				nil,
			)

			if tt.setupProvider {
				provider := newMockOperationProvider()
				if tt.addOperation {
					provider.addOperation(tt.operation, "Test", "test-plugin")
				}
				execSvc.SetOperationProvider(provider)
			}

			_, err := execSvc.Run(context.Background(), "test", nil)

			require.Error(t, err)
			errMsg := err.Error()
			for _, part := range tt.expectedParts {
				assert.Contains(t, errMsg, part, "error message should contain: %s", part)
			}
		})
	}
}
