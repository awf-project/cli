// Package agents — tests for T010: Update extractDisplayTextFromEvents to aggregate from DisplayEventParser.
//
// User Story:
//
//	US2: As a CLI developer, I want to extract and aggregate display text from parsed events
//	     so that the text content can be captured separately from the parsed event objects.
//
// Acceptance Criteria:
//
//	AC1: extractDisplayTextFromEvents aggregates text from parsed events with newlines between
//	AC2: Events with empty Kind are skipped silently
//	AC3: Returns empty string when parser returns only zero-value events
//	AC4: Handles empty raw string input gracefully
//	AC5: Works with single-line input
//	AC6: Works with multi-line input
//	AC7: Skips empty lines in raw input
//	AC8: Returns empty string when parser is nil
package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractDisplayTextFromEvents_AggregatesTextWithNewlines(t *testing.T) {
	parser := func(line []byte) []DisplayEvent {
		return []DisplayEvent{{
			Kind: EventText,
			Text: "parsed: " + string(line),
		}}
	}

	raw := "line1\nline2\nline3"
	result := extractDisplayTextFromEvents(raw, parser)

	assert.Equal(t, "parsed: line1\nparsed: line2\nparsed: line3", result)
}

func TestExtractDisplayTextFromEvents_SkipsEmptyKindEvents(t *testing.T) {
	parser := func(line []byte) []DisplayEvent {
		if string(line) == "skip_me" {
			return nil // nil means skip
		}
		return []DisplayEvent{{
			Kind: EventText,
			Text: string(line),
		}}
	}

	raw := "keep1\nskip_me\nkeep2"
	result := extractDisplayTextFromEvents(raw, parser)

	assert.Equal(t, "keep1\nkeep2", result)
}

func TestExtractDisplayTextFromEvents_ReturnsEmptyForNilEvents(t *testing.T) {
	parser := func(line []byte) []DisplayEvent {
		return nil // always return nil
	}

	raw := "line1\nline2"
	result := extractDisplayTextFromEvents(raw, parser)

	assert.Equal(t, "", result)
}

func TestExtractDisplayTextFromEvents_HandlesEmptyInput(t *testing.T) {
	parser := func(line []byte) []DisplayEvent {
		return []DisplayEvent{{Kind: EventText, Text: string(line)}}
	}

	result := extractDisplayTextFromEvents("", parser)

	assert.Equal(t, "", result)
}

func TestExtractDisplayTextFromEvents_SingleLine(t *testing.T) {
	parser := func(line []byte) []DisplayEvent {
		return []DisplayEvent{{Kind: EventText, Text: string(line)}}
	}

	result := extractDisplayTextFromEvents("single line", parser)

	assert.Equal(t, "single line", result)
}

func TestExtractDisplayTextFromEvents_MultipleLines(t *testing.T) {
	count := 0
	parser := func(line []byte) []DisplayEvent {
		count++
		return []DisplayEvent{{
			Kind: EventText,
			Text: string(line),
		}}
	}

	raw := "alpha\nbeta\ngamma\ndelta"
	result := extractDisplayTextFromEvents(raw, parser)

	assert.Equal(t, "alpha\nbeta\ngamma\ndelta", result)
	assert.Equal(t, 4, count, "parser should be called for each line")
}

func TestExtractDisplayTextFromEvents_SkipsEmptyLinesInRawInput(t *testing.T) {
	parser := func(line []byte) []DisplayEvent {
		if len(line) == 0 {
			return nil // empty line
		}
		return []DisplayEvent{{
			Kind: EventText,
			Text: string(line),
		}}
	}

	raw := "line1\n\nline2\n\nline3"
	result := extractDisplayTextFromEvents(raw, parser)

	assert.Equal(t, "line1\nline2\nline3", result)
}

func TestExtractDisplayTextFromEvents_NilParserReturnsEmpty(t *testing.T) {
	result := extractDisplayTextFromEvents("some text", nil)

	assert.Equal(t, "", result)
}

func TestExtractDisplayTextFromEvents_PreservesWhitespace(t *testing.T) {
	parser := func(line []byte) []DisplayEvent {
		return []DisplayEvent{{
			Kind: EventText,
			Text: string(line),
		}}
	}

	raw := "  leading\ntrailing  \n  both  "
	result := extractDisplayTextFromEvents(raw, parser)

	assert.Equal(t, "  leading\ntrailing  \n  both  ", result)
}

func TestExtractDisplayTextFromEvents_ToolUseEventsSkipped(t *testing.T) {
	parser := func(line []byte) []DisplayEvent {
		if string(line) == "tool_line" {
			return []DisplayEvent{{
				Kind: EventToolUse,
				Text: "tool_output",
			}}
		}
		return []DisplayEvent{{
			Kind: EventText,
			Text: string(line),
		}}
	}

	raw := "text1\ntool_line\ntext2"
	result := extractDisplayTextFromEvents(raw, parser)

	assert.Equal(t, "text1\ntext2", result)
}

func TestExtractDisplayTextFromEvents_EmptyTextInEvent(t *testing.T) {
	parser := func(line []byte) []DisplayEvent {
		if string(line) == "empty" {
			return []DisplayEvent{{
				Kind: EventText,
				Text: "", // empty text but non-empty Kind
			}}
		}
		return []DisplayEvent{{
			Kind: EventText,
			Text: string(line),
		}}
	}

	raw := "before\nempty\nafter"
	result := extractDisplayTextFromEvents(raw, parser)

	// Empty text is aggregated (different from skipped events)
	assert.Equal(t, "before\n\nafter", result)
}

// TestExtractDisplayTextFromEvents_MultipleEventsPerLine verifies that when a parser
// returns multiple events for a single line, all text events are aggregated.
func TestExtractDisplayTextFromEvents_MultipleEventsPerLine(t *testing.T) {
	parser := func(line []byte) []DisplayEvent {
		return []DisplayEvent{
			{Kind: EventText, Text: "first"},
			{Kind: EventToolUse},
			{Kind: EventText, Text: "second"},
		}
	}

	raw := "line1"
	result := extractDisplayTextFromEvents(raw, parser)

	assert.Equal(t, "first\nsecond", result)
}
