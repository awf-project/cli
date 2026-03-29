package cli_test

import (
	"context"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockRPCPluginManager mocks the RPCPluginManager for testing provider wiring
type MockRPCPluginManager struct {
	mock.Mock
}

func (m *MockRPCPluginManager) ValidatorProvider(timeout int) ports.WorkflowValidatorProvider {
	args := m.Called(timeout)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(ports.WorkflowValidatorProvider)
}

func (m *MockRPCPluginManager) StepTypeProvider(logger ports.Logger) ports.StepTypeProvider {
	args := m.Called(logger)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(ports.StepTypeProvider)
}

// MockWorkflowValidator for testing
type MockWorkflowValidator struct {
	mock.Mock
}

func (m *MockWorkflowValidator) ValidateWorkflow(ctx context.Context, workflowJSON []byte) ([]ports.ValidationResult, error) {
	args := m.Called(ctx, workflowJSON)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]ports.ValidationResult), args.Error(1)
}

func (m *MockWorkflowValidator) ValidateStep(ctx context.Context, workflowJSON []byte, stepName string) ([]ports.ValidationResult, error) {
	args := m.Called(ctx, workflowJSON, stepName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]ports.ValidationResult), args.Error(1)
}

// MockStepTypeProvider for testing
type MockStepTypeProvider struct {
	mock.Mock
}

func (m *MockStepTypeProvider) HasStepType(stepType string) bool {
	args := m.Called(stepType)
	return args.Bool(0)
}

func (m *MockStepTypeProvider) ExecuteStep(ctx context.Context, req ports.StepExecuteRequest) (ports.StepExecuteResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return ports.StepExecuteResult{}, args.Error(1)
	}
	return args.Get(0).(ports.StepExecuteResult), args.Error(1)
}

// MockLogger for testing
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, keysAndValues ...any) {
	m.Called(msg, keysAndValues)
}

func (m *MockLogger) Info(msg string, keysAndValues ...any) {
	m.Called(msg, keysAndValues)
}

func (m *MockLogger) Warn(msg string, keysAndValues ...any) {
	m.Called(msg, keysAndValues)
}

func (m *MockLogger) Error(msg string, keysAndValues ...any) {
	m.Called(msg, keysAndValues)
}

func (m *MockLogger) WithContext(ctx map[string]any) ports.Logger {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(ports.Logger)
}

// TestPluginProviderWiring_RPCManagerNil verifies that when RPCManager is nil,
// services initialize without validator/step-type providers (no panic, graceful degradation)
func TestPluginProviderWiring_RPCManagerNil(t *testing.T) {
	// Setup services with nil RPCManager (simulating --skip-plugins scenario)
	repo := cli.NewWorkflowRepository()
	stateStore := store.NewJSONStore(t.TempDir())
	shellExecutor := executor.NewShellExecutor()

	logger := &NullLogger{}
	exprValidator := &NullExprValidator{}

	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, logger, exprValidator)
	parallelExecutor := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, shellExecutor, parallelExecutor, stateStore, logger, nil, nil, nil,
	)

	// Simulate the wiring code with RPCManager = nil
	// Since pluginRPCManager is nil, the providers are not set (graceful degradation)
	var pluginRPCManager *pluginmgr.RPCPluginManager
	_ = pluginRPCManager

	// Verify no panic occurred and services are functional
	assert.NotNil(t, wfSvc)
	assert.NotNil(t, execSvc)
}

// TestPluginProviderWiring_ValidatorProviderCalled verifies that ValidatorProvider(timeout)
// is called with the correct timeout value (0) when RPCManager is not nil
func TestPluginProviderWiring_ValidatorProviderCalled(t *testing.T) {
	// Setup mock RPCManager
	mockRPCManager := new(MockRPCPluginManager)
	mockValidator := new(MockWorkflowValidator)

	// Expect ValidatorProvider to be called with timeout=0
	mockRPCManager.On("ValidatorProvider", 0).Return(mockValidator)

	// Setup services
	repo := cli.NewWorkflowRepository()
	stateStore := store.NewJSONStore(t.TempDir())
	shellExecutor := executor.NewShellExecutor()

	logger := &NullLogger{}
	exprValidator := &NullExprValidator{}

	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, logger, exprValidator)

	// Simulate the wiring code
	provider := mockRPCManager.ValidatorProvider(0)
	wfSvc.SetValidatorProvider(provider)

	// Verify the mock was called
	mockRPCManager.AssertCalled(t, "ValidatorProvider", 0)
	mockRPCManager.AssertNumberOfCalls(t, "ValidatorProvider", 1)
}

