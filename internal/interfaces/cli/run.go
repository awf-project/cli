package cli

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/vanoix/awf/internal/infrastructure/executor"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/internal/infrastructure/store"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
	"github.com/vanoix/awf/pkg/expression"
	"github.com/vanoix/awf/pkg/interpolation"
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
	cmd.Flags().StringArrayVar(&breakpointFlags, "breakpoint", nil,
		"Pause only at specified steps in interactive mode (comma-separated)")

	return cmd
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

	// Create history store and service
	historyStore, err := store.NewBadgerHistoryStore(filepath.Join(cfg.StoragePath, "history"))
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

	// Execute workflow
	execCtx, execErr := execSvc.Run(ctx, workflowName, inputs)

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
		if err := writer.WriteRunResult(result); err != nil {
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
func runInteractive(cmd *cobra.Command, cfg *Config, workflowName string, inputFlags []string, breakpointFlags []string) error {
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
// If the value starts with @prompts/, the file content is loaded from .awf/prompts/.
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

	// Build full path to prompt file
	promptsDir := ".awf/prompts"
	fullPath := filepath.Join(promptsDir, relativePath)

	// Security: verify resolved path is still within prompts directory
	cleanPath := filepath.Clean(fullPath)
	if !strings.HasPrefix(cleanPath, filepath.Clean(promptsDir)) {
		return "", fmt.Errorf("invalid prompt path: path traversal not allowed")
	}

	// Read file content
	content, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("prompt file not found: %s", relativePath)
		}
		return "", fmt.Errorf("failed to read prompt file: %w", err)
	}

	// Trim leading/trailing whitespace
	return strings.TrimSpace(string(content)), nil
}

func showExecutionDetails(formatter *ui.Formatter, execCtx *workflow.ExecutionContext) {
	formatter.Printf("\n--- Execution Details ---\n")
	formatter.Printf("Status: %s\n", execCtx.Status)
	formatter.Printf("Steps executed:\n")

	for name, state := range execCtx.States {
		duration := state.CompletedAt.Sub(state.StartedAt).Round(time.Millisecond)
		formatter.StatusLine("  "+name, string(state.Status), fmt.Sprintf("(%s)", duration))
	}
}

func showStepOutputs(formatter *ui.Formatter, execCtx *workflow.ExecutionContext) {
	for name, state := range execCtx.States {
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
	var steps []ui.StepInfo
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
	historyStore, err := store.NewBadgerHistoryStore(filepath.Join(cfg.StoragePath, "history"))
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
