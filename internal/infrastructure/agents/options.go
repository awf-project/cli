package agents

import "github.com/vanoix/awf/internal/domain/ports"

// ClaudeProviderOption configures a ClaudeProvider.
type ClaudeProviderOption func(*ClaudeProvider)

// WithClaudeExecutor sets a custom CLI executor for ClaudeProvider.
func WithClaudeExecutor(executor ports.CLIExecutor) ClaudeProviderOption {
	return func(p *ClaudeProvider) {
		p.executor = executor
	}
}

// GeminiProviderOption configures a GeminiProvider.
type GeminiProviderOption func(*GeminiProvider)

// WithGeminiExecutor sets a custom CLI executor for GeminiProvider.
func WithGeminiExecutor(executor ports.CLIExecutor) GeminiProviderOption {
	return func(p *GeminiProvider) {
		p.executor = executor
	}
}

// CodexProviderOption configures a CodexProvider.
type CodexProviderOption func(*CodexProvider)

// WithCodexExecutor sets a custom CLI executor for CodexProvider.
func WithCodexExecutor(executor ports.CLIExecutor) CodexProviderOption {
	return func(p *CodexProvider) {
		p.executor = executor
	}
}

// OpenCodeProviderOption configures an OpenCodeProvider.
type OpenCodeProviderOption func(*OpenCodeProvider)

// WithOpenCodeExecutor sets a custom CLI executor for OpenCodeProvider.
func WithOpenCodeExecutor(executor ports.CLIExecutor) OpenCodeProviderOption {
	return func(p *OpenCodeProvider) {
		p.executor = executor
	}
}

// CustomProviderOption configures a CustomProvider.
type CustomProviderOption func(*CustomProvider)

// WithCustomExecutor sets a custom CLI executor for CustomProvider.
func WithCustomExecutor(executor ports.CLIExecutor) CustomProviderOption {
	return func(p *CustomProvider) {
		p.executor = executor
	}
}
