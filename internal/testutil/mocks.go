package testutil

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/vanoix/awf/internal/domain/plugin"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// This file contains thread-safe mock implementations of domain port interfaces.
// All mocks use sync.Mutex or sync.RWMutex for concurrent access protection.

// Compile-time interface compliance verification.
var (
	_ ports.WorkflowRepository  = (*MockWorkflowRepository)(nil)
	_ ports.StateStore          = (*MockStateStore)(nil)
	_ ports.CommandExecutor     = (*MockCommandExecutor)(nil)
	_ ports.CLIExecutor         = (*MockCLIExecutor)(nil)
	_ ports.Logger              = (*MockLogger)(nil)
	_ ports.HistoryStore        = (*MockHistoryStore)(nil)
	_ ports.ExpressionValidator = (*MockExpressionValidator)(nil)
	_ ports.PluginManager       = (*MockPluginManager)(nil)
	_ ports.AgentRegistry       = (*MockAgentRegistry)(nil)
	_ ports.AgentProvider       = (*MockAgentProvider)(nil)
)

// MockWorkflowRepository is a thread-safe mock implementation of ports.WorkflowRepository.
// It uses sync.RWMutex to protect concurrent access to the workflows map.
//
// Usage:
//
//	repo := testutil.NewMockWorkflowRepository()
//	repo.AddWorkflow("test", &workflow.Workflow{Name: "test"})
//	wf, err := repo.Load(ctx, "test")
type MockWorkflowRepository struct {
	mu        sync.RWMutex
	workflows map[string]*workflow.Workflow
	loadErr   error
	listErr   error
	existsErr error
}

// NewMockWorkflowRepository creates a new thread-safe mock workflow repository.
func NewMockWorkflowRepository() *MockWorkflowRepository {
	return &MockWorkflowRepository{
		workflows: make(map[string]*workflow.Workflow),
	}
}

// Load retrieves a workflow by name. Returns nil if workflow does not exist.
// Thread-safe for concurrent access.
func (m *MockWorkflowRepository) Load(ctx context.Context, name string) (*workflow.Workflow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.loadErr != nil {
		return nil, m.loadErr
	}

	return m.workflows[name], nil
}

// List returns all workflow names in the repository.
// Thread-safe for concurrent access.
func (m *MockWorkflowRepository) List(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.listErr != nil {
		return nil, m.listErr
	}

	names := make([]string, 0, len(m.workflows))
	for name := range m.workflows {
		names = append(names, name)
	}

	return names, nil
}

// Exists checks if a workflow with the given name exists.
// Thread-safe for concurrent access.
func (m *MockWorkflowRepository) Exists(ctx context.Context, name string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.existsErr != nil {
		return false, m.existsErr
	}

	_, exists := m.workflows[name]
	return exists, nil
}

// AddWorkflow adds or updates a workflow in the repository.
// Thread-safe for concurrent access.
func (m *MockWorkflowRepository) AddWorkflow(name string, wf *workflow.Workflow) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.workflows[name] = wf
}

// SetLoadError configures an error to be returned by Load.
// Thread-safe for concurrent access.
func (m *MockWorkflowRepository) SetLoadError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.loadErr = err
}

// SetListError configures an error to be returned by List.
// Thread-safe for concurrent access.
func (m *MockWorkflowRepository) SetListError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.listErr = err
}

// SetExistsError configures an error to be returned by Exists.
// Thread-safe for concurrent access.
func (m *MockWorkflowRepository) SetExistsError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.existsErr = err
}

// Clear removes all workflows and resets error configuration.
// Thread-safe for concurrent access.
func (m *MockWorkflowRepository) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.workflows = make(map[string]*workflow.Workflow)
	m.loadErr = nil
	m.listErr = nil
	m.existsErr = nil
}

// =============================================================================
// MockStateStore - T003
// =============================================================================

// MockStateStore is a thread-safe mock implementation of ports.StateStore.
// It uses sync.RWMutex to protect concurrent access to the states map.
//
// Usage:
//
//	store := testutil.NewMockStateStore()
//	err := store.Save(ctx, state)
//	loaded, err := store.Load(ctx, "workflow-id")
type MockStateStore struct {
	mu        sync.RWMutex
	states    map[string]*workflow.ExecutionContext
	saveErr   error
	loadErr   error
	deleteErr error
	listErr   error
}

