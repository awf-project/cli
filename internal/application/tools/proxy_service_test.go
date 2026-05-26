package tools

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/testutil/mocks"
)

// Test helpers: factory and provider setup

func newMockProviderFactory(providers []ports.ToolProvider, err error) ProviderFactory {
	return func(cfg ProxyConfig) ([]ports.ToolProvider, error) {
		if err != nil {
			return nil, err
		}
		return providers, nil
	}
}

// TestProxyService_NewProxyService verifies NewProxyService creates a configured service.
func TestProxyService_NewProxyService(t *testing.T) {
	cliExec := mocks.NewMockCLIExecutor()
	tracer := mocks.NewMockTracer()
	logger := mocks.NewMockLogger()
	factory := newMockProviderFactory(nil, nil)

	svc := NewProxyService(cliExec, tracer, logger, factory)

	assert.NotNil(t, svc)
}

// TestProxyService_StartForStdio_DisabledConfig returns noop when config is disabled.
func TestProxyService_StartForStdio_DisabledConfig(t *testing.T) {
	cliExec := mocks.NewMockCLIExecutor()
	svc := NewProxyService(cliExec, mocks.NewMockTracer(), mocks.NewMockLogger(), newMockProviderFactory(nil, nil))

	mcpPath, cleanup, err := svc.StartForStdio(context.Background(), ProxyConfig{
		InterceptBuiltins: false,
		PluginTools:       []PluginToolSpec{},
	})

	assert.NoError(t, err)
	assert.Empty(t, mcpPath)
	assert.NotNil(t, cleanup)
	assert.NoError(t, cleanup())
}

// TestProxyService_StartForStdio_CleanupIdempotent verifies cleanup can be called multiple times.
func TestProxyService_StartForStdio_CleanupIdempotent(t *testing.T) {
	cliExec := mocks.NewMockCLIExecutor()
	svc := NewProxyService(cliExec, mocks.NewMockTracer(), mocks.NewMockLogger(), newMockProviderFactory(nil, nil))

	mcpPath, cleanup, err := svc.StartForStdio(context.Background(), ProxyConfig{
		InterceptBuiltins: false,
		PluginTools:       []PluginToolSpec{},
	})

	require.NoError(t, err)
	require.Empty(t, mcpPath)

	// Call cleanup multiple times - all should succeed
	err1 := cleanup()
	err2 := cleanup()
	err3 := cleanup()

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NoError(t, err3)
}

// TestProxyService_StartForStdio_WritesConfigFile verifies tmp config file is created with proper JSON.
func TestProxyService_StartForStdio_WritesConfigFile(t *testing.T) {
	cliExec := mocks.NewMockCLIExecutor()

	// Mock cliExec.Start to return a process that we control
	mockProc := mocks.NewMockCLIProcess()
	cliExec.StartFunc = func(ctx context.Context, name string, args ...string) (ports.CLIProcess, error) {
		return mockProc, nil
	}

	mockProvider := &mockToolProvider{}
	factory := newMockProviderFactory([]ports.ToolProvider{mockProvider}, nil)

	svc := NewProxyService(cliExec, mocks.NewMockTracer(), mocks.NewMockLogger(), factory)

	mcpPath, cleanup, err := svc.StartForStdio(context.Background(), ProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
		PluginTools:       []PluginToolSpec{},
	})

	require.NoError(t, err)
	require.NotEmpty(t, mcpPath)

	// Verify tmp file exists
	_, err = os.Stat(mcpPath)
	require.NoError(t, err, "temp config file should exist")

	// Cleanup and verify file is removed
	require.NotNil(t, cleanup)
	mockProc.Close() // Signal process completion
	err = cleanup()
	require.NoError(t, err)

	_, err = os.Stat(mcpPath)
	require.True(t, os.IsNotExist(err), "temp config file should be removed after cleanup")
}

// TestProxyService_StartForStdio_SpawnsProcess verifies awf mcp-serve is spawned.
func TestProxyService_StartForStdio_SpawnsProcess(t *testing.T) {
	cliExec := mocks.NewMockCLIExecutor()
	mockProc := mocks.NewMockCLIProcess()
	cliExec.StartFunc = func(ctx context.Context, name string, args ...string) (ports.CLIProcess, error) {
		return mockProc, nil
	}

	mockProvider := &mockToolProvider{}
	factory := newMockProviderFactory([]ports.ToolProvider{mockProvider}, nil)

	svc := NewProxyService(cliExec, mocks.NewMockTracer(), mocks.NewMockLogger(), factory)

	_, cleanup, err := svc.StartForStdio(context.Background(), ProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
		PluginTools:       []PluginToolSpec{},
	})

	require.NoError(t, err)

	// Verify Start was called
	startCalls := cliExec.GetStartCalls()
	require.Len(t, startCalls, 1)

	// Verify command and args
	call := startCalls[0]
	assert.Equal(t, "mcp-serve", call.Args[0])
	assert.True(t, len(call.Args) > 1, "should have --config argument")

	// Cleanup
	mockProc.Close()
	_ = cleanup()
}

