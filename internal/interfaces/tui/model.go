package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// Tab identifies a screen of the tab bar.
type Tab int

const (
	// TabWorkflows is the workflow list and detail screen (key "1").
	TabWorkflows Tab = iota
	// TabHistory is the execution history screen (key "2").
	TabHistory
	tabCount
)

// screenMode selects what fills the page: the tabbed screens or the full-frame run view.
type screenMode int

const (
	modeTabs screenMode = iota
	modeRun
)

// clockInterval drives the spinner and the elapsed clock while a run is in flight.
const clockInterval = 100 * time.Millisecond

// clockMsg advances the animation frame. It never carries state: run updates are pushed by
// facade events, not polled.
type clockMsg struct{}

func clockCmd() tea.Cmd {
	return tea.Tick(clockInterval, func(time.Time) tea.Msg { return clockMsg{} })
}

// Model is the root Bubble Tea model. It owns the page chrome (header, tab bar, key bar,
// background) and routes messages to the active screen.
type Model struct {
	mode      screenMode
	activeTab Tab

	// sw/sh are the terminal size; w/h the content area inside the page margins.
	sw, sh int
	w, h   int

	bridge *Bridge
	ctx    context.Context
	// dir is the working directory shown in the header.
	dir string

	// flash is a one-line message in the header, cleared on the next key press.
	flash    string
	flashErr bool

	workflows WorkflowsScreen
	history   HistoryScreen
	run       RunScreen

	// tick is the animation frame shared with the run screen.
	tick int
}

// New creates a Model with sensible zero-value defaults.
// bridge may be nil; Init then skips loading anything.
//
// Until the first WindowSizeMsg arrives the model lays out at the minimum supported size,
// so it always renders something sensible (tests, and the very first frame).
func New() Model {
	m := Model{
		ctx:       context.Background(),
		dir:       tildePath(workingDir()),
		workflows: newWorkflowsScreen(),
		history:   newHistoryScreen(),
		run:       newRunScreen(),
	}
	return m.resize(tea.WindowSizeMsg{Width: minTermWidth, Height: minTermHeight})
}

// NewWithBridge creates a Model wired to the given bridge and context.
func NewWithBridge(bridge *Bridge, ctx context.Context) Model { //nolint:revive // context.Context is intentional here
	m := New()
	m.bridge = bridge
	m.ctx = ctx
	return m
}

func workingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}

// tildePath shortens the home directory to "~" and clips very long paths from the left.
func tildePath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(p, home) {
		p = "~" + p[len(home):]
	}
	if r := []rune(p); len(r) > 50 {
		p = "…" + string(r[len(r)-49:])
	}
	return p
}

// reload fetches workflows and history through the bridge.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (m Model) reload() tea.Cmd {
	if m.bridge == nil {
		return nil
	}
	cmds := []tea.Cmd{}
	if m.bridge.workflows != nil {
		cmds = append(cmds, m.bridge.LoadWorkflows(m.ctx))
	}
	if m.bridge.history != nil {
		cmds = append(cmds, m.bridge.LoadHistory(m.ctx))
	}
	return tea.Batch(cmds...)
}

// Init implements tea.Model. Without a bridge it emits an empty workflow list so the
// screens render their empty state.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (m Model) Init() tea.Cmd {
	if m.bridge == nil {
		return func() tea.Msg { return WorkflowsLoadedMsg{} }
	}
	return m.reload()
}

