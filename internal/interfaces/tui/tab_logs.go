package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	maxLogEntries       = 1000
	logsPollingInterval = 200 * time.Millisecond
)

// Lipgloss styles for the logs tab.
var (
	logsHeaderStyle = StyleHeader
	logsEmptyStyle  = StyleEmptyState

	logsEventStartedStyle   = lipgloss.NewStyle().Foreground(ColorRunning)
	logsEventCompletedStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	logsErrorStyle          = lipgloss.NewStyle().Foreground(ColorError)
)

// logsTickMsg is sent by the polling timer to trigger a Next() read.
type logsTickMsg struct{}

// LogsTab displays external JSONL log entries in the TUI.
// It polls the watched file at a fixed interval using an offset-based Tailer,
// and renders entries in a scrollable viewport.
type LogsTab struct {
	width  int
	height int

	path    string
	tailer  *Tailer
	entries []LogEntry

	viewport   viewport.Model
	autoScroll bool
	watching   bool

	// rotationNotice is shown after a file deletion/rotation event.
	rotationNotice string
}

func newLogsTab(path string) LogsTab {
	vp := viewport.New()
	return LogsTab{
		path:       path,
		tailer:     NewTailer(path),
		viewport:   vp,
		autoScroll: true,
		watching:   path != "",
	}
}

// tick schedules a polling wakeup after logsPollingInterval.
func logsTickCmd() tea.Cmd {
	return tea.Tick(logsPollingInterval, func(time.Time) tea.Msg {
		return logsTickMsg{}
	})
}

// Init implements the Bubbletea sub-model convention.
// It loads the tail of the log file and starts the self-sustaining tick loop.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (t LogsTab) Init() tea.Cmd {
	if t.path == "" {
		return nil
	}
	return tea.Batch(t.tailer.Tail(), logsTickCmd())
}

// Update implements the Bubbletea sub-model convention.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (t LogsTab) Update(msg tea.Msg) (LogsTab, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		t.resizeViewport()
		return t, nil

	case LogBatchMsg:
		t.entries = append(t.entries, msg.Entries...)
		if len(t.entries) > maxLogEntries {
			t.entries = t.entries[len(t.entries)-maxLogEntries:]
		}
		t.viewport.SetContent(t.renderEntries())
		if t.autoScroll {
			t.viewport.GotoBottom()
		}
		return t, nil

	case LogLineMsg:
		t.entries = append(t.entries, msg.Entry)
		if len(t.entries) > maxLogEntries {
			t.entries = t.entries[len(t.entries)-maxLogEntries:]
		}
		t.viewport.SetContent(t.renderEntries())
		if t.autoScroll {
			t.viewport.GotoBottom()
		}
		return t, nil

	case logsTickMsg:
		if t.watching && t.path != "" {
			return t, tea.Batch(t.tailer.Follow(), logsTickCmd())
		}
		return t, nil

	case logRotationMsg:
		t.rotationNotice = fmt.Sprintf("Log file rotated or removed: %s. Waiting for re-creation…", msg.path)
		t.viewport.SetContent(t.renderEntries())
		return t, nil

	case tea.KeyPressMsg:
		if key.Matches(msg, keyFollow) {
			t.autoScroll = true
			t.viewport.GotoBottom()
			return t, nil
		}
		var vpCmd tea.Cmd
		t.viewport, vpCmd = viewportAutoScroll(t.viewport, &t.autoScroll, msg)
		return t, vpCmd
	}

	return t, nil
}

// View renders the logs tab content.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (t LogsTab) View() string {
	if t.path == "" && len(t.entries) == 0 {
		return EmptyStateView("📋", "No AWF audit log found", "Expected location: $XDG_DATA_HOME/awf/audit.jsonl")
	}

	var b strings.Builder

	// Header showing path.
	if t.path != "" {
		header := fmt.Sprintf("  Log: %s  (%d entries)", t.path, len(t.entries))
		b.WriteString(logsHeaderStyle.Render(header))
		b.WriteString("\n")
		b.WriteString(Separator(max(t.width, 1)))
		b.WriteString("\n")
	}

	if t.rotationNotice != "" {
		b.WriteString(logsEmptyStyle.Render(t.rotationNotice))
		b.WriteString("\n")
	}

	if len(t.entries) == 0 {
		b.WriteString(EmptyStateView("⏳", "Waiting for log entries…", "New entries will appear as workflows execute."))
		return b.String()
	}

	b.WriteString(t.viewport.View())
	return b.String()
}

// renderEntries formats all log entries as a single string for the viewport.
func (t LogsTab) renderEntries() string { //nolint:gocritic // value receiver: read-only
	var b strings.Builder
	for i := range t.entries {
		b.WriteString(formatLogEntry(&t.entries[i]))
		b.WriteString("\n")
	}
	return b.String()
}

// formatLogEntry formats a single AWF audit entry.
func formatLogEntry(e *LogEntry) string {
	tsStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorText)

	ts := e.Timestamp
	if ts == "" {
		ts = "—"
	}

	event := styledEvent(e.Event)
	base := fmt.Sprintf("%s %s %s", tsStyle.Render("["+ts+"]"), event, nameStyle.Render(e.WorkflowName))

	if e.Status != "" {
		base += " " + StatusBadgeFromString(e.Status)
	}
	if e.DurationMs > 0 {
		base += " " + lipgloss.NewStyle().Foreground(ColorMuted).Render(fmt.Sprintf("(%dms)", int64(e.DurationMs)))
	}
	if e.Error != "" {
		base += " " + logsErrorStyle.Render(e.Error)
	}

	return base
}

// styledEvent returns the event name with color.
func styledEvent(event string) string {
	switch event {
	case "workflow.started":
		return logsEventStartedStyle.Render("▶ started")
	case "workflow.completed":
		return logsEventCompletedStyle.Render("■ completed")
	default:
		return event
	}
}

// resizeViewport adjusts the viewport dimensions for the current terminal size.
// It reserves lines for the tab bar, separator, and header area.
func (t *LogsTab) resizeViewport() {
	// Reserve 4 lines for the tab bar + separator + help bar from model.go,
	// and 2 more for the local header (path label + divider) when path is set.
	headerLines := 4
	if t.path != "" {
		headerLines += 2
	}
	if t.rotationNotice != "" {
		headerLines += 2
	}

	vpWidth := t.width
	vpHeight := t.height - headerLines
	if vpWidth < 1 {
		vpWidth = 1
	}
	if vpHeight < 1 {
		vpHeight = 1
	}
	t.viewport.SetWidth(vpWidth)
	t.viewport.SetHeight(vpHeight)
}
