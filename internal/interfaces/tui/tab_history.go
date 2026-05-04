package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// historyView identifies which sub-view the history tab is rendering.
type historyView int

const (
	historyListView   historyView = iota
	historyDetailView historyView = iota
)

// statusFilter controls which execution statuses are shown in the list.
type statusFilter int

const (
	statusFilterAll     statusFilter = iota
	statusFilterSuccess statusFilter = iota
	statusFilterFailed  statusFilter = iota
)

// statusFilterLabel returns the human-readable label for a statusFilter value.
func (f statusFilter) label() string {
	switch f {
	case statusFilterSuccess:
		return "success"
	case statusFilterFailed:
		return "failed"
	default:
		return "all"
	}
}

// HistoryTab is the Bubbletea sub-model for the History tab.
// It provides a filterable table of execution records with a detail view.
type HistoryTab struct {
	width  int
	height int

	// Data — set by the Model from HistoryLoadedMsg.
	// history is the authoritative sorted slice (newest-first); accessed directly
	// by model.go and model_test.go via the field name for backward compatibility.
	history  []*workflow.ExecutionRecord
	stats    *workflow.HistoryStats
	filtered []*workflow.ExecutionRecord // parallel to table rows for Enter→detail mapping

	// List view state.
	view         historyView
	filterInput  textinput.Model
	statusFilter statusFilter
	tbl          table.Model

	// Detail view state.
	selected *workflow.ExecutionRecord
	detail   viewport.Model
}

func newHistoryTab() HistoryTab {
	ti := textinput.New()
	ti.Placeholder = "Filter by workflow name..."
	ti.CharLimit = 128
	ti.SetWidth(40)

	cols := []table.Column{
		{Title: "", Width: 12},         // status badge
		{Title: "Workflow", Width: 30}, // workflow name
		{Title: "ID", Width: 10},       // short execution ID
		{Title: "Started", Width: 18},  // formatted start time
		{Title: "Duration", Width: 10}, // duration in ms
	}
	km := table.DefaultKeyMap()
	km.PageDown.SetKeys("pgdown")
	km.PageUp.SetKeys("pgup")
	km.HalfPageDown.SetKeys("ctrl+d")
	km.HalfPageUp.SetKeys("ctrl+u")
	km.GotoTop.SetKeys("home")
	km.GotoBottom.SetKeys("end")

	tbl := table.New(
		table.WithColumns(cols),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(10),
		table.WithKeyMap(km),
	)
	s := table.DefaultStyles()
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("255")).
		Background(ColorPrimary).
		Bold(true)
	tbl.SetStyles(s)

	vp := viewport.New()

	return HistoryTab{
		filterInput: ti,
		tbl:         tbl,
		detail:      vp,
	}
}

// setRecords replaces the authoritative record list, sorts newest-first, and
// reapplies the current filters. Called by the Model when HistoryLoadedMsg arrives.
func (t *HistoryTab) setRecords(records []*workflow.ExecutionRecord, stats *workflow.HistoryStats) {
	t.stats = stats

	// Defensive copy so the tab owns its slice.
	cp := make([]*workflow.ExecutionRecord, len(records))
	copy(cp, records)

	// Sort newest-first by StartedAt.
	sort.Slice(cp, func(i, j int) bool {
		return cp[i].StartedAt.After(cp[j].StartedAt)
	})
	t.history = cp
	t.applyFilters()
}

// applyFilters rebuilds t.filtered from t.history according to the active
// statusFilter and the text input value, then refreshes the table rows.
func (t *HistoryTab) applyFilters() {
	t.filtered = nil
	needle := strings.ToLower(t.filterInput.Value())

	for _, rec := range t.history {
		// Status filter.
		switch t.statusFilter {
		case statusFilterAll:
			// No status filter — include all records.
		case statusFilterSuccess:
			if rec.Status != "success" {
				continue
			}
		case statusFilterFailed:
			if rec.Status != "failed" {
				continue
			}
		}

		// Text filter — substring match on workflow name (case-insensitive).
		if needle != "" && !strings.Contains(strings.ToLower(rec.WorkflowName), needle) {
			continue
		}

		t.filtered = append(t.filtered, rec)
	}

	t.rebuildTableRows()
}

