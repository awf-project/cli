package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/awf-project/cli/internal/application"
	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/agents"
	"github.com/awf-project/cli/internal/infrastructure/audit"
	"github.com/awf-project/cli/internal/infrastructure/config"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infraexpression "github.com/awf-project/cli/internal/infrastructure/expression"
	"github.com/awf-project/cli/internal/infrastructure/github"
	"github.com/awf-project/cli/internal/infrastructure/http"
	"github.com/awf-project/cli/internal/infrastructure/notify"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/infrastructure/xdg"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/awf-project/cli/pkg/httpx"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newRunCommand(cfg *Config) *cobra.Command {
	var inputFlags []string
	var outputMode string
	var stepFlag string
	var mockFlags []string
	var dryRunFlag bool
	var interactiveFlag bool
	var breakpointFlags []string
	var skipPlugins bool

	cmd := &cobra.Command{
		Use:   "run <workflow>",
		Short: "Execute a workflow",
		Long: `Execute a workflow by name with optional input parameters.

Input parameters are passed as key=value pairs via the --input flag.
Output modes control how command output is displayed:
  - silent (default): No streaming, only final result
  - streaming: Real-time output with [OUT]/[ERR] prefixes
  - buffered: Show output after each step completes

Single step execution:
  Use --step to execute only a specific step from the workflow.
  Use --mock to inject state values for step dependencies.

Interactive mode:
  Use --interactive for step-by-step execution with prompts.
  Use --breakpoint to pause only at specific steps.

Examples:
  awf run analyze-code --input file=main.go
  awf run build-project --input target=linux --output=streaming
  awf run my-workflow --step=process --input data=test
  awf run my-workflow --step=analyze --mock states.fetch.Output="cached data"
  awf run my-workflow --dry-run
  awf run my-workflow --interactive
  awf run my-workflow --interactive --breakpoint validate,process`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputMode != "" {
				mode, err := ParseOutputMode(outputMode)
				if err != nil {
					return err
				}
				cfg.OutputMode = mode
			}
			if dryRunFlag {
				return runDryRun(cmd, cfg, args[0], inputFlags, skipPlugins)
			}
			if interactiveFlag {
				return runInteractive(cmd, cfg, args[0], inputFlags, breakpointFlags, skipPlugins)
			}
			if stepFlag != "" {
				return runSingleStep(cmd, cfg, args[0], stepFlag, inputFlags, mockFlags, skipPlugins)
			}
			return runWorkflow(cmd, cfg, args[0], inputFlags, skipPlugins)
		},
	}

	cmd.Flags().StringArrayVarP(&inputFlags, "input", "i", nil, "Input parameter (key=value)")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "silent",
		"Output mode: silent (default), streaming, buffered")
	cmd.Flags().StringVarP(&stepFlag, "step", "s", "", "Execute only this step (skips state machine)")
	cmd.Flags().StringArrayVarP(&mockFlags, "mock", "m", nil,
		"Mock state value for single step execution (states.step.Output=value)")
	cmd.Flags().BoolVar(&dryRunFlag, "dry-run", false,
		"Show execution plan without running commands")
	cmd.Flags().BoolVar(&interactiveFlag, "interactive", false,
		"Execute in interactive step-by-step mode with prompts")
	cmd.Flags().StringArrayVarP(&breakpointFlags, "breakpoint", "b", nil,
		"Pause only at specified steps in interactive mode (comma-separated)")
	cmd.Flags().BoolVar(&skipPlugins, "skip-plugins", false, "Skip plugin validators")

	// Wire custom help function for workflow-specific help (F035)
	cmd.SetHelpFunc(workflowHelpFunc(cfg))

	return cmd
}

// workflowHelpFunc returns a custom help function that displays workflow-specific help
// when a workflow argument is provided, or falls back to default Cobra help otherwise.
// This enables `awf run <workflow> --help` to show dynamic workflow input parameters.
func workflowHelpFunc(cfg *Config) func(*cobra.Command, []string) {
	// Default help rendering fallback
	defaultHelp := func(cmd *cobra.Command) {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), cmd.Long)
		_, _ = fmt.Fprintln(cmd.OutOrStdout())
		if err := cmd.UsageFunc()(cmd); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error displaying usage: %v\n", err)
		}
	}

	return func(cmd *cobra.Command, _ []string) {
		// Get positional arguments (workflow name) from parsed flags
		positionalArgs := cmd.Flags().Args()

		// If no workflow argument, show default help
		if len(positionalArgs) == 0 {
			defaultHelp(cmd)
			return
		}

		workflowName := positionalArgs[0]

		// Load workflow from repository (supports pack/workflow namespace)
		packName, baseName := parseWorkflowNamespace(workflowName)
		var repo *repository.CompositeRepository
		if packName != "" {
			packDir := findPackDir(packName)
			if packDir != "" {
				workflowsDir := filepath.Join(packDir, "workflows")
				repo = repository.NewCompositeRepository([]repository.SourcedPath{
					{Path: workflowsDir, Source: repository.SourceLocal},
				})
			}
		}
		if repo == nil {
			repo = NewWorkflowRepository()
			baseName = workflowName
		}
		wf, err := repo.Load(context.Background(), baseName)
		if err != nil {
			// Error loading workflow - show error and default help
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error: workflow '%s' not found: %v\n\n", workflowName, err)
			defaultHelp(cmd)
			return
		}

		// Render workflow-specific help
		noColor := cfg != nil && cfg.NoColor
		if err := RenderWorkflowHelp(cmd, wf, cmd.OutOrStdout(), noColor); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error rendering help: %v\n", err)
			return
		}

		// Also show standard command flags
		_, _ = fmt.Fprintln(cmd.OutOrStdout())
		if err := cmd.UsageFunc()(cmd); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error displaying usage: %v\n", err)
		}
	}
}

