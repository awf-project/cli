package ui_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			writer := ui.NewOutputWriter(&out, &errOut, tt.format, true, false)

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

// Tests for PromptInfo.Source field (T003 - F044 XDG Prompt Discovery)

func TestPromptInfo_Source_JSONMarshal(t *testing.T) {
	tests := []struct {
		name       string
		prompt     ui.PromptInfo
		wantSource string
	}{
		{
			name: "local source is serialized",
			prompt: ui.PromptInfo{
				Name:   "system.md",
				Source: "local",
				Path:   ".awf/prompts/system.md",
				Size:   256,
			},
			wantSource: `"source":"local"`,
		},
		{
			name: "global source is serialized",
			prompt: ui.PromptInfo{
				Name:   "shared.md",
				Source: "global",
				Path:   "~/.config/awf/prompts/shared.md",
				Size:   128,
			},
			wantSource: `"source":"global"`,
		},
		{
			name: "empty source is serialized as empty string",
			prompt: ui.PromptInfo{
				Name: "legacy.md",
				Path: ".awf/prompts/legacy.md",
				Size: 64,
				// Source is empty (backward compatibility)
			},
			wantSource: `"source":""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.prompt)
			require.NoError(t, err)
			assert.Contains(t, string(data), tt.wantSource)
		})
	}
}

func TestPromptInfo_Source_JSONUnmarshal(t *testing.T) {
	tests := []struct {
		name       string
		jsonData   string
		wantSource string
	}{
		{
			name:       "unmarshal local source",
			jsonData:   `{"name":"test.md","source":"local","path":".awf/prompts/test.md","size":100}`,
			wantSource: "local",
		},
		{
			name:       "unmarshal global source",
			jsonData:   `{"name":"test.md","source":"global","path":"~/.config/awf/prompts/test.md","size":100}`,
			wantSource: "global",
		},
		{
			name:       "unmarshal empty source",
			jsonData:   `{"name":"test.md","source":"","path":".awf/prompts/test.md","size":100}`,
			wantSource: "",
		},
		{
			name:       "unmarshal missing source field",
			jsonData:   `{"name":"test.md","path":".awf/prompts/test.md","size":100}`,
			wantSource: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var prompt ui.PromptInfo
			err := json.Unmarshal([]byte(tt.jsonData), &prompt)
			require.NoError(t, err)
			assert.Equal(t, tt.wantSource, prompt.Source)
		})
	}
}

func TestOutputWriter_WritePrompts_SourceColumn(t *testing.T) {
	tests := []struct {
		name     string
		format   ui.OutputFormat
		prompts  []ui.PromptInfo
		validate func(t *testing.T, output string)
	}{
		{
			name:   "JSON format includes source for each prompt",
			format: ui.FormatJSON,
			prompts: []ui.PromptInfo{
				{Name: "local.md", Source: "local", Path: ".awf/prompts/local.md", Size: 100},
				{Name: "global.md", Source: "global", Path: "~/.config/awf/prompts/global.md", Size: 200},
			},
			validate: func(t *testing.T, output string) {
				var prompts []ui.PromptInfo
				err := json.Unmarshal([]byte(output), &prompts)
				require.NoError(t, err)
				require.Len(t, prompts, 2)
				assert.Equal(t, "local", prompts[0].Source)
				assert.Equal(t, "global", prompts[1].Source)
			},
		},
		{
			name:   "text format displays SOURCE column header",
			format: ui.FormatText,
			prompts: []ui.PromptInfo{
				{Name: "test.md", Source: "local", Path: ".awf/prompts/test.md", Size: 100},
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "SOURCE")
			},
		},
		{
			name:   "text format displays local source",
			format: ui.FormatText,
			prompts: []ui.PromptInfo{
				{Name: "local.md", Source: "local", Path: ".awf/prompts/local.md", Size: 100},
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "local.md")
				assert.Contains(t, output, "local")
			},
		},
		{
			name:   "text format displays global source",
			format: ui.FormatText,
			prompts: []ui.PromptInfo{
				{Name: "global.md", Source: "global", Path: "~/.config/awf/prompts/global.md", Size: 200},
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "global.md")
				assert.Contains(t, output, "global")
			},
		},
		{
			name:   "text format displays mixed sources correctly",
			format: ui.FormatText,
			prompts: []ui.PromptInfo{
				{Name: "system.md", Source: "local", Path: ".awf/prompts/system.md", Size: 100},
				{Name: "shared.md", Source: "global", Path: "~/.config/awf/prompts/shared.md", Size: 150},
				{Name: "task.md", Source: "local", Path: ".awf/prompts/task.md", Size: 200},
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "system.md")
				assert.Contains(t, output, "shared.md")
				assert.Contains(t, output, "task.md")
				// Source column should appear in output
				assert.Contains(t, output, "local")
				assert.Contains(t, output, "global")
			},
		},
		{
			name:   "table format displays SOURCE column header",
			format: ui.FormatTable,
			prompts: []ui.PromptInfo{
				{Name: "test.md", Source: "local", Path: ".awf/prompts/test.md", Size: 100},
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "SOURCE")
				assert.Contains(t, output, "+")
				assert.Contains(t, output, "|")
			},
		},
		{
			name:   "table format displays source values",
			format: ui.FormatTable,
			prompts: []ui.PromptInfo{
				{Name: "local.md", Source: "local", Path: ".awf/prompts/local.md", Size: 100},
				{Name: "global.md", Source: "global", Path: "~/.config/awf/prompts/global.md", Size: 200},
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "local")
				assert.Contains(t, output, "global")
			},
		},
		{
			name:   "quiet format does not include source",
			format: ui.FormatQuiet,
			prompts: []ui.PromptInfo{
				{Name: "test.md", Source: "local", Path: ".awf/prompts/test.md", Size: 100},
				{Name: "other.md", Source: "global", Path: "~/.config/awf/prompts/other.md", Size: 200},
			},
			validate: func(t *testing.T, output string) {
				// Quiet mode should only show names
				assert.Equal(t, "test.md\nother.md\n", output)
			},
		},
		{
			name:   "empty source displays as empty string",
			format: ui.FormatText,
			prompts: []ui.PromptInfo{
				{Name: "legacy.md", Source: "", Path: ".awf/prompts/legacy.md", Size: 100},
			},
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "legacy.md")
				// Should still have the column structure
				assert.Contains(t, output, "SOURCE")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			var errOut bytes.Buffer
			writer := ui.NewOutputWriter(&out, &errOut, tt.format, true, false)

			err := writer.WritePrompts(tt.prompts)
			require.NoError(t, err)

			tt.validate(t, out.String())
		})
	}
}

func TestPromptInfo_Source_DistinguishesLocalFromGlobal(t *testing.T) {
	// Simulates FR-004: awf list prompts displays source column (local/global)
	prompts := []ui.PromptInfo{
		{Name: "override.md", Source: "local", Path: ".awf/prompts/override.md", Size: 100},
		{Name: "shared.md", Source: "global", Path: "~/.config/awf/prompts/shared.md", Size: 200},
		{Name: "project.md", Source: "local", Path: ".awf/prompts/project.md", Size: 150},
	}

	// Test JSON output
	var jsonOut bytes.Buffer
	jsonWriter := ui.NewOutputWriter(&jsonOut, &bytes.Buffer{}, ui.FormatJSON, true, false)
	err := jsonWriter.WritePrompts(prompts)
	require.NoError(t, err)

	var parsed []ui.PromptInfo
	err = json.Unmarshal(jsonOut.Bytes(), &parsed)
	require.NoError(t, err)

	// Count sources
	localCount := 0
	globalCount := 0
	for _, p := range parsed {
		switch p.Source {
		case "local":
			localCount++
		case "global":
			globalCount++
		}
	}
	assert.Equal(t, 2, localCount, "should have 2 local prompts")
	assert.Equal(t, 1, globalCount, "should have 1 global prompt")
}

func TestPromptInfo_Source_WithNestedPaths(t *testing.T) {
	// Tests that nested prompt paths correctly show their source
	prompts := []ui.PromptInfo{
		{Name: "ai/agents/claude.md", Source: "local", Path: ".awf/prompts/ai/agents/claude.md", Size: 512},
		{Name: "templates/default.md", Source: "global", Path: "~/.config/awf/prompts/templates/default.md", Size: 256},
	}

	var out bytes.Buffer
	writer := ui.NewOutputWriter(&out, &bytes.Buffer{}, ui.FormatText, true, false)

	err := writer.WritePrompts(prompts)
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "ai/agents/claude.md")
	assert.Contains(t, output, "templates/default.md")
	assert.Contains(t, output, "local")
	assert.Contains(t, output, "global")
}

func TestPromptInfo_Source_EmptyList(t *testing.T) {
	// Ensures empty prompt list doesn't cause issues with source column
	var out bytes.Buffer
	writer := ui.NewOutputWriter(&out, &bytes.Buffer{}, ui.FormatText, true, false)

	err := writer.WritePrompts([]ui.PromptInfo{})
	require.NoError(t, err)

	output := out.String()
	// Should still show headers
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "SOURCE")
}

func TestPromptInfo_Source_OrderPreservation(t *testing.T) {
	// Verifies that prompt order is preserved (important for override display)
	prompts := []ui.PromptInfo{
		{Name: "first.md", Source: "local", Path: ".awf/prompts/first.md", Size: 100},
		{Name: "second.md", Source: "global", Path: "~/.config/awf/prompts/second.md", Size: 200},
		{Name: "third.md", Source: "local", Path: ".awf/prompts/third.md", Size: 300},
	}

	var out bytes.Buffer
	writer := ui.NewOutputWriter(&out, &bytes.Buffer{}, ui.FormatJSON, true, false)

	err := writer.WritePrompts(prompts)
	require.NoError(t, err)

	var parsed []ui.PromptInfo
	err = json.Unmarshal(out.Bytes(), &parsed)
	require.NoError(t, err)

	require.Len(t, parsed, 3)
	assert.Equal(t, "first.md", parsed[0].Name)
	assert.Equal(t, "local", parsed[0].Source)
	assert.Equal(t, "second.md", parsed[1].Name)
	assert.Equal(t, "global", parsed[1].Source)
	assert.Equal(t, "third.md", parsed[2].Name)
	assert.Equal(t, "local", parsed[2].Source)
}
