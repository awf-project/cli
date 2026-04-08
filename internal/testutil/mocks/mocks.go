package mocks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
)

// This file contains thread-safe mock implementations of domain port interfaces.
// All mocks use sync.Mutex or sync.RWMutex for concurrent access protection.

// Compile-time interface compliance verification.
var (
	_ ports.WorkflowRepository  = (*MockWorkflowRepository)(nil)
	_ ports.TemplateRepository  = (*MockTemplateRepository)(nil)
	_ ports.StateStore          = (*MockStateStore)(nil)
	_ ports.CommandExecutor     = (*MockCommandExecutor)(nil)
	_ ports.CLIExecutor         = (*MockCLIExecutor)(nil)
	_ ports.Logger              = (*MockLogger)(nil)
	_ ports.HistoryStore        = (*MockHistoryStore)(nil)
	_ ports.ExpressionValidator = (*MockExpressionValidator)(nil)
	_ ports.ExpressionEvaluator = (*MockExpressionEvaluator)(nil)
	_ ports.PluginManager       = (*MockPluginManager)(nil)
	_ ports.AgentRegistry       = (*MockAgentRegistry)(nil)
	_ ports.AgentProvider       = (*MockAgentProvider)(nil)
	_ ports.ErrorFormatter      = (*MockErrorFormatter)(nil)
	_ ports.AuditTrailWriter    = (*MockAuditTrailWriter)(nil)
	_ ports.PluginStore         = (*MockPluginStore)(nil)
	_ ports.PluginConfig        = (*MockPluginConfig)(nil)
	_ ports.PluginStateStore    = (*MockPluginStateStore)(nil)
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

// Load retrieves a workflow by name.
// Returns StructuredError with USER.INPUT.MISSING_FILE if workflow does not exist.
// Thread-safe for concurrent access.
func (m *MockWorkflowRepository) Load(ctx context.Context, name string) (*workflow.Workflow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.loadErr != nil {
		return nil, m.loadErr
	}

	wf, ok := m.workflows[name]
	if !ok {
		// Return StructuredError matching real repository behavior
		return nil, domainerrors.NewUserError(
			domainerrors.ErrorCodeUserInputMissingFile,
			"workflow file not found: "+name,
			map[string]any{"path": name},
			nil,
		)
	}

	return wf, nil
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

// MockTemplateRepository is a thread-safe mock implementation of ports.TemplateRepository.
// It uses sync.RWMutex to protect concurrent access to the templates map.
//
// Usage:
//
//	repo := testutil.NewMockTemplateRepository()
//	repo.AddTemplate("test", &workflow.Template{Name: "test"})
//	tpl, err := repo.GetTemplate(ctx, "test")
type MockTemplateRepository struct {
	mu        sync.RWMutex
	templates map[string]*workflow.Template
	getErr    error
	listErr   error
}

// NewMockTemplateRepository creates a new thread-safe mock template repository.
func NewMockTemplateRepository() *MockTemplateRepository {
	return &MockTemplateRepository{
		templates: make(map[string]*workflow.Template),
	}
}

// GetTemplate retrieves a template by name. Returns TemplateNotFoundError if template does not exist.
// Thread-safe for concurrent access.
func (m *MockTemplateRepository) GetTemplate(ctx context.Context, name string) (*workflow.Template, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getErr != nil {
		return nil, m.getErr
	}

	tpl, exists := m.templates[name]
	if !exists {
		return nil, &workflow.TemplateNotFoundError{TemplateName: name}
	}

	return tpl, nil
}

// ListTemplates returns all template names in the repository.
// Thread-safe for concurrent access.
func (m *MockTemplateRepository) ListTemplates(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.listErr != nil {
		return nil, m.listErr
	}

	names := make([]string, 0, len(m.templates))
	for name := range m.templates {
		names = append(names, name)
	}

	return names, nil
}

// Exists checks if a template with the given name exists.
// Thread-safe for concurrent access.
func (m *MockTemplateRepository) Exists(ctx context.Context, name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.templates[name]
	return exists
}

// AddTemplate adds or updates a template in the repository (test helper).
// Thread-safe for concurrent access.
func (m *MockTemplateRepository) AddTemplate(name string, tpl *workflow.Template) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.templates[name] = tpl
}

// SetGetError configures an error to be returned by GetTemplate (test helper).
// Thread-safe for concurrent access.
func (m *MockTemplateRepository) SetGetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getErr = err
}

