package agents

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/logger"
)

// CodexProvider implements AgentProvider for Codex CLI.
// Invokes: codex exec --json "prompt"
type CodexProvider struct {
	base     *baseCLIProvider
	logger   ports.Logger
	executor ports.CLIExecutor
}

func NewCodexProvider() *CodexProvider {
	p := &CodexProvider{
		logger:   logger.NopLogger{},
		executor: NewExecCLIExecutor(),
	}
	p.base = p.newBase()
	return p
}

func NewCodexProviderWithOptions(opts ...CodexProviderOption) *CodexProvider {
	p := &CodexProvider{
		logger:   logger.NopLogger{},
		executor: NewExecCLIExecutor(),
	}
	for _, opt := range opts {
		opt(p)
	}
	p.base = p.newBase()
	return p
}

func (p *CodexProvider) newBase() *baseCLIProvider {
	return newBaseCLIProvider("codex", "codex", p.executor, p.logger, cliProviderHooks{
		buildExecuteArgs:      p.buildExecuteArgs,
		buildConversationArgs: p.buildConversationArgs,
		extractSessionID:      p.extractSessionID,
	})
}

func (p *CodexProvider) Execute(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
	result, _, err := p.base.execute(ctx, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (p *CodexProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
	result, _, err := p.base.executeConversation(ctx, state, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (p *CodexProvider) Name() string {
	return "codex"
}

func (p *CodexProvider) Validate() error {
	_, err := exec.LookPath("codex")
	if err != nil {
		return fmt.Errorf("codex CLI not found in PATH: %w", err)
	}
	return nil
}

func (p *CodexProvider) buildExecuteArgs(prompt string, options map[string]any) ([]string, error) {
	args := []string{"exec", "--json", prompt}
	args = appendCodexOptions(args, options)
	return args, nil
}

func (p *CodexProvider) buildConversationArgs(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
	var args []string
	if state.SessionID != "" {
		// Resume an existing thread using the native resume subcommand.
		args = []string{"resume", state.SessionID, "--json", prompt}
	} else {
		// Codex CLI has no --system-prompt flag; inline the system prompt into
		// the first-turn message only when a session is not yet established.
		effectivePrompt := buildCodexFirstTurnPrompt(prompt, options)
		args = []string{"exec", "--json", effectivePrompt}
	}
	args = appendCodexOptions(args, options)
	return args, nil
}

// buildCodexFirstTurnPrompt combines an optional system prompt with the user
// prompt for the first turn. Codex CLI has no --system-prompt flag, so the
// system context must be embedded directly in the message.
func buildCodexFirstTurnPrompt(userPrompt string, options map[string]any) string {
	systemPrompt, ok := options["system_prompt"].(string)
	if !ok || systemPrompt == "" {
		return userPrompt
	}
	return systemPrompt + "\n\n" + userPrompt
}

// appendCodexOptions appends supported Codex CLI flags derived from the options map.
// Unknown options are silently ignored to match Codex CLI behavior.
func appendCodexOptions(args []string, options map[string]any) []string {
	if model, ok := options["model"].(string); ok && model != "" {
		args = append(args, "--model", model)
	}
	if skip, ok := options["dangerously_skip_permissions"].(bool); ok && skip {
		args = append(args, "--dangerously-bypass-approvals-and-sandbox")
	}
	return args
}

func (p *CodexProvider) extractThreadStartedEvent(output string) map[string]any {
	return findFirstNDJSONEvent(output, "thread.started")
}

func (p *CodexProvider) extractSessionID(output string) (string, error) {
	if output == "" {
		return "", errors.New("empty output")
	}
	evt := p.extractThreadStartedEvent(output)
	if evt == nil {
		return "", errors.New("thread.started event not found")
	}
	threadIDVal, ok := evt["thread_id"]
	if !ok || threadIDVal == nil {
		return "", errors.New("thread_id missing")
	}
	if str, ok := threadIDVal.(string); ok && str != "" {
		return str, nil
	}
	return "", errors.New("thread_id is not a non-empty string")
}
