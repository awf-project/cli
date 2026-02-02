// Package sdk provides testing utilities for AWF plugin development.
package sdk

import (
	"context"
	"sync"
)

// MockPlugin is a test implementation of the Plugin interface.
type MockPlugin struct {
	PluginName    string
	PluginVersion string

	mu             sync.Mutex
	InitCalled     bool
	ShutdownCalled bool
	LastConfig     map[string]any
	InitError      error
	ShutdownError  error
}

// NewMockPlugin creates a mock plugin for testing.
func NewMockPlugin(name, version string) *MockPlugin {
	return &MockPlugin{
		PluginName:    name,
		PluginVersion: version,
	}
}

// Name returns the plugin name.
func (m *MockPlugin) Name() string {
	return m.PluginName
}

// Version returns the plugin version.
func (m *MockPlugin) Version() string {
	return m.PluginVersion
}

// Init records the call and returns the configured error.
func (m *MockPlugin) Init(_ context.Context, config map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InitCalled = true
	m.LastConfig = config
	return m.InitError
}

// Shutdown records the call and returns the configured error.
func (m *MockPlugin) Shutdown(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ShutdownCalled = true
	return m.ShutdownError
}

// WasInitCalled returns whether Init was called (thread-safe).
func (m *MockPlugin) WasInitCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.InitCalled
}

// WasShutdownCalled returns whether Shutdown was called (thread-safe).
func (m *MockPlugin) WasShutdownCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ShutdownCalled
}

// Reset clears the mock state.
func (m *MockPlugin) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InitCalled = false
	m.ShutdownCalled = false
	m.LastConfig = nil
}

// MockOperationHandler is a test implementation of OperationHandler.
type MockOperationHandler struct {
	mu         sync.Mutex
	Outputs    map[string]any
	Error      error
	LastInputs map[string]any
	CallCount  int
	HandleFunc func(ctx context.Context, inputs map[string]any) (map[string]any, error)
}

// NewMockOperationHandler creates a mock operation handler.
func NewMockOperationHandler() *MockOperationHandler {
	return &MockOperationHandler{
		Outputs: make(map[string]any),
	}
}

// Handle executes the operation with recording.
func (m *MockOperationHandler) Handle(ctx context.Context, inputs map[string]any) (map[string]any, error) {
	m.mu.Lock()
	m.LastInputs = inputs
	m.CallCount++
	handleFunc := m.HandleFunc
	outputs := m.Outputs
	err := m.Error
	m.mu.Unlock()

	if handleFunc != nil {
		return handleFunc(ctx, inputs)
	}
	return outputs, err
}

// GetCallCount returns the number of times Handle was called (thread-safe).
func (m *MockOperationHandler) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.CallCount
}

// Reset clears the mock state.
func (m *MockOperationHandler) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LastInputs = nil
	m.CallCount = 0
}

// MockOperationProvider is a test implementation of OperationProvider.
type MockOperationProvider struct {
	mu             sync.Mutex
	OperationNames []string
	Results        map[string]*OperationResult
	Errors         map[string]error
	LastOperation  string
	LastInputs     map[string]any
	HandleFunc     func(ctx context.Context, name string, inputs map[string]any) (*OperationResult, error)
}

// NewMockOperationProvider creates a mock operation provider.
func NewMockOperationProvider(operations ...string) *MockOperationProvider {
	return &MockOperationProvider{
		OperationNames: operations,
		Results:        make(map[string]*OperationResult),
		Errors:         make(map[string]error),
	}
}

// Operations returns the list of operation names.
func (m *MockOperationProvider) Operations() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.OperationNames
}

// HandleOperation executes the specified operation.
func (m *MockOperationProvider) HandleOperation(ctx context.Context, name string, inputs map[string]any) (*OperationResult, error) {
	m.mu.Lock()
	m.LastOperation = name
	m.LastInputs = inputs
	handleFunc := m.HandleFunc
	result := m.Results[name]
	err := m.Errors[name]
	m.mu.Unlock()

	if handleFunc != nil {
		return handleFunc(ctx, name, inputs)
	}

	if err != nil {
		return nil, err
	}
	if result != nil {
		return result, nil
	}
	return NewSuccessResult("ok", nil), nil
}

// SetResult configures the mock to return a specific result for all operations (test helper).
//
// Deprecated: Use SetCommandResult for operation-specific results. Migration tracked in #150.
func (m *MockOperationProvider) SetResult(operation string, result *OperationResult) {
	// Legacy behavior: delegates to SetCommandResult
	m.SetCommandResult(operation, result)
}

// SetCommandResult configures the mock to return a specific result for a given operation (test helper).
func (m *MockOperationProvider) SetCommandResult(operation string, result *OperationResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Results[operation] = result
}

// SetError configures an error for an operation.
func (m *MockOperationProvider) SetError(operation string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Errors[operation] = err
}

// TestInputs creates a map of inputs for testing.
func TestInputs(keyValues ...any) map[string]any {
	result := make(map[string]any)
	for i := 0; i < len(keyValues)-1; i += 2 {
		if key, ok := keyValues[i].(string); ok {
			result[key] = keyValues[i+1]
		}
	}
	return result
}

// TestConfig creates a plugin config map for testing.
func TestConfig(keyValues ...any) map[string]any {
	return TestInputs(keyValues...)
}
