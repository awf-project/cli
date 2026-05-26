package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os/exec"
	"strings"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/logger"
	"github.com/awf-project/cli/pkg/interpolation"
)

// CodexProvider implements AgentProvider for Codex CLI.
// Invokes: codex exec --json "prompt"
type CodexProvider struct {
	base      *baseCLIProvider
	logger    ports.Logger
	executor  ports.CLIExecutor
	tokenizer ports.Tokenizer
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
	b := newBaseCLIProvider("codex", "codex", p.executor, p.logger, cliProviderHooks{
		buildExecuteArgs:      p.buildExecuteArgs,
		buildConversationArgs: p.buildConversationArgs,
		extractSessionID:      p.extractSessionID,
		validateOptions:       validateCodexOptions,
		parseDisplayEvents:    p.parseCodexDisplayEvents,
		extractTokenUsage:     p.extractCodexTokenUsage,
		mcpInjector:           p.codexMCPInjector,
	})
	if p.tokenizer != nil {
		b.tokenizer = p.tokenizer
	}
	return b
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
			if result.TokensEstimated {
				tokens, _ := p.base.tokenizer.CountTokens(extracted) //nolint:errcheck // ApproximationTokenizer never errors with a valid ratio
				result.Tokens = tokens
			}
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
		effectivePrompt := buildFirstTurnPrompt(prompt, options)
		args = []string{"exec", "--json", effectivePrompt}
	}
	args = appendCodexOptions(args, options)
	return args, nil
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

func (p *CodexProvider) extractCodexTokenUsage(rawOutput string) *tokenUsage {
	evt := findFirstNDJSONEvent(rawOutput, "turn.completed")
	if evt == nil {
		return nil
	}
	usageVal, ok := evt["usage"]
	if !ok || usageVal == nil {
		return nil
	}
	usage, ok := usageVal.(map[string]any)
	if !ok {
		return nil
	}
	input := intFromMap(usage, "input_tokens")
	output := intFromMap(usage, "output_tokens")
	return &tokenUsage{
		InputTokens:  input,
		OutputTokens: output,
		TotalTokens:  input + output,
	}
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

func (p *CodexProvider) codexMCPInjector(_ context.Context, args []string, cfg *workflow.MCPProxyConfig, mcpConfigPath string, options map[string]any) (newArgs []string, newOptions map[string]any, cleanup func() error, err error) {
	if cfg == nil {
		return args, options, noopMCPCleanup, nil
	}

	exe := resolvedExecutable()
	// interpolation.ShellEscape produces a POSIX-safe single-quoted string, matching the
	// quoting strategy used by Gemini's MCP injector (see gemini_provider.go).
	// %q (Go syntax double-quoting) is not POSIX-shell-safe: backslash escapes
	// differ and the result breaks on shells other than bash in --norc mode.
	commandArg := "mcp_servers.awf-proxy.command=" + interpolation.ShellEscape(exe)
	argsJSON, marshalErr := json.Marshal([]string{"mcp-serve", "--config=" + mcpConfigPath})
	if marshalErr != nil {
		return nil, options, noopMCPCleanup, fmt.Errorf("marshal codex mcp args: %w", marshalErr)
	}
	argsArg := fmt.Sprintf(`mcp_servers.awf-proxy.args=%s`, argsJSON)

	newArgs = make([]string, len(args), len(args)+6)
	copy(newArgs, args)
	newArgs = append(newArgs, "-c", commandArg, "-c", argsArg)

	// Clone options so we don't mutate the caller's map.
	newOpts := make(map[string]any, len(options)+1)
	maps.Copy(newOpts, options)

	if cfg.InterceptBuiltins {
		// -s read-only: restrict Codex to read-only sandbox mode as best-effort
		// mitigation when intercept_builtins=true (coexistence mode, not full enforcement).
		newArgs = append(newArgs, "-s", "read-only")
		p.logger.Warn("mcp_proxy on provider=codex runs in coexistence mode; built-in tools are not blocked")

		// Prepend MCP-only instruction to system_prompt (coexistence mitigation — T011 AC).
		// This guides the model to prefer MCP tools when intercept_builtins=true but
		// native tool blocking is unavailable (Codex has no --tools="" equivalent).
		const mcpOnlyPrefix = "Use only MCP tools, never built-in tools. "
		existing, _ := getStringOption(newOpts, "system_prompt")
		newOpts["system_prompt"] = mcpOnlyPrefix + existing
	}

	return newArgs, newOpts, noopMCPCleanup, nil
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
			preview := extractArgPreview(evt.Item.Arguments)
			return []DisplayEvent{{Type: evt.Type, Kind: EventToolUse, Name: evt.Item.Name, Arg: preview, ID: ""}}
		}
	}
	return nil
}