// NewMockStateStore creates a new thread-safe mock state store.
func NewMockStateStore() *MockStateStore {
	return &MockStateStore{
		states: make(map[string]*workflow.ExecutionContext),
	}
}

// Save persists an execution context.
func (m *MockStateStore) Save(ctx context.Context, state *workflow.ExecutionContext) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.saveErr != nil {
		return m.saveErr
	}

	if state == nil {
		return nil
	}

	// Make a copy to avoid external modifications
	// Cannot use struct copy due to sync.RWMutex - copy fields manually
	stateCopy := &workflow.ExecutionContext{
		WorkflowID:   state.WorkflowID,
		WorkflowName: state.WorkflowName,
		Status:       state.Status,
		CurrentStep:  state.CurrentStep,
		Inputs:       state.Inputs,
		States:       state.States,
		Env:          state.Env,
		StartedAt:    state.StartedAt,
		UpdatedAt:    state.UpdatedAt,
		CompletedAt:  state.CompletedAt,
		CurrentLoop:  state.CurrentLoop,
		CallStack:    state.CallStack,
	}
	m.states[state.WorkflowID] = stateCopy
	return nil
}

// Load retrieves an execution context by workflow ID.
func (m *MockStateStore) Load(ctx context.Context, workflowID string) (*workflow.ExecutionContext, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.loadErr != nil {
		return nil, m.loadErr
	}

	state, ok := m.states[workflowID]
	if !ok {
		return nil, nil
	}

	// Return a copy to prevent external modifications
	// Cannot use struct copy due to sync.RWMutex - copy fields manually
	stateCopy := &workflow.ExecutionContext{
		WorkflowID:   state.WorkflowID,
		WorkflowName: state.WorkflowName,
		Status:       state.Status,
		CurrentStep:  state.CurrentStep,
		Inputs:       state.Inputs,
		States:       state.States,
		Env:          state.Env,
		StartedAt:    state.StartedAt,
		UpdatedAt:    state.UpdatedAt,
		CompletedAt:  state.CompletedAt,
		CurrentLoop:  state.CurrentLoop,
		CallStack:    state.CallStack,
	}
	return stateCopy, nil
}

// Delete removes an execution context by workflow ID.
func (m *MockStateStore) Delete(ctx context.Context, workflowID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.deleteErr != nil {
		return m.deleteErr
	}

	delete(m.states, workflowID)
	return nil
}

// List returns all workflow IDs that have stored states.
func (m *MockStateStore) List(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.listErr != nil {
		return nil, m.listErr
	}

	if len(m.states) == 0 {
		return []string{}, nil
	}

	ids := make([]string, 0, len(m.states))
	for id := range m.states {
		ids = append(ids, id)
	}
	return ids, nil
}

// SetSaveError configures the mock to return an error on Save calls (test helper).
func (m *MockStateStore) SetSaveError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveErr = err
}

// SetLoadError configures the mock to return an error on Load calls (test helper).
func (m *MockStateStore) SetLoadError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadErr = err
}

// SetDeleteError configures the mock to return an error on Delete calls (test helper).
func (m *MockStateStore) SetDeleteError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteErr = err
}

// SetListError configures the mock to return an error on List calls (test helper).
func (m *MockStateStore) SetListError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listErr = err
}

// Clear removes all states from the store (test helper).
func (m *MockStateStore) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states = make(map[string]*workflow.ExecutionContext)
	m.saveErr = nil
	m.loadErr = nil
	m.deleteErr = nil
	m.listErr = nil
}

// =============================================================================
// MockCommandExecutor - T004
// =============================================================================

// MockCommandExecutor is a thread-safe mock implementation of ports.CommandExecutor.
// It uses sync.Mutex to protect concurrent access to call history.
//
// Usage:
//
//	executor := testutil.NewMockCommandExecutor()
//	executor.SetResult(&ports.CommandResult{Stdout: "output", ExitCode: 0})
//	result, err := executor.Execute(ctx, cmd)
type MockCommandExecutor struct {
	mu      sync.Mutex
	results map[string]*ports.CommandResult // Command-keyed results (key = Program field)
	execErr error
	calls   []*ports.Command
}