func runWorkflow(cmd *cobra.Command, cfg *Config, workflowName string, inputFlags []string, skipPlugins bool) error {
	// Parse inputs
	inputs, err := parseInputFlags(inputFlags)
	if err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}

	// Create output writer for JSON/quiet formats
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)

	// Create formatter for text output
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	// Setup output writers based on mode (only for non-JSON formats)
	var stdoutWriter, stderrWriter *ui.PrefixedWriter
	if !cfg.Quiet && cfg.OutputMode == OutputStreaming && !writer.IsJSONFormat() {
		colorizer := ui.NewColorizer(!cfg.NoColor)
		stdoutWriter = ui.NewPrefixedWriter(cmd.OutOrStdout(), ui.PrefixStdout, "", colorizer)
		stderrWriter = ui.NewPrefixedWriter(cmd.ErrOrStderr(), ui.PrefixStderr, "", colorizer)
	}

	// Setup context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cleanup := setupSignalHandler(ctx, cancel, func() {
		if cfg.OutputFormat != ui.FormatJSON && cfg.OutputFormat != ui.FormatTable {
			formatter.Warning("\nReceived interrupt signal, cancelling...")
		}
	})
	defer cleanup()

	// Initialize dependencies
	repo := NewWorkflowRepository()
	stateStore := store.NewJSONStore(cfg.StoragePath + "/states")
	shellExecutor := executor.NewShellExecutor()
	// Table and JSON formats should be silent (structured output only)
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
	inputs = mergeInputs(projectCfg.Inputs, inputs)

	// Parse namespace to check if this is a pack workflow
	packName, workflowBase := parseWorkflowNamespace(workflowName)
	var wf *workflow.Workflow

	// Load workflow once for input collection check and later execution
	if packName != "" {
		// Pack workflow: resolve from installed pack
		wf, _, err = resolvePackWorkflow(ctx, packName, workflowBase, xdg.LocalWorkflowPacksDir(), xdg.AWFWorkflowPacksDir())
	} else {
		// Local workflow: use regular repository
		wf, err = repo.Load(ctx, workflowName)
	}
	if err != nil {
		return writeErrorAndExit(writer, err, categorizeError(err))
	}

	// Collect missing inputs interactively if needed (F046)
	inputs, err = collectMissingInputsIfNeeded(ctx, cmd, wf, inputs, cfg, logger)
	if err != nil {
		return writeErrorAndExit(writer, err, categorizeError(err))
	}

	// Create history store and service
	historyStore, err := store.NewSQLiteHistoryStore(filepath.Join(cfg.StoragePath, "history.db"))
	if err != nil {
		return fmt.Errorf("failed to open history store: %w", err)
	}
	defer func() {
		if closeErr := historyStore.Close(); closeErr != nil {
			logger.Error("failed to close history store", "error", closeErr)
		}
	}()
	historySvc := application.NewHistoryService(historyStore, logger)

	// Initialize plugin system (skip if --skip-plugins flag is set)
	var pluginResult *PluginSystemResult
	if !skipPlugins {
		var initErr error
		pluginResult, initErr = initPluginSystem(ctx, cfg, logger)
		if initErr != nil {
			return fmt.Errorf("failed to initialize plugins: %w", initErr)
		}
		defer pluginResult.Cleanup()
	} else {
		// Create a stub plugin service when skipping plugins
		pluginResult = &PluginSystemResult{
			Service: application.NewPluginService(nil, nil, logger),
			Manager: nil,
			Cleanup: func() {},
		}
	}

	// Create services
	exprValidator := infraexpression.NewExprValidator()
	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, logger, exprValidator)
	parallelExecutor := application.NewParallelExecutor(logger)
	exprEvaluator := infraexpression.NewExprEvaluator()
	execSvc := application.NewExecutionServiceWithEvaluator(wfSvc, shellExecutor, parallelExecutor, stateStore, logger, resolver, historySvc, exprEvaluator)

	// Setup agent registry for F039 agent step execution
	agentRegistry := agents.NewAgentRegistry()
	if err = agentRegistry.RegisterDefaults(); err != nil {
		return fmt.Errorf("failed to register agent providers: %w", err)
	}
	execSvc.SetAgentRegistry(agentRegistry)

	// Set AWF paths with pack context if applicable
	if packName != "" {
		execSvc.SetAWFPaths(buildPackAWFPaths(packName))
	} else {
		execSvc.SetAWFPaths(buildAWFPaths())
	}

	// Setup PackWorkflowLoader for C072 pack workflow resolution in subworkflow calls
	execSvc.SetPackWorkflowLoader(func(ctx context.Context, targetPackName, targetWorkflow string) (*workflow.Workflow, string, error) {
		return resolvePackWorkflow(ctx, targetPackName, targetWorkflow, xdg.LocalWorkflowPacksDir(), xdg.AWFWorkflowPacksDir())
	})

	// Setup audit trail writer (F071)
	if auditWriter, auditCleanup, auditErr := buildAuditWriter(logger); auditErr != nil {
		logger.Warn("failed to initialize audit writer, audit trail disabled", "error", auditErr)
	} else {
		defer auditCleanup()
		if auditWriter != nil {
			execSvc.SetAuditTrailWriter(auditWriter)
		}
	}

	// Setup operation providers gated by plugin enable/disable state
	compositeProvider, err := buildBuiltinProviders(pluginResult.Service, projectCfg, logger, pluginResult.Manager)
	if err != nil {
		return fmt.Errorf("failed to build operation providers: %w", err)
	}
	execSvc.SetOperationProvider(compositeProvider)
	execSvc.SetPluginService(pluginResult.Service)
	if pluginResult.RPCManager != nil {
		wfSvc.SetValidatorProvider(pluginResult.RPCManager.ValidatorProvider(0))
		execSvc.SetStepTypeProvider(pluginResult.RPCManager.StepTypeProvider(logger))
	}

	// Setup template service for workflow template expansion
	templatePaths := []string{
		".awf/templates",
		filepath.Join(cfg.StoragePath, "templates"),
	}
	templateRepo := repository.NewYAMLTemplateRepository(templatePaths)
	templateSvc := application.NewTemplateService(templateRepo, logger)
	execSvc.SetTemplateService(templateSvc)

	// Pass writers to execution service for streaming mode
	if stdoutWriter != nil {
		execSvc.SetOutputWriters(stdoutWriter, stderrWriter)
	}

	// Show start message (text format only)
	if !silentOutput && cfg.OutputFormat != ui.FormatQuiet {
		formatter.Info(fmt.Sprintf("Running workflow: %s", workflowName))
	}
	startTime := time.Now()

	// Execute workflow with pre-loaded workflow (avoids double I/O)
	execCtx, execErr := execSvc.RunWithWorkflow(ctx, wf, inputs)

	// Flush any remaining output from streaming writers
	if stdoutWriter != nil {
		stdoutWriter.Flush()
	}
	if stderrWriter != nil {
		stderrWriter.Flush()
	}

	// Calculate duration
	durationMs := time.Since(startTime).Milliseconds()

	// JSON/quiet format: output result
	if cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatQuiet {
		result := ui.RunResult{
			Status:     "completed",
			DurationMs: durationMs,
		}
		if execCtx != nil {
			result.WorkflowID = execCtx.WorkflowID
			result.Status = string(execCtx.Status)
			// Include step outputs in buffered mode
			if cfg.OutputMode == OutputBuffered {
				result.Steps = buildStepInfos(execCtx)
			}
		}
		if execErr != nil {
			result.Status = "failed"
			result.Error = execErr.Error()
		}
		if err := writer.WriteRunResult(&result); err != nil {
			return err
		}
		if execErr != nil {
			return &exitError{code: categorizeError(execErr), err: execErr}
		}
		return nil
	}

	// Table format: use structured output
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
		if err := writer.WriteRunResult(&result); err != nil {
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
		// Show buffered output on error
		if cfg.OutputMode == OutputBuffered && execCtx != nil {
			showStepOutputs(formatter, execCtx)
		}
		if execCtx != nil {
			formatter.Info(fmt.Sprintf("Workflow ID: %s", execCtx.WorkflowID))
		}
		return writeErrorAndExit(writer, execErr, categorizeError(execErr))
	}

	// F037: Show success feedback for steps with no output (silent/streaming modes)
	if cfg.OutputMode != OutputBuffered && execCtx != nil {
		showEmptyStepFeedback(formatter, execCtx)
	}

	formatter.Success(fmt.Sprintf("Workflow completed successfully in %s", duration))
	formatter.Info(fmt.Sprintf("Workflow ID: %s", execCtx.WorkflowID))

	// Show buffered output after successful completion
	if cfg.OutputMode == OutputBuffered && execCtx != nil {
		showStepOutputs(formatter, execCtx)
	}

	if cfg.Verbose && execCtx != nil {
		showExecutionDetails(formatter, execCtx)
	}

	return nil
}

