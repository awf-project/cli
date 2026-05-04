package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// workflowsView identifies which sub-view the workflows tab is rendering.
type workflowsView int

const (
	workflowsListView  workflowsView = iota
	workflowsInputView               // input collection form
)

// workflowItem wraps a workflow entry and its optional full definition to satisfy list.Item (and list.DefaultItem).
type workflowItem struct {
	wf    *workflow.Workflow
	entry workflow.WorkflowEntry
}

func (i workflowItem) Title() string {
	return i.entry.Name
}

func (i workflowItem) Description() string {
	parts := []string{}
	if i.entry.Description != "" {
		parts = append(parts, i.entry.Description)
	}
	if i.entry.Source != "" {
		parts = append(parts, i.entry.Source)
	}
	if i.entry.Version != "" {
		parts = append(parts, "v"+i.entry.Version)
	}
	if len(parts) == 0 {
		if i.wf != nil {
			return fmt.Sprintf("%d steps", len(i.wf.Steps))
		}
		return ""
	}
	return strings.Join(parts, " · ")
}

func (i workflowItem) FilterValue() string {
	return i.entry.Name + " " + i.entry.Description + " " + i.entry.Source
}

// WorkflowsTab is the Bubbletea sub-model for the Workflows tab.
type WorkflowsTab struct {
	width  int
	height int
	view   workflowsView

	entries []workflow.WorkflowEntry
	list    list.Model
	bridge  *Bridge
	ctx     context.Context

	validationResult *string

	// Spinner shown during workflow validation.
	spinner    spinner.Model
	validating bool

	// Input form state (workflowsInputView).
	inputTarget   *workflow.Workflow
	inputFields   []textinput.Model
	inputNames    []string
	inputRequired []bool
	inputFocus    int
}

func newWorkflowsTab() WorkflowsTab {
	delegate := list.NewDefaultDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = "Workflows"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ColorRunning)

	return WorkflowsTab{
		list:    l,
		spinner: sp,
	}
}

func (t *WorkflowsTab) setWorkflows(entries []workflow.WorkflowEntry, workflows []*workflow.Workflow) {
	wfMap := make(map[string]*workflow.Workflow, len(workflows))
	for _, wf := range workflows {
		wfMap[wf.Name] = wf
	}

	t.entries = entries
	items := make([]list.Item, len(entries))
	for i, entry := range entries {
		items[i] = workflowItem{wf: wfMap[entry.Name], entry: entry}
	}
	t.list.SetItems(items) //nolint:errcheck // cmd not needed
}

//nolint:gocritic // Bubbletea convention: value receivers
func (t WorkflowsTab) Init() tea.Cmd {
	return nil
}

//nolint:gocritic // Bubbletea convention: value receivers
func (t WorkflowsTab) Update(msg tea.Msg) (WorkflowsTab, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		listHeight := max(msg.Height-3, 1)
		t.list.SetSize(msg.Width, listHeight)
		return t, nil

	case ValidationResultMsg:
		t.validating = false
		t.validationResult = validationOverlayText(msg)
		return t, nil

	case spinner.TickMsg:
		if t.validating {
			var cmd tea.Cmd
			t.spinner, cmd = t.spinner.Update(msg)
			return t, cmd
		}
		return t, nil
	}

	if t.view == workflowsInputView {
		return t.updateInputForm(msg)
	}
	return t.updateListView(msg)
}

//nolint:gocritic // Bubbletea convention: value receivers
func (t WorkflowsTab) updateListView(msg tea.Msg) (WorkflowsTab, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		if handled, tab, cmd := t.handleListKey(keyMsg); handled {
			return tab, cmd
		}
	}
	var cmd tea.Cmd
	t.list, cmd = t.list.Update(msg)
	return t, cmd
}

//nolint:gocritic // Bubbletea convention: value receivers
func (t WorkflowsTab) handleListKey(msg tea.KeyPressMsg) (bool, WorkflowsTab, tea.Cmd) {
	if key.Matches(msg, keyBack) && t.validationResult != nil {
		t.validationResult = nil
		return true, t, nil
	}
	if t.list.FilterState() == list.Filtering {
		return false, t, nil
	}
	switch {
	case key.Matches(msg, keySelect):
		return t.handleLaunch()
	case key.Matches(msg, keyValidate):
		return t.handleValidate()
	}
	return false, t, nil
}

//nolint:gocritic // Bubbletea convention: value receivers
func (t WorkflowsTab) handleLaunch() (bool, WorkflowsTab, tea.Cmd) {
	item, ok := t.selectedWorkflowItem()
	if !ok {
		return true, t, nil
	}

	wf := item.wf
	if wf == nil {
		wf = &workflow.Workflow{Name: item.entry.Name}
	}

	if len(wf.Inputs) == 0 {
		return true, t, func() tea.Msg {
			return LaunchWorkflowMsg{Workflow: wf, Inputs: nil}
		}
	}

	t.openInputForm(wf, wf.Inputs)
	return true, t, nil
}