// NewMockCommandExecutor creates a new thread-safe mock command executor.
func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{
		results: make(map[string]*ports.CommandResult),
		calls:   make([]*ports.Command, 0),
	}
}

// Execute runs a command and returns the result.
func (m *MockCommandExecutor) Execute(ctx context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.execErr != nil {
		return nil, m.execErr
	}

	// Record the call
	m.calls = append(m.calls, cmd)

	// Return command-specific result, or fall back to default (empty key)
	if cmd != nil {
		if result, ok := m.results[cmd.Program]; ok {
			return result, nil
		}
	}

	// Return default result if configured, otherwise return nil
	if defaultResult, ok := m.results[""]; ok {
		return defaultResult, nil
	}

	// No result configured - return nil (matching legacy behavior)
	return nil, nil
}

// SetResult configures the mock to return a specific result for all commands (test helper).
//
// Deprecated: Use SetCommandResult for command-specific results. Migration tracked in #150.
func (m *MockCommandExecutor) SetResult(result *ports.CommandResult) {
	// Legacy behavior: set a default result for empty command key
	m.SetCommandResult("", result)
}

// SetCommandResult configures the mock to return a specific result for a given command (test helper).
func (m *MockCommandExecutor) SetCommandResult(cmd string, result *ports.CommandResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results[cmd] = result
}

// SetExecuteError configures the mock to return an error on Execute calls (test helper).
func (m *MockCommandExecutor) SetExecuteError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execErr = err
}

// GetCalls returns all recorded Execute calls (test helper).
func (m *MockCommandExecutor) GetCalls() []*ports.Command {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Create deep copy to isolate internal state
	copied := make([]*ports.Command, len(m.calls))
	for i, cmd := range m.calls {
		cmdCopy := *cmd
		copied[i] = &cmdCopy
	}
	return copied
}

// Clear removes all recorded calls and resets errors (test helper).
func (m *MockCommandExecutor) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = make([]*ports.Command, 0)
	m.results = make(map[string]*ports.CommandResult)
	m.execErr = nil
}

// =============================================================================
// MockLogger - T005
// =============================================================================

// MockLogger is a thread-safe mock implementation of ports.Logger.
// It uses sync.Mutex to protect concurrent access to captured messages.
//
// Usage:
//
//	logger := testutil.NewMockLogger()
//	logger.Info("test message", "key", "value")
//	messages := logger.GetMessages()
type MockLogger struct {
	mu       sync.Mutex
	messages []LogMessage
}

// LogMessage represents a captured log message with level and content.
type LogMessage struct {
	Level  string
	Msg    string
	Fields []any
}

// NewMockLogger creates a new thread-safe mock logger.
func NewMockLogger() *MockLogger {
	return &MockLogger{
		messages: make([]LogMessage, 0),
	}
}

// Debug logs a debug message.
func (m *MockLogger) Debug(msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, LogMessage{
		Level:  "DEBUG",
		Msg:    msg,
		Fields: fields,
	})
}

// Info logs an info message.
func (m *MockLogger) Info(msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, LogMessage{
		Level:  "INFO",
		Msg:    msg,
		Fields: fields,
	})
}

// Warn logs a warning message.
func (m *MockLogger) Warn(msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, LogMessage{
		Level:  "WARN",
		Msg:    msg,
		Fields: fields,
	})
}

// Error logs an error message.
func (m *MockLogger) Error(msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, LogMessage{
		Level:  "ERROR",
		Msg:    msg,
		Fields: fields,
	})
}

// WithContext returns a logger with additional context fields.
func (m *MockLogger) WithContext(ctx map[string]any) ports.Logger {
	// TODO(#150): implement context support
	return m
}

// GetMessages returns all captured log messages (test helper).
func (m *MockLogger) GetMessages() []LogMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]LogMessage{}, m.messages...)
}

