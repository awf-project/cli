package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteHistoryRecords_DisplaysFullValues(t *testing.T) {
	var out bytes.Buffer
	writer := ui.NewOutputWriter(&out, &out, ui.FormatText, true, false)

	now := time.Now()
	records := []*workflow.ExecutionRecord{
		{
			ID:           "550e8400-e29b-41d4-a716-446655440000",
			WorkflowID:   "wf-001",
			WorkflowName: "deploy-staging-eu-west-1",
			Status:       "success",
			ExitCode:     0,
			StartedAt:    now.Add(-5 * time.Minute),
			CompletedAt:  now,
			DurationMs:   300000,
		},
	}

	err := writeHistoryRecords(writer, records)
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "550e8400-e29b-41d4-a716-446655440000")
	assert.Contains(t, output, "deploy-staging-eu-west-1")
	assert.NotContains(t, output, "...")
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

func TestWriteHistoryRecords_Empty(t *testing.T) {
	var out bytes.Buffer
	writer := ui.NewOutputWriter(&out, &out, ui.FormatText, true, false)

	err := writeHistoryRecords(writer, []*workflow.ExecutionRecord{})
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "No execution history found")
}

func TestWriteHistoryRecords_Text(t *testing.T) {
	var out bytes.Buffer
	writer := ui.NewOutputWriter(&out, &out, ui.FormatText, true, false)

	now := time.Now()
	records := []*workflow.ExecutionRecord{
		{
			ID:           "exec-12345",
			WorkflowID:   "wf-001",
			WorkflowName: "deploy",
			Status:       "success",
			ExitCode:     0,
			StartedAt:    now.Add(-5 * time.Minute),
			CompletedAt:  now,
			DurationMs:   300000,
		},
		{
			ID:           "exec-67890",
			WorkflowID:   "wf-002",
			WorkflowName: "test",
			Status:       "failed",
			ExitCode:     1,
			StartedAt:    now.Add(-10 * time.Minute),
			CompletedAt:  now.Add(-5 * time.Minute),
			DurationMs:   300000,
			ErrorMessage: "test failed",
		},
	}

	err := writeHistoryRecords(writer, records)
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "ID")
	assert.Contains(t, output, "WORKFLOW")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "DURATION")
	assert.Contains(t, output, "deploy")
	assert.Contains(t, output, "test")
	assert.Contains(t, output, "success")
	assert.Contains(t, output, "failed")
}

func TestWriteHistoryRecords_JSON(t *testing.T) {
	var out bytes.Buffer
	writer := ui.NewOutputWriter(&out, &out, ui.FormatJSON, true, false)

	now := time.Now()
	records := []*workflow.ExecutionRecord{
		{
			ID:           "exec-12345",
			WorkflowID:   "wf-001",
			WorkflowName: "deploy",
			Status:       "success",
			ExitCode:     0,
			StartedAt:    now.Add(-5 * time.Minute),
			CompletedAt:  now,
			DurationMs:   300000,
		},
	}

	err := writeHistoryRecords(writer, records)
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, `"id": "exec-12345"`)
	assert.Contains(t, output, `"workflow_name": "deploy"`)
	assert.Contains(t, output, `"status": "success"`)
	assert.Contains(t, output, `"duration_ms": 300000`)
}

func TestWriteHistoryRecords_DisplaysFullID(t *testing.T) {
	var out bytes.Buffer
	writer := ui.NewOutputWriter(&out, &out, ui.FormatText, true, false)

	now := time.Now()
	longID := "this-is-a-very-long-execution-id-that-exceeds-20-chars"
	longWorkflowName := "very-long-workflow-name-that-exceeds-limit"
	records := []*workflow.ExecutionRecord{
		{
			ID:           longID,
			WorkflowID:   "wf-001",
			WorkflowName: longWorkflowName,
			Status:       "success",
			ExitCode:     0,
			StartedAt:    now.Add(-5 * time.Minute),
			CompletedAt:  now,
			DurationMs:   1500,
		},
	}

	err := writeHistoryRecords(writer, records)
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, longID)
	assert.Contains(t, output, longWorkflowName)
}

func TestHistoryInfo_Struct(t *testing.T) {
	info := HistoryInfo{
		ID:           "test-id",
		WorkflowID:   "wf-id",
		WorkflowName: "test-workflow",
		Status:       "success",
		ExitCode:     0,
		StartedAt:    "2025-12-11T10:00:00Z",
		CompletedAt:  "2025-12-11T10:05:00Z",
		DurationMs:   300000,
		ErrorMessage: "",
	}

	assert.Equal(t, "test-id", info.ID)
	assert.Equal(t, "wf-id", info.WorkflowID)
	assert.Equal(t, "test-workflow", info.WorkflowName)
	assert.Equal(t, "success", info.Status)
	assert.Equal(t, 0, info.ExitCode)
	assert.Equal(t, int64(300000), info.DurationMs)
}

func TestWriteHistoryRecords_TabwriterFormattedTable(t *testing.T) {
	var out bytes.Buffer
	writer := ui.NewOutputWriter(&out, &out, ui.FormatText, true, false)

	now := time.Now()
	records := []*workflow.ExecutionRecord{
		{
			ID:           "exec-tabwriter-001",
			WorkflowID:   "wf-001",
			WorkflowName: "deploy",
			Status:       "success",
			StartedAt:    now.Add(-2 * time.Minute),
			CompletedAt:  now,
			DurationMs:   120000,
		},
	}

	err := writeHistoryRecords(writer, records)
	require.NoError(t, err)

	output := out.String()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	require.GreaterOrEqual(t, len(lines), 3, "tabwriter output must have header, separator, and data rows")

	headerLine := lines[0]
	for _, col := range []string{"ID", "WORKFLOW", "STATUS", "DURATION", "COMPLETED"} {
		assert.Contains(t, headerLine, col, "header must contain column %q", col)
	}

	assert.Contains(t, lines[1], "----")

	dataLine := lines[2]
	assert.Contains(t, dataLine, "exec-tabwriter-001")
	assert.Contains(t, dataLine, "deploy")
	assert.Contains(t, dataLine, "success")
}

func TestWriteHistoryRecords_NoTruncation(t *testing.T) {
	var out bytes.Buffer
	writer := ui.NewOutputWriter(&out, &out, ui.FormatText, true, false)

	now := time.Now()
	longExecID := "exec-" + strings.Repeat("abcdef01234567890", 4)
	longWorkflowName := "deploy-" + strings.Repeat("region-failover-", 3)
	records := []*workflow.ExecutionRecord{
		{
			ID:           longExecID,
			WorkflowID:   "wf-001",
			WorkflowName: longWorkflowName,
			Status:       "success",
			StartedAt:    now.Add(-time.Minute),
			CompletedAt:  now,
			DurationMs:   60000,
		},
	}

	err := writeHistoryRecords(writer, records)
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, longExecID, "execution ID must not be truncated")
	assert.Contains(t, output, longWorkflowName, "workflow name must not be truncated")
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
