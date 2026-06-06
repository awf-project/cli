package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infralogger "github.com/awf-project/cli/internal/infrastructure/logger"
)

// --- T036 acceptance tests: runACPServe lifecycle ---

// TestACPServe_LoggerWritesToStderr verifies NFR-002: ACP/SDK diagnostics go to the
// configured sink (os.Stderr in production), never to stdout (reserved for JSON-RPC frames).
func TestACPServe_LoggerWritesToStderr(t *testing.T) {
	var stderr, stdout bytes.Buffer
	logger := newACPSDKLogger(&stderr)

	logger.Info("acp diagnostic", "key", "value")

	assert.NotEmpty(t, stderr.String(), "log must reach the configured (stderr) sink")
	assert.Contains(t, stderr.String(), "acp diagnostic")
	assert.Empty(t, stdout.String(), "nothing must be written to stdout")
}

// TestACPServe_PerSessionFactoryCapturesSignalCtx verifies C2: every session-scoped I/O
// component derives from the cancellable shutdown signal context, not the parent ctx, so a
// SIGTERM/disconnect stops in-flight emission. The output writer is the observable seam.
func TestACPServe_PerSessionFactoryCapturesSignalCtx(t *testing.T) {
	signalCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deps := acpSessionFactoryDeps{
		signalCtx:     signalCtx,
		conn:          nil, // emitter degrades to a no-op with a nil conn; not needed here
		slogLogger:    newACPSDKLogger(io.Discard),
		logger:        infralogger.NewConsoleLogger(io.Discard, infralogger.LevelInfo, false),
		masker:        infralogger.NewSecretMasker(),
		envMap:        map[string]string{},
		baseOpts:      []application.SetupOption{application.WithTracer(ports.NopTracer{})},
		repo:          oneWorkflowRepo{name: "trivial"},
		shellExecutor: executor.NewShellExecutor(),
	}

	wiring, err := buildACPSessionWiring(&deps, "sess-test")
	require.NoError(t, err)
	require.NotNil(t, wiring)
	require.NotNil(t, wiring.textWriter)

	// C2: the session output writer must hold the shutdown signal context (same instance),
	// not a detached or parent context.
	assert.Equal(t, signalCtx, wiring.textWriter.ctx,
		"session components must capture signalCtx so shutdown stops emission")

	// Cancelling signalCtx must be observable through the captured ctx.
	cancel()
	assert.ErrorIs(t, wiring.textWriter.ctx.Err(), context.Canceled,
		"captured ctx must cancel together with signalCtx")
}

// TestACPServe_StdinCloseUnblocksReader verifies NFR-006: when stdin reaches EOF (the editor
// disconnects or the server closes os.Stdin on shutdown), runProtocolInterceptor closes the
// downstream pipe so the SDK's blocked reader unblocks instead of hanging.
func TestACPServe_StdinCloseUnblocksReader(t *testing.T) {
	srcR, srcW := io.Pipe()   // stands in for os.Stdin
	pipeR, pipeW := io.Pipe() // the pipe the SDK connection reads from

	go runProtocolInterceptor(context.Background(), srcR, io.Discard, pipeW)

	readDone := make(chan error, 1)
	go func() {
		_, err := pipeR.Read(make([]byte, 1)) // blocks until pipeW is closed
		readDone <- err
	}()

	// The reader must still be blocked while stdin is open.
	select {
	case <-readDone:
		t.Fatal("reader unblocked before stdin reached EOF")
	case <-time.After(50 * time.Millisecond):
	}

	_ = srcW.Close() // simulate stdin EOF → interceptor exits its loop → closes pipeW

	select {
	case err := <-readDone:
		assert.ErrorIs(t, err, io.EOF, "reader must unblock with EOF once the stdin pipe closes")
	case <-time.After(2 * time.Second):
		t.Fatal("reader did not unblock after stdin EOF")
	}
}

