package application_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application/tools"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/testutil/mocks"
)

// Component: T006
// Feature: F099

// TestSetToolProxyService_AcceptsInterface verifies that SetToolProxyService
// accepts a ProxyService instance, following the established Set*() DI pattern.
func TestSetToolProxyService_AcceptsInterface(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	proxyService := tools.NewProxyService(
		mocks.NewMockCLIExecutor(),
		mocks.NewMockTracer(),
		mocks.NewMockLogger(),
		func(cfg tools.ProxyConfig) ([]ports.ToolProvider, error) { return nil, nil },
	)

	execSvc.SetToolProxyService(proxyService)

	assert.NotNil(t, execSvc)
}

// TestSetToolProxyService_AcceptsNil verifies that SetToolProxyService can accept nil,
// which disables proxy behavior while keeping existing flows unaffected.
func TestSetToolProxyService_AcceptsNil(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	execSvc.SetToolProxyService(nil)

	assert.NotNil(t, execSvc)
}

// TestSetToolProxyService_SupportsReassignment verifies that SetToolProxyService
// can be called multiple times without panicking. This exercises the happy path
// for the DI pattern: first assignment, reassignment, and nil-after-set.
// The tests do not assert on the stored field directly (no exported getter) to
// avoid polluting the public API; behavior-level tests in execution_tool_proxy_test.go
// validate that the proxy is actually used when configured.
func TestSetToolProxyService_SupportsReassignment(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	first := tools.NewProxyService(
		mocks.NewMockCLIExecutor(),
		mocks.NewMockTracer(),
		mocks.NewMockLogger(),
		func(cfg tools.ProxyConfig) ([]ports.ToolProvider, error) { return nil, nil },
	)
	second := tools.NewProxyService(
		mocks.NewMockCLIExecutor(),
		mocks.NewMockTracer(),
		mocks.NewMockLogger(),
		func(cfg tools.ProxyConfig) ([]ports.ToolProvider, error) { return nil, nil },
	)

	// Must not panic on first assignment.
	require.NotPanics(t, func() { execSvc.SetToolProxyService(first) }, "first assignment must not panic")
	// Must not panic on reassignment.
	require.NotPanics(t, func() { execSvc.SetToolProxyService(second) }, "reassignment must not panic")
}

// TestSetToolProxyService_NilAfterSet verifies that nil can be set after a previous value,
// which disables proxy behavior. The call must not panic.
func TestSetToolProxyService_NilAfterSet(t *testing.T) {
	execSvc, _ := NewTestHarness(t).Build()

	proxyService := tools.NewProxyService(
		mocks.NewMockCLIExecutor(),
		mocks.NewMockTracer(),
		mocks.NewMockLogger(),
		func(cfg tools.ProxyConfig) ([]ports.ToolProvider, error) { return nil, nil },
	)

	require.NotPanics(t, func() { execSvc.SetToolProxyService(proxyService) }, "set must not panic")
	require.NotPanics(t, func() { execSvc.SetToolProxyService(nil) }, "setting nil must not panic")
}
