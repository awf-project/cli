package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/agents"
	"github.com/vanoix/awf/internal/infrastructure/config"
	"github.com/vanoix/awf/internal/infrastructure/executor"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/internal/infrastructure/store"
	"github.com/vanoix/awf/internal/infrastructure/xdg"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
	"github.com/vanoix/awf/pkg/expression"
	"github.com/vanoix/awf/pkg/interpolation"
	"golang.org/x/term"
)

// setupSignalHandler starts a goroutine that cancels ctx on SIGINT/SIGTERM.
// If onSignal is not nil, it's called when a signal is received before cancelling.
// Returns a cleanup function that MUST be deferred to prevent goroutine leaks.
func setupSignalHandler(ctx context.Context, cancel context.CancelFunc, onSignal func()) func() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigChan:
			if onSignal != nil {
				onSignal()
			}
			cancel()
		case <-ctx.Done():
			// Context cancelled externally, exit cleanly
		}
	}()
	return func() { signal.Stop(sigChan) }
}

func newRunCommand(cfg *Config) *cobra.Command {
	var inputFlags []string
	var outputMode string
	var stepFlag string
	var mockFlags []string
	var dryRunFlag bool
	var interactiveFlag bool
	var breakpointFlags []string

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
  awf run my-workflow --step=analyze --mock states.fetch.output="cached data"
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
				return runDryRun(cmd, cfg, args[0], inputFlags)
			}
			if interactiveFlag {
				return runInteractive(cmd, cfg, args[0], inputFlags, breakpointFlags)
			}
			if stepFlag != "" {
				return runSingleStep(cmd, cfg, args[0], stepFlag, inputFlags, mockFlags)
			}
			return runWorkflow(cmd, cfg, args[0], inputFlags)
		},
	}

	cmd.Flags().StringArrayVarP(&inputFlags, "input", "i", nil, "Input parameter (key=value)")
	cmd.Flags().StringVarP(&outputMode, "output", "o", "silent",
		"Output mode: silent (default), streaming, buffered")
	cmd.Flags().StringVarP(&stepFlag, "step", "s", "", "Execute only this step (skips state machine)")
	cmd.Flags().StringArrayVarP(&mockFlags, "mock", "m", nil,
		"Mock state value for single step execution (states.step.output=value)")
	cmd.Flags().BoolVar(&dryRunFlag, "dry-run", false,
		"Show execution plan without running commands")
	cmd.Flags().BoolVar(&interactiveFlag, "interactive", false,
		"Execute in interactive step-by-step mode with prompts")
	cmd.Flags().StringArrayVarP(&breakpointFlags, "breakpoint", "b", nil,
		"Pause only at specified steps in interactive mode (comma-separated)")

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

		// Load workflow from repository
		repo := NewWorkflowRepository()
		wf, err := repo.Load(context.Background(), workflowName)
		if err != nil {
			// Error loading workflow - show error and default help
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error: workflow '%s' not found: %v\n\n", workflowName, err)
			defaultHelp(cmd)
			return
		}
		if wf == nil {
			// Workflow not found (repository returns nil, nil for non-existent workflows)
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error: workflow '%s' not found\n\n", workflowName)
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

func runWorkflow(cmd *cobra.Command, cfg *Config, workflowName string, inputFlags []string) error {
	// Parse inputs
	inputs, err := parseInputFlags(inputFlags)
	if err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}

	// Create output writer for JSON/quiet formats
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor)

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

	// Load workflow once for input collection check and later execution
	wf, err := repo.Load(ctx, workflowName)
	if err != nil {
		return fmt.Errorf("workflow not found: %w", err)
	}

	// Collect missing inputs interactively if needed (F046)
	inputs, err = collectMissingInputsIfNeeded(cmd, wf, inputs, cfg, logger)
	if err != nil {
		return fmt.Errorf("input collection failed: %w", err)
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

	// Initialize plugin system
	pluginResult, err := initPluginSystem(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize plugins: %w", err)
	}
	defer pluginResult.Cleanup()
	_ = pluginResult.Service // Available for future integration with execution service

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
		formatter.Error(fmt.Sprintf("Workflow failed: %s", execErr))
		if execCtx != nil {
			formatter.Info(fmt.Sprintf("Workflow ID: %s", execCtx.WorkflowID))
		}
		// Show buffered output on error
		if cfg.OutputMode == OutputBuffered && execCtx != nil {
			showStepOutputs(formatter, execCtx)
		}
		return &exitError{code: categorizeError(execErr), err: execErr}
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
func runDryRun(cmd *cobra.Command, cfg *Config, workflowName string, inputFlags []string) error {
	// Parse inputs
	inputs, err := parseInputFlags(inputFlags)
	if err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}

	// Create output writer for JSON/quiet formats
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor)

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
	exprEvaluator := expression.NewExprEvaluator()

	// Create services
	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, logger)
	dryRunExec := application.NewDryRunExecutor(wfSvc, resolver, exprEvaluator, logger)

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
		return fmt.Errorf("dry-run failed: %w", err)
	}

	// Output the plan
	dryRunFormatter := ui.NewDryRunFormatter(cmd.OutOrStdout(), !cfg.NoColor)
	return writer.WriteDryRun(plan, dryRunFormatter)
}

