package api

import (
	"context"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// historyBackend is the subset of the legacy history provider used by the read facade fake.
// Both mockHistoryProvider and capturingHistoryProvider satisfy it, so history-handler tests
// keep their existing fixtures after the handlers were rewired off the Bridge.
type historyBackend interface {
	List(ctx context.Context, filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error)
	GetStats(ctx context.Context, filter *workflow.HistoryFilter) (*workflow.HistoryStats, error)
}

// readFacade adapts the existing workflow/history test mocks to the read surface of
// ports.WorkflowFacade and ports.WorkflowReader. It lets the workflow and history handler
// tests exercise the rewired handlers (which now consume the facade/reader instead of the
// Bridge) without rewriting every fixture. Execution methods are inert stubs — these tests
// never drive Run/Resume/Status.
type readFacade struct {
	lister  *mockWorkflowLister
	history historyBackend
}

var (
	_ ports.WorkflowFacade = (*readFacade)(nil)
	_ ports.WorkflowReader = (*readFacade)(nil)
)

func (f *readFacade) List(ctx context.Context) ([]ports.WorkflowSummary, error) {
	entries, err := f.lister.ListAllWorkflows(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ports.WorkflowSummary, len(entries))
	for i := range entries {
		out[i] = ports.WorkflowSummary{
			Name:        entries[i].Name,
			Scope:       entries[i].Scope,
			Workflow:    entries[i].Workflow,
			Version:     entries[i].Version,
			Description: entries[i].Description,
		}
	}
	return out, nil
}

func (f *readFacade) Validate(ctx context.Context, req ports.RunRequest) (ports.ValidationReport, error) {
	// The handler probes existence (via GetWorkflow) before calling Validate, so a missing
	// workflow never reaches here; this only models the valid/invalid distinction.
	if verr := f.lister.ValidateWorkflow(ctx, req.Identifier); verr != nil {
		return ports.ValidationReport{Valid: false, Errors: []ports.ValidationError{{Message: verr.Error()}}}, nil
	}
	return ports.ValidationReport{Valid: true}, nil
}

func (f *readFacade) GetWorkflow(ctx context.Context, identifier string) (*workflow.Workflow, error) {
	return f.lister.GetWorkflow(ctx, identifier)
}

func (f *readFacade) History(ctx context.Context, filter ports.HistoryFilter) ([]ports.RunRecord, error) {
	records, err := f.history.List(ctx, toWorkflowFilter(filter))
	if err != nil {
		return nil, err
	}
	out := make([]ports.RunRecord, len(records))
	for i, r := range records {
		out[i] = ports.RunRecord{
			RunID:        r.ID,
			WorkflowName: r.WorkflowName,
			Status:       ports.RunState(r.Status),
			StartedAt:    r.StartedAt,
			CompletedAt:  r.CompletedAt,
			DurationMs:   r.DurationMs,
			ErrorMessage: r.ErrorMessage,
		}
	}
	return out, nil
}

func (f *readFacade) HistoryStats(ctx context.Context, filter ports.HistoryFilter) (*workflow.HistoryStats, error) {
	return f.history.GetStats(ctx, toWorkflowFilter(filter))
}

// --- inert execution stubs (read-only fake) ---

func (f *readFacade) Status(context.Context, string) (ports.RunStatus, error) {
	return ports.RunStatus{}, nil
}

// Run/Resume are not exercised by the read-only handler tests; panic on accidental use so a
// future test that wires this fake into an execution path fails loudly instead of nil-deref'ing.
func (f *readFacade) Run(context.Context, ports.RunRequest) (ports.RunSession, error) {
	panic("readFacade.Run must not be called: this fake only models the read surface")
}

func (f *readFacade) Resume(context.Context, ports.ResumeRequest) (ports.RunSession, error) {
	panic("readFacade.Resume must not be called: this fake only models the read surface")
}

func toWorkflowFilter(f ports.HistoryFilter) *workflow.HistoryFilter {
	return &workflow.HistoryFilter{
		WorkflowName: f.WorkflowName,
		Status:       f.Status,
		Since:        f.Since,
		Until:        f.Until,
		Limit:        f.Limit,
	}
}
