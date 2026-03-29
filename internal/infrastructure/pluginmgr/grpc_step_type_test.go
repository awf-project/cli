package pluginmgr

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc" // grpc.CallOption for mock interface
)

// TestNewGRPCStepTypeAdapter_HappyPath tests successful construction with valid timeout.
func TestNewGRPCStepTypeAdapter_HappyPath(t *testing.T) {
	mockClient := new(MockStepTypeServiceClient)
	timeout := 3 * time.Second

	adapter := newGRPCStepTypeAdapter(mockClient, "test-plugin", timeout, nil)

	require.NotNil(t, adapter)
	assert.Equal(t, "test-plugin", adapter.pluginName)
	assert.Equal(t, 3*time.Second, adapter.timeout)
	assert.Equal(t, mockClient, adapter.client)
}

// TestNewGRPCStepTypeAdapter_DefaultTimeout tests that zero/negative timeout uses default.
func TestNewGRPCStepTypeAdapter_DefaultTimeout(t *testing.T) {
	mockClient := new(MockStepTypeServiceClient)

	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"zero timeout", 0},
		{"negative timeout", -1 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := newGRPCStepTypeAdapter(mockClient, "plugin", tt.timeout, nil)

			require.NotNil(t, adapter)
			assert.Equal(t, defaultStepTypeTimeout, adapter.timeout)
		})
	}
}

// TestListAndCache_HappyPath tests successful fetching and caching of step types
// with automatic namespacing: plugin declares "git-clone", host registers "test-plugin.git-clone".
func TestListAndCache_HappyPath(t *testing.T) {
	mockClient := new(MockStepTypeServiceClient)
	mockLogger := new(MockLogger)

	stepTypes := []*pluginv1.StepTypeInfo{
		{Name: "git-clone", Description: "Clone a git repository"},
		{Name: "http-request", Description: "Make an HTTP request"},
	}

	mockClient.On("ListStepTypes", mock.Anything, &pluginv1.ListStepTypesRequest{}).
		Return(&pluginv1.ListStepTypesResponse{
			StepTypes: stepTypes,
		}, nil)

	adapter := newGRPCStepTypeAdapter(mockClient, "test-plugin", 5*time.Second, mockLogger)
	err := adapter.listAndCache(context.Background(), make(map[string]bool))

	require.NoError(t, err)
	// Host-side uses qualified names
	assert.True(t, adapter.HasStepType("test-plugin.git-clone"))
	assert.True(t, adapter.HasStepType("test-plugin.http-request"))
	// Raw names are NOT registered on the host side
	assert.False(t, adapter.HasStepType("git-clone"))
	assert.False(t, adapter.HasStepType("unknown-type"))
}

// TestListAndCache_EmptyResponse tests caching with no step types returned.
func TestListAndCache_EmptyResponse(t *testing.T) {
	mockClient := new(MockStepTypeServiceClient)
	mockLogger := new(MockLogger)

	mockClient.On("ListStepTypes", mock.Anything, &pluginv1.ListStepTypesRequest{}).
		Return(&pluginv1.ListStepTypesResponse{
			StepTypes: []*pluginv1.StepTypeInfo{},
		}, nil)

	adapter := newGRPCStepTypeAdapter(mockClient, "test-plugin", 5*time.Second, mockLogger)
	err := adapter.listAndCache(context.Background(), make(map[string]bool))

	require.NoError(t, err)
	assert.False(t, adapter.HasStepType("any-type"))
}

