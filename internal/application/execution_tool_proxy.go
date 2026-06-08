package application

import (
	"context"
	"fmt"

	"github.com/awf-project/cli/internal/application/tools"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// openAICompatibleProviderName matches the resolved provider string for the OpenAI-
// compatible HTTP transport. The HTTP MCP-proxy path (StartForHTTP) wires an in-process
// ToolRouter consumed by the provider's multi-turn tool-call loop (T012).
const openAICompatibleProviderName = "openai_compatible"

// toolRouterSetter is implemented by providers that accept an in-process ToolRouter.
// OpenAICompatibleProvider implements this interface for the HTTP MCP proxy path (T012).
// tools.Router satisfies ports.ToolRouter structurally (ListTools + CallTool), so the
// router constructed in application can be injected without any agents-package import.
type toolRouterSetter interface {
	SetToolRouter(r ports.ToolRouter)
}

// startToolProxy starts the MCP tool proxy for the step when configured and wires the
// resulting temp-config path into the agent options map. Returns a cleanup function the
// caller must invoke after provider.Execute / ExecuteConversation returns (success or
// failure path). When the proxy is disabled, unset, or the provider does not yet support
// tool interception, returns a no-op cleanup and nil error.
func (s *ExecutionService) startToolProxy(
	ctx context.Context,
	step *workflow.Step,
	opts map[string]any,
	resolvedProvider string,
	provider ports.AgentProvider,
	execCtx *workflow.ExecutionContext,
) (cleanup func() error, err error) {
	// F106 FR-008: capture in-process (HTTP router) tool calls with fidelity:"router",
	// correlated to this run. recorderFor(ctx) routes sub-workflow tool calls to the
	// child transcript; execCtx.WorkflowID is the unified run id.
	return startToolProxyImpl(ctx, s.toolProxy, s.logger, step, opts, resolvedProvider, provider, s.recorderFor(ctx), execCtx.WorkflowID)
}

// startConversationToolProxy starts the MCP tool proxy for a conversation step. It is
// the conversation-manager counterpart of ExecutionService.startToolProxy; both delegate
// to the shared startToolProxyImpl so behavior stays identical across entry points.
func startConversationToolProxy(
	ctx context.Context,
	proxy *tools.ProxyService,
	logger ports.Logger,
	step *workflow.Step,
	opts map[string]any,
	resolvedProvider string,
	provider ports.AgentProvider,
	recorder ports.Recorder,
	runID string,
) (cleanup func() error, err error) {
	return startToolProxyImpl(ctx, proxy, logger, step, opts, resolvedProvider, provider, recorder, runID)
}

// startToolProxyImpl contains the actual start logic shared by single-turn and
// conversation entry points. Splitting it out keeps the call sites readable and ensures
// any policy change (e.g., HTTP vs stdio path selection) lands in exactly one place.
// recorder/runID wire the in-process ToolRouter for F106 router-fidelity tool capture.
func startToolProxyImpl(
	ctx context.Context,
	proxy *tools.ProxyService,
	logger ports.Logger,
	step *workflow.Step,
	opts map[string]any,
	resolvedProvider string,
	provider ports.AgentProvider,
	recorder ports.Recorder,
	runID string,
) (func() error, error) {
	if proxy == nil || step.MCPProxy == nil || !step.MCPProxy.Enable {
		return func() error { return nil }, nil
	}

	cfg := mcpProxyConfigToApp(step.MCPProxy)

	// OpenAI Compatible uses an in-process ToolRouter (HTTP path) instead of the stdio subprocess.
	// Wire the router directly into the provider via SetToolRouter and set MCPProxyConfigKey.
	if resolvedProvider == openAICompatibleProviderName {
		router, routerCleanup, startErr := proxy.StartForHTTP(ctx, cfg)
		if startErr != nil {
			return func() error { return nil }, fmt.Errorf("start tool proxy (http): %w", startErr)
		}
		if router != nil {
			// F106 FR-008: capture tool.call/tool.result at the router seam.
			router.SetRecorder(recorder)
			router.SetRunID(runID)
			if setter, ok := provider.(toolRouterSetter); ok {
				setter.SetToolRouter(router)
			} else {
				logger.Warn("openai_compatible provider does not implement toolRouterSetter; tool routing disabled",
					"step", step.Name)
			}
		}
		opts[workflow.MCPProxyConfigKey] = step.MCPProxy
		return routerCleanup, nil
	}

	// Stdio path for all other providers (Claude, Gemini, Codex, OpenCode).
	mcpConfigPath, proxyCleanup, startErr := proxy.StartForStdio(ctx, cfg)
	if startErr != nil {
		return func() error { return nil }, fmt.Errorf("start tool proxy: %w", startErr)
	}

	opts[workflow.MCPProxyConfigKey] = step.MCPProxy
	if mcpConfigPath != "" {
		opts[workflow.MCPProxyConfigPathKey] = mcpConfigPath
	}

	return proxyCleanup, nil
}

// mcpProxyConfigToApp converts the domain-level MCPProxyConfig to the application-level
// ProxyConfig consumed by ToolProxyService. The conversion is total (no nil branches)
// because callers gate on step.MCPProxy != nil before invoking the helper.
func mcpProxyConfigToApp(cfg *workflow.MCPProxyConfig) tools.ProxyConfig {
	pluginTools := make([]tools.PluginToolSpec, len(cfg.PluginTools))
	for i, pt := range cfg.PluginTools {
		pluginTools[i] = tools.PluginToolSpec{
			Plugin: pt.Plugin,
			Expose: pt.Expose,
		}
	}
	return tools.ProxyConfig{
		Enable:            cfg.Enable,
		InterceptBuiltins: cfg.InterceptBuiltins,
		PluginTools:       pluginTools,
	}
}
