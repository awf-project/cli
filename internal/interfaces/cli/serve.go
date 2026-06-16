package cli

import (
	"context"
	"fmt"
	"net"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/audit"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infraotel "github.com/awf-project/cli/internal/infrastructure/otel"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/awf-project/cli/internal/infrastructure/store"
	infraTranscript "github.com/awf-project/cli/internal/infrastructure/transcript"
	"github.com/awf-project/cli/internal/infrastructure/workflowpkg"
	"github.com/awf-project/cli/internal/infrastructure/xdg"
	"github.com/awf-project/cli/internal/interfaces/api"
	"github.com/spf13/cobra"
)

// NewServeCommand returns the cobra.Command for `awf serve`.
func NewServeCommand() *cobra.Command {
	var port int
	var host string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the AWF REST API HTTP server",
		Long: `Start the AWF REST API server for programmatic workflow management.

The server exposes workflow discovery, async execution, SSE streaming, and
execution history endpoints. By default it binds to 127.0.0.1:2511 to
prevent inadvertent network exposure; use --host to opt in to wider binding.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(cmd, host, port)
		},
	}

	cmd.Flags().IntVar(&port, "port", 2511, "TCP port to listen on (default 2511)")
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Interface address to bind (default 127.0.0.1)")

	return cmd
}

func runServe(cmd *cobra.Command, host string, port int) error {
	if ip := net.ParseIP(host); ip != nil && !ip.IsLoopback() {
		cmd.PrintErrf("warning: binding to non-loopback address %s — ensure access control is in place\n", host)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	storagePath := xdg.AWFDataDir()
	logger := &cliLogger{silent: true}

	projectCfg, err := loadProjectConfig(logger)
	if err != nil {
		projectCfg = nil
	}

	pluginDirs := []string{
		xdg.LocalPluginsDir(),
		xdg.AWFPluginsDir(),
	}
	pluginResult, pluginErr := pluginmgr.InitSystem(ctx, pluginDirs, filepath.Join(storagePath, "plugins"), "", logger)

	otelEndpoint := ""
	otelServiceName := ""
	if projectCfg != nil {
		otelEndpoint = projectCfg.Telemetry.Exporter
		otelServiceName = projectCfg.Telemetry.ServiceName
	}
	tracer, tracerShutdown, tracerErr := infraotel.NewTracerFromConfig(ctx, infraotel.TracerConfig{
		Endpoint:    otelEndpoint,
		ServiceName: otelServiceName,
	})
	if tracerErr != nil {
		tracer = ports.NopTracer{}
		tracerShutdown = func() {}
	}

	auditWriter, auditCleanup, auditErr := audit.NewWriterFromEnv()
	if auditErr != nil {
		auditWriter = nil
		auditCleanup = func() {}
	}

	historyStore, histErr := store.NewSQLiteHistoryStore(filepath.Join(storagePath, "history.db"))
	if histErr != nil {
		tracerShutdown()
		auditCleanup()
		if pluginErr == nil && pluginResult != nil {
			pluginResult.Cleanup()
		}
		return fmt.Errorf("serve: failed to open history store: %w", histErr)
	}

	repo := NewWorkflowRepository()
	stateStore := store.NewJSONStore(filepath.Join(storagePath, "states"))
	shellExecutor := executor.NewShellExecutor()

	setupOpts := []application.SetupOption{
		application.WithHistoryStore(historyStore),
		application.WithTemplatePaths([]string{
			".awf/templates",
			filepath.Join(storagePath, "templates"),
		}),
	}
	if projectCfg != nil {
		setupOpts = append(setupOpts, application.WithNotifyConfig(application.NotifyConfig{
			DefaultBackend: projectCfg.Notify.DefaultBackend,
		}))
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

	setupOpts = append(setupOpts, application.WithTracer(tracer))
	if auditWriter != nil {
		setupOpts = append(setupOpts, application.WithAuditWriter(auditWriter))
	}

	result, buildErr := application.NewExecutionSetup(repo, stateStore, shellExecutor, logger, setupOpts...).Build(ctx)
	if buildErr != nil {
		if closeErr := historyStore.Close(); closeErr != nil {
			logger.Warn("failed to close history store", "error", closeErr)
		}
		tracerShutdown()
		auditCleanup()
		if pluginErr == nil && pluginResult != nil {
			pluginResult.Cleanup()
		}
		return fmt.Errorf("serve: failed to initialize services: %w", buildErr)
	}

	// Defer secondary resources (tracer, audit, plugins) that have no ordering
	// constraint relative to execution goroutines.  result.Cleanup() is NOT deferred
	// here: it must run AFTER bridge.Shutdown() drains in-flight executions so facade
	// goroutines do not write to the history store after it is closed (see shutdown
	// sequence below).
	defer func() {
		tracerShutdown()
		auditCleanup()
		if pluginErr == nil && pluginResult != nil {
			pluginResult.Cleanup()
		}
	}()

	result.WorkflowSvc.SetPackDiscoverer(workflowpkg.NewPackDiscovererAdapter(workflowPackSearchDirs()))

	// F108: the Bridge holds no read or execution port — workflow list/get/validate and
	// history queries route through ports.WorkflowFacade / ports.WorkflowReader (wired below
	// via WithFacade / WithWorkflowReader), and execution routes through Run/Resume. The
	// Bridge only tracks metadata (List/Get/Cancel) for sessions started by the facade.
	bridge := api.NewBridge()

	// Build a single shared session registry for the HTTP interface. The same registry
	// is used by both the Adapter (which adds/removes sessions via onClose) and the
	// execution handler (which resolves sessions for SSE). A single shared registry is
	// the source of truth — there must be no private registry inside buildHTTPFacade
	// (B2: split-registry memory leak and ErrSessionExists on resume).
	httpRegistry := application.NewSessionRegistry()

	// Build a run-capable facade for the HTTP interface so all workflow execution
	// routes through ports.WorkflowFacade (Run and Resume). A per-run transcript recorder
	// is wired so SSE/GET receive LIVE step and message events (not just the terminal one).
	httpFacade := buildHTTPFacade(result, repo, historyStore, storagePath, httpRegistry)

	addr := fmt.Sprintf("%s:%d", host, port)
	srv := api.NewServer(
		bridge, addr,
		api.WithFacade(httpFacade),
		api.WithWorkflowReader(httpFacade),
		api.WithRegistryImpl(httpRegistry),
	)

	cmd.Printf("AWF API server listening on http://%s\n", addr)
	cmd.Printf("Swagger UI: http://%s/docs\n", addr)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	select {
	case serveErr := <-errCh:
		// Server exited before a signal (e.g. listen error). Clean up execution
		// resources on the way out; no in-flight runs to drain.
		result.Cleanup()
		return serveErr
	case <-ctx.Done():
		cmd.Println("Shutting down server...")
		// Shutdown ordering (must be preserved):
		//  1. srv.Shutdown — stops accepting new HTTP requests and waits for
		//     in-flight HTTP handlers to return (SSE streams included via sseWG).
		//  2. bridge.Shutdown — cancels every active execution context so facade
		//     goroutines (startExecution) begin winding down.
		//  3. result.Cleanup — closes the history store and other shared stores.
		//     This must come AFTER bridge.Shutdown so that execution goroutines
		//     have received their cancellation signal and have had a chance to
		//     finish their last write before the store is closed.
		//     The facade exposes no WaitGroup join, so there is an inherent
		//     window between context cancellation and goroutine exit. The HTTP
		//     server's graceful shutdown (step 1) ensures no new work starts,
		//     and the context cancel (step 2) makes existing work exit promptly;
		//     result.Cleanup immediately following is the closest safe ordering
		//     available without a facade-side WaitGroup (tracked for future work).
		shutdownErr := srv.Shutdown(context.Background())
		bridge.Shutdown()
		result.Cleanup()
		cmd.Println("Server stopped.")
		return shutdownErr
	}
}

// buildHTTPFacade constructs a run-capable ports.WorkflowFacade for the HTTP serve path
// (F108). It mirrors buildRunCapableFacade from run.go and wires a PER-RUN transcript
// recorder factory so each async run owns its recorder: live step/message events flow to
// the right session's stream (SSE, GET) with no cross-run contamination, and each run also
// gets a transcript file at storage/transcripts/{runID}.jsonl (parity with CLI). The
// constructor recorder stays a NopRecorder as a safe fallback when the factory fails.
//
// registry MUST be the same *application.SessionRegistry wired into WithRegistryImpl so that
// the Adapter's onClose hook (which calls registry.Remove) and the execution handler (which
// calls registry.Get) operate on the same map. Passing two different registries caused a
// memory leak (B2): sessions added to httpRegistry were never removed because onClose only
// removed from the private registry inside the Adapter, and ErrSessionExists was triggered
// on resume because both registries held the same ID after a run.
// buildHTTPFacade returns the concrete *application.Adapter (not the ports.WorkflowFacade
// interface) so the caller can wire it as BOTH the execution facade (WithFacade) and the
// read-only port (WithWorkflowReader) — the Adapter implements both.
func buildHTTPFacade(result *application.SetupResult, repo ports.WorkflowRepository, historyStore ports.HistoryStore, storagePath string, registry *application.SessionRegistry) *application.Adapter {
	discoverer := workflowpkg.NewPackDiscovererAdapter(workflowPackSearchDirs())
	resolver := application.NewResolver(discoverer, repo)
	historySvc := application.NewHistoryService(historyStore, &cliLogger{silent: true})
	adapter := application.NewAdapter(
		result.WorkflowSvc,
		result.ExecService,
		historySvc,
		resolver,
		infraTranscript.NewNopRecorder(),
		registry,
	)
	adapter.SetRunRecorderFactory(func(runID string) (ports.Recorder, error) {
		rec, _, err := WireTranscript(runID, storagePath)
		return rec, err
	})
	return adapter
}
