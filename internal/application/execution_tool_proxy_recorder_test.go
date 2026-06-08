package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application/tools"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
)

// fakeToolProvider is a minimal ports.ToolProvider exposing one tool for router tests.
type fakeToolProvider struct{}

func (fakeToolProvider) ListTools(context.Context) ([]ports.ToolDefinition, error) {
	return []ports.ToolDefinition{{Name: "ping", Source: "test"}}, nil
}

func (fakeToolProvider) CallTool(context.Context, string, map[string]any) (*ports.ToolResult, error) {
	return &ports.ToolResult{}, nil
}

func (fakeToolProvider) Close(context.Context) error { return nil }

// routerCapturingProvider satisfies ports.AgentProvider (via the nil embedded interface —
// startToolProxyImpl only calls SetToolRouter on the openai_compatible path) and captures
// the in-process router handed to it.
type routerCapturingProvider struct {
	ports.AgentProvider
	captured ports.ToolRouter
}

func (p *routerCapturingProvider) SetToolRouter(r ports.ToolRouter) { p.captured = r }

// TestStartToolProxy_WiresRecorderAndRunIDIntoRouter verifies F106 FR-008: the in-process
// HTTP ToolRouter created for openai_compatible is wired with the run's recorder and run id,
// so tool.call/tool.result are captured with fidelity:"router" and correlated to the run.
func TestStartToolProxy_WiresRecorderAndRunIDIntoRouter(t *testing.T) {
	rec := &fakeRecorder{}
	proxy := tools.NewProxyService(
		mocks.NewMockCLIExecutor(),
		mocks.NewMockTracer(),
		mocks.NewMockLogger(),
		func(tools.ProxyConfig) ([]ports.ToolProvider, error) {
			return []ports.ToolProvider{fakeToolProvider{}}, nil
		},
	)
	prov := &routerCapturingProvider{}
	step := &workflow.Step{Name: "s", MCPProxy: &workflow.MCPProxyConfig{Enable: true, InterceptBuiltins: true}}

	cleanup, err := startToolProxyImpl(
		context.Background(), proxy, mocks.NewMockLogger(), step, map[string]any{},
		"openai_compatible", prov, rec, "run-D",
	)
	require.NoError(t, err)
	defer func() { _ = cleanup() }()

	require.NotNil(t, prov.captured, "the router must be injected into the provider")

	_, err = prov.captured.CallTool(context.Background(), "ping", map[string]any{"x": 1})
	require.NoError(t, err)

	require.Len(t, rec.events, 2, "router must emit tool.call and tool.result")
	assert.Equal(t, transcript.EventTypeToolCall, rec.events[0].Type)
	assert.Equal(t, "run-D", rec.events[0].RunID)
	assert.Equal(t, transcript.EventTypeToolResult, rec.events[1].Type)
	assert.Equal(t, "run-D", rec.events[1].RunID)

	callPayload, ok := rec.events[0].Payload.(*transcript.ToolPayload)
	require.True(t, ok)
	assert.Equal(t, transcript.FidelityRouter, callPayload.Fidelity)
}