// rebuildTableRows populates the table with the current filtered records.
func (t *HistoryTab) rebuildTableRows() {
	rows := make([]table.Row, len(t.filtered))
	for i, rec := range t.filtered {
		shortID := rec.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		started := "—"
		if !rec.StartedAt.IsZero() {
			started = rec.StartedAt.Format("2006-01-02 15:04")
		}
		dur := fmt.Sprintf("%dms", rec.DurationMs)
		rows[i] = table.Row{StatusBadgeFromString(rec.Status), rec.WorkflowName, shortID, started, dur}
	}
	t.tbl.SetRows(rows)
}

// cycleStatusFilter advances the status filter by one step.
func (t *HistoryTab) cycleStatusFilter() {
	switch t.statusFilter {
	case statusFilterAll:
		t.statusFilter = statusFilterSuccess
	case statusFilterSuccess:
		t.statusFilter = statusFilterFailed
	default:
		t.statusFilter = statusFilterAll
	}
	t.applyFilters()
}

// openDetail switches to the detail view for the record at the table cursor.
func (t *HistoryTab) openDetail() {
	cursor := t.tbl.Cursor()
	if cursor < 0 || cursor >= len(t.filtered) {
		return
	}
	t.selected = t.filtered[cursor]
	t.view = historyDetailView
	t.detail.SetContent(t.renderDetailContent())
	t.detail.GotoTop()
}

// closeDetail returns to the list view.
func (t *HistoryTab) closeDetail() {
	t.view = historyListView
	t.selected = nil
}

// Init implements the Bubbletea sub-model convention.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (t HistoryTab) Init() tea.Cmd {
	return nil
}

// Update implements the Bubbletea sub-model convention.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (t HistoryTab) Update(msg tea.Msg) (HistoryTab, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		availH := t.availableListHeight()
		t.tbl.SetHeight(availH)
		t.tbl.SetWidth(msg.Width)
		t.detail = viewport.New(viewport.WithWidth(msg.Width), viewport.WithHeight(availH))
		if t.selected != nil {
			t.detail.SetContent(t.renderDetailContent())
		}
		return t, nil

	case HistoryLoadedMsg:
		t.setRecords(msg.Records, msg.Stats)
		return t, nil

	case tea.KeyPressMsg:
		// Detail view handles Escape and viewport scrolling.
		if t.view == historyDetailView {
			if key.Matches(msg, keyBack) {
				t.closeDetail()
				return t, nil
			}
			var vpCmd tea.Cmd
			t.detail, vpCmd = t.detail.Update(msg)
			return t, vpCmd
		}

		// When the filter input is focused, route keys to it.
		if t.filterInput.Focused() {
			switch {
			case key.Matches(msg, keyBack), key.Matches(msg, keySelect):
				t.filterInput.Blur()
				return t, nil
			default:
				var tiCmd tea.Cmd
				t.filterInput, tiCmd = t.filterInput.Update(msg)
				t.applyFilters()
				return t, tiCmd
			}
		}

		// List view key handling (filter input not focused).
		switch {
		case key.Matches(msg, keyFilter):
			t.filterInput.Focus()
			return t, nil
		case key.Matches(msg, keyCycleFilter):
			t.cycleStatusFilter()
			return t, nil
		case key.Matches(msg, keySelect):
			t.openDetail()
			return t, nil
		}
	}

	// Delegate remaining keys (up/down, page up/down) to the table.
	var tblCmd tea.Cmd
	t.tbl, tblCmd = t.tbl.Update(msg)
	return t, tblCmd
}

// InputActive reports whether the filter text input is focused.
func (t HistoryTab) InputActive() bool { //nolint:gocritic // read-only
	return t.filterInput.Focused()
}

// View renders the history tab content.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (t HistoryTab) View() string {
	if t.view == historyDetailView && t.selected != nil {
		return t.renderDetailView()
	}
	return t.renderListView()
}

