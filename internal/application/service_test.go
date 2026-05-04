package application_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/application"
	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock implementations
type mockRepository struct {
	workflows map[string]*workflow.Workflow
}

func newMockRepository() *mockRepository {
	return &mockRepository{workflows: make(map[string]*workflow.Workflow)}
}

func (m *mockRepository) Load(ctx context.Context, name string) (*workflow.Workflow, error) {
	if wf, ok := m.workflows[name]; ok {
		return wf, nil
	}
	// Return StructuredError matching real repository behavior
	return nil, domerrors.NewUserError(
		domerrors.ErrorCodeUserInputMissingFile,
		fmt.Sprintf("workflow file not found: %s", name),
		map[string]any{"path": name},
		nil,
	)
}

func (m *mockRepository) List(ctx context.Context) ([]string, error) {
	names := make([]string, 0, len(m.workflows))
	for name := range m.workflows {
		names = append(names, name)
	}
	return names, nil
}

func (m *mockRepository) Exists(ctx context.Context, name string) (bool, error) {
	_, ok := m.workflows[name]
	return ok, nil
}

type mockStateStore struct {
	states map[string]*workflow.ExecutionContext
}

func newMockStateStore() *mockStateStore {
	return &mockStateStore{states: make(map[string]*workflow.ExecutionContext)}
}

func (m *mockStateStore) Save(ctx context.Context, state *workflow.ExecutionContext) error {
	m.states[state.WorkflowID] = state
	return nil
}

func (m *mockStateStore) Load(ctx context.Context, id string) (*workflow.ExecutionContext, error) {
	if state, ok := m.states[id]; ok {
		return state, nil
	}
	return nil, nil
}

func (m *mockStateStore) Delete(ctx context.Context, id string) error {
	delete(m.states, id)
	return nil
}

func (m *mockStateStore) List(ctx context.Context) ([]string, error) {
	ids := make([]string, 0, len(m.states))
	for id := range m.states {
		ids = append(ids, id)
	}
	return ids, nil
}

type mockExecutor struct {
	results map[string]*ports.CommandResult
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{results: make(map[string]*ports.CommandResult)}
}

func (m *mockExecutor) Execute(ctx context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	if result, ok := m.results[cmd.Program]; ok {
		return result, nil
	}
	return &ports.CommandResult{ExitCode: 0, Stdout: "ok"}, nil
}

// capturingMockExecutor captures the last command for inspection
type capturingMockExecutor struct {
	lastCmd *ports.Command
	results map[string]*ports.CommandResult
}

func newCapturingMockExecutor() *capturingMockExecutor {
	return &capturingMockExecutor{results: make(map[string]*ports.CommandResult)}
}

func (m *capturingMockExecutor) Execute(ctx context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	m.lastCmd = cmd
	if result, ok := m.results[cmd.Program]; ok {
		return result, nil
	}
	return &ports.CommandResult{ExitCode: 0, Stdout: "ok"}, nil
}

type mockLogger struct {
	warnings []string
	errors   []string
}

func (m *mockLogger) Debug(msg string, fields ...any) {}
func (m *mockLogger) Info(msg string, fields ...any)  {}
func (m *mockLogger) Warn(msg string, fields ...any) {
	if m.warnings == nil {
		m.warnings = []string{}
	}
	m.warnings = append(m.warnings, msg)
}

func (m *mockLogger) Error(msg string, fields ...any) {
	if m.errors == nil {
		m.errors = []string{}
	}
	m.errors = append(m.errors, msg)
}

func (m *mockLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

// mockResolver passes commands through unchanged (no interpolation)
type mockResolver struct{}

func newMockResolver() *mockResolver {
	return &mockResolver{}
}

func (m *mockResolver) Resolve(template string, ctx *interpolation.Context) (string, error) {
	return template, nil
}

// mockExpressionValidator is a simple mock that always returns nil (valid).
type mockExpressionValidator struct{}

func newMockExpressionValidator() *mockExpressionValidator {
	return &mockExpressionValidator{}
}

func (m *mockExpressionValidator) Compile(expression string) error {
	return nil
}

// mockParallelExecutor is a simple mock that executes branches sequentially.
type mockParallelExecutor struct{}

func newMockParallelExecutor() *mockParallelExecutor {
	return &mockParallelExecutor{}
}

func (m *mockParallelExecutor) Execute(
	ctx context.Context,
	wf *workflow.Workflow,
	branches []string,
	config workflow.ParallelConfig,
	execCtx *workflow.ExecutionContext,
	stepExecutor ports.StepExecutor,
) (*workflow.ParallelResult, error) {
	result := workflow.NewParallelResult()
	for _, branch := range branches {
		branchResult, err := stepExecutor.ExecuteStep(ctx, wf, branch, execCtx)
		if branchResult != nil {
			result.AddResult(branchResult)
		}
		if err != nil && config.Strategy == workflow.StrategyAllSucceed {
			return result, fmt.Errorf("branch %s failed: %w", branch, err)
		}
	}
	return result, nil
}

func TestNewWorkflowService(t *testing.T) {
	repo := newMockRepository()
	store := newMockStateStore()
	exec := newMockExecutor()
	log := &mockLogger{}

	svc := application.NewWorkflowService(repo, store, exec, log, newMockExpressionValidator())
	if svc == nil {
		t.Error("expected service to be created")
	}
}

func TestWorkflowServiceListWorkflows(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["wf1"] = &workflow.Workflow{Name: "wf1"}
	repo.workflows["wf2"] = &workflow.Workflow{Name: "wf2"}

	svc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, newMockExpressionValidator())

	names, err := svc.ListWorkflows(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("expected 2 workflows, got %d", len(names))
	}
}

