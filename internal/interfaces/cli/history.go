package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/logger"
	"github.com/vanoix/awf/internal/infrastructure/store"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

// HistoryInfo is the JSON/output structure for history command.
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

// HistoryStatsInfo is the JSON/output structure for history stats.
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
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor)

	// Open history store
	historyPath := filepath.Join(cfg.StoragePath, "history")
	historyStore, err := store.NewBadgerHistoryStore(historyPath)
	if err != nil {
		return writer.WriteError(fmt.Errorf("open history store: %w", err), 4) // system error
	}
	defer func() { _ = historyStore.Close() }()

	// Create logger (silent for CLI commands)
	log := logger.NewConsoleLogger(io.Discard, logger.LevelWarn, cfg.NoColor)

	// Create history service
	historySvc := application.NewHistoryService(historyStore, log)

	// Build filter
	filter := &workflow.HistoryFilter{
		WorkflowName: workflowName,
		Status:       status,
		Limit:        limit,
	}
	if since != "" {
		t, err := time.Parse("2006-01-02", since)
		if err != nil {
			return writer.WriteError(fmt.Errorf("invalid --since format, expected YYYY-MM-DD: %w", err), 1) // user error
		}
		filter.Since = t
	}

	if showStats {
		stats, err := historySvc.GetStats(ctx, filter)
		if err != nil {
			return writer.WriteError(fmt.Errorf("get stats: %w", err), 4)
		}
		return writeHistoryStats(writer, cfg, stats)
	}

	records, err := historySvc.List(ctx, filter)
	if err != nil {
		return writer.WriteError(fmt.Errorf("list history: %w", err), 4)
	}

	return writeHistoryRecords(writer, cfg, records)
}

func writeHistoryStats(writer *ui.OutputWriter, cfg *Config, stats *workflow.HistoryStats) error {
	info := HistoryStatsInfo{
		TotalExecutions: stats.TotalExecutions,
		SuccessCount:    stats.SuccessCount,
		FailedCount:     stats.FailedCount,
		CancelledCount:  stats.CancelledCount,
		AvgDurationMs:   stats.AvgDurationMs,
	}

	if writer.IsJSONFormat() {
		return writeJSON(writer, info)
	}

	// Text/table output
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

func writeHistoryRecords(writer *ui.OutputWriter, cfg *Config, records []*workflow.ExecutionRecord) error {
	infos := make([]HistoryInfo, len(records))
	for i, r := range records {
		infos[i] = HistoryInfo{
			ID:           r.ID,
			WorkflowID:   r.WorkflowID,
			WorkflowName: r.WorkflowName,
			Status:       r.Status,
			ExitCode:     r.ExitCode,
			StartedAt:    r.StartedAt.Format(time.RFC3339),
			CompletedAt:  r.CompletedAt.Format(time.RFC3339),
			DurationMs:   r.DurationMs,
			ErrorMessage: r.ErrorMessage,
		}
	}

	if writer.IsJSONFormat() {
		return writeJSON(writer, infos)
	}

	if len(infos) == 0 {
		_, _ = fmt.Fprintln(writer.Out(), "No execution history found")
		return nil
	}

	// Text/table output
	_, _ = fmt.Fprintf(writer.Out(), "%-20s %-15s %-10s %-12s %s\n", "ID", "WORKFLOW", "STATUS", "DURATION", "COMPLETED")
	_, _ = fmt.Fprintf(writer.Out(), "%-20s %-15s %-10s %-12s %s\n", "--------------------", "---------------", "----------", "------------", "---------")
	for _, info := range infos {
		completedAt, _ := time.Parse(time.RFC3339, info.CompletedAt)
		duration := formatDuration(info.DurationMs)
		_, _ = fmt.Fprintf(writer.Out(), "%-20s %-15s %-10s %-12s %s\n",
			truncate(info.ID, 20),
			truncate(info.WorkflowName, 15),
			info.Status,
			duration,
			completedAt.Format("2006-01-02 15:04"),
		)
	}

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
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

func writeJSON(writer *ui.OutputWriter, v any) error {
	// OutputWriter handles JSON internally through other methods,
	// but we need direct JSON output here
	return writer.WriteJSON(v)
}
