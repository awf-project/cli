package ui_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputWriter_WriteWorkflows_JSON(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

	workflows := []ui.WorkflowInfo{
		{Name: "test-wf", Source: "local", Version: "1.0.0", Description: "Test workflow"},
	}

	err := w.WriteWorkflows(workflows)
	require.NoError(t, err)

	var got []ui.WorkflowInfo
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "test-wf", got[0].Name)
	assert.Equal(t, "local", got[0].Source)
	assert.Equal(t, "1.0.0", got[0].Version)
}

func TestOutputWriter_WriteWorkflows_Table(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatTable, true, false)

	workflows := []ui.WorkflowInfo{
		{Name: "deploy", Source: "local", Version: "1.0.0", Description: "Deploy app"},
	}

	err := w.WriteWorkflows(workflows)
	require.NoError(t, err)

	output := buf.String()
	// Should have ASCII borders
	assert.Contains(t, output, "+")
	assert.Contains(t, output, "|")
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "deploy")
	assert.Contains(t, output, "local")
}

func TestOutputWriter_WriteWorkflows_Quiet(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatQuiet, true, false)

	workflows := []ui.WorkflowInfo{
		{Name: "wf1"}, {Name: "wf2"}, {Name: "wf3"},
	}

	err := w.WriteWorkflows(workflows)
	require.NoError(t, err)

	output := buf.String()
	assert.Equal(t, "wf1\nwf2\nwf3\n", output)
}

func TestOutputWriter_WriteWorkflows_Text(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatText, true, false)

	workflows := []ui.WorkflowInfo{
		{Name: "deploy", Source: "local", Version: "1.0.0"},
	}

	err := w.WriteWorkflows(workflows)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "deploy")
}

func TestOutputWriter_WriteExecution_JSON(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

	exec := ui.ExecutionInfo{
		WorkflowID:   "abc123",
		WorkflowName: "test-wf",
		Status:       "completed",
		DurationMs:   1500,
		Steps: []ui.StepInfo{
			{Name: "step1", Status: "completed", ExitCode: 0},
		},
	}

	err := w.WriteExecution(&exec)
	require.NoError(t, err)

	var got ui.ExecutionInfo
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)
	assert.Equal(t, "abc123", got.WorkflowID)
	assert.Equal(t, "completed", got.Status)
	assert.Len(t, got.Steps, 1)
}

func TestOutputWriter_WriteExecution_Quiet(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatQuiet, true, false)

	exec := ui.ExecutionInfo{
		WorkflowID: "abc123",
		Status:     "running",
	}

	err := w.WriteExecution(&exec)
	require.NoError(t, err)

	assert.Equal(t, "running\n", buf.String())
}

func TestOutputWriter_WriteExecution_Table(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatTable, true, false)

	exec := ui.ExecutionInfo{
		WorkflowID:   "abc123",
		WorkflowName: "test-wf",
		Status:       "completed",
		DurationMs:   1500,
		Steps: []ui.StepInfo{
			{Name: "step1", Status: "completed"},
		},
	}

	err := w.WriteExecution(&exec)
	require.NoError(t, err)

	output := buf.String()
	// Should have ASCII borders
	assert.Contains(t, output, "+")
	assert.Contains(t, output, "|")
	assert.Contains(t, output, "Workflow:")
	assert.Contains(t, output, "STEP")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "step1")
}

func TestOutputWriter_WriteRunResult_JSON(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

	result := ui.RunResult{
		WorkflowID: "abc123",
		Status:     "completed",
		DurationMs: 2000,
	}

	err := w.WriteRunResult(&result)
	require.NoError(t, err)

	var got ui.RunResult
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)
	assert.Equal(t, "abc123", got.WorkflowID)
	assert.Equal(t, int64(2000), got.DurationMs)
}

func TestOutputWriter_WriteRunResult_Quiet(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatQuiet, true, false)

	result := ui.RunResult{
		WorkflowID: "abc123",
		Status:     "completed",
	}

	err := w.WriteRunResult(&result)
	require.NoError(t, err)

	assert.Equal(t, "abc123\n", buf.String())
}

