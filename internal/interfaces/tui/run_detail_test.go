package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
)

func stepNames(d *RunDetail) []string {
	out := make([]string, len(d.Steps))
	for i := range d.Steps {
		out[i] = d.Steps[i].State.Name
	}
	return out
}

func TestBuildRunDetail_OrdersByStartThenUnstartedByName(t *testing.T) {
	now := time.Now()
	ec := workflow.NewExecutionContext("wf-1", "deploy")
	ec.SetStepState("zeta", workflow.StepState{Name: "zeta", Status: workflow.StatusPending})
	ec.SetStepState("late", workflow.StepState{Name: "late", StartedAt: now})
	ec.SetStepState("alpha", workflow.StepState{Name: "alpha", Status: workflow.StatusPending})
	ec.SetStepState("early", workflow.StepState{Name: "early", StartedAt: now.Add(-time.Minute)})

	d := buildRunDetail(ec)

	assert.Equal(t, []string{"early", "late", "alpha", "zeta"}, stepNames(d))
}

func TestBuildRunDetail_DurationsAndNames(t *testing.T) {
	now := time.Now()
	ec := workflow.NewExecutionContext("wf-1", "deploy")
	ec.SetStepState("done", workflow.StepState{Name: "done", StartedAt: now, CompletedAt: now.Add(1500 * time.Millisecond)})
	ec.SetStepState("running", workflow.StepState{Name: "running", StartedAt: now.Add(time.Second)}) // never completed
	ec.SetStepState("nameless", workflow.StepState{StartedAt: now.Add(2 * time.Second)})             // key supplies the name

	d := buildRunDetail(ec)

	require.Len(t, d.Steps, 3)
	assert.Equal(t, 1500*time.Millisecond, d.Steps[0].Duration)
	assert.Zero(t, d.Steps[1].Duration, "a step that never completed has no duration")
	assert.Equal(t, "nameless", d.Steps[2].State.Name, "the map key names a step stored without one")
}

func TestBuildRunDetail_KeepsInputsAndHandlesNoSteps(t *testing.T) {
	ec := workflow.NewExecutionContext("wf-1", "deploy")
	ec.Inputs = map[string]any{"env": "prod"}

	d := buildRunDetail(ec)

	assert.Equal(t, map[string]any{"env": "prod"}, d.Inputs)
	assert.Empty(t, d.Steps)
}

func TestBridge_LoadRunDetail(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		reader    StateReader
		ctx       func() context.Context
		wantErr   string
		wantSteps int
	}{
		{"no state reader", nil, context.Background, labelDetailNoStore, 0},
		{"state is gone", &fakeStates{}, context.Background, labelDetailGone, 0},
		{"reader failure", &fakeStates{err: errors.New("disk on fire")}, context.Background, "disk on fire", 0},
		{"cancelled context", &fakeStates{ec: sampleExecution(now)}, func() context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx
		}, context.Canceled.Error(), 0},
		{"success", &fakeStates{ec: sampleExecution(now)}, context.Background, "", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBridge(nil, nil)
			if tt.reader != nil {
				b.SetStateReader(tt.reader)
			}

			msg := b.LoadRunDetail(tt.ctx(), "rec-1", "wf-1")()

			got, ok := msg.(RunDetailLoadedMsg)
			require.True(t, ok)
			assert.Equal(t, "rec-1", got.RecordID, "the record ID is echoed so replies can be matched")
			assert.Equal(t, tt.wantErr, got.Err)
			if tt.wantErr == "" {
				require.NotNil(t, got.Detail)
				assert.Len(t, got.Detail.Steps, tt.wantSteps)
			} else {
				assert.Nil(t, got.Detail)
			}
		})
	}
}

func TestBridge_LoadRunDetail_UsesTheWorkflowIDNotTheRecordID(t *testing.T) {
	states := &fakeStates{ec: sampleExecution(time.Now())}
	b := NewBridge(nil, nil)
	b.SetStateReader(states)

	b.LoadRunDetail(context.Background(), "history-record-id", "state-file-id")()

	assert.Equal(t, []string{"state-file-id"}, states.loadedIDs, "state is saved under the workflow ID")
}

func TestBridge_ImplementsBackendWithRunDetail(t *testing.T) {
	var _ Backend = (*Bridge)(nil)
}
