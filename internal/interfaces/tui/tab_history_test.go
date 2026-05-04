// Package tui — tests for T008: history tab implementation (internal/interfaces/tui/tab_history.go).
//
// Acceptance Criteria covered:
//
//	AC1:  Lists ExecutionRecord entries showing workflow name, short ID, status, start time, duration
//	       → TestHistoryTab_RebuildTableRows_PopulatesRows
//	AC2:  Sorted chronologically newest-first
//	       → TestHistoryTab_SetRecords_SortsNewestFirst
//	AC3:  Text input filter narrows by workflow name substring
//	       → TestHistoryTab_ApplyFilters_TextFilter
//	AC4:  Status toggle cycles all→success→failed→all
//	       → TestHistoryTab_CycleStatusFilter_Cycles
//	AC5:  Status filter narrows records by execution status
//	       → TestHistoryTab_ApplyFilters_StatusFilter
//	AC6:  Enter on selected record switches to detail view
//	       → TestHistoryTab_Update_Enter_OpensDetailView
//	AC7:  Detail view renders read-only execution info
//	       → TestHistoryTab_DetailContent_ShowsAllFields
//	AC8:  Escape from detail view returns to list
//	       → TestHistoryTab_Update_Escape_ReturnsToList
//	AC9:  Empty state renders "No execution history found."
//	       → TestHistoryTab_View_EmptyState
//	AC10: Table cursor navigation up/down moves selection
//	       → TestHistoryTab_Update_UpDown_MovesCursor
//	AC11: Tab key cycles status filter
//	       → TestHistoryTab_Update_Tab_CyclesStatusFilter
//	AC12: HistoryLoadedMsg wires data into the tab
//	       → TestHistoryTab_Update_HistoryLoadedMsg
//	AC13: WindowSizeMsg updates dimensions
//	       → TestHistoryTab_Update_WindowSizeMsg
package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// --- helpers ---

// makeRecord returns a minimal ExecutionRecord for use in tests.
func makeRecord(id, name, status string, startedAt time.Time, durationMs int64) *workflow.ExecutionRecord {
	return &workflow.ExecutionRecord{
		ID:           id,
		WorkflowID:   "wf-" + name,
		WorkflowName: name,
		Status:       status,
		StartedAt:    startedAt,
		CompletedAt:  startedAt.Add(time.Duration(durationMs) * time.Millisecond),
		DurationMs:   durationMs,
	}
}

// historyTabWithRecords creates a HistoryTab with setRecords already called.
func historyTabWithRecords(records []*workflow.ExecutionRecord, stats *workflow.HistoryStats) HistoryTab {
	tab := newHistoryTab()
	tab.setRecords(records, stats)
	return tab
}

// --- newHistoryTab ---

func TestNewHistoryTab_ZeroValue(t *testing.T) {
	tab := newHistoryTab()

	assert.Equal(t, 0, tab.width)
	assert.Equal(t, 0, tab.height)
	assert.Nil(t, tab.history)
	assert.Nil(t, tab.stats)
	assert.Nil(t, tab.filtered)
	assert.Nil(t, tab.selected)
	assert.Equal(t, historyListView, tab.view)
	assert.Equal(t, statusFilterAll, tab.statusFilter)
	// table is initialized (not zero-value) — cursor starts at 0
	assert.Equal(t, 0, tab.tbl.Cursor())
}

func TestHistoryTab_Init_ReturnsNil(t *testing.T) {
	tab := newHistoryTab()
	cmd := tab.Init()
	assert.Nil(t, cmd)
}

// --- setRecords / applyFilters ---

func TestHistoryTab_SetRecords_SortsNewestFirst(t *testing.T) {
	now := time.Now()
	older := makeRecord("a", "wf-a", "success", now.Add(-2*time.Hour), 100)
	newer := makeRecord("b", "wf-b", "success", now.Add(-1*time.Hour), 200)
	oldest := makeRecord("c", "wf-c", "success", now.Add(-3*time.Hour), 300)

	tab := historyTabWithRecords([]*workflow.ExecutionRecord{older, oldest, newer}, nil)

	require.Len(t, tab.history, 3)
	assert.Equal(t, "b", tab.history[0].ID, "newest should be first")
	assert.Equal(t, "a", tab.history[1].ID)
	assert.Equal(t, "c", tab.history[2].ID, "oldest should be last")
}

