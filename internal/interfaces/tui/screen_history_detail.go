package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// wheelStep is how many lines one mouse wheel notch scrolls.
const wheelStep = 3

// historyDetail is the open detail of one past execution: its steps on the left and the
// selected step's error, stderr and output on the right.
type historyDetail struct {
	record  *workflow.ExecutionRecord
	loading bool
	// err is why the detail could not be loaded ("" when it was).
	err    string
	detail *RunDetail
	sel    int
	// offset is the first visible line of the output panel.
	offset int
}

func newHistoryDetail(rec *workflow.ExecutionRecord) *historyDetail {
	return &historyDetail{record: rec, loading: true}
}

// loaded applies a load result. The first failed step is preselected so the reason a run
// failed is on screen immediately.
func (d *historyDetail) loaded(msg RunDetailLoadedMsg) {
	d.loading = false
	d.err = msg.Err
	d.detail = msg.Detail
	d.sel, d.offset = 0, 0
	if d.detail == nil {
		return
	}
	for i := range d.detail.Steps {
		st := &d.detail.Steps[i].State
		if st.Status == workflow.StatusFailed || st.Error != "" {
			d.sel = i
			return
		}
	}
}

func (d *historyDetail) steps() []StepDetail {
	if d.detail == nil {
		return nil
	}
	return d.detail.Steps
}

func (d *historyDetail) selected() (*StepDetail, bool) {
	steps := d.steps()
	if d.sel < 0 || d.sel >= len(steps) {
		return nil, false
	}
	return &steps[d.sel], true
}

// lines renders the selected step's content as lines of the output panel.
func (d *historyDetail) lines() []string {
	step, ok := d.selected()
	if !ok {
		return nil
	}
	st := &step.State
	var sb strings.Builder
	switch {
	case st.Conversation != nil && len(st.Conversation.Turns) > 0:
		writeConversation(&sb, st, "")
	case st.Output == "" && st.Stderr == "" && st.Error == "":
		// Nothing was captured. The live run view shows "waiting" or "running" here; for a
		// run that is over, say what actually happened to the step.
		sb.WriteString(StyleBlockDim.Render("  " + emptyStepLabel(st.Status)))
		sb.WriteString("\n")
	default:
		writeStepResult(&sb, st, nil)
	}
	if st.Truncated {
		sb.WriteString(StyleBlockDim.Render("  " + labelDetailTruncated))
		sb.WriteString("\n")
	}
	return strings.Split(strings.TrimRight(sb.String(), "\n"), "\n")
}

// emptyStepLabel explains a step that has no captured output, given how it ended.
func emptyStepLabel(status workflow.ExecutionStatus) string {
	switch status { //nolint:exhaustive // every other status simply produced no output
	case workflow.StatusPending:
		return labelDetailDidNotRun
	case workflow.StatusRunning:
		return labelDetailInterrupted
	}
	return labelDetailNoOutput
}

// clampOffset keeps the scroll position inside the content for a panel of rows lines.
func (d *historyDetail) clampOffset(rows int) {
	maxOffset := max(0, len(d.lines())-rows)
	d.offset = min(max(d.offset, 0), maxOffset)
}

func (d *historyDetail) moveSelection(delta int) {
	next := d.sel + delta
	if next < 0 || next >= len(d.steps()) {
		return
	}
	d.sel, d.offset = next, 0
}

// loadRequest builds the message that makes the Model (re)load this detail.
func (d *historyDetail) loadRequest() tea.Cmd {
	rec := d.record
	return func() tea.Msg { return loadRunDetailMsg{Record: rec} }
}

// copyText is the selected step's error, stderr and output as plain text.
func (d *historyDetail) copyText() string {
	step, ok := d.selected()
	if !ok {
		return ""
	}
	return stepCopyText(&step.State, nil)
}

// outputRows is how many output lines fit in the panel for a body of height h: one line
// for the summary and two for the box border.
func outputRows(h int) int { return max(1, h-1-2) }

// updateDetail handles keys while a detail is open.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (s HistoryScreen) updateDetail(msg tea.KeyPressMsg) (HistoryScreen, tea.Cmd) {
	d := s.open
	rows := outputRows(s.h)
	switch msg.String() {
	case "esc":
		s.open = nil
		return s, nil
	case "up", "k":
		d.moveSelection(-1)
	case "down", "j":
		d.moveSelection(1)
	case "pgup":
		d.offset -= max(1, rows-1)
	case "pgdown", " ":
		d.offset += max(1, rows-1)
	case "home", "g":
		d.offset = 0
	case "end", "G":
		d.offset = len(d.lines())
	case "r":
		d.loading, d.err = true, ""
		return s, d.loadRequest()
	case "y":
		return s, copyCmd(d.copyText())
	}
	d.clampOffset(rows)
	return s, nil
}

