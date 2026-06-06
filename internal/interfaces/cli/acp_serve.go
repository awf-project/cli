package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	acpinfra "github.com/awf-project/cli/internal/infrastructure/acp"
	"github.com/awf-project/cli/internal/infrastructure/agents"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infralogger "github.com/awf-project/cli/internal/infrastructure/logger"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/roles"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/infrastructure/workflowpkg"
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

	// Logs go to stderr so they never corrupt the stdout JSON-RPC stream (NFR-002).
	logger := infralogger.NewConsoleLogger(os.Stderr, infralogger.LevelInfo, false)
	// slogLogger wraps os.Stderr for SDK components that require a *slog.Logger
	// (conn.SetLogger, acpinfra.NewEmitter, acpinfra.NewRenderer). NFR-002: stdout is
	// reserved for protocol frames; all diagnostics go to stderr.
	slogLogger := newACPSDKLogger(os.Stderr)

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

	// Create the session service before the agent — the agent wraps the service and the
	// service is wired (via Set* calls) only after conn is created.
	sessionSvc := application.NewACPSessionService(nil, nil, repo, logger)

	// Agent wraps the session service and implements sdk.Agent; the Conn owns the transport.
	agent := acpinfra.NewAgent(sessionSvc)
	// Route stdin through a fresh pipe so the connection's receive goroutine blocks on
	// stdinPipeR until the forwarding goroutine (started below) writes or closes it. This
	// guarantees NewConnection's SetLogger write happens-before loggerOrDefault()'s read in
	// the SDK receive goroutine, eliminating the data race without SDK changes.
	stdinPipeR, stdinPipeW := io.Pipe()
	// NFR-002: NewConnection routes all SDK diagnostic logs to stderr (slogLogger); stdout
	// carries protocol frames only. The acpinfra.Conn wrapper keeps the SDK connection type
	// confined to internal/infrastructure/acp so this interface file never imports the SDK.
	conn := acpinfra.NewConnection(agent, os.Stdout, stdinPipeR, slogLogger)

	// Service-level emitter: used by the session service for service-scoped notifications.
	emitter := conn.NewEmitter(slogLogger)

	envMap := processEnvMap()

	// Per-session factory: shared base + session-scoped emitter/writers/renderer.
	// Extracted to buildACPSessionFactory so the ~95-line wiring is unit-testable and
	// runACPServe stays focused on lifecycle.
	factory := buildACPSessionFactory(&acpSessionFactoryDeps{
		signalCtx:          signalCtx,
		conn:               conn,
		slogLogger:         slogLogger,
		logger:             logger,
		masker:             masker,
		envMap:             envMap,
		baseOpts:           baseOpts,
		eventPublisher:     pluginResult.EventPublisher,
		repo:               repo,
		shellExecutor:      shellExecutor,
		wirePackDiscoverer: cfg.WorkflowsDir == "",
	})

	sessionSvc.SetServerContext(signalCtx)
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

	// F-1: forward real stdin into the pipe. The connection reads from stdinPipeR;
	// this goroutine is started after SetLogger so the happens-before chain is intact:
	// SetLogger write → go F() → F closes stdinPipeW → stdinPipeR.Read() returns →
	// loggerOrDefault() read. Closing stdinPipeW on EOF propagates peer disconnect.
	go func() {
		runProtocolInterceptor(signalCtx, os.Stdin, os.Stdout, stdinPipeW)
	}()

	// C-1: Close stdinPipeW (unblocks the connection's receive goroutine via the pipe)
	// and os.Stdin (stops the forwarding goroutine) when runACPServe returns for any
	// reason. Both closes are best-effort; errors are intentionally ignored.
	defer func() {
		_ = stdinPipeW.Close() //nolint:errcheck // unblock connection reader via pipe
		_ = os.Stdin.Close()   //nolint:errcheck // stop stdin forwarding goroutine
	}()

	// serveExit bounds the signal-watch goroutine's lifetime to this function. The
	// goroutine waits for a shutdown signal to close the pipe (so conn.Done() fires),
	// but if runACPServe returns for any other reason (peer disconnect, stdin EOF) it
	// must not outlive the call. Selecting on serveExit makes termination explicit
	// instead of relying solely on the deferred stop() cancelling signalCtx.
	serveExit := make(chan struct{})
	defer close(serveExit)

	// When the signal context fires (SIGTERM/SIGINT), close both ends so the connection
	// and the forwarding goroutine exit cleanly, causing conn.Done() to fire below.
	go func() {
		select {
		case <-signalCtx.Done():
		case <-serveExit:
			return
		}
		_ = stdinPipeW.Close() //nolint:errcheck // trigger conn.Done() via pipe EOF
		_ = os.Stdin.Close()   //nolint:errcheck // stop forwarding goroutine
	}()

	// Block until the connection closes (peer disconnect, stdin EOF, or signal-driven close above).
	<-conn.Done()
	// I1: run every session's per-session cleanup at server shutdown. Deferred here
	// (after conn.Done()) so it executes only once the connection is already closed,
	// ensuring the creation window is sealed before runWG is drained.
	defer sessionSvc.Shutdown()
	return nil
}

