package application

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// WorkflowRunner is the subset of *ExecutionService the ACP session service drives to
// dispatch a workflow. Declaring it as a consumer interface keeps HandleSessionPrompt
// unit-testable with a fake runner and avoids constructing the full ExecutionService in
// application-layer tests (which would require infrastructure). *ExecutionService
// satisfies it directly.
type WorkflowRunner interface {
	Run(ctx context.Context, name string, inputs map[string]any) (*workflow.ExecutionContext, error)
}

// MCPServerSpec is an editor-provided MCP server launch spec decoded from a session/new
// `mcpServers` array entry (ACP). Distinct from workflow.MCPProxyConfig (interception config).
type MCPServerSpec struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// ACPInputResponder is the subset of the ACP input reader the session service drives:
// it satisfies ports.UserInputReader (for the workflow executor) and accepts responses
// routed from subsequent session/prompt turns. Declaring it as an interface keeps the
// application layer free of a direct dependency on internal/infrastructure/acp; the
// concrete *acp.ACPInputReader is injected by the interfaces/cli wiring layer.
//
// SetParkHooks installs the OnPark/OnUnpark callbacks the reader fires around its blocking
// wait for user input. The session service wires them to bump ACPSession.ParkedTurnCount so
// a continuation prompt (arriving while a workflow goroutine is parked) is routed to Respond
// instead of starting a new run. This is the production seam that makes the parking branch
// in HandleSessionPrompt live (CRITIQUE-3); the contract is one OnUnpark per OnPark.
type ACPInputResponder interface {
	ports.UserInputReader
	Respond(text string)
	SetParkHooks(onPark, onUnpark func())
}

// ACPRunnerFactory builds a per-session WorkflowRunner with session-scoped wiring
// (input reader, event publisher, output writers, display renderer). It also returns
// the session's input reader (so continuation turns can route text to it), the
// streamed flag (set to true by the output writers/renderer when an emit succeeds),
// and a cleanup that releases that session's resources. Injected by the interfaces/cli
// wiring layer; nil in unit tests, where the shared runner field is used instead.
type ACPRunnerFactory func(sessionID string) (WorkflowRunner, ACPInputResponder, *atomic.Bool, func(), error)

// inputReaderHolder wraps an ACPInputResponder so that atomic.Pointer[inputReaderHolder]
// holds a concrete pointer rather than an interface value. Storing a pointer-to-interface
// in atomic.Pointer is an anti-pattern: the pointer indirection obscures nil checks and
// the interface slot itself is never atomic. Wrapping the interface in a concrete struct
// eliminates the indirection and makes Load/Store semantics explicit (C-2 fix).
type inputReaderHolder struct {
	r ACPInputResponder
}

// acpRun holds the coordination channels and outcome for one in-flight workflow run.
// It is published via ACPSession.run (atomic.Pointer) on first dispatch and read by the
// park hook (on the run goroutine) and by continuation-turn handlers.
//
// US2 conversation parking: a single workflow run spans multiple ACP turns. The run
// executes on its own goroutine (so HandleSessionPrompt can return a stopReason while the
// workflow is still parked, letting the editor re-enable its input field). parkedCh
// signals a turn boundary (the workflow blocked in ReadInput awaiting the next user turn);
// doneCh is closed when runner.Run returns.
//
// The outcome fields (execCtx/runErr/cancelled) are written by the run goroutine BEFORE
// close(doneCh) and read only AFTER <-doneCh, so the channel close establishes the
// happens-before relationship — no additional synchronization is required.
type acpRun struct {
	// parkedCh is buffered (cap 1): the park hook performs a non-blocking send so the
	// workflow goroutine is never blocked, and each parked turn boundary is delivered to
	// exactly one waiting turn. doneCh is closed (not sent) when runner.Run returns.
	parkedCh     chan struct{}
	doneCh       chan struct{}
	workflowName string

	// Outcome — written before close(doneCh), read after <-doneCh.
	execCtx   *workflow.ExecutionContext
	runErr    error
	cancelled bool
}

