package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	apptools "github.com/awf-project/cli/internal/application/tools"
	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infralogger "github.com/awf-project/cli/internal/infrastructure/logger"
	infratools "github.com/awf-project/cli/internal/infrastructure/tools"
	"github.com/awf-project/cli/internal/infrastructure/tools/builtins"
	"github.com/awf-project/cli/pkg/mcpserver"
	"github.com/spf13/cobra"
)

// Deps holds injectable dependencies for the mcp-serve subcommand.
//
// When Deps is populated (test or future in-process callers), runMCPServe uses
// OperationProviders directly for plugin_tools resolution. When Deps is empty
// (the subprocess case — ProxyService.StartForStdio spawns `awf mcp-serve`
// with no in-process state), runMCPServe self-bootstraps by calling
// initPluginSystem, which loads external plugins from the standard discovery
// paths. Either code path converges on the same registerTools call.
type Deps struct {
	PluginManager      ports.PluginManager
	OperationProviders map[string]ports.OperationProvider
}

type mcpProxyConfig struct {
	InterceptBuiltins bool                      `json:"intercept_builtins"`
	PluginTools       []apptools.PluginToolSpec `json:"plugin_tools"`
	// RootDir restricts built-in file-touching handlers (Read/Write/Edit/Glob/Grep,
	// and Bash cwd) to paths under this directory. When empty, runMCPServe defaults
	// to the subprocess's working directory (the workspace, in production wiring).
	RootDir string `json:"root_dir,omitempty"`
}

// annotationSkipFormatValidation is a Cobra command annotation key that signals
// PersistentPreRun to skip --format flag validation. Commands that communicate
// via a structured protocol (JSON-RPC, streaming) set this to avoid spurious
// os.Exit(1) calls from format validation logic intended for human-readable output.
// Using an annotation is more robust than matching c.Name() == "mcp-serve" because
// it survives command renames without a corresponding root.go change.
const annotationSkipFormatValidation = "skipFormatValidation"

func newMCPServeCommand(deps Deps) *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:    "mcp-serve",
		Hidden: true,
		Annotations: map[string]string{
			annotationSkipFormatValidation: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPServe(cmd.Context(), deps, configPath)
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "path to proxy config file")
	cmd.MarkFlagRequired("config") //nolint:errcheck,gosec // "config" was just registered; MarkFlagRequired only fails for unknown flag names

	return cmd
}

func runMCPServe(ctx context.Context, deps Deps, configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Config file missing or unreadable → user error (exit 1 per T007 error taxonomy).
		return &exitError{code: ExitUser, err: fmt.Errorf("mcp-serve: config file: %w", err)}
	}

	var cfg mcpProxyConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		// Malformed JSON config → user error (exit 1 per T007 error taxonomy).
		return &exitError{code: ExitUser, err: fmt.Errorf("mcp-serve: invalid config: %w", err)}
	}

	srv := mcpserver.New()

	if cfg.InterceptBuiltins {
		rootDir := cfg.RootDir
		if rootDir == "" {
			// Default: lock built-in file handlers to the subprocess's working directory.
			// In production wiring this is the workspace dir (proxy_service.go inherits CWD
			// from the awf parent). Without this default, an empty RootDir would mean
			// "no restriction", which would expose ~/.ssh/id_rsa et al. to prompt injection.
			if wd, wdErr := os.Getwd(); wdErr == nil {
				rootDir = wd
			}
		}
		// Inject a real shell executor so the Bash handler can execute commands.
		// Without this, p.executor is nil and the first Bash call panics, killing
		// the subprocess and causing "MCP connection closed" for all subsequent calls.
		provider := builtins.NewProvider(
			builtins.WithExecutor(executor.NewShellExecutor()),
			builtins.WithRootDir(rootDir),
		)
		defer provider.Close(context.Background()) //nolint:errcheck // Close is a no-op for the builtin provider

		tools, err := provider.ListTools(ctx)
		if err != nil {
			return fmt.Errorf("mcp-serve: listing tools: %w", err)
		}

		if regErr := registerTools(srv, provider, tools); regErr != nil {
			return fmt.Errorf("mcp-serve: registering builtin tools: %w", regErr)
		}
	}

	if len(cfg.PluginTools) > 0 {
		// Resolve the OperationProvider for plugin_tools. When Deps is populated
		// (in-process callers / tests), use the injected per-plugin map directly.
		// When Deps is empty (subprocess case), self-bootstrap via initPluginSystem
		// so that externally installed plugins are loaded from disk.
		opProvider, cleanupPlugins, resolveErr := resolveOperationProvider(ctx, deps)
		if resolveErr != nil {
			return &exitError{code: ExitExecution, err: fmt.Errorf("mcp-serve: plugin bootstrap: %w", resolveErr)}
		}
		if cleanupPlugins != nil {
			defer cleanupPlugins()
		}

		if err := registerPluginTools(ctx, srv, deps, opProvider, cfg.PluginTools); err != nil {
			return err
		}
	}

	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	if serveErr := srv.Serve(signalCtx, os.Stdin, os.Stdout); serveErr != nil {
		if signalCtx.Err() != nil {
			return nil
		}
		return &exitError{code: ExitExecution, err: fmt.Errorf("mcp-serve: %w", serveErr)}
	}
	return nil
}