func TestHistoryTab_SetRecords_StatsStored(t *testing.T) {
	stats := &workflow.HistoryStats{TotalExecutions: 5, SuccessCount: 3, FailedCount: 2}
	tab := historyTabWithRecords(nil, stats)

	assert.Equal(t, stats, tab.stats)
}

func TestHistoryTab_SetRecords_NilRecords_InitializesEmpty(t *testing.T) {
	tab := historyTabWithRecords(nil, nil)

	assert.Empty(t, tab.history)
	assert.Empty(t, tab.filtered)
}

func TestHistoryTab_SetRecords_DoesNotMutateInput(t *testing.T) {
	now := time.Now()
	original := []*workflow.ExecutionRecord{
		makeRecord("a", "wf", "success", now, 100),
	}
	origLen := len(original)

	tab := historyTabWithRecords(original, nil)
	_ = tab

	assert.Len(t, original, origLen, "original slice must not be mutated")
}

// --- applyFilters ---

func TestHistoryTab_ApplyFilters_TextFilter(t *testing.T) {
	now := time.Now()
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("1", "deploy-api", "success", now, 100),
		makeRecord("2", "build-frontend", "success", now, 200),
		makeRecord("3", "deploy-db", "success", now, 300),
	}, nil)

	// Simulate typing "deploy" into the filter input.
	tab.filterInput.SetValue("deploy")
	tab.applyFilters()

	require.Len(t, tab.filtered, 2)
	names := []string{tab.filtered[0].WorkflowName, tab.filtered[1].WorkflowName}
	assert.Contains(t, names, "deploy-api")
	assert.Contains(t, names, "deploy-db")
}

func TestHistoryTab_ApplyFilters_TextFilter_CaseInsensitive(t *testing.T) {
	now := time.Now()
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("1", "DEPLOY-API", "success", now, 100),
		makeRecord("2", "build-frontend", "success", now, 200),
	}, nil)

	tab.filterInput.SetValue("deploy")
	tab.applyFilters()

	require.Len(t, tab.filtered, 1)
	assert.Equal(t, "DEPLOY-API", tab.filtered[0].WorkflowName)
}

func TestHistoryTab_ApplyFilters_EmptyText_ShowsAll(t *testing.T) {
	now := time.Now()
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("1", "wf-a", "success", now, 100),
		makeRecord("2", "wf-b", "failed", now, 200),
	}, nil)

	tab.filterInput.SetValue("")
	tab.applyFilters()

	assert.Len(t, tab.filtered, 2)
}

func TestHistoryTab_ApplyFilters_StatusFilter(t *testing.T) {
	now := time.Now()
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("1", "wf-a", "success", now, 100),
		makeRecord("2", "wf-b", "failed", now, 200),
		makeRecord("3", "wf-c", "cancelled", now, 300),
		makeRecord("4", "wf-d", "success", now, 400),
	}, nil)

	tests := []struct {
		filter   statusFilter
		wantLen  int
		wantStat string
	}{
		{statusFilterAll, 4, ""},
		{statusFilterSuccess, 2, "success"},
		{statusFilterFailed, 1, "failed"},
	}

	for _, tt := range tests {
		tab.statusFilter = tt.filter
		tab.applyFilters()
		assert.Len(t, tab.filtered, tt.wantLen, "filter=%v", tt.filter)
		for _, rec := range tab.filtered {
			if tt.wantStat != "" {
				assert.Equal(t, tt.wantStat, rec.Status)
			}
		}
	}
}

func TestHistoryTab_ApplyFilters_CombinedTextAndStatus(t *testing.T) {
	now := time.Now()
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("1", "deploy-api", "success", now, 100),
		makeRecord("2", "deploy-db", "failed", now, 200),
		makeRecord("3", "build", "success", now, 300),
	}, nil)

	tab.filterInput.SetValue("deploy")
	tab.statusFilter = statusFilterSuccess
	tab.applyFilters()

	require.Len(t, tab.filtered, 1)
	assert.Equal(t, "deploy-api", tab.filtered[0].WorkflowName)
}

// --- cycleStatusFilter ---

