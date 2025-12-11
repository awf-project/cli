package workflow_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/vanoix/awf/internal/domain/workflow"
)

func TestExecutionRecord_Fields(t *testing.T) {
	tests := []struct {
		name   string
		record workflow.ExecutionRecord
		check  func(t *testing.T, r workflow.ExecutionRecord)
	}{
		{
			name: "success record with all fields",
			record: workflow.ExecutionRecord{
				ID:           "exec-123",
				WorkflowID:   "wf-456",
				WorkflowName: "deploy",
				Status:       "success",
				ExitCode:     0,
				StartedAt:    time.Date(2025, 12, 10, 10, 0, 0, 0, time.UTC),
				CompletedAt:  time.Date(2025, 12, 10, 10, 5, 0, 0, time.UTC),
				DurationMs:   300000,
				ErrorMessage: "",
			},
			check: func(t *testing.T, r workflow.ExecutionRecord) {
				assert.Equal(t, "exec-123", r.ID)
				assert.Equal(t, "wf-456", r.WorkflowID)
				assert.Equal(t, "deploy", r.WorkflowName)
				assert.Equal(t, "success", r.Status)
				assert.Equal(t, 0, r.ExitCode)
				assert.Equal(t, int64(300000), r.DurationMs)
				assert.Empty(t, r.ErrorMessage)
			},
		},
		{
			name: "failed record with error message",
			record: workflow.ExecutionRecord{
				ID:           "exec-789",
				WorkflowID:   "wf-789",
				WorkflowName: "build",
				Status:       "failed",
				ExitCode:     1,
				StartedAt:    time.Date(2025, 12, 10, 11, 0, 0, 0, time.UTC),
				CompletedAt:  time.Date(2025, 12, 10, 11, 0, 30, 0, time.UTC),
				DurationMs:   30000,
				ErrorMessage: "step 'compile' failed with exit code 1",
			},
			check: func(t *testing.T, r workflow.ExecutionRecord) {
				assert.Equal(t, "failed", r.Status)
				assert.Equal(t, 1, r.ExitCode)
				assert.Equal(t, "step 'compile' failed with exit code 1", r.ErrorMessage)
			},
		},
		{
			name: "cancelled record",
			record: workflow.ExecutionRecord{
				ID:           "exec-cancelled",
				WorkflowID:   "wf-test",
				WorkflowName: "test",
				Status:       "cancelled",
				ExitCode:     130,
				StartedAt:    time.Date(2025, 12, 10, 12, 0, 0, 0, time.UTC),
				CompletedAt:  time.Date(2025, 12, 10, 12, 0, 10, 0, time.UTC),
				DurationMs:   10000,
				ErrorMessage: "context cancelled",
			},
			check: func(t *testing.T, r workflow.ExecutionRecord) {
				assert.Equal(t, "cancelled", r.Status)
				assert.Equal(t, 130, r.ExitCode)
			},
		},
		{
			name: "zero duration record",
			record: workflow.ExecutionRecord{
				ID:           "exec-instant",
				WorkflowID:   "wf-instant",
				WorkflowName: "instant",
				Status:       "success",
				ExitCode:     0,
				StartedAt:    time.Now(),
				CompletedAt:  time.Now(),
				DurationMs:   0,
			},
			check: func(t *testing.T, r workflow.ExecutionRecord) {
				assert.Equal(t, int64(0), r.DurationMs)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, tc.record)
		})
	}
}