// Update implements tea.Model.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.resize(msg), nil

	case tea.KeyPressMsg:
		return m.updateKey(msg)

	case clockMsg:
		if !m.run.Running() {
			return m, nil
		}
		m.tick++
		m.run.SetTick(m.tick)
		return m, clockCmd()

	case WorkflowsLoadedMsg:
		m.workflows.setWorkflows(msg.Entries, msg.Workflows)
		return m, nil

	case HistoryLoadedMsg:
		m.history.setRecords(msg.Records, msg.Stats)
		m.workflows.setHistory(msg.Records)
		return m, nil

	case refreshMsg:
		return m, m.reload()

	case loadRunDetailMsg:
		return m.loadRunDetail(msg.Record)

	case RunDetailLoadedMsg:
		m.history, _ = m.history.Update(msg)
		return m, nil

	case flashMsg:
		return m.withFlash(msg.Text, msg.Err), nil

	case validateRequestMsg:
		if m.bridge == nil || m.bridge.workflows == nil {
			m.workflows, _ = m.workflows.Update(ValidationResultMsg{Name: msg.Name})
			return m.withFlash(labelNoBackend, true), nil
		}
		return m, m.bridge.ValidateWorkflow(m.ctx, msg.Name)

	case ValidationResultMsg:
		m.workflows, _ = m.workflows.Update(msg)
		if msg.Success {
			return m.withFlash(fmt.Sprintf(labelValidationOKFmt, msg.Name), false), nil
		}
		return m.withFlash(fmt.Sprintf(labelValidationBadFmt, msg.Name, msg.Error), true), nil

	case LaunchWorkflowMsg:
		return m.launch(msg)

	case ExecutionStartedMsg:
		return m.startRun(msg)

	case ExecutionFinishedMsg:
		var cmd tea.Cmd
		m.run, cmd = m.run.Update(msg)
		// Refresh history so the finished run shows up without relaunching the TUI.
		return m, tea.Batch(cmd, m.reload())

	case ErrMsg:
		if msg.Err != nil {
			return m.withFlash(msg.Err.Error(), true), nil
		}
		return m, nil

	case InputRequestedMsg:
		m.mode = modeRun
		var cmd tea.Cmd
		m.run, cmd = m.run.Update(msg)
		return m, cmd

	case facadeEventMsg:
		// Route facade events to the run screen whichever screen is showing, so step state
		// keeps up while the user browses elsewhere. A prompt pulls the user back to the run.
		if msg.Event.Kind == ports.EventInputRequired {
			m.mode = modeRun
		}
		var cmd tea.Cmd
		m.run, cmd = m.run.Update(msg)
		return m, cmd
	}

	return m.routeToScreen(msg)
}

// routeToScreen forwards a message to the screen that is showing.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (m Model) routeToScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch {
	case m.mode == modeRun:
		m.run, cmd = m.run.Update(msg)
	case m.activeTab == TabHistory:
		m.history, cmd = m.history.Update(msg)
	default:
		m.workflows, cmd = m.workflows.Update(msg)
	}
	return m, cmd
}

// loadRunDetail asks the bridge for the persisted detail of a history record. Without a
// bridge the screen is told why there is nothing to show.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (m Model) loadRunDetail(rec *workflow.ExecutionRecord) (tea.Model, tea.Cmd) {
	if rec == nil {
		return m, nil
	}
	if m.bridge == nil {
		m.history, _ = m.history.Update(RunDetailLoadedMsg{RecordID: rec.ID, Err: labelDetailNoStore})
		return m, nil
	}
	return m, m.bridge.LoadRunDetail(m.ctx, rec.ID, rec.WorkflowID)
}

// resize recomputes the content area and tells the run screen its body size.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (m Model) resize(msg tea.WindowSizeMsg) Model {
	m.sw, m.sh = msg.Width, msg.Height
	m.w = max(m.sw-2*marginX, 0)
	m.h = max(m.sh-2*marginY, 0)
	m.run, _ = m.run.Update(tea.WindowSizeMsg{Width: m.w, Height: m.runBodyHeight()})
	m.history.SetSize(m.w, max(m.h-3, 0))
	return m
}

// runBodyHeight is the height left for the run body: header, blank line, rule and the
// two-line key bar take five rows.
func (m Model) runBodyHeight() int { //nolint:gocritic // read-only
	return max(m.h-5, 0)
}

//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (m Model) withFlash(text string, isErr bool) Model {
	m.flash, m.flashErr = text, isErr
	return m
}

// launch starts a run through the bridge. The screen switches to the run view once the
// run has actually started (ExecutionStartedMsg), so a failed start leaves the user on the
// list with the error in the header.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (m Model) launch(msg LaunchWorkflowMsg) (tea.Model, tea.Cmd) {
	if msg.Workflow == nil {
		return m, nil
	}
	m.run.SetLaunch(msg)
	if m.bridge == nil {
		return m.withFlash(labelNoBackend, true), nil
	}
	return m.withFlash(fmt.Sprintf(labelStartingFmt, msg.Workflow.Name), false),
		m.bridge.RunWorkflowViaFacade(m.ctx, msg.Workflow.Name, msg.Inputs)
}

// startRun switches to the run view and starts consuming the session's events.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (m Model) startRun(msg ExecutionStartedMsg) (tea.Model, tea.Cmd) {
	m.mode = modeRun
	m.flash = ""
	m.tick = 0
	m.run.SetWorkflow(msg.Workflow)
	if msg.Session != nil {
		// The goroutine ranges over Session.Events() and forwards each event as a
		// facadeEventMsg through the sender wired in runTUI.
		m.run.StartEventLoop(m.ctx, msg.Session)
	}
	var cmd tea.Cmd
	m.run, cmd = m.run.Update(msg)
	m.run.SetTick(0)
	cmds := []tea.Cmd{cmd}
	if msg.Session != nil {
		cmds = append(cmds, clockCmd())
	}
	return m, tea.Batch(cmds...)
}

