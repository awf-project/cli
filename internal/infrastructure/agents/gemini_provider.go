package agents

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// GeminiProvider implements AgentProvider for Gemini CLI.
// Invokes: gemini -p "prompt"
type GeminiProvider struct {
	base     *baseCLIProvider
	executor ports.CLIExecutor
}

func NewGeminiProvider() *GeminiProvider {
	p := &GeminiProvider{
		executor: NewExecCLIExecutor(),
	}
	p.base = p.newBase()
	return p
}

func NewGeminiProviderWithOptions(opts ...GeminiProviderOption) *GeminiProvider {
	p := &GeminiProvider{
		executor: NewExecCLIExecutor(),
	}
	for _, opt := range opts {
		opt(p)
	}
	p.base = p.newBase()
	return p
}

func (p *GeminiProvider) newBase() *baseCLIProvider {
	return newBaseCLIProvider("gemini", "gemini", p.executor, nil, cliProviderHooks{
		buildExecuteArgs:      p.buildExecuteArgs,
		buildConversationArgs: p.buildConversationArgs,
		extractSessionID:      p.extractSessionID,
		validateOptions:       validateGeminiOptions,
	})
}

func validateGeminiOptions(options map[string]any) error {
	if options == nil {
		return nil
	}

	if model, ok := getStringOption(options, "model"); ok {
		if !isValidGeminiModel(model) {
			return fmt.Errorf("invalid model format: %s (must start with 'gemini-')", model)
		}
	}

	return nil
}

func isValidGeminiModel(model string) bool {
	return strings.HasPrefix(model, "gemini-")
}

func (p *GeminiProvider) Execute(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
	result, rawOutput, err := p.base.execute(ctx, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}

	if jsonResp := tryParseJSONResponse(rawOutput); jsonResp != nil {
		result.Response = jsonResp
	}

	return result, nil
}

func (p *GeminiProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
	result, _, err := p.base.executeConversation(ctx, state, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (p *GeminiProvider) Name() string {
	return "gemini"
}

func (p *GeminiProvider) Validate() error {
	_, err := exec.LookPath("gemini")
	if err != nil {
		return fmt.Errorf("gemini CLI not found in PATH: %w", err)
	}
	return nil
}

func (p *GeminiProvider) buildExecuteArgs(prompt string, options map[string]any) ([]string, error) {
	args := []string{"-p", prompt}

	if model, ok := getStringOption(options, "model"); ok {
		args = append([]string{"--model", model}, args...)
	}
	if outputFormat, ok := getStringOption(options, "output_format"); ok {
		if outputFormat == "json" {
			outputFormat = "stream-json"
		}
		args = append([]string{"--output-format", outputFormat}, args...)
	}
	if skipPerms, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skipPerms {
		args = append([]string{"--approval-mode=yolo"}, args...)
	}

	return args, nil
}

func (p *GeminiProvider) buildConversationArgs(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
	// Gemini CLI has no --system-prompt flag; inline the system prompt into
	// the first turn's message. Subsequent turns rely on --resume.
	var args []string
	if state != nil && state.SessionID != "" {
		args = []string{"--resume", state.SessionID, "-p", prompt}
	} else {
		effectivePrompt := prompt
		if sysPrompt, ok := getStringOption(options, "system_prompt"); ok && sysPrompt != "" {
			effectivePrompt = sysPrompt + "\n\n" + prompt
		}
		args = []string{"-p", effectivePrompt}
	}

	if model, ok := getStringOption(options, "model"); ok {
		args = append([]string{"--model", model}, args...)
	}
	// Force stream-json unconditionally for reliable session ID extraction.
	args = append([]string{"--output-format", "stream-json"}, args...)
	if skipPerms, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skipPerms {
		args = append([]string{"--approval-mode=yolo"}, args...)
	}

	return args, nil
}

func (p *GeminiProvider) extractInitEvent(output string) map[string]any {
	return findFirstNDJSONEvent(output, "init")
}

func (p *GeminiProvider) extractSessionID(output string) (string, error) {
	if output == "" {
		return "", errors.New("empty output")
	}
	evt := p.extractInitEvent(output)
	if evt == nil {
		return "", errors.New("init event not found")
	}
	sessionIDVal, ok := evt["session_id"]
	if !ok || sessionIDVal == nil {
		return "", errors.New("session_id missing")
	}
	str, ok := sessionIDVal.(string)
	if !ok {
		return "", errors.New("session_id is not a string")
	}
	if str == "" {
		return "", errors.New("session_id is empty")
	}
	return str, nil
}