// SetListError configures an error to be returned by ListTemplates (test helper).
// Thread-safe for concurrent access.
func (m *MockTemplateRepository) SetListError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.listErr = err
}

// Clear removes all templates and resets error configuration (test helper).
// Thread-safe for concurrent access.
func (m *MockTemplateRepository) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.templates = make(map[string]*workflow.Template)
	m.getErr = nil
	m.listErr = nil
}

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

// MockLogger is a thread-safe mock implementation of ports.Logger.
// It uses sync.Mutex to protect concurrent access to captured messages.
//
// Usage:
//
//	logger := testutil.NewMockLogger()
//	logger.Info("test message", "key", "value")
//	messages := logger.GetMessages()
type MockLogger struct {
	mu        *sync.Mutex
	messages  *[]LogMessage
	ctxFields []any // Context fields accumulated via WithContext()
}

// LogMessage represents a captured log message with level and content.
type LogMessage struct {
	Level  string
	Msg    string
	Fields []any
}

// NewMockLogger creates a new thread-safe mock logger.
func NewMockLogger() *MockLogger {
	messages := make([]LogMessage, 0)
	return &MockLogger{
		mu:       &sync.Mutex{},
		messages: &messages,
	}
}

// Debug logs a debug message.
func (m *MockLogger) Debug(msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Prepend context fields before message fields
	allFields := make([]any, 0, len(m.ctxFields)+len(fields))
	allFields = append(allFields, m.ctxFields...)
	allFields = append(allFields, fields...)
	*m.messages = append(*m.messages, LogMessage{
		Level:  "DEBUG",
		Msg:    msg,
		Fields: allFields,
	})
}

// Info logs an info message.
func (m *MockLogger) Info(msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Prepend context fields before message fields
	allFields := make([]any, 0, len(m.ctxFields)+len(fields))
	allFields = append(allFields, m.ctxFields...)
	allFields = append(allFields, fields...)
	*m.messages = append(*m.messages, LogMessage{
		Level:  "INFO",
		Msg:    msg,
		Fields: allFields,
	})
}

// Warn logs a warning message.
func (m *MockLogger) Warn(msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Prepend context fields before message fields
	allFields := make([]any, 0, len(m.ctxFields)+len(fields))
	allFields = append(allFields, m.ctxFields...)
	allFields = append(allFields, fields...)
	*m.messages = append(*m.messages, LogMessage{
		Level:  "WARN",
		Msg:    msg,
		Fields: allFields,
	})
}

// Error logs an error message.
func (m *MockLogger) Error(msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Prepend context fields before message fields
	allFields := make([]any, 0, len(m.ctxFields)+len(fields))
	allFields = append(allFields, m.ctxFields...)
	allFields = append(allFields, fields...)
	*m.messages = append(*m.messages, LogMessage{
		Level:  "ERROR",
		Msg:    msg,
		Fields: allFields,
	})
}

// WithContext returns a logger with additional context fields.
// Creates a new immutable logger instance with accumulated context.
func (m *MockLogger) WithContext(ctx map[string]any) ports.Logger {
	// Convert map to alternating key-value pairs
	ctxFields := make([]any, 0, len(ctx)*2)
	for k, v := range ctx {
		ctxFields = append(ctxFields, k, v)
	}

	// Create new logger with accumulated context fields
	// Share mutex and messages pointer to maintain thread safety
	return &MockLogger{
		mu:        m.mu,       // Share mutex
		messages:  m.messages, // Share messages pointer
		ctxFields: append(m.ctxFields, ctxFields...),
	}
}

// GetMessages returns all captured log messages (test helper).
func (m *MockLogger) GetMessages() []LogMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]LogMessage{}, *m.messages...)
}

