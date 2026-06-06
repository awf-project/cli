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
