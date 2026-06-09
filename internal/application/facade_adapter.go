package application

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/display"
	"github.com/google/uuid"
)

// compile-time check: Adapter implements ports.WorkflowFacade (SC-001, D15)
var _ ports.WorkflowFacade = (*Adapter)(nil)

// Adapter implements ports.WorkflowFacade by composing services without modifying them (D17).
// It is the sole caller of ports.Recorder.Subscribe — one Subscribe per RunSession (D15, SC-001).
type Adapter struct {
	workflowSvc  *WorkflowService
	executionSvc *ExecutionService
	historySvc   *HistoryService
	resolver     *Resolver
	recorder     ports.Recorder
	registry     *SessionRegistry

	// 4 edge bridges (D16) — all optional, nil-safe
	eventPublisher  ports.EventPublisher
	outputWriters   *OutputWriterPair
	displayRenderer display.EventRenderer
	userInputReader ports.UserInputReader
}

func NewAdapter(
	workflowSvc *WorkflowService,
	executionSvc *ExecutionService,
	historySvc *HistoryService,
	resolver *Resolver,
	recorder ports.Recorder,
	registry *SessionRegistry,
) *Adapter {
	return &Adapter{
		workflowSvc:  workflowSvc,
		executionSvc: executionSvc,
		historySvc:   historySvc,
		resolver:     resolver,
		recorder:     recorder,
		registry:     registry,
	}
}

func (a *Adapter) SetEventPublisher(p ports.EventPublisher) {
	a.eventPublisher = p
}

func (a *Adapter) SetOutputWriters(stdout, stderr io.Writer) {
	a.outputWriters = &OutputWriterPair{Stdout: stdout, Stderr: stderr}
}

func (a *Adapter) SetDisplayRenderer(r display.EventRenderer) {
	a.displayRenderer = r
}

func (a *Adapter) SetUserInputReader(r ports.UserInputReader) {
	a.userInputReader = r
}

// List returns every discoverable workflow (local, global, env, and packs) via the
// workflow service, mapped to lightweight summaries.
func (a *Adapter) List(ctx context.Context) ([]ports.WorkflowSummary, error) {
	entries, err := a.workflowSvc.ListAllWorkflows(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing workflows: %w", err)
	}
	summaries := make([]ports.WorkflowSummary, len(entries))
	for i := range entries {
		summaries[i] = ports.WorkflowSummary{
			Name:        entries[i].Name,
			Description: entries[i].Description,
			Version:     entries[i].Version,
		}
	}
	return summaries, nil
}

// Validate resolves the canonical identifier (FR-019) and reports validity.
// A resolver rejection (e.g. empty identifier) propagates as the validation error.
func (a *Adapter) Validate(ctx context.Context, req ports.RunRequest) (ports.ValidationReport, error) {
	if a.resolver != nil {
		if _, err := a.resolver.Resolve(ctx, req.Identifier); err != nil {
			return ports.ValidationReport{}, err
		}
	}
	return ports.ValidationReport{}, nil
}

func (a *Adapter) Status(_ context.Context, _ string) (ports.RunStatus, error) {
	return ports.RunStatus{}, nil
}

// History returns past run records from the history service, mapped to facade records.
func (a *Adapter) History(ctx context.Context, filter ports.HistoryFilter) ([]ports.RunRecord, error) { //nolint:gocritic // hugeParam: interface contract requires value type; pointer would break WorkflowFacade conformance
	records, err := a.historySvc.List(ctx, &workflow.HistoryFilter{
		WorkflowName: filter.WorkflowName,
		Status:       filter.Status,
		Since:        filter.Since,
		Until:        filter.Until,
		Limit:        filter.Limit,
	})
	if err != nil {
		return nil, fmt.Errorf("listing history: %w", err)
	}
	out := make([]ports.RunRecord, len(records))
	for i, rec := range records {
		out[i] = ports.RunRecord{
			RunID:        rec.ID,
			WorkflowName: rec.WorkflowName,
			Status:       rec.Status,
			StartedAt:    rec.StartedAt,
			CompletedAt:  rec.CompletedAt,
			DurationMs:   rec.DurationMs,
			ErrorMessage: rec.ErrorMessage,
		}
	}
	return out, nil
}

// Run resolves the canonical identifier (FR-019), creates a RunSession, subscribes to
// the Recorder exactly once (D15, SC-001), drives execution, and projects transcript
// events into the session. A resolver rejection propagates synchronously without leaking
// a session; execution success/failure is reported via the terminal event.
func (a *Adapter) Run(ctx context.Context, req ports.RunRequest) (ports.RunSession, error) {
	var wf *workflow.Workflow
	if a.resolver != nil {
		resolved, err := a.resolver.Resolve(ctx, req.Identifier)
		if err != nil {
			return nil, err
		}
		wf = resolved
	}
	return a.newSession(ctx, req, wf)
}