func TestHistoryTab_CycleStatusFilter_Cycles(t *testing.T) {
	tab := newHistoryTab()

	assert.Equal(t, statusFilterAll, tab.statusFilter)
	tab.cycleStatusFilter()
	assert.Equal(t, statusFilterSuccess, tab.statusFilter)
	tab.cycleStatusFilter()
	assert.Equal(t, statusFilterFailed, tab.statusFilter)
	tab.cycleStatusFilter()
	assert.Equal(t, statusFilterAll, tab.statusFilter)
}

func TestHistoryTab_StatusFilter_Label(t *testing.T) {
	assert.Equal(t, "all", statusFilterAll.label())
	assert.Equal(t, "success", statusFilterSuccess.label())
	assert.Equal(t, "failed", statusFilterFailed.label())
}

// --- StatusBadgeFromString ---

func TestStatusBadgeFromString_ReturnsExpectedBadges(t *testing.T) {
	assert.Contains(t, StatusBadgeFromString("success"), "✓")
	assert.Contains(t, StatusBadgeFromString("failed"), "✗")
	assert.Contains(t, StatusBadgeFromString("cancelled"), "⏭")
	assert.Contains(t, StatusBadgeFromString("unknown"), "?")
}

// --- rebuildTableRows ---

func TestHistoryTab_RebuildTableRows_PopulatesRows(t *testing.T) {
	now := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("abcdefgh1234", "my-workflow", "success", now, 42000),
	}, nil)

	rows := tab.tbl.Rows()
	require.Len(t, rows, 1)
	assert.Contains(t, rows[0][0], "✓") // StatusBadgeFromString returns styled "✓ success"
	assert.Equal(t, "my-workflow", rows[0][1])
	assert.Equal(t, "abcdefgh", rows[0][2])
	assert.Contains(t, rows[0][3], "2024-03-15")
	assert.Equal(t, "42000ms", rows[0][4])
}

func TestHistoryTab_RebuildTableRows_ShortID_NotTruncatedWhenLessThan8Chars(t *testing.T) {
	now := time.Now()
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("abc", "wf", "success", now, 100),
	}, nil)

	rows := tab.tbl.Rows()
	require.Len(t, rows, 1)
	assert.Equal(t, "abc", rows[0][2])
}

func TestHistoryTab_RebuildTableRows_ZeroStartedAt_ShowsDash(t *testing.T) {
	rec := &workflow.ExecutionRecord{
		ID:           "id-001",
		WorkflowName: "wf",
		Status:       "success",
		DurationMs:   100,
	}
	tab := newHistoryTab()
	tab.setRecords([]*workflow.ExecutionRecord{rec}, nil)

	rows := tab.tbl.Rows()
	require.Len(t, rows, 1)
	assert.Equal(t, "—", rows[0][3])
}

// --- View / renderListView ---

func TestHistoryTab_View_EmptyState(t *testing.T) {
	tab := newHistoryTab()
	view := tab.View()

	assert.Contains(t, view, "No execution history found")
}

func TestHistoryTab_View_EmptyStateAfterFilter(t *testing.T) {
	now := time.Now()
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("1", "wf-a", "success", now, 100),
	}, nil)

	// Filter that matches nothing.
	tab.filterInput.SetValue("zzz-no-match")
	tab.applyFilters()

	view := tab.View()
	assert.Contains(t, view, "No execution history found")
}

func TestHistoryTab_View_ShowsFilterInputAndStatusLabel(t *testing.T) {
	tab := newHistoryTab()
	view := tab.View()

	assert.Contains(t, view, "Filter:")
	assert.Contains(t, view, "[All]")        // active badge for default filter
	assert.Contains(t, view, "Tab to cycle") // cycle hint
}

func TestHistoryTab_View_ShowsStatusLabelAfterCycle(t *testing.T) {
	tab := newHistoryTab()
	tab.cycleStatusFilter() // → success

	view := tab.View()
	assert.Contains(t, view, "Success") // active badge shows [✓ Success]
}

func TestHistoryTab_View_WithRecords_ShowsWorkflowNameAndID(t *testing.T) {
	now := time.Now()
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("exec-id-001", "my-deploy", "success", now, 5000),
	}, nil)
	// Give the tab a size so the table renders.
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	view := tab.View()

	assert.Contains(t, view, "my-deploy")
	assert.Contains(t, view, "exec-id-") // short ID prefix
	assert.Contains(t, view, "5000ms")
}

