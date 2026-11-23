package cli

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/interfaces/api"
)

// inFlightDrainTimeout bounds how long the server waits for the executions it
// cancelled at shutdown to record their own outcome. Docker's default stop grace
// period is 10 seconds, so the wait stays well under it.
const inFlightDrainTimeout = 5 * time.Second

// inFlightPollInterval is how often the drain wait looks for finished executions.
const inFlightPollInterval = 25 * time.Millisecond

// interruptedExitCode is the exit code stored on an execution a shutdown cut short.
const interruptedExitCode = 1

// waitForExecutions blocks until the facade reports no running execution, or
// timeout elapses. It returns the IDs still running when it gives up.
func waitForExecutions(facade *application.Adapter, timeout time.Duration) []string {
	deadline := time.Now().Add(timeout)
	for {
		pending := facade.PendingExecutions()
		if len(pending) == 0 || !time.Now().Before(deadline) {
			return pending
		}
		time.Sleep(inFlightPollInterval)
	}
}

// interruptedExecutions maps the execution IDs left running at shutdown to the
// metadata the Bridge tracked when each one was accepted.
func interruptedExecutions(bridge *api.Bridge, ids []string) []*api.ActiveExecution {
	executions := make([]*api.ActiveExecution, 0, len(ids))
	for _, id := range ids {
		if execution, ok := bridge.GetExecution(id); ok {
			executions = append(executions, execution)
		}
	}
	return executions
}

// recordInterrupted writes one failed history record per execution a shutdown cut
// short, so the dashboard and `awf history` show the run instead of losing it. The
// record claims attempt 1 with no predecessor, because the attempt the live run
// allocated is not visible outside the execution.
//
// The write happens after the drain wait, so an execution that did finish already
// wrote its own record; this insert then fails on the identity constraint and the
// existing record is kept.
func recordInterrupted(
	ctx context.Context,
	store ports.HistoryStore,
	logger ports.Logger,
	executions []*api.ActiveExecution,
	now time.Time,
) int {
	recorded := 0
	for _, execution := range executions {
		record := &workflow.ExecutionRecord{
			ID:           uuid.NewString(),
			WorkflowID:   execution.ExecutionID,
			Attempt:      1,
			WorkflowName: execution.WorkflowName,
			Status:       string(workflow.StatusFailed),
			ExitCode:     interruptedExitCode,
			StartedAt:    execution.StartedAt,
			CompletedAt:  now,
			DurationMs:   now.Sub(execution.StartedAt).Milliseconds(),
			ErrorMessage: "server shut down before the execution finished",
		}
		if err := store.Record(ctx, record); err != nil {
			logger.Warn("failed to record interrupted execution",
				"execution_id", execution.ExecutionID,
				"error", err,
			)
			continue
		}
		recorded++
	}
	return recorded
}
