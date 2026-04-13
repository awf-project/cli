package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/logger"
)

// ClaudeProvider implements AgentProvider for Claude CLI.
// Invokes: claude -p "prompt" --output-format stream-json
type ClaudeProvider struct {
	base     *baseCLIProvider
	logger   ports.Logger
	executor ports.CLIExecutor
}

func NewClaudeProvider(l ...ports.Logger) *ClaudeProvider {
	var log ports.Logger
	if len(l) > 0 && l[0] != nil {
		log = l[0]
	} else {
		log = logger.NopLogger{}
	}
	p := &ClaudeProvider{
		logger:   log,
		executor: NewExecCLIExecutor(),
	}
	p.base = p.newBase()
	return p
}

func NewClaudeProviderWithOptions(opts ...ClaudeProviderOption) *ClaudeProvider {
	p := &ClaudeProvider{
		logger:   logger.NopLogger{},
		executor: NewExecCLIExecutor(),
	}
	for _, opt := range opts {
		opt(p)
	}
	p.base = p.newBase()
	return p
}

func (p *ClaudeProvider) newBase() *baseCLIProvider {
	return newBaseCLIProvider("claude", "claude", p.executor, p.logger, cliProviderHooks{
		buildExecuteArgs:      p.buildExecuteArgs,
		buildConversationArgs: p.buildConversationArgs,
		extractSessionID:      p.extractSessionID,
		extractTextContent:    p.extractTextFromJSON,
		validateOptions:       validateClaudeOptions,
	})
}

func (p *ClaudeProvider) Execute(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
	result, rawOutput, err := p.base.execute(ctx, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}

	userFormat, userFormatSet := getStringOption(options, "output_format")

	// When stream-json was forced internally (user didn't set output_format),
	// extract clean text from the NDJSON result event for downstream steps.
	if !userFormatSet {
		if extracted := p.extractTextFromJSON(rawOutput); extracted != "" {
			result.Output = extracted
			result.Tokens = estimateTokens(extracted)
		}
	}

	// Populate Response only when user explicitly requested structured output.
	if userFormatSet && (userFormat == "json" || userFormat == "stream-json") {
		if jsonResp := p.extractResultEvent(rawOutput); jsonResp != nil {
			result.Response = jsonResp
		}
	}

	return result, nil
}

// ExecuteConversation invokes the Claude CLI with conversation history for multi-turn interactions.
func (p *ClaudeProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
	result, rawOutput, err := p.base.executeConversation(ctx, state, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}

	// Populate Response only when user explicitly requested output_format: json.
	// The JSON wrapper (session_id, cost_usd, etc.) must NOT leak into workflow state.
	userFormat, userFormatSet := getStringOption(options, "output_format")
	if userFormatSet && userFormat == "json" {
		if jsonResp := p.extractResultEvent(rawOutput); jsonResp != nil {
			result.Response = jsonResp
		}
	}

	return result, nil
}

func (p *ClaudeProvider) Name() string {
	return "claude"
}

func (p *ClaudeProvider) Validate() error {
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found in PATH: %w", err)
	}
	return nil
}

// buildExecuteArgs constructs CLI arguments for a single-turn Execute call.
func (p *ClaudeProvider) buildExecuteArgs(prompt string, options map[string]any) ([]string, error) {
	args := []string{"-p", prompt}

	if model, ok := getStringOption(options, "model"); ok {
		args = append(args, "--model", model)
	}

	// Force stream-json (NDJSON events) unless the user explicitly set output_format: text.
	// stream-json requires --verbose in -p mode for live streaming.
	userFormat, userFormatSet := getStringOption(options, "output_format")
	switch {
	case !userFormatSet, userFormat == "json", userFormat == "stream-json":
		args = append(args, "--output-format", "stream-json", "--verbose")
	default:
		args = append(args, "--output-format", userFormat)
	}

	if tools, ok := getStringOption(options, "allowed_tools"); ok && tools != "" {
		args = append(args, "--allowedTools", tools)
	}

	if skip, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skip {
		args = append(args, "--dangerously-skip-permissions")
	}

	return args, nil
}

// buildConversationArgs constructs CLI arguments for a multi-turn ExecuteConversation call.
func (p *ClaudeProvider) buildConversationArgs(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
	args := []string{"-p", prompt}

	if model, ok := getStringOption(options, "model"); ok {
		args = append(args, "--model", model)
	}

	// Force stream-json output for session ID extraction on all conversation turns.
	// stream-json requires --verbose when combined with --print (-p).
	args = append(args, "--output-format", "stream-json", "--verbose")

	if state != nil && state.SessionID != "" {
		args = append(args, "-r", state.SessionID)
	} else {
		// First turn only: pass system prompt if provided.
		// On turns 2+, the provider retains the system prompt from the resumed session.
		if sysPrompt, ok := getStringOption(options, "system_prompt"); ok && sysPrompt != "" {
			args = append(args, "--system-prompt", sysPrompt)
		}
	}

	if tools, ok := getStringOption(options, "allowed_tools"); ok && tools != "" {
		args = append(args, "--allowedTools", tools)
	}

	if skip, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skip {
		args = append(args, "--dangerously-skip-permissions")
	}

	return args, nil
}

func validateClaudeOptions(options map[string]any) error {
	if options == nil {
		return nil
	}

	if model, ok := getStringOption(options, "model"); ok {
		if !isValidClaudeModel(model) {
			return fmt.Errorf("invalid model format: %s (must be an alias or start with 'claude-')", model)
		}
	}

	return nil
}

func isValidClaudeModel(model string) bool {
	aliases := []string{"sonnet", "opus", "haiku"}
	return slices.Contains(aliases, model) || strings.HasPrefix(model, "claude-")
}

// extractResultEvent scans NDJSON stream-json output and returns the final
// {"type":"result", ...} event as a parsed map, or nil if absent. Each line of
// claude's stream-json is a standalone JSON object (system, assistant, result,
// etc.); the "result" event is the authoritative final summary.
func (p *ClaudeProvider) extractResultEvent(output string) map[string]any {
	if output == "" {
		return nil
	}
	var found map[string]any
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var evt map[string]any
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}
		if t, ok := evt["type"].(string); ok && t == "result" {
			found = evt
		}
	}
	return found
}

func (p *ClaudeProvider) extractSessionID(output string) (string, error) {
	if output == "" {
		return "", errors.New("empty output")
	}
	evt := p.extractResultEvent(output)
	if evt == nil {
		return "", errors.New("result event not found")
	}
	sessionIDVal, ok := evt["session_id"]
	if !ok || sessionIDVal == nil {
		return "", errors.New("session_id missing")
	}
	if str, ok := sessionIDVal.(string); ok && str != "" {
		return str, nil
	}
	return "", errors.New("session_id is not a string")
}

func (p *ClaudeProvider) extractTextFromJSON(output string) string {
	evt := p.extractResultEvent(output)
	if evt == nil {
		return ""
	}
	result, ok := evt["result"]
	if !ok || result == nil {
		return ""
	}
	if str, ok := result.(string); ok {
		return str
	}
	if num, ok := result.(float64); ok {
		if num == float64(int64(num)) {
			return fmt.Sprintf("%.0f", num)
		}
		return fmt.Sprint(num)
	}
	return ""
}