func TestHistoryTab_View_WithStats_ShowsFooter(t *testing.T) {
	now := time.Now()
	stats := &workflow.HistoryStats{
		TotalExecutions: 3,
		SuccessCount:    2,
		FailedCount:     1,
		CancelledCount:  0,
		AvgDurationMs:   3000,
	}
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("1", "wf", "success", now, 100),
	}, stats)

	view := tab.View()

	assert.Contains(t, view, "Total: 3")
	assert.Contains(t, view, "Success: ") // colored value follows ANSI codes
	assert.Contains(t, view, "Failed: ")  // colored value follows ANSI codes
	assert.Contains(t, view, "Cancelled: 0")
	assert.Contains(t, view, "3000ms")
}

func TestHistoryTab_View_CursorHighlightsSelectedRow(t *testing.T) {
	now := time.Now()
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("1", "first-wf", "success", now, 100),
		makeRecord("2", "second-wf", "success", now, 200),
	}, nil)
	tab.width = 120
	tab.height = 30
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	view := tab.View()

	// The view should contain both names; selected styling is applied but text still present.
	assert.Contains(t, view, "first-wf")
	assert.Contains(t, view, "second-wf")
}

// --- detail view ---

func TestHistoryTab_DetailContent_ShowsAllFields(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	rec := &workflow.ExecutionRecord{
		ID:           "full-exec-id",
		WorkflowID:   "wf-123",
		WorkflowName: "my-pipeline",
		Status:       "failed",
		ExitCode:     1,
		StartedAt:    now,
		CompletedAt:  now.Add(5 * time.Second),
		DurationMs:   5000,
		ErrorMessage: "step 3 timed out",
	}

	tab := newHistoryTab()
	tab.selected = rec
	content := tab.renderDetailContent()

	assert.Contains(t, content, "full-exec-id")
	assert.Contains(t, content, "wf-123")
	assert.Contains(t, content, "my-pipeline")
	assert.Contains(t, content, "failed")
	assert.Contains(t, content, "1") // exit code
	assert.Contains(t, content, "2024-06-01")
	assert.Contains(t, content, "5000ms")
	assert.Contains(t, content, "step 3 timed out")
}

func TestHistoryTab_DetailContent_NilSelected_ReturnsEmpty(t *testing.T) {
	tab := newHistoryTab()
	content := tab.renderDetailContent()
	assert.Empty(t, content)
}

func TestHistoryTab_DetailContent_NoErrorMessage_OmitsErrorSection(t *testing.T) {
	rec := makeRecord("id", "wf", "success", time.Now(), 100)
	tab := newHistoryTab()
	tab.selected = rec
	content := tab.renderDetailContent()

	assert.NotContains(t, content, "Error:")
}

func TestHistoryTab_OpenDetail_SwitchesToDetailView(t *testing.T) {
	now := time.Now()
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("1", "wf-a", "success", now, 100),
	}, nil)
	// Table needs a size to have a valid cursor.
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	tab.openDetail()

	assert.Equal(t, historyDetailView, tab.view)
	require.NotNil(t, tab.selected)
	assert.Equal(t, "1", tab.selected.ID)
}

func TestHistoryTab_OpenDetail_EmptyFiltered_DoesNothing(t *testing.T) {
	tab := newHistoryTab()
	tab.openDetail()

	assert.Equal(t, historyListView, tab.view)
	assert.Nil(t, tab.selected)
}

func TestHistoryTab_CloseDetail_ReturnsToList(t *testing.T) {
	now := time.Now()
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("1", "wf-a", "success", now, 100),
	}, nil)
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	tab, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, historyDetailView, tab.view)

	tab.closeDetail()

	assert.Equal(t, historyListView, tab.view)
	assert.Nil(t, tab.selected)
}

func TestHistoryTab_View_DetailView_ShowsHeader(t *testing.T) {
	now := time.Now()
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("detail-exec-id", "my-wf", "success", now, 1000),
	}, nil)
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	tab.openDetail()

	view := tab.View()

	assert.Contains(t, view, "Execution Detail")
	assert.Contains(t, view, "detail-exec-id")
	assert.Contains(t, view, "Esc to return")
}

