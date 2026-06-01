package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"

	"github.com/awf-project/cli/internal/application"
	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/acp"
	"github.com/awf-project/cli/internal/infrastructure/agents"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infralogger "github.com/awf-project/cli/internal/infrastructure/logger"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/roles"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/infrastructure/workflowpkg"
	"github.com/awf-project/cli/pkg/acpserver"
	"github.com/awf-project/cli/pkg/display"
)

// acpServeConfig is the on-disk configuration for the acp-serve subprocess. It is parsed
// from the project AWF config file (`.awf/config.yaml` by convention; see the ACP editor
// integration guide). The file is YAML — JSON is also accepted since JSON is a subset of
// YAML. Unknown fields (e.g. the general config's `inputs:`) are ignored.
type acpServeConfig struct {
	// WorkflowsDir scopes workflow discovery/execution to a single directory. When empty —
	// the common case for the general `.awf/config.yaml` — the standard discovery paths
	// (env / project-local `.awf/workflows/` / global) are used.
	WorkflowsDir string `json:"workflows_dir,omitempty" yaml:"workflows_dir,omitempty"`
}

func newACPServeCommand(deps Deps) *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:    "acp-serve",
		Hidden: true,
		Short:  "Start an ACP transparent agent server (stdio transport)",
		Annotations: map[string]string{
			annotationSkipFormatValidation: "true",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runACPServe(cmd.Context(), deps, configPath)
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "path to ACP server config file")
	cmd.MarkFlagRequired("config") //nolint:errcheck,gosec // "config" was just registered; MarkFlagRequired only fails for unknown flag names

	return cmd
}

