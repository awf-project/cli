package application

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/validation"
)

// sessionNewParams decodes a session/new request. Per ACP the client sends only `cwd` and
// the `mcpServers` array — it does NOT supply a sessionId; the agent mints one and returns
// it in the result. Wire fields are camelCase (Zed, acp.nvim, JetBrains all speak camelCase).
type sessionNewParams struct {
	CWD        string          `json:"cwd"`
	MCPServers []MCPServerSpec `json:"mcpServers"`
}

// contentBlock is one element of a session/prompt content array.
type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// sessionPromptParams decodes a session/prompt request. Per ACP, the turn content is the
// `prompt` array of content blocks.
type sessionPromptParams struct {
	SessionID string         `json:"sessionId"`
	Prompt    []contentBlock `json:"prompt"`
}

// sessionCancelParams decodes a session/cancel request.
type sessionCancelParams struct {
	SessionID string `json:"sessionId"`
}

// InputSpec describes a single input parameter for a workflow slash command.
type InputSpec struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// WorkflowSlashCommand is the DTO emitted in available_commands_update.
// Built from ports.WorkflowInfo + workflow.Input definitions.
type WorkflowSlashCommand struct {
	Name string `json:"name"`
	// Description is REQUIRED by the ACP AvailableCommand schema: strict clients (e.g. Zed's
	// serde parser) reject the entire availableCommands array when a command omits it, which
	// blanks the slash-command suggestion menu. It is therefore never omitempty and is always
	// populated with a non-empty fallback (see ensureCommandDescriptions).
	Description    string      `json:"description"`
	RequiredInputs []InputSpec `json:"requiredInputs,omitempty"`
	OptionalInputs []InputSpec `json:"optionalInputs,omitempty"`
}

// SessionUpdateEmitter streams a session/update notification to the editor for the given
// session. The interfaces/cli wiring backs it with acpserver.Server.Notify. It is optional:
// when unset the session service runs workflows without streaming lifecycle updates.
type SessionUpdateEmitter interface {
	EmitSessionUpdate(ctx context.Context, sessionID, kind string, fields map[string]any) error
}

// WorkflowProvider lists and loads workflows from every configured source — including
// installed packs — by delegating to the application's pack-aware WorkflowService.
//
// ACPSessionService depends on this narrow port (rather than the pack-blind
// ports.WorkflowRepository) so that pack workflows ("packName/workflowName") are advertised
// as ACP slash commands in available_commands_update and resolvable by name. This mirrors
// the CLI, TUI, and HTTP interfaces, all of which list via WorkflowService.ListAllWorkflows
// (which merges ports.PackDiscoverer results) and load via WorkflowService.GetWorkflow
// (which routes a "pack/workflow" name to PackDiscoverer.LoadWorkflow). *WorkflowService
// satisfies this interface. It is optional: when unset, HandleSessionNew falls back to the
// pack-blind workflowRepo path for callers that do not inject a provider (the legacy default).
type WorkflowProvider interface {
	ListAllWorkflows(ctx context.Context) ([]workflow.WorkflowEntry, error)
	GetWorkflow(ctx context.Context, name string) (*workflow.Workflow, error)
}

// ACPSessionService owns the per-session state map and routes ACP method calls
// to the workflow runner and ConversationManager. Mirrors ConversationManager placement.
type ACPSessionService struct {
	runner       WorkflowRunner
	convMgr      *ConversationManager
	workflowRepo ports.WorkflowRepository
	// workflows is the pack-aware lister/loader. When set (via SetWorkflowProvider) it is the
	// authoritative source for available-command discovery in HandleSessionNew, superseding the
	// pack-blind workflowRepo. Optional, following the same Set* wiring convention as emitter
	// and runnerFactory below; read-only once Serve is running.
	workflows WorkflowProvider
	sessions  sync.Map // string → *ACPSession
	logger    ports.Logger

	// emitter and runnerFactory are set before Serve is called (via SetSessionUpdateEmitter
	// and SetRunnerFactory) and are read-only during the server's lifetime. They are NOT
	// safe to mutate concurrently once request handlers are running — the happens-before
	// guarantee is established by the single-threaded initialization sequence in the
	// interfaces/cli wiring layer (m-6 documentation fix). Using plain fields (rather than
	// atomic.Pointer) is intentional: the cost and complexity of atomic access would not add
	// safety after Serve starts, and adding synchronization only at Set* call sites would
	// give a false sense of security for callers that mutate after Serve.
	emitter       SessionUpdateEmitter
	runnerFactory ACPRunnerFactory

	// shutdownStarted is set atomically at the top of Shutdown to close the
	// creation window between the two-pass Range in Shutdown. HandleSessionNew
	// checks this flag and returns an explicit error immediately when it is true,
	// preventing a session created between the two passes from leaking resources
	// that Shutdown already skipped (issue #8).
	shutdownStarted atomic.Bool
}

// SetSessionUpdateEmitter wires the session/update notification sink. Optional.
func (s *ACPSessionService) SetSessionUpdateEmitter(e SessionUpdateEmitter) {
	s.emitter = e
}