func TestOutputWriter_WriteRunResult_Table(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatTable, true, false)

	result := ui.RunResult{
		WorkflowID: "abc123",
		Status:     "completed",
		DurationMs: 2000,
		Steps: []ui.StepInfo{
			{Name: "build", Status: "completed", Output: "Build success"},
			{Name: "test", Status: "completed", Output: "Tests passed"},
		},
	}

	err := w.WriteRunResult(&result)
	require.NoError(t, err)

	output := buf.String()
	// Should have ASCII borders
	assert.Contains(t, output, "+")
	assert.Contains(t, output, "|")
	assert.Contains(t, output, "Workflow ID:")
	assert.Contains(t, output, "STEP")
	assert.Contains(t, output, "OUTPUT")
	assert.Contains(t, output, "build")
	assert.Contains(t, output, "test")
	assert.Contains(t, output, "Total:")
}

func TestOutputWriter_WriteValidation_JSON(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

	result := ui.ValidationResult{
		Valid:    true,
		Workflow: "deploy",
	}

	err := w.WriteValidation(result)
	require.NoError(t, err)

	var got ui.ValidationResult
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)
	assert.True(t, got.Valid)
	assert.Equal(t, "deploy", got.Workflow)
}

func TestOutputWriter_WriteValidation_JSON_WithErrors(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

	result := ui.ValidationResult{
		Valid:    false,
		Workflow: "broken",
		Errors:   []string{"missing initial state", "undefined step"},
	}

	err := w.WriteValidation(result)
	require.NoError(t, err)

	var got ui.ValidationResult
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)
	assert.False(t, got.Valid)
	assert.Len(t, got.Errors, 2)
}

func TestOutputWriter_WriteValidation_Table(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatTable, true, false)

	result := ui.ValidationResult{
		Valid:    true,
		Workflow: "deploy",
	}

	err := w.WriteValidation(result)
	require.NoError(t, err)

	output := buf.String()
	// Should have ASCII borders
	assert.Contains(t, output, "+")
	assert.Contains(t, output, "|")
	assert.Contains(t, output, "Workflow:")
	assert.Contains(t, output, "deploy")
	assert.Contains(t, output, "valid")
}

func TestOutputWriter_WriteValidationTable(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatTable, true, false)

	result := ui.ValidationResultTable{
		Valid:    true,
		Workflow: "deploy",
		Inputs: []ui.InputInfo{
			{Name: "env", Type: "string", Required: true, Default: ""},
			{Name: "tag", Type: "string", Required: false, Default: "latest"},
		},
		Steps: []ui.StepSummary{
			{Name: "build", Type: "step", Next: "test"},
			{Name: "test", Type: "step", Next: "deploy"},
			{Name: "deploy", Type: "terminal", Next: "(terminal)"},
		},
	}

	err := w.WriteValidationTable(&result)
	require.NoError(t, err)

	output := buf.String()
	// Should have ASCII borders and content
	assert.Contains(t, output, "+")
	assert.Contains(t, output, "|")
	assert.Contains(t, output, "INPUT")
	assert.Contains(t, output, "STEP")
	assert.Contains(t, output, "env")
	assert.Contains(t, output, "build")
}

func TestOutputWriter_WriteError_JSON(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

	err := w.WriteError(errors.New("something failed"), 2)
	require.NoError(t, err)

	var got ui.ErrorResponse
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)
	assert.Equal(t, "something failed", got.Error)
	assert.Equal(t, 2, got.Code)
}

func TestOutputWriter_WriteError_Text(t *testing.T) {
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, errBuf, ui.FormatText, true, false)

	err := w.WriteError(errors.New("something failed"), 1)
	require.NoError(t, err)

	// Text errors go to errOut
	assert.Contains(t, errBuf.String(), "something failed")
}

