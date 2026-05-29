package api

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// WorkflowLister is the driven port for listing and loading workflow definitions.
// It is satisfied by *application.WorkflowService.
type WorkflowLister interface {
	ListAllWorkflows(ctx context.Context) ([]workflow.WorkflowEntry, error)
	GetWorkflow(ctx context.Context, name string) (*workflow.Workflow, error)
	ValidateWorkflow(ctx context.Context, name string) error
}

// WorkflowRunner is the driven port for executing workflows.
// It is satisfied by *application.ExecutionService.
type WorkflowRunner interface {
	RunWorkflowAsync(ctx context.Context, wf *workflow.Workflow, inputs map[string]any) (*workflow.ExecutionContext, <-chan error, error)
}

// WorkflowResumer is the driven port for resuming interrupted workflow executions.
// Declared separately from WorkflowRunner per Interface Segregation Principle.
// It is satisfied by *application.ExecutionService.
type WorkflowResumer interface {
	Resume(ctx context.Context, workflowID string, inputOverrides map[string]any, fromStep string) (*workflow.ExecutionContext, error)
}

// HistoryProvider is the driven port for querying execution history.
// It is satisfied by *application.HistoryService.
type HistoryProvider interface {
	List(ctx context.Context, filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error)
	GetStats(ctx context.Context, filter *workflow.HistoryFilter) (*workflow.HistoryStats, error)
}

// ActiveExecution holds the runtime state of an async workflow execution.
type ActiveExecution struct {
	ExecutionID      string
	WorkflowName     string
	Ctx              context.Context
	Cancel           context.CancelFunc
	ExecutionContext *workflow.ExecutionContext
	Done             <-chan error
}

// Bridge adapts application service interfaces to HTTP handlers.
type Bridge struct {
	workflows        WorkflowLister
	runner           WorkflowRunner
	history          HistoryProvider
	resumer          WorkflowResumer
	activeExecutions sync.Map
}

// NewBridge creates a Bridge wiring the given service interface implementations.
// runner may be nil; calling StartExecution on a nil runner returns a descriptive error.
// workflows and history must not be nil; handlers accessing them will panic otherwise.
func NewBridge(workflows WorkflowLister, runner WorkflowRunner, history HistoryProvider) *Bridge {
	return &Bridge{
		workflows: workflows,
		runner:    runner,
		history:   history,
	}
}

// StartExecution starts an async workflow execution and tracks it.
// It derives a new execution ID (UUID v4), creates a cancellable child context,
// calls runner.RunWorkflowAsync, stores the ActiveExecution in the sync.Map,
// and spawns a cleanup goroutine that removes the entry once Done closes.
func (b *Bridge) StartExecution(ctx context.Context, wf *workflow.Workflow, inputs map[string]any) (string, *ActiveExecution, error) {
	if b.runner == nil {
		return "", nil, errors.New("workflow runner is not available")
	}

	// Decouple execution lifetime from the HTTP request context so the workflow
	// survives after the /run response is sent and the request context closes.
	childCtx, cancel := context.WithCancel(context.Background())

	execCtx, done, err := b.runner.RunWorkflowAsync(childCtx, wf, inputs)
	if err != nil {
		cancel()
		return "", nil, err
	}

	id := uuid.NewString()
	ae := &ActiveExecution{
		ExecutionID:      id,
		WorkflowName:     wf.Name,
		Ctx:              childCtx,
		Cancel:           cancel,
		ExecutionContext: execCtx,
		Done:             done,
	}
	b.activeExecutions.Store(id, ae)

	go func() {
		// Drain all values and wait for done to close before removing the entry.
		for range done { //nolint:revive // empty body intentional: drain only
		}
		b.activeExecutions.Delete(id)
	}()

	return id, ae, nil
}

// GetExecution returns the active execution by ID.
// Returns ok=false if not found.
func (b *Bridge) GetExecution(id string) (*ActiveExecution, bool) {
	val, ok := b.activeExecutions.Load(id)
	if !ok {
		return nil, false
	}
	return val.(*ActiveExecution), true //nolint:forcetypeassert,errcheck // sync.Map only stores *ActiveExecution
}

// CancelExecution cancels the execution by ID.
// Returns a descriptive error if not found. Idempotent.
func (b *Bridge) CancelExecution(id string) error {
	val, ok := b.activeExecutions.Load(id)
	if !ok {
		return fmt.Errorf("execution not found: %s", id)
	}
	val.(*ActiveExecution).Cancel() //nolint:forcetypeassert,errcheck // sync.Map only stores *ActiveExecution
	return nil
}

// ListExecutions returns all active executions currently in the map.
// Order is unspecified.
func (b *Bridge) ListExecutions() []*ActiveExecution {
	var result []*ActiveExecution
	b.activeExecutions.Range(func(_, val any) bool {
		result = append(result, val.(*ActiveExecution)) //nolint:forcetypeassert,errcheck // sync.Map only stores *ActiveExecution
		return true
	})
	return result
}

// TrackResumedExecution wraps a synchronously-resumed ExecutionContext in an
// ActiveExecution, assigns it a new UUID, stores it in activeExecutions, and
// returns the assigned ID. Because resume is synchronous the execution is
// already complete when this returns; the entry is kept intentionally so that
// subsequent GET /api/executions/{id} (and DELETE / SSE endpoints) can serve
// the terminal state to clients querying the just-resumed execution. Without
// this persistence the /resume handler would return an ID that immediately
// 404s on read. Eviction/TTL of completed entries is a separate concern.
func (b *Bridge) TrackResumedExecution(execCtx *workflow.ExecutionContext) string {
	id := uuid.NewString()
	closed := make(chan error)
	close(closed)

	ae := &ActiveExecution{
		ExecutionID:      id,
		WorkflowName:     execCtx.WorkflowName,
		Ctx:              context.Background(),
		Cancel:           func() {},
		ExecutionContext: execCtx,
		Done:             closed,
	}
	b.activeExecutions.Store(id, ae)
	return id
}

// SetResumer wires the optional WorkflowResumer dependency.
func (b *Bridge) SetResumer(r WorkflowResumer) {
	b.resumer = r
}