func (a *Adapter) Resume(ctx context.Context, _ string) (ports.RunSession, error) {
	return a.newSession(ctx, ports.RunRequest{}, nil)
}

// newSession allocates a RunSession, registers it, wires the sole Recorder subscription
// (D15, SC-001), and spawns a goroutine that runs execution and projects transcript events.
func (a *Adapter) newSession(ctx context.Context, req ports.RunRequest, wf *workflow.Workflow) (*RunSession, error) {
	id := uuid.New().String()
	session := newRunSession(id, ctx, 0)

	if err := a.registry.Add(session); err != nil {
		return nil, err
	}

	// Register a close hook so that registry.Remove is called synchronously inside
	// session.Close(), regardless of who triggers it (manual caller or goroutine).
	// This covers BUG #7 (TestFacadeAdapter_RegistryRemovesSessionOnClose) without
	// requiring the goroutine to have been scheduled first.
	session.onClose = func() { a.registry.Remove(session.id) }

	// SC-001: exactly one Subscribe call per RunSession — only this method calls Subscribe.
	sub, cancelSub := a.recorder.Subscribe()

	go func() {
		// On exit: cancelSub() releases the recorder subscription; session.Close() seals
		// the events channel (terminating consumers' range loops) and fires onClose to
		// evict the session from the registry (BUG #7). Both are idempotent.
		defer cancelSub()
		defer session.Close() //nolint:errcheck // Close always returns nil

		// Execution is started here (not before the goroutine) so the recorder's already-
		// buffered transcript events keep select priority before execDone becomes ready.
		execDone := a.startExecution(ctx, id, req, wf)

		for {
			select {
			case ev, ok := <-sub:
				if !ok {
					// Recorder stopped before execution completed — exit and clean up.
					return
				}
				if fEv, err := ProjectEvent(ev); err == nil {
					session.appendEvent(fEv)
				}

			case execErr := <-execDone:
				// Execution finished: project any already-buffered transcript events
				// before the terminal event so ordering is deterministic, then continue
				// draining late events in the background until the deferred cancelSub.
				a.drainBuffered(session, sub)
				a.emitTerminalEvent(session, execErr)
				go a.drainTranscript(session, sub)
				return

			case <-session.ctx.Done():
				// Session closed manually before execution finished — exit and clean up.
				return
			}
		}
	}()

	return session, nil
}

// drainBuffered projects every transcript event currently buffered on sub (non-blocking)
// into the session. Used before emitting the terminal event so buffered events keep their
// position ahead of EventWorkflowCompleted/Failed.
func (a *Adapter) drainBuffered(session *RunSession, sub <-chan transcript.ExchangeEvent) {
	for {
		select {
		case ev, ok := <-sub:
			if !ok {
				return
			}
			if fEv, err := ProjectEvent(ev); err == nil {
				session.appendEvent(fEv)
			}
		default:
			return
		}
	}
}

// startExecution drives the workflow execution in a goroutine and reports completion on
// the returned channel. A nil workflow (no resolver configured) is a no-op success; a
// panic from the execution service (e.g. unconfigured dependencies) is captured as error.
func (a *Adapter) startExecution(ctx context.Context, id string, req ports.RunRequest, wf *workflow.Workflow) <-chan error {
	execDone := make(chan error, 1)
	go func() {
		var execErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					execErr = fmt.Errorf("execution panic: %v", r)
				}
			}()
			if wf != nil {
				_, execErr = a.executionSvc.RunWithWorkflowAndRunID(ctx, wf, req.Inputs, id)
			}
		}()
		execDone <- execErr
	}()
	return execDone
}

// drainTranscript projects any remaining transcript events into the session after the
// terminal event has been emitted, until the recorder subscription is cancelled.
func (a *Adapter) drainTranscript(session *RunSession, sub <-chan transcript.ExchangeEvent) {
	for ev := range sub {
		if fEv, err := ProjectEvent(ev); err == nil {
			session.appendEvent(fEv)
		}
	}
}

// emitTerminalEvent appends the single terminal event for the run: EventWorkflowCompleted
// on success, or EventWorkflowFailed (with the mapped error code) on failure (Criteria #6/#7).
func (a *Adapter) emitTerminalEvent(session *RunSession, execErr error) {
	kind := ports.EventWorkflowCompleted
	if execErr != nil {
		kind = ports.EventWorkflowFailed
		session.setErr(execErr)
	}
	session.appendEvent(ports.Event{
		Kind:      kind,
		RunID:     session.id,
		Timestamp: time.Now(),
		Payload:   MapError(execErr),
	})
}
