package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
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
	displayRunStatus(formatter, &status)
	return nil
}

// runStatusToExecutionInfo converts a ports.RunStatus to ui.ExecutionInfo for structured output.
func runStatusToExecutionInfo(s *ports.RunStatus) ui.ExecutionInfo {
	var durationMs int64
	if s.CompletedAt.IsZero() && !s.StartedAt.IsZero() {
		durationMs = time.Since(s.StartedAt).Milliseconds()
	} else if !s.CompletedAt.IsZero() && !s.StartedAt.IsZero() {
		durationMs = s.CompletedAt.Sub(s.StartedAt).Milliseconds()
	}

	info := ui.ExecutionInfo{
		WorkflowID: s.RunID,
		Status:     s.Status,
		DurationMs: durationMs,
	}
	if !s.StartedAt.IsZero() {
		info.StartedAt = s.StartedAt.Format(time.RFC3339)
	}
	if !s.CompletedAt.IsZero() {
		info.CompletedAt = s.CompletedAt.Format(time.RFC3339)
	}
	return info
}

// displayRunStatus renders a ports.RunStatus in human-readable text format.
func displayRunStatus(formatter *ui.Formatter, s *ports.RunStatus) {
	color := formatter.Colorizer()

	formatter.Printf("ID:       %s\n", s.RunID)
	formatter.StatusLine("Status", s.Status, "")

	var duration time.Duration
	if s.CompletedAt.IsZero() && !s.StartedAt.IsZero() {
		duration = time.Since(s.StartedAt)
	} else if !s.CompletedAt.IsZero() && !s.StartedAt.IsZero() {
		duration = s.CompletedAt.Sub(s.StartedAt)
	}
	if duration > 0 {
		formatter.Printf("Duration: %s\n", duration.Round(time.Millisecond))
	}
	_ = color // colorizer available for future field coloring
}

func toExecutionInfo(execCtx *workflow.ExecutionContext) ui.ExecutionInfo {
	var durationMs int64
	if execCtx.CompletedAt.IsZero() {
		durationMs = time.Since(execCtx.StartedAt).Milliseconds()
	} else {
		durationMs = execCtx.CompletedAt.Sub(execCtx.StartedAt).Milliseconds()
	}

	steps := make([]ui.StepInfo, 0, len(execCtx.States))
	for name, state := range execCtx.States {
		step := ui.StepInfo{
			Name:     name,
			Status:   string(state.Status),
			Output:   state.Output,
			Stderr:   state.Stderr,
			ExitCode: state.ExitCode,
			Error:    state.Error,
		}
		if !state.StartedAt.IsZero() {
			step.StartedAt = state.StartedAt.Format(time.RFC3339)
		}
		if !state.CompletedAt.IsZero() {
			step.CompletedAt = state.CompletedAt.Format(time.RFC3339)
		}
		steps = append(steps, step)
	}

	info := ui.ExecutionInfo{
		WorkflowID:   execCtx.WorkflowID,
		WorkflowName: execCtx.WorkflowName,
		Status:       string(execCtx.Status),
		CurrentStep:  execCtx.CurrentStep,
		DurationMs:   durationMs,
		Steps:        steps,
	}
	if !execCtx.StartedAt.IsZero() {
		info.StartedAt = execCtx.StartedAt.Format(time.RFC3339)
	}
	if !execCtx.CompletedAt.IsZero() {
		info.CompletedAt = execCtx.CompletedAt.Format(time.RFC3339)
	}
	return info
}

func displayStatus(formatter *ui.Formatter, execCtx *workflow.ExecutionContext, verbose bool) {
	color := formatter.Colorizer()

	// Header
	formatter.Printf("Workflow: %s\n", color.Bold(execCtx.WorkflowName))
	formatter.Printf("ID:       %s\n", execCtx.WorkflowID)
	formatter.StatusLine("Status", string(execCtx.Status), "")

	// Duration
	var duration time.Duration
	if execCtx.CompletedAt.IsZero() {
		duration = time.Since(execCtx.StartedAt)
	} else {
		duration = execCtx.CompletedAt.Sub(execCtx.StartedAt)
	}
	formatter.Printf("Duration: %s\n", duration.Round(time.Millisecond))

	// Current step
	if execCtx.CurrentStep != "" {
		formatter.Printf("Current:  %s\n", execCtx.CurrentStep)
	}

	// Progress
	completed := 0
	failed := 0
	total := len(execCtx.States)
	for _, state := range execCtx.States {
		switch state.Status {
		case workflow.StatusCompleted:
			completed++
		case workflow.StatusFailed:
			failed++
		case workflow.StatusPending, workflow.StatusRunning, workflow.StatusCancelled:
			// Not counted in completed or failed
		}
	}

	if total > 0 {
		formatter.Printf("Progress: %d/%d steps", completed, total)
		if failed > 0 {
			formatter.Printf(" (%d failed)", failed)
		}
		formatter.Println()
	}

	// Verbose: show all steps
	if verbose && len(execCtx.States) > 0 {
		formatter.Println()
		formatter.Println(color.Bold("Steps:"))

		for name, state := range execCtx.States {
			stepDuration := state.CompletedAt.Sub(state.StartedAt).Round(time.Millisecond)
			statusStr := color.Status(string(state.Status), string(state.Status))
			formatter.Printf("  %-20s %s (%s)\n", name, statusStr, stepDuration)

			if state.Error != "" {
				formatter.Printf("    Error: %s\n", color.Error(state.Error))
			}
		}
	}

	// Inputs (verbose)
	if verbose && len(execCtx.Inputs) > 0 {
		formatter.Println()
		formatter.Println(color.Bold("Inputs:"))
		for k, v := range execCtx.Inputs {
			formatter.Printf("  %s: %v\n", k, v)
		}
	}
}