//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (m Model) updateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.flash = ""
	k := msg.String()

	if k == "ctrl+c" {
		if m.mode == modeRun && m.run.Running() {
			m.run.RequestCancel()
			return m, nil
		}
		return m, tea.Quit
	}

	if m.mode == modeRun {
		if k == "esc" {
			m.mode = modeTabs
			return m, nil
		}
		var cmd tea.Cmd
		m.run, cmd = m.run.Update(msg)
		return m, cmd
	}

	// An open execution detail owns the keys (esc leaves it); only quit stays global.
	if m.activeTab == TabHistory && m.history.InDetail() {
		if k == "q" {
			return m, tea.Quit
		}
		return m.routeToScreen(msg)
	}

	if m.screenCapturesKeys() {
		return m.routeToScreen(msg)
	}
	switch k {
	case "q":
		return m, tea.Quit
	case "tab":
		m.activeTab = (m.activeTab + 1) % tabCount
		return m, nil
	case "shift+tab":
		m.activeTab = (m.activeTab + tabCount - 1) % tabCount
		return m, nil
	case "1":
		m.activeTab = TabWorkflows
		return m, nil
	case "2":
		m.activeTab = TabHistory
		return m, nil
	case "o":
		if m.run.HasRun() {
			m.mode = modeRun
		}
		return m, nil
	}
	return m.routeToScreen(msg)
}

// screenCapturesKeys reports whether the active screen is in a text-entry state (filter or
// form), in which case digits, "q" and tab belong to the text field, not to navigation.
//
//nolint:gocritic // Bubbletea convention: value receivers
func (m Model) screenCapturesKeys() bool {
	if m.activeTab == TabHistory {
		return m.history.InputActive()
	}
	return m.workflows.InputActive()
}

// View implements tea.Model.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

//nolint:gocritic // Bubbletea convention: value receivers
func (m Model) render() string {
	if m.sw < minTermWidth || m.sh < minTermHeight {
		return fmt.Sprintf(labelTerminalTooFmt, m.sw, m.sh, minTermWidth, minTermHeight)
	}
	var content string
	if m.mode == modeRun {
		content = m.renderRun()
	} else {
		content = m.renderTabs()
	}
	return paintPage(content, m.sw, m.sh, m.w, m.h)
}

// headerRight is the flash message when there is one, otherwise the given default.
//
//nolint:gocritic // Bubbletea convention: value receivers
func (m Model) headerRight(def string) string {
	switch {
	case m.flash == "":
		return def
	case m.flashErr:
		return sBad.Render(m.flash)
	}
	return sRun.Render(m.flash)
}

//nolint:gocritic // Bubbletea convention: value receivers
func (m Model) renderTabs() string {
	left := sLogo.Render("AWF") + "  " + m.tabBar()
	right := m.headerRight(sDim.Render(m.dir))
	if lipgloss.Width(left)+lipgloss.Width(right)+2 > m.w && m.flash == "" {
		right = ""
	}
	top := row(left, right, m.w)

	bodyH := m.h - 3
	var body, keys, popup string
	if m.activeTab == TabHistory {
		body, keys = m.history.view(m.w, bodyH), m.history.keys(m.w)
	} else {
		body, keys = m.workflows.view(m.w, bodyH), m.workflows.keys(m.w)
		popup = m.workflows.popup()
	}
	out := top + "\n" + body + "\n" + keys
	if popup != "" {
		out = overlay(out, popup, m.w, m.h)
	}
	return out
}

//nolint:gocritic // Bubbletea convention: value receivers
func (m Model) renderRun() string {
	head := sBadge.Render("AWF") + " " + sPink.Bold(true).Render(m.run.title())
	top := row(head, m.headerRight(m.run.statusText()), m.w)
	body := m.run.view(m.w, m.runBodyHeight())
	return top + "\n" + fit("", m.w) + "\n" + ruleLine(m.w) + "\n" + body + "\n" + m.run.keys(m.w)
}

//nolint:gocritic // Bubbletea convention: value receivers
func (m Model) tabBar() string {
	items := []struct {
		label string
		tab   Tab
	}{
		{fmt.Sprintf(labelTabWorkflowsFmt, 1, len(m.workflows.entries)), TabWorkflows},
		{fmt.Sprintf(labelTabHistoryFmt, 2, len(m.history.records)), TabHistory},
	}
	parts := make([]string, len(items))
	for i, it := range items {
		if it.tab == m.activeTab {
			parts[i] = sTabOn.Render(it.label)
		} else {
			parts[i] = sTabOff.Render(it.label)
		}
	}
	return strings.Join(parts, " ")
}