// runDryRun executes a dry-run of the workflow, showing the execution plan without running commands.
func runDryRun(cmd *cobra.Command, cfg *Config, workflowName string, inputFlags []string, skipPlugins bool) error {
	// Parse inputs
	inputs, err := parseInputFlags(inputFlags)
	if err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}

	// Create output writer for JSON/quiet formats
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)

	// Create formatter for text output
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	// Initialize dependencies
	repo := NewWorkflowRepository()
	stateStore := store.NewJSONStore(cfg.StoragePath + "/states")
	shellExecutor := executor.NewShellExecutor()
	logger := &cliLogger{
		formatter: formatter,
		verbose:   cfg.Verbose,
		silent:    cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatTable,
	}
	resolver := interpolation.NewTemplateResolver()
	exprEvaluator := infraexpression.NewExprEvaluator()

	// Load project config from .awf/config.yaml
	projectCfg, err := loadProjectConfig(logger)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	// Merge config inputs with CLI inputs (CLI wins)
	inputs = mergeInputs(projectCfg.Inputs, inputs)

	// Create services
	exprValidator := infraexpression.NewExprValidator()
	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, logger, exprValidator)
	dryRunExec := application.NewDryRunExecutor(wfSvc, resolver, exprEvaluator, logger)
	dryRunExec.SetAWFPaths(buildAWFPaths())

	// Setup template service for workflow template expansion
	templatePaths := []string{
		".awf/templates",
		filepath.Join(cfg.StoragePath, "templates"),
	}
	templateRepo := repository.NewYAMLTemplateRepository(templatePaths)
	templateSvc := application.NewTemplateService(templateRepo, logger)
	dryRunExec.SetTemplateService(templateSvc)

	// Execute dry-run
	plan, err := dryRunExec.Execute(cmd.Context(), workflowName, inputs)
	if err != nil {
		return writeErrorAndExit(writer, fmt.Errorf("dry-run failed: %w", err), categorizeError(err))
	}

	// Output the plan
	dryRunFormatter := ui.NewDryRunFormatter(cmd.OutOrStdout(), !cfg.NoColor)
	return writer.WriteDryRun(plan, dryRunFormatter)
}