// ACPSession holds the per-session runtime state for an ACP server session.
type ACPSession struct {
	ID string
	// CWD is the working directory provided at session/new.
	// TODO(F102-v2): currently stored but not yet propagated to the workflow
	// runner as an interpolation variable. See ADR-018.
	CWD        string
	MCPServers map[string]MCPServerSpec

	// inputReader holds the session's ACPInputResponder wrapped in inputReaderHolder.
	// Written once under runnerMu in ensureRunner and read by HandleSessionPrompt
	// (parking check) without the lock. An atomic.Pointer[inputReaderHolder] makes the
	// publish/consume race-free — the same pattern already used for execCtx (M7 fix).
	// The holder wrapper avoids the pointer-on-interface anti-pattern (C-2 fix).
	inputReader atomic.Pointer[inputReaderHolder]

	// execCtx holds the ExecutionContext of the most recent run. It is written by the
	// session/prompt handler and read by workflowOutputText; an atomic.Pointer makes the
	// publish/consume race-free without taking mu (verified by -race).
	execCtx  atomic.Pointer[workflow.ExecutionContext]
	InFlight atomic.Bool

	// run holds the coordination state for the current in-flight workflow run (US2
	// conversation parking). It is published on first dispatch and read lock-free by the
	// park hook and by continuation-turn handlers. Nil before the first dispatch; a
	// completed run is left in place (its doneCh closed) until the next dispatch replaces it.
	run atomic.Pointer[acpRun]
	// ParkedTurnCount is atomic: the parked workflow goroutine increments it while a
	// prompt handler reads it (lock-free, mirroring InFlight). It is bumped via the
	// ACPInputReader park hooks wired in the interfaces layer, so a continuation prompt
	// routes to the parked reader instead of starting a new workflow.
	ParkedTurnCount atomic.Int32

	// runWG tracks the in-flight workflow run goroutine(s) for this session. The
	// session/prompt handler adds before runner.Run and decrements after; Shutdown waits
	// on it (after cancelling) so no per-session resource is released while a workflow is
	// still touching it (SQLite, temp files, etc.).
	runWG sync.WaitGroup

	// mu guards cancelFn, which is written by the session/prompt handler when a workflow
	// run starts and read by a concurrent session/cancel handler. Both run on independent
	// server-dispatched goroutines, so the swap must be synchronized (verified by -race).
	mu       sync.Mutex
	cancelFn context.CancelFunc

	// Per-session lazily-built runner state. runnerMu serializes construction and allows
	// a failed factory call to be retried on the next prompt (unlike sync.Once, which would
	// permanently brick the session). runnerBuilt is set true only after a successful build.
	runnerMu      sync.Mutex
	runnerBuilt   bool
	runner        WorkflowRunner
	runnerCleanup func()

	// streamed is set to true by the output writers / renderer when at least one emit
	// succeeds during a run. It is reset to false at the start of each run so the
	// aggregate-suppression check in HandleSessionPrompt reflects the current run only.
	// Stored as an atomic.Pointer so it can be read outside runnerMu without a race
	// (M7 fix). Nil pointer for sessions using the shared fallback runner (factory not set).
	streamed atomic.Pointer[atomic.Bool]
}

// setCancel records the cancel function for the in-flight workflow run.
func (s *ACPSession) setCancel(fn context.CancelFunc) {
	s.mu.Lock()
	s.cancelFn = fn
	s.mu.Unlock()
}

// cancel invokes the recorded cancel function, if any. Safe to call concurrently with
// setCancel and idempotent (a nil cancelFn is a no-op).
func (s *ACPSession) cancel() {
	s.mu.Lock()
	fn := s.cancelFn
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// parseInputPairs splits "key=value" strings into a map.
// Rejects empty keys and entries without a "=" separator.
// Does not resolve @prompts/ prefixes (CLI-only, not applicable to ACP).
func parseInputPairs(pairs []string) (map[string]any, error) {
	result := make(map[string]any, len(pairs))
	for _, pair := range pairs {
		k, v, ok := strings.Cut(pair, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid input pair %q: expected key=value", pair)
		}
		result[k] = strings.TrimSpace(v)
	}
	return result, nil
}