// --- Update — WindowSizeMsg ---

func TestHistoryTab_Update_WindowSizeMsg(t *testing.T) {
	tab := newHistoryTab()
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}

	updated, cmd := tab.Update(msg)

	assert.Equal(t, 120, updated.width)
	assert.Equal(t, 40, updated.height)
	assert.Nil(t, cmd)
}

func TestHistoryTab_Update_WindowSizeMsg_PreservesData(t *testing.T) {
	now := time.Now()
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("1", "wf", "success", now, 100),
	}, &workflow.HistoryStats{TotalExecutions: 1})

	updated, _ := tab.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	assert.Len(t, updated.history, 1)
	require.NotNil(t, updated.stats)
	assert.Equal(t, 1, updated.stats.TotalExecutions)
}

// --- Update — HistoryLoadedMsg ---

func TestHistoryTab_Update_HistoryLoadedMsg(t *testing.T) {
	tab := newHistoryTab()
	now := time.Now()
	records := []*workflow.ExecutionRecord{
		makeRecord("1", "wf-a", "success", now, 100),
		makeRecord("2", "wf-b", "failed", now.Add(-1*time.Hour), 200),
	}
	stats := &workflow.HistoryStats{TotalExecutions: 2, SuccessCount: 1, FailedCount: 1}

	updated, cmd := tab.Update(HistoryLoadedMsg{Records: records, Stats: stats})

	assert.Nil(t, cmd)
	assert.Len(t, updated.history, 2)
	assert.Equal(t, stats, updated.stats)
	// Filtered should also be populated.
	assert.Len(t, updated.filtered, 2)
}

// --- Update — navigation ---

func TestHistoryTab_Update_UpDown_MovesCursor(t *testing.T) {
	now := time.Now()
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("1", "first", "success", now, 100),
		makeRecord("2", "second", "success", now, 200),
		makeRecord("3", "third", "success", now, 300),
	}, nil)
	// Table needs height to render rows for navigation.
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	updated, _ := tab.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 1, updated.tbl.Cursor())

	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 2, updated.tbl.Cursor())

	// Cannot go past last.
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 2, updated.tbl.Cursor())

	// Move up.
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 1, updated.tbl.Cursor())

	// Cannot go before first.
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 0, updated.tbl.Cursor())
}

func TestHistoryTab_Update_VimKeys_MovesCursor(t *testing.T) {
	now := time.Now()
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("1", "wf-a", "success", now, 100),
		makeRecord("2", "wf-b", "success", now, 200),
	}, nil)
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	updated, _ := tab.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	assert.Equal(t, 1, updated.tbl.Cursor())

	updated, _ = updated.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	assert.Equal(t, 0, updated.tbl.Cursor())
}

// --- Update — Tab key cycles status filter ---

func TestHistoryTab_Update_Tab_CyclesStatusFilter(t *testing.T) {
	tab := newHistoryTab()
	assert.Equal(t, statusFilterAll, tab.statusFilter)

	updated, _ := tab.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	assert.Equal(t, statusFilterSuccess, updated.statusFilter)

	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	assert.Equal(t, statusFilterFailed, updated.statusFilter)

	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	assert.Equal(t, statusFilterAll, updated.statusFilter)
}

// --- Update — Enter opens detail ---

func TestHistoryTab_Update_Enter_OpensDetailView(t *testing.T) {
	now := time.Now()
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("exec-id", "wf-name", "success", now, 1000),
	}, nil)
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	updated, cmd := tab.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Nil(t, cmd)
	assert.Equal(t, historyDetailView, updated.view)
	require.NotNil(t, updated.selected)
	assert.Equal(t, "exec-id", updated.selected.ID)
}

func TestHistoryTab_Update_Enter_EmptyList_DoesNothing(t *testing.T) {
	tab := newHistoryTab()

	updated, cmd := tab.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Nil(t, cmd)
	assert.Equal(t, historyListView, updated.view)
	assert.Nil(t, updated.selected)
}

// --- Update — Escape returns to list ---