// runInteractive executes the workflow in interactive step-by-step mode.
func runInteractive(cmd *cobra.Command, cfg *Config, workflowName string, inputFlags, breakpointFlags []string, skipPlugins bool) error {
	// Parse inputs
	inputs, err := parseInputFlags(inputFlags)
	if err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}

	// Parse breakpoints (flatten comma-separated values)
	var breakpoints []string
	for _, bp := range breakpointFlags {
		for _, b := range strings.Split(bp, ",") {
			b = strings.TrimSpace(b)
			if b != "" {
				breakpoints = append(breakpoints, b)
			}
		}
	}

	// Create formatter for text output
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	// Setup context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cleanup := setupSignalHandler(ctx, cancel, nil)
	defer cleanup()

	// Initialize dependencies
	repo := NewWorkflowRepository()
	stateStore := store.NewJSONStore(cfg.StoragePath + "/states")
	shellExecutor := executor.NewShellExecutor()
	logger := &cliLogger{
		formatter: formatter,
		verbose:   cfg.Verbose,
		silent:    false,
	}
	resolver := interpolation.NewTemplateResolver()
	exprEvaluator := infraexpression.NewExprEvaluator()

	// Load project config from .awf/config.yaml
	projectCfg, err := loadProjectConfig(logger)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	// Merge config inputs with CLI inputs (CLI wins)
	inputs = mergeInputs(projectCfg.Inputs, inputs)

	// Create services
	exprValidator := infraexpression.NewExprValidator()
	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, logger, exprValidator)
	parallelExecutor := application.NewParallelExecutor(logger)

	// Note: skipPlugins flag is accepted for consistency with other modes but
	// not used in interactive mode as InteractiveExecutor doesn't use plugin providers

	// Create interactive prompt
	prompt := ui.NewCLIPrompt(cmd.InOrStdin(), cmd.OutOrStdout(), !cfg.NoColor)

	// Create interactive executor
	interactiveExec := application.NewInteractiveExecutor(
		wfSvc, shellExecutor, parallelExecutor, stateStore,
		logger, resolver, exprEvaluator, prompt,
	)

	// Setup template service
	templatePaths := []string{
		".awf/templates",
		filepath.Join(cfg.StoragePath, "templates"),
	}
	templateRepo := repository.NewYAMLTemplateRepository(templatePaths)
	templateSvc := application.NewTemplateService(templateRepo, logger)
	interactiveExec.SetTemplateService(templateSvc)
	interactiveExec.SetAWFPaths(buildAWFPaths())

	// Set breakpoints if specified
	if len(breakpoints) > 0 {
		interactiveExec.SetBreakpoints(breakpoints)
	}

	// Execute interactive workflow
	_, execErr := interactiveExec.Run(ctx, workflowName, inputs)
	if execErr != nil {
		// Context cancellation is handled gracefully in interactive mode
		if errors.Is(execErr, context.Canceled) {
			return nil // Not an error for interactive abort
		}
		// Create writer for error routing
		writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), ui.FormatText, cfg.NoColor, cfg.NoHints)
		return writeErrorAndExit(writer, execErr, categorizeError(execErr))
	}

	return nil
}

func parseInputFlags(flags []string) (map[string]any, error) {
	inputs := make(map[string]any)

	for _, flag := range flags {
		parts := strings.SplitN(flag, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid input format '%s', expected key=value", flag)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, errors.New("input key cannot be empty")
		}

		// Resolve @prompts/ prefix to file content
		resolved, err := resolvePromptInput(value)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve prompt for '%s': %w", key, err)
		}
		inputs[key] = resolved
	}

	return inputs, nil
}

// promptPrefix is the prefix for referencing prompt files.
const promptPrefix = "@prompts/"

// resolvePromptInput resolves a value that may reference a prompt file.
// If the value starts with @prompts/, the file content is loaded by searching
// multiple paths in priority order (local first, then global).
// Otherwise, the value is returned as-is.
func resolvePromptInput(value string) (string, error) {
	if !strings.HasPrefix(value, promptPrefix) {
		return value, nil
	}

	// Extract relative path after @prompts/
	relativePath := strings.TrimPrefix(value, promptPrefix)

	// Security: block path traversal and absolute paths
	if strings.Contains(relativePath, "..") || filepath.IsAbs(relativePath) || strings.HasPrefix(relativePath, "/") {
		return "", fmt.Errorf("invalid prompt path: path traversal not allowed")
	}

	// Search multiple paths in priority order
	return resolvePromptFromPaths(relativePath, BuildPromptPaths())
}

