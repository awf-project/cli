package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// Tab identifies which tab panel is currently active.
type Tab int

const (
	// TabWorkflows is the workflow list and selector tab (key "1").
	TabWorkflows Tab = iota
	// TabMonitoring is the live execution metrics tab (key "2").
	TabMonitoring
	// TabHistory is the execution history tab (key "3").
	TabHistory
	// TabExternalLogs is the tailed external JSONL log tab (key "4").
	TabExternalLogs
)

var tabLabels = [4]string{
	"1:Workflows",
	"2:Monitoring",
	"3:History",
	"4:Logs",
}

// executionState holds live execution data for the monitoring view.
type executionState struct {
	id     string
	status workflow.ExecutionStatus
	steps  map[string]*workflow.StepState
}

// Model is the root Bubbletea model for the TUI.
// It holds all tab sub-models and delegates rendering and update to the active one.
type Model struct {
	activeTab        Tab
	width            int
	height           int
	bridge           *Bridge
	ctx              context.Context
	lastErr          string
	help             help.Model
	showFullHelp     bool
	tabNotifications [4]bool

	tabWorkflows  WorkflowsTab
	tabMonitoring MonitoringTab
	tabHistory    HistoryTab
	tabLogs       LogsTab
}

// New creates a Model with sensible zero-value defaults.
// bridge may be nil; Init will skip the fetch command if so.
func New() Model {
	m := Model{
		activeTab:     TabWorkflows,
		tabWorkflows:  newWorkflowsTab(),
		tabMonitoring: newMonitoringTab(),
		tabHistory:    newHistoryTab(),
		tabLogs:       newLogsTab(""),
		ctx:           context.Background(),
	}
	m.help = help.New()
	return m
}

// NewWithBridge creates a Model wired to the given bridge and context.
// logsPath is the path to the AWF audit JSONL log file (may be empty).
func NewWithBridge(bridge *Bridge, ctx context.Context, logsPath string) Model { //nolint:revive // context.Context is intentional here
	m := New()
	m.bridge = bridge
	m.ctx = ctx
	m.tabWorkflows.bridge = bridge
	m.tabWorkflows.ctx = ctx
	if logsPath != "" {
		m.tabLogs = newLogsTab(logsPath)
	}
	return m
}

// Init implements tea.Model. It returns batched commands that load workflows
// and history via the bridge. If no bridge is configured, emits empty messages.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (m Model) Init() tea.Cmd {
	if m.bridge == nil {
		return func() tea.Msg { return WorkflowsLoadedMsg{} }
	}

	cmds := []tea.Cmd{m.bridge.LoadWorkflows(m.ctx)}
	if m.bridge.history != nil {
		cmds = append(cmds, m.bridge.LoadHistory(m.ctx))
	}
	cmds = append(cmds, m.tabLogs.Init())
	return tea.Batch(cmds...)
}

// Update implements tea.Model and routes messages to the active tab sub-model.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// ctrl+c always quits regardless of input state.
		if key.Matches(msg, keyForceQuit) {
			return m, tea.Quit
		}

		// When the active tab has a focused text input, let the tab handle
		// all key events (including "q" and digit keys) so the user can type.
		if m.isInputActive() {
			return m.updateActiveTab(msg)
		}

		switch {
		case key.Matches(msg, keyTab1):
			m.activeTab = TabWorkflows
			m.tabNotifications[TabWorkflows] = false
			return m, nil
		case key.Matches(msg, keyTab2):
			m.activeTab = TabMonitoring
			m.tabNotifications[TabMonitoring] = false
			return m, nil
		case key.Matches(msg, keyTab3):
			m.activeTab = TabHistory
			m.tabNotifications[TabHistory] = false
			return m, nil
		case key.Matches(msg, keyTab4):
			m.activeTab = TabExternalLogs
			m.tabNotifications[TabExternalLogs] = false
			return m, nil
		case key.Matches(msg, keyQuit):
			return m, tea.Quit
		case key.Matches(msg, keyHelp):
			m.showFullHelp = !m.showFullHelp
			m.help.ShowAll = m.showFullHelp
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.SetWidth(msg.Width)
		// Propagate size to all tabs.
		m.tabWorkflows, _ = m.tabWorkflows.Update(msg)
		m.tabMonitoring, _ = m.tabMonitoring.Update(msg)
		m.tabHistory, _ = m.tabHistory.Update(msg)
		m.tabLogs, _ = m.tabLogs.Update(msg)
		return m, nil

	case WorkflowsLoadedMsg:
		m.tabWorkflows.setWorkflows(msg.Entries, msg.Workflows)
		return m, nil

	case LaunchWorkflowMsg:
		m.activeTab = TabMonitoring
		if m.bridge != nil && msg.Workflow != nil {
			return m, m.bridge.RunWorkflow(m.ctx, msg.Workflow, msg.Inputs)
		}
		return m, nil

	case ValidationResultMsg:
		// Forward the result to the workflows tab for overlay display.
		m.tabWorkflows, _ = m.tabWorkflows.Update(msg)
		return m, nil

	case HistoryLoadedMsg:
		m.tabHistory.setRecords(msg.Records, msg.Stats)
		m.tabMonitoring.history = msg.Records
		m.tabMonitoring.stats = msg.Stats
		return m, nil

	case ExecutionStartedMsg:
		m.tabMonitoring.SetExecCtx(msg.ExecCtx, msg.Workflow)
		if m.bridge != nil && m.bridge.stream != nil {
			m.bridge.stream.Reset()
			m.tabMonitoring.SetStream(m.bridge.stream)
		}
		m.lastErr = ""
		var monCmd tea.Cmd
		m.tabMonitoring, monCmd = m.tabMonitoring.Update(msg)
		return m, tea.Batch(monCmd, WaitForExecution(msg.Done))

	case ExecutionFinishedMsg:
		if msg.Err != nil {
			m.lastErr = msg.Err.Error()
		}
		if m.activeTab != TabMonitoring {
			m.tabNotifications[TabMonitoring] = true
		}
		var monCmd tea.Cmd
		m.tabMonitoring, monCmd = m.tabMonitoring.Update(msg)
		return m, monCmd

	case ErrMsg:
		if msg.Err != nil {
			m.lastErr = msg.Err.Error()
		}
		return m, nil

	case LogBatchMsg:
		m.tabLogs, _ = m.tabLogs.Update(msg)
		return m, nil

	case LogLineMsg:
		m.tabLogs, _ = m.tabLogs.Update(msg)
		return m, nil

	case logsTickMsg:
		var cmd tea.Cmd
		m.tabLogs, cmd = m.tabLogs.Update(msg)
		return m, cmd

	case logRotationMsg:
		m.tabLogs, _ = m.tabLogs.Update(msg)
		return m, nil

	case InputRequestedMsg:
		m.activeTab = TabMonitoring
		var cmd tea.Cmd
		m.tabMonitoring, cmd = m.tabMonitoring.Update(msg)
		return m, cmd

	case tickMsg, executionPollMsg:
		var cmd tea.Cmd
		m.tabMonitoring, cmd = m.tabMonitoring.Update(msg)
		return m, cmd
	}

	// Delegate unhandled messages to the active tab.
	return m.updateActiveTab(msg)
}