// TestListAndCache_FirstRegisteredWins tests conflict resolution with duplicate names.
func TestListAndCache_FirstRegisteredWins(t *testing.T) {
	mockClient := new(MockStepTypeServiceClient)
	mockLogger := new(MockLogger)

	stepTypes := []*pluginv1.StepTypeInfo{
		{Name: "duplicate-step", Description: "First registration"},
		{Name: "duplicate-step", Description: "Second registration (should be ignored)"},
	}

	mockClient.On("ListStepTypes", mock.Anything, &pluginv1.ListStepTypesRequest{}).
		Return(&pluginv1.ListStepTypesResponse{
			StepTypes: stepTypes,
		}, nil)

	mockLogger.On("Warn", mock.MatchedBy(func(msg string) bool {
		return msg != ""
	}), mock.Anything).Return()

	adapter := newGRPCStepTypeAdapter(mockClient, "test-plugin", 5*time.Second, mockLogger)
	err := adapter.listAndCache(context.Background(), make(map[string]bool))

	require.NoError(t, err)
	assert.True(t, adapter.HasStepType("test-plugin.duplicate-step"))
	mockLogger.AssertCalled(t, "Warn", mock.Anything, mock.Anything)
}

// TestListAndCache_ExistingConflict tests conflict with already-registered qualified names.
func TestListAndCache_ExistingConflict(t *testing.T) {
	mockClient := new(MockStepTypeServiceClient)
	mockLogger := new(MockLogger)

	stepTypes := []*pluginv1.StepTypeInfo{
		{Name: "pre-existing-type", Description: "From plugin"},
	}

	mockClient.On("ListStepTypes", mock.Anything, &pluginv1.ListStepTypesRequest{}).
		Return(&pluginv1.ListStepTypesResponse{
			StepTypes: stepTypes,
		}, nil)

	mockLogger.On("Warn", mock.MatchedBy(func(msg string) bool {
		return msg != ""
	}), mock.Anything).Return()

	// Conflict on qualified name — another plugin already registered "test-plugin.pre-existing-type"
	existing := map[string]bool{"test-plugin.pre-existing-type": true}

	adapter := newGRPCStepTypeAdapter(mockClient, "test-plugin", 5*time.Second, mockLogger)
	err := adapter.listAndCache(context.Background(), existing)

	require.NoError(t, err)
	mockLogger.AssertCalled(t, "Warn", mock.Anything, mock.Anything)
}

// TestListAndCache_ClientError tests handling of gRPC client errors.
func TestListAndCache_ClientError(t *testing.T) {
	mockClient := new(MockStepTypeServiceClient)
	mockLogger := new(MockLogger)

	mockClient.On("ListStepTypes", mock.Anything, &pluginv1.ListStepTypesRequest{}).
		Return(nil, context.DeadlineExceeded)

	adapter := newGRPCStepTypeAdapter(mockClient, "test-plugin", 5*time.Second, mockLogger)
	err := adapter.listAndCache(context.Background(), make(map[string]bool))

	assert.Error(t, err)
	assert.False(t, adapter.HasStepType("any-type"))
}

// TestHasStepType_CachedLookup tests O(1) cached lookup with qualified names.
func TestHasStepType_CachedLookup(t *testing.T) {
	mockClient := new(MockStepTypeServiceClient)
	mockLogger := new(MockLogger)

	stepTypes := []*pluginv1.StepTypeInfo{
		{Name: "cached-type", Description: "Test"},
	}

	mockClient.On("ListStepTypes", mock.Anything, &pluginv1.ListStepTypesRequest{}).
		Return(&pluginv1.ListStepTypesResponse{
			StepTypes: stepTypes,
		}, nil)

	adapter := newGRPCStepTypeAdapter(mockClient, "test-plugin", 5*time.Second, mockLogger)
	_ = adapter.listAndCache(context.Background(), make(map[string]bool))

	assert.True(t, adapter.HasStepType("test-plugin.cached-type"))
	assert.False(t, adapter.HasStepType("cached-type"))
	assert.False(t, adapter.HasStepType("uncached-type"))
}