// resolvePromptFromPaths searches for a prompt file across multiple paths in priority order.
// Returns the content of the first found prompt file.
func resolvePromptFromPaths(relativePath string, paths []repository.SourcedPath) (string, error) {
	// Preallocate for all search paths
	searchedPaths := make([]string, 0, len(paths))

	for _, sp := range paths {
		fullPath := filepath.Join(sp.Path, relativePath)
		searchedPaths = append(searchedPaths, fullPath)

		// Check if file exists and is a regular file (not a directory)
		info, err := os.Stat(fullPath)
		if err != nil {
			// File doesn't exist in this path, continue to next
			continue
		}
		if info.IsDir() {
			// Path exists but is a directory, not a file - this is an error
			return "", fmt.Errorf("prompt path is a directory, not a file: %s", fullPath)
		}

		// File found - read and return content
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return "", fmt.Errorf("failed to read prompt file %s: %w", fullPath, err)
		}

		return strings.TrimSpace(string(content)), nil
	}

	// File not found in any path
	if len(searchedPaths) == 0 {
		return "", fmt.Errorf("prompt '%s' not found: no search paths configured", relativePath)
	}
	return "", fmt.Errorf("prompt '%s' not found in any of the search paths: %s",
		relativePath, strings.Join(searchedPaths, ", "))
}

func showExecutionDetails(formatter *ui.Formatter, execCtx *workflow.ExecutionContext) {
	formatter.Printf("\n--- Execution Details ---\n")
	formatter.Printf("Status: %s\n", execCtx.Status)
	formatter.Printf("Steps executed:\n")

	allStates := execCtx.GetAllStepStates()
	for name, state := range allStates {
		duration := state.CompletedAt.Sub(state.StartedAt).Round(time.Millisecond)
		formatter.StatusLine("  "+name, string(state.Status), fmt.Sprintf("(%s)", duration))
	}
}

func showStepOutputs(formatter *ui.Formatter, execCtx *workflow.ExecutionContext) {
	allStates := execCtx.GetAllStepStates()
	for name, state := range allStates {
		if state.Output != "" {
			formatter.Printf("\n--- [%s] stdout ---\n", name)
			formatter.Printf("%s", state.Output)
		}
		if state.Stderr != "" {
			formatter.Printf("\n--- [%s] stderr ---\n", name)
			formatter.Printf("%s", state.Stderr)
		}
		// F037: Success feedback for steps with no output
		if state.Output == "" && state.Stderr == "" &&
			state.Status == workflow.StatusCompleted {
			formatter.StepSuccess(name)
		}
	}
}

// showEmptyStepFeedback displays success message for steps that had no output.
// Used for silent/streaming modes where showStepOutputs is not called.
func showEmptyStepFeedback(formatter *ui.Formatter, execCtx *workflow.ExecutionContext) {
	for name, state := range execCtx.States {
		if state.Output == "" && state.Stderr == "" &&
			state.Status == workflow.StatusCompleted {
			formatter.StepSuccess(name)
		}
	}
}

func buildStepInfos(execCtx *workflow.ExecutionContext) []ui.StepInfo {
	// Preallocate for all steps
	steps := make([]ui.StepInfo, 0, len(execCtx.States))
	for name, state := range execCtx.States {
		steps = append(steps, ui.StepInfo{
			Name:        name,
			Status:      string(state.Status),
			Output:      state.Output,
			Stderr:      state.Stderr,
			ExitCode:    state.ExitCode,
			StartedAt:   state.StartedAt.Format(time.RFC3339),
			CompletedAt: state.CompletedAt.Format(time.RFC3339),
			Error:       state.Error,
		})
	}
	return steps
}

// categorizeError maps errors to exit codes using a two-phase approach:
//  1. Check for StructuredError via errors.As() and use its ExitCode()
//  2. Fall back to legacy string matching for unconverted errors
//
// This enables incremental migration from string-based to structured errors
// while preserving backward compatibility (ADR-003).
//
// Exit code mapping:
//   - 1 (ExitUser): User input errors
//   - 2 (ExitWorkflow): Workflow definition errors
//   - 3 (ExitExecution): Runtime execution errors (default)
//   - 4 (ExitSystem): System/infrastructure errors
func categorizeError(err error) int {
	if err == nil {
		return ExitExecution
	}

	// Phase 1 - Check for StructuredError first
	var structuredErr *domerrors.StructuredError
	if errors.As(err, &structuredErr) {
		return structuredErr.ExitCode()
	}

	// Phase 2 - Fall back to legacy string matching
	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "not found"):
		return ExitUser
	case strings.Contains(errStr, "invalid"):
		return ExitWorkflow
	case strings.Contains(errStr, "timeout"):
		return ExitExecution
	case strings.Contains(errStr, "exit code"):
		return ExitExecution
	case strings.Contains(errStr, "permission"):
		return ExitSystem
	default:
		return ExitExecution
	}
}

// exitError wraps an error with an exit code.
type exitError struct {
	code    int
	err     error
	handled bool // error message was already written to output
}

func (e *exitError) Error() string {
	return e.err.Error()
}

func (e *exitError) ExitCode() int {
	return e.code
}

func (e *exitError) Handled() bool {
	return e.handled
}

// writeErrorAndExit writes the error through WriteError and returns an exitError
// marked as handled so main.go won't double-print.
func writeErrorAndExit(writer *ui.OutputWriter, err error, exitCode int) error {
	if writeErr := writer.WriteError(err, exitCode); writeErr != nil {
		return writeErr
	}
	return &exitError{code: exitCode, err: err, handled: true}
}

// cliLogger implements ports.Logger for CLI output.
type cliLogger struct {
	formatter *ui.Formatter
	verbose   bool
	silent    bool // Suppress all output (for JSON format)
	context   map[string]any
}