// registerPluginTools registers each PluginToolSpec on srv using either the in-process
// deps map or the bootstrapped composite opProvider from initPluginSystem.
func registerPluginTools(ctx context.Context, srv *mcpserver.Server, deps Deps, opProvider ports.OperationProvider, specs []apptools.PluginToolSpec) error {
	for _, spec := range specs {
		provider, err := lookupPluginProvider(deps, opProvider, spec.Plugin)
		if err != nil {
			return err
		}

		adapter, err := infratools.NewPluginToolAdapter(spec.Plugin, provider, spec.Expose)
		if err != nil {
			return &exitError{code: ExitUser, err: fmt.Errorf("mcp-serve: plugin adapter: %w", err)}
		}

		toolList, listErr := adapter.ListTools(ctx)
		if listErr != nil {
			return &exitError{code: ExitExecution, err: fmt.Errorf("mcp-serve: listing plugin tools: %w", listErr)}
		}

		if regErr := registerTools(srv, adapter, toolList); regErr != nil {
			return &exitError{code: ExitExecution, err: fmt.Errorf("mcp-serve: registering plugin tools: %w", regErr)}
		}
	}
	return nil
}

// lookupPluginProvider returns the OperationProvider for pluginName.
// In-process path: looks up in deps.OperationProviders by name.
// Subprocess path: returns the bootstrapped composite opProvider (may be nil when no plugin
// directories exist on disk — returns UNKNOWN_PLUGIN in that case).
func lookupPluginProvider(deps Deps, opProvider ports.OperationProvider, pluginName string) (ports.OperationProvider, error) {
	if len(deps.OperationProviders) > 0 {
		p, ok := deps.OperationProviders[pluginName]
		if !ok {
			return nil, &exitError{
				code: ExitUser,
				err: fmt.Errorf(
					"mcp-serve: %s: plugin not found: %s",
					domerrors.ErrorCodeUserMCPProxyUnknownPlugin, pluginName,
				),
			}
		}
		return p, nil
	}

	if opProvider == nil {
		return nil, &exitError{
			code: ExitUser,
			err: fmt.Errorf(
				"mcp-serve: %s: plugin not found: %s (no plugin directories discovered)",
				domerrors.ErrorCodeUserMCPProxyUnknownPlugin, pluginName,
			),
		}
	}
	return opProvider, nil
}

// resolveOperationProvider returns the OperationProvider to use for plugin_tools.
// When deps.OperationProviders is populated, it returns nil (callers use the map directly).
// When empty (subprocess case), it calls initPluginSystem with the default config so that
// externally installed plugins are discovered from the standard search paths on disk.
// When no plugin directories exist on disk, Manager will be nil; callers must guard against nil.
// The returned cleanup function must be deferred when non-nil.
func resolveOperationProvider(ctx context.Context, deps Deps) (ports.OperationProvider, func(), error) {
	if len(deps.OperationProviders) > 0 {
		// In-process callers already have a populated map; no bootstrap needed.
		return nil, nil, nil
	}

	cfg := DefaultConfig()
	pluginResult, err := initPluginSystem(ctx, cfg, infralogger.NopLogger{})
	if err != nil {
		return nil, nil, fmt.Errorf("plugin system init: %w", err)
	}

	// Manager is nil when no plugin directories exist on disk (graceful degradation).
	// Callers handle nil by returning USER.MCP_PROXY.UNKNOWN_PLUGIN per plugin spec.
	return pluginResult.Manager, pluginResult.Cleanup, nil
}

// registerTools registers each tool from a provider on the MCP server with a uniform
// argument-unmarshal + dispatch + result-mapping closure. Both built-in and plugin
// adapters expose ports.ToolProvider, so this single helper covers both registration sites.
// The Description from ports.ToolDefinition is forwarded to mcpserver.ToolDefinition so that
// agents such as Gemini (which refuse opaque tools) receive a populated description field.
// Returns an error if any tool name is already registered (duplicate).
func registerTools(srv *mcpserver.Server, provider ports.ToolProvider, tools []ports.ToolDefinition) error {
	for _, tool := range tools {
		def := mcpserver.ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: portSchemaToMCP(tool.InputSchema),
		}
		name := tool.Name
		if regErr := srv.RegisterTool(def, func(callCtx context.Context, args json.RawMessage) (mcpserver.Result, error) {
			var argsMap map[string]any
			if unmarshalErr := json.Unmarshal(args, &argsMap); unmarshalErr != nil {
				return mcpserver.Result{}, fmt.Errorf("invalid args: %w", unmarshalErr)
			}
			result, callErr := provider.CallTool(callCtx, name, argsMap)
			if callErr != nil {
				return mcpserver.Result{}, callErr
			}
			return portResultToMCP(result), nil
		}); regErr != nil {
			return fmt.Errorf("register tool %q: %w", tool.Name, regErr)
		}
	}
	return nil
}

func portSchemaToMCP(m map[string]any) mcpserver.InputSchema {
	data, err := json.Marshal(m)
	if err != nil {
		return mcpserver.InputSchema{Type: "object"}
	}
	var s mcpserver.InputSchema
	if err := json.Unmarshal(data, &s); err != nil {
		return mcpserver.InputSchema{Type: "object"}
	}
	if s.Type == "" {
		s.Type = "object"
	}
	return s
}

func portResultToMCP(r *ports.ToolResult) mcpserver.Result {
	res := mcpserver.Result{IsError: r.IsError}
	for _, c := range r.Content {
		res.Content = append(res.Content, mcpserver.ContentBlock{Type: c.Type, Text: c.Text})
	}
	return res
}