func TestOutputWriter_IsJSONFormat(t *testing.T) {
	tests := []struct {
		format ui.OutputFormat
		want   bool
	}{
		{ui.FormatJSON, true},
		{ui.FormatText, false},
		{ui.FormatTable, false},
		{ui.FormatQuiet, false},
	}

	for _, tt := range tests {
		buf := new(bytes.Buffer)
		w := ui.NewOutputWriter(buf, buf, tt.format, true, false)
		assert.Equal(t, tt.want, w.IsJSONFormat())
	}
}

// RED Phase: Test stubs for text output mode functions
// These tests will compile and validate text format output

func TestOutputWriter_WriteExecution_Text(t *testing.T) {
	tests := []struct {
		name    string
		exec    ui.ExecutionInfo
		wantOut []string
	}{
		{
			name: "basic execution info",
			exec: ui.ExecutionInfo{
				WorkflowID:   "exec-123",
				WorkflowName: "my-workflow",
				Status:       "completed",
				DurationMs:   1500,
			},
			wantOut: []string{"Workflow:", "my-workflow", "ID:", "exec-123", "Status:", "completed"},
		},
		{
			name: "execution with current step",
			exec: ui.ExecutionInfo{
				WorkflowID:   "exec-456",
				WorkflowName: "running-wf",
				Status:       "running",
				CurrentStep:  "process",
				DurationMs:   500,
			},
			wantOut: []string{"running-wf", "running", "Current Step:", "process"},
		},
		{
			name: "execution with steps",
			exec: ui.ExecutionInfo{
				WorkflowID:   "exec-789",
				WorkflowName: "step-wf",
				Status:       "completed",
				DurationMs:   2000,
				Steps: []ui.StepInfo{
					{Name: "fetch", Status: "completed"},
					{Name: "process", Status: "completed"},
				},
			},
			wantOut: []string{"Steps:", "fetch", "process", "completed"},
		},
		{
			name: "execution with step error",
			exec: ui.ExecutionInfo{
				WorkflowID:   "exec-err",
				WorkflowName: "error-wf",
				Status:       "failed",
				DurationMs:   100,
				Steps: []ui.StepInfo{
					{Name: "failing", Status: "failed", Error: "command not found"},
				},
			},
			wantOut: []string{"failing", "failed", "Error:", "command not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			w := ui.NewOutputWriter(buf, buf, ui.FormatText, true, false)

			err := w.WriteExecution(&tt.exec)
			require.NoError(t, err)

			output := buf.String()
			for _, want := range tt.wantOut {
				assert.Contains(t, output, want)
			}
		})
	}
}

func TestOutputWriter_WriteRunResult_Text(t *testing.T) {
	tests := []struct {
		name    string
		result  ui.RunResult
		wantOut []string
	}{
		{
			name: "successful run",
			result: ui.RunResult{
				WorkflowID: "run-123",
				Status:     "completed",
				DurationMs: 2000,
			},
			wantOut: []string{"completed", "2000ms", "Workflow ID:", "run-123"},
		},
		{
			name: "failed run with error",
			result: ui.RunResult{
				WorkflowID: "run-456",
				Status:     "failed",
				DurationMs: 500,
				Error:      "step failed with exit code 1",
			},
			wantOut: []string{"failed", "Error:", "exit code 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			w := ui.NewOutputWriter(buf, buf, ui.FormatText, true, false)

			err := w.WriteRunResult(&tt.result)
			require.NoError(t, err)

			output := buf.String()
			for _, want := range tt.wantOut {
				assert.Contains(t, output, want)
			}
		})
	}
}

func TestOutputWriter_WriteValidation_Text(t *testing.T) {
	tests := []struct {
		name    string
		result  ui.ValidationResult
		wantOut []string
	}{
		{
			name: "valid workflow",
			result: ui.ValidationResult{
				Valid:    true,
				Workflow: "deploy",
			},
			wantOut: []string{"✓", "deploy", "valid"},
		},
		{
			name: "invalid workflow with errors",
			result: ui.ValidationResult{
				Valid:    false,
				Workflow: "broken",
				Errors:   []string{"missing initial state", "cycle detected"},
			},
			wantOut: []string{"✗", "broken", "invalid", "missing initial state", "cycle detected"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			w := ui.NewOutputWriter(buf, buf, ui.FormatText, true, false)

			err := w.WriteValidation(tt.result)
			require.NoError(t, err)

			output := buf.String()
			for _, want := range tt.wantOut {
				assert.Contains(t, output, want)
			}
		})
	}
}

func TestParseOutputFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ui.OutputFormat
		wantErr bool
	}{
		{
			name:  "text format",
			input: "text",
			want:  ui.FormatText,
		},
		{
			name:  "empty string defaults to text",
			input: "",
			want:  ui.FormatText,
		},
		{
			name:  "json format",
			input: "json",
			want:  ui.FormatJSON,
		},
		{
			name:  "table format",
			input: "table",
			want:  ui.FormatTable,
		},
		{
			name:  "quiet format",
			input: "quiet",
			want:  ui.FormatQuiet,
		},
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "unknown format",
			input:   "xml",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ui.ParseOutputFormat(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid output format")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestOutputFormat_String(t *testing.T) {
	tests := []struct {
		format ui.OutputFormat
		want   string
	}{
		{ui.FormatText, "text"},
		{ui.FormatJSON, "json"},
		{ui.FormatTable, "table"},
		{ui.FormatQuiet, "quiet"},
		{ui.OutputFormat(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.format.String())
		})
	}
}

func TestOutputWriter_WritePlugins_JSON(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

	plugins := []ui.PluginInfo{
		{
			Name:         "test-plugin",
			Type:         "builtin",
			Version:      "1.0.0",
			Description:  "Test plugin",
			Status:       "discovered",
			Enabled:      true,
			Capabilities: []string{"operations"},
		},
	}

	err := w.WritePlugins(plugins)
	require.NoError(t, err)

	var got []ui.PluginInfo
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "test-plugin", got[0].Name)
	assert.Equal(t, "builtin", got[0].Type)
	assert.Equal(t, "1.0.0", got[0].Version)
	assert.True(t, got[0].Enabled)
	assert.Contains(t, got[0].Capabilities, "operations")
}

func TestOutputWriter_WritePlugins_JSON_Multiple(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

	plugins := []ui.PluginInfo{
		{Name: "plugin-a", Version: "1.0.0", Status: "running", Enabled: true},
		{Name: "plugin-b", Version: "2.0.0", Status: "discovered", Enabled: false},
		{Name: "plugin-c", Version: "3.0.0", Status: "stopped", Enabled: true},
	}

	err := w.WritePlugins(plugins)
	require.NoError(t, err)

	var got []ui.PluginInfo
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)
	assert.Len(t, got, 3)
}

func TestOutputWriter_WritePlugins_JSON_Empty(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

	plugins := []ui.PluginInfo{}

	err := w.WritePlugins(plugins)
	require.NoError(t, err)

	var got []ui.PluginInfo
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestOutputWriter_WritePlugins_Table(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatTable, true, false)

	plugins := []ui.PluginInfo{
		{
			Name:         "my-plugin",
			Type:         "external",
			Version:      "1.0.0",
			Status:       "running",
			Enabled:      true,
			Capabilities: []string{"operations", "commands"},
		},
	}

	err := w.WritePlugins(plugins)
	require.NoError(t, err)

	output := buf.String()
	// Should have ASCII borders
	assert.Contains(t, output, "+")
	assert.Contains(t, output, "TYPE")
	assert.Contains(t, output, "|")
	// Should have headers
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "VERSION")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "ENABLED")
	assert.Contains(t, output, "CAPABILITIES")
	// Should have data
	assert.Contains(t, output, "my-plugin")
	assert.Contains(t, output, "1.0.0")
	assert.Contains(t, output, "running")
}

func TestOutputWriter_WritePlugins_Table_Empty(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatTable, true, false)

	plugins := []ui.PluginInfo{}

	err := w.WritePlugins(plugins)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No plugins found")
}

