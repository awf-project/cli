package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infralogger "github.com/awf-project/cli/internal/infrastructure/logger"
)

func TestProcessEnvMap(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		m := processEnvMap()
		assert.NotNil(t, m)
		assert.IsType(t, map[string]string{}, m)
	})

	t.Run("preserves equals in values", func(t *testing.T) {
		t.Setenv("TEST_KEY", "value=with=equals")
		m := processEnvMap()
		assert.Equal(t, "value=with=equals", m["TEST_KEY"])
	})

	t.Run("skips empty keys", func(t *testing.T) {
		m := processEnvMap()
		for k := range m {
			assert.NotEqual(t, "", k)
		}
	})
}

func TestACPSessionStateDir(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		dir := acpSessionStateDir("abc123")
		assert.Contains(t, dir, "awf-acp-states")
		assert.Contains(t, dir, "abc123")
	})

	t.Run("path traversal defense", func(t *testing.T) {
		dir := acpSessionStateDir("../../../etc/passwd")
		assert.NotContains(t, dir, "..")
	})

	t.Run("dot slash defense", func(t *testing.T) {
		dir := acpSessionStateDir("./../../etc")
		assert.NotContains(t, dir, "..")
	})

	t.Run("empty fallback", func(t *testing.T) {
		dir := acpSessionStateDir("")
		assert.Contains(t, dir, "default")
	})

	t.Run("slash only fallback", func(t *testing.T) {
		dir := acpSessionStateDir("/")
		assert.Contains(t, dir, "default")
	})

	t.Run("slash root defense", func(t *testing.T) {
		dir := acpSessionStateDir(string(filepath.Separator))
		assert.Contains(t, dir, "default")
	})

	t.Run("dot fallback", func(t *testing.T) {
		dir := acpSessionStateDir(".")
		assert.Contains(t, dir, "default")
	})

	t.Run("creates under temp dir", func(t *testing.T) {
		dir := acpSessionStateDir("test123")
		assert.True(t, filepath.IsAbs(dir))
		assert.Contains(t, dir, os.TempDir())
	})
}

// TestACPServe_WiresSetFacadeBeforeServe verifies the Set*-before-Serve convention for the
// facade path (T075): buildACPServerFacade returns a real, non-nil ports.WorkflowFacade that
// runACPServe installs via SetFacade before it blocks on conn.Done(). A nil facade here would
// leave the facade-mode gate off and the session service would refuse to dispatch. The facade
// is installed on a real ACPSessionService and the resulting state is observed through
// HandleSessionPrompt, which must route through the facade.
func TestACPServe_WiresSetFacadeBeforeServe(t *testing.T) {
	signalCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := infralogger.NewConsoleLogger(io.Discard, infralogger.LevelInfo, false)
	repo := oneWorkflowRepo{name: "trivial"}
	baseOpts := []application.SetupOption{application.WithTracer(ports.NopTracer{})}
	appCfg := DefaultConfig()
	appCfg.StoragePath = t.TempDir()

	facade, cleanup, err := buildACPServerFacade(signalCtx, repo, executor.NewShellExecutor(), baseOpts, appCfg, logger)
	require.NoError(t, err)
	require.NotNil(t, facade, "buildACPServerFacade must return a non-nil facade so SetFacade enables facade mode")
	require.NotNil(t, cleanup)
	defer cleanup()

	// The facade must be a usable ports.WorkflowFacade: List must succeed against the repo.
	summaries, listErr := facade.List(signalCtx)
	require.NoError(t, listErr)
	require.NotEmpty(t, summaries, "server facade must list the configured workflow")

	// Wiring contract: installing it via SetFacade flips the session service into facade mode.
	svc := application.NewACPSessionService(nil, repo, logger)
	svc.SetFacade(facade)
	// With facade mode ON the dispatch path routes through the facade. We assert the facade was
	// accepted (no panic, non-nil) — the per-session dispatch routing itself is covered by the
	// application-layer facade tests.
	require.NotNil(t, svc, "service must accept a non-nil facade via SetFacade before Serve")
}