// SetWorkflowProvider installs the pack-aware workflow lister/loader. When set, HandleSessionNew
// advertises every workflow returned by WorkflowProvider.ListAllWorkflows — including pack
// workflows — instead of the pack-blind workflowRepo.ListWithSource. Optional; must be called
// during the single-threaded initialization sequence before Serve, like the other Set* wiring.
func (s *ACPSessionService) SetWorkflowProvider(p WorkflowProvider) {
	s.workflows = p
}

// SetRunnerFactory installs a per-session runner factory. When set, each session builds
// its own ExecutionService (with session-scoped wiring) on first prompt. Optional: when
// unset, the shared runner passed to NewACPSessionService is used.
func (s *ACPSessionService) SetRunnerFactory(f ACPRunnerFactory) {
	s.runnerFactory = f
}

// NewACPSessionService constructs an ACPSessionService. A nil logger is replaced with a
// no-op so the handlers never panic on a missing logger. A nil execSvc leaves the runner
// unset; HandleSessionPrompt then returns a structured ErrInternal rather than panicking.
func NewACPSessionService(
	execSvc *ExecutionService,
	convMgr *ConversationManager,
	workflowRepo ports.WorkflowRepository,
	logger ports.Logger,
) *ACPSessionService {
	if logger == nil {
		logger = ports.NopLogger{}
	}
	s := &ACPSessionService{
		convMgr:      convMgr,
		workflowRepo: workflowRepo,
		logger:       logger,
	}
	// Guard against a typed-nil interface: assigning a nil *ExecutionService directly to
	// the interface field would make s.runner != nil yet panic on call.
	if execSvc != nil {
		s.runner = execSvc
	}
	return s
}

// discoverSlashCommands enumerates the workflow catalog and projects it into ACP slash commands.
// It selects the source (pack-aware provider, else pack-blind repository), loads each workflow
// best-effort for its description and input metadata, and guarantees every command carries a
// non-empty description (the ACP AvailableCommand schema requires it; a missing description makes
// strict clients reject the whole catalog and blank the slash menu).
func (s *ACPSessionService) discoverSlashCommands(ctx context.Context) ([]WorkflowSlashCommand, *ACPHandlerError) {
	commands, loadNames, loadWorkflow, derr := s.workflowCatalog(ctx)
	if derr != nil {
		return nil, derr
	}
	s.loadCommandMetadata(ctx, commands, loadNames, loadWorkflow)

	// ACP requires a non-empty description per command; fall back to the command name for any
	// workflow that declares none, so a strict client does not reject the whole catalog.
	for i := range commands {
		if commands[i].Description == "" {
			commands[i].Description = commands[i].Name
		}
	}
	return commands, nil
}

// workflowCatalog resolves the advertised slash commands and the per-workflow loader. It prefers
// the pack-aware WorkflowProvider (which merges installed pack workflows and routes "pack/workflow"
// names), falling back to the pack-blind workflowRepo for callers that do not inject a provider.
//
// The returned commands carry the slash-safe wire names advertised to the editor; loadNames carry
// the internal names used to load each workflow's metadata (pack workflows differ: the wire name
// uses a ':' namespace separator while the internal name keeps the '/' that GetWorkflow routes on).
// The two slices are index-aligned with the returned loader.
func (s *ACPSessionService) workflowCatalog(ctx context.Context) (commands []WorkflowSlashCommand, loadNames []string, loadWorkflow func(context.Context, string) (*workflow.Workflow, error), derr *ACPHandlerError) {
	switch {
	case s.workflows != nil:
		entries, err := s.workflows.ListAllWorkflows(ctx)
		if err != nil {
			// Log the detail server-side; never surface raw infra errors to the caller (M5a fix).
			s.logger.Warn("session/new: workflow discovery failed", "error", err)
			return nil, nil, nil, acpInternal("workflow discovery failed")
		}
		commands = make([]WorkflowSlashCommand, len(entries))
		loadNames = make([]string, len(entries))
		for i, e := range entries {
			// Advertise the slash-safe command name (':' namespace separator for pack workflows
			// whose internal name is "pack/workflow"); a '/' in the name would break the editor's
			// slash-command menu. Seed the description from the entry (pack manifest summary or the
			// local description ListAllWorkflows populated); loadCommandMetadata upgrades it to the
			// canonical workflow description and adds input metadata when available.
			commands[i] = WorkflowSlashCommand{Name: acpCommandName(e.Name), Description: e.Description}
			loadNames[i] = e.Name
		}
		return commands, loadNames, s.workflows.GetWorkflow, nil
	case s.workflowRepo != nil:
		infos, err := s.workflowRepo.ListWithSource(ctx)
		if err != nil {
			s.logger.Warn("session/new: workflow discovery failed", "error", err)
			return nil, nil, nil, acpInternal("workflow discovery failed")
		}
		commands = make([]WorkflowSlashCommand, len(infos))
		loadNames = make([]string, len(infos))
		for i, info := range infos {
			commands[i] = WorkflowSlashCommand{Name: info.Name}
			loadNames[i] = info.Name
		}
		return commands, loadNames, s.workflowRepo.Load, nil
	default:
		return nil, nil, nil, acpInternal("workflow repository not configured")
	}
}