// runACPServe wires the ACP transparent agent server and serves JSON-RPC 2.0 over stdio.
// deps mirrors runMCPServe for signature parity; ACP v1 exposes no plugin_tools surface,
// so it is reserved for future use rather than consumed here.
func runACPServe(ctx context.Context, _ Deps, configPath string) error {
	data, err := os.ReadFile(configPath) //nolint:gosec // configPath is an operator-supplied CLI flag
	if err != nil {
		return &exitError{code: ExitUser, err: fmt.Errorf("acp-serve: config file: %w", err)}
	}

	var cfg acpServeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return &exitError{code: ExitUser, err: fmt.Errorf("acp-serve: invalid config (expected YAML or JSON): %w", err)}
	}

	// Mi-1: validate workflows_dir at startup so the server fails fast with a clear
	// user-facing error instead of silently serving zero workflows or crashing later.
	if cfg.WorkflowsDir != "" {
		if err := validateWorkflowsDir(cfg.WorkflowsDir); err != nil {
			return err
		}
	}

	srv := acpserver.New(slog.Default())

	// Logs go to stderr so they never corrupt the stdout JSON-RPC stream.
	logger := infralogger.NewConsoleLogger(os.Stderr, infralogger.LevelInfo, false)
	repo := buildACPWorkflowRepository(cfg)

	appCfg := DefaultConfig()

	// Project config is best-effort for a long-running server (missing/invalid config must
	// not prevent serving). Only the notify backend default is consumed.
	notifyBackend := ""
	if projectCfg, projErr := loadProjectConfig(logger); projErr != nil {
		logger.Warn("acp-serve: project config not loaded, using defaults", "error", projErr)
	} else if projectCfg != nil {
		notifyBackend = projectCfg.Notify.DefaultBackend
	}

	// Plugin system (shared; graceful-degrades when no plugins installed).
	pluginResult, pErr := initPluginSystem(ctx, appCfg, logger)
	if pErr != nil {
		return &exitError{code: ExitExecution, err: fmt.Errorf("acp-serve: plugins: %w", pErr)}
	}
	defer pluginResult.Cleanup()

	// History store (shared; opened once, closed at shutdown — wrapped so per-session
	// Build cleanup does not close it).
	var historyStore ports.HistoryStore
	if hs, hErr := store.NewSQLiteHistoryStore(filepath.Join(appCfg.StoragePath, "history.db")); hErr != nil {
		logger.Warn("acp-serve: history disabled", "error", hErr)
	} else {
		historyStore = hs
		defer func() { _ = hs.Close() }()
	}

	shellExecutor := executor.NewShellExecutor()
	toolCLIExec := agents.NewExecCLIExecutor()
	masker := infralogger.NewSecretMasker()
	emitter := &acpUpdateEmitter{server: srv}

	baseOpts := []application.SetupOption{
		application.WithNotifyConfig(application.NotifyConfig{DefaultBackend: notifyBackend}),
		application.WithTemplatePaths([]string{".awf/templates", filepath.Join(appCfg.StoragePath, "templates")}),
		application.WithTracer(ports.NopTracer{}),
		application.WithAgentRoleRepository(roles.NewFilesystemAgentRoleRepository(logger)),
		application.WithToolProxy(toolCLIExec),
		application.WithPluginState(pluginResult.Service),
		application.WithPluginService(pluginResult.Service),
	}
	if pluginResult.RPCManager != nil {
		baseOpts = append(baseOpts, application.WithPluginProviders(application.PluginProviders{
			Operations: pluginResult.Manager,
			Validators: pluginResult.RPCManager.ValidatorProvider(0),
			StepTypes:  pluginResult.RPCManager.StepTypeProvider(logger),
		}))
	}
	if historyStore != nil {
		baseOpts = append(baseOpts, application.WithHistoryStore(sharedHistoryStore{HistoryStore: historyStore}))
	}

	// Bind the shutdown signal context BEFORE building the per-session factory so every
	// session-scoped emitter/reader/renderer captures the cancellable signalCtx (C2). If
	// they captured the parent ctx instead, a SIGTERM would stop Serve but leave in-flight
	// session goroutines still emitting to a closing stdout. Deriving them from signalCtx
	// makes a disconnect/shutdown stop emission as intended.
	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Per-session factory: shared base + session-scoped reader/publisher/writers/renderer.
	factory := func(sessionID string) (application.WorkflowRunner, application.ACPInputResponder, *atomic.Bool, func(), error) {
		// M3: give the user a one-time explanation when the workflow requests interactive
		// input, which the ACP server does not support yet (US2 parking is a future story).
		var inputNoticeOnce sync.Once
		reader := acp.NewACPInputReader(func() {
			inputNoticeOnce.Do(func() {
				//nolint:errcheck // best-effort user notice; EndTurnNotifier has no error return
				_ = emitter.EmitSessionUpdate(signalCtx, sessionID, "agent_message_chunk", map[string]any{
					"content": map[string]any{
						"type": "text",
						"text": "This workflow is waiting for interactive input, which the ACP server does not support yet. Cancel the prompt to abort.",
					},
				})
			})
		})

		// I2: streamed flag — set to true by writers/renderer when an emit succeeds so
		// HandleSessionPrompt can safely suppress the post-run aggregate.
		streamed := &atomic.Bool{}
		textWriter := newACPTextWriter(signalCtx, emitter, sessionID, streamed)
		sender := newACPMessageSender(emitter, sessionID, streamed)
		projector := acp.NewWorkflowEventProjector(newACPSessionNotifier(emitter, sessionID), logger)

		var publisher ports.EventPublisher = projector
		if pluginResult.EventPublisher != nil {
			publisher = acp.NewFanoutPublisher(logger, pluginResult.EventPublisher, projector)
		}

		// Isolate persisted workflow state per ACP session. Concurrent sessions running the
		// same workflow share its WorkflowID as the state-file key; a single shared store
		// would let them clobber each other's state. A per-session subdirectory keeps each
		// session's state files disjoint.
		sessionStateDir := acpSessionStateDir(sessionID)
		stateStore := store.NewJSONStore(sessionStateDir)

		opts := append([]application.SetupOption{}, baseOpts...)
		opts = append(
			opts,
			application.WithUserInputReader(reader),
			application.WithEventPublisher(publisher),
			// NOTE(F102): stdout and stderr of a workflow step are both surfaced as
			// agent_message_chunk via the same writer; the ACP protocol output does not
			// yet distinguish the two streams. Tracked as a known limitation for F102-v2.
			// See docs/ADR/018-acp-transparent-agent-server-protocol.md.
			application.WithOutputWriters(textWriter, textWriter),
			application.WithDisplayRendererFactory(func(stepID string) display.EventRenderer {
				// M-4: pass the process environment so MaskText can redact secrets
				// (API keys, passwords, tokens) before they reach the editor over the
				// ACP stream. os.Environ() is used as the source because no per-step
				// env context is available at factory construction time; it covers all
				// secrets that were exported to this process, which is the right scope
				// for a long-running server launched by the editor.
				r := acp.NewACPRenderer(stepID, sender, masker, logger, processEnvMap())
				return display.EventRenderer(r.RenderFunc(signalCtx))
			}),
		)
		res, bErr := application.NewExecutionSetup(repo, stateStore, shellExecutor, logger, opts...).Build(signalCtx)
		if bErr != nil {
			return nil, nil, nil, nil, fmt.Errorf("build session execution: %w", bErr)
		}
		// Make pack workflows runnable, not just listable: the ExecutionService resolves the
		// dispatched workflow via WorkflowSvc.GetWorkflow, which routes a "pack/workflow" name to
		// the PackDiscoverer only when one is wired. Gated identically to available-command
		// discovery so a scoped workflows_dir is honored verbatim (no pack resolution outside it).
		if cfg.WorkflowsDir == "" {
			res.WorkflowSvc.SetPackDiscoverer(workflowpkg.NewPackDiscovererAdapter(workflowPackSearchDirs()))
		}

		// C3: wrap the Build cleanup so the per-session state directory is removed when the
		// session is torn down — otherwise each session leaks a /tmp/awf-acp-states/<id>
		// subtree for the lifetime of the (long-running) server.
		//
		// M-2: RemoveAll is deferred inside the closure so that a panic inside
		// res.Cleanup() cannot skip the directory removal and leak temp state on disk.
		// The defer runs even when the panic propagates upward.
		cleanup := func() {
			defer func() {
				if rmErr := os.RemoveAll(sessionStateDir); rmErr != nil {
					logger.Warn("acp-serve: failed to remove session state dir", "dir", sessionStateDir, "error", rmErr)
				}
			}()
			res.Cleanup()
		}
		return res.ExecService, reader, streamed, cleanup, nil
	}

	sessionSvc := application.NewACPSessionService(nil, nil, repo, logger)
	sessionSvc.SetSessionUpdateEmitter(emitter)
	sessionSvc.SetRunnerFactory(factory)
	// Pack-aware available-command discovery. Wrapping the repository in a WorkflowService with a
	// PackDiscoverer makes session/new advertise installed pack workflows ("pack/workflow") as
	// slash commands — consistent with the CLI/TUI/HTTP interfaces, which all list via
	// WorkflowService.ListAllWorkflows. Gated on the standard discovery mode: when the operator
	// scopes the server to a single workflows_dir, that scope is honored verbatim and pack
	// workflows outside it are intentionally NOT surfaced (the session service then falls back to
	// the scoped repository for discovery).
	if cfg.WorkflowsDir == "" {
		provider := application.NewWorkflowService(repo, nil, nil, logger, nil)
		provider.SetPackDiscoverer(workflowpkg.NewPackDiscovererAdapter(workflowPackSearchDirs()))
		sessionSvc.SetWorkflowProvider(provider)
	}
	// I1: run every session's per-session cleanup at server shutdown.
	defer sessionSvc.Shutdown()

	srv.RegisterHandler(acpserver.MethodInitialize, makeInitializeHandler(Version))
	srv.RegisterHandler(acpserver.MethodSessionNew, adaptACPHandler(sessionSvc.HandleSessionNew))
	srv.RegisterHandler(acpserver.MethodSessionPrompt, adaptACPHandler(sessionSvc.HandleSessionPrompt))
	srv.RegisterHandler(acpserver.MethodSessionCancel, adaptACPHandler(sessionSvc.HandleSessionCancel))

	// C-1: Server.Serve requires the caller to close 'in' after Serve returns so
	// that the internal reader goroutine unblocks its Read(os.Stdin) call and exits.
	// Without this close the goroutine would block indefinitely on stdin, creating a
	// goroutine leak. The error is intentionally ignored: stdin close after Serve is
	// a best-effort cleanup and a failure here does not affect the served result.
	defer func() { _ = os.Stdin.Close() }() //nolint:errcheck // best-effort stdin cleanup; see comment above

	if serveErr := srv.Serve(signalCtx, os.Stdin, os.Stdout); serveErr != nil {
		if signalCtx.Err() != nil {
			return nil
		}
		return &exitError{code: ExitExecution, err: fmt.Errorf("acp-serve: %w", serveErr)}
	}
	return nil
}