// TestExecuteStep_HappyPath tests successful step execution with prefix stripping.
// Host sends qualified name "test-plugin.http-request", adapter strips to "http-request" for gRPC.
func TestExecuteStep_HappyPath(t *testing.T) {
	mockClient := new(MockStepTypeServiceClient)
	mockLogger := new(MockLogger)

	configData := map[string]any{"timeout": 30}
	inputsData := map[string]any{"url": "https://example.com"}

	configJSON, _ := json.Marshal(configData)
	inputsJSON, _ := json.Marshal(inputsData)

	responseData := map[string]any{"status": "success"}
	responseJSON, _ := json.Marshal(responseData)

	// gRPC receives the raw (stripped) step type name
	mockClient.On("ExecuteStep", mock.Anything, &pluginv1.ExecuteStepRequest{
		StepName: "test-step",
		StepType: "http-request",
		Config:   configJSON,
		Inputs:   inputsJSON,
	}).Return(&pluginv1.ExecuteStepResponse{
		Output:   "Request completed",
		Data:     responseJSON,
		ExitCode: 0,
	}, nil)

	adapter := newGRPCStepTypeAdapter(mockClient, "test-plugin", 5*time.Second, mockLogger)

	// Application sends qualified name
	req := ports.StepExecuteRequest{
		StepName: "test-step",
		StepType: "test-plugin.http-request",
		Config:   configData,
		Inputs:   inputsData,
	}

	result, err := adapter.ExecuteStep(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "Request completed", result.Output)
	assert.Equal(t, int(0), result.ExitCode)
	assert.Equal(t, "success", result.Data["status"])
}

// TestExecuteStep_ClientError tests handling of step execution errors.
func TestExecuteStep_ClientError(t *testing.T) {
	mockClient := new(MockStepTypeServiceClient)
	mockLogger := new(MockLogger)

	mockClient.On("ExecuteStep", mock.Anything, mock.Anything).
		Return(nil, context.Canceled)

	adapter := newGRPCStepTypeAdapter(mockClient, "test-plugin", 5*time.Second, mockLogger)

	req := ports.StepExecuteRequest{
		StepName: "test-step",
		StepType: "http-request",
		Config:   make(map[string]any),
		Inputs:   make(map[string]any),
	}

	result, err := adapter.ExecuteStep(context.Background(), req)

	assert.Error(t, err)
	assert.Equal(t, ports.StepExecuteResult{}, result)
}

// TestExecuteStep_NonZeroExitCode tests execution with non-zero exit code.
func TestExecuteStep_NonZeroExitCode(t *testing.T) {
	mockClient := new(MockStepTypeServiceClient)
	mockLogger := new(MockLogger)

	mockClient.On("ExecuteStep", mock.Anything, mock.Anything).
		Return(&pluginv1.ExecuteStepResponse{
			Output:   "Command failed",
			Data:     []byte("{}"),
			ExitCode: 1,
		}, nil)

	adapter := newGRPCStepTypeAdapter(mockClient, "test-plugin", 5*time.Second, mockLogger)

	req := ports.StepExecuteRequest{
		StepName: "test-step",
		StepType: "shell-command",
		Config:   make(map[string]any),
		Inputs:   make(map[string]any),
	}

	result, err := adapter.ExecuteStep(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, 1, result.ExitCode)
	assert.Equal(t, "Command failed", result.Output)
}

// MockStepTypeServiceClient is a testify mock for StepTypeServiceClient.
type MockStepTypeServiceClient struct {
	mock.Mock
}

func (m *MockStepTypeServiceClient) ListStepTypes(ctx context.Context, in *pluginv1.ListStepTypesRequest, opts ...grpc.CallOption) (*pluginv1.ListStepTypesResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pluginv1.ListStepTypesResponse), args.Error(1)
}

func (m *MockStepTypeServiceClient) ExecuteStep(ctx context.Context, in *pluginv1.ExecuteStepRequest, opts ...grpc.CallOption) (*pluginv1.ExecuteStepResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pluginv1.ExecuteStepResponse), args.Error(1)
}

// MockLogger is a testify mock for ports.Logger.
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...any) {
	m.Called(msg, fields)
}

func (m *MockLogger) Info(msg string, fields ...any) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields ...any) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, fields ...any) {
	m.Called(msg, fields)
}

func (m *MockLogger) WithContext(ctx map[string]any) ports.Logger {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(ports.Logger)
}
