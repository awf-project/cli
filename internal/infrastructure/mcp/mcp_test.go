package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
)

// TestNew_ReturnsNonNilServer verifies that New returns a non-nil *Server.
func TestNew_ReturnsNonNilServer(t *testing.T) {
	srv := New("1.0.0")
	require.NotNil(t, srv)
	assert.NotNil(t, srv.srv)
	assert.NotNil(t, srv.names)
}

// TestNew_WithVersionString verifies that New correctly passes version to SDK.
func TestNew_WithVersionString(t *testing.T) {
	versions := []string{
		"1.0.0",
		"0.1.0",
		"v2.3.4-beta",
	}

	for _, version := range versions {
		t.Run(fmt.Sprintf("version_%s", version), func(t *testing.T) {
			srv := New(version)
			require.NotNil(t, srv)
			// Verify the implementation field was set (SDK stores implementation)
			require.NotNil(t, srv.srv)
		})
	}
}

// TestNew_WithEmptyVersion verifies that New accepts empty version string.
func TestNew_WithEmptyVersion(t *testing.T) {
	srv := New("")
	require.NotNil(t, srv)
	assert.NotNil(t, srv.srv)
	assert.NotNil(t, srv.names)
}

// TestNew_StartsWithEmptyRegistry verifies that a fresh Server has no registered tools.
func TestNew_StartsWithEmptyRegistry(t *testing.T) {
	srv := New("1.0.0")
	require.NotNil(t, srv)
	assert.Equal(t, 0, len(srv.names))
}

// TestRegisterProvider_SingleTool verifies registration of one tool from a provider.
func TestRegisterProvider_SingleTool(t *testing.T) {
	srv := New("1.0.0")
	provider := &fakeProvider{
		tools: []ports.ToolDefinition{
			{
				Name:        "test-tool",
				Description: "A test tool",
				InputSchema: map[string]any{"type": "object"},
			},
		},
	}

	err := srv.RegisterProvider(provider)
	require.NoError(t, err)
	assert.Equal(t, 1, len(srv.names))
	_, exists := srv.names["test-tool"]
	assert.True(t, exists)
}

// TestRegisterProvider_MultipleToolsSingleProvider verifies registration of multiple tools from one provider.
func TestRegisterProvider_MultipleToolsSingleProvider(t *testing.T) {
	srv := New("1.0.0")
	provider := &fakeProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool-1", Description: "First tool"},
			{Name: "tool-2", Description: "Second tool"},
			{Name: "tool-3", Description: "Third tool"},
		},
	}

	err := srv.RegisterProvider(provider)
	require.NoError(t, err)
	assert.Equal(t, 3, len(srv.names))
	assert.True(t, assertToolExists(srv.names, "tool-1"))
	assert.True(t, assertToolExists(srv.names, "tool-2"))
	assert.True(t, assertToolExists(srv.names, "tool-3"))
}

// TestRegisterProvider_MultipleProviders verifies registration from multiple providers.
func TestRegisterProvider_MultipleProviders(t *testing.T) {
	srv := New("1.0.0")

	provider1 := &fakeProvider{
		tools: []ports.ToolDefinition{
			{Name: "bash", Description: "Execute bash"},
			{Name: "python", Description: "Execute python"},
		},
	}

	provider2 := &fakeProvider{
		tools: []ports.ToolDefinition{
			{Name: "grep", Description: "Search files"},
			{Name: "find", Description: "Find files"},
		},
	}

	err := srv.RegisterProvider(provider1)
	require.NoError(t, err)
	assert.Equal(t, 2, len(srv.names))

	err = srv.RegisterProvider(provider2)
	require.NoError(t, err)
	assert.Equal(t, 4, len(srv.names))

	assert.True(t, assertToolExists(srv.names, "bash"))
	assert.True(t, assertToolExists(srv.names, "python"))
	assert.True(t, assertToolExists(srv.names, "grep"))
	assert.True(t, assertToolExists(srv.names, "find"))
}