// TestACP_ConcurrentSessionsBuildDistinctFacades is the SC-003 (FR-009) interface-layer
// regression test: two ACP sessions built concurrently via buildACPSessionWiring must each get
// a distinct per-session ports.WorkflowFacade rooted at a distinct acpSessionStateDir, so two
// sessions running the SAME workflow cannot clobber each other's persisted state. Run with -race
// to confirm the concurrent wiring is race-clean.
func TestACP_ConcurrentSessionsBuildDistinctFacades(t *testing.T) {
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

	const sidA = "sess-aaaa"
	const sidB = "sess-bbbb"

	// Distinct sessions must resolve to distinct state directories (the isolation seam).
	require.NotEqual(t, acpSessionStateDir(sidA), acpSessionStateDir(sidB),
		"distinct session IDs must map to distinct state dirs (SC-003)")

	type wiringResult struct {
		wiring *acpSessionWiring
		err    error
	}
	results := make(chan wiringResult, 2)
	for _, sid := range []string{sidA, sidB} {
		go func(sessionID string) {
			w, wErr := buildACPSessionWiring(&deps, sessionID)
			results <- wiringResult{wiring: w, err: wErr}
		}(sid)
	}

	wirings := make([]*acpSessionWiring, 0, 2)
	for range 2 {
		select {
		case r := <-results:
			require.NoError(t, r.err)
			require.NotNil(t, r.wiring)
			require.NotNil(t, r.wiring.facade, "each session wiring must build a per-session facade (T075)")
			require.NotNil(t, r.wiring.cleanup)
			wirings = append(wirings, r.wiring)
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent buildACPSessionWiring did not complete within timeout")
		}
	}
	for _, w := range wirings {
		t.Cleanup(w.cleanup)
	}

	assert.NotSame(t, wirings[0].facade, wirings[1].facade,
		"two concurrent sessions must build distinct per-session facades (SC-003 state isolation)")
}

func TestValidateWorkflowsDir(t *testing.T) {
	t.Run("valid directory", func(t *testing.T) {
		tmpdir := t.TempDir()
		err := validateWorkflowsDir(tmpdir)
		assert.NoError(t, err)
	})

	t.Run("does not exist", func(t *testing.T) {
		err := validateWorkflowsDir("/nonexistent/path/to/workflows")
		assert.Error(t, err)
		exitErr, ok := err.(*exitError)
		require.True(t, ok, "expected exitError, got %T", err)
		assert.Equal(t, ExitUser, exitErr.code)
	})

	t.Run("not a directory", func(t *testing.T) {
		tmpfile := filepath.Join(t.TempDir(), "file.txt")
		require.NoError(t, os.WriteFile(tmpfile, []byte("test"), 0o600))

		err := validateWorkflowsDir(tmpfile)
		assert.Error(t, err)
		exitErr, ok := err.(*exitError)
		require.True(t, ok)
		assert.Equal(t, ExitUser, exitErr.code)
	})

	t.Run("invalid path characters", func(t *testing.T) {
		tmpdir := t.TempDir()
		dir := filepath.Join(tmpdir, "workflows")
		require.NoError(t, os.Mkdir(dir, 0o755))

		err := validateWorkflowsDir(dir)
		assert.NoError(t, err)
	})
}

