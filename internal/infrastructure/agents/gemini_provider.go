package agents

import (
	"context"
	"encoding/json"
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
		parseStreamLine:       p.parseGeminiStreamLine,
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

	// Gemini CLI is always invoked with --output-format stream-json.
	// For text intent (default), aggregate assistant content for state.Output;
	// for json intent, keep raw NDJSON and expose parsed result in Response.
	userFormat, _ := getStringOption(options, "output_format")
	if userFormat == "json" || userFormat == "stream-json" {
		if jsonResp := tryParseJSONResponse(rawOutput); jsonResp != nil {
			result.Response = jsonResp
		}
	} else {
		if extracted := extractDisplayText(rawOutput, p.parseGeminiStreamLine); extracted != "" {
			result.Output = extracted
			result.Tokens = estimateTokens(extracted)
		}
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
	// Always force stream-json NDJSON at the CLI level so the F082 display filter
	// and text extraction have a consistent wire format (F082, aligned with Claude).
	args = append([]string{"--output-format", "stream-json"}, args...)
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

// parseGeminiStreamLine extracts displayable assistant text from Gemini CLI's
// stream-json output. Gemini CLI (`gemini --output-format stream-json -p`) emits
// one JSON object per line with these top-level types:
//   - "init"    — {session_id, model} (ignored)
//   - "message" — {role, content, delta?} (surface role=="assistant")
//   - "result"  — {status, stats} (ignored)
//
// Only assistant messages are surfaced. User echoes and metadata are skipped.
func (p *GeminiProvider) parseGeminiStreamLine(line []byte) string {
	var evt struct {
		Type    string `json:"type"`
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(line, &evt); err != nil {
		return ""
	}
	if evt.Type != "message" || evt.Role != "assistant" {
		return ""
	}
	return evt.Content
}