// acpSessionStateDir returns the per-session directory used to persist workflow state for
// a single ACP session. Isolating state by session prevents concurrent sessions that run
// the same workflow (and therefore share its WorkflowID as the state-file key) from
// overwriting each other's persisted state.
//
// Session IDs are server-generated UUIDs and thus already safe, but the ID is run through
// filepath.Clean and stripped of any path separators before being joined as a defensive
// measure against path traversal if the source of the ID ever changes.
func acpSessionStateDir(sessionID string) string {
	// filepath.Clean normalizes traversal sequences ("/../.." -> "/"), then
	// filepath.Base extracts only the final path component, so no parent/subdir
	// component can reach the filepath.Join below (path-traversal safe).
	safeID := filepath.Base(filepath.Clean("/" + sessionID))
	if safeID == "." || safeID == string(filepath.Separator) || safeID == "" {
		safeID = "default"
	}
	return filepath.Join(os.TempDir(), "awf-acp-states", safeID)
}

// buildACPWorkflowRepository returns the workflow repository the server serves from.
// A configured WorkflowsDir scopes discovery to that single directory; otherwise the
// standard composite discovery paths are used.
//
// Precondition: validateWorkflowsDir must be called before this function when
// WorkflowsDir is non-empty to ensure the directory exists and is readable.
func buildACPWorkflowRepository(cfg acpServeConfig) ports.WorkflowRepository {
	if cfg.WorkflowsDir != "" {
		return repository.NewCompositeRepository([]repository.SourcedPath{
			{Path: filepath.Clean(cfg.WorkflowsDir), Source: repository.SourceLocal},
		})
	}
	return NewWorkflowRepository()
}

