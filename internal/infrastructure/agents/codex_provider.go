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
	return NewCodexProviderWithOptions()
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
		extractTextContent:    p.extractCodexTextContent,
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

	// F103: presence-aware extraction — when the NDJSON carried an assistant_message event (even an
	// empty one), overwrite Output with the aggregated assistant text and re-derive every dependent
	// field (Response, token estimate) from it, regardless of output_format. Gating all of them on
	// hadText mirrors the result.Output != "" guard in ExecuteConversation: an empty assistant_message
	// must not leave a JSON Response or a token count estimated from the raw NDJSON envelope.
	if extracted, hadText := p.extractCodexAssistantText(rawOutput); hadText {
		result.Output = extracted
		if extracted != "" {
			// Array payloads ([…]) leave Response nil — tryParseJSONResponse matches HasPrefix("{") only.
			if jsonResp := tryParseJSONResponse(extracted); jsonResp != nil {
				result.Response = jsonResp
			}
		}
		if result.TokensEstimated {
			tokens, _ := p.base.tokenizer.CountTokens(extracted) //nolint:errcheck // ApproximationTokenizer never errors with a valid ratio
			result.Tokens = tokens
		}
	}

	return result, nil
}

func (p *CodexProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
	result, rawOutput, err := p.base.executeConversation(ctx, state, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}
	if text, hadText := p.extractCodexAssistantText(rawOutput); hadText {
		// F103: presence-aware overwrite. The base derived Output, the trailing assistant
		// turn's Content, and the estimated token counts from the raw NDJSON envelope; once
		// Output is replaced with the aggregated assistant text we must re-derive all three so
		// they stay consistent (mirrors the token recount performed in Execute). Without this,
		// an empty assistant_message leaves inflated TokensOutput/TokensTotal and a turn whose
		// Content still holds the raw NDJSON, violating the turn.Content == Output invariant.
		result.Output = text
		if result.TokensEstimated {
			tokens, _ := p.base.tokenizer.CountTokens(text) //nolint:errcheck // ApproximationTokenizer never errors with a valid ratio
			result.TokensOutput = tokens
			result.TokensTotal = result.TokensInput + tokens
		}
		if result.State != nil {
			if n := len(result.State.Turns); n > 0 {
				if last := &result.State.Turns[n-1]; last.Role == workflow.TurnRoleAssistant {
					last.Content = text
					if result.TokensEstimated {
						last.Tokens = result.TokensOutput
					}
				}
			}
		}
	}
	if result.Output != "" {
		// F103: array payloads ([…]) leave Response nil — tryParseJSONResponse matches HasPrefix("{") only. F107 will revisit.
		if jsonResp := tryParseJSONResponse(result.Output); jsonResp != nil {
			result.Response = jsonResp
		}
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
		// Resume an existing thread headlessly. The top-level `codex resume` subcommand is the
		// interactive TUI (session picker, no --json); headless JSONL resume lives under
		// `codex exec resume <SESSION_ID> [PROMPT] --json` (codex-cli >= 0.x). Using the TUI
		// resume here makes the CLI reject --json with "unexpected argument '--json'".
		args = []string{"exec", "resume", state.SessionID, "--json", prompt}
	} else {
		// Codex CLI has no --system-prompt flag; inline the system prompt into
		// the first-turn message only when a session is not yet established.
		effectivePrompt := buildFirstTurnPrompt(prompt, options)
		args = []string{"exec", "--json", effectivePrompt}
	}
	args = appendCodexOptions(args, options)
	return args, nil
}

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

