package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// StreamBuffer is a thread-safe byte buffer implementing io.Writer.
// It captures streaming output from step execution for real-time TUI display.
type StreamBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *StreamBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.buf.Write(p)
	if err != nil {
		return n, fmt.Errorf("stream buffer write: %w", err)
	}
	return n, nil
}

// String returns the accumulated content.
func (s *StreamBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// Len returns the current buffer length.
func (s *StreamBuffer) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Len()
}

// Reset clears the buffer for a new execution.
func (s *StreamBuffer) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf.Reset()
}

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

// HistoryProvider is the driven port for querying execution history.
// It is satisfied by *application.HistoryService.
type HistoryProvider interface {
	List(ctx context.Context, filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error)
	GetStats(ctx context.Context, filter *workflow.HistoryFilter) (*workflow.HistoryStats, error)
}

// Bridge adapts application service interfaces to tea.Cmd factories consumed by Model.
//
// Secret masking is handled at the infrastructure layer (ShellExecutor, Logger) via the
// existing convention of prefixing variable names with SECRET_, API_KEY, or PASSWORD.
// Bridge itself does not re-mask content because it only operates on metadata (IDs,
// names, structs) rather than raw command output.
type Bridge struct {
	workflows WorkflowLister
	runner    WorkflowRunner
	history   HistoryProvider
	stream    *StreamBuffer

	// facade is the optional ports.WorkflowFacade used by RunWorkflowViaFacade.
	// When set, RunWorkflowViaFacade returns an ExecutionStartedMsg that includes
	// a live RunSession whose Events() channel drives the monitoring tab event loop
	// (D27, FR-011) instead of relying solely on the 200 ms polling tick.
	facade ports.WorkflowFacade
}

// NewBridge creates a Bridge wiring the given service interface implementations.
// Any of the three dependencies may be nil; calling the corresponding method
// on a nil dependency returns ErrMsg wrapping a descriptive error.
func NewBridge(workflows WorkflowLister, runner WorkflowRunner, history HistoryProvider) *Bridge {
	return &Bridge{
		workflows: workflows,
		runner:    runner,
		history:   history,
		stream:    &StreamBuffer{},
	}
}

// SetFacade wires a WorkflowFacade into the Bridge so that RunWorkflowViaFacade
// can start event-driven executions via facade.Run. Safe to call after NewBridge;
// a nil facade is accepted and causes RunWorkflowViaFacade to return ErrMsg.
func (b *Bridge) SetFacade(f ports.WorkflowFacade) {
	b.facade = f
}

// Stream returns the shared output stream buffer.
// Pass it to WithOutputWriters so execution output is captured for display.
func (b *Bridge) Stream() *StreamBuffer {
	return b.stream
}

// LoadWorkflows returns a tea.Cmd that fetches all available workflows and emits
// WorkflowsLoadedMsg. Individual workflow load failures are skipped so that one
// broken YAML file does not prevent the entire list from rendering.
func (b *Bridge) LoadWorkflows(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			return ErrMsg{Err: err}
		}

		entries, err := b.workflows.ListAllWorkflows(ctx)
		if err != nil {
			return ErrMsg{Err: err}
		}

		var workflows []*workflow.Workflow
		for _, entry := range entries {
			wf, loadErr := b.workflows.GetWorkflow(ctx, entry.Name)
			if loadErr != nil {
				continue
			}
			// Pack workflow YAML names (e.g. "deploy") differ from entry
			// names (e.g. "my-pack/deploy"). Normalize so setWorkflows can
			// map entries to their loaded definitions.
			wf.Name = entry.Name
			workflows = append(workflows, wf)
		}

		return WorkflowsLoadedMsg{Workflows: workflows, Entries: entries}
	}
}

// LoadHistory returns a tea.Cmd that fetches execution history records together
// with summary statistics and emits HistoryLoadedMsg on success or ErrMsg on failure.
func (b *Bridge) LoadHistory(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			return ErrMsg{Err: err}
		}

		records, err := b.history.List(ctx, nil)
		if err != nil {
			return ErrMsg{Err: err}
		}

		stats, err := b.history.GetStats(ctx, nil)
		if err != nil {
			return ErrMsg{Err: err}
		}

		return HistoryLoadedMsg{Records: records, Stats: stats}
	}
}

// RunWorkflow returns a tea.Cmd that prepares and starts async workflow execution.
// It emits ExecutionStartedMsg with a live ExecCtx that can be polled during execution.
func (b *Bridge) RunWorkflow(ctx context.Context, wf *workflow.Workflow, inputs map[string]any) tea.Cmd {
	return func() tea.Msg {
		if b.runner == nil {
			return ErrMsg{Err: errors.New("workflow execution is not available in this session")}
		}
		if err := ctx.Err(); err != nil {
			return ErrMsg{Err: err}
		}

		workflowToRun := wf
		if len(workflowToRun.Steps) == 0 {
			loadedWf, err := b.workflows.GetWorkflow(ctx, workflowToRun.Name)
			if err != nil {
				return ErrMsg{Err: err}
			}
			workflowToRun = loadedWf
		}

		execCtx, done, err := b.runner.RunWorkflowAsync(ctx, workflowToRun, inputs)
		if err != nil {
			return ErrMsg{Err: err}
		}

		return ExecutionStartedMsg{
			ExecutionID: workflowToRun.Name,
			Workflow:    workflowToRun,
			ExecCtx:     execCtx,
			Done:        done,
		}
	}
}