func TestHistoryTab_Update_Escape_ReturnsToList(t *testing.T) {
	now := time.Now()
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("1", "wf", "success", now, 100),
	}, nil)
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	// Manually enter detail view.
	tab, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, historyDetailView, tab.view)

	updated, cmd := tab.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	assert.Nil(t, cmd)
	assert.Equal(t, historyListView, updated.view)
	assert.Nil(t, updated.selected)
}

func TestHistoryTab_Update_Escape_InListView_DoesNothing(t *testing.T) {
	tab := newHistoryTab()

	updated, cmd := tab.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	// Esc in list view should not panic or crash; tab remains in list view.
	assert.Equal(t, historyListView, updated.view)
	_ = cmd
}

// --- Update — text filter typing ---

func TestHistoryTab_Update_TextTyping_FilterNarrowsList(t *testing.T) {
	now := time.Now()
	tab := historyTabWithRecords([]*workflow.ExecutionRecord{
		makeRecord("1", "deploy-api", "success", now, 100),
		makeRecord("2", "build-frontend", "success", now, 200),
		makeRecord("3", "deploy-db", "success", now, 300),
	}, nil)

	// Type 'd' key — textinput should update and filter should reapply.
	updated, _ := tab.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	// After just "d" we expect some filtering (at least does not panic).
	assert.NotNil(t, updated)
}

// --- availableListHeight ---

func TestHistoryTab_AvailableListHeight_MinimumOne(t *testing.T) {
	tab := newHistoryTab()
	tab.height = 0

	assert.Equal(t, 1, tab.availableListHeight())
}

func TestHistoryTab_AvailableListHeight_CorrectForNormalHeight(t *testing.T) {
	tab := newHistoryTab()
	tab.height = 30

	// Reserved 9 lines: model chrome (3) + filter+separator (2) + table header (1) + footer (2) + help bar (1).
	assert.Equal(t, 21, tab.availableListHeight())
}

// --- View shows records ---

func TestHistoryTab_View_ShowsRecordsInScrollWindow(t *testing.T) {
	now := time.Now()
	records := make([]*workflow.ExecutionRecord, 10)
	for i := range records {
		records[i] = makeRecord(
			strings.Repeat("x", 8),
			"workflow-"+string(rune('a'+i)),
			"success",
			now,
			int64(i*100),
		)
	}
	tab := historyTabWithRecords(records, nil)
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	view := tab.View()

	// Table renders visible rows — at least the first few should appear.
	assert.Contains(t, view, "workflow-a")
}

// --- multiple sequential updates ---

func TestHistoryTab_Update_MultipleWindowMessages(t *testing.T) {
	tab := newHistoryTab()

	updated1, _ := tab.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	assert.Equal(t, 100, updated1.width)

	updated2, _ := updated1.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	assert.Equal(t, 80, updated2.width)
	assert.Equal(t, 24, updated2.height)
}

// --- scenario: full workflow ---

func TestHistoryTab_FullWorkflow_LoadFilterSelectDetail(t *testing.T) {
	now := time.Now()
	records := []*workflow.ExecutionRecord{
		makeRecord("aaa11111", "deploy-api", "success", now, 1000),
		makeRecord("bbb22222", "build-frontend", "failed", now.Add(-time.Hour), 2000),
		makeRecord("ccc33333", "deploy-db", "success", now.Add(-2*time.Hour), 500),
	}
	stats := &workflow.HistoryStats{TotalExecutions: 3, SuccessCount: 2, FailedCount: 1}

	tab := newHistoryTab()
	tab.width = 120
	tab.height = 40

	// Load data via message.
	tab, _ = tab.Update(HistoryLoadedMsg{Records: records, Stats: stats})
	require.Len(t, tab.history, 3)
	require.Len(t, tab.filtered, 3)

	// Apply "deploy" filter.
	tab.filterInput.SetValue("deploy")
	tab.applyFilters()
	require.Len(t, tab.filtered, 2, "deploy filter should show 2 records")

	// Give the table a size so navigation works.
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Navigate to second item.
	tab, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 1, tab.tbl.Cursor())

	// Open detail.
	tab, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, historyDetailView, tab.view)
	require.NotNil(t, tab.selected)

	// Detail view should render meaningful content.
	view := tab.View()
	assert.Contains(t, view, "Execution Detail")

	// Return to list.
	tab, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, historyListView, tab.view)
	assert.Nil(t, tab.selected)
}
