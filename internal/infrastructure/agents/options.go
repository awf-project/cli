package agents

import (
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/pkg/httpx"
)

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

func WithCodexLogger(l ports.Logger) CodexProviderOption {
	return func(p *CodexProvider) {
		p.logger = l
	}
}

type CursorProviderOption func(*CursorProvider)

func WithCursorExecutor(executor ports.CLIExecutor) CursorProviderOption {
	return func(p *CursorProvider) {
		p.executor = executor
	}
}

func WithCursorLogger(l ports.Logger) CursorProviderOption {
	return func(p *CursorProvider) {
		p.logger = l
	}
}

type OpenCodeProviderOption func(*OpenCodeProvider)

func WithOpenCodeExecutor(executor ports.CLIExecutor) OpenCodeProviderOption {
	return func(p *OpenCodeProvider) {
		p.executor = executor
	}
}

func WithOpenCodeLogger(l ports.Logger) OpenCodeProviderOption {
	return func(p *OpenCodeProvider) {
		p.logger = l
	}
}

type OpenAICompatibleProviderOption func(*OpenAICompatibleProvider)

func WithHTTPClient(client *httpx.Client) OpenAICompatibleProviderOption {
	return func(p *OpenAICompatibleProvider) {
		p.httpClient = client
	}
}
