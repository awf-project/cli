package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/awf-project/cli/internal/infrastructure/audit"
	"github.com/awf-project/cli/internal/infrastructure/config"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infraotel "github.com/awf-project/cli/internal/infrastructure/otel"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/infrastructure/workflowpkg"
	"github.com/awf-project/cli/internal/infrastructure/xdg"
)

var (
	errNoTerminal   = errors.New("no terminal: TERM is not set")
	errDumbTerminal = errors.New("terminal does not support interactive mode")
)

// NewCommand returns the cobra.Command for `awf tui`.
func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive terminal UI",
		Long: `Launch the awf interactive terminal user interface.

The TUI provides a multi-tab view for browsing workflows, monitoring live
executions, inspecting execution history, reading agent conversations, and
tailing external JSONL log files.

Keyboard shortcuts:
  1-5   Switch tabs (Workflows, Monitoring, History, Agent, Logs)
  /     Start filtering (in Workflows or History tab)
  q     Quit
  ctrl+c  Force quit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI(cmd)
		},
	}
}

func runTUI(_ *cobra.Command) error {
	if err := validateTerminal(); err != nil {
		return err
	}

	bridge, inputReader, cleanup, err := buildBridge()
	if err != nil {
		return fmt.Errorf("failed to initialize TUI services: %w", err)
	}
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	model := NewWithBridge(bridge, ctx, findAWFAuditLog())
	model.tabMonitoring.SetInputReader(inputReader)
	p := tea.NewProgram(model)

	inputReader.SetSender(p.Send)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}

	return nil
}

func validateTerminal() error {
	term := os.Getenv("TERM")
	if term == "" {
		return errNoTerminal
	}
	if term == "dumb" {
		return errDumbTerminal
	}
	return nil
}

// buildBridge constructs infrastructure services using the ExecutionSetup builder
// and returns a wired Bridge and TUIInputReader. The returned cleanup function releases all resources.
func buildBridge() (*Bridge, *TUIInputReader, func(), error) {
	storagePath := xdg.AWFDataDir()
	nopCleanup := func() {}

	// Load project config (same as CLI).
	projectCfg, err := config.NewYAMLConfigLoader(xdg.LocalConfigPath()).Load()
	if err != nil {
		projectCfg = &config.ProjectConfig{}
	}

	// Initialize plugin system (same as CLI).
	pluginDirs := []string{
		xdg.LocalPluginsDir(),
		xdg.AWFPluginsDir(),
	}
	pluginResult, pluginErr := pluginmgr.InitSystem(context.Background(), pluginDirs, filepath.Join(storagePath, "plugins"), "", &nopLogger{})

	// Initialize tracer (same as CLI).
	otelCfg := infraotel.TracerConfig{
		Endpoint:    projectCfg.Telemetry.Exporter,
		ServiceName: projectCfg.Telemetry.ServiceName,
	}
	tracer, tracerShutdown, tracerErr := infraotel.NewTracerFromConfig(context.Background(), otelCfg)
	if tracerErr != nil {
		tracer = ports.NopTracer{}
		tracerShutdown = func() {}
	}

	// Initialize audit writer (same as CLI).
	auditWriter, auditCleanup, auditErr := audit.NewWriterFromEnv()
	if auditErr != nil {
		auditWriter = nil
		auditCleanup = func() {}
	}

	// History store.
	historyStore, histErr := store.NewSQLiteHistoryStore(filepath.Join(storagePath, "history.db"))
	if histErr != nil {
		return NewBridge(nil, nil, nil), nil, nopCleanup, nil
	}

	// Core infrastructure dependencies.
	repo := repository.NewCompositeRepository(buildWorkflowPaths())
	stateStore := store.NewJSONStore(filepath.Join(storagePath, "states"))
	shellExecutor := executor.NewShellExecutor()
	logger := &nopLogger{}

	// Assemble setup options.
	setupOpts := []application.SetupOption{
		application.WithNotifyConfig(application.NotifyConfig{DefaultBackend: projectCfg.Notify.DefaultBackend}),
		application.WithHistoryStore(historyStore),
		application.WithTemplatePaths([]string{
			".awf/templates",
			filepath.Join(storagePath, "templates"),
		}),
	}

	if pluginErr == nil && pluginResult != nil {
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
	}

	// Tracer is always present (may be NopTracer when endpoint is unconfigured).
	setupOpts = append(setupOpts, application.WithTracer(tracer))

	if auditWriter != nil {
		setupOpts = append(setupOpts, application.WithAuditWriter(auditWriter))
	}

	// Streaming output buffer — shared between execution and monitoring viewport.
	streamBuf := &StreamBuffer{}

	// Channel-based conversation input reader for multi-turn agent conversations.
	inputReader := NewTUIInputReader(nil)

	// Conversation reader + streaming output.
	setupOpts = append(
		setupOpts,
		application.WithUserInputReader(inputReader),
		application.WithOutputWriters(streamBuf, io.Discard),
	)

	result, buildErr := application.NewExecutionSetup(repo, stateStore, shellExecutor, logger, setupOpts...).Build(context.Background())
	if buildErr != nil {
		_ = historyStore.Close()
		return NewBridge(nil, nil, nil), nil, nopCleanup, nil
	}

	// Wire pack discovery into WorkflowService for unified listing.
	packDirs := []string{
		xdg.LocalWorkflowPacksDir(),
		xdg.AWFWorkflowPacksDir(),
	}
	result.WorkflowSvc.SetPackDiscoverer(workflowpkg.NewPackDiscovererAdapter(packDirs))

	cleanup := func() {
		result.Cleanup()
		tracerShutdown()
		auditCleanup()
		if pluginErr == nil && pluginResult != nil {
			pluginResult.Cleanup()
		}
	}

	bridge := NewBridge(result.WorkflowSvc, result.ExecService, result.HistorySvc)
	bridge.stream = streamBuf

	// Wire the facade for event-driven execution (T061, D27, FR-011).
	// The facade uses the same services already wired into the bridge so there is no
	// duplicate resource ownership. The recorder is a no-op: transcript events flow
	// through the execution service's internal recorder (wired by ExecutionSetup), not
	// through this facade-level nopRecorder which is only required by the Adapter constructor.
	if facade := buildTUIFacade(result); facade != nil {
		bridge.SetFacade(facade)
	}

	return bridge, inputReader, cleanup, nil
}

func buildWorkflowPaths() []repository.SourcedPath {
	var paths []repository.SourcedPath

	if envPath := os.Getenv("AWF_WORKFLOWS_PATH"); envPath != "" {
		paths = append(paths, repository.SourcedPath{
			Path:   envPath,
			Source: repository.SourceEnv,
		})
	}

	paths = append(
		paths,
		repository.SourcedPath{
			Path:   xdg.LocalWorkflowsDir(),
			Source: repository.SourceLocal,
		},
		repository.SourcedPath{
			Path:   xdg.AWFWorkflowsDir(),
			Source: repository.SourceGlobal,
		},
	)

	return paths
}

// findAWFAuditLog returns the AWF audit log path if the file exists.
// Checks AWF_AUDIT_LOG env var first, then the default XDG location.
// Returns "" if audit logging is disabled or the file does not exist.
func findAWFAuditLog() string {
	if envPath := os.Getenv("AWF_AUDIT_LOG"); envPath != "" {
		if envPath == "off" {
			return ""
		}
		cleaned := filepath.Clean(envPath)
		if _, err := os.Stat(cleaned); err == nil {
			return cleaned
		}
		return ""
	}

	path := filepath.Join(xdg.AWFDataDir(), "audit.jsonl")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

// buildTUIFacade constructs a ports.WorkflowFacade from the already-built services in
// result. It mirrors the pattern used by cli.buildFacade (T060) but reuses the services
// that ExecutionSetup already wired rather than constructing new ones.
//
// The facade's Adapter receives a tuiNopRecorder because ExecutionSetup wires its own
// per-execution recorder internally; the facade-level recorder is only required by the
// Adapter constructor and is never called in the code paths exercised by the TUI.
// Returns nil on any setup error so the caller falls back gracefully.
func buildTUIFacade(result *application.SetupResult) ports.WorkflowFacade {
	if result == nil || result.WorkflowSvc == nil {
		return nil
	}

	packDirs := []string{
		xdg.LocalWorkflowPacksDir(),
		xdg.AWFWorkflowPacksDir(),
	}
	discoverer := workflowpkg.NewPackDiscovererAdapter(packDirs)
	repo := repository.NewCompositeRepository(buildWorkflowPaths())
	resolver := application.NewResolver(discoverer, repo)

	// Use zero ExecutionService when result.ExecService is unavailable; the facade's
	// Run method panics gracefully (recover) for truly missing execution dependencies.
	execSvc := result.ExecService
	if execSvc == nil {
		execSvc = &application.ExecutionService{}
	}

	return application.NewAdapter(
		result.WorkflowSvc,
		execSvc,
		result.HistorySvc,
		resolver,
		tuiNopRecorder{},
		application.NewSessionRegistry(),
	)
}

// tuiNopRecorder is a no-op ports.Recorder for the TUI facade. The TUI facade
// Adapter needs a Recorder in its constructor (SC-001 / sole-subscriber contract)
// but the actual transcript recording is handled by the ExecutionService's own
// per-execution recorder wired by ExecutionSetup. This recorder's Subscribe channel
// is immediately closed so any accidental consumer terminates without blocking.
type tuiNopRecorder struct{}

func (tuiNopRecorder) Record(_ context.Context, _ transcript.ExchangeEvent) error { //nolint:gocritic // hugeParam: ports.Recorder contract requires value type
	return nil
}

func (tuiNopRecorder) Subscribe() (ch <-chan transcript.ExchangeEvent, cancel func()) {
	c := make(chan transcript.ExchangeEvent)
	close(c)
	return c, func() {}
}

func (tuiNopRecorder) Close() error { return nil }

// nopLogger satisfies ports.Logger for silent TUI operation.
type nopLogger struct{}

func (l *nopLogger) Debug(string, ...any)                    {}
func (l *nopLogger) Info(string, ...any)                     {}
func (l *nopLogger) Warn(string, ...any)                     {}
func (l *nopLogger) Error(string, ...any)                    {}
func (l *nopLogger) WithContext(map[string]any) ports.Logger { return l }