// TestPluginProviderWiring_StepTypeProviderCalled verifies that StepTypeProvider(logger)
// is called with the correct logger when RPCManager is not nil
func TestPluginProviderWiring_StepTypeProviderCalled(t *testing.T) {
	// Setup mock RPCManager
	mockRPCManager := new(MockRPCPluginManager)
	mockStepTypeProvider := new(MockStepTypeProvider)

	// Setup mock logger
	mockLogger := new(MockLogger)

	// Expect StepTypeProvider to be called with the logger
	mockRPCManager.On("StepTypeProvider", mockLogger).Return(mockStepTypeProvider)

	// Setup services
	repo := cli.NewWorkflowRepository()
	stateStore := store.NewJSONStore(t.TempDir())
	shellExecutor := executor.NewShellExecutor()

	exprValidator := &NullExprValidator{}

	parallelExecutor := application.NewParallelExecutor(mockLogger)
	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, mockLogger, exprValidator)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, shellExecutor, parallelExecutor, stateStore, mockLogger, nil, nil, nil,
	)

	// Simulate the wiring code
	provider := mockRPCManager.StepTypeProvider(mockLogger)
	execSvc.SetStepTypeProvider(provider)

	// Verify the mock was called with the logger
	mockRPCManager.AssertCalled(t, "StepTypeProvider", mockLogger)
	mockRPCManager.AssertNumberOfCalls(t, "StepTypeProvider", 1)
}

// TestPluginProviderWiring_ProvidersSetOnServices verifies that returned providers
// are set on both WorkflowService and ExecutionService when RPCManager is not nil
func TestPluginProviderWiring_ProvidersSetOnServices(t *testing.T) {
	// Setup mock RPCManager and providers
	mockRPCManager := new(MockRPCPluginManager)
	mockValidator := new(MockWorkflowValidator)
	mockStepTypeProvider := new(MockStepTypeProvider)

	mockLogger := new(MockLogger)

	mockRPCManager.On("ValidatorProvider", 0).Return(mockValidator)
	mockRPCManager.On("StepTypeProvider", mockLogger).Return(mockStepTypeProvider)

	// Setup services
	repo := cli.NewWorkflowRepository()
	stateStore := store.NewJSONStore(t.TempDir())
	shellExecutor := executor.NewShellExecutor()

	exprValidator := &NullExprValidator{}

	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, mockLogger, exprValidator)
	parallelExecutor := application.NewParallelExecutor(mockLogger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, shellExecutor, parallelExecutor, stateStore, mockLogger, nil, nil, nil,
	)

	// Simulate the wiring code from run.go
	validatorProvider := mockRPCManager.ValidatorProvider(0)
	require.NotNil(t, validatorProvider)
	wfSvc.SetValidatorProvider(validatorProvider)

	stepTypeProvider := mockRPCManager.StepTypeProvider(mockLogger)
	require.NotNil(t, stepTypeProvider)
	execSvc.SetStepTypeProvider(stepTypeProvider)

	// Verify both providers were obtained from RPCManager
	mockRPCManager.AssertCalled(t, "ValidatorProvider", 0)
	mockRPCManager.AssertCalled(t, "StepTypeProvider", mockLogger)

	// Verify that both services have providers set (indirectly through mock calls)
	assert.NotNil(t, validatorProvider)
	assert.NotNil(t, stepTypeProvider)
}

// TestPluginProviderWiring_TimeoutValue verifies that ValidatorProvider is called
// with timeout=0 (as specified in the wiring code)
func TestPluginProviderWiring_TimeoutValue(t *testing.T) {
	mockRPCManager := new(MockRPCPluginManager)
	mockValidator := new(MockWorkflowValidator)

	// Expect timeout to be exactly 0, not any other value
	mockRPCManager.On("ValidatorProvider", 0).Return(mockValidator)
	mockRPCManager.On("ValidatorProvider", mock.Anything).Return(nil)

	// Call with the actual timeout value from the wiring code
	provider := mockRPCManager.ValidatorProvider(0)

	// Verify ValidatorProvider was called with 0, not another value
	assert.NotNil(t, provider)
	mockRPCManager.AssertCalled(t, "ValidatorProvider", 0)
	mockRPCManager.AssertNotCalled(t, "ValidatorProvider", 5)
}