func (l *cliLogger) Debug(msg string, keysAndValues ...any) {
	if l.silent {
		return
	}
	if l.verbose {
		l.formatter.Debug(formatLog(msg, l.mergeContext(keysAndValues)...))
	}
}

func (l *cliLogger) Info(msg string, keysAndValues ...any) {
	if l.silent {
		return
	}
	l.formatter.Info(formatLog(msg, l.mergeContext(keysAndValues)...))
}

func (l *cliLogger) Warn(msg string, keysAndValues ...any) {
	if l.silent {
		return
	}
	l.formatter.Warning(formatLog(msg, l.mergeContext(keysAndValues)...))
}

func (l *cliLogger) Error(msg string, keysAndValues ...any) {
	if l.silent {
		return
	}
	l.formatter.Error(formatLog(msg, l.mergeContext(keysAndValues)...))
}

func (l *cliLogger) WithContext(ctx map[string]any) ports.Logger {
	merged := make(map[string]any)
	maps.Copy(merged, l.context)
	maps.Copy(merged, ctx)
	return &cliLogger{
		formatter: l.formatter,
		verbose:   l.verbose,
		silent:    l.silent,
		context:   merged,
	}
}

func (l *cliLogger) mergeContext(keysAndValues []any) []any {
	if len(l.context) == 0 {
		return keysAndValues
	}
	result := make([]any, 0, len(keysAndValues)+len(l.context)*2)
	for k, v := range l.context {
		result = append(result, k, v)
	}
	result = append(result, keysAndValues...)
	return result
}

func formatLog(msg string, keysAndValues ...any) string {
	if len(keysAndValues) == 0 {
		return msg
	}

	parts := []string{msg}
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			parts = append(parts, fmt.Sprintf("%v=%v", keysAndValues[i], keysAndValues[i+1]))
		}
	}
	return strings.Join(parts, " ")
}

// runSingleStep executes a single step from a workflow.
func runSingleStep(
	cmd *cobra.Command,
	cfg *Config,
	workflowName string,
	stepName string,
	inputFlags []string,
	mockFlags []string,
	skipPlugins bool,
) error {
	// Parse inputs
	inputs, err := parseInputFlags(inputFlags)
	if err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}

	// Parse mocks
	mocks, err := ParseMockFlags(mockFlags)
	if err != nil {
		return fmt.Errorf("invalid mock: %w", err)
	}

	// Create formatter for text output
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	// Setup context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cleanup := setupSignalHandler(ctx, cancel, func() {
		formatter.Warning("\nReceived interrupt signal, cancelling...")
	})
	defer cleanup()

	// Initialize dependencies
	repo := NewWorkflowRepository()
	stateStore := store.NewJSONStore(cfg.StoragePath + "/states")
	shellExecutor := executor.NewShellExecutor()
	logger := &cliLogger{
		formatter: formatter,
		verbose:   cfg.Verbose,
		silent:    cfg.Quiet,
	}
	resolver := interpolation.NewTemplateResolver()

	// Load project config from .awf/config.yaml
	// TODO(T027): Use projectCfg.Notify to register backends dynamically
	projectCfg, err := loadProjectConfig(logger)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	// Create history store and service
	historyStore, err := store.NewSQLiteHistoryStore(filepath.Join(cfg.StoragePath, "history.db"))
	if err != nil {
		return fmt.Errorf("failed to open history store: %w", err)
	}
	defer func() {
		if closeErr := historyStore.Close(); closeErr != nil {
			logger.Error("failed to close history store", "error", closeErr)
		}
	}()
	historySvc := application.NewHistoryService(historyStore, logger)

	// Initialize plugin system (skip if --skip-plugins flag is set)
	var pluginResult *PluginSystemResult
	if !skipPlugins {
		var initErr error
		pluginResult, initErr = initPluginSystem(ctx, cfg, logger)
		if initErr != nil {
			return fmt.Errorf("failed to initialize plugins: %w", initErr)
		}
		defer pluginResult.Cleanup()
	} else {
		// Create a stub plugin service when skipping plugins
		pluginResult = &PluginSystemResult{
			Service: application.NewPluginService(nil, nil, logger),
			Manager: nil,
			Cleanup: func() {},
		}
	}

	// Create services
	exprValidator := infraexpression.NewExprValidator()
	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, logger, exprValidator)
	parallelExecutor := application.NewParallelExecutor(logger)
	exprEvaluator := infraexpression.NewExprEvaluator()
	execSvc := application.NewExecutionServiceWithEvaluator(wfSvc, shellExecutor, parallelExecutor, stateStore, logger, resolver, historySvc, exprEvaluator)

	// Setup agent registry for F039 agent step execution
	agentRegistry := agents.NewAgentRegistry()
	if err = agentRegistry.RegisterDefaults(); err != nil {
		return fmt.Errorf("failed to register agent providers: %w", err)
	}
	execSvc.SetAgentRegistry(agentRegistry)

	// Parse namespace to set up pack context if applicable
	packName, _ := parseWorkflowNamespace(workflowName)
	if packName != "" {
		execSvc.SetAWFPaths(buildPackAWFPaths(packName))
	} else {
		execSvc.SetAWFPaths(buildAWFPaths())
	}

	// Setup PackWorkflowLoader for C072 pack workflow resolution in subworkflow calls
	execSvc.SetPackWorkflowLoader(func(ctx context.Context, targetPackName, targetWorkflow string) (*workflow.Workflow, string, error) {
		return resolvePackWorkflow(ctx, targetPackName, targetWorkflow, xdg.LocalWorkflowPacksDir(), xdg.AWFWorkflowPacksDir())
	})

	// Setup operation providers gated by plugin enable/disable state
	compositeProvider, err := buildBuiltinProviders(pluginResult.Service, projectCfg, logger, pluginResult.Manager)
	if err != nil {
		return fmt.Errorf("failed to build operation providers: %w", err)
	}
	execSvc.SetOperationProvider(compositeProvider)
	execSvc.SetPluginService(pluginResult.Service)
	if pluginResult.RPCManager != nil {
		execSvc.SetStepTypeProvider(pluginResult.RPCManager.StepTypeProvider(logger))
	}

	// Setup template service for workflow template expansion
	templatePaths := []string{
		".awf/templates",
		filepath.Join(cfg.StoragePath, "templates"),
	}
	templateRepo := repository.NewYAMLTemplateRepository(templatePaths)
	templateSvc := application.NewTemplateService(templateRepo, logger)
	execSvc.SetTemplateService(templateSvc)

	// Show start message
	if !cfg.Quiet {
		formatter.Info(fmt.Sprintf("Running single step: %s from workflow: %s", stepName, workflowName))
	}
	startTime := time.Now()

	// Execute single step
	result, execErr := execSvc.ExecuteSingleStep(ctx, workflowName, stepName, inputs, mocks)

	// Calculate duration
	duration := time.Since(startTime).Round(time.Millisecond)

	if execErr != nil {
		// Create writer for error routing
		writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), ui.FormatText, cfg.NoColor, cfg.NoHints)
		return writeErrorAndExit(writer, execErr, categorizeError(execErr))
	}

	// Display result
	if result.Output != "" && !cfg.Quiet {
		formatter.Printf("\n--- [%s] stdout ---\n", result.StepName)
		formatter.Printf("%s", result.Output)
	}
	if result.Stderr != "" && !cfg.Quiet {
		formatter.Printf("\n--- [%s] stderr ---\n", result.StepName)
		formatter.Printf("%s", result.Stderr)
	}

	if result.Status == workflow.StatusCompleted {
		formatter.Success(fmt.Sprintf("Step '%s' completed successfully in %s", stepName, duration))
	} else {
		formatter.Error(fmt.Sprintf("Step '%s' failed (exit code: %d)", stepName, result.ExitCode))
	}

	return nil
}