// GetMessagesByLevel returns captured messages filtered by level (test helper).
func (m *MockLogger) GetMessagesByLevel(level string) []LogMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	filtered := make([]LogMessage, 0)
	for _, msg := range m.messages {
		if msg.Level == level {
			filtered = append(filtered, msg)
		}
	}
	return filtered
}

// Clear removes all captured messages (test helper).
func (m *MockLogger) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = make([]LogMessage, 0)
}

// MockHistoryStore is a thread-safe mock implementation of ports.HistoryStore.
// It uses sync.RWMutex to protect concurrent access to the records slice.
//
// Usage:
//
//	store := testutil.NewMockHistoryStore()
//	store.Record(ctx, &workflow.ExecutionRecord{...})
//	records, _ := store.List(ctx, nil)
type MockHistoryStore struct {
	mu      sync.RWMutex
	records []*workflow.ExecutionRecord
}

// NewMockHistoryStore creates a new thread-safe mock history store.
func NewMockHistoryStore() *MockHistoryStore {
	return &MockHistoryStore{
		records: make([]*workflow.ExecutionRecord, 0),
	}
}

// Record stores an execution record.
func (m *MockHistoryStore) Record(ctx context.Context, record *workflow.ExecutionRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, record)
	return nil
}

// List returns all recorded execution records (filter is ignored in mock).
func (m *MockHistoryStore) List(ctx context.Context, filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*workflow.ExecutionRecord, len(m.records))
	copy(result, m.records)
	return result, nil
}

// GetStats returns empty stats (not implemented in mock).
func (m *MockHistoryStore) GetStats(ctx context.Context, filter *workflow.HistoryFilter) (*workflow.HistoryStats, error) {
	return &workflow.HistoryStats{}, nil
}

// Cleanup removes records older than the given duration (returns 0 in mock).
func (m *MockHistoryStore) Cleanup(ctx context.Context, olderThan time.Duration) (int, error) {
	return 0, nil
}

// Close closes the store (no-op in mock).
func (m *MockHistoryStore) Close() error {
	return nil
}

// =============================================================================
// MockExpressionValidator - T006
// =============================================================================

// MockExpressionValidator is a thread-safe mock implementation of ports.ExpressionValidator.
// It uses sync.Mutex to protect concurrent access to configuration.
//
// Usage:
//
//	validator := testutil.NewMockExpressionValidator()
//	validator.SetCompileError(errors.New("syntax error"))
//	err := validator.Compile("invalid expression")
type MockExpressionValidator struct {
	mu          sync.Mutex
	compileErr  error
	compileFunc func(string) error
}

// NewMockExpressionValidator creates a new thread-safe mock expression validator.
func NewMockExpressionValidator() *MockExpressionValidator {
	return &MockExpressionValidator{}
}

// Compile validates the syntax of an expression string.
func (m *MockExpressionValidator) Compile(expression string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.compileFunc != nil {
		return m.compileFunc(expression)
	}

	if m.compileErr != nil {
		return m.compileErr
	}

	return nil
}

// SetCompileError configures the mock to return an error on Compile calls (test helper).
func (m *MockExpressionValidator) SetCompileError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.compileErr = err
	m.compileFunc = nil
}

// SetCompileFunc configures a custom function to handle Compile calls (test helper).
func (m *MockExpressionValidator) SetCompileFunc(fn func(string) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.compileFunc = fn
	m.compileErr = nil
}

// Clear resets the mock to default state (test helper).
func (m *MockExpressionValidator) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.compileErr = nil
	m.compileFunc = nil
}

// =============================================================================
// MockPluginManager - T007 (C037)
// =============================================================================

// MockPluginManager is a thread-safe mock implementation of ports.PluginManager.
// It uses sync.RWMutex to protect concurrent access to the plugins map.
// This mock consolidates duplicate implementations from plugin_test.go and plugin_service_test.go.
//
// Usage:
//
//	mgr := testutil.NewMockPluginManager()
//	mgr.AddPlugin("test-plugin", plugin.StatusRunning)
//	info, found := mgr.Get("test-plugin")
type MockPluginManager struct {
	mu            sync.RWMutex
	plugins       map[string]*plugin.PluginInfo
	discoverFunc  func(ctx context.Context) ([]*plugin.PluginInfo, error)
	loadFunc      func(ctx context.Context, name string) error
	initFunc      func(ctx context.Context, name string, config map[string]any) error
	shutdownFunc  func(ctx context.Context, name string) error
	shutdownError error
}

