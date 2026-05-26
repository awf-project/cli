package workflow

import (
	domerrors "github.com/awf-project/cli/internal/domain/errors"
)

// MCPProxyConfigPathKey is the agent-options map key carrying the tmp MCP config file
// path written by ToolProxyService.StartForStdio. The application layer sets it before
// invoking provider.Execute; infrastructure provider injectors read it to build CLI flags.
// Defined in the domain layer so both application and infrastructure reference the same
// constant without crossing a forbidden import boundary.
const MCPProxyConfigPathKey = "mcp_proxy_config_path"

// MCPProxyConfigKey is the agent-options map key carrying the *MCPProxyConfig value
// active for the current step. Consumers retrieve it with a type assertion to *MCPProxyConfig.
const MCPProxyConfigKey = "mcp_proxy_config"

// PluginToolExpose specifies which operations of a plugin to expose via MCP proxy.
type PluginToolExpose struct {
	Plugin string   `yaml:"plugin"`
	Expose []string `yaml:"expose"`
}

// MCPProxyConfig configures MCP tool interception for an agent step.
type MCPProxyConfig struct {
	Enable            bool               `yaml:"enable"`
	InterceptBuiltins bool               `yaml:"intercept_builtins"`
	PluginTools       []PluginToolExpose `yaml:"plugin_tools"`
}

// Validate checks MCPProxyConfig structural correctness.
// Returns nil when Enable is false or when no errors are found.
func (m *MCPProxyConfig) Validate() []ValidationError {
	if !m.Enable {
		return nil
	}

	var errs []ValidationError

	// EMPTY_PROXY: enable=true && intercept_builtins=false && no plugin_tools
	if !m.InterceptBuiltins && len(m.PluginTools) == 0 {
		errs = append(errs, ValidationError{
			Level:   ValidationLevelError,
			Code:    ValidationCode(domerrors.ErrorCodeUserMCPProxyEmptyProxy),
			Message: "MCP proxy enabled with intercept_builtins=false but no plugin_tools specified",
		})
	}

	// NAME_COLLISION: duplicate Plugin entries in PluginTools
	seen := make(map[string]bool, len(m.PluginTools))
	for _, tool := range m.PluginTools {
		if seen[tool.Plugin] {
			errs = append(errs, ValidationError{
				Level:   ValidationLevelError,
				Code:    ValidationCode(domerrors.ErrorCodeUserMCPProxyNameCollision),
				Message: "duplicate plugin entry: " + tool.Plugin,
			})
		}
		seen[tool.Plugin] = true
	}

	return errs
}
