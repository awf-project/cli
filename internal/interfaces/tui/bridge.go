package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"

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
