package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/awf-project/cli/pkg/display"
	"github.com/stretchr/testify/assert"
)

func TestRenderEvents_DefaultModeTextOnly(t *testing.T) {
	events := []display.DisplayEvent{
		{Kind: display.EventText, Text: "hello"},
		{Kind: display.EventText, Text: " world"},
	}

	var w bytes.Buffer
	RenderEvents(&w, events, display.DisplayModeDefault)

	assert.Equal(t, "hello world", w.String())
}

func TestRenderEvents_DefaultModeSkipsToolUse(t *testing.T) {
	events := []display.DisplayEvent{
		{Kind: display.EventText, Text: "starting"},
		{Kind: display.EventToolUse, Name: "Read", Arg: "file.txt"},
		{Kind: display.EventText, Text: "finished"},
	}

	var w bytes.Buffer
	RenderEvents(&w, events, display.DisplayModeDefault)

	result := w.String()
	assert.Equal(t, "startingfinished", result)
	assert.NotContains(t, result, "Read")
	assert.NotContains(t, result, "[tool:")
}

func TestRenderEvents_VerboseModeWithTextEvents(t *testing.T) {
	events := []display.DisplayEvent{
		{Kind: display.EventText, Text: "output"},
	}

	var w bytes.Buffer
	RenderEvents(&w, events, display.DisplayModeVerbose)

	assert.Equal(t, "output", w.String())
}

func TestRenderEvents_VerboseModeWithToolUseMarker(t *testing.T) {
	events := []display.DisplayEvent{
		{Kind: display.EventToolUse, Name: "Read", Arg: "file.txt"},
	}

	var w bytes.Buffer
	RenderEvents(&w, events, display.DisplayModeVerbose)

	assert.Equal(t, "[tool: Read(file.txt)]", w.String())
}

func TestRenderEvents_VerboseModeInterleavedTextAndTools(t *testing.T) {
	events := []display.DisplayEvent{
		{Kind: display.EventText, Text: "before"},
		{Kind: display.EventToolUse, Name: "Write", Arg: "output.txt"},
		{Kind: display.EventText, Text: "after"},
	}

	var w bytes.Buffer
	RenderEvents(&w, events, display.DisplayModeVerbose)

	assert.Equal(t, "before[tool: Write(output.txt)]after", w.String())
}

func TestRenderEvents_UnknownToolName(t *testing.T) {
	events := []display.DisplayEvent{
		{Kind: display.EventToolUse, Name: "UnknownTool", Arg: "arg"},
	}

	var w bytes.Buffer
	RenderEvents(&w, events, display.DisplayModeVerbose)

	result := w.String()
	assert.Contains(t, result, "UnknownTool")
	assert.NotPanics(t, func() {
		RenderEvents(&w, events, display.DisplayModeVerbose)
	})
}

func TestRenderEvents_EmptyArg(t *testing.T) {
	events := []display.DisplayEvent{
		{Kind: display.EventToolUse, Name: "Read", Arg: ""},
	}

	var w bytes.Buffer
	RenderEvents(&w, events, display.DisplayModeVerbose)

	result := w.String()
	assert.Equal(t, "[tool: Read]", result)
	assert.NotContains(t, result, "()")
}

func TestRenderEvents_ArgTruncationOver40Chars(t *testing.T) {
	longArg := strings.Repeat("a", 50)
	events := []display.DisplayEvent{
		{Kind: display.EventToolUse, Name: "Read", Arg: longArg},
	}

	var w bytes.Buffer
	RenderEvents(&w, events, display.DisplayModeVerbose)

	result := w.String()
	assert.Contains(t, result, "[tool: Read(")
	assert.True(t, len(result) <= len("[tool: Read()] ")+40,
		"marker plus arg should not exceed 40 character arg limit")

	assert.Contains(t, result, "…")
}

