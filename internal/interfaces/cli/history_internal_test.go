package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/awf-project/cli/internal/testutil/facadetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteHistoryRunRecordsStats_CountsPersistedSuccess(t *testing.T) {
	var output bytes.Buffer
	writer := ui.NewOutputWriter(&output, &output, ui.FormatJSON, true, true)

	err := writeHistoryRunRecordsStats(writer, []ports.RunRecord{
		{Status: ports.RunState("success"), DurationMs: 1000},
		{Status: ports.RunStateFailed, DurationMs: 3000},
	})
	require.NoError(t, err)

	var stats HistoryStatsInfo
	require.NoError(t, json.Unmarshal(output.Bytes(), &stats))
	assert.Equal(t, 2, stats.TotalExecutions)
	assert.Equal(t, 1, stats.SuccessCount)
	assert.Equal(t, 1, stats.FailedCount)
	assert.Equal(t, int64(2000), stats.AvgDurationMs)
}

// failingHistoryFacade preserves the real facade contract while injecting the
// sole failure this command-level test needs to exercise.
type failingHistoryFacade struct {
	*facadetest.Fake
	err error
}

func (f *failingHistoryFacade) History(context.Context, ports.HistoryFilter) ([]ports.RunRecord, error) {
	return nil, f.err
}

func TestHistoryCommand_FacadeFailureRetainsSystemExit(t *testing.T) {
	const failureMessage = "history backend unavailable"
	failure := errors.New(failureMessage)

	cfg := DefaultConfig()
	cfg.Facade = &failingHistoryFacade{
		Fake: facadetest.New(),
		err:  failure,
	}
	cmd, cleanup := newRootCommandWithConfig(cfg, false)
	t.Cleanup(cleanup)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--storage", t.TempDir(), "--no-color", "history", "--workflow-id", "opaque-workflow-id"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.ErrorIs(t, err, failure)
	assert.ErrorContains(t, err, "list history: "+failureMessage)

	var exitErr *exitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, ExitSystem, exitErr.ExitCode())
	assert.Equal(t, 4, exitErr.ExitCode())
	assert.Contains(t, output.String(), "list history: "+failureMessage)
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		ms   int64
		want string
	}{
		{"zero", 0, "0ms"},
		{"milliseconds", 500, "500ms"},
		{"just under 1s", 999, "999ms"},
		{"exactly 1s", 1000, "1.0s"},
		{"1.5 seconds", 1500, "1.5s"},
		{"30 seconds", 30000, "30.0s"},
		{"just under 1m", 59999, "60.0s"},
		{"exactly 1m", 60000, "1.0m"},
		{"1.5 minutes", 90000, "1.5m"},
		{"5 minutes", 300000, "5.0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.ms)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWriteHistoryStats_Text(t *testing.T) {
	var out bytes.Buffer
	writer := ui.NewOutputWriter(&out, &out, ui.FormatText, true, false)

	stats := &workflow.HistoryStats{
		TotalExecutions: 100,
		SuccessCount:    80,
		FailedCount:     15,
		CancelledCount:  5,
		AvgDurationMs:   1500,
	}

	err := writeHistoryStats(writer, stats)
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Execution Statistics")
	assert.Contains(t, output, "Total Executions: 100")
	assert.Contains(t, output, "Success: 80")
	assert.Contains(t, output, "Failed: 15")
	assert.Contains(t, output, "Cancelled: 5")
	assert.Contains(t, output, "Average Duration: 1500ms")
}

func TestWriteHistoryStats_TextNoExecutions(t *testing.T) {
	var out bytes.Buffer
	writer := ui.NewOutputWriter(&out, &out, ui.FormatText, true, false)

	stats := &workflow.HistoryStats{
		TotalExecutions: 0,
		SuccessCount:    0,
		FailedCount:     0,
		CancelledCount:  0,
		AvgDurationMs:   0,
	}

	err := writeHistoryStats(writer, stats)
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Total Executions: 0")
	assert.NotContains(t, output, "Average Duration")
}

func TestWriteHistoryStats_JSON(t *testing.T) {
	var out bytes.Buffer
	writer := ui.NewOutputWriter(&out, &out, ui.FormatJSON, true, false)

	stats := &workflow.HistoryStats{
		TotalExecutions: 50,
		SuccessCount:    40,
		FailedCount:     8,
		CancelledCount:  2,
		AvgDurationMs:   2000,
	}

	err := writeHistoryStats(writer, stats)
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, `"total_executions": 50`)
	assert.Contains(t, output, `"success_count": 40`)
	assert.Contains(t, output, `"failed_count": 8`)
	assert.Contains(t, output, `"cancelled_count": 2`)
	assert.Contains(t, output, `"avg_duration_ms": 2000`)
}

func TestHistoryInfo_Struct(t *testing.T) {
	info := HistoryInfo{
		ID:           "test-id",
		WorkflowID:   "wf-id",
		WorkflowName: "test-workflow",
		Status:       "success",
		ExitCode:     0,
		DurationMs:   300000,
	}

	assert.Equal(t, "test-id", info.ID)
	assert.Equal(t, "wf-id", info.WorkflowID)
	assert.Equal(t, "test-workflow", info.WorkflowName)
	assert.Equal(t, "success", info.Status)
	assert.Equal(t, 0, info.ExitCode)
	assert.Equal(t, int64(300000), info.DurationMs)
}

func TestHistoryStatsInfo_Struct(t *testing.T) {
	info := HistoryStatsInfo{
		TotalExecutions: 100,
		SuccessCount:    80,
		FailedCount:     15,
		CancelledCount:  5,
		AvgDurationMs:   2500,
	}

	assert.Equal(t, 100, info.TotalExecutions)
	assert.Equal(t, 80, info.SuccessCount)
	assert.Equal(t, 15, info.FailedCount)
	assert.Equal(t, 5, info.CancelledCount)
	assert.Equal(t, int64(2500), info.AvgDurationMs)
}
