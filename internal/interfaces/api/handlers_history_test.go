package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
)

type capturingHistoryProvider struct {
	capturedFilter *workflow.HistoryFilter
	records        []*workflow.ExecutionRecord
	stats          *workflow.HistoryStats
}

func (m *capturingHistoryProvider) List(_ context.Context, filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error) {
	m.capturedFilter = filter
	return m.records, nil
}

func (m *capturingHistoryProvider) GetStats(_ context.Context, filter *workflow.HistoryFilter) (*workflow.HistoryStats, error) {
	m.capturedFilter = filter
	return m.stats, nil
}

func TestHistoryHandler_List_FiltersByWorkflowAndStatus(t *testing.T) {
	mock := &capturingHistoryProvider{
		records: []*workflow.ExecutionRecord{
			{ID: "rec-1", WorkflowName: "deploy-prod", Status: "success"},
			{ID: "rec-2", WorkflowName: "deploy-prod", Status: "success"},
		},
	}

	bridge := NewBridge(newMockWorkflowLister(), nil, mock)
	handler := NewHistoryHandlers(bridge)
	_, api := humatest.New(t)
	RegisterHistoryRoutes(api, handler)

	resp := api.Get("/api/history?workflow=deploy-prod&status=success")
	require.Equal(t, 200, resp.Code, "List must return 200 OK")

	// Assert filter values reached the mock unchanged.
	require.NotNil(t, mock.capturedFilter, "filter must be captured")
	assert.Equal(t, "deploy-prod", mock.capturedFilter.WorkflowName, "filter must contain workflow name from query")
	assert.Equal(t, "success", mock.capturedFilter.Status, "filter must contain status from query")
	assert.True(t, mock.capturedFilter.Since.IsZero(), "zero Since must remain zero (no-filter convention)")

	var result struct {
		Body struct {
			Entries []HistoryEntry `json:"entries"`
		} `json:"body"`
	}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err, "response must be valid JSON")

	assert.Len(t, result.Body.Entries, 2, "response must contain both mocked records")
	assert.Equal(t, "rec-1", result.Body.Entries[0].ID)
	assert.Equal(t, "deploy-prod", result.Body.Entries[0].WorkflowName)
	assert.Equal(t, "success", result.Body.Entries[0].Status)
}

func TestHistoryHandler_Stats_ReturnsAggregates(t *testing.T) {
	mock := &capturingHistoryProvider{
		stats: &workflow.HistoryStats{
			TotalExecutions: 5,
			SuccessCount:    3,
			FailedCount:     1,
			CancelledCount:  1,
			AvgDurationMs:   2500,
		},
	}

	bridge := NewBridge(newMockWorkflowLister(), nil, mock)
	handler := NewHistoryHandlers(bridge)
	_, api := humatest.New(t)
	RegisterHistoryRoutes(api, handler)

	resp := api.Get("/api/history/stats")
	require.Equal(t, 200, resp.Code, "Stats must return 200 OK")

	// Decode using the typed struct directly — consistent with GetWorkflowOutput test pattern.
	var result struct {
		Body *workflow.HistoryStats `json:"body"`
	}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err, "response must be valid JSON")
	require.NotNil(t, result.Body, "stats body must not be nil")

	assert.Equal(t, 5, result.Body.TotalExecutions, "must return TotalExecutions from GetStats")
	assert.Equal(t, 3, result.Body.SuccessCount, "must return SuccessCount from GetStats")
	assert.Equal(t, 1, result.Body.FailedCount, "must return FailedCount from GetStats")
	assert.Equal(t, 1, result.Body.CancelledCount, "must return CancelledCount from GetStats")
	assert.Equal(t, int64(2500), result.Body.AvgDurationMs, "must return AvgDurationMs from GetStats")
}

func TestHistoryHandler_Stats_FiltersByWorkflowAndStatus(t *testing.T) {
	mock := &capturingHistoryProvider{
		stats: &workflow.HistoryStats{
			TotalExecutions: 2,
			SuccessCount:    1,
			FailedCount:     1,
		},
	}

	bridge := NewBridge(newMockWorkflowLister(), nil, mock)
	handler := NewHistoryHandlers(bridge)
	_, api := humatest.New(t)
	RegisterHistoryRoutes(api, handler)

	resp := api.Get("/api/history/stats?workflow=deploy-prod&status=success")
	require.Equal(t, 200, resp.Code, "Stats must return 200 OK")

	// Assert filter values reached the mock unchanged.
	require.NotNil(t, mock.capturedFilter, "filter must be captured")
	assert.Equal(t, "deploy-prod", mock.capturedFilter.WorkflowName, "filter must contain workflow name from query")
	assert.Equal(t, "success", mock.capturedFilter.Status, "filter must contain status from query")
	assert.True(t, mock.capturedFilter.Since.IsZero(), "zero Since must remain zero (no-filter convention)")
}
