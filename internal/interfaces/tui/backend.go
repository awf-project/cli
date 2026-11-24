package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

// Backend is the only surface through which the TUI screens reach the
// application layer. Every method returns a tea.Cmd so that screens stay free
// of blocking calls, and tests can substitute a fake without a facade.
//
// *Bridge is the in-process implementation backed by ports.WorkflowFacade.
// A remote implementation (for example over the HTTP API) can be added later
// without touching the screens.
type Backend interface {
	// LoadWorkflows emits WorkflowsLoadedMsg, or ErrMsg on failure.
	LoadWorkflows(ctx context.Context) tea.Cmd
	// LoadHistory emits HistoryLoadedMsg, or ErrMsg on failure.
	LoadHistory(ctx context.Context) tea.Cmd
	// ValidateWorkflow emits ValidationResultMsg.
	ValidateWorkflow(ctx context.Context, name string) tea.Cmd
	// LoadRunDetail emits RunDetailLoadedMsg with the persisted per-step detail of a run.
	LoadRunDetail(ctx context.Context, recordID, workflowID string) tea.Cmd
	// RunWorkflowViaFacade starts a run and emits ExecutionStartedMsg, or ErrMsg.
	RunWorkflowViaFacade(ctx context.Context, name string, inputs map[string]any) tea.Cmd
}

var _ Backend = (*Bridge)(nil)