// loadCommandMetadata loads each command's workflow best-effort (bounded to 8 concurrent readers)
// to populate its description and input metadata, writing results by index to preserve order.
// Errors are best-effort (skip + log) so a single unreadable workflow does not abort the catalog.
func (s *ACPSessionService) loadCommandMetadata(ctx context.Context, commands []WorkflowSlashCommand, loadNames []string, loadWorkflow func(context.Context, string) (*workflow.Workflow, error)) {
	// A plain WaitGroup + semaphore is used rather than errgroup since errors are not propagated.
	const maxParallelLoads = 8
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxParallelLoads)
	for i := range commands {
		name := loadNames[i]
		wg.Go(func() {
			// Issue #2: acquire the semaphore with a ctx-aware select so that a cancelled
			// context does not leave this goroutine blocked forever waiting for a slot.
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			// Respect context cancellation before issuing the Load; if ctx is already done
			// after acquiring the semaphore we skip the I/O operation rather than racing it.
			select {
			case <-ctx.Done():
				return
			default:
			}

			wf, loadErr := loadWorkflow(ctx, name)
			if loadErr != nil {
				s.logger.Warn("session/new: workflow load failed", "workflow", name, "error", loadErr)
				return // best-effort: skip rather than aborting session/new
			}
			if wf != nil {
				// Only overwrite the seeded description when the loaded workflow actually has one,
				// so a pack entry's manifest summary is not blanked by an empty wf.Description.
				if wf.Description != "" {
					commands[i].Description = wf.Description
				}
				commands[i].RequiredInputs, commands[i].OptionalInputs = splitWorkflowInputs(wf.Inputs)
			}
		})
	}
	wg.Wait()
}

// HandleSessionNew handles a session/new request.
// The transport-neutral *ACPHandlerError is lifted to acpserver.HandlerFunc by the
// interfaces/cli adapter (adaptACPHandler).
func (s *ACPSessionService) HandleSessionNew(ctx context.Context, params json.RawMessage) (any, *ACPHandlerError) {
	// Issue #8: reject session creation immediately if Shutdown is already in progress.
	// This closes the creation window between the two-pass Range in Shutdown — a session
	// created after Phase 1 (cancel all) but before Phase 2 (wait + cleanup) would have
	// its resources leaked because Phase 2 already skipped it.
	if s.shutdownStarted.Load() {
		return nil, acpInternal("server is shutting down")
	}

	var p sessionNewParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, acpInvalidParams(err.Error())
	}

	commands, derr := s.discoverSlashCommands(ctx)
	if derr != nil {
		return nil, derr
	}

	// The agent mints the sessionId (ACP: the client does not supply one).
	sessionID := "sess_" + uuid.NewString()

	// Store editor-provided MCP servers, keyed by name; editor entry wins on collision (ADR-018).
	mcpServers := make(map[string]MCPServerSpec, len(p.MCPServers))
	for _, m := range p.MCPServers {
		mcpServers[m.Name] = m
	}

	session := &ACPSession{
		ID:         sessionID,
		CWD:        p.CWD,
		MCPServers: mcpServers,
	}
	s.sessions.Store(sessionID, session)

	s.logger.Debug("session/new: session created", "sessionId", sessionID, "commands", len(commands))

	// Advertise the workflow slash commands as an ACP available_commands_update notification
	// (the canonical channel), in addition to returning them in the result for clients that
	// read it inline.
	s.emitAvailableCommands(ctx, sessionID, commands)

	return map[string]any{
		"sessionId": sessionID,
		"commands":  commands,
	}, nil
}

// emitAvailableCommands streams the slash-command catalog as an ACP
// available_commands_update session/update notification. Best-effort.
func (s *ACPSessionService) emitAvailableCommands(ctx context.Context, sessionID string, commands []WorkflowSlashCommand) {
	if s.emitter == nil {
		return
	}
	// WorkflowSlashCommand is already JSON-serializable with the correct wire tags
	// (including requiredInputs/optionalInputs), so emit the catalog directly rather
	// than re-mapping into []map[string]any (which dropped the input metadata).
	if err := s.emitter.EmitSessionUpdate(ctx, sessionID, "available_commands_update", map[string]any{
		"availableCommands": commands,
	}); err != nil {
		s.logger.Warn("session/new: available_commands_update emit failed", "sessionId", sessionID, "error", err)
	}
}