// RunWorkflowViaFacade returns a tea.Cmd that starts workflow execution through
// the WorkflowFacade. The resulting ExecutionStartedMsg includes a live RunSession
// whose Events() channel drives state updates in the monitoring tab (D27, FR-011).
//
// Unlike RunWorkflow, this path does not supply an ExecCtx or Done channel:
// the monitoring tab's StartEventLoop goroutine becomes the sole reader of
// Session.Events(), and when it detects a terminal event (EventWorkflowCompleted or
// EventWorkflowFailed) it sends ExecutionFinishedMsg to stop the tick loop. The
// caller must nil-guard msg.Done before calling WaitForExecution.
func (b *Bridge) RunWorkflowViaFacade(ctx context.Context, name string, inputs map[string]any) tea.Cmd {
	return func() tea.Msg {
		if b.facade == nil {
			return ErrMsg{Err: errors.New("facade not configured for event-driven execution")}
		}
		if err := ctx.Err(); err != nil {
			return ErrMsg{Err: err}
		}
		sess, err := b.facade.Run(ctx, ports.RunRequest{Identifier: name, Inputs: inputs})
		if err != nil {
			return ErrMsg{Err: err}
		}
		return ExecutionStartedMsg{
			ExecutionID: name,
			Session:     sess,
			// ExecCtx and Done are nil: state is driven entirely by Session.Events().
			// model.go nil-guards Done before calling WaitForExecution.
		}
	}
}

// WaitForExecution returns a tea.Cmd that blocks until the execution finishes
// and then delivers the result as an ExecutionFinishedMsg.
func WaitForExecution(done <-chan error) tea.Cmd {
	return func() tea.Msg {
		err := <-done
		return ExecutionFinishedMsg{Err: err}
	}
}

// ValidateWorkflow returns a tea.Cmd that validates a workflow by name and emits
// ValidationResultMsg with the outcome.
func (b *Bridge) ValidateWorkflow(ctx context.Context, name string) tea.Cmd {
	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			return ValidationResultMsg{Name: name, Success: false, Error: err.Error()}
		}

		if err := b.workflows.ValidateWorkflow(ctx, name); err != nil {
			return ValidationResultMsg{Name: name, Success: false, Error: err.Error()}
		}

		return ValidationResultMsg{Name: name, Success: true}
	}
}

var _ ports.UserInputReader = (*TUIInputReader)(nil)

// MsgSender is a function that sends a tea.Msg to the Bubble Tea program.
// Typically bound to (*tea.Program).Send.
type MsgSender func(msg tea.Msg)

// TUIInputReader implements ports.UserInputReader for the TUI.
// It bridges the blocking ConversationManager goroutine with the Bubble Tea
// event loop via channels.
type TUIInputReader struct {
	requestCh  chan struct{}
	responseCh chan string
	sender     MsgSender
}

// NewTUIInputReader creates a TUIInputReader. sender may be nil during tests;
// when non-nil it is called to notify the Bubble Tea program that input is needed.
func NewTUIInputReader(sender MsgSender) *TUIInputReader {
	return &TUIInputReader{
		requestCh:  make(chan struct{}, 1),
		responseCh: make(chan string, 1),
		sender:     sender,
	}
}

// SetSender sets the tea.Msg sender (typically (*tea.Program).Send).
// Called after the program is created but before any execution starts.
func (r *TUIInputReader) SetSender(sender MsgSender) {
	r.sender = sender
}

// ReadInput blocks until the user submits input via the TUI or the context
// is cancelled. It signals the Bubble Tea model that input is needed by
// sending InputRequestedMsg.
func (r *TUIInputReader) ReadInput(ctx context.Context) (string, error) {
	select {
	case r.requestCh <- struct{}{}:
	default:
	}

	if r.sender != nil {
		r.sender(InputRequestedMsg{})
	}

	select {
	case text := <-r.responseCh:
		return text, nil
	case <-ctx.Done():
		return "", fmt.Errorf("input cancelled: %w", ctx.Err())
	}
}

// RequestCh returns the channel that signals when input is requested.
// Used in tests; the TUI model uses InputRequestedMsg instead.
func (r *TUIInputReader) RequestCh() <-chan struct{} {
	return r.requestCh
}

// Respond sends user input back to the blocked ReadInput call.
func (r *TUIInputReader) Respond(text string) {
	r.responseCh <- text
}