// ParseMockFlags parses --mock flags into a map.
// Format: states.step_name.Output=value
func ParseMockFlags(flags []string) (map[string]string, error) {
	mocks := make(map[string]string)

	for _, flag := range flags {
		parts := strings.SplitN(flag, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid mock format '%s', expected key=value", flag)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, errors.New("mock key cannot be empty")
		}
		mocks[key] = value
	}

	return mocks, nil
}

// loadProjectConfig loads the project configuration from .awf/config.yaml.
// Returns empty config if the file doesn't exist.
func loadProjectConfig(_ ports.Logger) (*config.ProjectConfig, error) {
	configPath := xdg.LocalConfigPath()
	return config.NewYAMLConfigLoader(configPath).Load()
}

// hasMissingRequiredInputs reports whether any required workflow input is absent from inputs.
func hasMissingRequiredInputs(wf *workflow.Workflow, inputs map[string]any) bool {
	if wf == nil {
		return false
	}
	for _, input := range wf.Inputs {
		if input.Required {
			if _, exists := inputs[input.Name]; !exists {
				return true
			}
		}
	}
	return false
}

// mergeInputs returns configInputs merged with cliInputs. CLI wins on conflict.
func mergeInputs(configInputs, cliInputs map[string]any) map[string]any {
	result := make(map[string]any)
	maps.Copy(result, configInputs)
	maps.Copy(result, cliInputs)
	return result
}

// collectMissingInputsIfNeeded checks if required inputs are missing and
// prompts the user interactively if stdin is a terminal.
//
// Returns:
//   - Updated inputs map with collected values
//   - Error if stdin is not a terminal and inputs are missing
func collectMissingInputsIfNeeded(
	ctx context.Context,
	cmd *cobra.Command,
	wf *workflow.Workflow,
	inputs map[string]any,
	cfg *Config,
	logger ports.Logger,
) (map[string]any, error) {
	// Check if any required inputs are missing
	if !hasMissingRequiredInputs(wf, inputs) {
		return inputs, nil
	}

	// Check if stdin is a terminal
	if !isTerminal(cmd.InOrStdin()) {
		// Build lists for MissingInputHintGenerator
		var missingInputs, requiredInputs []string
		for _, input := range wf.Inputs {
			if input.Required {
				requiredInputs = append(requiredInputs, input.Name)
				if _, exists := inputs[input.Name]; !exists {
					missingInputs = append(missingInputs, input.Name)
				}
			}
		}
		return nil, domerrors.NewUserError(
			domerrors.ErrorCodeUserInputValidationFailed,
			"missing required inputs and stdin is not a terminal; provide inputs via --input flags",
			map[string]any{
				"missing_inputs":  missingInputs,
				"required_inputs": requiredInputs,
			},
			nil,
		)
	}

	// Create collector and service
	colorizer := ui.NewColorizer(!cfg.NoColor)
	collector := ui.NewCLIInputCollector(cmd.InOrStdin(), cmd.OutOrStdout(), colorizer)
	service := application.NewInputCollectionService(collector, logger)

	// Collect missing inputs
	return service.CollectMissingInputs(ctx, wf, inputs)
}