func (t *WorkflowsTab) openInputForm(wf *workflow.Workflow, inputs []workflow.Input) {
	t.view = workflowsInputView
	t.inputTarget = wf
	t.inputFields = make([]textinput.Model, len(inputs))
	t.inputNames = make([]string, len(inputs))
	t.inputRequired = make([]bool, 0, len(inputs))
	t.inputFocus = 0

	for i, inp := range inputs {
		ti := textinput.New()
		ti.Placeholder = inp.Description
		if inp.Type != "" {
			ti.Placeholder += " (" + inp.Type + ")"
		}
		if inp.Default != nil {
			ti.SetValue(fmt.Sprintf("%v", inp.Default))
		}
		ti.CharLimit = 512
		ti.SetWidth(50)
		if i == 0 {
			ti.Focus()
		}
		t.inputFields[i] = ti
		t.inputNames[i] = inp.Name
		t.inputRequired = append(t.inputRequired, inp.Required)
	}
}

//nolint:gocritic // Bubbletea convention: value receivers
func (t WorkflowsTab) updateInputForm(msg tea.Msg) (WorkflowsTab, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return t, nil
	}

	switch keyMsg.String() {
	case "esc":
		t.view = workflowsListView
		t.inputTarget = nil
		t.inputFields = nil
		t.inputNames = nil
		t.inputRequired = nil
		return t, nil

	case "tab", "down":
		if t.inputFocus < len(t.inputFields)-1 {
			t.inputFields[t.inputFocus].Blur()
			t.inputFocus++
			t.inputFields[t.inputFocus].Focus()
		}
		return t, nil

	case "shift+tab", "up":
		if t.inputFocus > 0 {
			t.inputFields[t.inputFocus].Blur()
			t.inputFocus--
			t.inputFields[t.inputFocus].Focus()
		}
		return t, nil

	case "enter":
		if t.inputFocus < len(t.inputFields)-1 {
			t.inputFields[t.inputFocus].Blur()
			t.inputFocus++
			t.inputFields[t.inputFocus].Focus()
			return t, nil
		}
		return t.submitInputForm()
	}

	var cmd tea.Cmd
	t.inputFields[t.inputFocus], cmd = t.inputFields[t.inputFocus].Update(msg)
	return t, cmd
}

//nolint:gocritic // Bubbletea convention: value receivers
func (t WorkflowsTab) submitInputForm() (WorkflowsTab, tea.Cmd) {
	inputs := make(map[string]any, len(t.inputFields))
	for i, field := range t.inputFields {
		val := strings.TrimSpace(field.Value())
		if val == "" && t.inputRequired[i] {
			return t, nil
		}
		if val != "" {
			inputs[t.inputNames[i]] = val
		}
	}

	wf := t.inputTarget
	t.view = workflowsListView
	t.inputTarget = nil
	t.inputFields = nil
	t.inputNames = nil
	t.inputRequired = nil

	return t, func() tea.Msg {
		return LaunchWorkflowMsg{Workflow: wf, Inputs: inputs}
	}
}

//nolint:gocritic // Bubbletea convention: value receivers
func (t WorkflowsTab) handleValidate() (bool, WorkflowsTab, tea.Cmd) {
	item, ok := t.selectedWorkflowItem()
	if !ok {
		return true, t, nil
	}
	if t.bridge != nil && t.ctx != nil {
		t.validating = true
		tick := t.spinner.Tick
		return true, t, tea.Batch(t.bridge.ValidateWorkflow(t.ctx, item.wf.Name), func() tea.Msg { return tick() })
	}
	return true, t, nil
}

func (t WorkflowsTab) selectedWorkflowItem() (workflowItem, bool) { //nolint:gocritic // read-only
	selected := t.list.SelectedItem()
	if selected == nil {
		return workflowItem{}, false
	}
	item, ok := selected.(workflowItem)
	return item, ok
}

func validationOverlayText(msg ValidationResultMsg) *string {
	var s string
	if msg.Success {
		s = "Validation passed"
	} else {
		s = fmt.Sprintf("Validation failed: %s", msg.Error)
	}
	return &s
}

//nolint:gocritic // Bubbletea convention: value receivers
func (t WorkflowsTab) View() string {
	if t.validationResult != nil {
		overlayContent := *t.validationResult
		return Panel("Validation", overlayContent+"\n\n  Press Escape to dismiss.", min(t.width-4, 60), 6, false)
	}

	if t.validating {
		return fmt.Sprintf("\n  %s Validating...", t.spinner.View())
	}

	if t.view == workflowsInputView {
		return t.renderInputForm()
	}

	if len(t.list.Items()) == 0 && t.list.FilterState() == list.Unfiltered {
		return EmptyStateView("📂", "No workflows found", "Add YAML workflow files to your configured directories.")
	}

	return t.list.View()
}

func (t WorkflowsTab) renderInputForm() string { //nolint:gocritic // read-only
	var b strings.Builder

	for i, name := range t.inputNames {
		prefix := "  "
		if i == t.inputFocus {
			prefix = "▸ "
		}
		b.WriteString(prefix)
		label := name
		if i < len(t.inputRequired) && !t.inputRequired[i] {
			label += " (optional)"
		}
		b.WriteString(StyleInputLabel.Render(label))
		b.WriteString("\n  ")
		b.WriteString(t.inputFields[i].View())
		b.WriteString("\n\n")
	}

	b.WriteString(StyleInputHint.Render("  Tab/↓ next • Shift+Tab/↑ prev • Enter submit • Esc cancel"))

	title := fmt.Sprintf("Launch: %s", t.inputTarget.Name)
	return Panel(title, b.String(), min(t.width-4, 70), len(t.inputNames)*4+4, true)
}

// InputActive reports whether any text input is focused (filter or input form).
func (t WorkflowsTab) InputActive() bool { //nolint:gocritic // read-only
	return t.list.FilterState() == list.Filtering || t.view == workflowsInputView
}
