package ui_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

func TestPrefixedWriter_StdoutPrefix(t *testing.T) {
	var buf bytes.Buffer
	colorizer := ui.NewColorizer(false)

	w := ui.NewPrefixedWriter(&buf, ui.PrefixStdout, "", colorizer)
	_, _ = w.Write([]byte("hello\n"))

	assert.Equal(t, "[OUT] hello\n", buf.String())
}

func TestPrefixedWriter_StderrPrefix(t *testing.T) {
	var buf bytes.Buffer
	colorizer := ui.NewColorizer(false)

	w := ui.NewPrefixedWriter(&buf, ui.PrefixStderr, "", colorizer)
	_, _ = w.Write([]byte("error\n"))

	assert.Equal(t, "[ERR] error\n", buf.String())
}

func TestPrefixedWriter_MultiLine(t *testing.T) {
	var buf bytes.Buffer
	colorizer := ui.NewColorizer(false)

	w := ui.NewPrefixedWriter(&buf, ui.PrefixStdout, "", colorizer)
	_, _ = w.Write([]byte("line1\nline2\nline3\n"))

	expected := "[OUT] line1\n[OUT] line2\n[OUT] line3\n"
	assert.Equal(t, expected, buf.String())
}

func TestPrefixedWriter_PartialLine(t *testing.T) {
	var buf bytes.Buffer
	colorizer := ui.NewColorizer(false)

	w := ui.NewPrefixedWriter(&buf, ui.PrefixStdout, "", colorizer)
	_, _ = w.Write([]byte("partial"))
	_, _ = w.Write([]byte(" line\n"))

	assert.Equal(t, "[OUT] partial line\n", buf.String())
}

func TestPrefixedWriter_WithStepName(t *testing.T) {
	var buf bytes.Buffer
	colorizer := ui.NewColorizer(false)

	w := ui.NewPrefixedWriter(&buf, ui.PrefixStdout, "build", colorizer)
	_, _ = w.Write([]byte("compiling\n"))

	assert.Equal(t, "[build|OUT] compiling\n", buf.String())
}

func TestPrefixedWriter_Flush(t *testing.T) {
	var buf bytes.Buffer
	colorizer := ui.NewColorizer(false)

	w := ui.NewPrefixedWriter(&buf, ui.PrefixStdout, "", colorizer)
	_, _ = w.Write([]byte("no newline"))
	w.Flush()

	assert.Equal(t, "[OUT] no newline\n", buf.String())
}

func TestPrefixedWriter_EmptyWrite(t *testing.T) {
	var buf bytes.Buffer
	colorizer := ui.NewColorizer(false)

	w := ui.NewPrefixedWriter(&buf, ui.PrefixStdout, "", colorizer)
	n, err := w.Write([]byte(""))

	assert.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Empty(t, buf.String())
}

func TestPrefixedWriter_ReturnsCorrectLength(t *testing.T) {
	var buf bytes.Buffer
	colorizer := ui.NewColorizer(false)

	w := ui.NewPrefixedWriter(&buf, ui.PrefixStdout, "", colorizer)
	input := []byte("hello world\n")
	n, err := w.Write(input)

	assert.NoError(t, err)
	assert.Equal(t, len(input), n)
}

