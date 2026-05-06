package display

import "io"

// EventKind discriminates between event types: text output or tool use.
type EventKind string

const (
	EventText    EventKind = "text"
	EventToolUse EventKind = "tool_use"
)

// DisplayEvent represents a parsed event from a provider's streaming output.
// Kind discriminates the event type for rendering; Type holds the raw provider JSON event type.
// Text holds displayable output for EventText events; Name, Arg, ID are populated for EventToolUse events.
// Delta indicates the event is a streaming token fragment that should be written without a trailing newline.
type DisplayEvent struct {
	Type  string
	Kind  EventKind
	Text  string
	Name  string
	Arg   string
	ID    string
	Delta bool
}

// DisplayMode controls which event categories are rendered to the writer.
type DisplayMode int

const (
	DisplayModeDefault DisplayMode = iota // text events only
	DisplayModeVerbose                    // text events + tool-use markers
)

// DisplayEventParser parses a raw NDJSON line from a provider's streaming output
// into a slice of DisplayEvents. Implementations return nil to indicate that the
// line carries no displayable content.
type DisplayEventParser func(line []byte) []DisplayEvent

// RenderFunc is a callback that renders display events to a writer.
// Used by StreamFilterWriter to decouple parsing from rendering.
type RenderFunc func(w io.Writer, events []DisplayEvent, mode DisplayMode)
