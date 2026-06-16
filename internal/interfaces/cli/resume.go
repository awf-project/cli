package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/audit"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	"github.com/awf-project/cli/internal/infrastructure/roles"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func newResumeCommand(cfg *Config) *cobra.Command {
	var listFlag bool
	var inputFlags []string
	var outputMode string

	cmd := &cobra.Command{
		Use:   "resume [workflow-id]",
		Short: "Resume an interrupted workflow",
		Long: `Resume a workflow execution from where it left off.

The workflow-id is the unique identifier shown when running a workflow.
Use --list to see all resumable workflows.
Input overrides can be provided to change input values on resume.

Examples:
  awf resume --list
  awf resume abc123-def456
  awf resume abc123-def456 --input max_tokens=5000
  awf resume abc123-def456 --from current
  awf resume abc123-def456 --from previous
  awf resume abc123-def456 --from validate`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if listFlag {
				return runResumeList(cmd, cfg)
			}
			if len(args) == 0 {
				return fmt.Errorf("workflow-id required (use --list to see resumable workflows)")
			}
			if outputMode != "" {
				mode, err := ParseOutputMode(outputMode)
				if err != nil {
					return err
				}
				cfg.OutputMode = mode
			}
			return runResume(cmd, cfg, args[0], inputFlags)
		},
	}

	cmd.Flags().BoolVarP(&listFlag, "list", "l", false, "List resumable workflows")
	cmd.Flags().StringArrayVarP(&inputFlags, "input", "i", nil, "Override input parameter (key=value)")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "silent", "Output mode: silent, streaming, buffered")
	cmd.Flags().String("from", "current", "Step to resume from: current (default), previous, or <step-name>")

	return cmd
}

func runResumeList(cmd *cobra.Command, cfg *Config) error {
	ctx := context.Background()
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)

	// Route through WorkflowFacade when wired (T069). resume-list has no dedicated
	// facade method (ports.WorkflowFacade is List/Validate/Status/History/Run/Resume),
	// so it is served by History + a resumable filter: any record whose status is not
	// "completed" is resumable.
	if cfg.Facade != nil {
		return runResumeListViaFacade(cmd, cfg, writer, ctx)
	}

	// Facade not wired: return a meaningful error stub. Production always wires
	// the facade via NewRootCommandAutoFacade, so this branch is never reached in
	// normal usage.
	err := fmt.Errorf("resume --list requires facade wiring (use NewRootCommandAutoFacade)")
	return writeErrorAndExit(writer, err, ExitSystem)
}

// runResumeListViaFacade serves resume-list through ports.WorkflowFacade.History
// (T069). The facade exposes no ResumeList method, so resumable runs are derived
// by filtering History records to any run whose status is not "completed".
func runResumeListViaFacade(cmd *cobra.Command, cfg *Config, writer *ui.OutputWriter, ctx context.Context) error { //nolint:revive // context.Context not first param: writer is a pre-built dependency, not a new chain
	records, err := cfg.Facade.History(ctx, ports.HistoryFilter{})
	if err != nil {
		return writeErrorAndExit(writer, fmt.Errorf("list history: %w", err), ExitSystem)
	}

	infos := make([]ui.ResumableInfo, 0, len(records))
	for i := range records {
		rec := &records[i]
		if rec.Status == ports.RunStateCompleted {
			continue
		}
		infos = append(infos, ui.ResumableInfo{
			WorkflowID:   rec.RunID,
			WorkflowName: rec.WorkflowName,
			Status:       string(rec.Status),
			UpdatedAt:    rec.CompletedAt.Format(time.RFC3339),
		})
	}

	if cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatTable || cfg.OutputFormat == ui.FormatQuiet {
		return writer.WriteResumableList(infos)
	}

	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		NoColor: cfg.NoColor,
	})

	if len(infos) == 0 {
		formatter.Info("No resumable workflows found")
		return nil
	}

	formatter.Printf("Resumable workflows:\n\n")
	for _, info := range infos {
		formatter.Printf("  %s\n", info.WorkflowID)
		formatter.Printf("    Workflow: %s\n", info.WorkflowName)
		formatter.Printf("    Status:   %s\n", info.Status)
		formatter.Printf("    Current:  %s\n", info.CurrentStep)
		formatter.Printf("    Progress: %s\n", info.Progress)
		formatter.Printf("    Updated:  %s\n\n", info.UpdatedAt)
	}

	return nil
}

