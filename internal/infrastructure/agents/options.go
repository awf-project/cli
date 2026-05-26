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

func WithClaudeLogger(l ports.Logger) ClaudeProviderOption {
	return func(p *ClaudeProvider) {
		p.logger = l
	}
}

func WithClaudeTokenizer(tok ports.Tokenizer) ClaudeProviderOption {
	return func(p *ClaudeProvider) {
		p.tokenizer = tok
	}
}

type GeminiProviderOption func(*GeminiProvider)

func WithGeminiExecutor(executor ports.CLIExecutor) GeminiProviderOption {
	return func(p *GeminiProvider) {
		p.executor = executor
	}
}

func WithGeminiLogger(l ports.Logger) GeminiProviderOption {
	return func(p *GeminiProvider) {
		p.logger = l
	}
}

func WithGeminiTokenizer(tok ports.Tokenizer) GeminiProviderOption {
	return func(p *GeminiProvider) {
		p.tokenizer = tok
	}
}

func WithGeminiDenyAllPolicy(policyPath string) GeminiProviderOption {
	return func(p *GeminiProvider) {
		p.denyAllPolicyPath = policyPath
	}
}

func WithGeminiCommandExecutor(executor ports.CommandExecutor) GeminiProviderOption {
	return func(p *GeminiProvider) {
		p.cmdExecutor = executor
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

func WithCodexTokenizer(tok ports.Tokenizer) CodexProviderOption {
	return func(p *CodexProvider) {
		p.tokenizer = tok
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

func WithOpenCodeTokenizer(tok ports.Tokenizer) OpenCodeProviderOption {
	return func(p *OpenCodeProvider) {
		p.tokenizer = tok
	}
}

type CopilotProviderOption func(*CopilotProvider)

func WithCopilotExecutor(executor ports.CLIExecutor) CopilotProviderOption {
	return func(p *CopilotProvider) {
		p.executor = executor
	}
}

func WithCopilotLogger(l ports.Logger) CopilotProviderOption {
	return func(p *CopilotProvider) {
		p.logger = l
	}
}

func WithCopilotTokenizer(tok ports.Tokenizer) CopilotProviderOption {
	return func(p *CopilotProvider) {
		p.tokenizer = tok
	}
}

type OpenAICompatibleProviderOption func(*OpenAICompatibleProvider)

func WithHTTPClient(client *httpx.Client) OpenAICompatibleProviderOption {
	return func(p *OpenAICompatibleProvider) {
		p.httpClient = client
	}
}