// GetMessagesByLevel returns captured messages filtered by level (test helper).
func (m *MockLogger) GetMessagesByLevel(level string) []LogMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	filtered := make([]LogMessage, 0)
	for _, msg := range *m.messages {
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
	*m.messages = make([]LogMessage, 0)
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

// MockExpressionEvaluator is a thread-safe mock implementation of ports.ExpressionEvaluator.
// It uses sync.Mutex to protect concurrent access to configuration.
//
// Usage:
//
//	evaluator := testutil.NewMockExpressionEvaluator()
//	evaluator.SetBoolResult(true, nil)
//	result, err := evaluator.EvaluateBool("inputs.count > 5", ctx)
type MockExpressionEvaluator struct {
	mu               sync.Mutex
	boolResult       bool
	boolErr          error
	intResult        int
	intErr           error
	evaluateBoolFunc func(string, *interpolation.Context) (bool, error)
	evaluateIntFunc  func(string, *interpolation.Context) (int, error)
}

// NewMockExpressionEvaluator creates a new thread-safe mock expression evaluator.
func NewMockExpressionEvaluator() *MockExpressionEvaluator {
	return &MockExpressionEvaluator{}
}

// EvaluateBool evaluates a boolean expression against the provided context.
func (m *MockExpressionEvaluator) EvaluateBool(expr string, ctx *interpolation.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.evaluateBoolFunc != nil {
		return m.evaluateBoolFunc(expr, ctx)
	}

	if m.boolErr != nil {
		return false, m.boolErr
	}

	return m.boolResult, nil
}

// EvaluateInt evaluates an arithmetic expression against the provided context.
func (m *MockExpressionEvaluator) EvaluateInt(expr string, ctx *interpolation.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.evaluateIntFunc != nil {
		return m.evaluateIntFunc(expr, ctx)
	}

	if m.intErr != nil {
		return 0, m.intErr
	}

	return m.intResult, nil
}

// SetBoolResult configures the mock to return a specific result for EvaluateBool calls (test helper).
func (m *MockExpressionEvaluator) SetBoolResult(result bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.boolResult = result
	m.boolErr = err
	m.evaluateBoolFunc = nil
}

// SetIntResult configures the mock to return a specific result for EvaluateInt calls (test helper).
func (m *MockExpressionEvaluator) SetIntResult(result int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.intResult = result
	m.intErr = err
	m.evaluateIntFunc = nil
}

// SetEvaluateBoolFunc configures a custom function to handle EvaluateBool calls (test helper).
func (m *MockExpressionEvaluator) SetEvaluateBoolFunc(fn func(string, *interpolation.Context) (bool, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.evaluateBoolFunc = fn
	m.boolErr = nil
}

// SetEvaluateIntFunc configures a custom function to handle EvaluateInt calls (test helper).
func (m *MockExpressionEvaluator) SetEvaluateIntFunc(fn func(string, *interpolation.Context) (int, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.evaluateIntFunc = fn
	m.intErr = nil
}

// Clear resets the mock to default state (test helper).
func (m *MockExpressionEvaluator) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.boolResult = false
	m.boolErr = nil
	m.intResult = 0
	m.intErr = nil
	m.evaluateBoolFunc = nil
	m.evaluateIntFunc = nil
}

// MockPluginManager is a thread-safe mock implementation of ports.PluginManager.
// It uses sync.RWMutex to protect concurrent access to the plugins map.
// This mock consolidates duplicate implementations from plugin_test.go and plugin_service_test.go.
//
// Usage:
//
//	mgr := testutil.NewMockPluginManager()
//	mgr.AddPlugin("test-plugin", pluginmodel.StatusRunning)
//	info, found := mgr.Get("test-plugin")
type MockPluginManager struct {
	mu            sync.RWMutex
	plugins       map[string]*pluginmodel.PluginInfo
	discoverFunc  func(ctx context.Context) ([]*pluginmodel.PluginInfo, error)
	loadFunc      func(ctx context.Context, name string) error
	initFunc      func(ctx context.Context, name string, config map[string]any) error
	shutdownFunc  func(ctx context.Context, name string) error
	shutdownError error
}

// NewMockPluginManager creates a new thread-safe mock plugin manager.
func NewMockPluginManager() *MockPluginManager {
	return &MockPluginManager{
		plugins: make(map[string]*pluginmodel.PluginInfo),
	}
}

// Discover finds plugins in the plugins directory.
// Thread-safe for concurrent access.
func (m *MockPluginManager) Discover(ctx context.Context) ([]*pluginmodel.PluginInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.discoverFunc != nil {
		return m.discoverFunc(ctx)
	}

	result := make([]*pluginmodel.PluginInfo, 0, len(m.plugins))
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
	m.plugins[name].Status = pluginmodel.StatusLoaded
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
	m.plugins[name].Status = pluginmodel.StatusRunning
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
		info.Status = pluginmodel.StatusStopped
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
		if info.Status == pluginmodel.StatusRunning {
			info.Status = pluginmodel.StatusStopped
		}
	}
	return nil
}

// Get returns plugin info by name.
// Thread-safe for concurrent access.
func (m *MockPluginManager) Get(name string) (*pluginmodel.PluginInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info, ok := m.plugins[name]
	return info, ok
}

// List returns all known plugins.
// Thread-safe for concurrent access.
func (m *MockPluginManager) List() []*pluginmodel.PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*pluginmodel.PluginInfo, 0, len(m.plugins))
	for _, p := range m.plugins {
		result = append(result, p)
	}
	return result
}