// ensureRunner returns the session's WorkflowRunner. With a factory configured, it builds
// the runner once per session (caching it on the session) and records the session's input
// reader; otherwise it falls back to the shared s.runner.
//
// Construction is guarded by session.runnerMu (not sync.Once): a factory call that fails is
// not memoized, so the next prompt retries the build rather than leaving the session
// permanently bricked.
func (s *ACPSessionService) ensureRunner(session *ACPSession) (WorkflowRunner, *ACPHandlerError) {
	if s.runnerFactory == nil {
		if s.runner == nil {
			return nil, acpInternal("workflow runner not configured")
		}
		return s.runner, nil
	}
	session.runnerMu.Lock()
	defer session.runnerMu.Unlock()
	if session.runnerBuilt {
		return session.runner, nil
	}
	runner, reader, streamed, cleanup, err := s.runnerFactory(session.ID)
	if err != nil {
		// Not memoized: a later prompt retries the factory.
		s.logger.Warn("ensureRunner: runner factory failed", "sessionId", session.ID, "error", err)
		return nil, acpInternal("failed to initialize session runner")
	}
	session.runner = runner
	// Store via atomic.Pointer[inputReaderHolder] so reads in HandleSessionPrompt are
	// race-free (M7 fix). The holder wrapper avoids the pointer-on-interface anti-pattern:
	// storing &reader (pointer-to-interface) is unsafe because the interface slot is not
	// atomic; wrapping in inputReaderHolder gives us a stable concrete pointer (C-2 fix).
	if reader != nil {
		session.inputReader.Store(&inputReaderHolder{r: reader})
	}
	if streamed != nil {
		session.streamed.Store(streamed)
	}
	session.runnerCleanup = cleanup
	session.runnerBuilt = true

	// CRITIQUE-3: wire the reader's park hooks to this session's parked-turn counter so a
	// continuation prompt routes to InputReader.Respond. The same *ACPInputReader instance
	// set as inputReader is the one whose hooks bump the counter, keeping the dormant
	// parking branch in HandleSessionPrompt live in production.
	// Use reader directly (not loaded from the atomic) because we are still under runnerMu
	// and reader was just validated non-nil above (C-2 fix: no pointer-on-interface load needed).
	if reader != nil {
		reader.SetParkHooks(
			func() {
				session.ParkedTurnCount.Add(1)
				// Signal the waiting turn that the workflow has parked awaiting the next
				// user turn, so HandleSessionPrompt returns end_turn (the editor re-enables
				// input). The send is non-blocking (parkedCh is buffered cap 1) so the
				// workflow goroutine is never blocked, and reads session.run dynamically so
				// the same hook serves every run of this session (ensureRunner runs once).
				if run := session.run.Load(); run != nil {
					select {
					case run.parkedCh <- struct{}{}:
					default:
					}
				}
			},
			func() { session.ParkedTurnCount.Add(-1) },
		)
	}
	return session.runner, nil
}