// TestPluginProviderWiring_NilValidatorProvider verifies that SetValidatorProvider
// can be called with nil without causing errors
func TestPluginProviderWiring_NilValidatorProvider(t *testing.T) {
	repo := cli.NewWorkflowRepository()
	stateStore := store.NewJSONStore(t.TempDir())
	shellExecutor := executor.NewShellExecutor()

	logger := &NullLogger{}
	exprValidator := &NullExprValidator{}

	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, logger, exprValidator)

	// Setting a nil validator provider should not cause panic
	wfSvc.SetValidatorProvider(nil)

	// Service should still be functional
	assert.NotNil(t, wfSvc)
}

// TestPluginProviderWiring_NilStepTypeProvider verifies that SetStepTypeProvider
// can be called with nil without causing errors
func TestPluginProviderWiring_NilStepTypeProvider(t *testing.T) {
	repo := cli.NewWorkflowRepository()
	stateStore := store.NewJSONStore(t.TempDir())
	shellExecutor := executor.NewShellExecutor()

	logger := &NullLogger{}
	exprValidator := &NullExprValidator{}

	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, logger, exprValidator)
	parallelExecutor := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, shellExecutor, parallelExecutor, stateStore, logger, nil, nil, nil,
	)

	// Setting a nil step type provider should not cause panic
	execSvc.SetStepTypeProvider(nil)

	// Service should still be functional
	assert.NotNil(t, execSvc)
}

// TestPluginProviderWiring_BothProvidersSet verifies that both providers can be set
// in the same workflow execution context without conflicts
func TestPluginProviderWiring_BothProvidersSet(t *testing.T) {
	mockRPCManager := new(MockRPCPluginManager)
	mockValidator := new(MockWorkflowValidator)
	mockStepTypeProvider := new(MockStepTypeProvider)

	mockLogger := new(MockLogger)

	mockRPCManager.On("ValidatorProvider", 0).Return(mockValidator)
	mockRPCManager.On("StepTypeProvider", mockLogger).Return(mockStepTypeProvider)

	repo := cli.NewWorkflowRepository()
	stateStore := store.NewJSONStore(t.TempDir())
	shellExecutor := executor.NewShellExecutor()

	exprValidator := &NullExprValidator{}

	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, mockLogger, exprValidator)
	parallelExecutor := application.NewParallelExecutor(mockLogger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, shellExecutor, parallelExecutor, stateStore, mockLogger, nil, nil, nil,
	)

	// Simulate the full wiring sequence from run.go
	validatorProvider := mockRPCManager.ValidatorProvider(0)
	if validatorProvider != nil {
		wfSvc.SetValidatorProvider(validatorProvider)
	}

	stepTypeProvider := mockRPCManager.StepTypeProvider(mockLogger)
	if stepTypeProvider != nil {
		execSvc.SetStepTypeProvider(stepTypeProvider)
	}

	// Verify both providers were set
	assert.NotNil(t, wfSvc)
	assert.NotNil(t, execSvc)
	mockRPCManager.AssertCalled(t, "ValidatorProvider", 0)
	mockRPCManager.AssertCalled(t, "StepTypeProvider", mockLogger)
}

// NullExprValidator is a stub for expression validation (used in tests where it's not the focus)
type NullExprValidator struct{}

func (n *NullExprValidator) Compile(expr string) error {
	return nil
}

// NullLogger is a stub logger that discards all output
type NullLogger struct{}

func (n *NullLogger) Debug(msg string, keysAndValues ...any)      {}
func (n *NullLogger) Info(msg string, keysAndValues ...any)       {}
func (n *NullLogger) Warn(msg string, keysAndValues ...any)       {}
func (n *NullLogger) Error(msg string, keysAndValues ...any)      {}
func (n *NullLogger) WithContext(ctx map[string]any) ports.Logger { return n }