func TestHistoryFilter_Fields(t *testing.T) {
	tests := []struct {
		name   string
		filter workflow.HistoryFilter
		check  func(t *testing.T, f workflow.HistoryFilter)
	}{
		{
			name:   "empty filter",
			filter: workflow.HistoryFilter{},
			check: func(t *testing.T, f workflow.HistoryFilter) {
				assert.Empty(t, f.WorkflowName)
				assert.Empty(t, f.Status)
				assert.True(t, f.Since.IsZero())
				assert.True(t, f.Until.IsZero())
				assert.Equal(t, 0, f.Limit)
			},
		},
		{
			name: "filter by workflow name",
			filter: workflow.HistoryFilter{
				WorkflowName: "deploy",
			},
			check: func(t *testing.T, f workflow.HistoryFilter) {
				assert.Equal(t, "deploy", f.WorkflowName)
			},
		},
		{
			name: "filter by status",
			filter: workflow.HistoryFilter{
				Status: "failed",
			},
			check: func(t *testing.T, f workflow.HistoryFilter) {
				assert.Equal(t, "failed", f.Status)
			},
		},
		{
			name: "filter by date range",
			filter: workflow.HistoryFilter{
				Since: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
				Until: time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
			},
			check: func(t *testing.T, f workflow.HistoryFilter) {
				assert.Equal(t, 2025, f.Since.Year())
				assert.Equal(t, time.December, f.Since.Month())
				assert.Equal(t, 1, f.Since.Day())
			},
		},
		{
			name: "filter with limit",
			filter: workflow.HistoryFilter{
				Limit: 50,
			},
			check: func(t *testing.T, f workflow.HistoryFilter) {
				assert.Equal(t, 50, f.Limit)
			},
		},
		{
			name: "combined filter",
			filter: workflow.HistoryFilter{
				WorkflowName: "build",
				Status:       "success",
				Since:        time.Date(2025, 12, 10, 0, 0, 0, 0, time.UTC),
				Limit:        100,
			},
			check: func(t *testing.T, f workflow.HistoryFilter) {
				assert.Equal(t, "build", f.WorkflowName)
				assert.Equal(t, "success", f.Status)
				assert.Equal(t, 10, f.Since.Day())
				assert.Equal(t, 100, f.Limit)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, tc.filter)
		})
	}
}

func TestHistoryStats_Fields(t *testing.T) {
	tests := []struct {
		name  string
		stats workflow.HistoryStats
		check func(t *testing.T, s workflow.HistoryStats)
	}{
		{
			name:  "zero stats",
			stats: workflow.HistoryStats{},
			check: func(t *testing.T, s workflow.HistoryStats) {
				assert.Equal(t, 0, s.TotalExecutions)
				assert.Equal(t, 0, s.SuccessCount)
				assert.Equal(t, 0, s.FailedCount)
				assert.Equal(t, 0, s.CancelledCount)
				assert.Equal(t, int64(0), s.AvgDurationMs)
			},
		},
		{
			name: "all success",
			stats: workflow.HistoryStats{
				TotalExecutions: 10,
				SuccessCount:    10,
				FailedCount:     0,
				CancelledCount:  0,
				AvgDurationMs:   5000,
			},
			check: func(t *testing.T, s workflow.HistoryStats) {
				assert.Equal(t, 10, s.TotalExecutions)
				assert.Equal(t, 10, s.SuccessCount)
				assert.Equal(t, 0, s.FailedCount)
				assert.Equal(t, int64(5000), s.AvgDurationMs)
			},
		},
		{
			name: "mixed results",
			stats: workflow.HistoryStats{
				TotalExecutions: 100,
				SuccessCount:    70,
				FailedCount:     25,
				CancelledCount:  5,
				AvgDurationMs:   12500,
			},
			check: func(t *testing.T, s workflow.HistoryStats) {
				assert.Equal(t, 100, s.TotalExecutions)
				assert.Equal(t, 70, s.SuccessCount)
				assert.Equal(t, 25, s.FailedCount)
				assert.Equal(t, 5, s.CancelledCount)
				// Verify counts add up
				assert.Equal(t, s.TotalExecutions, s.SuccessCount+s.FailedCount+s.CancelledCount)
			},
		},
		{
			name: "high duration average",
			stats: workflow.HistoryStats{
				TotalExecutions: 5,
				SuccessCount:    3,
				FailedCount:     2,
				CancelledCount:  0,
				AvgDurationMs:   3600000, // 1 hour average
			},
			check: func(t *testing.T, s workflow.HistoryStats) {
				assert.Equal(t, int64(3600000), s.AvgDurationMs)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, tc.stats)
		})
	}
}

