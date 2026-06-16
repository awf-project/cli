package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/spf13/cobra"
)

func newStatusCommand(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "status <workflow-id>",
		Short: "Show workflow execution status",
		Long: `Display the current status of a workflow execution.

Shows the execution state, progress, duration, and details of completed
and pending steps.

Examples:
  awf status abc123
  awf status abc123 --verbose`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd, cfg, args[0])
		},
	}
}

func runStatus(cmd *cobra.Command, cfg *Config, workflowID string) error {
	ctx := context.Background()

	// Create output writer
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)

	// Route through WorkflowFacade when wired (T060).
	if cfg.Facade != nil {
		return runStatusViaFacade(cmd, cfg, writer, ctx, workflowID)
	}

	// Facade not yet wired: return a "not found" error so callers get a meaningful message
	// instead of a nil-pointer panic. Once facade wiring is complete in main.go this branch
	// will never be reached in production.
	err := fmt.Errorf("workflow execution not found: %s (status lookup requires facade wiring)", workflowID)
	if writer.IsJSONFormat() {
		return writer.WriteError(err, ExitUser)
	}
	return writeErrorAndExit(writer, err, ExitUser)
}

// runStatusViaFacade delegates the status lookup to ports.WorkflowFacade.Status.
// It formats the returned RunStatus for all output modes (text/JSON/table/quiet).
func runStatusViaFacade(cmd *cobra.Command, cfg *Config, writer *ui.OutputWriter, ctx context.Context, runID string) error { //nolint:revive // context.Context not first param: writer is a pre-built dependency, not a new chain
	status, err := cfg.Facade.Status(ctx, runID)
	if err != nil {
		if writer.IsJSONFormat() {
			return writer.WriteError(err, ExitUser)
		}
		return writeErrorAndExit(writer, err, ExitUser)
	}
	if status.RunID == "" {
		notFound := fmt.Errorf("workflow execution not found: %s", runID)
		if writer.IsJSONFormat() {
			return writer.WriteError(notFound, ExitUser)
		}
		return writeErrorAndExit(writer, notFound, ExitUser)
	}

	// JSON/quiet/table format
	if cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatQuiet || cfg.OutputFormat == ui.FormatTable {
		execInfo := runStatusToExecutionInfo(&status)
		return writer.WriteExecution(&execInfo)
	}

	// Text format
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})
	displayRunStatus(formatter, &status, cfg.Verbose)
	return nil
}

// runStatusToExecutionInfo converts a ports.RunStatus to ui.ExecutionInfo for structured output.
// Populates Steps and CurrentStep from the enriched DTO fields for JSON/table/quiet parity.
func runStatusToExecutionInfo(s *ports.RunStatus) ui.ExecutionInfo {
	var durationMs int64
	if s.CompletedAt.IsZero() && !s.StartedAt.IsZero() {
		durationMs = time.Since(s.StartedAt).Milliseconds()
	} else if !s.CompletedAt.IsZero() && !s.StartedAt.IsZero() {
		durationMs = s.CompletedAt.Sub(s.StartedAt).Milliseconds()
	}

	info := ui.ExecutionInfo{
		WorkflowID:  s.RunID,
		Status:      string(s.Status),
		CurrentStep: s.CurrentStep,
		DurationMs:  durationMs,
	}
	if !s.StartedAt.IsZero() {
		info.StartedAt = s.StartedAt.Format(time.RFC3339)
	}
	if !s.CompletedAt.IsZero() {
		info.CompletedAt = s.CompletedAt.Format(time.RFC3339)
	}
	if len(s.Steps) > 0 {
		steps := make([]ui.StepInfo, 0, len(s.Steps))
		for i := range s.Steps {
			step := ui.StepInfo{
				Name:   s.Steps[i].Name,
				Status: string(s.Steps[i].Status),
				Error:  s.Steps[i].Error,
			}
			if !s.Steps[i].StartedAt.IsZero() {
				step.StartedAt = s.Steps[i].StartedAt.Format(time.RFC3339)
			}
			if !s.Steps[i].CompletedAt.IsZero() {
				step.CompletedAt = s.Steps[i].CompletedAt.Format(time.RFC3339)
			}
			steps = append(steps, step)
		}
		info.Steps = steps
	}
	return info
}

// displayRunStatus renders a ports.RunStatus in human-readable text format.
// Displays CurrentStep, Progress (X/Y steps, N failed), and in verbose mode
// the full step list (name, colored status, duration, error) plus Inputs.
// Sections are only shown when the relevant data is present in the DTO.
func displayRunStatus(formatter *ui.Formatter, s *ports.RunStatus, verbose bool) {
	color := formatter.Colorizer()

	formatter.Printf("ID:       %s\n", s.RunID)
	formatter.StatusLine("Status", string(s.Status), "")

	var duration time.Duration
	if s.CompletedAt.IsZero() && !s.StartedAt.IsZero() {
		duration = time.Since(s.StartedAt)
	} else if !s.CompletedAt.IsZero() && !s.StartedAt.IsZero() {
		duration = s.CompletedAt.Sub(s.StartedAt)
	}
	if duration > 0 {
		formatter.Printf("Duration: %s\n", duration.Round(time.Millisecond))
	}

	// Current step (running/paused)
	if s.CurrentStep != "" {
		formatter.Printf("Current:  %s\n", s.CurrentStep)
	}

	// Progress summary derived from the enriched DTO (populated from event stream).
	if s.Progress.Total > 0 {
		formatter.Printf("Progress: %d/%d steps", s.Progress.Completed, s.Progress.Total)
		if s.Progress.Failed > 0 {
			formatter.Printf(" (%d failed)", s.Progress.Failed)
		}
		formatter.Println()
	}

	// Verbose: per-step table
	if verbose && len(s.Steps) > 0 {
		formatter.Println()
		formatter.Println(color.Bold("Steps:"))
		for i := range s.Steps {
			st := &s.Steps[i]
			var stepDuration time.Duration
			if !st.StartedAt.IsZero() && !st.CompletedAt.IsZero() {
				stepDuration = st.CompletedAt.Sub(st.StartedAt).Round(time.Millisecond)
			}
			statusStr := color.Status(string(st.Status), string(st.Status))
			formatter.Printf("  %-20s %s (%s)\n", st.Name, statusStr, stepDuration)
			if st.Error != "" {
				formatter.Printf("    Error: %s\n", color.Error(st.Error))
			}
		}
	}

	// Verbose: inputs (populated only on the live-session path via setInputs)
	if verbose && len(s.Inputs) > 0 {
		formatter.Println()
		formatter.Println(color.Bold("Inputs:"))
		for k, v := range s.Inputs {
			formatter.Printf("  %s: %v\n", k, v)
		}
	}
}
