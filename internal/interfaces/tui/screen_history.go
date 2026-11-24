package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// zeroTime renders as "never" through relTime.
var zeroTime time.Time

// sortedNewestFirst returns a copy of records ordered by start time, newest first.
func sortedNewestFirst(records []*workflow.ExecutionRecord) []*workflow.ExecutionRecord {
	out := make([]*workflow.ExecutionRecord, len(records))
	copy(out, records)
	sort.SliceStable(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out
}

// lastStart returns the start time of the first record, or the zero time.
func lastStart(records []*workflow.ExecutionRecord) time.Time {
	if len(records) == 0 {
		return zeroTime
	}
	return records[0].StartedAt
}

func durationOf(r *workflow.ExecutionRecord) time.Duration {
	return time.Duration(r.DurationMs) * time.Millisecond
}

// statusFilter narrows the history list to one outcome.
type statusFilter int

const (
	statusFilterAll statusFilter = iota
	statusFilterSuccess
	statusFilterFailed
)

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

func (f statusFilter) next() statusFilter { return (f + 1) % 3 }

func (f statusFilter) matches(status string) bool {
	switch f {
	case statusFilterSuccess:
		return status == "success"
	case statusFilterFailed:
		return status == "failed"
	default:
		return true
	}
}

// HistoryScreen lists past executions on the left and details the selected one on the right.
type HistoryScreen struct {
	records []*workflow.ExecutionRecord // newest first
	stats   *workflow.HistoryStats

	filter    textinput.Model
	filtering bool
	status    statusFilter
	sel       int

	// open is the detail of one execution, nil while the list is showing.
	open *historyDetail
	// w/h are the size of the screen body, set by the Model; the detail needs the height
	// to bound its scrolling.
	w, h int
}

func newHistoryScreen() HistoryScreen {
	ti := textinput.New()
	ti.Placeholder = labelHistoryFilterHint
	ti.Prompt = ""
	ti.CharLimit = 128
	ti.SetWidth(30)
	return HistoryScreen{filter: ti}
}

func (s *HistoryScreen) setRecords(records []*workflow.ExecutionRecord, stats *workflow.HistoryStats) {
	s.records = sortedNewestFirst(records)
	s.stats = stats
	s.clampSelection()
}

// visible returns the records passing the name filter and the status filter.
func (s *HistoryScreen) visible() []*workflow.ExecutionRecord {
	needle := strings.ToLower(strings.TrimSpace(s.filter.Value()))
	var out []*workflow.ExecutionRecord
	for _, r := range s.records {
		if !s.status.matches(r.Status) {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(r.WorkflowName), needle) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func (s *HistoryScreen) clampSelection() {
	s.sel = max(0, min(s.sel, len(s.visible())-1))
}

func (s *HistoryScreen) selected() (*workflow.ExecutionRecord, bool) {
	v := s.visible()
	if s.sel < 0 || s.sel >= len(v) {
		return nil, false
	}
	return v[s.sel], true
}

// InputActive reports whether the filter is capturing keys.
func (s *HistoryScreen) InputActive() bool { return s.filtering }

// InDetail reports whether an execution's detail is open.
func (s *HistoryScreen) InDetail() bool { return s.open != nil }

// SetSize tells the screen the size of its body.
func (s *HistoryScreen) SetSize(w, h int) {
	s.w, s.h = w, h
	if s.open != nil {
		s.open.clampOffset(outputRows(h))
	}
}

// Update handles key presses.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (s HistoryScreen) Update(msg tea.Msg) (HistoryScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case RunDetailLoadedMsg:
		// A late reply for an execution the user already left is dropped.
		if s.open != nil && s.open.record.ID == msg.RecordID {
			s.open.loaded(msg)
			s.open.clampOffset(outputRows(s.h))
		}
		return s, nil
	case tea.MouseWheelMsg:
		switch tea.Mouse(msg).Button {
		case tea.MouseWheelUp:
			s.scrollDetail(-wheelStep)
		case tea.MouseWheelDown:
			s.scrollDetail(wheelStep)
		}
		return s, nil
	case tea.KeyPressMsg:
		return s.updateKey(msg)
	}
	return s, nil
}

//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (s HistoryScreen) updateKey(keyMsg tea.KeyPressMsg) (HistoryScreen, tea.Cmd) {
	if s.open != nil {
		return s.updateDetail(keyMsg)
	}
	if s.filtering {
		return s.updateFilter(keyMsg)
	}
	switch keyMsg.String() {
	case "up", "k":
		s.sel = max(0, s.sel-1)
	case "down", "j":
		s.sel = max(0, min(len(s.visible())-1, s.sel+1))
	case "home", "g":
		s.sel = 0
	case "end", "G":
		s.sel = max(0, len(s.visible())-1)
	case "enter":
		if rec, ok := s.selected(); ok {
			s.open = newHistoryDetail(rec)
			return s, s.open.loadRequest()
		}
	case "/":
		s.filtering = true
		return s, s.filter.Focus()
	case "s":
		s.status = s.status.next()
		s.clampSelection()
	case "esc":
		if s.filter.Value() != "" {
			s.filter.Reset()
			s.clampSelection()
		}
	case "r":
		return s, func() tea.Msg { return refreshMsg{} }
	}
	return s, nil
}