// AddPlugin adds or updates a plugin in the manager (test helper).
// Thread-safe for concurrent access.
func (m *MockPluginManager) AddPlugin(name string, status pluginmodel.PluginStatus) *pluginmodel.PluginInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	info := &pluginmodel.PluginInfo{
		Manifest: &pluginmodel.Manifest{
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
func (m *MockPluginManager) SetDiscoverFunc(fn func(ctx context.Context) ([]*pluginmodel.PluginInfo, error)) {
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

	m.plugins = make(map[string]*pluginmodel.PluginInfo)
	m.discoverFunc = nil
	m.loadFunc = nil
	m.initFunc = nil
	m.shutdownFunc = nil
	m.shutdownError = nil
}

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

// MockAgentProvider is a thread-safe mock implementation of ports.AgentProvider.
// It uses sync.RWMutex to protect concurrent access to the mock state and
// callback functions.
//
// Usage:
//
//	provider := testutil.NewMockAgentProvider("test-agent")
//	provider.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
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
	executeFunc      func(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error)
	conversationFunc func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error)
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
func (m *MockAgentProvider) Execute(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.executeFunc != nil {
		return m.executeFunc(ctx, prompt, options, stdout, stderr)
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
func (m *MockAgentProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.conversationFunc != nil {
		return m.conversationFunc(ctx, state, prompt, options, stdout, stderr)
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
func (m *MockAgentProvider) SetExecuteFunc(f func(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.executeFunc = f
}

// SetConversationFunc sets the callback function for ExecuteConversation method (test helper).
// Thread-safe for concurrent access.
func (m *MockAgentProvider) SetConversationFunc(f func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error)) {
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

// MockCLIExecutor is a thread-safe mock implementation of ports.CLIExecutor.
// It uses sync.Mutex to protect concurrent access to call history.
//
// Usage:
//
//	executor := testutil.NewMockCLIExecutor()
//	executor.SetOutput([]byte("output"), []byte(""))
//	stdout, stderr, err := executor.Run(ctx, "claude", nil, nil, "--version")
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
func (m *MockCLIExecutor) Run(ctx context.Context, name string, stdoutW, stderrW io.Writer, args ...string) (stdout, stderr []byte, err error) {
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

// MockErrorFormatter is a thread-safe mock implementation of ports.ErrorFormatter.
// It uses sync.Mutex to protect concurrent access to the format function and hint generators.
//
// C048 Extension: Now supports hint generators for testing hint-aware formatting behavior.
//
// Usage:
//
//	formatter := testutil.NewMockErrorFormatter()
//	formatter.SetFormatFunc(func(err *domainerrors.StructuredError) string {
//		return fmt.Sprintf("[%s] %s", err.Code, err.Message)
//	})
//	formatter.AddHintGenerator(func(err *domainerrors.StructuredError) []domainerrors.Hint {
//		return []domainerrors.Hint{{Message: "Did you mean 'workflow.yaml'?"}}
//	})
//	output := formatter.FormatError(structuredErr)
type MockErrorFormatter struct {
	mu             sync.Mutex
	formatFunc     func(err *domainerrors.StructuredError) string
	hintGenerators []domainerrors.HintGenerator
	hintsEnabled   bool
}

// NewMockErrorFormatter creates a new thread-safe mock error formatter.
// By default, hints are disabled (hintsEnabled=false) to maintain backward compatibility.
func NewMockErrorFormatter() *MockErrorFormatter {
	return &MockErrorFormatter{
		hintGenerators: make([]domainerrors.HintGenerator, 0),
		hintsEnabled:   false,
	}
}

// FormatError formats a structured error using the configured format function.
// Thread-safe for concurrent access.
// Returns empty string if no formatFunc is configured.
func (m *MockErrorFormatter) FormatError(err *domainerrors.StructuredError) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.formatFunc != nil {
		return m.formatFunc(err)
	}

	// Default stub behavior: return empty string
	return ""
}

// SetFormatFunc configures a custom function for FormatError calls (test helper).
// Thread-safe for concurrent access.
func (m *MockErrorFormatter) SetFormatFunc(fn func(err *domainerrors.StructuredError) string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.formatFunc = fn
}

// AddHintGenerator adds a hint generator to the mock formatter (test helper).
// Thread-safe for concurrent access.
// Useful for testing hint generation behavior without implementing full formatters.
func (m *MockErrorFormatter) AddHintGenerator(gen domainerrors.HintGenerator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hintGenerators = append(m.hintGenerators, gen)
}

// SetHintGenerators replaces all hint generators with the provided slice (test helper).
// Thread-safe for concurrent access.
func (m *MockErrorFormatter) SetHintGenerators(generators []domainerrors.HintGenerator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hintGenerators = generators
}

// EnableHints enables hint generation in the mock formatter (test helper).
// Thread-safe for concurrent access.
func (m *MockErrorFormatter) EnableHints(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hintsEnabled = enabled
}

// GetHints returns all hints generated by registered generators for the given error (test helper).
// Thread-safe for concurrent access.
// Returns empty slice if hints are disabled or no generators are configured.
func (m *MockErrorFormatter) GetHints(err *domainerrors.StructuredError) []domainerrors.Hint {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.hintsEnabled || len(m.hintGenerators) == 0 {
		return []domainerrors.Hint{}
	}

	hints := make([]domainerrors.Hint, 0)
	for _, gen := range m.hintGenerators {
		if gen == nil {
			continue
		}
		generatedHints := gen(err)
		hints = append(hints, generatedHints...)
	}

	return hints
}

// Clear resets the format function and hint generators (test helper).
// Thread-safe for concurrent access.
func (m *MockErrorFormatter) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.formatFunc = nil
	m.hintGenerators = make([]domainerrors.HintGenerator, 0)
	m.hintsEnabled = false
}

// MockAuditTrailWriter is a thread-safe mock implementation of ports.AuditTrailWriter.
// It uses sync.RWMutex to protect concurrent access to the events slice.
// Supports lifecycle state tracking (isClosed) for write-after-close and close-idempotency tests.
//
// Usage:
//
//	writer := testutil.NewMockAuditTrailWriter()
//	err := writer.Write(ctx, &event)
//	events := writer.GetEvents()
type MockAuditTrailWriter struct {
	mu       sync.RWMutex
	events   []workflow.AuditEvent
	writeErr error
	closeErr error
	isClosed bool
}

// NewMockAuditTrailWriter creates a new thread-safe mock audit trail writer.
func NewMockAuditTrailWriter() *MockAuditTrailWriter {
	return &MockAuditTrailWriter{
		events: make([]workflow.AuditEvent, 0),
	}
}

// Write appends an audit event to the recorded events.
// Returns error if writer is closed or writeErr is set.
// Thread-safe for concurrent access.
func (m *MockAuditTrailWriter) Write(ctx context.Context, event *workflow.AuditEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isClosed {
		return fmt.Errorf("writer is closed")
	}

	if m.writeErr != nil {
		return m.writeErr
	}

	m.events = append(m.events, *event)
	return nil
}

// Close closes the writer. Returns error if already closed or closeErr is set.
// Thread-safe for concurrent access.
func (m *MockAuditTrailWriter) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isClosed {
		return fmt.Errorf("already closed")
	}

	m.isClosed = true

	return m.closeErr
}

// GetEvents returns all recorded audit events (test helper).
// Thread-safe for concurrent access.
func (m *MockAuditTrailWriter) GetEvents() []workflow.AuditEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]workflow.AuditEvent, len(m.events))
	copy(result, m.events)
	return result
}

// SetWriteError configures the mock to return an error on Write calls (test helper).
// Thread-safe for concurrent access.
func (m *MockAuditTrailWriter) SetWriteError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.writeErr = err
}