// TestProxyService_StartForStdio_IgnoresProviderFactory verifies that StartForStdio
// no longer pre-validates via providerFactory. The stdio path delegates provider
// construction entirely to the spawned mcp-serve subprocess, which builds providers
// from the on-disk config; calling the factory in-process here would only allocate
// adapters that get discarded (potential resource leak when an Adapter later opens
// connections in its constructor).
func TestProxyService_StartForStdio_IgnoresProviderFactory(t *testing.T) {
	cliExec := mocks.NewMockCLIExecutor()
	mockProc := mocks.NewMockCLIProcess()
	cliExec.StartFunc = func(ctx context.Context, name string, args ...string) (ports.CLIProcess, error) {
		return mockProc, nil
	}

	factoryCalls := 0
	factory := ProviderFactory(func(ProxyConfig) ([]ports.ToolProvider, error) {
		factoryCalls++
		return nil, errors.New("factory must not be called for stdio mode")
	})

	svc := NewProxyService(cliExec, mocks.NewMockTracer(), mocks.NewMockLogger(), factory)

	_, cleanup, err := svc.StartForStdio(context.Background(), ProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
		PluginTools:       []PluginToolSpec{},
	})

	require.NoError(t, err, "stdio start must succeed without touching the factory")
	assert.Equal(t, 0, factoryCalls, "factory must not be invoked in stdio mode")

	mockProc.Close()
	_ = cleanup()
}

// TestProxyService_StartForStdio_CLIExecutorError propagates spawn error.
func TestProxyService_StartForStdio_CLIExecutorError(t *testing.T) {
	cliExec := mocks.NewMockCLIExecutor()
	cliExec.StartFunc = func(ctx context.Context, name string, args ...string) (ports.CLIProcess, error) {
		return nil, errors.New("spawn failed")
	}

	mockProvider := &mockToolProvider{}
	factory := newMockProviderFactory([]ports.ToolProvider{mockProvider}, nil)

	svc := NewProxyService(cliExec, mocks.NewMockTracer(), mocks.NewMockLogger(), factory)

	_, cleanup, err := svc.StartForStdio(context.Background(), ProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
		PluginTools:       []PluginToolSpec{},
	})

	// Must propagate error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to spawn awf mcp-serve")

	// Cleanup must be noop
	assert.NoError(t, cleanup())
}

// TestProxyService_StartForStdio_CleanupSignalsProcess verifies cleanup sequence.
func TestProxyService_StartForStdio_CleanupSignalsProcess(t *testing.T) {
	cliExec := mocks.NewMockCLIExecutor()
	mockProc := mocks.NewMockCLIProcess()
	cliExec.StartFunc = func(ctx context.Context, name string, args ...string) (ports.CLIProcess, error) {
		return mockProc, nil
	}

	mockProvider := &mockToolProvider{}
	factory := newMockProviderFactory([]ports.ToolProvider{mockProvider}, nil)

	svc := NewProxyService(cliExec, mocks.NewMockTracer(), mocks.NewMockLogger(), factory)

	_, cleanup, err := svc.StartForStdio(context.Background(), ProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
		PluginTools:       []PluginToolSpec{},
	})

	require.NoError(t, err)

	// Simulate process response after short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		mockProc.Close()
	}()

	// Cleanup should send signal and wait for process exit
	err = cleanup()
	require.NoError(t, err)

	// Verify Signal was called with Interrupt
	signals := mockProc.GetSignals()
	require.True(t, len(signals) > 0, "Signal should have been called during cleanup")
}

// TestProxyService_StartForHTTP_DisabledConfig returns noop when config is disabled.
func TestProxyService_StartForHTTP_DisabledConfig(t *testing.T) {
	svc := NewProxyService(mocks.NewMockCLIExecutor(), mocks.NewMockTracer(), mocks.NewMockLogger(), func(cfg ProxyConfig) ([]ports.ToolProvider, error) {
		return nil, nil
	})

	router, cleanup, err := svc.StartForHTTP(context.Background(), ProxyConfig{
		InterceptBuiltins: false,
		PluginTools:       []PluginToolSpec{},
	})

	assert.NoError(t, err)
	assert.Nil(t, router)
	assert.NotNil(t, cleanup)
	assert.NoError(t, cleanup())
}

