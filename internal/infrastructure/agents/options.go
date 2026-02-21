package agents

import (
	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/pkg/httputil"
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

type OpenCodeProviderOption func(*OpenCodeProvider)

func WithOpenCodeExecutor(executor ports.CLIExecutor) OpenCodeProviderOption {
	return func(p *OpenCodeProvider) {
		p.executor = executor
	}
}

type OpenAICompatibleProviderOption func(*OpenAICompatibleProvider)

func WithHTTPClient(client *httputil.Client) OpenAICompatibleProviderOption {
	return func(p *OpenAICompatibleProvider) {
		p.httpClient = client
	}
}