// isTerminal checks if the given reader is connected to a terminal.
func isTerminal(r io.Reader) bool {
	if f, ok := r.(*os.File); ok {
		return term.IsTerminal(int(f.Fd())) //nolint:gosec // G115: file descriptor values are within int range on all supported platforms
	}
	return false
}

// registerNotifyBackends registers notification backends from config (T027 stub).
//
// This function reads backend configuration from .awf/config.yaml and registers
// the appropriate backends with the NotifyOperationProvider. It supports:
//   - Desktop notifications (always enabled)
//   - Webhook (always enabled, URL provided per-operation)
//
// If default_backend is configured, it sets the default backend for the provider.
//
// Parameters:
//   - provider: notification provider to register backends with
//   - cfg: project configuration containing backend settings
//   - logger: structured logger for backend registration tracing
//
// Returns:
//   - error: non-nil if backend registration fails
func registerNotifyBackends(provider *notify.NotifyOperationProvider, cfg *config.ProjectConfig, logger ports.Logger) error {
	// Validate inputs
	if provider == nil {
		return fmt.Errorf("notify provider cannot be nil")
	}
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// 1. Register desktop backend (always enabled)
	desktopBackend := notify.NewDesktopBackend()
	if err := provider.RegisterBackend("desktop", desktopBackend); err != nil {
		return fmt.Errorf("failed to register desktop backend: %w", err)
	}
	logger.Debug("registered desktop notification backend")

	// 2. Register webhook backend (always enabled)
	webhookBackend := notify.NewWebhookBackend()
	if err := provider.RegisterBackend("webhook", webhookBackend); err != nil {
		return fmt.Errorf("failed to register webhook backend: %w", err)
	}
	logger.Debug("registered webhook notification backend")

	// 3. Set default backend if cfg.Notify.DefaultBackend is set
	if strings.TrimSpace(cfg.Notify.DefaultBackend) != "" {
		provider.SetDefaultBackend(cfg.Notify.DefaultBackend)
		logger.Debug("set default notification backend", "backend", cfg.Notify.DefaultBackend)
	}

	return nil
}

// buildAWFPaths returns the AWF XDG directory paths for template interpolation (F063).
func buildAWFPaths() map[string]string {
	return map[string]string{
		"prompts_dir":   xdg.AWFPromptsDir(),
		"scripts_dir":   xdg.AWFScriptsDir(),
		"config_dir":    xdg.AWFConfigDir(),
		"data_dir":      xdg.AWFDataDir(),
		"workflows_dir": xdg.AWFWorkflowsDir(),
		"plugins_dir":   xdg.AWFPluginsDir(),
	}
}

// buildAuditWriter creates a FileAuditTrailWriter based on the AWF_AUDIT_LOG env var.
// Returns (nil, noop, nil) when AWF_AUDIT_LOG=off.
// The caller must defer the returned cleanup func.
func buildAuditWriter(logger ports.Logger) (ports.AuditTrailWriter, func(), error) {
	auditLog := os.Getenv("AWF_AUDIT_LOG")
	if auditLog == "off" {
		return nil, func() {}, nil
	}

	auditPath := auditLog
	if auditPath == "" {
		auditPath = filepath.Join(xdg.AWFDataDir(), "audit.jsonl")
	}

	w, err := audit.NewFileAuditTrailWriter(auditPath)
	if err != nil {
		return nil, func() {}, err
	}

	cleanup := func() {
		if closeErr := w.Close(); closeErr != nil {
			logger.Error("failed to close audit writer", "error", closeErr)
		}
	}
	return w, cleanup, nil
}

// buildBuiltinProviders constructs a CompositeOperationProvider from the three built-in providers,
// gating each behind an IsPluginEnabled() check so disabled providers are excluded from execution.
// If manager is non-nil (external plugins available), it is appended last so plugin operations
// are dispatched after built-in providers.
func buildBuiltinProviders(pluginSvc *application.PluginService, projectCfg *config.ProjectConfig, logger ports.Logger, manager ports.OperationProvider) (*pluginmgr.CompositeOperationProvider, error) {
	// pluginSvc nil check is a defensive guard — both call-sites pass non-nil service,
	// but we fall back to enabling all providers if called without a service.
	var providers []ports.OperationProvider

	if pluginSvc == nil || pluginSvc.IsPluginEnabled("github") {
		githubClient := github.NewClient(logger)
		providers = append(providers, github.NewGitHubOperationProvider(githubClient, logger))
	}

	if pluginSvc == nil || pluginSvc.IsPluginEnabled("notify") {
		notifyProvider := notify.NewNotifyOperationProvider(logger)
		cfg := projectCfg
		if cfg == nil {
			cfg = &config.ProjectConfig{}
		}
		if err := registerNotifyBackends(notifyProvider, cfg, logger); err != nil {
			return nil, fmt.Errorf("failed to register notify backends: %w", err)
		}
		providers = append(providers, notifyProvider)
	}

	if pluginSvc == nil || pluginSvc.IsPluginEnabled("http") {
		httpClient := httpx.NewClient()
		providers = append(providers, http.NewHTTPOperationProvider(httpClient, logger))
	}

	if manager != nil {
		providers = append(providers, manager)
	}

	return pluginmgr.NewCompositeOperationProvider(providers...), nil
}