// TestRegisterProvider_DuplicateToolWithinProvider detects duplicates within a single ListTools result.
func TestRegisterProvider_DuplicateToolWithinProvider(t *testing.T) {
	srv := New("1.0.0")
	provider := &fakeProvider{
		tools: []ports.ToolDefinition{
			{Name: "duplicate-tool", Description: "First"},
			{Name: "unique-tool", Description: "Second"},
			{Name: "duplicate-tool", Description: "Third"},
		},
	}

	err := srv.RegisterProvider(provider)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate-tool")
	assert.Contains(t, err.Error(), "already registered")
	// Atomicity: the unique-tool that precedes the duplicate must NOT have been
	// committed — registration is all-or-nothing (see RegisterProvider validation pass).
	assert.Equal(t, 0, len(srv.names), "no tool should be registered when the provider list contains an internal duplicate")
}

// TestRegisterProvider_DuplicateToolAcrossProviders detects duplicates across multiple providers.
func TestRegisterProvider_DuplicateToolAcrossProviders(t *testing.T) {
	srv := New("1.0.0")

	provider1 := &fakeProvider{
		tools: []ports.ToolDefinition{
			{Name: "shared-tool", Description: "From provider 1"},
		},
	}

	provider2 := &fakeProvider{
		tools: []ports.ToolDefinition{
			{Name: "shared-tool", Description: "From provider 2"},
		},
	}

	err := srv.RegisterProvider(provider1)
	require.NoError(t, err)

	err = srv.RegisterProvider(provider2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shared-tool")
	assert.Contains(t, err.Error(), "already registered")
}

// TestRegisterProvider_ListToolsError propagates errors from provider.ListTools.
func TestRegisterProvider_ListToolsError(t *testing.T) {
	srv := New("1.0.0")
	expectedErr := errors.New("provider enumeration failed")
	provider := &fakeProvider{
		listErr: expectedErr,
	}

	err := srv.RegisterProvider(provider)
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

// TestRegisterProvider_PreservesExistingOnError verifies that failed registrations don't modify state.
func TestRegisterProvider_PreservesExistingOnError(t *testing.T) {
	srv := New("1.0.0")

	// Register first provider successfully
	provider1 := &fakeProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool-1", Description: "First"},
		},
	}
	err := srv.RegisterProvider(provider1)
	require.NoError(t, err)
	assert.Equal(t, 1, len(srv.names))

	// Second provider fails due to duplicate
	provider2 := &fakeProvider{
		tools: []ports.ToolDefinition{
			{Name: "tool-1", Description: "Duplicate"},
		},
	}
	err = srv.RegisterProvider(provider2)
	require.Error(t, err)

	// State should be unchanged (still just tool-1)
	assert.Equal(t, 1, len(srv.names))
}

// TestRegisterProvider_EmptyToolList handles provider with no tools.
func TestRegisterProvider_EmptyToolList(t *testing.T) {
	srv := New("1.0.0")
	provider := &fakeProvider{
		tools: []ports.ToolDefinition{},
	}

	err := srv.RegisterProvider(provider)
	require.NoError(t, err)
	assert.Equal(t, 0, len(srv.names))
}

// TestServeStdio_CanceledContextReturnsError verifies that ServeStdio respects context cancellation.
func TestServeStdio_CanceledContextReturnsError(t *testing.T) {
	srv := New("1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Create a pipe to prevent blocking on stdin
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()
	defer w.Close()

	// Replace stdin temporarily (this is a best-effort approach for testing context handling).
	// NOTE: mutates the process-global os.Stdin — do NOT add t.Parallel() to this test.
	// Prefer ServeIO with in-memory pipes for new tests (see TestServeIO_*).
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	done := make(chan error, 1)
	go func() {
		done <- srv.ServeStdio(ctx)
	}()

	select {
	case err := <-done:
		// The SDK does not contractually guarantee WHICH error surfaces on a cancelled
		// context over a closed/cancelled transport — it may be context.Canceled,
		// context.DeadlineExceeded, or io.EOF depending on which path observes shutdown
		// first. We assert only that one of these expected terminal errors is returned;
		// if a future SDK version narrows this, tighten the assertion accordingly.
		assert.Error(t, err)
		assert.True(
			t,
			errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.EOF),
			"expected context cancellation-related error, got: %v", err,
		)
	case <-time.After(2 * time.Second):
		t.Fatal("ServeStdio did not return within 2 seconds")
	}
}

