package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// refreshMsg asks the Model to reload workflows and history.
type refreshMsg struct{}

// WorkflowsScreen lists the available workflows on the left and shows the selected
// one on the right: its inputs, its steps and its most recent runs.
type WorkflowsScreen struct {
	entries []workflow.WorkflowEntry
	wfs     map[string]*workflow.Workflow
	// records is the execution history, newest first, used for the "last run" column
	// and the "Last runs" section.
	records []*workflow.ExecutionRecord

	filter    textinput.Model
	filtering bool
	sel       int

	// form is the input popup, nil when closed.
	form *inputForm
	// validating is true while a validation request is in flight.
	validating bool

	// cfg is the open config view of one workflow, nil while the list is showing.
	cfg *workflowConfig
	// w/h are the size of the screen body, set by the Model; the config view needs the
	// height to bound its scrolling.
	w, h int
}

func newWorkflowsScreen() WorkflowsScreen {
	ti := textinput.New()
	ti.Placeholder = labelWorkflowsFilterHint
	ti.Prompt = ""
	ti.CharLimit = 128
	ti.SetWidth(30)
	return WorkflowsScreen{
		wfs:    make(map[string]*workflow.Workflow),
		filter: ti,
	}
}

// setWorkflows replaces the workflow list. Loaded definitions are matched to entries by name.
func (s *WorkflowsScreen) setWorkflows(entries []workflow.WorkflowEntry, loaded []*workflow.Workflow) {
	s.entries = entries
	s.wfs = make(map[string]*workflow.Workflow, len(loaded))
	for _, wf := range loaded {
		s.wfs[wf.Name] = wf
	}
	s.clampSelection()
}

// setHistory stores the execution history (any order); it is kept newest first.
func (s *WorkflowsScreen) setHistory(records []*workflow.ExecutionRecord) {
	s.records = sortedNewestFirst(records)
}

// visible returns the entries matching the filter, in list order.
func (s *WorkflowsScreen) visible() []workflow.WorkflowEntry {
	needle := strings.ToLower(strings.TrimSpace(s.filter.Value()))
	if needle == "" {
		return s.entries
	}
	var out []workflow.WorkflowEntry
	for i := range s.entries {
		e := &s.entries[i]
		if strings.Contains(strings.ToLower(e.Name+" "+e.Description), needle) {
			out = append(out, *e)
		}
	}
	return out
}

func (s *WorkflowsScreen) clampSelection() {
	s.sel = max(0, min(s.sel, len(s.visible())-1))
}

func (s *WorkflowsScreen) selected() (workflow.WorkflowEntry, bool) {
	v := s.visible()
	if s.sel < 0 || s.sel >= len(v) {
		return workflow.WorkflowEntry{}, false
	}
	return v[s.sel], true
}

// lastRuns returns up to n most recent runs of the named workflow.
func (s *WorkflowsScreen) lastRuns(name string, n int) []*workflow.ExecutionRecord {
	var out []*workflow.ExecutionRecord
	for _, r := range s.records {
		if r.WorkflowName == name {
			out = append(out, r)
			if len(out) == n {
				break
			}
		}
	}
	return out
}

// InConfig reports whether a workflow's config view is open.
func (s *WorkflowsScreen) InConfig() bool { return s.cfg != nil }

// SetSize tells the screen the size of its body.
func (s *WorkflowsScreen) SetSize(w, h int) {
	s.w, s.h = w, h
	if s.cfg != nil {
		s.cfg.clampOffset(outputRows(h))
	}
}

// InputActive reports whether keys must reach this screen (typing a filter or a form
// field) instead of being treated as global shortcuts.
func (s *WorkflowsScreen) InputActive() bool { return s.filtering || s.form != nil }

// Update handles key presses.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (s WorkflowsScreen) Update(msg tea.Msg) (WorkflowsScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case ValidationResultMsg:
		s.validating = false
		return s, nil
	case WorkflowSourceLoadedMsg:
		// A late reply for a workflow the user already left is dropped.
		if s.cfg != nil && s.cfg.entry.Name == msg.Name {
			s.cfg.loaded(msg)
			s.cfg.clampOffset(outputRows(s.h))
		}
		return s, nil
	case tea.MouseWheelMsg:
		switch tea.Mouse(msg).Button {
		case tea.MouseWheelUp:
			s.scrollConfig(-wheelStep)
		case tea.MouseWheelDown:
			s.scrollConfig(wheelStep)
		}
		return s, nil
	case tea.KeyPressMsg:
		switch {
		case s.form != nil:
			return s.updateForm(msg)
		case s.cfg != nil:
			return s.updateConfig(msg)
		case s.filtering:
			return s.updateFilter(msg)
		}
		return s.updateList(msg)
	}
	return s, nil
}

