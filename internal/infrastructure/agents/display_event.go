package agents

import "github.com/awf-project/cli/pkg/display"

// Type aliases re-export display types for internal use within the agents package.
// External consumers should import pkg/display directly.
type (
	EventKind          = display.EventKind
	DisplayEvent       = display.DisplayEvent
	DisplayMode        = display.DisplayMode
	DisplayEventParser = display.DisplayEventParser
)

const (
	EventText    = display.EventText
	EventToolUse = display.EventToolUse
)

const (
	DisplayModeDefault = display.DisplayModeDefault
	DisplayModeVerbose = display.DisplayModeVerbose
)
