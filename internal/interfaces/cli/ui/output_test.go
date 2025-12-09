package ui_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

func TestOutputWriter_WriteWorkflows_JSON(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true)

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
	w := ui.NewOutputWriter(buf, buf, ui.FormatTable, true)

	workflows := []ui.WorkflowInfo{
		{Name: "deploy", Source: "local", Version: "1.0.0", Description: "Deploy app"},
	}

	err := w.WriteWorkflows(workflows)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "deploy")
	assert.Contains(t, output, "local")
}

func TestOutputWriter_WriteWorkflows_Quiet(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatQuiet, true)

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
	w := ui.NewOutputWriter(buf, buf, ui.FormatText, true)

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
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true)

	exec := ui.ExecutionInfo{
		WorkflowID:   "abc123",
		WorkflowName: "test-wf",
		Status:       "completed",
		DurationMs:   1500,
		Steps: []ui.StepInfo{
			{Name: "step1", Status: "completed", ExitCode: 0},
		},
	}

	err := w.WriteExecution(exec)
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
	w := ui.NewOutputWriter(buf, buf, ui.FormatQuiet, true)

	exec := ui.ExecutionInfo{
		WorkflowID: "abc123",
		Status:     "running",
	}

	err := w.WriteExecution(exec)
	require.NoError(t, err)

	assert.Equal(t, "running\n", buf.String())
}

func TestOutputWriter_WriteRunResult_JSON(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true)

	result := ui.RunResult{
		WorkflowID: "abc123",
		Status:     "completed",
		DurationMs: 2000,
	}

	err := w.WriteRunResult(result)
	require.NoError(t, err)

	var got ui.RunResult
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)
	assert.Equal(t, "abc123", got.WorkflowID)
	assert.Equal(t, int64(2000), got.DurationMs)
}

func TestOutputWriter_WriteRunResult_Quiet(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatQuiet, true)

	result := ui.RunResult{
		WorkflowID: "abc123",
		Status:     "completed",
	}

	err := w.WriteRunResult(result)
	require.NoError(t, err)

	assert.Equal(t, "abc123\n", buf.String())
}

func TestOutputWriter_WriteValidation_JSON(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true)

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
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true)

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

func TestOutputWriter_WriteError_JSON(t *testing.T) {
	buf := new(bytes.Buffer)
	w := ui.NewOutputWriter(buf, buf, ui.FormatJSON, true)

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
	w := ui.NewOutputWriter(buf, errBuf, ui.FormatText, true)

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
		w := ui.NewOutputWriter(buf, buf, tt.format, true)
		assert.Equal(t, tt.want, w.IsJSONFormat())
	}
}