// HandleSessionPrompt handles a session/prompt request.
// The transport-neutral *ACPHandlerError is lifted to acpserver.HandlerFunc by the
// interfaces/cli adapter (adaptACPHandler).
func (s *ACPSessionService) HandleSessionPrompt(ctx context.Context, params json.RawMessage) (any, *ACPHandlerError) {
	var p sessionPromptParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, acpInvalidParams(err.Error())
	}

	session, acpErr := s.lookupSession(p.SessionID)
	if acpErr != nil {
		return nil, acpErr
	}

	// Reject concurrent prompts on the same session.
	//
	// NOTE: InFlight is released by the deferred Store(false) when this handler returns,
	// which the JSON-RPC server schedules *before* it writes this turn's response frame
	// (the server only serializes the write, it does not gate the InFlight reset on it).
	// A second prompt arriving in that narrow window is therefore admitted; its own
	// notifications may interleave with the tail of this turn's response. This is
	// acceptable for ACP (each turn carries its own sessionId/stopReason) and the
	// alternative — holding InFlight until the frame is on the wire — is not expressible
	// without the handler owning the write path. Documented rather than reworked.
	if !session.InFlight.CompareAndSwap(false, true) {
		// C-3: message is human-readable; the machine code goes to the Data field so
		// editors display a meaningful string instead of "USER.ACP.PROMPT_IN_FLIGHT".
		return nil, acpInvalidParamsWithData(
			"a prompt is already in flight for this session; wait for it to complete before sending another",
			string(domainerrors.ErrorCodeUserACPPromptInFlight),
		)
	}
	defer session.InFlight.Store(false)

	text, flattenErr := flattenContentBlocks(p.Prompt)
	if flattenErr != nil {
		// Unsupported blocks: tell the user why (as an agent message) and end the turn with
		// a valid ACP stop reason. Send a human-readable message to the editor; the machine
		// code (ErrorCodeUserACPUnsupportedBlock) is not part of the visible text — it is only
		// relevant at the protocol/logging level (m-2 fix).
		s.sendAgentText(ctx, p.SessionID, fmt.Sprintf("Unsupported content: %s", flattenErr.Error()))
		return promptStop("end_turn"), nil
	}

	// Continuation turn: a workflow goroutine is already parked on the InputReader, so route
	// the editor's text to it rather than starting a new workflow (US2 conversation parking).
	// inputReader is read via atomic.Pointer so this is race-free with ensureRunner (M7 fix).
	//
	// INVARIANT: if ParkedTurnCount > 0, inputReader MUST be non-nil. Both fields are written
	// together in ensureRunner (inputReader is stored first, then the park hooks that bump
	// ParkedTurnCount are wired). A non-nil ParkedTurnCount with a nil inputReader signals a
	// broken wiring in the factory — guard explicitly rather than falling through into
	// parseSlashCommand which would treat a continuation text as a new slash command.
	if parkedCount := session.ParkedTurnCount.Load(); parkedCount > 0 {
		// Load the holder via atomic.Pointer[inputReaderHolder]; a nil holder means the
		// factory never stored a reader, which violates the invariant documented below
		// (C-2 fix: holder wrapper eliminates pointer-on-interface indirection).
		h := session.inputReader.Load()
		if h == nil {
			// Invariant violation: parked turn count is positive but no input reader is
			// registered. This indicates a factory wiring bug (reader was never stored) and
			// cannot be recovered by the current prompt — report internal error so the editor
			// surfaces the failure rather than silently misrouting the continuation text.
			s.logger.Warn(
				"session/prompt: invariant violation — parked turn but no input reader",
				"sessionId", p.SessionID,
				"parkedCount", parkedCount,
			)
			return nil, acpInternal("session input reader not available")
		}
		// The run goroutine must exist whenever a turn is parked: it is published in
		// session.run on first dispatch, before the park hook can ever fire. A nil run with
		// a positive ParkedTurnCount is the same class of factory-wiring bug as a nil reader.
		run := session.run.Load()
		if run == nil {
			s.logger.Warn(
				"session/prompt: invariant violation — parked turn but no run state",
				"sessionId", p.SessionID,
				"parkedCount", parkedCount,
			)
			return nil, acpInternal("session run state not available")
		}
		// Route the editor's text to the parked workflow goroutine, then wait for the turn
		// to resolve: the workflow either parks again (→ end_turn) or completes (→ output).
		h.r.Respond(text)
		return s.waitTurn(ctx, session, run), nil
	}

	// First dispatch: the prompt must name a workflow via a leading /<slash-command>.
	workflowName, inputs, parseErr := parseSlashCommand(text)
	if parseErr != nil {
		// Send a human-readable message to the editor; the machine code (ErrorCodeUserACPInvalidPrompt)
		// is not part of the visible text — mixing machine codes into displayed messages makes
		// the UI noisy and confusing for end users (m-2 fix).
		s.sendAgentText(ctx, p.SessionID, fmt.Sprintf("Invalid prompt: %s", parseErr))
		return promptStop("end_turn"), nil
	}

	// US2 conversation parking — run the workflow on its OWN goroutine so this handler can
	// return a stopReason while the workflow is still parked, letting the editor re-enable its
	// input field. The synchronous alternative blocked the turn until the whole workflow
	// finished, which deadlocked any workflow that waits for user input: the turn never ended,
	// the editor stayed disabled, and the awaited input could never be sent. This mirrors the
	// TUI, which runs the workflow async (RunWorkflowAsync) and signals InputRequestedMsg when
	// the ConversationManager parks.
	//
	// Ordering contract (issue #1): create the cancel func and register it via setCancel
	// BEFORE runWG.Add(1), so a concurrent Shutdown that observes a positive runWG always has
	// a non-nil cancelFn to interrupt. Unlike the old synchronous handler, cancel() is owned by
	// the run goroutine (which outlives this call) and is therefore NOT deferred here.
	runCtx, cancel := context.WithCancel(ctx)
	session.setCancel(cancel)

	// runWG.Add(1) BEFORE ensureRunner so Shutdown's runWG.Wait() covers the runner build
	// (C1 fix): without this, Shutdown could observe runWG==0 and read session.runnerCleanup
	// while ensureRunner is concurrently writing it. Done() is balanced explicitly on the
	// ensureRunner error path and deferred inside the run goroutine on the success path.
	session.runWG.Add(1)
	runner, runnerErr := s.ensureRunner(session)
	if runnerErr != nil {
		session.runWG.Done() // balance Add(1): no run goroutine was started.
		cancel()
		return nil, runnerErr
	}

	// Reset the per-run streamed flag so suppression logic reflects this run only.
	// Read via atomic.Pointer so the reset is race-free with ensureRunner (M7 fix).
	if sp := session.streamed.Load(); sp != nil {
		sp.Store(false)
	}

	s.logger.Debug("session/prompt: dispatching", "sessionId", p.SessionID, "workflow", workflowName, "inputs", len(inputs))

	// Publish the run's coordination state BEFORE launching the goroutine so the park hook
	// (which reads session.run) can deliver a park signal as soon as the workflow blocks on
	// ReadInput. A completed run is left in session.run (doneCh closed) until the next dispatch.
	run := &acpRun{
		parkedCh:     make(chan struct{}, 1),
		doneCh:       make(chan struct{}),
		workflowName: workflowName,
	}
	session.run.Store(run)

	// NOTE: this is intentionally a manual Add(1)/go/Done() rather than runWG.Go — the Add(1)
	// is hoisted above ensureRunner (C1 fix) so Shutdown's runWG.Wait() covers the runner
	// build. runWG.Go would Add only at goroutine launch (after the build), reopening the
	// Shutdown-vs-build race. Done() is deferred inside the goroutine below.
	go func() {
		defer session.runWG.Done()
		defer cancel()
		execCtx, runErr := runner.Run(runCtx, workflowName, inputs)
		// Record the outcome BEFORE closing doneCh; waitTurn reads it only after <-doneCh,
		// so the close establishes the happens-before relationship (no extra locking).
		run.execCtx = execCtx
		run.runErr = runErr
		run.cancelled = runCtx.Err() != nil
		session.execCtx.Store(execCtx)
		close(run.doneCh)
	}()

	return s.waitTurn(ctx, session, run), nil
}