// newACPSDKLogger builds the *slog.Logger handed to the SDK connection and the ACP infra
// components. All ACP diagnostics are routed to w (os.Stderr in production) so stdout stays
// reserved for JSON-RPC protocol frames (NFR-002).
func newACPSDKLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, nil))
}

// acpSessionFactoryDeps groups the dependencies the per-session runner factory needs.
// Extracted from runACPServe so the factory wiring is independently unit-testable and the
// lifecycle function stays readable.
type acpSessionFactoryDeps struct {
	// signalCtx is the server shutdown signal context; every session-scoped component
	// captures it so a SIGTERM/disconnect stops in-flight emission (C2).
	signalCtx      context.Context //nolint:containedctx // captured shutdown ctx; session components must derive from it (C2)
	conn           *acpinfra.Conn
	slogLogger     *slog.Logger
	logger         ports.Logger
	masker         acpinfra.SecretMasker
	envMap         map[string]string
	baseOpts       []application.SetupOption
	eventPublisher ports.EventPublisher
	repo           ports.WorkflowRepository
	shellExecutor  ports.CommandExecutor
	// wirePackDiscoverer mirrors cfg.WorkflowsDir == "": when true the session resolves
	// pack workflows at run time; a scoped server honors its directory verbatim.
	wirePackDiscoverer bool
}

// acpSessionWiring holds the per-session components built by buildACPSessionWiring. The
// concrete (non-interface) fields are exposed so tests can assert wiring invariants — e.g.
// that the output writer captured the shutdown signal context (C2).
type acpSessionWiring struct {
	execService application.WorkflowRunner
	reader      application.ACPInputResponder
	streamed    *atomic.Bool
	textWriter  *acpTextWriter
	cleanup     func()
}

// buildACPSessionFactory returns the ACPRunnerFactory installed on the session service.
// Each invocation builds a fresh, self-contained set of session-scoped I/O components.
func buildACPSessionFactory(deps *acpSessionFactoryDeps) application.ACPRunnerFactory {
	return func(sessionID string) (application.WorkflowRunner, application.ACPInputResponder, *atomic.Bool, func(), error) {
		w, err := buildACPSessionWiring(deps, sessionID)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		return w.execService, w.reader, w.streamed, w.cleanup, nil
	}
}