// renderListView renders the filter bar, status badges, table, and stats footer.
func (t HistoryTab) renderListView() string { //nolint:gocritic // value receiver: read-only view
	var b strings.Builder

	filterLine := "  Filter: " + t.filterInput.View() + "   "

	allBadge := lipgloss.NewStyle().Foreground(ColorMuted).Render("[All]")
	successBadge := lipgloss.NewStyle().Foreground(ColorMuted).Render("[✓ Success]")
	failedBadge := lipgloss.NewStyle().Foreground(ColorMuted).Render("[✗ Failed]")

	activeBadgeStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	switch t.statusFilter {
	case statusFilterAll:
		allBadge = activeBadgeStyle.Render("[All]")
	case statusFilterSuccess:
		successBadge = activeBadgeStyle.Render("[✓ Success]")
	case statusFilterFailed:
		failedBadge = activeBadgeStyle.Render("[✗ Failed]")
	}

	filterLine += allBadge + " " + successBadge + " " + failedBadge + "  (Tab to cycle)"
	b.WriteString(StyleHeader.Render(filterLine))
	b.WriteString("\n")
	b.WriteString(Separator(max(t.width, 1)))
	b.WriteString("\n")

	if len(t.filtered) == 0 {
		b.WriteString(EmptyStateView("📜", "No execution history found", "Run some workflows to populate history."))
		return b.String()
	}

	b.WriteString(t.tbl.View())
	b.WriteString("\n")

	if t.stats != nil {
		b.WriteString(Separator(max(t.width, 1)))
		b.WriteString("\n")
		successStat := lipgloss.NewStyle().Foreground(ColorSuccess).Render(fmt.Sprintf("Success: %d", t.stats.SuccessCount))
		failedStat := lipgloss.NewStyle().Foreground(ColorError).Render(fmt.Sprintf("Failed: %d", t.stats.FailedCount))
		footer := fmt.Sprintf("  Total: %d  %s  %s  Cancelled: %d  Avg: %dms",
			t.stats.TotalExecutions, successStat, failedStat,
			t.stats.CancelledCount, t.stats.AvgDurationMs)
		b.WriteString(footer)
		b.WriteString("\n")
	}

	return b.String()
}

// renderDetailView renders a read-only detail panel for the selected record.
func (t HistoryTab) renderDetailView() string { //nolint:gocritic // value receiver: read-only view
	var b strings.Builder
	header := StyleHeader.Render(fmt.Sprintf("  Execution Detail — %s  (Esc to return)", t.selected.ID))
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(Separator(max(t.width, 1)))
	b.WriteString("\n")
	b.WriteString(t.detail.View())
	return b.String()
}

// renderDetailContent builds the text content placed inside the detail viewport.
func (t HistoryTab) renderDetailContent() string { //nolint:gocritic // value receiver: read-only view
	if t.selected == nil {
		return ""
	}
	rec := t.selected

	var b strings.Builder
	fmt.Fprintf(&b, "  %s  %s\n\n", StatusBadgeFromString(rec.Status), rec.WorkflowName)
	fmt.Fprintf(&b, "  Execution ID : %s\n", rec.ID)
	fmt.Fprintf(&b, "  Workflow ID  : %s\n", rec.WorkflowID)
	fmt.Fprintf(&b, "  Status       : %s\n", StatusBadgeFromString(rec.Status))
	fmt.Fprintf(&b, "  Exit Code    : %d\n", rec.ExitCode)

	if !rec.StartedAt.IsZero() {
		fmt.Fprintf(&b, "  Started      : %s\n", rec.StartedAt.Format(time.RFC3339))
	}
	if !rec.CompletedAt.IsZero() {
		fmt.Fprintf(&b, "  Completed    : %s\n", rec.CompletedAt.Format(time.RFC3339))
	}
	fmt.Fprintf(&b, "  Duration     : %dms\n", rec.DurationMs)

	if rec.ErrorMessage != "" {
		errStyle := lipgloss.NewStyle().Foreground(ColorError)
		fmt.Fprintf(&b, "\n  %s\n    %s\n", errStyle.Render("Error:"), rec.ErrorMessage)
	}

	return b.String()
}

// availableListHeight returns the number of data rows available for the table.
// It subtracts all surrounding chrome: model tab bar + separator (3),
// filter header + separator (2), table column header (1, rendered by table.Model),
// stats footer + separator (2), and help bar (1).
func (t HistoryTab) availableListHeight() int { //nolint:gocritic // value receiver: read-only
	h := t.height - 9
	if h < 1 {
		h = 1
	}
	return h
}