// waitTurn blocks until the in-flight run resolves the current ACP turn: the workflow parks
// awaiting the next user turn (→ end_turn, the run goroutine stays alive), the run completes
// (→ its output/error/cancellation via finishedTurn), or the server context is cancelled
// (→ cancelled). It is the application-layer analog of the TUI's InputRequestedMsg handling:
// a park ends the turn so the editor re-enables input, and the next session/prompt resumes the
// same run by routing its text to the parked reader via Respond.
func (s *ACPSessionService) waitTurn(ctx context.Context, session *ACPSession, run *acpRun) any {
	select {
	case <-run.parkedCh:
		return promptStop("end_turn")
	case <-run.doneCh:
		return s.finishedTurn(ctx, session, run)
	case <-ctx.Done():
		// Server shutting down; Shutdown cancels and drains the run goroutine separately.
		return promptStop("cancelled")
	}
}

// finishedTurn builds the terminal result for a completed run and streams its outcome (output,
// error, or cancellation) back to the editor as agent text so the user always sees a result
// instead of a silent end_turn. The run's outcome fields are safe to read here: they were
// written before close(doneCh), which waitTurn has already observed.
func (s *ACPSessionService) finishedTurn(ctx context.Context, session *ACPSession, run *acpRun) any {
	switch {
	case run.cancelled:
		s.sendAgentText(ctx, session.ID, fmt.Sprintf("Workflow %q cancelled.", run.workflowName))
		return promptStop("cancelled")
	case run.runErr != nil:
		s.logger.Debug("session/prompt: workflow run failed", "workflow", run.workflowName, "error", run.runErr)
		s.sendAgentText(ctx, session.ID, fmt.Sprintf("Workflow %q failed: %s", run.workflowName, run.runErr))
		return promptStop("end_turn")
	default:
		out := workflowOutputText(run.execCtx)
		// streamed is read via atomic.Pointer so this is race-free with ensureRunner (M7 fix).
		streamedFlag := session.streamed.Load()
		switch {
		case streamedFlag != nil && streamedFlag.Load():
			// Output was already delivered live (and confirmed by at least one successful emit)
			// via the session's output writers / renderer. Do not re-send the aggregate.
		case strings.TrimSpace(out) == "":
			s.sendAgentText(ctx, session.ID, fmt.Sprintf("Workflow %q completed.", run.workflowName))
		default:
			s.sendAgentText(ctx, session.ID, out)
		}
		return promptStop("end_turn")
	}
}

// sendAgentText streams a text chunk to the editor as an ACP agent_message_chunk
// session/update. Best-effort: a nil emitter or empty text is a no-op.
func (s *ACPSessionService) sendAgentText(ctx context.Context, sessionID, text string) {
	if s.emitter == nil || text == "" {
		return
	}
	if err := s.emitter.EmitSessionUpdate(ctx, sessionID, "agent_message_chunk", map[string]any{
		"content": map[string]any{"type": "text", "text": text},
	}); err != nil {
		s.logger.Warn("session/prompt: agent_message_chunk emit failed", "sessionId", sessionID, "error", err)
	}
}

// workflowOutputText collects the non-empty step outputs of a completed execution into a
// single text blob for display in the editor.
//
// GetAllStepStates returns a map (random iteration order), which would make the aggregated
// response non-deterministic. To produce a stable, meaningful ordering we sort the step
// names by their CompletedAt timestamp (execution order), falling back to alphabetical for
// steps that share a timestamp or have a zero CompletedAt — this keeps output deterministic
// regardless of map iteration order (MINEUR-3).
func workflowOutputText(execCtx *workflow.ExecutionContext) string {
	if execCtx == nil {
		return ""
	}
	states := execCtx.GetAllStepStates()
	// Snapshot (name, output, completedAt) once so sorting does not re-index the map on
	// every comparison, then order by execution time (CompletedAt), falling back to the
	// step name for ties / zero timestamps to keep the aggregate deterministic (MINEUR-3).
	type stepOutput struct {
		name        string
		output      string
		completedAt time.Time
	}
	steps := make([]stepOutput, 0, len(states))
	for name := range states {
		// Single map lookup + local bind avoids both the double lookup (MINEUR-5)
		// and the per-iteration range-value copy of the large StepState (gocritic).
		state := states[name]
		steps = append(steps, stepOutput{name: name, output: state.Output, completedAt: state.CompletedAt})
	}
	slices.SortFunc(steps, func(a, b stepOutput) int {
		if !a.completedAt.Equal(b.completedAt) {
			return a.completedAt.Compare(b.completedAt)
		}
		return strings.Compare(a.name, b.name)
	})
	var parts []string
	for i := range steps {
		out := strings.TrimRight(steps[i].output, "\n")
		if strings.TrimSpace(out) != "" {
			parts = append(parts, out)
		}
	}
	return strings.Join(parts, "\n")
}

// HandleSessionCancel handles a session/cancel request.
// The transport-neutral *ACPHandlerError is lifted to acpserver.HandlerFunc by the
// interfaces/cli adapter (adaptACPHandler).
func (s *ACPSessionService) HandleSessionCancel(ctx context.Context, params json.RawMessage) (any, *ACPHandlerError) {
	var p sessionCancelParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, acpInvalidParams(err.Error())
	}

	session, acpErr := s.lookupSession(p.SessionID)
	if acpErr != nil {
		return nil, acpErr
	}

	session.cancel()
	s.logger.Debug("session/cancel: cancelled", "sessionId", p.SessionID)

	return promptStop("cancelled"), nil
}

