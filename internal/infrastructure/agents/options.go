package agents

import "github.com/vanoix/awf/internal/domain/ports"

type ClaudeProviderOption func(*ClaudeProvider)

func WithClaudeExecutor(executor ports.CLIExecutor) ClaudeProviderOption {
	return func(p *ClaudeProvider) {
		p.executor = executor
	}
}

type GeminiProviderOption func(*GeminiProvider)

func WithGeminiExecutor(executor ports.CLIExecutor) GeminiProviderOption {
	return func(p *GeminiProvider) {
		p.executor = executor
	}
}

type CodexProviderOption func(*CodexProvider)

func WithCodexExecutor(executor ports.CLIExecutor) CodexProviderOption {
	return func(p *CodexProvider) {
		p.executor = executor
	}
}

type OpenCodeProviderOption func(*OpenCodeProvider)

func WithOpenCodeExecutor(executor ports.CLIExecutor) OpenCodeProviderOption {
	return func(p *OpenCodeProvider) {
		p.executor = executor
	}
}

type CustomProviderOption func(*CustomProvider)

func WithCustomExecutor(executor ports.CLIExecutor) CustomProviderOption {
	return func(p *CustomProvider) {
		p.executor = executor
	}
}
