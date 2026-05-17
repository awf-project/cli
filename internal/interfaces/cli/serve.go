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
	pluginResult, pluginErr := pluginmgr.InitSystem(context.Background(), pluginDirs, filepath.Join(storagePath, "plugins"), "", logger)

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

	defer func() {
		result.Cleanup()
		tracerShutdown()
		auditCleanup()
		if pluginErr == nil && pluginResult != nil {
			pluginResult.Cleanup()
		}
	}()

	result.WorkflowSvc.SetPackDiscoverer(workflowpkg.NewPackDiscovererAdapter(workflowPackSearchDirs()))

	bridge := api.NewBridge(result.WorkflowSvc, result.ExecService, result.HistorySvc)
	bridge.SetResumer(result.ExecService)
	addr := fmt.Sprintf("%s:%d", host, port)
	srv := api.NewServer(bridge, addr)

	cmd.Printf("AWF API server listening on http://%s\n", addr)
	cmd.Printf("Swagger UI: http://%s/docs\n", addr)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start(ctx)
	}()

	select {
	case serveErr := <-errCh:
		return serveErr
	case <-ctx.Done():
		cmd.Println("Shutting down server...")
		shutdownErr := srv.Shutdown(context.Background())
		cmd.Println("Server stopped.")
		return shutdownErr
	}
}
