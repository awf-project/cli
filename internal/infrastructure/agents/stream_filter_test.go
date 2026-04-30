// Package agents — tests for T009: Update StreamFilterWriter to use DisplayEventParser
// and forward events to renderer.
//
// User Stories:
//
//	US1: As a CLI user, I want streaming output parsed into typed display events so that
//	     text and tool-use events can be rendered distinctly in the terminal.
//	US2: As a CLI developer, I want a standard DisplayEventParser contract so that each
//	     provider implements its own parsing without coupling to the writer's buffering logic.
//
// Acceptance Criteria:
//
//	AC1 (US1): NewStreamFilterWriterWithParser initializes with parser and renderer — TestNewStreamFilterWriterWithParser_InitializesWithParserAndRenderer
//	AC2 (US2): nil parser falls back to a pass-through writer — TestNewStreamFilterWriterWithParser_NilParserReturnsEmptyWriter
//	AC3 (US1): Write() forwards each parsed EventText line to the renderer — TestStreamFilterWriter_WriteWithParserForwardsEventsToRenderer
//	AC4 (US1): Events with empty Kind are skipped silently — TestStreamFilterWriter_WriteWithParserSkipsEmptyKindEvents
//	AC5 (US2): nil renderer does not panic — TestStreamFilterWriter_WriteWithNilRenderer
//	AC6 (US1): Multiple lines in one Write produce one event each — TestStreamFilterWriter_WriteWithParserMultipleLines
//	AC7 (US1): Partial lines are buffered until newline arrives — TestStreamFilterWriter_WriteWithParserPartialLines
//	AC8 (US1): Flush() emits buffered partial line through parser and renderer — TestStreamFilterWriter_FlushWithParserEmitsBufiidContent
//	AC9 (US1): EventToolUse events are forwarded to renderer — TestStreamFilterWriter_WriteWithParserToolUseEvent
//	AC10 (US2): Write errors from inner writer are wrapped and returned to caller — TestStreamFilterWriter_WriteWithParserWriteError
//	AC11 (US1): Oversized lines are dumped raw without parsing — TestStreamFilterWriter_WriteWithParserOversizedLine
//	AC12 (US2): extract and parser can coexist on the same writer — TestStreamFilterWriter_WithExtractorAndParser
package agents

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStreamFilterWriterWithParser_InitializesWithParserAndRenderer(t *testing.T) {
	inner := &bytes.Buffer{}
	parser := func(line []byte) []DisplayEvent {
		return []DisplayEvent{{Kind: EventText, Text: string(line)}}
	}
	renderer := func(events []DisplayEvent) {}

	w := NewStreamFilterWriterWithParser(inner, parser, renderer)

	assert.NotNil(t, w)
	assert.Equal(t, inner, w.inner)
	assert.NotNil(t, w.parser)
	assert.NotNil(t, w.renderer)
	assert.NotNil(t, w.logger)
}

func TestNewStreamFilterWriterWithParser_NilParserReturnsEmptyWriter(t *testing.T) {
	inner := &bytes.Buffer{}

	w := NewStreamFilterWriterWithParser(inner, nil, nil)

	assert.NotNil(t, w)
	assert.Equal(t, inner, w.inner)
}

func TestStreamFilterWriter_WriteWithParserForwardsEventsToRenderer(t *testing.T) {
	inner := &bytes.Buffer{}
	var renderedEvents []DisplayEvent

	parser := func(line []byte) []DisplayEvent {
		return []DisplayEvent{{
			Type: "text_delta",
			Kind: EventText,
			Text: string(line),
		}}
	}

	renderer := func(events []DisplayEvent) {
		renderedEvents = append(renderedEvents, events...)
	}

	w := NewStreamFilterWriterWithParser(inner, parser, renderer)
	require.NotNil(t, w)

	data := []byte("hello world\n")
	n, err := w.Write(data)

	assert.NoError(t, err)
	assert.Equal(t, len(data), n)
	assert.Len(t, renderedEvents, 1)
	assert.Equal(t, "hello world", renderedEvents[0].Text)
	assert.Equal(t, EventText, renderedEvents[0].Kind)
}

func TestStreamFilterWriter_WriteWithParserSkipsEmptyKindEvents(t *testing.T) {
	inner := &bytes.Buffer{}
	var renderedEvents []DisplayEvent

	parser := func(line []byte) []DisplayEvent {
		return nil // nil means skip
	}

	renderer := func(events []DisplayEvent) {
		renderedEvents = append(renderedEvents, events...)
	}

	w := NewStreamFilterWriterWithParser(inner, parser, renderer)
	require.NotNil(t, w)

	data := []byte("ignored line\n")
	_, err := w.Write(data)

	assert.NoError(t, err)
	assert.Empty(t, renderedEvents)
}

func TestStreamFilterWriter_WriteWithNilRenderer(t *testing.T) {
	inner := &bytes.Buffer{}

	parser := func(line []byte) []DisplayEvent {
		return []DisplayEvent{{Kind: EventText, Text: string(line)}}
	}

	w := NewStreamFilterWriterWithParser(inner, parser, nil)
	require.NotNil(t, w)

	data := []byte("event line\n")
	n, err := w.Write(data)

	assert.NoError(t, err)
	assert.Equal(t, len(data), n)
}