// SetCloseError configures the mock to return an error on Close calls (test helper).
// Thread-safe for concurrent access.
func (m *MockAuditTrailWriter) SetCloseError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closeErr = err
}

// IsClosed returns whether the writer has been closed (test helper).
// Thread-safe for concurrent access.
func (m *MockAuditTrailWriter) IsClosed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.isClosed
}

// Clear removes all recorded events, resets errors, and reopens the writer (test helper).
// Thread-safe for concurrent access.
func (m *MockAuditTrailWriter) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = make([]workflow.AuditEvent, 0)
	m.writeErr = nil
	m.closeErr = nil
	m.isClosed = false
}

// MockPluginStore is a thread-safe mock implementation of ports.PluginStore.
// It supports optional error injection via SaveErr and LoadErr fields, and
// custom behavior via SaveFunc and LoadFunc overrides.
type MockPluginStore struct {
	mu       sync.RWMutex
	states   map[string]*pluginmodel.PluginState
	SaveFunc func(ctx context.Context) error
	LoadFunc func(ctx context.Context) error
	SaveErr  error
	LoadErr  error
}

// NewMockPluginStore creates a new thread-safe mock plugin store.
func NewMockPluginStore() *MockPluginStore {
	return &MockPluginStore{
		states: make(map[string]*pluginmodel.PluginState),
	}
}