//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (s HistoryScreen) updateFilter(msg tea.KeyPressMsg) (HistoryScreen, tea.Cmd) {
	switch msg.String() {
	case "enter":
		s.filtering = false
		s.filter.Blur()
		return s, nil
	case "esc":
		s.filtering = false
		s.filter.Reset()
		s.filter.Blur()
		s.clampSelection()
		return s, nil
	}
	var cmd tea.Cmd
	s.filter, cmd = s.filter.Update(msg)
	s.sel = 0
	return s, cmd
}

func (s *HistoryScreen) keys(w int) string {
	if s.open != nil {
		return s.detailKeys(w)
	}
	if s.filtering {
		return keyBar(w, keyHint{"enter", labelKeyApply}, keyHint{"esc", labelKeyClear})
	}
	return keyBar(w,
		keyHint{"tab", labelKeyTabs}, keyHint{"↑↓", labelKeyNavigate}, keyHint{"enter", labelKeyOpen}, keyHint{"/", labelKeyFilter},
		keyHint{"s", fmt.Sprintf(labelKeyStatusFmt, s.status.label())}, keyHint{"r", labelKeyRefresh}, keyHint{"q", labelKeyQuit})
}

func (s *HistoryScreen) view(w, h int) string {
	if s.open != nil {
		return s.detailView(w, h)
	}
	lw := min(52, w*2/5)
	return joinH(s.listBox(lw, h), s.detailBox(w-lw-1, h), 1)
}

func (s *HistoryScreen) listBox(w, h int) string {
	inner := w - 4
	filterView := s.filter.View()
	if !s.filtering && s.filter.Value() == "" {
		filterView = sDim.Render(labelHistoryFilterHint)
	}
	lines := []string{sPink.Render("/") + " " + filterView, ""}

	v := s.visible()
	rows := max(1, h-2-3-1)
	start := 0
	if s.sel >= rows {
		start = s.sel - rows + 1
	}
	if len(v) == 0 {
		lines = append(lines, sDim.Render(labelHistoryNone))
	}
	for i := start; i < len(v) && i-start < rows; i++ {
		r := v[i]
		line := row(statusIcon(r.Status)+" "+r.WorkflowName, relTime(r.StartedAt), inner)
		if i == s.sel {
			lines = append(lines, selRow(line, inner, cAccent, true))
			continue
		}
		lines = append(lines, row(statusIcon(r.Status)+" "+r.WorkflowName, sDim.Render(relTime(r.StartedAt)), inner))
	}
	for len(lines) < h-3 {
		lines = append(lines, "")
	}
	pos := 0
	if len(v) > 0 {
		pos = s.sel + 1
	}
	lines = append(lines[:h-3], row(sDim.Render(s.summary()), sDim.Render(fmt.Sprintf("%d/%d", pos, len(v))), inner))
	return box(labelHistoryTitle, lines, w, h, !s.filtering)
}

// summary is the list footer: totals from the stats when available.
func (s *HistoryScreen) summary() string {
	if s.stats != nil && s.stats.TotalExecutions > 0 {
		return fmt.Sprintf(labelHistorySummaryFmt, s.stats.TotalExecutions, s.stats.SuccessCount, s.stats.FailedCount)
	}
	return fmt.Sprintf(labelHistoryCountFmt, len(s.records))
}

func (s *HistoryScreen) detailBox(w, h int) string {
	r, ok := s.selected()
	if !ok {
		return box(labelDetailsTitle, []string{sDim.Render(labelHistorySelect)}, w, h, false)
	}
	inner := w - 4
	kv := func(k, v string) string { return sDim.Render(fit(k, 12)) + v }

	lines := []string{
		sPink.Bold(true).Render(r.WorkflowName),
		statusIcon(r.Status) + " " + statusText(r.Status),
		"",
		kv(labelDetailsRunID, shortID(r.ID)),
		kv(labelDetailsStarted, r.StartedAt.Format("2006-01-02 15:04:05")+" "+sDim.Render("("+relTime(r.StartedAt)+")")),
		kv(labelDetailsDuration, fmtDur(durationOf(r))),
		kv(labelDetailsExitCode, fmt.Sprint(r.ExitCode)),
	}
	if r.Attempt > 1 {
		lines = append(lines, kv(labelDetailsAttempt, fmt.Sprint(r.Attempt)))
	}
	if r.ResumedFromID != "" {
		lines = append(lines, kv(labelDetailsResumed, shortID(r.ResumedFromID)))
	}
	if r.ErrorMessage != "" {
		lines = append(lines, "", sPurple.Bold(true).Render(labelDetailsError))
		wrapped := lipgloss.NewStyle().Width(max(inner, 1)).Render(r.ErrorMessage)
		for _, l := range strings.Split(wrapped, "\n") {
			lines = append(lines, sBad.Render(strings.TrimRight(l, " ")))
		}
	}
	return box(labelDetailsTitle, lines, w, h, false)
}

func statusText(status string) string {
	switch status {
	case "success":
		return sOK.Render("success")
	case "failed":
		return sBad.Render("failed")
	case "cancelled":
		return sDim.Render("cancelled")
	}
	return sDim.Render(status)
}