func TestOutputWriter_WritePlugins_Text(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatText, true, false)

	plugins := []ui.PluginInfo{
		{
			Name:         "text-plugin",
			Type:         "external",
			Version:      "2.0.0",
			Status:       "initialized",
			Enabled:      true,
			Capabilities: []string{"validators"},
		},
	}

	err := w.WritePlugins(plugins)
	require.NoError(t, err)

	output := buf.String()
	// Should have tabular output (tabwriter format)
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "TYPE")
	assert.Contains(t, output, "text-plugin")
	assert.Contains(t, output, "2.0.0")
}

func TestOutputWriter_WritePlugins_Text_Empty(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatText, true, false)

	plugins := []ui.PluginInfo{}

	err := w.WritePlugins(plugins)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No plugins found")
}

func TestOutputWriter_WritePlugins_Quiet(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatQuiet, true, false)

	plugins := []ui.PluginInfo{
		{Name: "plugin-1"},
		{Name: "plugin-2"},
		{Name: "plugin-3"},
	}

	err := w.WritePlugins(plugins)
	require.NoError(t, err)

	output := buf.String()
	assert.Equal(t, "plugin-1\nplugin-2\nplugin-3\n", output)
}

func TestOutputWriter_WritePlugins_Quiet_Empty(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatQuiet, true, false)

	plugins := []ui.PluginInfo{}

	err := w.WritePlugins(plugins)
	require.NoError(t, err)

	output := buf.String()
	assert.Empty(t, output)
}

func TestOutputWriter_WritePlugins_ShowsEnabledStatus(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		wantText string
	}{
		{
			name:     "enabled plugin",
			enabled:  true,
			wantText: "yes",
		},
		{
			name:     "disabled plugin",
			enabled:  false,
			wantText: "no",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			w := ui.NewOutputWriter(buf, buf, ui.FormatText, true, false)

			plugins := []ui.PluginInfo{
				{Name: "test-plugin", Version: "1.0.0", Status: "discovered", Enabled: tt.enabled},
			}

			err := w.WritePlugins(plugins)
			require.NoError(t, err)

			output := buf.String()
			assert.Contains(t, output, tt.wantText)
		})
	}
}

func TestOutputWriter_WritePlugins_ShowsCapabilities(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatText, true, false)

	plugins := []ui.PluginInfo{
		{
			Name:         "cap-plugin",
			Version:      "1.0.0",
			Status:       "running",
			Enabled:      true,
			Capabilities: []string{"operations", "commands", "validators"},
		},
	}

	err := w.WritePlugins(plugins)
	require.NoError(t, err)

	output := buf.String()
	// Capabilities should be displayed (comma-separated or similar)
	assert.Contains(t, output, "operations")
}

func TestOutputWriter_WritePlugins_EmptyCapabilities(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatText, true, false)

	plugins := []ui.PluginInfo{
		{
			Name:         "no-cap-plugin",
			Version:      "1.0.0",
			Status:       "discovered",
			Enabled:      true,
			Capabilities: []string{}, // No capabilities
		},
	}

	err := w.WritePlugins(plugins)
	require.NoError(t, err)

	output := buf.String()
	// Should show a placeholder for no capabilities
	assert.Contains(t, output, "-")
}

func TestOutputWriter_WritePlugins_AllFormats(t *testing.T) {
	formats := []struct {
		name   string
		format ui.OutputFormat
	}{
		{"text", ui.FormatText},
		{"json", ui.FormatJSON},
		{"table", ui.FormatTable},
		{"quiet", ui.FormatQuiet},
	}

	plugins := []ui.PluginInfo{
		{Name: "format-test-plugin", Version: "1.0.0", Status: "running", Enabled: true},
	}

	for _, tt := range formats {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			w := ui.NewOutputWriter(buf, buf, tt.format, true, false)

			err := w.WritePlugins(plugins)
			require.NoError(t, err)

			output := buf.String()
			assert.NotEmpty(t, output, "output should not be empty")
		})
	}
}

