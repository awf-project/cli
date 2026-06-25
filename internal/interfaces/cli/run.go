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
	infraotel "github.com/awf-project/cli/internal/infrastructure/otel"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/roles"
	"github.com/awf-project/cli/internal/infrastructure/store"
	infraTranscript "github.com/awf-project/cli/internal/infrastructure/transcript"
	"github.com/awf-project/cli/internal/infrastructure/workflowpkg"
	"github.com/awf-project/cli/internal/infrastructure/xdg"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/google/uuid"
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
	var debugTranscriptMirror string

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
			return runWorkflow(cmd, cfg, args[0], inputFlags, skipPlugins, debugTranscriptMirror)
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
	cmd.Flags().StringVar(&cfg.OtelExporter, "otel-exporter", "", "OpenTelemetry OTLP exporter endpoint (e.g. localhost:4317)")
	cmd.Flags().StringVar(&cfg.OtelServiceName, "otel-service-name", "awf", "OpenTelemetry service name")
	cmd.Flags().StringVar(&debugTranscriptMirror, "debug-transcript-mirror", "",
		"[DEBUG] Write received transcript events to this path (subscription mirror)")

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

func runWorkflow(cmd *cobra.Command, cfg *Config, workflowName string, inputFlags []string, skipPlugins bool, debugTranscriptMirror string) error {
	// Parse inputs
	inputs, err := parseInputFlags(inputFlags)
	if err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}

	// The facade executes the workflow on a background goroutine while this goroutine
	// drains and renders the session's events. Every writer below (OutputWriter,
	// Formatter→logger, per-step PrefixedWriters) and the event projector in
	// runWorkflowViaFacade share stdout/stderr, so concurrent writes would data-race.
	// Wrap each sink ONCE in a SyncWriter and thread that single instance through the
	// whole run path so all writes serialize through one mutex.
	syncOut := ui.NewSyncWriter(cmd.OutOrStdout())
	syncErr := ui.NewSyncWriter(cmd.ErrOrStderr())

	// Create output writer for JSON/quiet formats
	writer := ui.NewOutputWriter(syncOut, syncErr, cfg.OutputFormat, cfg.NoColor, cfg.NoHints)

	// Create formatter for text output
	formatter := ui.NewFormatter(syncOut, ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	// Setup output writers based on mode (only for non-JSON formats). Both OutputStreaming
	// ("real-time") and OutputBuffered ("show after completion") must surface a step
	// command's stdout/stderr — only OutputSilent suppresses it. The facade migration had
	// restricted this to OutputStreaming, which silently dropped command output in buffered
	// mode (regression: `awf run --output buffered` showed step status but no command output).
	var stdoutWriter, stderrWriter *ui.PrefixedWriter
	if !cfg.Quiet && (cfg.OutputMode == OutputStreaming || cfg.OutputMode == OutputBuffered) && !writer.IsJSONFormat() {
		colorizer := ui.NewColorizer(!cfg.NoColor)
		stdoutWriter = ui.NewPrefixedWriter(syncOut, ui.PrefixStdout, "", colorizer)
		stderrWriter = ui.NewPrefixedWriter(syncErr, ui.PrefixStderr, "", colorizer)
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
	repo, workflowName := newWorkflowRepositoryForIdentifier(workflowName)
	stateStore := store.NewJSONStore(cfg.StoragePath + "/states")
	shellExecutor := executor.NewShellExecutor()
	// Table and JSON formats should be silent (structured output only)
	silentOutput := cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatTable
	logger := &cliLogger{
		formatter: formatter,
		verbose:   cfg.Verbose && !silentOutput,
		silent:    silentOutput,
	}

	// Purge orphan MCP registrations left by crashed prior runs before any
	// workflow logic runs. Failures are non-fatal and logged at debug level.
	if purgeErr := agents.PurgeOrphanMCPRegistrations(ctx, shellExecutor, logger); purgeErr != nil {
		logger.Debug("orphan MCP purge returned unexpected error", "error", purgeErr)
	}

	// Load project config from .awf/config.yaml
	projectCfg, err := loadProjectConfig(logger)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	// Apply telemetry defaults from project config (CLI flags win)
	if !cmd.Flags().Changed("otel-exporter") && projectCfg.Telemetry.Exporter != "" {
		cfg.OtelExporter = projectCfg.Telemetry.Exporter
	}
	if !cmd.Flags().Changed("otel-service-name") && projectCfg.Telemetry.ServiceName != "" {
		cfg.OtelServiceName = projectCfg.Telemetry.ServiceName
	}

	tracer, tracerShutdown, tracerInitErr := infraotel.NewTracerFromConfig(ctx, infraotel.TracerConfig{
		Endpoint:    cfg.OtelExporter,
		ServiceName: cfg.OtelServiceName,
	})
	if tracerInitErr != nil {
		logger.Warn("failed to initialize tracer, tracing disabled", "error", tracerInitErr)
		tracer = ports.NopTracer{}
		tracerShutdown = func() {}
	}
	defer tracerShutdown()

	// Merge config inputs with CLI inputs (CLI wins)
	inputs = application.MergeInputs(projectCfg.Inputs, inputs)

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

	// NOTE: a read-only AutoFacade may already be wired into cfg.Facade (config.go
	// buildFacade, used by list/validate/status/history). It carries a ZERO
	// ExecutionService and therefore cannot execute, so we do NOT dispatch through it
	// here. Instead, after the full execution stack is built below, we construct a
	// run-capable Adapter over the real ExecutionService and dispatch through that
	// (see the run-capable facade wiring after setupResult.Build).

	// Create history store (lifecycle managed by ExecutionSetup builder via WithHistoryStore)
	historyStore, err := store.NewSQLiteHistoryStore(filepath.Join(cfg.StoragePath, "history.db"))
	if err != nil {
		return fmt.Errorf("failed to open history store: %w", err)
	}

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

	// Setup audit trail writer (F071)
	var auditWriter ports.AuditTrailWriter
	if aw, auditCleanup, auditErr := audit.NewWriterFromEnv(); auditErr != nil {
		logger.Warn("failed to initialize audit writer, audit trail disabled", "error", auditErr)
	} else {
		defer auditCleanup()
		auditWriter = aw
	}

	// Setup transcript recorder (F106)
	runID := uuid.New().String()
	var recorder ports.Recorder
	var mirrorCancel func()
	if rec, recCleanup, recErr := WireTranscript(runID, cfg.StoragePath); recErr != nil {
		logger.Warn("failed to initialize transcript recorder, transcripts disabled", "error", recErr)
	} else {
		defer recCleanup() //nolint:errcheck // best-effort transcript flush on exit
		recorder = rec
		mirrorCancel = AttachMirrorSubscriber(rec, debugTranscriptMirror)
		defer mirrorCancel()
	}

	templatePaths := []string{
		".awf/templates",
		filepath.Join(cfg.StoragePath, "templates"),
	}

	packResolver := func(ctx context.Context, targetPackName, targetWorkflow string) (*workflow.Workflow, string, error) {
		return resolvePackWorkflow(ctx, targetPackName, targetWorkflow, xdg.LocalWorkflowPacksDir(), xdg.AWFWorkflowPacksDir())
	}

	// Build the F099 MCP tool proxy CLIExecutor for subprocess lifecycle management.
	// The ProviderFactory itself is built inside ExecutionSetup.Build so it can capture
	// the composite OperationProvider and expose plugin tools alongside builtins.
	toolCLIExec := agents.NewExecCLIExecutor()

	setupOpts := []application.SetupOption{
		application.WithNotifyConfig(application.NotifyConfig{DefaultBackend: projectCfg.Notify.DefaultBackend}),
		application.WithHistoryStore(historyStore),
		application.WithTemplatePaths(templatePaths),
		application.WithTracer(tracer),
		application.WithAgentRoleRepository(roles.NewFilesystemAgentRoleRepository(logger)),
		application.WithUserInputReader(ui.NewStdinInputReader(os.Stdin, os.Stdout)),
		application.WithPackContext(packName, packResolver),
		application.WithToolProxy(toolCLIExec),
	}

	if !skipPlugins && pluginResult != nil {
		setupOpts = append(
			setupOpts,
			application.WithPluginState(pluginResult.Service),
			application.WithPluginService(pluginResult.Service),
		)
		if pluginResult.RPCManager != nil {
			setupOpts = append(setupOpts, application.WithPluginProviders(application.PluginProviders{
				Operations: pluginResult.Manager,
				Validators: pluginResult.RPCManager.ValidatorProvider(0),
				StepTypes:  pluginResult.RPCManager.StepTypeProvider(logger),
			}))
		}
		if pluginResult.EventPublisher != nil {
			setupOpts = append(setupOpts, application.WithEventPublisher(pluginResult.EventPublisher))
		}
	}

	if auditWriter != nil {
		setupOpts = append(setupOpts, application.WithAuditWriter(auditWriter))
	}

	if recorder != nil {
		setupOpts = append(
			setupOpts,
			application.WithRecorder(recorder),
			application.WithRecorderFactory(NewRecorderFactory()),
			application.WithTranscriptDir(filepath.Join(cfg.StoragePath, "transcripts")),
		)
	}

	if stdoutWriter != nil {
		setupOpts = append(setupOpts, application.WithOutputWriters(stdoutWriter, stderrWriter))
	}

	setupResult, err := application.NewExecutionSetup(repo, stateStore, shellExecutor, logger, setupOpts...).Build(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize execution: %w", err)
	}
	defer setupResult.Cleanup()

	execSvc := setupResult.ExecService

	// Run-capable facade wiring (F108 T070): build an application.Adapter over the REAL
	// ExecutionService just built, a Resolver over the same repo+discoverer the read-only
	// facade uses, and the SAME transcript recorder wired above so transcript events stream
	// into the session. Dispatch through the already-tested driver runWorkflowViaFacade. When
	// the recorder is nil (transcript init failed), buildRunCapableFacade substitutes a
	// NopRecorder so the Adapter's sole Subscribe still has a (closed) channel to drain.
	runFacade := buildRunCapableFacade(execSvc, setupResult.WorkflowSvc, historyStore, repo, recorder, logger)
	if runFacade == nil {
		return fmt.Errorf("failed to initialize execution facade")
	}
	cfg.Facade = runFacade
	return runWorkflowViaFacade(ctx, cmd, cfg, writer, formatter, workflowName, inputs)
}

// runDryRun executes a dry-run of the workflow, showing the execution plan without running commands.
func runDryRun(cmd *cobra.Command, cfg *Config, workflowName string, inputFlags []string, _ bool) error {
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
	repo, workflowName := newWorkflowRepositoryForIdentifier(workflowName)
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
	inputs = application.MergeInputs(projectCfg.Inputs, inputs)

	// Create services
	exprValidator := infraexpression.NewExprValidator()
	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, logger, exprValidator)
	dryRunExec := application.NewDryRunExecutor(wfSvc, resolver, exprEvaluator, logger)
	dryRunExec.SetAWFPaths(xdg.AWFPaths())

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

// runInteractive executes the workflow in interactive step-by-step mode (F020).
//
// F108 routed every interface through the single-core facade and severed the legacy
// InteractiveExecutor wiring, which silently removed the entire interactive feature
// (step preview, per-step continue/skip/abort/inspect/edit/retry prompts, breakpoints,
// "Type: command/parallel" details). F110 G3 restores it.
//
// The facade's Run surface streams a workflow to completion with no per-step parking for
// the legacy interactive action set, and RunStep executes a single step in isolation with
// a fresh interpolation context (no transition resolution or cross-step state threading) —
// neither can drive the documented stepping UX without re-architecting the facade event
// model. The application-layer InteractiveExecutor (still present and the behavioral spec
// mined by interactive_test.go) already owns that loop: step traversal, breakpoint gating,
// the prompt action set, inspect/edit parking, retry, and state checkpointing. We re-wire
// it here from the interface layer (an arch-permitted interfaces-cli -> application edge,
// matching the pre-F108 design) so interactive runs regain full parity. Non-interactive
// run/resume/run-step paths remain facade-routed.
func runInteractive(cmd *cobra.Command, cfg *Config, workflowName string, inputFlags, breakpointFlags []string, _ bool) error {
	// Parse inputs
	inputs, err := parseInputFlags(inputFlags)
	if err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}

	// Parse breakpoints (flatten comma-separated values). An empty/nil breakpoint set means
	// pause at every step; a non-empty set pauses only at the named steps.
	var breakpoints []string
	for _, bp := range breakpointFlags {
		for b := range strings.SplitSeq(bp, ",") {
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

	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)

	// Setup context with signal handling so Ctrl+C aborts a parked prompt gracefully.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalCleanup := setupSignalHandler(ctx, cancel, nil)
	defer signalCleanup()

	// Initialize dependencies
	repo, workflowName := newWorkflowRepositoryForIdentifier(workflowName)
	stateStore := store.NewJSONStore(cfg.StoragePath + "/states")
	shellExecutor := executor.NewShellExecutor()
	logger := &cliLogger{
		formatter: formatter,
		verbose:   cfg.Verbose,
		silent:    false,
	}
	resolver := interpolation.NewTemplateResolver()
	exprEvaluator := infraexpression.NewExprEvaluator()

	// Merge config inputs with CLI inputs (CLI wins) so config-provided required inputs
	// are honored without prompting, matching the standard run path (B007).
	projectCfg, cfgErr := loadProjectConfig(logger)
	if cfgErr != nil {
		return writeErrorAndExit(writer, fmt.Errorf("config error: %w", cfgErr), categorizeError(cfgErr))
	}
	inputs = application.MergeInputs(projectCfg.Inputs, inputs)

	// Create services
	exprValidator := infraexpression.NewExprValidator()
	wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, logger, exprValidator)
	parallelExecutor := application.NewParallelExecutor(logger)

	// Create interactive prompt bound to the command's I/O streams (tests inject stdin).
	prompt := ui.NewCLIPrompt(cmd.InOrStdin(), cmd.OutOrStdout(), !cfg.NoColor)

	// Create interactive executor
	interactiveExec := application.NewInteractiveExecutor(
		wfSvc, shellExecutor, parallelExecutor, stateStore,
		logger, resolver, exprEvaluator, prompt,
	)

	// Setup template service for workflow template expansion
	templatePaths := []string{
		".awf/templates",
		filepath.Join(cfg.StoragePath, "templates"),
	}
	templateRepo := repository.NewYAMLTemplateRepository(templatePaths)
	templateSvc := application.NewTemplateService(templateRepo, logger)
	interactiveExec.SetTemplateService(templateSvc)
	interactiveExec.SetAWFPaths(xdg.AWFPaths())

	// Set breakpoints if specified (nil/empty => pause at every step).
	if len(breakpoints) > 0 {
		interactiveExec.SetBreakpoints(breakpoints)
	}

	// Execute interactive workflow
	_, execErr := interactiveExec.Run(ctx, workflowName, inputs)
	if execErr != nil {
		// Context cancellation is a graceful interactive abort, not an error.
		if errors.Is(execErr, context.Canceled) {
			return nil
		}
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
	repo, workflowName := newWorkflowRepositoryForIdentifier(workflowName)
	stateStore := store.NewJSONStore(cfg.StoragePath + "/states")
	shellExecutor := executor.NewShellExecutor()
	logger := &cliLogger{
		formatter: formatter,
		verbose:   cfg.Verbose,
		silent:    cfg.Quiet,
	}

	// Load project config from .awf/config.yaml
	// TODO(T027): Use projectCfg.Notify to register backends dynamically
	projectCfg, err := loadProjectConfig(logger)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	// Create history store (lifecycle managed by ExecutionSetup builder via WithHistoryStore)
	historyStore, err := store.NewSQLiteHistoryStore(filepath.Join(cfg.StoragePath, "history.db"))
	if err != nil {
		return fmt.Errorf("failed to open history store: %w", err)
	}

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

	// Parse namespace to set up pack context if applicable
	packName, _ := parseWorkflowNamespace(workflowName)
	packResolver := func(ctx context.Context, targetPackName, targetWorkflow string) (*workflow.Workflow, string, error) {
		return resolvePackWorkflow(ctx, targetPackName, targetWorkflow, xdg.LocalWorkflowPacksDir(), xdg.AWFWorkflowPacksDir())
	}

	templatePaths := []string{
		".awf/templates",
		filepath.Join(cfg.StoragePath, "templates"),
	}

	stepSetupOpts := []application.SetupOption{
		application.WithNotifyConfig(application.NotifyConfig{DefaultBackend: projectCfg.Notify.DefaultBackend}),
		application.WithHistoryStore(historyStore),
		application.WithTemplatePaths(templatePaths),
		application.WithUserInputReader(ui.NewStdinInputReader(os.Stdin, os.Stdout)),
		application.WithPackContext(packName, packResolver),
		application.WithAgentRoleRepository(roles.NewFilesystemAgentRoleRepository(logger)),
	}

	if !skipPlugins && pluginResult != nil {
		stepSetupOpts = append(
			stepSetupOpts,
			application.WithPluginState(pluginResult.Service),
			application.WithPluginService(pluginResult.Service),
		)
		if pluginResult.RPCManager != nil {
			stepSetupOpts = append(stepSetupOpts, application.WithPluginProviders(application.PluginProviders{
				Operations: pluginResult.Manager,
				StepTypes:  pluginResult.RPCManager.StepTypeProvider(logger),
			}))
		}
	}

	stepSetupResult, err := application.NewExecutionSetup(repo, stateStore, shellExecutor, logger, stepSetupOpts...).Build(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize execution: %w", err)
	}
	defer stepSetupResult.Cleanup()

	execSvc := stepSetupResult.ExecService

	// Route through the facade so the single-step path is no longer a direct
	// ExecutionService caller (F108 last non-facade execution path).
	// RunStep is a CLI-only operation on ports.SingleStepRunner (M1); the
	// production Adapter implements both WorkflowFacade and SingleStepRunner.
	runFacade := buildRunCapableFacade(execSvc, stepSetupResult.WorkflowSvc, historyStore, repo, infraTranscript.NewNopRecorder(), logger)
	if runFacade == nil {
		return fmt.Errorf("failed to initialize execution facade for step")
	}
	stepRunner, ok := runFacade.(ports.SingleStepRunner)
	if !ok {
		return fmt.Errorf("execution facade does not support single-step execution")
	}

	// Show start message
	if !cfg.Quiet {
		formatter.Info(fmt.Sprintf("Running single step: %s from workflow: %s", stepName, workflowName))
	}
	startTime := time.Now()

	// Execute single step via SingleStepRunner
	stepRes, execErr := stepRunner.RunStep(ctx, ports.RunStepRequest{
		Identifier: workflowName,
		StepName:   stepName,
		Inputs:     inputs,
		Mocks:      mocks,
	})

	// Calculate duration
	duration := time.Since(startTime).Round(time.Millisecond)

	if execErr != nil {
		// Create writer for error routing
		writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), ui.FormatText, cfg.NoColor, cfg.NoHints)
		return writeErrorAndExit(writer, execErr, categorizeError(execErr))
	}

	// Display result
	if stepRes.Output != "" && !cfg.Quiet {
		formatter.Printf("\n--- [%s] stdout ---\n", stepRes.StepName)
		formatter.Printf("%s", stepRes.Output)
	}
	if stepRes.Stderr != "" && !cfg.Quiet {
		formatter.Printf("\n--- [%s] stderr ---\n", stepRes.StepName)
		formatter.Printf("%s", stepRes.Stderr)
	}

	if stepRes.Status == ports.RunStateCompleted {
		formatter.Success(fmt.Sprintf("Step '%s' completed successfully in %s", stepName, duration))
	} else {
		formatter.Error(fmt.Sprintf("Step '%s' failed (exit code: %d)", stepName, stepRes.ExitCode))
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

// buildRunCapableFacade constructs an execution-capable ports.WorkflowFacade for the CLI
// `run` path (F108 T070 enabler). Unlike the read-only AutoFacade in config.go (zero
// ExecutionService), this Adapter wraps the REAL ExecutionService built by ExecutionSetup,
// a Resolver over the same repo + pack discoverer, and the SAME transcript recorder used by
// the run so transcript events stream into the RunSession. Returns nil when no execution
// service is available so callers can fall back to the legacy direct-execution path.
func buildRunCapableFacade(
	execSvc *application.ExecutionService,
	workflowSvc *application.WorkflowService,
	historyStore ports.HistoryStore,
	repo ports.WorkflowRepository,
	recorder ports.Recorder,
	logger ports.Logger,
) ports.WorkflowFacade {
	if execSvc == nil {
		return nil
	}
	// The Adapter is the sole Recorder.Subscribe caller (SC-001). If transcript wiring
	// failed (nil recorder), give it a NopRecorder whose Subscribe yields an already-closed
	// channel — the Adapter tolerates this and still emits the terminal event.
	facadeRecorder := recorder
	if facadeRecorder == nil {
		facadeRecorder = infraTranscript.NewNopRecorder()
	}
	discoverer := workflowpkg.NewPackDiscovererAdapter(workflowPackSearchDirs())
	resolver := application.NewResolver(discoverer, repo)
	historySvc := application.NewHistoryService(historyStore, logger)
	adapter := application.NewAdapter(
		workflowSvc,
		execSvc,
		historySvc,
		resolver,
		facadeRecorder,
		application.NewSessionRegistry(),
	)
	// Wire the user-input edge so interactive conversation turns route through the
	// RunSession parking mechanism (EventInputRequired -> RunSession.Respond) instead
	// of a process-global stdin read that EOFs before the agent runs (F110 G4). The
	// CLI run driver (runWorkflowViaFacade) answers these parks from stdin. The reader
	// passed here is a non-nil signal that the CLI owns input; the Adapter binds a
	// session-scoped bridge per run.
	adapter.SetUserInputReader(ui.NewStdinInputReader(os.Stdin, os.Stdout))
	return adapter
}

// runWorkflowViaFacade executes a workflow through the single-core ports.WorkflowFacade
// (T068): it dispatches via cfg.Facade.Run, streams the projected events to stdout as they
// arrive, and maps the terminal outcome to the CLI exit-code taxonomy. A dispatch rejection
// (e.g. unknown/empty identifier) and a terminal failure both flow through categorizeError
// so the exit codes match the legacy execSvc path.
func runWorkflowViaFacade(
	ctx context.Context,
	cmd *cobra.Command,
	cfg *Config,
	writer *ui.OutputWriter,
	_ *ui.Formatter,
	workflowName string,
	inputs map[string]any,
) error {
	session, err := cfg.Facade.Run(ctx, ports.RunRequest{
		Identifier: workflowName,
		Inputs:     inputs,
	})
	if err != nil {
		return writeErrorAndExit(writer, err, categorizeError(err))
	}
	defer func() { _ = session.Close() }()

	// Restore the legacy "Workflow started:" / "Workflow ID:" lines (F110 G2). The
	// production facade does not emit a run.started transcript event, so no EventRunStarted
	// reaches the session; the run ID is the session ID. Render a synthetic run-started event
	// through the same renderer so the phrasing stays centralized (and quiet-suppressed).
	if out := RenderFacadeEventsToTextWithOptions(
		[]ports.Event{{Kind: ports.EventRunStarted, RunID: session.ID()}},
		facadeRenderOptions{Quiet: cfg.Quiet},
	); len(out) > 0 {
		// writer.Out() is the shared SyncWriter, so projector writes serialize with the
		// background execution goroutine's logger/step output (no stdout data race).
		fmt.Fprint(writer.Out(), string(out))
	}

	// Interactive conversation turns park the workflow with EventInputRequired; the
	// driver answers them from stdin and resumes via session.Respond (F110 G4). This
	// mirrors how ACP routes input requests, so the conversation reaches the agent
	// instead of EOFing on a process-global stdin read. A non-terminal/EOF stdin yields
	// an empty response, which the ConversationManager treats as a graceful user exit.
	inputReader := ui.NewStdinInputReader(cmd.InOrStdin(), writer.Out())

	// Stream each projected event to stdout as it arrives (the same renderer the facade
	// conformance test exercises). Draining to completion also guarantees the terminal
	// event has been observed before we read the outcome. Failure is signaled either via
	// session.Err() (the production Adapter) or via an EventWorkflowFailed terminal (the
	// test fake, whose Err() is always nil) — handle both.
	var failed bool
	var failPayload any
	for ev := range session.Events() {
		if ev.Kind == ports.EventInputRequired {
			value, readErr := inputReader.ReadInput(ctx)
			if readErr != nil {
				// EOF (non-interactive) or cancellation: respond with empty input so the
				// conversation ends cleanly instead of stalling the parked turn.
				value = ""
			}
			_ = session.Respond(ports.InputResponse{Value: value}) //nolint:errcheck // a closed/duplicate respond is non-fatal; the run terminates via its own events
			continue
		}
		if ev.Kind == ports.EventWorkflowFailed {
			failed = true
			failPayload = ev.Payload
		}
		if out := RenderFacadeEventsToTextWithOptions([]ports.Event{ev}, facadeRenderOptions{Quiet: cfg.Quiet}); len(out) > 0 {
			fmt.Fprint(writer.Out(), string(out))
		}
	}

	if runErr := session.Err(); runErr != nil {
		return writeErrorAndExit(writer, runErr, categorizeError(runErr))
	}
	if failed {
		runErr := fmt.Errorf("workflow failed")
		if failPayload != nil {
			// EventWorkflowFailed always carries *ports.EnrichedTerminal (see facade_adapter.go
			// emitTerminalEvent and input_bridge.go). Type-assert to extract the structured error
			// message rather than formatting the raw struct pointer (which produces "&{...}").
			if termPayload, ok := failPayload.(*ports.EnrichedTerminal); ok && termPayload.Error != "" {
				runErr = fmt.Errorf("workflow failed: %s", termPayload.Error)
			} else if !ok {
				// Unexpected payload type: format as-is so debug info is not silently lost.
				runErr = fmt.Errorf("workflow failed: %v", failPayload)
			}
		}
		return writeErrorAndExit(writer, runErr, categorizeError(runErr))
	}
	return nil
}

// isTerminal checks if the given reader is connected to a terminal.
func isTerminal(r io.Reader) bool {
	if f, ok := r.(*os.File); ok {
		return term.IsTerminal(int(f.Fd())) //nolint:gosec // G115: file descriptor values are within int range on all supported platforms
	}
	return false
}