// extractCodexTextContent adapts extractCodexAssistantText to the extractTextContent hook
// signature (func(string) string) required by cliProviderHooks. The hadText bool is deliberately
// discarded: the base provider only needs the aggregated text to build outputStr, and it already
// treats "" as "fall back to TrimSpace(rawOutput)".
//
// CONTRACT (do not "simplify" away the ExecuteConversation override): because this hook cannot
// surface hadText, an empty assistant_message (hadText=true, text="") is indistinguishable from
// non-NDJSON plain text (hadText=false) at the base layer — both yield "". CodexProvider.Execute
// and CodexProvider.ExecuteConversation therefore re-run extractCodexAssistantText to recover
// hadText and apply the presence-aware overwrite that corrects Output, the token counts, and the
// trailing turn Content for the empty-message case. This intentional re-walk of the NDJSON is the
// accepted cost of keeping the base hook signature uniform across all providers.
func (p *CodexProvider) extractCodexTextContent(output string) string {
	text, _ := p.extractCodexAssistantText(output)
	return text
}

// extractCodexAssistantText aggregates assistant text from NDJSON output via
// p.parseCodexDisplayEvents, mirroring the join semantics of extractDisplayTextFromEvents.
// hadText is true iff at least one EventText was produced (even when its .Text is empty),
// enabling callers to distinguish "empty assistant_message" from "non-NDJSON plain text".
func (p *CodexProvider) extractCodexAssistantText(output string) (string, bool) {
	var result strings.Builder
	hadText := false
	for line := range strings.SplitSeq(output, "\n") {
		if line == "" {
			continue
		}
		for _, evt := range p.parseCodexDisplayEvents([]byte(line)) {
			if evt.Kind != EventText {
				continue
			}
			hadText = true
			if result.Len() > 0 {
				result.WriteRune('\n')
			}
			result.WriteString(evt.Text)
		}
	}
	return result.String(), hadText
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
// DisplayEvents. It emits EventText for assistant-message items and EventToolUse
// for function-call items. All other event types return nil (skip signal).
//
// Codex CLI versions disagree on the item discriminator and its values:
//   - legacy schema:        item.item_type == "assistant_message" / "function_call"
//   - codex-cli >= 0.133.0: item.type      == "agent_message"     / "command_execution"
//
// We read BOTH discriminator keys (item_type wins when present) and accept both naming
// families, so the same parser handles old fixtures and the current CLI. When the schema
// is not recognized, extraction yields no text and the base provider falls back to the raw
// NDJSON — which is exactly the leak this dual-schema handling prevents.
func (p *CodexProvider) parseCodexDisplayEvents(line []byte) []DisplayEvent {
	// Replace NUL bytes (0x00) with the 6-byte JSON unicode escape sequence
	// {0x5c,0x75,0x30,0x30,0x30,0x30} = backslash + u + 0 + 0 + 0 + 0.
	// json.Unmarshal decodes this escape back to NUL, preserving string content.
	// This diverges intentionally from the Claude provider, which replaces NUL with a space
	// (discarding the byte): Codex tool output may embed NUL as a meaningful sentinel, so we
	// preserve it rather than mangle it. Do not align the two without re-checking that contract.
	sanitized := bytes.ReplaceAll(line, []byte{0x00}, []byte{0x5c, 0x75, 0x30, 0x30, 0x30, 0x30})
	var evt struct {
		Type string `json:"type"`
		Item *struct {
			ItemType  string `json:"item_type"` // legacy schema discriminator
			Kind      string `json:"type"`      // codex-cli >= 0.133.0 discriminator
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
		// item_type (legacy) takes precedence; fall back to type (current CLI).
		itemKind := evt.Item.ItemType
		if itemKind == "" {
			itemKind = evt.Item.Kind
		}
		switch itemKind {
		case "assistant_message", "agent_message":
			return []DisplayEvent{{Type: evt.Type, Kind: EventText, Text: evt.Item.Text}}
		case "function_call", "command_execution":
			// Codex does not emit tool-call IDs; ID is always empty.
			preview := extractArgPreview(evt.Item.Arguments)
			return []DisplayEvent{{Type: evt.Type, Kind: EventToolUse, Name: evt.Item.Name, Arg: preview, ID: ""}}
		}
	}
	return nil
}
