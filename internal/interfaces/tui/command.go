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
	"github.com/awf-project/cli/internal/infrastructure/audit"
	"github.com/awf-project/cli/internal/infrastructure/config"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infraotel "github.com/awf-project/cli/internal/infrastructure/otel"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/store"
	infraTranscript "github.com/awf-project/cli/internal/infrastructure/transcript"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bridge, inputReader, cleanup, err := buildBridge(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize TUI services: %w", err)
	}
	defer cleanup()

	model := NewWithBridge(bridge, ctx, findAWFAuditLog())
	model.tabMonitoring.SetInputReader(inputReader)
	p := tea.NewProgram(model)

	inputReader.SetSender(p.Send)
	// Wire the monitoring tab's event dispatcher so StartEventLoop can push facade events
	// into the update loop. Without this the Monitoring tab stays blank: the event goroutine
	// has no sender. The tab's senderRef is a shared pointer created before NewProgram, so
	// setting it here reaches the program's model copy (same mechanism as inputReader).
	model.tabMonitoring.SetSender(p.Send)

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
func buildBridge(ctx context.Context) (*Bridge, *TUIInputReader, func(), error) {
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
	// Use the caller's cancellable ctx (mirrors the M8 fix in cli/serve.go) so plugin
	// initialization is interruptible rather than tied to context.Background().
	pluginResult, pluginErr := pluginmgr.InitSystem(ctx, pluginDirs, filepath.Join(storagePath, "plugins"), "", &nopLogger{})

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
		return NewBridge(nil, nil), nil, nopCleanup, nil
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
		return NewBridge(nil, nil), nil, nopCleanup, nil
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

	bridge := NewBridge(result.WorkflowSvc, result.HistorySvc)
	bridge.stream = streamBuf

	// Wire the facade for event-driven execution (T061, D27, FR-011).
	// The facade uses the same services already wired into the bridge so there is no
	// duplicate resource ownership. A per-run transcript recorder factory is wired so the
	// monitoring tab receives LIVE step/message events from Session.Events() — not just the
	// terminal event — and each run gets a transcript file at storage/transcripts/{runID}.
	if facade := buildTUIFacade(result, storagePath, inputReader); facade != nil {
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
// The facade's Adapter receives a transcript.NopRecorder because ExecutionSetup wires its
// own per-execution recorder internally; the facade-level recorder is only required by the
// Adapter constructor and is never called in the code paths exercised by the TUI.
//
// inputReader must be the same TUIInputReader wired into buildBridge so that interactive
// conversation turns route through the unified EventInputRequired path (F107). Passing nil
// disables multi-turn input — the adapter skips newSessionInputReader instantiation.
// Returns nil on any setup error so the caller falls back gracefully.
func buildTUIFacade(result *application.SetupResult, storagePath string, inputReader *TUIInputReader) ports.WorkflowFacade {
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

	adapter := application.NewAdapter(
		result.WorkflowSvc,
		execSvc,
		result.HistorySvc,
		resolver,
		infraTranscript.NewNopRecorder(),
		application.NewSessionRegistry(),
	)
	// Wire the TUI input reader so that interactive conversation turns route through the
	// unified EventInputRequired / RunSession.Respond path (F107). This mirrors the
	// adapter.SetUserInputReader call in cli.buildFacade (~run.go:1229). The inputReader
	// is the same instance already registered with ExecutionSetup via WithUserInputReader,
	// so there is no double-wiring — both paths share a single TUIInputReader.
	// When inputReader is nil (test or degraded startup) the adapter falls back safely:
	// userInputReader == nil skips newSessionInputReader instantiation.
	if inputReader != nil {
		adapter.SetUserInputReader(inputReader)
	}
	// Per-run transcript recorder: each run owns its recorder so live step/message events
	// reach this session's stream (monitoring tab) without contaminating concurrent runs.
	adapter.SetRunRecorderFactory(func(runID string) (ports.Recorder, error) {
		dir := filepath.Join(storagePath, "transcripts")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, err
		}
		return infraTranscript.NewRecorder(filepath.Join(dir, runID+".jsonl"))
	})
	return adapter
}

// nopLogger satisfies ports.Logger for silent TUI operation.
type nopLogger struct{}

func (l *nopLogger) Debug(string, ...any)                    {}
func (l *nopLogger) Info(string, ...any)                     {}
func (l *nopLogger) Warn(string, ...any)                     {}
func (l *nopLogger) Error(string, ...any)                    {}
func (l *nopLogger) WithContext(map[string]any) ports.Logger { return l }
