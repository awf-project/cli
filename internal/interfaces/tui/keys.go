package tui

import (
	"charm.land/bubbles/v2/key"
)

var (
	keyTab1        = key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "workflows"))
	keyTab2        = key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "monitoring"))
	keyTab3        = key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "history"))
	keyTab4        = key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "logs"))
	keyQuit        = key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit"))
	keyForceQuit   = key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "force quit"))
	keyHelp        = key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help"))
	keyUp          = key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up"))
	keyDown        = key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down"))
	keySelect      = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select"))
	keyBack        = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back"))
	keyFilter      = key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter"))
	keyFollow      = key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "follow"))
	keyValidate    = key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "validate"))
	keyCycleFilter = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "cycle status"))
)

var (
	tabKeys  = []key.Binding{keyTab1, keyTab2, keyTab3, keyTab4}
	quitKeys = []key.Binding{keyQuit, keyForceQuit, keyHelp}
)

// globalHelpKeys is the default help key map shown when no tab-specific map applies.
type globalHelpKeys struct{}

func (globalHelpKeys) ShortHelp() []key.Binding {
	return []key.Binding{keyQuit, keyHelp}
}

func (globalHelpKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		tabKeys,
		quitKeys,
	}
}

// workflowsHelpKeys is the help key map for the Workflows tab.
type workflowsHelpKeys struct{}

func (workflowsHelpKeys) ShortHelp() []key.Binding {
	return []key.Binding{keySelect, keyValidate, keyFilter, keyBack, keyQuit, keyHelp}
}

func (workflowsHelpKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{keySelect, keyValidate, keyFilter, keyBack},
		tabKeys,
		quitKeys,
	}
}

// monitoringHelpKeys is the help key map for the Monitoring tab.
type monitoringHelpKeys struct{}

func (monitoringHelpKeys) ShortHelp() []key.Binding {
	return []key.Binding{keyUp, keyDown, keyFollow, keyQuit, keyHelp}
}

func (monitoringHelpKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{keyUp, keyDown, keyFollow},
		tabKeys,
		quitKeys,
	}
}

// historyHelpKeys is the help key map for the History tab.
type historyHelpKeys struct{}

func (historyHelpKeys) ShortHelp() []key.Binding {
	return []key.Binding{keyUp, keyDown, keySelect, keyFilter, keyCycleFilter, keyBack, keyQuit, keyHelp}
}

func (historyHelpKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{keyUp, keyDown, keySelect, keyFilter, keyCycleFilter, keyBack},
		tabKeys,
		quitKeys,
	}
}

// logsHelpKeys is the help key map for the External Logs tab.
type logsHelpKeys struct{}

func (logsHelpKeys) ShortHelp() []key.Binding {
	return []key.Binding{keyFollow, keyQuit, keyHelp}
}

func (logsHelpKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{keyFollow},
		tabKeys,
		quitKeys,
	}
}