// scrollDetail scrolls the output panel by delta lines (mouse wheel).
func (s *HistoryScreen) scrollDetail(delta int) {
	if s.open == nil {
		return
	}
	s.open.offset += delta
	s.open.clampOffset(outputRows(s.h))
}

// copyCmd puts text on the clipboard and tells the user what happened.
func copyCmd(text string) tea.Cmd {
	if text == "" {
		return func() tea.Msg { return flashMsg{Text: labelRunNothingToCopy} }
	}
	return tea.Batch(
		tea.SetClipboard(text),
		func() tea.Msg { return flashMsg{Text: fmt.Sprintf(labelRunCopiedFmt, len(text))} },
	)
}

func (s *HistoryScreen) detailKeys(w int) string {
	return keyBar(w,
		keyHint{"↑↓", labelKeyPickStep}, keyHint{"pgup/dn", labelKeyScroll},
		keyHint{"y", labelKeyCopy}, keyHint{"r", labelKeyReload}, keyHint{"esc", labelKeyBack})
}

// detailView draws the open detail: a summary line over the Steps and Output boxes.
func (s *HistoryScreen) detailView(w, h int) string {
	rec := s.open.record
	left := statusIcon(rec.Status) + " " + sPink.Bold(true).Render(rec.WorkflowName) + " " + sDim.Render(shortID(rec.ID))
	right := sDim.Render(relTime(rec.StartedAt) + " · " + fmtDur(durationOf(rec)))

	lw := max(min(38, w/3), 4)
	bh := max(h-1, 3)
	return row(left, right, w) + "\n" + joinH(s.stepsBox(lw, bh), s.outputBox(w-lw-1, bh), 1)
}

func (s *HistoryScreen) stepsBox(w, h int) string {
	d := s.open
	inner := w - 4
	var lines []string
	switch {
	case d.loading:
		lines = append(lines, sDim.Render(labelDetailLoading))
	case d.err != "":
		lines = append(lines, sBad.Render("✗"))
	case len(d.steps()) == 0:
		lines = append(lines, sDim.Render(labelDetailNoSteps))
	}
	for i := range d.steps() {
		step := &d.steps()[i]
		right := sDim.Render(string(step.State.Status))
		if step.Duration > 0 {
			right = sDim.Render(fmtDur(step.Duration))
		}
		line := row(fmt.Sprintf("%s %d %s", statusIcon(string(step.State.Status)), i+1, step.State.Name), right, inner)
		if i == d.sel {
			line = selRow(line, inner, cSelBg, true)
		}
		lines = append(lines, line)
	}

	if d.detail != nil && len(d.detail.Inputs) > 0 {
		keys := sortedKeys(d.detail.Inputs)
		for len(lines) < h-2-len(keys)-1 {
			lines = append(lines, "")
		}
		lines = append(lines, sDim.Render(labelRunInputsTitle))
		for _, k := range keys {
			lines = append(lines, sPurple.Render(k)+" "+maskValue(k, fmt.Sprint(d.detail.Inputs[k])))
		}
	}
	return box(labelRunStepsTitle, lines, w, h, false)
}

func (s *HistoryScreen) outputBox(w, h int) string {
	d := s.open
	title := labelRunOutputTitle
	if step, ok := d.selected(); ok {
		title = fmt.Sprintf("%s · %s", labelRunOutputTitle, step.State.Name)
	}

	switch {
	case d.loading:
		return box(title, []string{sDim.Render(labelDetailLoading)}, w, h, true)
	case d.err != "":
		return box(title, []string{sBad.Render(d.err)}, w, h, true)
	case len(d.steps()) == 0:
		return box(title, []string{sDim.Render(labelDetailNoStepsHint)}, w, h, true)
	}

	all := d.lines()
	rows := max(1, h-2)
	start := min(max(d.offset, 0), max(0, len(all)-rows))
	end := min(start+rows, len(all))
	visible := append([]string(nil), all[start:end]...)
	if len(all) > rows {
		title += " " + sDim.Render(fmt.Sprintf("(%d–%d/%d)", start+1, end, len(all)))
	}
	return box(title, visible, w, h, true)
}
