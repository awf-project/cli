package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturingValidatorProvider records calls for assertion in tests.
type capturingValidatorProvider struct {
	validateWorkflowCalled bool
	validateWorkflowJSON   []byte
	results                []ports.ValidationResult
	err                    error
}

func (m *capturingValidatorProvider) ValidateWorkflow(ctx context.Context, workflowJSON []byte) ([]ports.ValidationResult, error) {
	m.validateWorkflowCalled = true
	m.validateWorkflowJSON = workflowJSON
	return m.results, m.err
}

func (m *capturingValidatorProvider) ValidateStep(ctx context.Context, workflowJSON []byte, stepName string) ([]ports.ValidationResult, error) {
	return nil, nil
}

// Compile-time check: capturingValidatorProvider implements the port.
var _ ports.WorkflowValidatorProvider = (*capturingValidatorProvider)(nil)

func validWorkflow() *workflow.Workflow {
	return &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeTerminal},
		},
	}
}

func newWorkflowServiceWithProvider(repo *mockRepository, provider ports.WorkflowValidatorProvider) *application.WorkflowService {
	svc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, newMockExpressionValidator())
	svc.SetValidatorProvider(provider)
	return svc
}

// TestWorkflowService_SetValidatorProvider_AcceptsNil verifies the setter handles nil
// without panicking, preserving existing behavior when no provider is configured.
func TestWorkflowService_SetValidatorProvider_AcceptsNil(t *testing.T) {
	svc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, newMockExpressionValidator())

	// Must not panic
	svc.SetValidatorProvider(nil)
}

// TestWorkflowService_ValidateWorkflow_CallsPluginProvider verifies the provider is
// invoked after successful built-in validation and receives the workflow as JSON.
func TestWorkflowService_ValidateWorkflow_CallsPluginProvider(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["test"] = validWorkflow()

	provider := &capturingValidatorProvider{}
	svc := newWorkflowServiceWithProvider(repo, provider)

	err := svc.ValidateWorkflow(context.Background(), "test")

	require.NoError(t, err)
	assert.True(t, provider.validateWorkflowCalled, "plugin provider should be called after built-in validation")
	assert.NotEmpty(t, provider.validateWorkflowJSON, "provider should receive the workflow encoded as JSON")
}

// TestWorkflowService_ValidateWorkflow_PluginErrorResultsReturnError verifies that
// ValidationResult entries with SeverityError cause ValidateWorkflow to return an error.
func TestWorkflowService_ValidateWorkflow_PluginErrorResultsReturnError(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["test"] = validWorkflow()

	provider := &capturingValidatorProvider{
		results: []ports.ValidationResult{
			{Severity: ports.SeverityError, Message: "required step 'deploy' is missing", Step: ""},
		},
	}
	svc := newWorkflowServiceWithProvider(repo, provider)

	err := svc.ValidateWorkflow(context.Background(), "test")

	require.Error(t, err, "SeverityError results from plugin provider should cause ValidateWorkflow to return error")
	assert.Contains(t, err.Error(), "required step 'deploy' is missing")
}

// TestWorkflowService_ValidateWorkflow_PluginWarningsDoNotCauseError verifies that
// findings at Warning or Info severity do not block validation (no error returned).
func TestWorkflowService_ValidateWorkflow_PluginWarningsDoNotCauseError(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["test"] = validWorkflow()

	provider := &capturingValidatorProvider{
		results: []ports.ValidationResult{
			{Severity: ports.SeverityWarning, Message: "deprecated field 'retries'"},
			{Severity: ports.SeverityInfo, Message: "consider adding a timeout"},
		},
	}
	svc := newWorkflowServiceWithProvider(repo, provider)

	err := svc.ValidateWorkflow(context.Background(), "test")

	require.NoError(t, err, "Warning/Info results from plugin provider should not cause ValidateWorkflow to return error")
	assert.True(t, provider.validateWorkflowCalled, "provider should still be called")
}

// TestWorkflowService_ValidateWorkflow_SkipsProviderOnBuiltinFailure verifies that
// the plugin provider is NOT called when built-in workflow validation fails first.
func TestWorkflowService_ValidateWorkflow_SkipsProviderOnBuiltinFailure(t *testing.T) {
	repo := newMockRepository()
	// Missing Initial makes built-in validation fail.
	repo.workflows["broken"] = &workflow.Workflow{
		Name: "broken",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeTerminal},
		},
	}

	provider := &capturingValidatorProvider{}
	svc := newWorkflowServiceWithProvider(repo, provider)

	err := svc.ValidateWorkflow(context.Background(), "broken")

	require.Error(t, err, "built-in validation should fail for workflow with missing initial step")
	assert.False(t, provider.validateWorkflowCalled, "plugin provider must NOT be called when built-in validation fails")
}

// TestWorkflowService_ValidateWorkflow_ProviderCallErrorPropagated verifies that a
// transport/call error from the provider (e.g. plugin crashed) is returned to the caller.
func TestWorkflowService_ValidateWorkflow_ProviderCallErrorPropagated(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["test"] = validWorkflow()

	callErr := errors.New("plugin process unavailable")
	provider := &capturingValidatorProvider{err: callErr}
	svc := newWorkflowServiceWithProvider(repo, provider)

	err := svc.ValidateWorkflow(context.Background(), "test")

	require.Error(t, err)
	assert.True(t, errors.Is(err, callErr), "plugin call error should be in the error chain")
}