// NewMockPluginManager creates a new thread-safe mock plugin manager.
func NewMockPluginManager() *MockPluginManager {
	return &MockPluginManager{
		plugins: make(map[string]*plugin.PluginInfo),
	}
}

// Discover finds plugins in the plugins directory.
// Thread-safe for concurrent access.
func (m *MockPluginManager) Discover(ctx context.Context) ([]*plugin.PluginInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.discoverFunc != nil {
		return m.discoverFunc(ctx)
	}

	result := make([]*plugin.PluginInfo, 0, len(m.plugins))
	for _, p := range m.plugins {
		result = append(result, p)
	}
	return result, nil
}

// Load loads a plugin by name.
// Thread-safe for concurrent access.
func (m *MockPluginManager) Load(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.loadFunc != nil {
		return m.loadFunc(ctx, name)
	}

	if _, ok := m.plugins[name]; !ok {
		return errors.New("plugin not found")
	}
	m.plugins[name].Status = plugin.StatusLoaded
	return nil
}

// Init initializes a loaded plugin.
// Thread-safe for concurrent access.
func (m *MockPluginManager) Init(ctx context.Context, name string, config map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initFunc != nil {
		return m.initFunc(ctx, name, config)
	}

	if _, ok := m.plugins[name]; !ok {
		return errors.New("plugin not found")
	}
	m.plugins[name].Status = plugin.StatusRunning
	return nil
}

// Shutdown stops a running plugin.
// Thread-safe for concurrent access.
func (m *MockPluginManager) Shutdown(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shutdownFunc != nil {
		return m.shutdownFunc(ctx, name)
	}

	if info, ok := m.plugins[name]; ok {
		info.Status = plugin.StatusStopped
	}
	return nil
}

// ShutdownAll stops all running plugins.
// Thread-safe for concurrent access.
func (m *MockPluginManager) ShutdownAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shutdownError != nil {
		return m.shutdownError
	}

	for _, info := range m.plugins {
		if info.Status == plugin.StatusRunning {
			info.Status = plugin.StatusStopped
		}
	}
	return nil
}

// Get returns plugin info by name.
// Thread-safe for concurrent access.
func (m *MockPluginManager) Get(name string) (*plugin.PluginInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info, ok := m.plugins[name]
	return info, ok
}

// List returns all known plugins.
// Thread-safe for concurrent access.
func (m *MockPluginManager) List() []*plugin.PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*plugin.PluginInfo, 0, len(m.plugins))
	for _, p := range m.plugins {
		result = append(result, p)
	}
	return result
}

// AddPlugin adds or updates a plugin in the manager (test helper).
// Thread-safe for concurrent access.
func (m *MockPluginManager) AddPlugin(name string, status plugin.PluginStatus) *plugin.PluginInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	info := &plugin.PluginInfo{
		Manifest: &plugin.Manifest{
			Name:        name,
			Version:     "1.0.0",
			AWFVersion:  ">=0.4.0",
			Description: "Test plugin",
		},
		Status: status,
		Path:   "/test/plugins/" + name,
	}
	m.plugins[name] = info
	return info
}

// SetDiscoverFunc configures a custom function for Discover calls (test helper).
// Thread-safe for concurrent access.
func (m *MockPluginManager) SetDiscoverFunc(fn func(ctx context.Context) ([]*plugin.PluginInfo, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.discoverFunc = fn
}

// SetLoadFunc configures a custom function for Load calls (test helper).
// Thread-safe for concurrent access.
func (m *MockPluginManager) SetLoadFunc(fn func(ctx context.Context, name string) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadFunc = fn
}

// SetInitFunc configures a custom function for Init calls (test helper).
// Thread-safe for concurrent access.
func (m *MockPluginManager) SetInitFunc(fn func(ctx context.Context, name string, config map[string]any) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initFunc = fn
}

// SetShutdownFunc configures a custom function for Shutdown calls (test helper).
// Thread-safe for concurrent access.
func (m *MockPluginManager) SetShutdownFunc(fn func(ctx context.Context, name string) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shutdownFunc = fn
}

// SetShutdownError configures an error to be returned by ShutdownAll (test helper).
// Thread-safe for concurrent access.
func (m *MockPluginManager) SetShutdownError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shutdownError = err
}