// runInteractive executes the workflow in interactive step-by-step mode.
func runInteractive(cmd *cobra.Command, cfg *Config, workflowName string, inputFlags, breakpointFlags []string) error {
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
	exprEvaluator := expression.NewExprEvaluator()

	// Create services
	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, logger)
	parallelExecutor := application.NewParallelExecutor(logger)

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
		return &exitError{code: categorizeError(execErr), err: execErr}
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

	for name := range execCtx.States {
		state := execCtx.States[name]
		duration := state.CompletedAt.Sub(state.StartedAt).Round(time.Millisecond)
		formatter.StatusLine("  "+name, string(state.Status), fmt.Sprintf("(%s)", duration))
	}
}

func showStepOutputs(formatter *ui.Formatter, execCtx *workflow.ExecutionContext) {
	for name := range execCtx.States {
		state := execCtx.States[name]
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

func categorizeError(err error) int {
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
	code int
	err  error
}

func (e *exitError) Error() string {
	return e.err.Error()
}

func (e *exitError) ExitCode() int {
	return e.code
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
	for k, v := range l.context {
		merged[k] = v
	}
	for k, v := range ctx {
		merged[k] = v
	}
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

	// Initialize plugin system
	pluginResult, err := initPluginSystem(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize plugins: %w", err)
	}
	defer pluginResult.Cleanup()
	_ = pluginResult.Service // Available for future integration with execution service

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
		formatter.Error(fmt.Sprintf("Step execution failed: %s", execErr))
		return &exitError{code: categorizeError(execErr), err: execErr}
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
// Format: states.step_name.output=value
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
// Returns empty config if file doesn't exist (not an error).
// Returns error for invalid YAML or file read errors.
//
// The logger is used to emit warnings for unknown config keys.
func loadProjectConfig(logger ports.Logger) (*config.ProjectConfig, error) {
	_ = logger // Reserved for future warning logging (unknown keys)

	configPath := xdg.LocalConfigPath()
	loader := config.NewYAMLConfigLoader(configPath)

	return loader.Load()
}

// hasMissingRequiredInputs checks if workflow has any required inputs that are not
// present in the provided inputs map.
//
// This helper is used to determine if interactive input collection is needed.
//
// Parameters:
//   - wf: Workflow definition
//   - inputs: Input values already provided
//
// Returns:
//   - true if any required input is missing
//   - false if all required inputs are present
func hasMissingRequiredInputs(wf *workflow.Workflow, inputs map[string]any) bool {
	// Handle nil workflow or inputs
	if wf == nil || wf.Inputs == nil {
		return false
	}

	// Check each required input
	for _, input := range wf.Inputs {
		if input.Required {
			if _, exists := inputs[input.Name]; !exists {
				return true
			}
		}
	}

	return false
}

// mergeInputs merges config file inputs with CLI flag inputs.
// CLI inputs take precedence over config inputs (CLI always wins).
// Returns a new map containing all merged inputs.
//
// Merge priority (highest wins):
//
//	CLI flags (--input key=value) > Config file (.awf/config.yaml)
func mergeInputs(configInputs, cliInputs map[string]any) map[string]any {
	result := make(map[string]any)

	// Copy config inputs first (lower priority)
	for k, v := range configInputs {
		result[k] = v
	}

	// Apply CLI inputs (higher priority, overwrites config)
	for k, v := range cliInputs {
		result[k] = v
	}

	return result
}

// collectMissingInputsIfNeeded checks if required inputs are missing and
// prompts the user interactively if stdin is a terminal.
//
// Returns:
//   - Updated inputs map with collected values
//   - Error if stdin is not a terminal and inputs are missing
func collectMissingInputsIfNeeded(
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
		return nil, fmt.Errorf("missing required inputs and stdin is not a terminal; provide inputs via --input flags")
	}

	// Create collector and service
	colorizer := ui.NewColorizer(!cfg.NoColor)
	collector := ui.NewCLIInputCollector(cmd.InOrStdin(), cmd.OutOrStdout(), colorizer)
	service := application.NewInputCollectionService(collector, logger)

	// Collect missing inputs
	return service.CollectMissingInputs(wf, inputs)
}

// isTerminal checks if the given reader is connected to a terminal.
func isTerminal(r io.Reader) bool {
	if f, ok := r.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}