// Shutdown releases every session's per-session resources (the cleanup returned by the
// runner factory). Safe to call once at server shutdown; idempotent on sessions without
// a factory-built runner.
//
// Ordering matters (CRITIQUE-1): the JSON-RPC server's wait group only covers request
// handlers, not the internal goroutines an ExecutionService spawns. So Shutdown must
// (1) set shutdownStarted to close the session-creation window (issue #8),
// (2) cancel every session's run context to interrupt in-flight workflows,
// (3) wait for each session's run goroutine to actually return (runWG), and only then
// (4) invoke the per-session cleanup — otherwise cleanup could close SQLite/temp
//
//	resources a workflow is still using.
func (s *ACPSessionService) Shutdown() {
	// Issue #8: mark shutdown started so HandleSessionNew rejects new sessions immediately.
	// This must happen before either Range pass to close the window where a session created
	// between the two passes would escape both the cancel sweep and the cleanup sweep.
	s.shutdownStarted.Store(true)

	// Phase 1: cancel all in-flight runs.
	s.sessions.Range(func(_, v any) bool {
		if session, ok := v.(*ACPSession); ok {
			session.cancel()
		}
		return true
	})
	// Phase 2: wait for each run to finish, then release its resources and remove the
	// session from the map (C2 fix — prevents unbounded memory growth across many sessions).
	s.sessions.Range(func(k, v any) bool {
		if session, ok := v.(*ACPSession); ok {
			session.runWG.Wait()
			if session.runnerCleanup != nil {
				session.runnerCleanup()
			}
		}
		s.sessions.Delete(k)
		return true
	})
}

// lookupSession resolves a session by ID, returning USER.ACP.UNKNOWN_SESSION when absent.
func (s *ACPSessionService) lookupSession(sessionID string) (*ACPSession, *ACPHandlerError) {
	val, ok := s.sessions.Load(sessionID)
	if !ok {
		// C-3: message is human-readable; the machine code goes to the Data field so
		// editors display a meaningful string instead of "USER.ACP.UNKNOWN_SESSION".
		return nil, acpInvalidParamsWithData(
			fmt.Sprintf("unknown session %q; send a session/new request first", sessionID),
			string(domainerrors.ErrorCodeUserACPUnknownSession),
		)
	}
	session, typeOK := val.(*ACPSession)
	if !typeOK {
		return nil, acpInternal("corrupted session state")
	}
	return session, nil
}

// promptResult is the typed result envelope for session/prompt and session/cancel responses.
// Using a named struct instead of map[string]any prevents accidental key misspellings and
// makes the wire format explicit. The json tag preserves the camelCase ACP wire key.
type promptResult struct {
	StopReason string `json:"stopReason"`
}

// promptStop builds the session/prompt result envelope carrying a stop reason.
func promptStop(reason string) promptResult {
	return promptResult{StopReason: reason}
}

// maxPromptBytes is the upper bound on prompt text accepted by parseSlashCommand.
// A 1 MiB cap prevents tokenizePrompt from consuming unbounded memory on a malicious
// or misbehaving editor client that sends an arbitrarily large prompt (m-4 fix).
const maxPromptBytes = 1 << 20 // 1 MiB

// parseSlashCommand extracts the workflow name and its inputs from a prompt whose first
// token is a /<workflow> slash command. The leading "/" selects the workflow; the remaining
// tokens carry inputs as key=value pairs in any of the forms accepted by extractInputPairs.
// The prompt is tokenized shell-style (single/double quotes group their contents and are
// stripped), so quoted values may contain spaces — parity with how the CLI's shell tokenizes
// --input values. No @prompts/ resolution is performed (ACP editors send literal values).
// Returns an error immediately when len(text) > maxPromptBytes without tokenizing.
func parseSlashCommand(text string) (name string, inputs map[string]any, err error) {
	if len(text) > maxPromptBytes {
		return "", nil, fmt.Errorf("prompt too large (%d bytes, max %d)", len(text), maxPromptBytes)
	}
	tokens := tokenizePrompt(text)
	if len(tokens) == 0 || !strings.HasPrefix(tokens[0], "/") {
		return "", nil, fmt.Errorf("prompt must begin with a /<workflow> slash command")
	}
	name = strings.TrimPrefix(tokens[0], "/")
	if name == "" {
		return "", nil, fmt.Errorf("empty slash command")
	}

	// Map the advertised pack namespace separator back to the internal "pack/workflow" form.
	// Pack workflows are advertised as "pack:workflow" (slash-safe for the editor menu); rewriting
	// the first ':' to '/' restores the name GetWorkflow / the runner route on. A hand-typed
	// "/pack/workflow" (already using '/') is unaffected and still works.
	name = strings.Replace(name, acpPackNamespaceSeparator, "/", 1)

	// C-1: validate each path component through the canonical authority (pkg/validation.ValidateName
	// which enforces ^[a-z][a-z0-9-]*$). This is stricter than the old artisanal guards
	// (HasPrefix "/", Contains "..") and makes path-traversal structurally impossible because
	// the regex rejects ".", "/", "..", and any uppercase or special characters.
	// The pack/workflow separator "/" is handled by splitting: "mypack/myworkflow" → ["mypack","myworkflow"].
	// A plain workflow name (no "/") is validated as a single component.
	//
	// Issue #11: the component role (pack vs workflow) is included in the error message so
	// the editor surfaces which part of "pack/workflow" failed validation rather than just
	// showing the full name. With a plain workflow name the role is "workflow".
	components := strings.SplitN(name, "/", 2)
	componentRoles := [2]string{"pack", "workflow"} // index mirrors SplitN position
	for i, component := range components {
		if validateErr := validation.ValidateName(component); validateErr != nil {
			role := componentRoles[i]
			if len(components) == 1 {
				// Single component: no pack separator — role is simply "workflow".
				role = "workflow"
			}
			return "", nil, fmt.Errorf("invalid %s name %q in slash command %q: %w", role, component, name, validateErr)
		}
	}

	inputs, err = parseInputPairs(extractInputPairs(tokens[1:]))
	if err != nil {
		return "", nil, err
	}
	return name, inputs, nil
}