func TestPluginInfo_JSONSerialization(t *testing.T) {
	// Test that PluginInfo serializes correctly with all fields
	plugin := ui.PluginInfo{
		Name:         "full-plugin",
		Type:         "builtin",
		Version:      "1.2.3",
		Description:  "A full plugin for testing",
		Status:       "running",
		Enabled:      true,
		Capabilities: []string{"operations", "commands"},
		Operations:   []string{"run", "validate"},
	}

	data, err := json.Marshal(plugin)
	require.NoError(t, err)

	var got ui.PluginInfo
	err = json.Unmarshal(data, &got)
	require.NoError(t, err)

	assert.Equal(t, plugin.Name, got.Name)
	assert.Equal(t, plugin.Type, got.Type)
	assert.Equal(t, plugin.Version, got.Version)
	assert.Equal(t, plugin.Description, got.Description)
	assert.Equal(t, plugin.Status, got.Status)
	assert.Equal(t, plugin.Enabled, got.Enabled)
	assert.Equal(t, plugin.Capabilities, got.Capabilities)
	assert.Equal(t, plugin.Operations, got.Operations)
}

func TestPluginInfo_JSONOmitsEmptyFields(t *testing.T) {
	// Test that optional fields are omitted when empty
	plugin := ui.PluginInfo{
		Name:    "minimal-plugin",
		Status:  "discovered",
		Enabled: false,
		// Version, Description, Capabilities are empty
	}

	data, err := json.Marshal(plugin)
	require.NoError(t, err)

	output := string(data)
	// Should not contain empty optional fields (due to omitempty tags)
	assert.NotContains(t, output, `"version":""`)
	assert.NotContains(t, output, `"description":""`)
	assert.NotContains(t, output, `"type":""`)
	assert.NotContains(t, output, `"operations":null`)
}

func TestOutputWriter_WritePlugins_ShowsTypeValue(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatText, true, false)

	plugins := []ui.PluginInfo{
		{Name: "typed-plugin", Type: "builtin", Version: "1.0.0", Status: "running", Enabled: true},
	}

	err := w.WritePlugins(plugins)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "builtin")
}

func TestOutputWriter_WritePlugins_ShowsTypeValueInBorderedTable(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatTable, true, false)

	plugins := []ui.PluginInfo{
		{Name: "typed-plugin", Type: "external", Version: "1.0.0", Status: "running", Enabled: true},
	}

	err := w.WritePlugins(plugins)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "external")
}

func TestOutputWriter_WritePlugins_JSONIncludesOperations(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

	plugins := []ui.PluginInfo{
		{
			Name:       "ops-plugin",
			Type:       "builtin",
			Status:     "running",
			Enabled:    true,
			Operations: []string{"run", "validate", "list"},
		},
	}

	err := w.WritePlugins(plugins)
	require.NoError(t, err)

	var got []ui.PluginInfo
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, []string{"run", "validate", "list"}, got[0].Operations)
	assert.Equal(t, "builtin", got[0].Type)
}

func TestInputInfo_DescriptionField(t *testing.T) {
	tests := []struct {
		name        string
		input       ui.InputInfo
		wantDesc    string
		description string
	}{
		{
			name: "input with description",
			input: ui.InputInfo{
				Name:        "greeting",
				Type:        "string",
				Required:    false,
				Default:     "hello",
				Description: "Greeting message to display",
			},
			wantDesc:    "Greeting message to display",
			description: "should store and return description",
		},
		{
			name: "input without description (empty string)",
			input: ui.InputInfo{
				Name:        "count",
				Type:        "integer",
				Required:    true,
				Default:     "",
				Description: "",
			},
			wantDesc:    "",
			description: "should allow empty description for backward compatibility",
		},
		{
			name: "input with long description",
			input: ui.InputInfo{
				Name:        "verbose",
				Type:        "boolean",
				Required:    false,
				Default:     "false",
				Description: "Enable verbose output mode with detailed logging for debugging purposes",
			},
			wantDesc:    "Enable verbose output mode with detailed logging for debugging purposes",
			description: "should handle long descriptions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantDesc, tt.input.Description, tt.description)
		})
	}
}

func TestInputInfo_AllFieldsPresent(t *testing.T) {
	input := ui.InputInfo{
		Name:        "environment",
		Type:        "string",
		Required:    true,
		Default:     "production",
		Description: "Target deployment environment",
	}

	assert.Equal(t, "environment", input.Name)
	assert.Equal(t, "string", input.Type)
	assert.True(t, input.Required)
	assert.Equal(t, "production", input.Default)
	assert.Equal(t, "Target deployment environment", input.Description)
}

