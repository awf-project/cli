package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
)

func loadedHistory() HistoryScreen {
	s := newHistoryScreen()
	s.setRecords(sampleRecords(time.Now()), &workflow.HistoryStats{TotalExecutions: 3, SuccessCount: 1, FailedCount: 1})
	return s
}

func runIDs(records []*workflow.ExecutionRecord) []string {
	out := make([]string, len(records))
	for i, r := range records {
		out[i] = r.ID
	}
	return out
}

func TestHistoryScreen_RecordsAreNewestFirst(t *testing.T) {
	s := loadedHistory()

	assert.Equal(t, []string{"run-cccccccc33", "run-aaaaaaaa11", "run-bbbbbbbb22"}, runIDs(s.visible()))
}

func TestHistoryScreen_SetRecordsDoesNotMutateTheCallersSlice(t *testing.T) {
	in := sampleRecords(time.Now())
	first := in[0]

	s := newHistoryScreen()
	s.setRecords(in, nil)

	assert.Same(t, first, in[0], "the caller's slice order must be left alone")
}

func TestHistoryScreen_StatusFilterCycle(t *testing.T) {
	s := loadedHistory()

	for _, step := range []struct {
		wantLabel string
		wantIDs   []string
	}{
		{"success", []string{"run-aaaaaaaa11"}},
		{"failed", []string{"run-bbbbbbbb22"}},
		{"all", []string{"run-cccccccc33", "run-aaaaaaaa11", "run-bbbbbbbb22"}},
	} {
		s, _ = s.Update(press("s"))
		assert.Equal(t, step.wantLabel, s.status.label())
		assert.Equal(t, step.wantIDs, runIDs(s.visible()))
	}
}

func TestHistoryScreen_NameFilterCombinesWithStatus(t *testing.T) {
	s := loadedHistory()

	s, _ = s.Update(press("/"))
	require.True(t, s.InputActive())
	for _, r := range "DEPLOY" {
		s, _ = s.Update(press(string(r)))
	}
	assert.Equal(t, []string{"run-aaaaaaaa11", "run-bbbbbbbb22"}, runIDs(s.visible()), "filter is case-insensitive")

	s, _ = s.Update(press("enter"))
	s, _ = s.Update(press("s")) // success only
	assert.Equal(t, []string{"run-aaaaaaaa11"}, runIDs(s.visible()))

	s, _ = s.Update(press("esc"))
	assert.Equal(t, []string{"run-aaaaaaaa11"}, runIDs(s.visible()), "esc clears the name filter, the status filter stays")
}

func TestHistoryScreen_SelectionStaysInBounds(t *testing.T) {
	s := loadedHistory()

	for i := 0; i < 5; i++ {
		s, _ = s.Update(press("down"))
	}
	assert.Equal(t, 2, s.sel)

	s, _ = s.Update(press("s")) // only one record left
	assert.Equal(t, 0, s.sel, "the cursor is clamped when the list shrinks")

	s, _ = s.Update(press("up"))
	assert.Equal(t, 0, s.sel)
}

func TestHistoryScreen_DetailShowsTheSelectedRun(t *testing.T) {
	s := loadedHistory()
	s, _ = s.Update(press("down")) // run-aaaaaaaa11 (success)

	out := ansi.Strip(s.detailBox(70, 20))

	for _, want := range []string{"deploy", "success", "run-aaaa", "12.4s", "Exit code", "3 h ago"} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "Error", "a successful run has no error section")
}

func TestHistoryScreen_DetailWrapsLongErrorMessages(t *testing.T) {
	s := loadedHistory()
	s.sel = 2 // run-bbbbbbbb22 (failed, has an error message)

	out := ansi.Strip(s.detailBox(40, 20))

	assert.Contains(t, out, "Error")
	assert.Contains(t, out, "exit status 2")
	for _, l := range lines(out) {
		assert.LessOrEqual(t, ansiWidth(l), 40, "wrapped lines must stay inside the box")
	}
}

func TestHistoryScreen_DetailShowsAttemptAndResumeOnlyWhenRelevant(t *testing.T) {
	s := newHistoryScreen()
	s.setRecords([]*workflow.ExecutionRecord{{ID: "run-1", WorkflowName: "wf", Status: "success", StartedAt: time.Now(), Attempt: 3, ResumedFromID: "run-0000000000"}}, nil)

	out := ansi.Strip(s.detailBox(70, 20))

	assert.Contains(t, out, "Attempt")
	assert.Contains(t, out, "Resumed from")
	assert.Contains(t, out, "run-0000")
}

func TestHistoryScreen_EmptyStates(t *testing.T) {
	s := newHistoryScreen()

	assert.Contains(t, ansi.Strip(s.listBox(50, 12)), "no executions")
	assert.Contains(t, ansi.Strip(s.detailBox(60, 10)), "Select an execution.")
	assert.Contains(t, ansi.Strip(s.listBox(50, 12)), "0/0")
}

func TestHistoryScreen_FooterSummary(t *testing.T) {
	withStats := loadedHistory()
	assert.Contains(t, ansi.Strip(withStats.listBox(60, 12)), "3 runs · 1 ok · 1 failed")

	noStats := newHistoryScreen()
	noStats.setRecords(sampleRecords(time.Now()), nil)
	assert.Contains(t, ansi.Strip(noStats.listBox(60, 12)), "3 runs")
}

func TestHistoryScreen_ListScrollsToTheSelection(t *testing.T) {
	var recs []*workflow.ExecutionRecord
	for i := 0; i < 40; i++ {
		recs = append(recs, &workflow.ExecutionRecord{
			ID: fmt.Sprintf("run-%02d", i), WorkflowName: fmt.Sprintf("wf-%02d", i),
			Status: "success", StartedAt: time.Now().Add(-time.Duration(i) * time.Minute),
		})
	}
	s := newHistoryScreen()
	s.setRecords(recs, nil)
	s.sel = 30

	out := ansi.Strip(s.listBox(50, 12))

	assert.Contains(t, out, "wf-30")
	assert.NotContains(t, out, "wf-00")
	assert.Contains(t, out, "31/40")
}

func TestHistoryScreen_RefreshEmitsRequest(t *testing.T) {
	s := loadedHistory()
	_, cmd := s.Update(press("r"))
	require.NotNil(t, cmd)
	assert.Equal(t, refreshMsg{}, cmd())
}

func TestHistoryScreen_KeysBarShowsTheStatusFilter(t *testing.T) {
	s := loadedHistory()
	assert.Contains(t, ansi.Strip(s.keys(100)), "status: all")

	s, _ = s.Update(press("s"))
	assert.Contains(t, ansi.Strip(s.keys(100)), "status: success")
}

func TestStatusFilter_Matches(t *testing.T) {
	assert.True(t, statusFilterAll.matches("anything"))
	assert.True(t, statusFilterSuccess.matches("success"))
	assert.False(t, statusFilterSuccess.matches("failed"))
	assert.True(t, statusFilterFailed.matches("failed"))
	assert.False(t, statusFilterFailed.matches("cancelled"))
	assert.Equal(t, statusFilterAll, statusFilterFailed.next(), "the cycle wraps")
}