// validateWorkflowsDir checks that WorkflowsDir exists and is a readable directory.
// Returns an ExitUser error with a descriptive message when the check fails (Mi-1 fix).
func validateWorkflowsDir(dir string) error {
	cleaned := filepath.Clean(dir)
	info, err := os.Stat(cleaned)
	if err != nil {
		if os.IsNotExist(err) {
			return &exitError{code: ExitUser, err: fmt.Errorf("acp-serve: workflows_dir %q does not exist", cleaned)}
		}
		return &exitError{code: ExitUser, err: fmt.Errorf("acp-serve: workflows_dir %q: %w", cleaned, err)}
	}
	if !info.IsDir() {
		return &exitError{code: ExitUser, err: fmt.Errorf("acp-serve: workflows_dir %q is not a directory", cleaned)}
	}
	return nil
}

// acpUpdateEmitter streams application-layer session/update notifications to the editor
// via the JSON-RPC server's one-way Notify primitive.
type acpUpdateEmitter struct {
	server *acpserver.Server
}

func (e *acpUpdateEmitter) EmitSessionUpdate(ctx context.Context, sessionID, kind string, fields map[string]any) error {
	// ACP discriminates the SessionUpdate union with the `sessionUpdate` field. Copy the
	// caller's fields first, then set the discriminator last so a stray "sessionUpdate"
	// key in fields can never clobber it (m6).
	update := make(map[string]any, len(fields)+1)
	maps.Copy(update, fields)
	update["sessionUpdate"] = kind
	return e.server.Notify(ctx, acpserver.MethodSessionUpdate, map[string]any{
		"sessionId": sessionID,
		"update":    update,
	})
}