func TestInputInfo_BackwardCompatibility(t *testing.T) {
	// Test that InputInfo can be created without Description field
	// This ensures backward compatibility with existing code
	input := ui.InputInfo{
		Name:     "legacy_input",
		Type:     "string",
		Required: false,
		Default:  "default_value",
		// Description field omitted - should default to empty string
	}

	assert.Equal(t, "legacy_input", input.Name)
	assert.Equal(t, "", input.Description, "Description should default to empty string")
}

func TestInputInfo_UsedInValidationResultTable(t *testing.T) {
	// Test that InputInfo with Description works in ValidationResultTable
	result := ui.ValidationResultTable{
		Valid:    true,
		Workflow: "test-workflow",
		Inputs: []ui.InputInfo{
			{
				Name:        "branch",
				Type:        "string",
				Required:    true,
				Default:     "",
				Description: "Git branch to deploy",
			},
			{
				Name:        "dry_run",
				Type:        "boolean",
				Required:    false,
				Default:     "false",
				Description: "Simulate deployment without making changes",
			},
			{
				Name:        "timeout",
				Type:        "integer",
				Required:    false,
				Default:     "30",
				Description: "", // No description for this input
			},
		},
		Steps: []ui.StepSummary{
			{Name: "deploy", Type: "step", Next: "(terminal)"},
		},
	}

	assert.Len(t, result.Inputs, 3)
	assert.Equal(t, "Git branch to deploy", result.Inputs[0].Description)
	assert.Equal(t, "Simulate deployment without making changes", result.Inputs[1].Description)
	assert.Equal(t, "", result.Inputs[2].Description, "third input should have no description")
}

func TestInputInfo_DescriptionWithSpecialCharacters(t *testing.T) {
	tests := []struct {
		name        string
		description string
	}{
		{
			name:        "with quotes",
			description: `Value for "special" input`,
		},
		{
			name:        "with newline escape",
			description: "First line\\nSecond line",
		},
		{
			name:        "with unicode",
			description: "Deployment target \u2192 production",
		},
		{
			name:        "with angle brackets",
			description: "Format: <name>@<version>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := ui.InputInfo{
				Name:        "special",
				Type:        "string",
				Required:    false,
				Default:     "",
				Description: tt.description,
			}
			assert.Equal(t, tt.description, input.Description)
		})
	}
}

func TestOutputWriter_WriteOperations_JSON(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

	operations := []ui.OperationEntry{
		{Name: "process-data", Plugin: "dataproc"},
		{Name: "validate", Plugin: "dataproc"},
		{Name: "transform", Plugin: "transform-engine"},
	}

	err := w.WriteOperations(operations)
	require.NoError(t, err)

	var got []ui.OperationEntry
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)
	assert.Len(t, got, 3)
	assert.Equal(t, "process-data", got[0].Name)
	assert.Equal(t, "dataproc", got[0].Plugin)
	assert.Equal(t, "validate", got[1].Name)
	assert.Equal(t, "transform", got[2].Name)
	assert.Equal(t, "transform-engine", got[2].Plugin)
}

func TestOutputWriter_WriteOperations_JSON_Empty(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true, false)

	err := w.WriteOperations([]ui.OperationEntry{})
	require.NoError(t, err)

	var got []ui.OperationEntry
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)
	assert.Len(t, got, 0)
}

func TestOutputWriter_WriteOperations_Table(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatTable, true, false)

	operations := []ui.OperationEntry{
		{Name: "process", Plugin: "core"},
		{Name: "analyze", Plugin: "analytics"},
	}

	err := w.WriteOperations(operations)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "+")
	assert.Contains(t, output, "|")
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "PLUGIN")
	assert.Contains(t, output, "process")
	assert.Contains(t, output, "core")
	assert.Contains(t, output, "analyze")
	assert.Contains(t, output, "analytics")
}