// Clear removes all plugins and resets configuration (test helper).
// Thread-safe for concurrent access.
func (m *MockPluginManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.plugins = make(map[string]*plugin.PluginInfo)
	m.discoverFunc = nil
	m.loadFunc = nil
	m.initFunc = nil
	m.shutdownFunc = nil
	m.shutdownError = nil
}

// =============================================================================
// MockAgentRegistry - T001 (C038)
// =============================================================================

// MockAgentRegistry is a thread-safe mock implementation of ports.AgentRegistry.
// It uses sync.RWMutex to protect concurrent access to the providers map.
//
// Usage:
//
//	registry := testutil.NewMockAgentRegistry()
//	provider := testutil.NewMockAgentProvider("test-agent")
//	registry.Register(provider)
//	p, err := registry.Get("test-agent")
type MockAgentRegistry struct {
	mu        sync.RWMutex
	providers map[string]ports.AgentProvider
}

// NewMockAgentRegistry creates a new thread-safe mock agent registry.
func NewMockAgentRegistry() *MockAgentRegistry {
	return &MockAgentRegistry{
		providers: make(map[string]ports.AgentProvider),
	}
}

// Register adds a provider to the registry.
// Thread-safe for concurrent access.
// Returns error if a provider with the same name already exists.
func (m *MockAgentRegistry) Register(provider ports.AgentProvider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := provider.Name()
	if _, exists := m.providers[name]; exists {
		return errors.New("provider already registered: " + name)
	}

	m.providers[name] = provider
	return nil
}

// Get retrieves a provider by name.
// Thread-safe for concurrent access.
// Returns error if provider is not found.
func (m *MockAgentRegistry) Get(name string) (ports.AgentProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider, ok := m.providers[name]
	if !ok {
		return nil, errors.New("provider not found: " + name)
	}

	return provider, nil
}

// List returns all registered provider names.
// Thread-safe for concurrent access.
func (m *MockAgentRegistry) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		names = append(names, name)
	}

	return names
}

// Has checks if a provider with the given name is registered.
// Thread-safe for concurrent access.
func (m *MockAgentRegistry) Has(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.providers[name]
	return ok
}

// Clear removes all providers from the registry (test helper).
// Thread-safe for concurrent access.
func (m *MockAgentRegistry) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.providers = make(map[string]ports.AgentProvider)
}

// =============================================================================
// MockAgentProvider - T002 (C038)
// =============================================================================

// MockAgentProvider is a thread-safe mock implementation of ports.AgentProvider.
// It uses sync.RWMutex to protect concurrent access to the mock state and
// callback functions.
//
// Usage:
//
//	provider := testutil.NewMockAgentProvider("test-agent")
//	provider.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
//		return &workflow.AgentResult{
//			Provider: "test-agent",
//			Output:   "mock response",
//			Tokens:   100,
//		}, nil
//	})
//	result, err := provider.Execute(ctx, "test prompt", nil)
type MockAgentProvider struct {
	mu               sync.RWMutex
	name             string
	executeFunc      func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error)
	conversationFunc func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error)
	validateFunc     func() error
}

// NewMockAgentProvider creates a new thread-safe mock agent provider with the given name.
func NewMockAgentProvider(name string) *MockAgentProvider {
	return &MockAgentProvider{
		name: name,
	}
}

// Execute invokes the agent with the given prompt and options.
// Thread-safe for concurrent access.
// Returns a stub result if no executeFunc is configured.
func (m *MockAgentProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.executeFunc != nil {
		return m.executeFunc(ctx, prompt, options)
	}

	// Default stub behavior
	return &workflow.AgentResult{
		Provider: m.name,
		Output:   "",
		Tokens:   0,
	}, nil
}