func runResume(cmd *cobra.Command, cfg *Config, workflowID string, inputFlags []string) error {
	// Parse input overrides (validated here so a bad key=value fails before any I/O).
	inputOverrides, err := parseInputFlags(inputFlags)
	if err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}

	// Read --from flag; default is "current" (empty string means "resume from last persisted step").
	fromStep, err := cmd.Flags().GetString("from")
	if err != nil {
		fromStep = ""
	}
	// Normalize "current" (the flag default) to an empty string so the facade
	// receives the canonical "no override" value and can apply its own default.
	if fromStep == "current" {
		fromStep = ""
	}

	// Create output components
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	// Resume requires a RUN-CAPABLE facade (T069). The read-only AutoFacade in
	// cfg.Facade carries a zero ExecutionService, so it cannot re-drive a run. Build
	// the real execution stack here (mirroring the `run` path) and overwrite cfg.Facade
	// with a run-capable Adapter, then dispatch strictly through cfg.Facade.Resume — no
	// direct ExecutionService/ResumeService call remains in this file.
	runFacade, cleanup, err := buildResumeFacade(cmd, cfg, formatter)
	if err != nil {
		return writeErrorAndExit(writer, err, categorizeError(err))
	}
	defer cleanup()
	cfg.Facade = runFacade

	return runResumeViaFacade(cmd, cfg, writer, formatter, workflowID, inputOverrides, fromStep)
}

// buildResumeFacade constructs an execution-capable ports.WorkflowFacade for the
// `resume` command (T069). It loads project config (so a malformed .awf/config.yaml
// surfaces as a "config" error before any execution), builds the real ExecutionService
// via the canonical application.ExecutionSetup builder, and wraps it in the same
// run-capable Adapter the `run` path uses (buildRunCapableFacade). The returned cleanup
// releases the execution stack's resources (history store, etc.).
func buildResumeFacade(cmd *cobra.Command, cfg *Config, formatter *ui.Formatter) (facade ports.WorkflowFacade, cleanup func(), err error) {
	ctx := context.Background()

	repo := NewWorkflowRepository()
	stateStore := store.NewJSONStore(cfg.StoragePath + "/states")
	shellExecutor := executor.NewShellExecutor()
	silentOutput := cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatTable
	logger := &cliLogger{
		formatter: formatter,
		verbose:   cfg.Verbose && !silentOutput,
		silent:    silentOutput,
	}

	// Load project config from .awf/config.yaml. A malformed config is a USER error
	// (FR-005): surface it before building the execution stack.
	if _, cfgErr := loadProjectConfig(logger); cfgErr != nil {
		return nil, nil, fmt.Errorf("config error: %w", cfgErr)
	}

	// History store: lifecycle is owned by the ExecutionSetup builder via WithHistoryStore,
	// which closes it during SetupResult.Cleanup.
	historyStore, hsErr := store.NewSQLiteHistoryStore(filepath.Join(cfg.StoragePath, "history.db"))
	if hsErr != nil {
		return nil, nil, fmt.Errorf("failed to open history store: %w", hsErr)
	}

	setupOpts := []application.SetupOption{
		application.WithHistoryStore(historyStore),
		application.WithAgentRoleRepository(roles.NewFilesystemAgentRoleRepository(logger)),
		application.WithUserInputReader(ui.NewStdinInputReader(os.Stdin, os.Stdout)),
	}

	// Setup audit trail writer (F071), best-effort.
	var auditCleanup func()
	if auditWriter, ac, auditErr := audit.NewWriterFromEnv(); auditErr != nil {
		logger.Warn("failed to initialize audit writer, audit trail disabled", "error", auditErr)
	} else if auditWriter != nil {
		auditCleanup = ac
		setupOpts = append(setupOpts, application.WithAuditWriter(auditWriter))
	}

	// Setup transcript recorder (F106) so resumed-run transcript events stream into the
	// session, matching the `run` path. Best-effort: a nil recorder falls back to a
	// NopRecorder inside buildRunCapableFacade.
	runID := uuid.New().String()
	var recorder ports.Recorder
	var transcriptCleanup func() error
	if rec, rc, recErr := WireTranscript(runID, cfg.StoragePath); recErr != nil {
		logger.Warn("failed to initialize transcript recorder, transcripts disabled", "error", recErr)
	} else {
		recorder = rec
		transcriptCleanup = rc
		setupOpts = append(
			setupOpts,
			application.WithRecorder(recorder),
			application.WithRecorderFactory(NewRecorderFactory()),
			application.WithTranscriptDir(filepath.Join(cfg.StoragePath, "transcripts")),
		)
	}

	setupResult, buildErr := application.NewExecutionSetup(repo, stateStore, shellExecutor, logger, setupOpts...).Build(ctx)
	if buildErr != nil {
		if auditCleanup != nil {
			auditCleanup()
		}
		if transcriptCleanup != nil {
			_ = transcriptCleanup() //nolint:errcheck // best-effort transcript flush on cleanup
		}
		return nil, nil, fmt.Errorf("failed to initialize execution: %w", buildErr)
	}

	runFacade := buildRunCapableFacade(setupResult.ExecService, setupResult.WorkflowSvc, historyStore, repo, recorder, logger)
	if runFacade == nil {
		setupResult.Cleanup()
		if auditCleanup != nil {
			auditCleanup()
		}
		if transcriptCleanup != nil {
			_ = transcriptCleanup() //nolint:errcheck // best-effort transcript flush on cleanup
		}
		return nil, nil, fmt.Errorf("resume: failed to construct run-capable facade")
	}

	cleanup = func() {
		setupResult.Cleanup()
		if auditCleanup != nil {
			auditCleanup()
		}
		if transcriptCleanup != nil {
			_ = transcriptCleanup() //nolint:errcheck // best-effort transcript flush on cleanup
		}
	}
	return runFacade, cleanup, nil
}

