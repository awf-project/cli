package builders

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/transcript"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
)

// CancellationHarness exposes the real adapter and observations at its command boundary.
type CancellationHarness struct {
	Adapter  *application.Adapter
	Executor *CancellationExecutor
	Logger   *CancellationLogger
}

// CancellationExecutor is the owned execution boundary used by the
// cancellation contract test. It does no synthetic facade work: the production Adapter,
// RunSession, and ExecutionService own the lifecycle above it.
type CancellationExecutor struct {
	Started  chan struct{}
	Canceled chan error
	once     sync.Once
}

type cancellationLogSink struct {
	mu      sync.Mutex
	records []string
}

// CancellationLogger captures diagnostics across derived logger contexts.
type CancellationLogger struct {
	sink    *cancellationLogSink
	context map[string]any
}

func newCancellationLogger() *CancellationLogger {
	return &CancellationLogger{sink: &cancellationLogSink{}}
}

func (l *CancellationLogger) Debug(msg string, fields ...any) {
	l.record("debug", msg, fields)
}

func (l *CancellationLogger) Info(msg string, fields ...any) {
	l.record("info", msg, fields)
}

func (l *CancellationLogger) Warn(msg string, fields ...any) {
	l.record("warn", msg, fields)
}

func (l *CancellationLogger) Error(msg string, fields ...any) {
	l.record("error", msg, fields)
}

func (l *CancellationLogger) WithContext(ctx map[string]any) ports.Logger {
	merged := make(map[string]any, len(l.context)+len(ctx))
	for key, value := range l.context {
		merged[key] = value
	}
	for key, value := range ctx {
		merged[key] = value
	}
	return &CancellationLogger{sink: l.sink, context: merged}
}

func (l *CancellationLogger) record(level, msg string, fields []any) {
	l.sink.mu.Lock()
	defer l.sink.mu.Unlock()
	l.sink.records = append(l.sink.records, fmt.Sprint(level, " ", msg, " ", l.context, " ", fields))
}

// Snapshot returns a thread-safe copy of the captured diagnostic text.
func (l *CancellationLogger) Snapshot() string {
	l.sink.mu.Lock()
	defer l.sink.mu.Unlock()
	return strings.Join(l.sink.records, "\n")
}

func newCancellationExecutor() *CancellationExecutor {
	return &CancellationExecutor{
		Started:  make(chan struct{}),
		Canceled: make(chan error, 1),
	}
}

func (e *CancellationExecutor) Execute(ctx context.Context, _ *ports.Command) (*ports.CommandResult, error) {
	e.once.Do(func() { close(e.Started) })
	<-ctx.Done()
	e.Canceled <- ctx.Err()
	return nil, fmt.Errorf("%w", ctx.Err())
}

// NewCancellationHarness builds real application execution around a blocking command.
// API and TUI tests share this boundary while asserting their own interface behavior.
func NewCancellationHarness(workflowName string) *CancellationHarness {
	repo := testmocks.NewMockWorkflowRepository()
	repo.AddWorkflow(workflowName, &workflow.Workflow{
		Name:    workflowName,
		Initial: "wait-for-cancel",
		Steps: map[string]*workflow.Step{
			"wait-for-cancel": {
				Name:      "wait-for-cancel",
				Type:      workflow.StepTypeCommand,
				Command:   "owned-adapter-cancellation-boundary",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	})

	executor := newCancellationExecutor()
	logger := newCancellationLogger()
	workflowSvc := application.NewWorkflowService(repo, nil, nil, nil, nil)
	executionSvc := application.NewExecutionService(
		workflowSvc,
		executor,
		nil,
		testmocks.NewMockStateStore(),
		logger,
		interpolation.NewTemplateResolver(),
		nil,
	)
	adapter := application.NewAdapter(
		workflowSvc,
		executionSvc,
		&application.HistoryService{},
		application.NewResolver(nil, repo),
		transcript.NewNopRecorder(),
		application.NewSessionRegistry(),
	)
	return &CancellationHarness{Adapter: adapter, Executor: executor, Logger: logger}
}