//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (s WorkflowsScreen) updateList(msg tea.KeyPressMsg) (WorkflowsScreen, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		s.sel = max(0, s.sel-1)
	case "down", "j":
		s.sel = min(len(s.visible())-1, s.sel+1)
		s.sel = max(0, s.sel)
	case "home", "g":
		s.sel = 0
	case "end", "G":
		s.sel = max(0, len(s.visible())-1)
	case "/":
		s.filtering = true
		return s, s.filter.Focus()
	case "esc":
		if s.filter.Value() != "" {
			s.filter.Reset()
			s.clampSelection()
		}
	case "enter":
		if e, ok := s.selected(); ok {
			s.cfg = newWorkflowConfig(e)
			return s, s.cfg.loadRequest()
		}
	case "x":
		if e, ok := s.selected(); ok {
			return s.launchEntry(e)
		}
	case "v":
		if e, ok := s.selected(); ok {
			s.validating = true
			name := e.Name
			return s, func() tea.Msg { return validateRequestMsg{Name: name} }
		}
	case "h":
		if e, ok := s.selected(); ok {
			name := e.Name
			return s, func() tea.Msg { return showHistoryMsg{Workflow: name} }
		}
	case "r":
		return s, func() tea.Msg { return refreshMsg{} }
	}
	return s, nil
}

// showHistoryMsg asks the Model to open the History screen on one workflow's runs.
type showHistoryMsg struct{ Workflow string }

// validateRequestMsg asks the Model to validate a workflow through the Bridge.
type validateRequestMsg struct{ Name string }

//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (s WorkflowsScreen) updateFilter(msg tea.KeyPressMsg) (WorkflowsScreen, tea.Cmd) {
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

// launchEntry runs a workflow, asking for its inputs first when it has some.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (s WorkflowsScreen) launchEntry(e workflow.WorkflowEntry) (WorkflowsScreen, tea.Cmd) {
	wf := s.wfs[e.Name]
	if wf == nil {
		wf = &workflow.Workflow{Name: e.Name}
	}
	if len(wf.Inputs) == 0 {
		return s, func() tea.Msg { return LaunchWorkflowMsg{Workflow: wf} }
	}
	s.form = newInputForm(wf)
	return s, textinput.Blink
}

//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (s WorkflowsScreen) updateForm(msg tea.KeyPressMsg) (WorkflowsScreen, tea.Cmd) {
	result, cmd := s.form.update(msg)
	switch result {
	case formCancelled:
		s.form = nil
		return s, nil
	case formSubmitted:
		wf, inputs := s.form.wf, s.form.values()
		s.form = nil
		return s, func() tea.Msg { return LaunchWorkflowMsg{Workflow: wf, Inputs: inputs} }
	}
	return s, cmd
}

// keys returns the key bar for the current state.
func (s *WorkflowsScreen) keys(w int) string {
	switch {
	case s.form == nil && s.cfg != nil:
		return s.configKeys(w)
	case s.filtering:
		return keyBar(w, keyHint{"enter", labelKeyApply}, keyHint{"esc", labelKeyClear})
	case s.form != nil:
		return keyBar(w, keyHint{"enter", labelFormNextOrRun}, keyHint{"tab", labelFormField}, keyHint{"esc", labelFormCancel})
	}
	return keyBar(w,
		keyHint{"tab", labelKeyTabs}, keyHint{"↑↓", labelKeyNavigate}, keyHint{"enter", labelKeyConfig}, keyHint{"x", labelKeyRun},
		keyHint{"v", labelKeyValidate}, keyHint{"h", labelKeyHistory}, keyHint{"/", labelKeyFilter}, keyHint{"r", labelKeyRefresh}, keyHint{"q", labelKeyQuit})
}

// view draws the list and the detail panel side by side.
func (s *WorkflowsScreen) view(w, h int) string {
	if s.cfg != nil {
		return s.configView(w, h)
	}
	lw := min(44, w*2/5)
	return joinH(s.listBox(lw, h), s.detailBox(w-lw-1, h), 1)
}