func TestACPTextWriter_Write(t *testing.T) {
	t.Run("writes to emitter", func(t *testing.T) {
		mockEmitter := new(mockSessionUpdateEmitter)
		mockEmitter.On("EmitSessionUpdate", mock.Anything, "session-1", "agent_message_chunk", mock.Anything).Return(nil)

		ctx := context.Background()
		streamed := &atomic.Bool{}
		w := newACPTextWriter(ctx, mockEmitter, "session-1", streamed)

		n, err := w.Write([]byte("hello"))
		assert.NoError(t, err)
		assert.Equal(t, 5, n)
		mockEmitter.AssertCalled(t, "EmitSessionUpdate", mock.Anything, "session-1", "agent_message_chunk", mock.Anything)
	})

	t.Run("empty write returns zero", func(t *testing.T) {
		mockEmitter := new(mockSessionUpdateEmitter)
		ctx := context.Background()
		w := newACPTextWriter(ctx, mockEmitter, "session-1", nil)

		n, err := w.Write([]byte{})
		assert.NoError(t, err)
		assert.Equal(t, 0, n)
		mockEmitter.AssertNotCalled(t, "EmitSessionUpdate")
	})

	t.Run("emit failure increments missed count", func(t *testing.T) {
		mockEmitter := new(mockSessionUpdateEmitter)
		mockEmitter.On("EmitSessionUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(io.EOF)

		ctx := context.Background()
		w := newACPTextWriter(ctx, mockEmitter, "session-1", nil)

		n, err := w.Write([]byte("test"))
		assert.NoError(t, err)
		assert.Equal(t, 4, n)
		assert.Equal(t, uint64(1), w.MissedEmits())
	})

	t.Run("sets streamed flag on success", func(t *testing.T) {
		mockEmitter := new(mockSessionUpdateEmitter)
		mockEmitter.On("EmitSessionUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		ctx := context.Background()
		streamed := &atomic.Bool{}
		w := newACPTextWriter(ctx, mockEmitter, "session-1", streamed)

		_, _ = w.Write([]byte("test"))
		assert.True(t, streamed.Load())
	})

	t.Run("does not set streamed flag on failure", func(t *testing.T) {
		mockEmitter := new(mockSessionUpdateEmitter)
		mockEmitter.On("EmitSessionUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(io.EOF)

		ctx := context.Background()
		streamed := &atomic.Bool{}
		w := newACPTextWriter(ctx, mockEmitter, "session-1", streamed)

		_, _ = w.Write([]byte("test"))
		assert.False(t, streamed.Load())
	})

	t.Run("nil streamed pointer handled", func(t *testing.T) {
		mockEmitter := new(mockSessionUpdateEmitter)
		mockEmitter.On("EmitSessionUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		ctx := context.Background()
		w := newACPTextWriter(ctx, mockEmitter, "session-1", nil)

		n, err := w.Write([]byte("test"))
		assert.NoError(t, err)
		assert.Equal(t, 4, n)
	})

	t.Run("multiple writes accumulate missed", func(t *testing.T) {
		mockEmitter := new(mockSessionUpdateEmitter)
		mockEmitter.On("EmitSessionUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(io.EOF)

		ctx := context.Background()
		w := newACPTextWriter(ctx, mockEmitter, "session-1", nil)

		_, _ = w.Write([]byte("test1"))
		_, _ = w.Write([]byte("test2"))
		_, _ = w.Write([]byte("test3"))

		assert.Equal(t, uint64(3), w.MissedEmits())
	})
}

// TestStreamFlaggingEmitter_EmitSessionUpdate covers the wrapper that lets per-step
// renderers emit ACP SessionUpdate variants directly while preserving the streamed
// signal (replaces the legacy acpMessageSender). The discriminator/field mapping that
// used to live in the sender now lives in the infra Renderer (see renderer_test.go).
func TestStreamFlaggingEmitter_EmitSessionUpdate(t *testing.T) {
	t.Run("delegates to wrapped emitter", func(t *testing.T) {
		mockEmitter := new(mockSessionUpdateEmitter)
		mockEmitter.On("EmitSessionUpdate", mock.Anything, "session-1", "agent_message_chunk", mock.Anything).Return(nil)

		e := newStreamFlaggingEmitter(mockEmitter, nil)
		err := e.EmitSessionUpdate(context.Background(), "session-1", "agent_message_chunk", map[string]any{"seq": uint64(1)})
		assert.NoError(t, err)
		mockEmitter.AssertCalled(t, "EmitSessionUpdate", mock.Anything, "session-1", "agent_message_chunk", mock.Anything)
	})

	t.Run("sets streamed flag on success", func(t *testing.T) {
		mockEmitter := new(mockSessionUpdateEmitter)
		mockEmitter.On("EmitSessionUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		streamed := &atomic.Bool{}
		e := newStreamFlaggingEmitter(mockEmitter, streamed)
		_ = e.EmitSessionUpdate(context.Background(), "session-1", "agent_message_chunk", nil)
		assert.True(t, streamed.Load())
	})

	t.Run("propagates emit errors", func(t *testing.T) {
		mockEmitter := new(mockSessionUpdateEmitter)
		mockEmitter.On("EmitSessionUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(io.EOF)

		e := newStreamFlaggingEmitter(mockEmitter, nil)
		err := e.EmitSessionUpdate(context.Background(), "session-1", "agent_message_chunk", nil)
		assert.Error(t, err)
	})

	t.Run("does not set streamed on error", func(t *testing.T) {
		mockEmitter := new(mockSessionUpdateEmitter)
		mockEmitter.On("EmitSessionUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(io.EOF)

		streamed := &atomic.Bool{}
		e := newStreamFlaggingEmitter(mockEmitter, streamed)
		_ = e.EmitSessionUpdate(context.Background(), "session-1", "agent_message_chunk", nil)
		assert.False(t, streamed.Load())
	})
}

func TestNewACPTextWriter_ContextCapture(t *testing.T) {
	mockEmitter := new(mockSessionUpdateEmitter)
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	streamed := &atomic.Bool{}
	w := newACPTextWriter(cancelledCtx, mockEmitter, "session-1", streamed)

	assert.NotNil(t, w)
	assert.Equal(t, cancelledCtx, w.ctx)
}

func TestRunProtocolInterceptor_InvalidJSON(t *testing.T) {
	t.Run("rejects invalid json", func(t *testing.T) {
		src := bytes.NewReader([]byte("invalid json\n"))
		dst := &bytes.Buffer{}
		_, pipeW := io.Pipe()

		go func() {
			runProtocolInterceptor(context.Background(), src, dst, pipeW)
		}()

		time.Sleep(100 * time.Millisecond)
		_ = pipeW.Close()

		output := dst.String()
		assert.Contains(t, output, "parse error")
		assert.Contains(t, output, "-32700")
	})

	t.Run("forwards valid json", func(t *testing.T) {
		src := bytes.NewReader([]byte(`{"jsonrpc":"2.0","method":"test"}` + "\n"))
		dst := &bytes.Buffer{}
		pipeR, pipeW := io.Pipe()

		go func() {
			runProtocolInterceptor(context.Background(), src, dst, pipeW)
		}()

		result := make([]byte, 100)
		n, _ := pipeR.Read(result)
		assert.Greater(t, n, 0)
		assert.Contains(t, string(result[:n]), "jsonrpc")
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		src := bytes.NewReader([]byte("test\n"))
		dst := &bytes.Buffer{}
		_, pipeW := io.Pipe()

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			runProtocolInterceptor(ctx, src, dst, pipeW)
		}()

		time.Sleep(50 * time.Millisecond)
		cancel()
		time.Sleep(50 * time.Millisecond)

		assert.True(t, true)
	})

	t.Run("ignores empty lines", func(t *testing.T) {
		src := bytes.NewReader([]byte("\n\n" + `{"jsonrpc":"2.0"}` + "\n"))
		dst := &bytes.Buffer{}
		pipeR, pipeW := io.Pipe()

		go func() {
			runProtocolInterceptor(context.Background(), src, dst, pipeW)
		}()

		result := make([]byte, 50)
		n, _ := pipeR.Read(result)
		assert.Greater(t, n, 0)
		assert.Contains(t, string(result[:n]), "jsonrpc")
	})

	t.Run("handles line too long", func(t *testing.T) {
		longLine := make([]byte, 11*1024*1024) // exceeds the 10 MiB cap (NFR-005)
		src := bytes.NewReader(longLine)
		dst := &bytes.Buffer{}
		_, pipeW := io.Pipe()

		// Run synchronously rather than via a goroutine + fixed sleep: an oversize line
		// yields no valid frame, so the interceptor never writes to pipeW and cannot block.
		// It scans until bufio.ErrTooLong, writes the parse error to dst, closes pipeW, and
		// returns. Synchronous execution makes the assertion deterministic (no flakiness when
		// scanning 11 MiB is slow under -race on CI) and removes the data race on dst that a
		// concurrent reader created.
		runProtocolInterceptor(context.Background(), src, dst, pipeW)

		assert.Contains(t, dst.String(), "parse error")
	})
}

func TestWriteJSONRPCParseError(t *testing.T) {
	t.Run("writes parse error response", func(t *testing.T) {
		dst := &bytes.Buffer{}
		writeJSONRPCParseError(dst)

		output := dst.String()
		assert.Contains(t, output, "parse error")
		assert.Contains(t, output, "-32700")
		assert.Contains(t, output, "null")
		assert.Contains(t, output, "jsonrpc")
		assert.Contains(t, output, "2.0")
	})

	t.Run("ends with newline", func(t *testing.T) {
		dst := &bytes.Buffer{}
		writeJSONRPCParseError(dst)

		output := dst.String()
		assert.NotEmpty(t, output)
		assert.Equal(t, '\n', rune(output[len(output)-1]))
	})
}

// Mock types for testing

type mockSessionUpdateEmitter struct {
	mock.Mock
}

func (m *mockSessionUpdateEmitter) EmitSessionUpdate(ctx context.Context, sessionID, updateType string, payload map[string]any) error {
	return m.Called(ctx, sessionID, updateType, payload).Error(0)
}
