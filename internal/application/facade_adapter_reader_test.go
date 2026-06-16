package application

import (
	"context"
	"errors"
	"testing"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests cover the ports.WorkflowReader methods added so the HTTP API can route its
// read-only "get single workflow" and "history stats" endpoints through the facade Adapter
// instead of bypassing it via legacy Bridge ports.

func TestAdapter_GetWorkflow_DelegatesToWorkflowService(t *testing.T) {
	repo := testmocks.NewMockWorkflowRepository()
	repo.AddWorkflow("demo", &workflow.Workflow{
		Name:  "demo",
		Steps: map[string]*workflow.Step{"s1": {Name: "s1"}},
	})
	adapter := NewAdapter(
		NewWorkflowService(repo, nil, nil, nil, nil),
		&ExecutionService{},
		&HistoryService{},
		nil,
		&mockRecorder{},
		NewSessionRegistry(),
	)

	wf, err := adapter.GetWorkflow(context.Background(), "demo")
	require.NoError(t, err)
	require.NotNil(t, wf)
	assert.Equal(t, "demo", wf.Name)
}

func TestAdapter_GetWorkflow_MissingWorkflow_ReturnsMissingFileError(t *testing.T) {
	adapter := NewAdapter(
		NewWorkflowService(testmocks.NewMockWorkflowRepository(), nil, nil, nil, nil),
		&ExecutionService{},
		&HistoryService{},
		nil,
		&mockRecorder{},
		NewSessionRegistry(),
	)

	_, err := adapter.GetWorkflow(context.Background(), "does-not-exist")
	require.Error(t, err)
	var se *domainerrors.StructuredError
	require.True(t, errors.As(err, &se), "missing workflow must surface a StructuredError")
	assert.Equal(t, domainerrors.ErrorCodeUserInputMissingFile, se.Code,
		"missing workflow must map to USER.INPUT.MISSING_FILE so the HTTP handler returns 404")
}

func TestAdapter_GetWorkflow_NilService_ReturnsError(t *testing.T) {
	adapter := &Adapter{}
	_, err := adapter.GetWorkflow(context.Background(), "anything")
	require.Error(t, err, "GetWorkflow must error (not panic) when the workflow service is not configured")
}

func TestAdapter_HistoryStats_DelegatesToHistoryService(t *testing.T) {
	store := testmocks.NewMockHistoryStore()
	require.NoError(t, store.Record(context.Background(), &workflow.ExecutionRecord{
		ID:           "run-1",
		WorkflowName: "demo",
		Status:       "success",
		DurationMs:   100,
	}))
	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		NewHistoryService(store, nil),
		nil,
		&mockRecorder{},
		NewSessionRegistry(),
	)

	// The mock store's GetStats returns a (zero) HistoryStats value; the point of this test
	// is that HistoryStats delegates to the configured service/store and returns its result
	// (non-nil, no error) rather than hitting the nil-service guard below.
	stats, err := adapter.HistoryStats(context.Background(), ports.HistoryFilter{})
	require.NoError(t, err)
	require.NotNil(t, stats, "configured store must yield a non-nil stats value")
}

func TestAdapter_HistoryStats_NilService_ReturnsError(t *testing.T) {
	adapter := &Adapter{}
	_, err := adapter.HistoryStats(context.Background(), ports.HistoryFilter{})
	require.Error(t, err, "HistoryStats must error (not panic) when the history service is not configured")
}
