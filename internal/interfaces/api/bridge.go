package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// ActiveExecution holds the runtime state of an async workflow execution.
type ActiveExecution struct {
	ExecutionID      string
	WorkflowName     string
	Ctx              context.Context
	Cancel           context.CancelFunc
	ExecutionContext *workflow.ExecutionContext
	// StartedAt is stamped when a facade session is tracked. The facade path uses a
	// NopRecorder, so the session emits no run.started event to derive a start time from;
	// this field is the authoritative start time for facade-tracked executions.
	StartedAt time.Time
}

// Bridge tracks the runtime metadata of async workflow executions for the HTTP layer.
//
// It no longer owns any read or execution port: workflow listing/get/validate and history
// queries route through ports.WorkflowFacade / ports.WorkflowReader (F108 read-path
// consolidation), and execution routes through ports.WorkflowFacade.Run/Resume. Bridge's
// sole remaining responsibility is the activeExecutions map that backs
// GET/DELETE /api/executions and graceful shutdown.
type Bridge struct {
	activeExecutions sync.Map
}

// NewBridge creates an empty Bridge. It takes no dependencies because all workflow and
// history access is now served by the facade/reader wired on NewServer; the Bridge only
// holds in-flight execution metadata populated via TrackFacadeSession.
func NewBridge() *Bridge {
	return &Bridge{}
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

// TrackFacadeSession records a metadata entry for a facade-driven execution so the
// List/Get/Cancel handlers keep working while the live RunSession itself lives in the
// application.SessionRegistry (A4 / R5). The entry is keyed by the session's own ID()
// so that GET /api/executions/{id} and SessionRegistry.Get(execID) resolve the same key.
//
// Cancel is wired to session.Close() (idempotent) so DELETE /api/executions/{id} tears
// down the facade session; Bridge.Shutdown also invokes Cancel on every tracked entry.
//
// Ctx is a dedicated cancelable context for this ActiveExecution entry. The facade owns
// the real execution goroutine and its session context; this Ctx is intentionally
// decoupled from it (ports.RunSession does not expose its internal context). Cancel calls
// both session.Close() (to stop execution) and the context cancel function (so that
// ae.Ctx.Err() returns context.Canceled after Cancel() — required by M7 contract).
// Done is pre-closed since the facade — not the Bridge — owns the completion signal.
func (b *Bridge) TrackFacadeSession(session ports.RunSession, workflowName string) *ActiveExecution {
	ctx, cancelCtx := context.WithCancel(context.Background()) //nolint:gocritic // cancelCtx is captured in ae.Cancel and called by Delete/Shutdown; intentionally not deferred here
	ae := &ActiveExecution{
		ExecutionID:  session.ID(),
		WorkflowName: workflowName,
		Ctx:          ctx,
		Cancel: func() {
			type cancellableSession interface {
				CancelWithEvent(reason string) error
			}
			if cancellable, ok := session.(cancellableSession); ok {
				_ = cancellable.CancelWithEvent("cancelled") //nolint:errcheck // cancellation is best-effort; Close is still idempotent
			} else {
				_ = session.Close() //nolint:errcheck // Close is idempotent and always returns nil
			}
			cancelCtx()
		},
		StartedAt: time.Now(),
	}
	b.activeExecutions.Store(session.ID(), ae)
	return ae
}

// Shutdown cancels every execution that is still tracked in activeExecutions.
// It must be called after the HTTP server has stopped accepting requests so
// that no new executions can be started concurrently.  Calling Shutdown more
// than once is safe — context.CancelFunc is idempotent.
func (b *Bridge) Shutdown() {
	b.activeExecutions.Range(func(_, val any) bool {
		ae := val.(*ActiveExecution) //nolint:forcetypeassert,errcheck // sync.Map only stores *ActiveExecution
		ae.Cancel()
		return true
	})
}
