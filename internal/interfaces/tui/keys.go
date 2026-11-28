package tui

import "charm.land/bubbles/v2/key"

// Bindings used by the run screen. The tabbed screens and the Model match keys by name
// (see Model.updateKey and the screens' Update methods); their key bars are drawn from
// the keyHint lists next to each screen.
var (
	keyUp     = key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up"))
	keyDown   = key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down"))
	keySelect = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select"))
	keyFollow = key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "follow"))
	keyCancel = key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "cancel run"))
	keyRerun  = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "re-run"))
	keyCopy   = key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy output"))
)