func TestStreamFilterWriter_WriteWithParserMultipleLines(t *testing.T) {
	inner := &bytes.Buffer{}
	var renderedEvents []DisplayEvent

	parser := func(line []byte) []DisplayEvent {
		return []DisplayEvent{{
			Kind: EventText,
			Text: string(line),
		}}
	}

	renderer := func(events []DisplayEvent) {
		renderedEvents = append(renderedEvents, events...)
	}

	w := NewStreamFilterWriterWithParser(inner, parser, renderer)
	require.NotNil(t, w)

	data := []byte("line1\nline2\nline3\n")
	n, err := w.Write(data)

	assert.NoError(t, err)
	assert.Equal(t, len(data), n)
	assert.Len(t, renderedEvents, 3)
	assert.Equal(t, "line1", renderedEvents[0].Text)
	assert.Equal(t, "line2", renderedEvents[1].Text)
	assert.Equal(t, "line3", renderedEvents[2].Text)
}

func TestStreamFilterWriter_WriteWithParserPartialLines(t *testing.T) {
	inner := &bytes.Buffer{}
	var renderedEvents []DisplayEvent

	parser := func(line []byte) []DisplayEvent {
		return []DisplayEvent{{Kind: EventText, Text: string(line)}}
	}

	renderer := func(events []DisplayEvent) {
		renderedEvents = append(renderedEvents, events...)
	}

	w := NewStreamFilterWriterWithParser(inner, parser, renderer)
	require.NotNil(t, w)

	w.Write([]byte("partial"))
	w.Write([]byte(" line"))
	assert.Empty(t, renderedEvents)

	w.Write([]byte("\n"))
	assert.Len(t, renderedEvents, 1)
	assert.Equal(t, "partial line", renderedEvents[0].Text)
}

func TestStreamFilterWriter_FlushWithParserEmitsBufiidContent(t *testing.T) {
	inner := &bytes.Buffer{}
	var renderedEvents []DisplayEvent

	parser := func(line []byte) []DisplayEvent {
		return []DisplayEvent{{Kind: EventText, Text: string(line)}}
	}

	renderer := func(events []DisplayEvent) {
		renderedEvents = append(renderedEvents, events...)
	}

	w := NewStreamFilterWriterWithParser(inner, parser, renderer)
	require.NotNil(t, w)

	w.Write([]byte("buffered content without newline"))
	assert.Empty(t, renderedEvents)

	err := w.Flush()
	assert.NoError(t, err)
	assert.Len(t, renderedEvents, 1)
	assert.Equal(t, "buffered content without newline", renderedEvents[0].Text)
}

func TestStreamFilterWriter_WriteWithParserToolUseEvent(t *testing.T) {
	inner := &bytes.Buffer{}
	var renderedEvents []DisplayEvent

	parser := func(line []byte) []DisplayEvent {
		return []DisplayEvent{{
			Type: "tool_use",
			Kind: EventToolUse,
			Name: "search",
			ID:   "tool-123",
		}}
	}

	renderer := func(events []DisplayEvent) {
		renderedEvents = append(renderedEvents, events...)
	}

	w := NewStreamFilterWriterWithParser(inner, parser, renderer)
	require.NotNil(t, w)

	data := []byte("{\"type\":\"tool_use\",\"name\":\"search\"}\n")
	_, err := w.Write(data)

	assert.NoError(t, err)
	assert.Len(t, renderedEvents, 1)
	assert.Equal(t, EventToolUse, renderedEvents[0].Kind)
	assert.Equal(t, "search", renderedEvents[0].Name)
}

func TestStreamFilterWriter_WriteWithParserWriteError(t *testing.T) {
	failWriter := &failingWriter{failAfter: 0}
	parser := func(line []byte) []DisplayEvent {
		return []DisplayEvent{{Kind: EventText, Text: string(line)}}
	}
	renderer := func(events []DisplayEvent) {}

	w := NewStreamFilterWriterWithParser(failWriter, parser, renderer)
	require.NotNil(t, w)

	data := []byte("test\n")
	_, err := w.Write(data)

	assert.Error(t, err)
	assert.ErrorContains(t, err, "write failed")
}

func TestStreamFilterWriter_WriteWithParserOversizedLine(t *testing.T) {
	inner := &bytes.Buffer{}
	var renderedEvents []DisplayEvent

	parser := func(line []byte) []DisplayEvent {
		return []DisplayEvent{{Kind: EventText, Text: string(line)}}
	}

	renderer := func(events []DisplayEvent) {
		renderedEvents = append(renderedEvents, events...)
	}

	w := NewStreamFilterWriterWithParser(inner, parser, renderer)
	require.NotNil(t, w)

	largeData := make([]byte, maxLineSize+1)
	for i := range largeData {
		largeData[i] = 'x'
	}
	largeData[len(largeData)-1] = '\n'

	n, err := w.Write(largeData)

	assert.NoError(t, err)
	assert.Equal(t, len(largeData), n)
	assert.Empty(t, renderedEvents, "oversized line should be dumped raw, not parsed")
}

func TestStreamFilterWriter_WithExtractorAndParser(t *testing.T) {
	inner := &bytes.Buffer{}
	var renderedEvents []DisplayEvent

	extract := func(line []byte) string {
		return "extracted:" + string(line)
	}

	parser := func(line []byte) []DisplayEvent {
		return []DisplayEvent{{Kind: EventText, Text: string(line)}}
	}

	renderer := func(events []DisplayEvent) {
		renderedEvents = append(renderedEvents, events...)
	}

	w := &StreamFilterWriter{
		inner:    inner,
		extract:  extract,
		parser:   parser,
		renderer: renderer,
	}

	data := []byte("test\n")
	_, err := w.Write(data)

	assert.NoError(t, err)
	assert.Len(t, renderedEvents, 1)
}