func TestWorkflowServiceGetWorkflow(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["test"] = &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeTerminal},
		},
	}

	svc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, newMockExpressionValidator())

	wf, err := svc.GetWorkflow(context.Background(), "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if wf == nil {
		t.Fatal("expected workflow, got nil")
	}
	if wf.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", wf.Name)
	}
}

func TestWorkflowServiceGetWorkflowNotFound(t *testing.T) {
	svc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, newMockExpressionValidator())

	wf, err := svc.GetWorkflow(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent workflow")
	}
	if wf != nil {
		t.Error("expected nil workflow when error occurs")
	}
	if err != nil && !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error message, got: %v", err)
	}
}

func TestWorkflowServiceValidateWorkflow(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["valid"] = &workflow.Workflow{
		Name:    "valid",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeTerminal},
		},
	}
	repo.workflows["invalid"] = &workflow.Workflow{
		Name: "invalid",
		// missing Initial - should fail validation
	}

	svc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, newMockExpressionValidator())

	// Valid workflow
	err := svc.ValidateWorkflow(context.Background(), "valid")
	if err != nil {
		t.Errorf("expected valid workflow to pass, got error: %v", err)
	}

	// Invalid workflow
	err = svc.ValidateWorkflow(context.Background(), "invalid")
	if err == nil {
		t.Error("expected invalid workflow to fail validation")
	}
}

func TestWorkflowServiceWorkflowExists(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["exists"] = &workflow.Workflow{Name: "exists"}

	svc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, newMockExpressionValidator())

	exists, err := svc.WorkflowExists(context.Background(), "exists")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected workflow to exist")
	}

	exists, err = svc.WorkflowExists(context.Background(), "nonexistent")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected workflow to not exist")
	}
}

// mockRepo is a repository mock with explicit name ordering for ListAllWorkflows tests.
type mockRepo struct {
	workflows map[string]*workflow.Workflow
	names     []string
}

func (m *mockRepo) Load(_ context.Context, name string) (*workflow.Workflow, error) {
	if wf, ok := m.workflows[name]; ok {
		return wf, nil
	}
	return nil, fmt.Errorf("workflow not found: %s", name)
}

func (m *mockRepo) List(_ context.Context) ([]string, error) {
	return m.names, nil
}

func (m *mockRepo) Exists(_ context.Context, name string) (bool, error) {
	_, ok := m.workflows[name]
	return ok, nil
}

// noopValidator is an ExpressionValidator that always returns nil.
type noopValidator struct{}

func (n *noopValidator) Compile(_ string) error { return nil }

// mockPackDiscoverer is a PackDiscoverer that returns pre-configured entries.
type mockPackDiscoverer struct {
	entries []workflow.WorkflowEntry
	err     error
}

func (m *mockPackDiscoverer) DiscoverWorkflows(_ context.Context) ([]workflow.WorkflowEntry, error) {
	return m.entries, m.err
}

func (m *mockPackDiscoverer) LoadWorkflow(_ context.Context, _, _ string) (*workflow.Workflow, error) {
	return nil, fmt.Errorf("not implemented in mock")
}

func TestWorkflowServiceListAllWorkflows_WithoutPacks(t *testing.T) {
	repo := &mockRepo{
		workflows: map[string]*workflow.Workflow{
			"wf-a": {Name: "wf-a", Description: "First", Version: "1.0"},
			"wf-b": {Name: "wf-b", Description: "Second"},
		},
		names: []string{"wf-a", "wf-b"},
	}
	svc := application.NewWorkflowService(repo, nil, nil, nil, &noopValidator{})

	entries, err := svc.ListAllWorkflows(context.Background())

	require.NoError(t, err)
	assert.Len(t, entries, 2)
	assert.Equal(t, "wf-a", entries[0].Name)
	assert.Equal(t, "1.0", entries[0].Version)
	assert.Equal(t, "First", entries[0].Description)
}

func TestWorkflowServiceListAllWorkflows_WithPacks(t *testing.T) {
	repo := &mockRepo{
		workflows: map[string]*workflow.Workflow{
			"local-wf": {Name: "local-wf", Description: "Local"},
		},
		names: []string{"local-wf"},
	}
	packs := &mockPackDiscoverer{
		entries: []workflow.WorkflowEntry{
			{Name: "mypack/deploy", Source: "pack", Version: "2.0", Description: "Deploy it"},
		},
	}
	svc := application.NewWorkflowService(repo, nil, nil, nil, &noopValidator{})
	svc.SetPackDiscoverer(packs)

	entries, err := svc.ListAllWorkflows(context.Background())

	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "local-wf", entries[0].Name)
	assert.Equal(t, "mypack/deploy", entries[1].Name)
	assert.Equal(t, "pack", entries[1].Source)
}

func TestWorkflowServiceListAllWorkflows_PackErrorDoesNotBlock(t *testing.T) {
	repo := &mockRepo{
		workflows: map[string]*workflow.Workflow{
			"wf": {Name: "wf"},
		},
		names: []string{"wf"},
	}
	packs := &mockPackDiscoverer{err: fmt.Errorf("disk error")}
	svc := application.NewWorkflowService(repo, nil, nil, nil, &noopValidator{})
	svc.SetPackDiscoverer(packs)

	entries, err := svc.ListAllWorkflows(context.Background())

	require.NoError(t, err)
	assert.Len(t, entries, 1, "pack errors should not block regular workflow listing")
}