func TestRenderEvents_ArgExactly40Chars(t *testing.T) {
	arg40 := strings.Repeat("a", 40)
	events := []display.DisplayEvent{
		{Kind: display.EventToolUse, Name: "Read", Arg: arg40},
	}

	var w bytes.Buffer
	RenderEvents(&w, events, display.DisplayModeVerbose)

	result := w.String()
	assert.Contains(t, result, "[tool: Read(")
	assert.Contains(t, result, arg40)
}

func TestRenderEvents_ArgUnder40Chars(t *testing.T) {
	shortArg := "short"
	events := []display.DisplayEvent{
		{Kind: display.EventToolUse, Name: "Read", Arg: shortArg},
	}

	var w bytes.Buffer
	RenderEvents(&w, events, display.DisplayModeVerbose)

	result := w.String()
	assert.Equal(t, "[tool: Read(short)]", result)
	assert.NotContains(t, result, "…")
}

func TestRenderEvents_EmptyEventsSlice(t *testing.T) {
	var w bytes.Buffer
	RenderEvents(&w, []display.DisplayEvent{}, display.DisplayModeDefault)

	assert.Equal(t, "", w.String())
}

func TestRenderEvents_EmptyEventsSliceVerbose(t *testing.T) {
	var w bytes.Buffer
	RenderEvents(&w, []display.DisplayEvent{}, display.DisplayModeVerbose)

	assert.Equal(t, "", w.String())
}

func TestRenderEvents_MultipleEventsDefault(t *testing.T) {
	events := []display.DisplayEvent{
		{Kind: display.EventText, Text: "line1"},
		{Kind: display.EventText, Text: "line2"},
		{Kind: display.EventText, Text: "line3"},
	}

	var w bytes.Buffer
	RenderEvents(&w, events, display.DisplayModeDefault)

	assert.Equal(t, "line1line2line3", w.String())
}

func TestRenderEvents_MultipleEventsVerbose(t *testing.T) {
	events := []display.DisplayEvent{
		{Kind: display.EventText, Text: "start"},
		{Kind: display.EventToolUse, Name: "Bash", Arg: "ls"},
		{Kind: display.EventToolUse, Name: "Read", Arg: "file"},
		{Kind: display.EventText, Text: "end"},
	}

	var w bytes.Buffer
	RenderEvents(&w, events, display.DisplayModeVerbose)

	assert.Equal(t, "start[tool: Bash(ls)][tool: Read(file)]end", w.String())
}

func TestRenderEvents_NoANSIControlsFormatting(t *testing.T) {
	events := []display.DisplayEvent{
		{Kind: display.EventText, Text: "plain text"},
		{Kind: display.EventToolUse, Name: "Edit", Arg: "config"},
	}

	var w bytes.Buffer
	RenderEvents(&w, events, display.DisplayModeVerbose)

	result := w.String()
	assert.NotContains(t, result, "\x1b")
	assert.NotContains(t, result, "\033")
}

func TestRenderEvents_TextEventPreservesExactContent(t *testing.T) {
	testText := "special\nchars\t&$@#%"
	events := []display.DisplayEvent{
		{Kind: display.EventText, Text: testText},
	}

	var w bytes.Buffer
	RenderEvents(&w, events, display.DisplayModeDefault)

	assert.Equal(t, testText, w.String())
}

func TestRenderEvents_WriterImplementation(t *testing.T) {
	events := []display.DisplayEvent{
		{Kind: display.EventText, Text: "test"},
	}

	var w bytes.Buffer
	RenderEvents(&w, events, display.DisplayModeDefault)

	assert.Equal(t, "test", w.String())
}

func TestRenderEvents_LargeEventSlice(t *testing.T) {
	events := make([]display.DisplayEvent, 1000)
	for i := 0; i < 1000; i++ {
		events[i] = display.DisplayEvent{Kind: display.EventText, Text: "a"}
	}

	var w bytes.Buffer
	RenderEvents(&w, events, display.DisplayModeDefault)

	assert.Equal(t, strings.Repeat("a", 1000), w.String())
}
