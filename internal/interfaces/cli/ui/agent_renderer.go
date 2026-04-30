package ui

import (
	"fmt"
	"io"

	"github.com/awf-project/cli/pkg/display"
)

const toolArgMaxLen = 40

// RenderEvents writes the displayable content of events to w according to mode.
// Default mode emits text content only; verbose mode also emits tool-use markers
// in the form [tool: Name(Arg)].
func RenderEvents(w io.Writer, events []display.DisplayEvent, mode display.DisplayMode) {
	for _, e := range events {
		switch e.Kind {
		case display.EventText:
			fmt.Fprint(w, e.Text)
		case display.EventToolUse:
			if mode == display.DisplayModeVerbose {
				fmt.Fprint(w, formatToolMarker(e.Name, e.Arg))
			}
		}
	}
}

func formatToolMarker(name, arg string) string {
	if arg == "" {
		return "[tool: " + name + "]"
	}
	if len(arg) > toolArgMaxLen {
		// "…" is 3 UTF-8 bytes; truncate so the arg+ellipsis fits within toolArgMaxLen bytes
		arg = arg[:toolArgMaxLen-3] + "…"
	}
	return "[tool: " + name + "(" + arg + ")]"
}