// TestACPServe_ShutdownDrainsRunWG verifies the two-phase shutdown that runACPServe defers
// after conn.Done(): phase 1 seals the session-creation window (new sessions rejected), and
// phase 2 drains the run WaitGroup (Shutdown returns only once the in-flight run goroutine
// has observed cancellation and returned). The deep runWG race ordering is additionally
// covered by TestACPSessionService_C1/C2 in the application layer; this asserts the contract
// through the public API a server relies on.
func TestACPServe_ShutdownDrainsRunWG(t *testing.T) {
	logger := infralogger.NewConsoleLogger(io.Discard, infralogger.LevelInfo, false)
	svc := application.NewACPSessionService(nil, nil, oneWorkflowRepo{name: "trivial"}, logger)

	entered := make(chan struct{})
	svc.SetRunnerFactory(func(string) (application.WorkflowRunner, application.ACPInputResponder, *atomic.Bool, func(), error) {
		return &fakeBlockingRunner{entered: entered}, fakeInputResponder{}, &atomic.Bool{}, func() {}, nil
	})

	baseCtx := context.Background()
	newResult, acpErr := svc.HandleSessionNew(baseCtx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
	require.Nil(t, acpErr)
	sessionID, _ := newResult.(map[string]any)["sessionId"].(string)
	require.NotEmpty(t, sessionID)

	promptParams, _ := json.Marshal(map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": "/trivial"}},
	})
	promptCtx, promptCancel := context.WithCancel(baseCtx)
	defer promptCancel()

	promptDone := make(chan struct{})
	go func() {
		defer close(promptDone)
		_, _ = svc.HandleSessionPrompt(promptCtx, promptParams)
	}()

	// Wait until the run goroutine is in flight (runWG incremented).
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("runner.Run was never entered")
	}

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		svc.Shutdown()
	}()

	// Unblock the runner the way the JSON-RPC server does at shutdown: cancel the request ctx.
	promptCancel()

	// Phase 2: Shutdown must return once the run goroutine drains.
	select {
	case <-shutdownDone:
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown did not drain runWG within timeout")
	}
	<-promptDone

	// Phase 1: the creation window is sealed — new sessions are rejected after shutdown.
	_, rejectErr := svc.HandleSessionNew(baseCtx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
	require.NotNil(t, rejectErr, "HandleSessionNew must be rejected after Shutdown")
	assert.Equal(t, application.ACPErrInternal, rejectErr.Kind)
}

// --- fakes ---

// fakeBlockingRunner signals when Run is entered then blocks until its ctx is cancelled,
// returning ctx.Err() so the handler maps the run to a cancelled stop reason.
type fakeBlockingRunner struct {
	entered  chan struct{}
	onceDone atomic.Bool
}

func (r *fakeBlockingRunner) Run(ctx context.Context, _ string, _ map[string]any) (*workflow.ExecutionContext, error) {
	if r.onceDone.CompareAndSwap(false, true) {
		close(r.entered)
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

// fakeInputResponder is a no-op ACPInputResponder for the drain test.
type fakeInputResponder struct{}

func (fakeInputResponder) ReadInput(ctx context.Context) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}
func (fakeInputResponder) Respond(string)           {}
func (fakeInputResponder) SetParkHooks(_, _ func()) {}

// oneWorkflowRepo is a minimal WorkflowRepository exposing a single terminal workflow so the
// session handlers can discover and load it.
type oneWorkflowRepo struct{ name string }

func (r oneWorkflowRepo) Load(_ context.Context, name string) (*workflow.Workflow, error) {
	if name != r.name {
		return nil, fmt.Errorf("workflow not found: %s", name)
	}
	return &workflow.Workflow{
		Name:    r.name,
		Version: "1.0.0",
		Initial: "start",
		Steps:   map[string]*workflow.Step{"start": {Name: "start", Type: workflow.StepTypeTerminal}},
	}, nil
}

func (r oneWorkflowRepo) List(context.Context) ([]string, error) { return []string{r.name}, nil }

func (r oneWorkflowRepo) ListWithSource(context.Context) ([]ports.WorkflowInfo, error) {
	return []ports.WorkflowInfo{{Name: r.name, Source: ports.SourceLocal, Path: "/p/" + r.name + ".yaml"}}, nil
}

func (r oneWorkflowRepo) Exists(_ context.Context, name string) (bool, error) {
	return name == r.name, nil
}
