package application_test

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/awf-project/cli/internal/domain/workflow"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/display"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// closableHistoryStore wraps MockHistoryStore and tracks Close() calls.
// The real MockHistoryStore already implements io.Closer, so this verifies
// that the cleanup function actually invokes Close().
type closableHistoryStore struct {
	*testmocks.MockHistoryStore
	mu     sync.Mutex
	closed bool
}

func newClosableHistoryStore() *closableHistoryStore {
	return &closableHistoryStore{
		MockHistoryStore: testmocks.NewMockHistoryStore(),
	}
}

// Close records that the store was closed.
func (s *closableHistoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// IsClosed returns whether Close was called.
func (s *closableHistoryStore) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// Compile-time check: closableHistoryStore satisfies both ports.HistoryStore and io.Closer.
var (
	_ ports.HistoryStore = (*closableHistoryStore)(nil)
	_ io.Closer          = (*closableHistoryStore)(nil)
)

// stubPluginChecker implements application.PluginStateChecker for tests.
type stubPluginChecker struct {
	enabled map[string]bool
}

func newStubPluginChecker(enabled map[string]bool) *stubPluginChecker {
	return &stubPluginChecker{enabled: enabled}
}

func (s *stubPluginChecker) IsPluginEnabled(name string) bool {
	if v, ok := s.enabled[name]; ok {
		return v
	}
	return true
}

// buildMinimalSetup creates an ExecutionSetup with the minimum required dependencies.
func buildMinimalSetup(opts ...application.SetupOption) *application.ExecutionSetup {
	repo := testmocks.NewMockWorkflowRepository()
	store := testmocks.NewMockStateStore()
	executor := testmocks.NewMockCommandExecutor()
	logger := testmocks.NewMockLogger()
	return application.NewExecutionSetup(repo, store, executor, logger, opts...)
}

func TestBuild_MinimalConfig(t *testing.T) {
	setup := buildMinimalSetup()

	result, err := setup.Build(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.ExecService, "ExecService must be non-nil with minimal config")
	assert.NotNil(t, result.WorkflowSvc, "WorkflowSvc must be non-nil with minimal config")
	assert.Nil(t, result.HistorySvc, "HistorySvc must be nil when no HistoryStore is provided")
	assert.NotNil(t, result.Cleanup, "Cleanup func must always be non-nil")
}

func TestBuild_Cleanup_NoOp_WithoutHistoryStore(t *testing.T) {
	setup := buildMinimalSetup()

	result, err := setup.Build(context.Background())
	require.NoError(t, err)

	// Calling Cleanup when no io.Closer was registered must not panic.
	assert.NotPanics(t, result.Cleanup)
}

func TestBuild_PluginGating_NilChecker(t *testing.T) {
	// When no PluginStateChecker is provided (nil checker), all built-in providers
	// are enabled — this is the backward-compatible default.
	setup := buildMinimalSetup() // no WithPluginState option

	result, err := setup.Build(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.ExecService, "service must be built when all providers are enabled by default")
}

func TestBuild_PluginGating_AllDisabled(t *testing.T) {
	checker := newStubPluginChecker(map[string]bool{
		"github": false,
		"notify": false,
		"http":   false,
	})
	setup := buildMinimalSetup(application.WithPluginState(checker))

	result, err := setup.Build(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	// Even with all built-in providers disabled, Build must succeed because
	// CompositeOperationProvider supports an empty provider list.
	assert.NotNil(t, result.ExecService)
}

func TestBuild_Cleanup_ClosesHistoryStore(t *testing.T) {
	store := newClosableHistoryStore()

	setup := buildMinimalSetup(application.WithHistoryStore(store))

	result, err := setup.Build(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.HistorySvc, "HistorySvc must be non-nil when HistoryStore is provided")

	assert.False(t, store.IsClosed(), "store must not be closed before Cleanup()")
	result.Cleanup()
	assert.True(t, store.IsClosed(), "Cleanup() must call Close() on the HistoryStore")
}

func TestBuild_Cleanup_CalledMultipleTimes_DoesNotPanic(t *testing.T) {
	store := newClosableHistoryStore()
	setup := buildMinimalSetup(application.WithHistoryStore(store))

	result, err := setup.Build(context.Background())
	require.NoError(t, err)

	// Cleanup is safe to call multiple times (defensive contract).
	assert.NotPanics(t, func() {
		result.Cleanup()
		result.Cleanup()
	})
}

func TestBuild_WithHistoryStore_PopulatesHistorySvc(t *testing.T) {
	store := testmocks.NewMockHistoryStore()
	setup := buildMinimalSetup(application.WithHistoryStore(store))

	result, err := setup.Build(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, result.HistorySvc)
}

func TestBuild_WithOutputWriters_DoesNotError(t *testing.T) {
	var stdout, stderr nopWriter
	setup := buildMinimalSetup(application.WithOutputWriters(&stdout, &stderr))

	result, err := setup.Build(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, result.ExecService)
}

func TestBuild_WithPackContext_DoesNotError(t *testing.T) {
	setup := buildMinimalSetup(application.WithPackContext("my-pack", nil))

	result, err := setup.Build(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, result.ExecService)
}

func TestBuild_WithTemplatePaths_DoesNotError(t *testing.T) {
	setup := buildMinimalSetup(application.WithTemplatePaths([]string{"/nonexistent/path"}))

	result, err := setup.Build(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, result.ExecService)
}

func TestBuild_WithPluginService_DoesNotError(t *testing.T) {
	setup := buildMinimalSetup(application.WithPluginService(nil))

	result, err := setup.Build(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, result.ExecService)
}

func TestMergeInputs_CLIWinsOnConflict(t *testing.T) {
	config := map[string]any{"key": "config-value", "shared": "from-config"}
	cli := map[string]any{"shared": "from-cli", "extra": "cli-only"}

	merged := application.MergeInputs(config, cli)

	assert.Equal(t, "config-value", merged["key"])
	assert.Equal(t, "from-cli", merged["shared"], "CLI value must override config value")
	assert.Equal(t, "cli-only", merged["extra"])
}

func TestMergeInputs_BothNil_ReturnsEmptyMap(t *testing.T) {
	merged := application.MergeInputs(nil, nil)

	assert.NotNil(t, merged)
	assert.Empty(t, merged)
}

func TestMergeInputs_NilConfig_ReturnsCLIOnly(t *testing.T) {
	cli := map[string]any{"key": "value"}

	merged := application.MergeInputs(nil, cli)

	assert.Equal(t, "value", merged["key"])
}

func TestMergeInputs_NilCLI_ReturnsConfigOnly(t *testing.T) {
	config := map[string]any{"key": "value"}

	merged := application.MergeInputs(config, nil)

	assert.Equal(t, "value", merged["key"])
}

func TestMergeInputs_DoesNotMutateInputs(t *testing.T) {
	config := map[string]any{"key": "config"}
	cli := map[string]any{"key": "cli"}

	application.MergeInputs(config, cli)

	assert.Equal(t, "config", config["key"], "config map must not be mutated")
	assert.Equal(t, "cli", cli["key"], "cli map must not be mutated")
}

func TestBuild_WithTracer(t *testing.T) {
	setup := buildMinimalSetup(application.WithTracer(ports.NopTracer{}))

	result, err := setup.Build(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, result.ExecService, "ExecService must be non-nil when a tracer is provided")
}

func TestBuild_WithAuditWriter(t *testing.T) {
	writer := testmocks.NewMockAuditTrailWriter()
	setup := buildMinimalSetup(application.WithAuditWriter(writer))

	result, err := setup.Build(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, result.ExecService, "ExecService must be non-nil when an audit writer is provided")
}

func TestBuild_WithRecorder(t *testing.T) {
	recorder := &mockRecorder{}
	setup := buildMinimalSetup(application.WithRecorder(recorder))

	result, err := setup.Build(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, result.ExecService, "ExecService must be non-nil when a recorder is provided")
}

func TestBuild_PluginGating_DisabledProvider(t *testing.T) {
	// Only notify is enabled; github and http are disabled.
	checker := newStubPluginChecker(map[string]bool{
		"github": false,
		"notify": true,
		"http":   false,
	})
	setup := buildMinimalSetup(application.WithPluginState(checker))

	result, err := setup.Build(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.ExecService, "ExecService must be non-nil with partial provider gating")
}

func TestBuild_WithAllOptions(t *testing.T) {
	historyStore := newClosableHistoryStore()
	var stdout, stderr nopWriter
	checker := newStubPluginChecker(map[string]bool{})
	auditWriter := testmocks.NewMockAuditTrailWriter()

	setup := buildMinimalSetup(
		application.WithTracer(ports.NopTracer{}),
		application.WithAuditWriter(auditWriter),
		application.WithHistoryStore(historyStore),
		application.WithOutputWriters(&stdout, &stderr),
		application.WithPluginState(checker),
		application.WithPackContext("my-pack", nil),
		application.WithTemplatePaths([]string{"/nonexistent/path"}),
		application.WithPluginService(nil),
	)

	result, err := setup.Build(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.ExecService, "ExecService must be non-nil with all options")
	assert.NotNil(t, result.WorkflowSvc, "WorkflowSvc must be non-nil with all options")
	assert.NotNil(t, result.HistorySvc, "HistorySvc must be non-nil when HistoryStore is provided")
	assert.NotNil(t, result.Cleanup, "Cleanup must be non-nil")

	assert.False(t, historyStore.IsClosed(), "store must not be closed before Cleanup()")
	assert.NotPanics(t, result.Cleanup)
	assert.True(t, historyStore.IsClosed(), "Cleanup() must close the HistoryStore")
}

func TestExecutionSetup_WithEventPublisher(t *testing.T) {
	mockPublisher := testmocks.NewMockEventPublisher()

	repo := testmocks.NewMockWorkflowRepository()
	store := testmocks.NewMockStateStore()
	executor := testmocks.NewMockCommandExecutor()
	logger := testmocks.NewMockLogger()

	simpleWf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}
	repo.AddWorkflow("test-workflow", simpleWf)
	executor.SetCommandResult("echo hello", &ports.CommandResult{ExitCode: 0, Stdout: "hello"})

	setup := application.NewExecutionSetup(
		repo,
		store,
		executor,
		logger,
		application.WithEventPublisher(mockPublisher),
	)

	result, err := setup.Build(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.ExecService)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = result.ExecService.Run(ctx, "test-workflow", map[string]any{})
	require.NoError(t, err)

	events := mockPublisher.GetEvents()
	assert.Greater(t, len(events), 0, "MockEventPublisher must receive events after Run()")
}

func TestWithDisplayRendererFactory_Wired(t *testing.T) {
	mockRenderer := func(stepID string) display.EventRenderer {
		return func(events []display.DisplayEvent) {}
	}

	setup := buildMinimalSetup(application.WithDisplayRendererFactory(mockRenderer))

	result, err := setup.Build(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.ExecService)
	// Verify that the setup option was accepted and Build completed without error.
	// The actual renderer wiring is tested at the ExecutionService level.
}

// nopWriter is a no-op io.Writer used in tests.
type nopWriter struct{}

func (n *nopWriter) Write(p []byte) (int, error) { return len(p), nil }

// mockRecorder is a minimal test stub for ports.Recorder.
type mockRecorder struct{}

func (m *mockRecorder) Record(ctx context.Context, event transcript.ExchangeEvent) error {
	return nil
}

func (m *mockRecorder) Subscribe() (<-chan transcript.ExchangeEvent, func()) {
	ch := make(chan transcript.ExchangeEvent)
	return ch, func() { close(ch) }
}

func (m *mockRecorder) Close() error {
	return nil
}
