package tui

import (
	"context"
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// StateReader is the driven port for reading the state AWF persisted for a run: what each
// step printed, how long it took and how it ended. It is satisfied by ports.StateStore.
//
// Load returns (nil, nil) when no state exists for the identifier.
type StateReader interface {
	Load(ctx context.Context, workflowID string) (*workflow.ExecutionContext, error)
}

// StepDetail is one step of a past run.
type StepDetail struct {
	State    workflow.StepState
	Duration time.Duration
}

// RunDetail is what the History screen shows for a past execution: its inputs and, step by
// step, the output it produced.
type RunDetail struct {
	Inputs map[string]any
	// Steps are ordered by start time; steps that never started come last, by name.
	Steps []StepDetail
}

// RunDetailLoadedMsg carries the detail of one execution, or the reason it could not be
// loaded. RecordID identifies the history record the request was made for, so a late reply
// for an execution the user already left is ignored.
type RunDetailLoadedMsg struct {
	RecordID string
	Detail   *RunDetail
	Err      string
}

// loadRunDetailMsg asks the Model to load the detail of a history record.
type loadRunDetailMsg struct{ Record *workflow.ExecutionRecord }

// flashMsg shows a one-line message in the page header.
type flashMsg struct {
	Text string
	Err  bool
}

// buildRunDetail turns a persisted execution into the view model.
func buildRunDetail(ec *workflow.ExecutionContext) *RunDetail {
	states := ec.GetAllStepStates()
	steps := make([]StepDetail, 0, len(states))
	for name := range states {
		st := states[name]
		if st.Name == "" {
			st.Name = name
		}
		d := StepDetail{State: st}
		if !st.StartedAt.IsZero() && !st.CompletedAt.IsZero() {
			d.Duration = st.CompletedAt.Sub(st.StartedAt)
		}
		steps = append(steps, d)
	}
	sort.Slice(steps, func(i, j int) bool {
		a, b := &steps[i].State, &steps[j].State
		switch {
		case a.StartedAt.IsZero() != b.StartedAt.IsZero():
			return !a.StartedAt.IsZero() // steps that ran come first
		case !a.StartedAt.Equal(b.StartedAt):
			return a.StartedAt.Before(b.StartedAt)
		}
		return a.Name < b.Name
	})
	return &RunDetail{Inputs: ec.Inputs, Steps: steps}
}

// SetStateReader wires the reader used by LoadRunDetail. A nil reader is accepted and
// makes LoadRunDetail report that history detail is unavailable.
func (b *Bridge) SetStateReader(r StateReader) {
	b.states = r
}

// LoadRunDetail returns a tea.Cmd that reads the persisted state of a run and emits
// RunDetailLoadedMsg. workflowID is ExecutionRecord.WorkflowID, the identifier the state
// was saved under; recordID is echoed back so the screen can match the reply.
func (b *Bridge) LoadRunDetail(ctx context.Context, recordID, workflowID string) tea.Cmd {
	return func() tea.Msg {
		if b.states == nil {
			return RunDetailLoadedMsg{RecordID: recordID, Err: labelDetailNoStore}
		}
		if err := ctx.Err(); err != nil {
			return RunDetailLoadedMsg{RecordID: recordID, Err: err.Error()}
		}
		ec, err := b.states.Load(ctx, workflowID)
		if err != nil {
			return RunDetailLoadedMsg{RecordID: recordID, Err: err.Error()}
		}
		if ec == nil {
			return RunDetailLoadedMsg{RecordID: recordID, Err: labelDetailGone}
		}
		return RunDetailLoadedMsg{RecordID: recordID, Detail: buildRunDetail(ec)}
	}
}
