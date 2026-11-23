package cli

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

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

// recordInterruptedExecutions waits for the executions `awf serve` cancelled to
// record their own outcome, then records the ones still running so a restart
// does not look like a run that vanished.
func recordInterruptedExecutions(
	cmd *cobra.Command,
	facade *application.Adapter,
	bridge *api.Bridge,
	historyStore ports.HistoryStore,
	stateStore ports.StateStore,
	logger ports.Logger,
) {
	ids := waitForExecutions(facade, inFlightDrainTimeout)
	if len(ids) == 0 {
		return
	}
	recorded := recordInterrupted(context.Background(), historyStore, stateStore, logger,
		interruptedExecutions(bridge, ids), time.Now())
	cmd.Printf("Recorded %d interrupted execution(s)\n", recorded)
}

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
// short, so the dashboard and `awf history` show the run instead of losing it.
//
// The record reuses the history identity the execution persisted before its
// attempt started: a resumed run owns a later attempt and a predecessor, so
// claiming attempt 1 here would collide with the predecessor and leave the live
// attempt out of history. An execution that already finalized its own record is
// skipped, and one that finalizes after this write keeps the record below: both
// writes claim the same attempt, so one of them wins and the other is a no-op.
func recordInterrupted(
	ctx context.Context,
	store ports.HistoryStore,
	stateStore ports.StateStore,
	logger ports.Logger,
	executions []*api.ActiveExecution,
	now time.Time,
) int {
	recorded := 0
	for _, execution := range executions {
		state, err := stateStore.Load(ctx, execution.ExecutionID)
		if err != nil {
			logger.Warn("failed to read interrupted execution state",
				"execution_id", execution.ExecutionID,
				"error", err,
			)
			state = nil
		}
		if state != nil && state.HistoryRecorded {
			continue
		}
		record := interruptedRecord(execution, state, now)
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

// interruptedRecord builds the failed record for one execution, using the
// persisted attempt identity when the state is readable and a fresh first
// attempt otherwise so an unreadable state never costs the run its record.
func interruptedRecord(
	execution *api.ActiveExecution,
	state *workflow.ExecutionContext,
	now time.Time,
) *workflow.ExecutionRecord {
	record := &workflow.ExecutionRecord{
		ID:           uuid.NewString(),
		WorkflowID:   execution.ExecutionID,
		Attempt:      1,
		WorkflowName: execution.WorkflowName,
		Status:       string(workflow.StatusFailed),
		ExitCode:     interruptedExitCode,
		StartedAt:    execution.StartedAt,
		CompletedAt:  now,
		ErrorMessage: "server shut down before the execution finished",
	}
	if state != nil && state.HistoryRecordID != "" && state.HistoryAttempt > 0 {
		record.ID = state.HistoryRecordID
		record.Attempt = state.HistoryAttempt
		record.ResumedFromID = state.PreviousHistoryRecordID
		if !state.StartedAt.IsZero() {
			record.StartedAt = state.StartedAt
		}
	}
	record.DurationMs = now.Sub(record.StartedAt).Milliseconds()
	return record
}