// updateActiveTab forwards a message to the currently active tab sub-model.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (m Model) updateActiveTab(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.activeTab {
	case TabWorkflows:
		m.tabWorkflows, cmd = m.tabWorkflows.Update(msg)
	case TabMonitoring:
		m.tabMonitoring, cmd = m.tabMonitoring.Update(msg)
	case TabHistory:
		m.tabHistory, cmd = m.tabHistory.Update(msg)
	case TabExternalLogs:
		m.tabLogs, cmd = m.tabLogs.Update(msg)
	}
	return m, cmd
}

// View implements tea.Model. It renders the tab bar followed by the active tab content.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (m Model) View() tea.View {
	var b strings.Builder

	// Render tab bar using theme styles.
	for i, label := range tabLabels {
		tab := Tab(i)
		displayLabel := label
		if m.tabNotifications[i] {
			displayLabel = label + " ●"
		}
		if tab == m.activeTab {
			b.WriteString(StyleTabActive.Render(displayLabel))
		} else {
			b.WriteString(StyleTabInactive.Render(displayLabel))
		}
	}
	b.WriteString("\n")
	b.WriteString(Separator(m.width))
	b.WriteString("\n")

	// Render active tab content.
	switch m.activeTab {
	case TabWorkflows:
		b.WriteString(m.tabWorkflows.View())
	case TabMonitoring:
		b.WriteString(m.tabMonitoring.View())
	case TabHistory:
		b.WriteString(m.tabHistory.View())
	case TabExternalLogs:
		b.WriteString(m.tabLogs.View())
	default:
		fmt.Fprintf(&b, "unknown tab: %d", m.activeTab)
	}

	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n")
	b.WriteString(m.help.View(m.activeHelpKeys()))

	if m.lastErr != "" {
		b.WriteString("\n")
		b.WriteString(StyleErrorBanner.Render("Error: " + m.lastErr))
	}

	v := tea.NewView(b.String())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// renderStatusBar renders a status bar between the content area and the help bar.
//
//nolint:gocritic // Bubbletea convention: value receiver
func (m Model) renderStatusBar() string {
	var left, right string

	if m.tabMonitoring.ticking && m.tabMonitoring.wf != nil {
		completed := 0
		total := len(m.tabMonitoring.flatNodes)
		for _, node := range m.tabMonitoring.flatNodes {
			switch node.Status { //nolint:exhaustive // only counting terminal statuses
			case workflow.StatusCompleted, workflow.StatusFailed, workflow.StatusCancelled:
				completed++
			}
		}
		left = fmt.Sprintf(
			" %s %s (step %d/%d)",
			StatusBadge(workflow.StatusRunning),
			m.tabMonitoring.wf.Name,
			completed, total,
		)
	}

	wfCount := len(m.tabWorkflows.entries)
	histCount := len(m.tabHistory.history)
	right = fmt.Sprintf("%d workflows · %d executions ", wfCount, histCount)

	return HeaderBar(left, right, m.width)
}

// activeHelpKeys returns the help.KeyMap appropriate for the currently active tab.
//
//nolint:gocritic // Bubbletea convention: value receiver
func (m Model) activeHelpKeys() help.KeyMap {
	switch m.activeTab {
	case TabWorkflows:
		return workflowsHelpKeys{}
	case TabMonitoring:
		return monitoringHelpKeys{}
	case TabHistory:
		return historyHelpKeys{}
	case TabExternalLogs:
		return logsHelpKeys{}
	default:
		return globalHelpKeys{}
	}
}

// isInputActive returns true when the active tab has a focused text input,
// meaning key events should be forwarded to the tab rather than intercepted
// for global shortcuts (tab switching, quit).
//
//nolint:gocritic // Bubbletea convention: value receiver
func (m Model) isInputActive() bool {
	switch m.activeTab {
	case TabWorkflows:
		return m.tabWorkflows.InputActive()
	case TabHistory:
		return m.tabHistory.InputActive()
	case TabMonitoring:
		return m.tabMonitoring.InputActive()
	case TabExternalLogs:
		return false
	}
	return false
}