// acpPackNamespaceSeparator is the slash-safe separator used in the ACP slash-command name of a
// pack workflow. The internal name is "pack/workflow"; '/' is the editor's slash-command trigger
// and breaks its command menu, so the wire name uses ':' ("pack:workflow"). parseSlashCommand
// performs the inverse mapping on invocation.
const acpPackNamespaceSeparator = ":"

// acpCommandName converts an internal workflow name to its ACP slash-command (wire) form. A pack
// workflow "pack/workflow" is exposed as "pack:workflow"; only the first '/' is rewritten so the
// pack and workflow components stay intact. Local/global names (no '/') are returned unchanged.
func acpCommandName(internal string) string {
	return strings.Replace(internal, "/", acpPackNamespaceSeparator, 1)
}

// splitWorkflowInputs partitions workflow inputs into required and optional InputSpecs.
func splitWorkflowInputs(inputs []workflow.Input) (required, optional []InputSpec) {
	for i := range inputs {
		spec := InputSpec{Name: inputs[i].Name, Type: inputs[i].Type, Description: inputs[i].Description}
		if inputs[i].Required {
			required = append(required, spec)
		} else {
			optional = append(optional, spec)
		}
	}
	return required, optional
}

// flattenContentBlocks concatenates text and resource_link blocks into a single string.
// Returns ErrUnsupportedContentBlock (wrapping a human-readable message) for image, audio,
// or embedded resource blocks so callers can use errors.Is for typed dispatch while still
// surfacing a descriptive message to the editor.
func flattenContentBlocks(blocks []contentBlock) (text string, err error) {
	var parts []string
	for _, block := range blocks {
		switch block.Type {
		case "text", "resource_link":
			parts = append(parts, block.Text)
		case "image", "audio", "resource":
			return "", fmt.Errorf("%w: %s blocks are not supported", ErrUnsupportedContentBlock, block.Type)
		}
	}
	return strings.Join(parts, "\n"), nil
}

// extractInputFlags extracts key=value strings from --input=key=value tokens in text.
// extractInputPairs collects key=value input pairs from the post-command tokens. Three
// forms are accepted, in order of preference:
//
//	key=value          bare pair (no flag needed) — the recommended ACP form
//	--input=key=value  CLI "=" form
//	--input key=value  CLI space form (consumes the following token)
//
// Tokens beginning with "--" other than --input are treated as unrecognized flags and
// ignored; any other token without an "=" is ignored (it is not an input pair). The
// returned slice is handed to parseInputPairs for key/value splitting and validation.
func extractInputPairs(tokens []string) []string {
	const flag = "--input"
	const flagEq = "--input="
	var pairs []string
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		switch {
		case tok == flag:
			if i+1 < len(tokens) {
				pairs = append(pairs, tokens[i+1])
				i++
			}
		case strings.HasPrefix(tok, flagEq):
			pairs = append(pairs, strings.TrimPrefix(tok, flagEq))
		case strings.HasPrefix(tok, "--"):
			// Unrecognized flag (only --input is supported): ignore.
		case strings.Contains(tok, "="):
			pairs = append(pairs, tok)
		default:
			// Non-pair token (no "="): ignore.
		}
	}
	return pairs
}

// tokenizePrompt splits a slash-command prompt into tokens, honoring single and double
// quotes the way a shell does: a quoted span is kept within its token and the surrounding
// quotes are stripped, so `name="hello world"` becomes the single token `name=hello world`.
// Unterminated quotes are tolerant — the remaining text is flushed as the final token. This
// gives ACP slash commands parity with how the CLI's shell tokenizes --input values.
func tokenizePrompt(text string) []string {
	var tokens []string
	var cur strings.Builder
	inToken := false
	var quote rune // 0 when not inside a quote; '\'' or '"' otherwise

	flush := func() {
		if inToken {
			tokens = append(tokens, cur.String())
			cur.Reset()
			inToken = false
		}
	}

	for _, r := range text {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0 // closing quote: drop it
			} else {
				cur.WriteRune(r)
			}
			inToken = true
		case r == '\'' || r == '"':
			quote = r // opening quote: drop it
			inToken = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
		default:
			cur.WriteRune(r)
			inToken = true
		}
	}
	flush()
	return tokens
}