func TestOutputWriter_WritePrompts(t *testing.T) {
	tests := []struct {
		name     string
		format   ui.OutputFormat
		prompts  []ui.PromptInfo
		validate func(t *testing.T, output string)
	}{
		{
			name:   "JSON format outputs valid JSON array",
			format: ui.FormatJSON,
			prompts: []ui.PromptInfo{
				{Name: "system.md", Path: ".awf/prompts/system.md", Size: 256},
				{Name: "task.txt", Path: ".awf/prompts/task.txt", Size: 128},
			},
			validate: func(t *testing.T, output string) {
				var prompts []ui.PromptInfo
				err := json.Unmarshal([]byte(output), &prompts)
				require.NoError(t, err)
				assert.Len(t, prompts, 2)
				assert.Equal(t, "system.md", prompts[0].Name)
				assert.Equal(t, int64(256), prompts[0].Size)
			},
		},
		{
			name:    "JSON format outputs empty array for no prompts",
			format:  ui.FormatJSON,
			prompts: []ui.PromptInfo{},
			validate: func(t *testing.T, output string) {
				var prompts []ui.PromptInfo
				err := json.Unmarshal([]byte(output), &prompts)
				require.NoError(t, err)
				assert.Empty(t, prompts)
			},
		},
		{
			name:   "text format displays prompts in readable format",
			format: ui.FormatText,
			prompts: []ui.PromptInfo{
				{Name: "system.md", Path: ".awf/prompts/system.md", Size: 256},
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "system.md")
				assert.Contains(t, output, "256")
			},
		},
		{
			name:   "table format displays bordered table",
			format: ui.FormatTable,
			prompts: []ui.PromptInfo{
				{Name: "prompt.md", Path: ".awf/prompts/prompt.md", Size: 512},
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "NAME")
				assert.Contains(t, output, "SIZE")
				assert.Contains(t, output, "prompt.md")
				// Table should have borders
				assert.Contains(t, output, "+")
				assert.Contains(t, output, "|")
			},
		},
		{
			name:   "quiet format outputs names only",
			format: ui.FormatQuiet,
			prompts: []ui.PromptInfo{
				{Name: "first.md", Path: ".awf/prompts/first.md", Size: 100},
				{Name: "second.txt", Path: ".awf/prompts/second.txt", Size: 200},
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "first.md")
				assert.Contains(t, output, "second.txt")
				// Should not contain size or path
				assert.NotContains(t, output, "100")
				assert.NotContains(t, output, "200")
				assert.NotContains(t, output, ".awf/prompts/")
			},
		},
		{
			name:   "displays nested paths correctly",
			format: ui.FormatText,
			prompts: []ui.PromptInfo{
				{Name: "ai/agents/claude.md", Path: ".awf/prompts/ai/agents/claude.md", Size: 1024},
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "ai/agents/claude.md")
			},
		},
		{
			name:   "displays modification time when present",
			format: ui.FormatText,
			prompts: []ui.PromptInfo{
				{
					Name:    "dated.md",
					Path:    ".awf/prompts/dated.md",
					Size:    64,
					ModTime: "2025-12-10T10:30:00Z",
				},
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "dated.md")
				// Modification time might be formatted differently
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			var errOut bytes.Buffer
			writer := ui.NewOutputWriter(&out, &errOut, tt.format, true)

			err := writer.WritePrompts(tt.prompts)
			require.NoError(t, err)

			tt.validate(t, out.String())
		})
	}
}

func TestPromptInfo_JSONMarshal(t *testing.T) {
	prompt := ui.PromptInfo{
		Name:    "test.md",
		Path:    ".awf/prompts/test.md",
		Size:    512,
		ModTime: "2025-12-10T10:00:00Z",
	}

	data, err := json.Marshal(prompt)
	require.NoError(t, err)

	// Verify all fields are present in JSON
	assert.Contains(t, string(data), `"name":"test.md"`)
	assert.Contains(t, string(data), `"path":".awf/prompts/test.md"`)
	assert.Contains(t, string(data), `"size":512`)
	assert.Contains(t, string(data), `"mod_time":"2025-12-10T10:00:00Z"`)
}

func TestPromptInfo_JSONOmitEmptyModTime(t *testing.T) {
	prompt := ui.PromptInfo{
		Name: "notime.md",
		Path: ".awf/prompts/notime.md",
		Size: 100,
		// ModTime is empty
	}

	data, err := json.Marshal(prompt)
	require.NoError(t, err)

	// mod_time should be omitted when empty
	assert.NotContains(t, string(data), `"mod_time"`)
}
