package tools

// CoexistenceProviders returns the agent provider names that operate the MCP proxy
// in coexistence mode: they register the proxy server but cannot block built-in
// tool access at the CLI level. A fresh slice is returned on each call to prevent
// callers from mutating the canonical list.
//
// This list is the single source of truth for both the static-validation warning
// (application layer) and the runtime warn path (infrastructure providers). It
// lives in the application layer — not the domain — because the values are
// infrastructure provider names ("codex", "copilot", "opencode"), which are
// infrastructure concerns that the domain must not depend on.
func CoexistenceProviders() []string {
	return []string{"codex", "copilot", "opencode"}
}

// ProxyConfig describes what the MCP proxy should expose to clients.
// Enable must be true for the proxy to start; when false, StartForStdio and
// StartForHTTP return a noop immediately without spawning any subprocess.
type ProxyConfig struct {
	Enable            bool
	InterceptBuiltins bool
	PluginTools       []PluginToolSpec
}

// PluginToolSpec describes which tools from a named plugin to expose via the proxy.
//
// The JSON tags must match the format consumed by `awf mcp-serve` (interfaces/cli/mcp_serve.go),
// which reads the on-disk config written by ProxyService.StartForStdio. Renaming a tag here
// without updating the subprocess reader will silently break tool discovery.
type PluginToolSpec struct {
	Plugin string   `json:"plugin"`
	Expose []string `json:"expose"`
}