// ExecuteConversation invokes the agent with conversation history for multi-turn interactions.
// Thread-safe for concurrent access.
// Returns a stub result if no conversationFunc is configured.
func (m *MockAgentProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.conversationFunc != nil {
		return m.conversationFunc(ctx, state, prompt, options)
	}

	// Default stub behavior
	return &workflow.ConversationResult{
		Provider: m.name,
		State:    state,
		Output:   "",
	}, nil
}

// Name returns the provider identifier.
// Thread-safe for concurrent access.
func (m *MockAgentProvider) Name() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.name
}

// Validate checks if the provider is properly configured and available.
// Thread-safe for concurrent access.
// Returns nil if no validateFunc is configured.
func (m *MockAgentProvider) Validate() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.validateFunc != nil {
		return m.validateFunc()
	}

	return nil
}

// SetExecuteFunc sets the callback function for Execute method (test helper).
// Thread-safe for concurrent access.
func (m *MockAgentProvider) SetExecuteFunc(f func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.executeFunc = f
}

// SetConversationFunc sets the callback function for ExecuteConversation method (test helper).
// Thread-safe for concurrent access.
func (m *MockAgentProvider) SetConversationFunc(f func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.conversationFunc = f
}

// SetValidateFunc sets the callback function for Validate method (test helper).
// Thread-safe for concurrent access.
func (m *MockAgentProvider) SetValidateFunc(f func() error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.validateFunc = f
}

// Clear resets all callback functions (test helper).
// Thread-safe for concurrent access.
func (m *MockAgentProvider) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.executeFunc = nil
	m.conversationFunc = nil
	m.validateFunc = nil
}

// =============================================================================
// MockCLIExecutor - T003
// =============================================================================

// MockCLIExecutor is a thread-safe mock implementation of ports.CLIExecutor.
// It uses sync.Mutex to protect concurrent access to call history.
//
// Usage:
//
//	executor := testutil.NewMockCLIExecutor()
//	executor.SetOutput([]byte("output"), []byte(""))
//	stdout, stderr, err := executor.Run(ctx, "claude", "--version")
type MockCLIExecutor struct {
	mu      sync.Mutex
	stdout  []byte
	stderr  []byte
	execErr error
	calls   []MockCLICall
}

// MockCLICall records a single CLI execution call.
type MockCLICall struct {
	Name string
	Args []string
}

// NewMockCLIExecutor creates a new thread-safe mock CLI executor.
func NewMockCLIExecutor() *MockCLIExecutor {
	return &MockCLIExecutor{
		calls: make([]MockCLICall, 0),
	}
}

// Run executes a binary and returns the configured output.
func (m *MockCLIExecutor) Run(ctx context.Context, name string, args ...string) (stdout, stderr []byte, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the call
	m.calls = append(m.calls, MockCLICall{Name: name, Args: args})

	// Return configured error if set
	if m.execErr != nil {
		return nil, nil, m.execErr
	}

	// Return configured output
	return m.stdout, m.stderr, nil
}

// SetOutput configures the mock to return specific stdout and stderr (test helper).
func (m *MockCLIExecutor) SetOutput(stdout, stderr []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stdout = stdout
	m.stderr = stderr
}

// SetError configures the mock to return an error on Run calls (test helper).
func (m *MockCLIExecutor) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execErr = err
}

// GetCalls returns all recorded Run calls (test helper).
func (m *MockCLIExecutor) GetCalls() []MockCLICall {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Create deep copy to isolate internal state
	copied := make([]MockCLICall, len(m.calls))
	for i, call := range m.calls {
		argsCopy := make([]string, len(call.Args))
		copy(argsCopy, call.Args)
		copied[i] = MockCLICall{
			Name: call.Name,
			Args: argsCopy,
		}
	}
	return copied
}

// Clear removes all recorded calls and resets output/errors (test helper).
func (m *MockCLIExecutor) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = make([]MockCLICall, 0)
	m.stdout = nil
	m.stderr = nil
	m.execErr = nil
}