func TestExecutionRecord_DurationCalculation(t *testing.T) {
	// Test that DurationMs can be correctly derived from timestamps
	tests := []struct {
		name               string
		startedAt          time.Time
		completedAt        time.Time
		expectedDurationMs int64
	}{
		{
			name:               "5 minute duration",
			startedAt:          time.Date(2025, 12, 10, 10, 0, 0, 0, time.UTC),
			completedAt:        time.Date(2025, 12, 10, 10, 5, 0, 0, time.UTC),
			expectedDurationMs: 300000,
		},
		{
			name:               "sub-second duration",
			startedAt:          time.Date(2025, 12, 10, 10, 0, 0, 0, time.UTC),
			completedAt:        time.Date(2025, 12, 10, 10, 0, 0, 500000000, time.UTC),
			expectedDurationMs: 500,
		},
		{
			name:               "zero duration",
			startedAt:          time.Date(2025, 12, 10, 10, 0, 0, 0, time.UTC),
			completedAt:        time.Date(2025, 12, 10, 10, 0, 0, 0, time.UTC),
			expectedDurationMs: 0,
		},
		{
			name:               "1 hour duration",
			startedAt:          time.Date(2025, 12, 10, 10, 0, 0, 0, time.UTC),
			completedAt:        time.Date(2025, 12, 10, 11, 0, 0, 0, time.UTC),
			expectedDurationMs: 3600000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			record := workflow.ExecutionRecord{
				ID:          "test",
				WorkflowID:  "test",
				Status:      "success",
				StartedAt:   tc.startedAt,
				CompletedAt: tc.completedAt,
				DurationMs:  tc.completedAt.Sub(tc.startedAt).Milliseconds(),
			}
			assert.Equal(t, tc.expectedDurationMs, record.DurationMs)
		})
	}
}

func TestHistoryFilter_DateRangeLogic(t *testing.T) {
	// Test date range boundary conditions
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)
	lastWeek := now.Add(-7 * 24 * time.Hour)

	tests := []struct {
		name   string
		filter workflow.HistoryFilter
		check  func(t *testing.T, f workflow.HistoryFilter)
	}{
		{
			name: "since yesterday",
			filter: workflow.HistoryFilter{
				Since: yesterday,
			},
			check: func(t *testing.T, f workflow.HistoryFilter) {
				assert.True(t, f.Since.Before(now))
				assert.True(t, f.Until.IsZero())
			},
		},
		{
			name: "until tomorrow",
			filter: workflow.HistoryFilter{
				Until: tomorrow,
			},
			check: func(t *testing.T, f workflow.HistoryFilter) {
				assert.True(t, f.Until.After(now))
				assert.True(t, f.Since.IsZero())
			},
		},
		{
			name: "last week range",
			filter: workflow.HistoryFilter{
				Since: lastWeek,
				Until: now,
			},
			check: func(t *testing.T, f workflow.HistoryFilter) {
				assert.True(t, f.Since.Before(f.Until))
				duration := f.Until.Sub(f.Since)
				assert.InDelta(t, 7*24*time.Hour, duration, float64(time.Hour))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, tc.filter)
		})
	}
}

func TestExecutionRecord_StatusValues(t *testing.T) {
	// Document expected status values
	validStatuses := []string{"success", "failed", "cancelled"}

	for _, status := range validStatuses {
		t.Run("status_"+status, func(t *testing.T) {
			record := workflow.ExecutionRecord{
				ID:           "test",
				WorkflowID:   "test",
				WorkflowName: "test",
				Status:       status,
			}
			assert.Equal(t, status, record.Status)
		})
	}
}