func TestOutputWriter_WriteOperations_Table_Empty(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatTable, true, false)

	err := w.WriteOperations([]ui.OperationEntry{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No operations found")
}

func TestOutputWriter_WriteOperations_Text(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatText, true, false)

	operations := []ui.OperationEntry{
		{Name: "execute", Plugin: "executor"},
		{Name: "validate", Plugin: "validator"},
	}

	err := w.WriteOperations(operations)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "PLUGIN")
	assert.Contains(t, output, "execute")
	assert.Contains(t, output, "executor")
	assert.Contains(t, output, "validate")
	assert.Contains(t, output, "validator")
}

func TestOutputWriter_WriteOperations_Text_Empty(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatText, true, false)

	err := w.WriteOperations([]ui.OperationEntry{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No operations found")
}

func TestOutputWriter_WriteOperations_Quiet(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatQuiet, true, false)

	operations := []ui.OperationEntry{
		{Name: "op1", Plugin: "plugin-a"},
		{Name: "op2", Plugin: "plugin-b"},
		{Name: "op3", Plugin: "plugin-c"},
	}

	err := w.WriteOperations(operations)
	require.NoError(t, err)

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Len(t, lines, 3)
	assert.Equal(t, "op1", lines[0])
	assert.Equal(t, "op2", lines[1])
	assert.Equal(t, "op3", lines[2])
}

func TestOutputWriter_WriteOperations_Quiet_Empty(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatQuiet, true, false)

	err := w.WriteOperations([]ui.OperationEntry{})
	require.NoError(t, err)

	output := buf.String()
	assert.Equal(t, "", strings.TrimSpace(output))
}

func TestOperationEntry_JSONSerialization(t *testing.T) {
	op := ui.OperationEntry{
		Name:   "transform",
		Plugin: "data-transform",
	}

	data, err := json.Marshal(op)
	require.NoError(t, err)

	var decoded ui.OperationEntry
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "transform", decoded.Name)
	assert.Equal(t, "data-transform", decoded.Plugin)

	var asMap map[string]interface{}
	err = json.Unmarshal(data, &asMap)
	require.NoError(t, err)
	assert.Equal(t, "transform", asMap["name"])
	assert.Equal(t, "data-transform", asMap["plugin"])
}

func TestOperationEntry_Fields(t *testing.T) {
	op := ui.OperationEntry{
		Name:   "extract",
		Plugin: "extract-plugin",
	}

	assert.Equal(t, "extract", op.Name)
	assert.Equal(t, "extract-plugin", op.Plugin)
}

func TestOutputWriter_WriteOperations_AllFormats(t *testing.T) {
	tests := []struct {
		name     string
		format   ui.OutputFormat
		validate func(t *testing.T, output string)
	}{
		{
			name:   "JSON format",
			format: ui.FormatJSON,
			validate: func(t *testing.T, output string) {
				var ops []ui.OperationEntry
				err := json.Unmarshal([]byte(output), &ops)
				require.NoError(t, err)
				assert.Len(t, ops, 1)
				assert.Equal(t, "test-op", ops[0].Name)
			},
		},
		{
			name:   "Table format",
			format: ui.FormatTable,
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "test-op")
				assert.Contains(t, output, "test-plugin")
				assert.Contains(t, output, "+")
			},
		},
		{
			name:   "Text format",
			format: ui.FormatText,
			validate: func(t *testing.T, output string) {
				assert.Contains(t, output, "test-op")
				assert.Contains(t, output, "test-plugin")
				assert.Contains(t, output, "NAME")
			},
		},
		{
			name:   "Quiet format",
			format: ui.FormatQuiet,
			validate: func(t *testing.T, output string) {
				assert.Equal(t, "test-op", strings.TrimSpace(output))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			w := ui.NewOutputWriter(buf, buf, tt.format, true, false)

			operations := []ui.OperationEntry{
				{Name: "test-op", Plugin: "test-plugin"},
			}

			err := w.WriteOperations(operations)
			require.NoError(t, err)

			tt.validate(t, buf.String())
		})
	}
}