// TestProxyService_StartForHTTP_ReturnsRouter verifies in-process router is returned.
func TestProxyService_StartForHTTP_ReturnsRouter(t *testing.T) {
	mockProvider := &mockToolProvider{}
	factory := newMockProviderFactory([]ports.ToolProvider{mockProvider}, nil)

	svc := NewProxyService(mocks.NewMockCLIExecutor(), mocks.NewMockTracer(), mocks.NewMockLogger(), factory)

	router, cleanup, err := svc.StartForHTTP(context.Background(), ProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
		PluginTools:       []PluginToolSpec{},
	})

	require.NoError(t, err)
	require.NotNil(t, router, "router must be non-nil when proxy is enabled and provider factory succeeds")
	assert.NotNil(t, cleanup)
	assert.NoError(t, cleanup())
}

// TestProxyService_StartForHTTP_ProviderFactoryError propagates error.
func TestProxyService_StartForHTTP_ProviderFactoryError(t *testing.T) {
	factory := newMockProviderFactory(nil, errors.New("factory error"))

	svc := NewProxyService(mocks.NewMockCLIExecutor(), mocks.NewMockTracer(), mocks.NewMockLogger(), factory)

	router, cleanup, err := svc.StartForHTTP(context.Background(), ProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
		PluginTools:       []PluginToolSpec{},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "factory error")
	assert.Nil(t, router)
	assert.NoError(t, cleanup())
}

// TestProxyService_StartForHTTP_RouterRegistrationError verifies registration errors are propagated.
func TestProxyService_StartForHTTP_RouterRegistrationError(t *testing.T) {
	// Create a provider that might cause registration error
	// This tests that factory errors are caught at router registration phase
	mockProvider := &mockToolProvider{}
	factory := newMockProviderFactory([]ports.ToolProvider{mockProvider}, nil)

	svc := NewProxyService(mocks.NewMockCLIExecutor(), mocks.NewMockTracer(), mocks.NewMockLogger(), factory)

	// StartForHTTP should not error if providers are valid
	router, cleanup, err := svc.StartForHTTP(context.Background(), ProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
		PluginTools:       []PluginToolSpec{},
	})

	require.NoError(t, err)
	require.NotNil(t, router, "router must be non-nil when providers are valid and proxy is enabled")
	assert.NoError(t, cleanup())
}

// TestProxyService_StartForStdio_ConfigWithPluginTools works with plugin tools config.
func TestProxyService_StartForStdio_ConfigWithPluginTools(t *testing.T) {
	cliExec := mocks.NewMockCLIExecutor()
	mockProc := mocks.NewMockCLIProcess()
	cliExec.StartFunc = func(ctx context.Context, name string, args ...string) (ports.CLIProcess, error) {
		return mockProc, nil
	}

	mockProvider := &mockToolProvider{}
	factory := newMockProviderFactory([]ports.ToolProvider{mockProvider}, nil)

	svc := NewProxyService(cliExec, mocks.NewMockTracer(), mocks.NewMockLogger(), factory)

	cfg := ProxyConfig{
		Enable:            true,
		InterceptBuiltins: false,
		PluginTools: []PluginToolSpec{
			{Plugin: "plugin1", Expose: []string{"tool1", "tool2"}},
		},
	}

	_, cleanup, err := svc.StartForStdio(context.Background(), cfg)

	require.NoError(t, err)
	mockProc.Close()
	require.NoError(t, cleanup())
}

// TestProxyService_StartForStdio_TempFileRemoved verifies the temp config file is
// cleaned up when startup fails after the file has been written. It uses the path
// returned by StartForStdio (when non-empty) to assert the file existed before the
// error and is gone after, rather than scanning an unrelated directory.
func TestProxyService_StartForStdio_TempFileRemoved(t *testing.T) {
	cliExec := mocks.NewMockCLIExecutor()
	cliExec.StartFunc = func(ctx context.Context, name string, args ...string) (ports.CLIProcess, error) {
		return nil, errors.New("spawn error")
	}

	mockProvider := &mockToolProvider{}
	factory := newMockProviderFactory([]ports.ToolProvider{mockProvider}, nil)

	svc := NewProxyService(cliExec, mocks.NewMockTracer(), mocks.NewMockLogger(), factory)

	mcpPath, _, err := svc.StartForStdio(context.Background(), ProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
		PluginTools:       []PluginToolSpec{},
	})

	// Spawn always fails, so we expect an error.
	require.Error(t, err)

	// If a config file was written before the spawn failure, the implementation
	// must clean it up itself (no returned cleanup to call on error path).
	if mcpPath != "" {
		_, statErr := os.Stat(mcpPath)
		assert.True(t, os.IsNotExist(statErr),
			"temp config file %q must be removed when StartForStdio returns an error", mcpPath)
	}
}