// runResumeViaFacade resumes a run through ports.WorkflowFacade.Resume (T069).
// The facade owns the resume lifecycle and returns a RunSession; this consumes the
// session's event stream to completion, then maps the terminal outcome to an exit
// code via categorizeError, mirroring the legacy path's result handling.
//
// inputOverrides and fromStep come from the --input and --from CLI flags respectively.
// A nil inputOverrides map means "no overrides; use the values stored with the original run".
// An empty fromStep means "resume from the current/last persisted step" (the facade default).
func runResumeViaFacade(cmd *cobra.Command, cfg *Config, writer *ui.OutputWriter, formatter *ui.Formatter, workflowID string, inputOverrides map[string]any, fromStep string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	silentOutput := cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatTable
	if !silentOutput && cfg.OutputFormat != ui.FormatQuiet {
		formatter.Info(fmt.Sprintf("Resuming workflow: %s", workflowID))
	}
	startTime := time.Now()

	session, err := cfg.Facade.Resume(ctx, ports.ResumeRequest{
		RunID:          workflowID,
		InputOverrides: inputOverrides,
		FromStep:       fromStep,
	})
	if err != nil {
		return writeErrorAndExit(writer, err, categorizeError(err))
	}
	defer func() { _ = session.Close() }()

	// Drain the session's event stream until the terminal event seals the channel and
	// return session.Err(). application.Drain is the single shared consumer helper
	// (FR-015); this interface layer MUST NOT reimplement the loop directly.
	execErr := application.Drain(session)
	durationMs := time.Since(startTime).Milliseconds()

	if cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatQuiet || cfg.OutputFormat == ui.FormatTable {
		result := ui.RunResult{
			WorkflowID: session.ID(),
			Status:     "completed",
			DurationMs: durationMs,
		}
		if execErr != nil {
			result.Status = "failed"
			result.Error = execErr.Error()
		}
		if writeErr := writer.WriteRunResult(&result); writeErr != nil {
			return writeErr
		}
		if execErr != nil {
			return &exitError{code: categorizeError(execErr), err: execErr}
		}
		return nil
	}

	if execErr != nil {
		formatter.Info(fmt.Sprintf("Workflow ID: %s", session.ID()))
		return writeErrorAndExit(writer, execErr, categorizeError(execErr))
	}

	duration := time.Since(startTime).Round(time.Millisecond)
	formatter.Success(fmt.Sprintf("Workflow resumed and completed in %s", duration))
	formatter.Info(fmt.Sprintf("Workflow ID: %s", session.ID()))
	return nil
}