func (s *WorkflowsScreen) popup() string {
	if s.form == nil {
		return ""
	}
	return s.form.view()
}

func (s *WorkflowsScreen) listBox(w, h int) string {
	inner := w - 4
	filterView := s.filter.View()
	if !s.filtering && s.filter.Value() == "" {
		filterView = sDim.Render(labelWorkflowsFilterHint)
	}
	lines := []string{sPink.Render("/") + " " + filterView, ""}

	v := s.visible()
	rows := max(1, h-2-3-1) // frame, filter block, footer
	start := 0
	if s.sel >= rows {
		start = s.sel - rows + 1
	}
	if len(v) == 0 {
		lines = append(lines, sDim.Render(labelWorkflowsNone))
	}
	for i := start; i < len(v) && i-start < rows; i++ {
		last := s.lastRuns(v[i].Name, 1)
		icon, when := sDim.Render("○"), sDim.Render(relTime(zeroTime))
		if len(last) > 0 {
			icon, when = statusIcon(last[0].Status), sDim.Render(relTime(last[0].StartedAt))
		}
		if i == s.sel {
			line := row("● "+v[i].Name, relTime(lastStart(last)), inner)
			lines = append(lines, selRow(line, inner, cAccent, true))
			continue
		}
		lines = append(lines, row(icon+" "+v[i].Name, when, inner))
	}
	for len(lines) < h-3 {
		lines = append(lines, "")
	}
	pos := 0
	if len(v) > 0 {
		pos = s.sel + 1
	}
	lines = append(lines[:h-3], row(sDim.Render(fmt.Sprintf(labelWorkflowsCountFmt, len(v))), sDim.Render(fmt.Sprintf("%d/%d", pos, len(v))), inner))
	return box(labelWorkflowsTitle, lines, w, h, !s.filtering)
}

func (s *WorkflowsScreen) detailBox(w, h int) string {
	e, ok := s.selected()
	if !ok {
		return box(labelDetailsTitle, []string{sDim.Render(labelWorkflowsSelect)}, w, h, false)
	}
	inner := w - 4
	lines := []string{sPink.Bold(true).Render(e.Name)}
	if e.Description != "" {
		lines = append(lines, sDim.Render(e.Description))
	}
	lines = append(lines, "")

	wf := s.wfs[e.Name]
	if wf == nil {
		lines = append(lines, sBad.Render(labelWorkflowsUnreadable))
		return box(labelDetailsTitle, lines, w, h, false)
	}

	kv := func(k, v string) string { return sDim.Render(fit(k, 10)) + v }
	scope := e.Source
	if e.Version != "" {
		scope += " · v" + e.Version
	}
	steps := workflow.ExecutionOrder(wf)
	lines = append(lines,
		kv(labelDetailsScope, scope),
		kv(labelDetailsInputs, inputsSummary(wf.Inputs)),
		kv(labelDetailsSteps, fmt.Sprint(len(steps))),
		"", sPurple.Bold(true).Render(labelDetailsStepsTitle))
	for i := range steps {
		branch := "├─"
		if i == len(steps)-1 {
			branch = "└─"
		}
		lines = append(lines, sDim.Render(branch)+fmt.Sprintf(" %d %s ", i+1, steps[i].Name)+sDim.Render("· "+string(steps[i].Type)))
	}

	lines = append(lines, "", sPurple.Bold(true).Render(labelDetailsLastRuns))
	runs := s.lastRuns(e.Name, 3)
	if len(runs) == 0 {
		lines = append(lines, sDim.Render(labelDetailsNoRuns))
	}
	for _, r := range runs {
		dur := fmtDur(durationOf(r))
		lines = append(lines, row(statusIcon(r.Status)+" "+sDim.Render(relTime(r.StartedAt)), dur, inner))
	}
	return box(labelDetailsTitle, lines, w, h, false)
}

// inputsSummary renders "name=default" pairs, masking secret defaults.
func inputsSummary(inputs []workflow.Input) string {
	if len(inputs) == 0 {
		return sDim.Render(labelDetailsNone)
	}
	parts := make([]string, len(inputs))
	for i := range inputs {
		def := "·"
		if inputs[i].Default != nil {
			def = fmt.Sprint(inputs[i].Default)
		}
		parts[i] = sPurple.Render(inputs[i].Name) + "=" + maskValue(inputs[i].Name, def)
	}
	return strings.Join(parts, "  ")
}
