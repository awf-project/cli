//go:build integration

package tui_test

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/interfaces/tui"
)

// drive feeds a command's message back into the model until no command is left, the way
// the Bubble Tea runtime does. The messages are unexported types, so the loop stays generic.
func drive(m tui.Model, cmd tea.Cmd) tui.Model {
	for cmd != nil {
		msg := cmd()
		if _, isBatch := msg.(tea.BatchMsg); isBatch || msg == nil {
			return m
		}
		var next tea.Model
		next, cmd = m.Update(msg)
		m = next.(tui.Model)
	}
	return m
}

func press(m tui.Model, msg tea.KeyPressMsg) (tui.Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(tui.Model), cmd
}

// TestTUIHistoryDetail_ReadsTheRealPersistedState checks the whole chain against the real
// JSON state store: what a run saved is what the History detail shows, found through the
// history record's workflow ID.
func TestTUIHistoryDetail_ReadsTheRealPersistedState(t *testing.T) {
	states := store.NewJSONStore(t.TempDir())
	now := time.Now()

	ec := workflow.NewExecutionContext("run-real-1", "deploy")
	ec.Inputs = map[string]any{"env": "prod", "API_KEY": "s3cr3t-value"}
	ec.SetStepState("build", workflow.StepState{
		Name: "build", Status: workflow.StatusCompleted, Output: "compiled ok",
		StartedAt: now.Add(-9 * time.Second), CompletedAt: now.Add(-4 * time.Second),
	})
	ec.SetStepState("test", workflow.StepState{
		Name: "test", Status: workflow.StatusFailed, Stderr: "FAIL: TestX", Error: "exit status 2",
		StartedAt: now.Add(-4 * time.Second), CompletedAt: now.Add(-1 * time.Second),
	})
	require.NoError(t, states.Save(context.Background(), ec))

	bridge := tui.NewBridge(nil, nil)
	bridge.SetStateReader(states)
	m := tui.NewWithBridge(bridge, context.Background())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 110, Height: 30})
	m = updated.(tui.Model)

	updated, _ = m.Update(tui.HistoryLoadedMsg{Records: []*workflow.ExecutionRecord{
		{ID: "history-record-1", WorkflowID: "run-real-1", WorkflowName: "deploy", Status: "failed", StartedAt: now.Add(-10 * time.Second), DurationMs: 9000},
		{ID: "history-record-2", WorkflowID: "run-was-cleaned", WorkflowName: "lint", Status: "success", StartedAt: now.Add(-20 * time.Minute), DurationMs: 500},
	}})
	m = updated.(tui.Model)
	m, _ = press(m, tea.KeyPressMsg{Code: '2', Text: "2"})

	t.Run("the saved steps, outputs and masked inputs are shown", func(t *testing.T) {
		opened, cmd := press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
		opened = drive(opened, cmd)

		view := opened.View().Content
		for _, want := range []string{"build", "test", "5.0s", "3.0s", "exit status 2", "FAIL: TestX", "env", "prod", "API_KEY"} {
			assert.Contains(t, view, want)
		}
		assert.Contains(t, view, "***", "secret inputs are masked")
		assert.NotContains(t, view, "s3cr3t-value", "a secret read back from disk must never be drawn")
	})

	t.Run("a run whose state was cleaned up says so", func(t *testing.T) {
		second, _ := press(m, tea.KeyPressMsg{Code: tea.KeyDown})
		opened, cmd := press(second, tea.KeyPressMsg{Code: tea.KeyEnter})
		opened = drive(opened, cmd)

		assert.Contains(t, opened.View().Content, "no longer available")
	})
}
