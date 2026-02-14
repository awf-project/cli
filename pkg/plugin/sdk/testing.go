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

func NewMockPlugin(name, version string) *MockPlugin {
	return &MockPlugin{
		PluginName:    name,
		PluginVersion: version,
	}
}

func (m *MockPlugin) Name() string {
	return m.PluginName
}

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

func (m *MockPlugin) WasShutdownCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ShutdownCalled
}

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

func (m *MockOperationHandler) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.CallCount
}

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

func (m *MockOperationProvider) SetResult(operation string, result *OperationResult) {
	m.SetCommandResult(operation, result)
}

func (m *MockOperationProvider) SetCommandResult(operation string, result *OperationResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Results[operation] = result
}

func (m *MockOperationProvider) SetError(operation string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Errors[operation] = err
}

func TestInputs(keyValues ...any) map[string]any {
	result := make(map[string]any)
	for i := 0; i < len(keyValues)-1; i += 2 {
		if key, ok := keyValues[i].(string); ok {
			result[key] = keyValues[i+1]
		}
	}
	return result
}

func TestConfig(keyValues ...any) map[string]any {
	return TestInputs(keyValues...)
}
