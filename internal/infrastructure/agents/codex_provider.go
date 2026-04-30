package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

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
		validateOptions:       validateCodexOptions,
		parseDisplayEvents:    p.parseCodexDisplayEvents,
	})
}

func (p *CodexProvider) Execute(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
	result, rawOutput, err := p.base.execute(ctx, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}

	// Codex CLI is always invoked with `exec --json` (NDJSON). For text intent,
	// aggregate assistant message content for state.Output so downstream
	// interpolation ({{states.step.Output}}) is human-readable (F082).
	userFormat, _ := getStringOption(options, "output_format")
	if userFormat != "json" && userFormat != "stream-json" {
		if extracted := extractDisplayTextFromEvents(rawOutput, p.parseCodexDisplayEvents); extracted != "" {
			result.Output = extracted
			result.Tokens = estimateTokens(extracted)
		}
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

// buildCodexFirstTurnPrompt prepends an optional system prompt for the first turn.
// Codex CLI has no --system-prompt flag, so the system context must be embedded in the message.
func buildCodexFirstTurnPrompt(userPrompt string, options map[string]any) string {
	if sysPrompt, ok := getStringOption(options, "system_prompt"); ok && sysPrompt != "" {
		return sysPrompt + "\n\n" + userPrompt
	}
	return userPrompt
}

// appendCodexOptions appends Codex CLI flags from options; unknown keys are silently ignored.
func appendCodexOptions(args []string, options map[string]any) []string {
	if model, ok := getStringOption(options, "model"); ok && model != "" {
		args = append(args, "--model", model)
	}
	if skip, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skip {
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

func validateCodexOptions(options map[string]any) error {
	if options == nil {
		return nil
	}

	if model, ok := getStringOption(options, "model"); ok {
		if !isValidCodexModel(model) {
			return fmt.Errorf("invalid model format: %s (must start with 'gpt-', 'codex-', or be an o-series model like 'o1', 'o3', 'o4-mini')", model)
		}
	}

	return nil
}

func isValidCodexModel(model string) bool {
	if strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "codex-") {
		return true
	}
	// o-series: "o" followed by a digit (e.g., o1, o3, o4-mini); rejects "ollama", "oracle"
	return len(model) >= 2 && model[0] == 'o' && model[1] >= '0' && model[1] <= '9'
}

// parseCodexDisplayEvents parses a single NDJSON line from Codex CLI output into
// DisplayEvents. It emits EventText for assistant_message items and EventToolUse
// for function_call items. All other event types return nil (skip signal).
func (p *CodexProvider) parseCodexDisplayEvents(line []byte) []DisplayEvent {
	// Replace NUL bytes (0x00) with the 6-byte JSON unicode escape sequence
	// {0x5c,0x75,0x30,0x30,0x30,0x30} = backslash + u + 0 + 0 + 0 + 0.
	// json.Unmarshal decodes this escape back to NUL, preserving string content.
	sanitized := bytes.ReplaceAll(line, []byte{0x00}, []byte{0x5c, 0x75, 0x30, 0x30, 0x30, 0x30})
	var evt struct {
		Type string `json:"type"`
		Item *struct {
			ItemType  string `json:"item_type"`
			Text      string `json:"text"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"item"`
	}
	if err := json.Unmarshal(sanitized, &evt); err != nil {
		return nil
	}
	if evt.Type == "" {
		return nil
	}
	if evt.Type == "item.completed" && evt.Item != nil {
		switch evt.Item.ItemType {
		case "assistant_message":
			return []DisplayEvent{{Type: evt.Type, Kind: EventText, Text: evt.Item.Text}}
		case "function_call":
			// Codex does not emit tool-call IDs; ID is always empty.
			preview := parseToolCallArgPreview(evt.Item.Arguments)
			return []DisplayEvent{{Type: evt.Type, Kind: EventToolUse, Name: evt.Item.Name, Arg: preview, ID: ""}}
		}
	}
	return nil
}