// makeInitializeHandler returns an ACP initialize handler that advertises the given
// version string. Accepting version as a parameter decouples the handler from the
// package-level Version variable (ldflags), making it testable without mutating
// globals and documenting the dependency explicitly (Mi-6 fix).
func makeInitializeHandler(version string) acpserver.HandlerFunc {
	return func(ctx context.Context, params json.RawMessage) (any, *acpserver.Error) {
		return handleInitialize(ctx, params, version)
	}
}

// handleInitialize responds to ACP initialize handshakes. It negotiates the protocol
// version (ADR-018): ACP versions are integers and the agent answers with the highest
// version it supports that does not exceed the client's request. A request below the
// minimum we can serve (1) is rejected as USER.ACP.PROTOCOL_VERSION_UNSUPPORTED (m5).
// agentCapabilities advertises the supported prompt content; no authMethods are
// advertised — ACP auth is out of scope for v1.
func handleInitialize(_ context.Context, params json.RawMessage, version string) (any, *acpserver.Error) {
	negotiated := acpserver.ProtocolVersion
	if len(params) > 0 {
		// protocolVersion is decoded leniently: ACP defines it as an integer, but the field
		// is captured as RawMessage so a non-integer value (older string-style versions, or
		// none at all) is tolerated rather than rejected — only a well-formed integer below
		// the minimum we can serve (1) is unsupported (m5).
		var init struct {
			ProtocolVersion json.RawMessage `json:"protocolVersion"`
		}
		if err := json.Unmarshal(params, &init); err != nil {
			return nil, &acpserver.Error{Code: acpserver.ErrInvalidParams, Message: err.Error()}
		}
		var requested int
		if json.Unmarshal(init.ProtocolVersion, &requested) == nil {
			if requested < 1 {
				// M-6: surface a human-readable message for the editor rather than
				// the raw machine code. The error code is preserved in Data so that
				// automated clients can still match it programmatically.
				return nil, &acpserver.Error{
					Code:    acpserver.ErrInvalidParams,
					Message: fmt.Sprintf("unsupported protocol version %d; minimum supported version is 1", requested),
					Data:    string(domainerrors.ErrorCodeUserACPProtocolVersionUnsupported),
				}
			}
			if requested < negotiated {
				negotiated = requested
			}
		}
	}
	return map[string]any{
		"protocolVersion": negotiated,
		"agentCapabilities": map[string]any{
			"loadSession": false,
			"promptCapabilities": map[string]any{
				"image":           false,
				"audio":           false,
				"embeddedContext": false,
			},
			"mcpCapabilities": map[string]any{
				"http": false,
				"sse":  false,
			},
		},
		"agentInfo": map[string]any{
			"name":    "awf",
			"title":   "AI Workflow CLI",
			"version": version,
		},
		// No authentication methods are advertised — ACP auth is out of scope for v1.
		"authMethods": []any{},
	}, nil
}

// processEnvMap builds a map[string]string from os.Environ() for use with
// SecretMasker.MaskText. Each entry is split on the first '=' only — values
// may themselves contain '=' characters (e.g. base64-encoded secrets).
// This helper is extracted to make the env construction independently testable.
func processEnvMap() map[string]string {
	raw := os.Environ()
	m := make(map[string]string, len(raw))
	for _, entry := range raw {
		k, v, _ := strings.Cut(entry, "=")
		if k != "" {
			m[k] = v
		}
	}
	return m
}