// TestServeStdio_DoesNotPanic verifies that ServeStdio can be called on a valid server
// with an already-cancelled context without panicking.
func TestServeStdio_DoesNotPanic(t *testing.T) {
	srv := New("1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Create a pipe for stdin
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()
	defer w.Close()

	// NOTE: mutates the process-global os.Stdin — do NOT add t.Parallel() to this test.
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	// Should not panic
	assert.NotPanics(t, func() {
		_ = srv.ServeStdio(ctx)
	})
}

// TestServeIO_CanceledContextReturnsError exercises the serve path through ServeIO using
// in-memory pipes, avoiding any os.Stdin mutation (so this test is parallel-safe and does
// not share global state). It mirrors TestServeStdio_CanceledContextReturnsError.
func TestServeIO_CanceledContextReturnsError(t *testing.T) {
	t.Parallel()

	srv := New("1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r, w := io.Pipe()
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})

	done := make(chan error, 1)
	go func() {
		done <- srv.ServeIO(ctx, r, w)
	}()

	select {
	case err := <-done:
		assert.Error(t, err)
		assert.True(
			t,
			errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.EOF),
			"expected context cancellation-related error, got: %v", err,
		)
	case <-time.After(2 * time.Second):
		t.Fatal("ServeIO did not return within 2 seconds")
	}
}

// TestServeIO_DoesNotPanic verifies ServeIO can be invoked on a valid server with an
// already-cancelled context without panicking.
func TestServeIO_DoesNotPanic(t *testing.T) {
	t.Parallel()

	srv := New("1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r, w := io.Pipe()
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})

	assert.NotPanics(t, func() {
		_ = srv.ServeIO(ctx, r, w)
	})
}

// TestRegisterProvider_WithInputSchema verifies that tool input schemas are preserved.
func TestRegisterProvider_WithInputSchema(t *testing.T) {
	srv := New("1.0.0")
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type": "string",
			},
		},
		"required": []any{"query"},
	}

	provider := &fakeProvider{
		tools: []ports.ToolDefinition{
			{
				Name:        "search",
				Description: "Search function",
				InputSchema: schema,
			},
		},
	}

	err := srv.RegisterProvider(provider)
	require.NoError(t, err)
	assert.True(t, assertToolExists(srv.names, "search"))
}

// TestRegisterProvider_WithNilInputSchema handles tools with nil InputSchema.
func TestRegisterProvider_WithNilInputSchema(t *testing.T) {
	srv := New("1.0.0")
	provider := &fakeProvider{
		tools: []ports.ToolDefinition{
			{
				Name:        "simple-tool",
				Description: "Tool with nil schema",
				InputSchema: nil,
			},
		},
	}

	err := srv.RegisterProvider(provider)
	require.NoError(t, err)
	assert.True(t, assertToolExists(srv.names, "simple-tool"))
}

// TestNew_MultipleServersIndependent verifies that multiple Server instances are independent.
func TestNew_MultipleServersIndependent(t *testing.T) {
	srv1 := New("1.0.0")
	srv2 := New("2.0.0")

	provider := &fakeProvider{
		tools: []ports.ToolDefinition{
			{Name: "shared-tool", Description: "Test"},
		},
	}

	err := srv1.RegisterProvider(provider)
	require.NoError(t, err)
	assert.Equal(t, 1, len(srv1.names))

	// srv2 should still be empty
	assert.Equal(t, 0, len(srv2.names))

	// Now register same provider on srv2 - should succeed (no global state)
	err = srv2.RegisterProvider(provider)
	require.NoError(t, err)
	assert.Equal(t, 1, len(srv2.names))
}