// Save persists plugin states. Honors SaveFunc override, then SaveErr, then returns nil.
// Thread-safe for concurrent access.
func (m *MockPluginStore) Save(ctx context.Context) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx)
	}
	if m.SaveErr != nil {
		return m.SaveErr
	}
	return nil
}

// Load reads plugin states. Honors LoadFunc override, then LoadErr, then returns nil.
// Thread-safe for concurrent access.
func (m *MockPluginStore) Load(ctx context.Context) error {
	if m.LoadFunc != nil {
		return m.LoadFunc(ctx)
	}
	if m.LoadErr != nil {
		return m.LoadErr
	}
	return nil
}

// GetState returns the full state for a plugin, or nil if not found.
// Thread-safe for concurrent access.
func (m *MockPluginStore) GetState(name string) *pluginmodel.PluginState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.states[name]
}

// ListDisabled returns names of all explicitly disabled plugins.
// Thread-safe for concurrent access.
func (m *MockPluginStore) ListDisabled() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var disabled []string
	for name, state := range m.states {
		if !state.Enabled {
			disabled = append(disabled, name)
		}
	}
	return disabled
}

// MockPluginConfig is a thread-safe mock implementation of ports.PluginConfig.
// It supports optional error injection via SetEnabledErr and SetConfigErr fields.
type MockPluginConfig struct {
	mu            sync.RWMutex
	states        map[string]*pluginmodel.PluginState
	SetEnabledErr error
	SetConfigErr  error
}

// NewMockPluginConfig creates a new thread-safe mock plugin config.
func NewMockPluginConfig() *MockPluginConfig {
	return &MockPluginConfig{
		states: make(map[string]*pluginmodel.PluginState),
	}
}

// SetEnabled enables or disables a plugin by name.
// Returns SetEnabledErr if set. Thread-safe for concurrent access.
func (m *MockPluginConfig) SetEnabled(ctx context.Context, name string, enabled bool) error {
	if m.SetEnabledErr != nil {
		return m.SetEnabledErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.states[name]
	if !ok {
		state = pluginmodel.NewPluginState()
		m.states[name] = state
	}
	state.Enabled = enabled
	return nil
}

// IsEnabled returns whether a plugin is enabled.
// Unknown plugin names return true (default-enabled contract).
// Thread-safe for concurrent access.
func (m *MockPluginConfig) IsEnabled(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.states[name]
	if !ok {
		return true
	}
	return state.Enabled
}

// GetConfig returns the stored configuration for a plugin.
// Thread-safe for concurrent access.
func (m *MockPluginConfig) GetConfig(name string) map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.states[name]
	if !ok {
		return nil
	}
	return state.Config
}

// SetConfig stores configuration for a plugin.
// Returns SetConfigErr if set. Thread-safe for concurrent access.
func (m *MockPluginConfig) SetConfig(ctx context.Context, name string, config map[string]any) error {
	if m.SetConfigErr != nil {
		return m.SetConfigErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.states[name]
	if !ok {
		state = pluginmodel.NewPluginState()
		m.states[name] = state
	}
	state.Config = config
	return nil
}

// MockPluginStateStore combines MockPluginStore and MockPluginConfig behind the
// ports.PluginStateStore composite interface. Both halves share a single states
// map so that mutations made through one interface are visible through the other.
type MockPluginStateStore struct {
	*MockPluginStore
	*MockPluginConfig
}

// NewMockPluginStateStore creates a new MockPluginStateStore with shared state.
func NewMockPluginStateStore() *MockPluginStateStore {
	store := NewMockPluginStore()
	config := NewMockPluginConfig()
	// Share the same states map so mutations are visible across both interfaces.
	config.states = store.states
	return &MockPluginStateStore{
		MockPluginStore:  store,
		MockPluginConfig: config,
	}
}
