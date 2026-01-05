package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/agents"
	"github.com/vanoix/awf/internal/infrastructure/executor"
	"github.com/vanoix/awf/internal/infrastructure/store"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
	"github.com/vanoix/awf/pkg/expression"
	"github.com/vanoix/awf/pkg/interpolation"
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
  awf resume abc123-def456 --input max_tokens=5000`,
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

	return cmd
}

func runResumeList(cmd *cobra.Command, cfg *Config) error {
	ctx := context.Background()
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor)

	stateStore := store.NewJSONStore(cfg.StoragePath + "/states")

	// List all state IDs and filter resumable
	ids, err := stateStore.List(ctx)
	if err != nil {
		return fmt.Errorf("list states: %w", err)
	}

	var infos []ui.ResumableInfo
	for _, id := range ids {
		execCtx, err := stateStore.Load(ctx, id)
		if err != nil || execCtx == nil {
			continue
		}
		if execCtx.Status == workflow.StatusCompleted {
			continue
		}

		// Calculate progress
		completed := 0
		for _, state := range execCtx.States {
			if state.Status == workflow.StatusCompleted {
				completed++
			}
		}
		progress := fmt.Sprintf("%d steps completed", completed)

		infos = append(infos, ui.ResumableInfo{
			WorkflowID:   execCtx.WorkflowID,
			WorkflowName: execCtx.WorkflowName,
			Status:       string(execCtx.Status),
			CurrentStep:  execCtx.CurrentStep,
			UpdatedAt:    execCtx.UpdatedAt.Format(time.RFC3339),
			Progress:     progress,
		})
	}

	// Output based on format
	if cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatTable || cfg.OutputFormat == ui.FormatQuiet {
		return writer.WriteResumableList(infos)
	}

	// Text format
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
	// Parse input overrides
	cliInputs, err := parseInputFlags(inputFlags)
	if err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}

	// Create output components
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor)
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	// Setup streaming writers if needed
	var stdoutWriter, stderrWriter *ui.PrefixedWriter
	if !cfg.Quiet && cfg.OutputMode == OutputStreaming && !writer.IsJSONFormat() {
		colorizer := ui.NewColorizer(!cfg.NoColor)
		stdoutWriter = ui.NewPrefixedWriter(cmd.OutOrStdout(), ui.PrefixStdout, "", colorizer)
		stderrWriter = ui.NewPrefixedWriter(cmd.ErrOrStderr(), ui.PrefixStderr, "", colorizer)
	}

	// Context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		if cfg.OutputFormat != ui.FormatJSON && cfg.OutputFormat != ui.FormatTable {
			formatter.Warning("\nReceived interrupt signal, cancelling...")
		}
		cancel()
	}()

	// Initialize dependencies
	repo := NewWorkflowRepository()
	stateStore := store.NewJSONStore(cfg.StoragePath + "/states")
	shellExecutor := executor.NewShellExecutor()
	silentOutput := cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatTable
	logger := &cliLogger{
		formatter: formatter,
		verbose:   cfg.Verbose && !silentOutput,
		silent:    silentOutput,
	}
	resolver := interpolation.NewTemplateResolver()

	// Load project config from .awf/config.yaml
	projectCfg, err := loadProjectConfig(logger)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	// Merge config inputs with CLI inputs (CLI wins)
	inputs := mergeInputs(projectCfg.Inputs, cliInputs)

	// Create history store and service
	historyStore, err := store.NewSQLiteHistoryStore(filepath.Join(cfg.StoragePath, "history.db"))
	if err != nil {
		return fmt.Errorf("failed to open history store: %w", err)
	}
	defer func() {
		if err := historyStore.Close(); err != nil {
			logger.Error("failed to close history store", "error", err)
		}
	}()
	historySvc := application.NewHistoryService(historyStore, logger)

	// Create services
	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, logger)
	parallelExecutor := application.NewParallelExecutor(logger)
	exprEvaluator := expression.NewExprEvaluator()
	execSvc := application.NewExecutionServiceWithEvaluator(wfSvc, shellExecutor, parallelExecutor, stateStore, logger, resolver, historySvc, exprEvaluator)

	// Setup agent registry for F039 agent step execution
	agentRegistry := agents.NewAgentRegistry()
	if err := agentRegistry.RegisterDefaults(); err != nil {
		return fmt.Errorf("failed to register agent providers: %w", err)
	}
	execSvc.SetAgentRegistry(agentRegistry)

	if stdoutWriter != nil {
		execSvc.SetOutputWriters(stdoutWriter, stderrWriter)
	}

	// Show start message
	if !silentOutput && cfg.OutputFormat != ui.FormatQuiet {
		formatter.Info(fmt.Sprintf("Resuming workflow: %s", workflowID))
	}
	startTime := time.Now()

	// Resume execution
	execCtx, execErr := execSvc.Resume(ctx, workflowID, inputs)

	// Flush streaming writers
	if stdoutWriter != nil {
		stdoutWriter.Flush()
	}
	if stderrWriter != nil {
		stderrWriter.Flush()
	}

	// Calculate duration
	durationMs := time.Since(startTime).Milliseconds()

	// Output result (same pattern as runWorkflow)
	if cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatQuiet {
		result := ui.RunResult{
			Status:     "completed",
			DurationMs: durationMs,
		}
		if execCtx != nil {
			result.WorkflowID = execCtx.WorkflowID
			result.Status = string(execCtx.Status)
			if cfg.OutputMode == OutputBuffered {
				result.Steps = buildStepInfos(execCtx)
			}
		}
		if execErr != nil {
			result.Status = "failed"
			result.Error = execErr.Error()
		}
		if err := writer.WriteRunResult(result); err != nil {
			return err
		}
		if execErr != nil {
			return &exitError{code: categorizeError(execErr), err: execErr}
		}
		return nil
	}

	// Table format
	if cfg.OutputFormat == ui.FormatTable {
		result := ui.RunResult{
			Status:     "completed",
			DurationMs: durationMs,
		}
		if execCtx != nil {
			result.WorkflowID = execCtx.WorkflowID
			result.Status = string(execCtx.Status)
			result.Steps = buildStepInfos(execCtx)
		}
		if execErr != nil {
			result.Status = "failed"
			result.Error = execErr.Error()
		}
		if err := writer.WriteRunResult(result); err != nil {
			return err
		}
		if execErr != nil {
			return &exitError{code: categorizeError(execErr), err: execErr}
		}
		return nil
	}

	// Text format
	duration := time.Since(startTime).Round(time.Millisecond)

	if execErr != nil {
		formatter.Error(fmt.Sprintf("Resume failed: %s", execErr))
		if execCtx != nil {
			formatter.Info(fmt.Sprintf("Workflow ID: %s", execCtx.WorkflowID))
		}
		if cfg.OutputMode == OutputBuffered && execCtx != nil {
			showStepOutputs(formatter, execCtx)
		}
		return &exitError{code: categorizeError(execErr), err: execErr}
	}

	if cfg.OutputMode != OutputBuffered && execCtx != nil {
		showEmptyStepFeedback(formatter, execCtx)
	}

	formatter.Success(fmt.Sprintf("Workflow resumed and completed in %s", duration))
	formatter.Info(fmt.Sprintf("Workflow ID: %s", execCtx.WorkflowID))

	if cfg.OutputMode == OutputBuffered && execCtx != nil {
		showStepOutputs(formatter, execCtx)
	}

	if cfg.Verbose && execCtx != nil {
		showExecutionDetails(formatter, execCtx)
	}

	return nil
}
