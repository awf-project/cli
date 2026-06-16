package cli

import (
	"context"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/spf13/cobra"
)

type HistoryInfo struct {
	ID           string `json:"id"`
	WorkflowID   string `json:"workflow_id"`
	WorkflowName string `json:"workflow_name"`
	Status       string `json:"status"`
	ExitCode     int    `json:"exit_code,omitempty"`
	StartedAt    string `json:"started_at"`
	CompletedAt  string `json:"completed_at"`
	DurationMs   int64  `json:"duration_ms"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type HistoryStatsInfo struct {
	TotalExecutions int   `json:"total_executions"`
	SuccessCount    int   `json:"success_count"`
	FailedCount     int   `json:"failed_count"`
	CancelledCount  int   `json:"cancelled_count"`
	AvgDurationMs   int64 `json:"avg_duration_ms"`
}

func newHistoryCommand(cfg *Config) *cobra.Command {
	var (
		workflowName string
		status       string
		since        string
		limit        int
		showStats    bool
	)

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show workflow execution history",
		Long: `Display past workflow executions with filtering and statistics.

Examples:
  awf history
  awf history --workflow deploy
  awf history --status failed --since 2025-12-01
  awf history --stats`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHistory(cmd, cfg, workflowName, status, since, limit, showStats)
		},
	}

	cmd.Flags().StringVarP(&workflowName, "workflow", "w", "", "Filter by workflow name")
	cmd.Flags().StringVarP(&status, "status", "s", "", "Filter by status (success, failed, cancelled)")
	cmd.Flags().StringVar(&since, "since", "", "Show executions since date (YYYY-MM-DD)")
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Maximum entries to show")
	cmd.Flags().BoolVar(&showStats, "stats", false, "Show statistics only")

	return cmd
}

func runHistory(cmd *cobra.Command, cfg *Config, workflowName, status, since string, limit int, showStats bool) error {
	ctx := context.Background()
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)

	var sinceTime time.Time
	if since != "" {
		t, parseErr := time.Parse("2006-01-02", since)
		if parseErr != nil {
			return writeErrorAndExit(writer, fmt.Errorf("invalid --since format, expected YYYY-MM-DD: %w", parseErr), ExitUser)
		}
		sinceTime = t
	}

	// Route through WorkflowFacade when wired (T069). Both the record-listing and
	// --stats paths go through the facade: stats are computed client-side from the
	// RunRecord slice returned by cfg.Facade.History (ports.WorkflowFacade has no
	// dedicated stats method and must not be modified by this task).
	if cfg.Facade != nil {
		records, err := cfg.Facade.History(ctx, ports.HistoryFilter{
			WorkflowName: workflowName,
			Status:       status,
			Since:        sinceTime,
			Limit:        limit,
		})
		if err != nil {
			return writeErrorAndExit(writer, fmt.Errorf("list history: %w", err), ExitSystem)
		}
		if showStats {
			return writeHistoryRunRecordsStats(writer, records)
		}
		return writeHistoryRunRecords(writer, records)
	}

	// Facade not wired: return a meaningful error stub. Production always wires
	// the facade via NewRootCommandAutoFacade, so this branch is never reached in
	// normal usage.
	err := fmt.Errorf("history requires facade wiring (use NewRootCommandAutoFacade)")
	return writeErrorAndExit(writer, err, ExitSystem)
}

// writeHistoryRunRecords renders facade RunRecords as HistoryInfo rows. The
// mapping is field-for-field, so JSON and table output stay identical
// regardless of the underlying source.
func writeHistoryRunRecords(writer *ui.OutputWriter, records []ports.RunRecord) error {
	infos := make([]HistoryInfo, len(records))
	for i := range records {
		r := &records[i]
		infos[i] = HistoryInfo{
			ID:           r.RunID,
			WorkflowID:   r.WorkflowID,
			WorkflowName: r.WorkflowName,
			Status:       string(r.Status),
			StartedAt:    r.StartedAt.Format(time.RFC3339),
			CompletedAt:  r.CompletedAt.Format(time.RFC3339),
			DurationMs:   r.DurationMs,
			ErrorMessage: r.ErrorMessage,
		}
	}
	return writeHistoryInfos(writer, infos)
}

// writeHistoryRunRecordsStats computes execution statistics client-side from
// the RunRecord slice returned by cfg.Facade.History and renders them in the
// same format as writeHistoryStats. This avoids requiring a dedicated stats
// method on ports.WorkflowFacade (which must not be modified by T069).
func writeHistoryRunRecordsStats(writer *ui.OutputWriter, records []ports.RunRecord) error {
	var total, success, failed, cancelled int
	var totalDurationMs int64
	for i := range records {
		r := &records[i]
		total++
		switch r.Status {
		case ports.RunStateCompleted:
			success++
		case ports.RunStateFailed:
			failed++
		case ports.RunStateCancelled:
			cancelled++
		case ports.RunStatePending, ports.RunStateRunning:
			// in-progress: not counted as terminal outcome
		}
		totalDurationMs += r.DurationMs
	}
	var avgDurationMs int64
	if total > 0 {
		avgDurationMs = totalDurationMs / int64(total)
	}
	stats := &workflow.HistoryStats{
		TotalExecutions: total,
		SuccessCount:    success,
		FailedCount:     failed,
		CancelledCount:  cancelled,
		AvgDurationMs:   avgDurationMs,
	}
	return writeHistoryStats(writer, stats)
}

func writeHistoryStats(writer *ui.OutputWriter, stats *workflow.HistoryStats) error {
	info := HistoryStatsInfo{
		TotalExecutions: stats.TotalExecutions,
		SuccessCount:    stats.SuccessCount,
		FailedCount:     stats.FailedCount,
		CancelledCount:  stats.CancelledCount,
		AvgDurationMs:   stats.AvgDurationMs,
	}

	if writer.IsJSONFormat() {
		return writer.WriteJSON(info)
	}

	_, _ = fmt.Fprintf(writer.Out(), "Execution Statistics\n")
	_, _ = fmt.Fprintf(writer.Out(), "====================\n")
	_, _ = fmt.Fprintf(writer.Out(), "Total Executions: %d\n", stats.TotalExecutions)
	_, _ = fmt.Fprintf(writer.Out(), "Success: %d\n", stats.SuccessCount)
	_, _ = fmt.Fprintf(writer.Out(), "Failed: %d\n", stats.FailedCount)
	_, _ = fmt.Fprintf(writer.Out(), "Cancelled: %d\n", stats.CancelledCount)
	if stats.TotalExecutions > 0 {
		_, _ = fmt.Fprintf(writer.Out(), "Average Duration: %dms\n", stats.AvgDurationMs)
	}

	return nil
}

// writeHistoryInfos renders a prepared HistoryInfo slice in JSON or table form.
// Shared by the facade RunRecord rendering paths so output formatting stays
// identical regardless of the caller.
func writeHistoryInfos(writer *ui.OutputWriter, infos []HistoryInfo) error {
	if writer.IsJSONFormat() {
		return writer.WriteJSON(infos)
	}

	if len(infos) == 0 {
		_, _ = fmt.Fprintln(writer.Out(), "No execution history found")
		return nil
	}

	w := tabwriter.NewWriter(writer.Out(), 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tWORKFLOW\tSTATUS\tDURATION\tCOMPLETED")
	_, _ = fmt.Fprintln(w, "----\t--------\t------\t--------\t---------")
	for i := range infos {
		info := &infos[i]
		duration := formatDuration(info.DurationMs)
		completedAt, _ := time.Parse(time.RFC3339, info.CompletedAt)
		_, _ = fmt.Fprintf(
			w, "%s\t%s\t%s\t%s\t%s\n",
			info.ID,
			info.WorkflowName,
			info.Status,
			duration,
			completedAt.Format("2006-01-02 15:04"),
		)
	}

	return w.Flush()
}

func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	if ms < 60000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	return fmt.Sprintf("%.1fm", float64(ms)/60000)
}