// buildACPSessionWiring constructs the session-scoped emitter, reader, writers, renderer
// factory and execution service for one ACP session. Returned as a struct (rather than the
// bare ACPRunnerFactory tuple) so tests can inspect the wiring.
func buildACPSessionWiring(deps *acpSessionFactoryDeps, sessionID string) (*acpSessionWiring, error) {
	// Per-session emitter binds to the shared conn. Creating it inside the factory makes
	// each session's I/O components self-contained and avoids shared mutable state.
	sessionEmitter := deps.conn.NewEmitter(deps.slogLogger)
	// NOTE: the ACP permission transport (acpinfra.PermissionClient, ports.ACPClient) is
	// intentionally NOT wired here. ports.ACPClient has no consumer in F105 — the call site
	// that drives permission requests (the neutral PermissionGate) is delivered by F108
	// Axis B (spec US2). Wiring an unused client would be dead code.

	// Pass nil notifier: interactive input (conversation parking) is now fully supported
	// across ACP turns. No user-facing notice is needed; the editor manages turn state.
	reader := acpinfra.NewACPInputReader(nil)

	// I2: streamed flag — set to true by writers/renderer when an emit succeeds so
	// HandleSessionPrompt can safely suppress the post-run aggregate.
	streamed := &atomic.Bool{}
	textWriter := newACPTextWriter(deps.signalCtx, sessionEmitter, sessionID, streamed)
	// renderEmitter lets per-step renderers emit ACP SessionUpdate variants directly while
	// still flipping `streamed` on success (replaces the legacy Sender/Message DTO).
	renderEmitter := newStreamFlaggingEmitter(sessionEmitter, streamed)
	projector := acpinfra.NewWorkflowEventProjector(sessionID, sessionEmitter, deps.logger)

	var publisher ports.EventPublisher = projector
	if deps.eventPublisher != nil {
		publisher = acpinfra.NewFanoutPublisher(deps.logger, deps.eventPublisher, projector)
	}

	// Isolate persisted workflow state per ACP session so concurrent sessions running the
	// same workflow do not clobber each other's state-file (keyed by WorkflowID).
	sessionStateDir := acpSessionStateDir(sessionID)
	stateStore := store.NewJSONStore(sessionStateDir)

	opts := make([]application.SetupOption, 0, len(deps.baseOpts)+4)
	opts = append(opts, deps.baseOpts...)
	opts = append(
		opts,
		application.WithUserInputReader(reader),
		application.WithEventPublisher(publisher),
		// NOTE(F102): stdout and stderr of a workflow step are both surfaced as
		// agent_message_chunk via the same writer; the ACP protocol output does not yet
		// distinguish the two streams. Tracked as a known limitation for F102-v2.
		// See docs/ADR/018-acp-transparent-agent-server-protocol.md.
		application.WithOutputWriters(textWriter, textWriter),
		application.WithDisplayRendererFactory(func(stepID string) display.EventRenderer {
			// M-4: pass the process environment so MaskText can redact secrets before they
			// reach the editor over the ACP stream.
			r := acpinfra.NewRenderer(sessionID, stepID, renderEmitter, deps.masker, deps.slogLogger, deps.envMap)
			return display.EventRenderer(r.RenderFunc(deps.signalCtx))
		}),
	)
	res, bErr := application.NewExecutionSetup(deps.repo, stateStore, deps.shellExecutor, deps.logger, opts...).Build(deps.signalCtx)
	if bErr != nil {
		return nil, fmt.Errorf("build session execution: %w", bErr)
	}
	// Make pack workflows runnable, not just listable, when discovery is unscoped.
	if deps.wirePackDiscoverer {
		res.WorkflowSvc.SetPackDiscoverer(workflowpkg.NewPackDiscovererAdapter(workflowPackSearchDirs()))
	}

	// C3/M-2: wrap the Build cleanup so the per-session state directory is removed when the
	// session is torn down (deferred so a panic in res.Cleanup() cannot leak temp state).
	cleanup := func() {
		defer func() {
			if rmErr := os.RemoveAll(sessionStateDir); rmErr != nil {
				deps.logger.Warn("acp-serve: failed to remove session state dir", "dir", sessionStateDir, "error", rmErr)
			}
		}()
		res.Cleanup()
	}

	return &acpSessionWiring{
		execService: res.ExecService,
		reader:      reader,
		streamed:    streamed,
		textWriter:  textWriter,
		cleanup:     cleanup,
	}, nil
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

// runProtocolInterceptor reads newline-delimited JSON-RPC frames from src,
// writes protocol-level error responses to dst for invalid frames, and
// forwards valid frames to pipeW for SDK consumption. The SDK silently
// discards malformed lines without a response; this layer handles them
// (NFR-005: oversize lines also fail JSON validation and get a -32700).
func runProtocolInterceptor(ctx context.Context, src io.Reader, dst io.Writer, pipeW *io.PipeWriter) {
	const maxLineBytes = 10 * 1024 * 1024 // 10 MiB per NFR-005

	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 64*1024), maxLineBytes+1)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			_ = pipeW.Close()
			return
		default:
		}

		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		if !json.Valid(line) {
			writeJSONRPCParseError(dst)
			continue
		}

		if _, err := pipeW.Write(line); err != nil {
			return
		}
		if _, err := pipeW.Write([]byte{'\n'}); err != nil {
			return
		}
	}

	if err := scanner.Err(); err != nil {
		// ErrTooLong: line exceeded the buffer cap (>10 MiB). Send a parse error
		// before closing the pipe so the client sees a structured response.
		writeJSONRPCParseError(dst)
	}
	_ = pipeW.Close()
}

// jsonRPCParseErrorLine is the pre-marshaled JSON-RPC 2.0 parse-error response (RFC 4.2).
// id is null per spec when the request could not be parsed.
var jsonRPCParseErrorLine = func() []byte {
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage("null"),
		"error": map[string]any{
			"code":    -32700,
			"message": "parse error",
		},
	})
	return append(b, '\n')
}()

func writeJSONRPCParseError(w io.Writer) {
	_, _ = w.Write(jsonRPCParseErrorLine)
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
