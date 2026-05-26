package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/logger"
)

// ClaudeProvider implements AgentProvider for Claude CLI.
// Invokes: claude -p "prompt" --output-format stream-json
type ClaudeProvider struct {
	base      *baseCLIProvider
	logger    ports.Logger
	executor  ports.CLIExecutor
	tokenizer ports.Tokenizer
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
	b := newBaseCLIProvider("claude", "claude", p.executor, p.logger, cliProviderHooks{
		buildExecuteArgs:      p.buildExecuteArgs,
		buildConversationArgs: p.buildConversationArgs,
		extractSessionID:      p.extractSessionID,
		extractTextContent:    p.extractTextFromJSON,
		validateOptions:       validateClaudeOptions,
		parseDisplayEvents:    p.parseClaudeDisplayEvents,
		extractTokenUsage:     p.extractClaudeTokenUsage,
		mcpInjector:           claudeMCPInjector,
	})
	if p.tokenizer != nil {
		b.tokenizer = p.tokenizer
	}
	return b
}

func (p *ClaudeProvider) Execute(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
	result, rawOutput, err := p.base.execute(ctx, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}

	userFormat, _ := getStringOption(options, "output_format")

	// CLI is forced to stream-json, so stdout interleaves lifecycle events
	// (system, assistant, hook_*) ahead of the result event. Extract the
	// response unconditionally — leaving NDJSON in state.Output breaks any
	// downstream JSON post-processing.
	if extracted := p.extractTextFromJSON(rawOutput); extracted != "" {
		result.Output = extracted
		if result.TokensEstimated {
			tokens, _ := p.base.tokenizer.CountTokens(extracted) //nolint:errcheck // ApproximationTokenizer never errors with a valid ratio
			result.Tokens = tokens
		}
	}

	if userFormat == "json" || userFormat == "stream-json" {
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

	// Always force stream-json NDJSON at the CLI level so the F082 display filter
	// and text extraction have a consistent wire format. The user-facing
	// output_format (text vs json) is resolved in the application layer and the
	// display filter — not by toggling the Claude CLI's --output-format flag.
	// stream-json requires --verbose in -p mode for live streaming.
	args = append(args, "--output-format", "stream-json", "--verbose")

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

// claudeMCPInjector appends Claude-specific MCP flags to args.
//
// Claude CLI's --mcp-config flag expects a file in the standard
// claude_desktop_config.json shape (a top-level "mcpServers" record mapping
// server names to {command, args}). AWF's internal proxy config — read by
// `awf mcp-serve` — has a different shape and is not what Claude wants.
//
// This injector therefore writes a small wrapper config file that maps the
// server name "awf-proxy" to the spawn command `awf mcp-serve --config=<internal>`,
// and passes the WRAPPER path (not the internal path) to --mcp-config. The
// returned cleanup removes the wrapper file after Execute returns.
//
// intercept_builtins=true: --mcp-config <wrapper> --tools "" --strict-mcp-config
// intercept_builtins=false: --mcp-config <wrapper> only
// Returns a new slice and the input options unchanged (Claude does not mutate system_prompt).
func claudeMCPInjector(_ context.Context, args []string, cfg *workflow.MCPProxyConfig, mcpConfigPath string, options map[string]any) (newArgs []string, newOptions map[string]any, cleanup func() error, err error) {
	if cfg == nil {
		return args, options, noopMCPCleanup, nil
	}

	wrapperPath, wrapperCleanup, werr := writeClaudeMCPWrapper(mcpConfigPath)
	if werr != nil {
		return nil, options, noopMCPCleanup, werr
	}

	newArgs = make([]string, len(args), len(args)+4)
	copy(newArgs, args)
	newArgs = append(newArgs, "--mcp-config", wrapperPath)
	if cfg.InterceptBuiltins {
		newArgs = append(newArgs, "--tools", "", "--strict-mcp-config")
	}
	return newArgs, options, wrapperCleanup, nil
}

// claudeMCPWrapperServer is one entry under "mcpServers" in the Claude wrapper config.
type claudeMCPWrapperServer struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// claudeMCPWrapperConfig is the shape Claude CLI expects for --mcp-config.
type claudeMCPWrapperConfig struct {
	MCPServers map[string]claudeMCPWrapperServer `json:"mcpServers"`
}

// writeClaudeMCPWrapper writes a Claude-compatible MCP config that maps the
// "awf-proxy" server name to "<awf-bin> mcp-serve --config=<internalConfigPath>",
// returns the wrapper file path and an idempotent cleanup that removes the file.
// The internal config path itself is owned by ProxyService and removed by its own
// cleanup; this function manages ONLY the wrapper file.
func writeClaudeMCPWrapper(internalConfigPath string) (path string, cleanup func() error, err error) {
	cmd := mcpServeCommand(internalConfigPath)
	if len(cmd) == 0 {
		return "", noopMCPCleanup, fmt.Errorf("claude mcp wrapper: empty mcp-serve command")
	}

	wrapper := claudeMCPWrapperConfig{
		MCPServers: map[string]claudeMCPWrapperServer{
			"awf-proxy": {Command: cmd[0], Args: cmd[1:]},
		},
	}
	data, err := json.Marshal(wrapper)
	if err != nil {
		return "", noopMCPCleanup, fmt.Errorf("marshal claude mcp wrapper: %w", err)
	}

	f, createErr := os.CreateTemp("", "awf-claude-mcp-*.json")
	if createErr != nil {
		return "", noopMCPCleanup, fmt.Errorf("create claude mcp wrapper: %w", createErr)
	}
	tmpPath := f.Name()
	if _, writeErr := f.Write(data); writeErr != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return "", noopMCPCleanup, fmt.Errorf("write claude mcp wrapper: %w", writeErr)
	}
	if closeErr := f.Close(); closeErr != nil {
		_ = os.Remove(tmpPath)
		return "", noopMCPCleanup, fmt.Errorf("close claude mcp wrapper: %w", closeErr)
	}

	var once sync.Once
	cleanup = func() error {
		var rerr error
		once.Do(func() {
			if removeErr := os.Remove(tmpPath); removeErr != nil && !os.IsNotExist(removeErr) {
				rerr = removeErr
			}
		})
		return rerr
	}
	return tmpPath, cleanup, nil
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
	for line := range strings.SplitSeq(output, "\n") {
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

func (p *ClaudeProvider) extractClaudeTokenUsage(rawOutput string) *tokenUsage {
	evt := p.extractResultEvent(rawOutput)
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
	input := intFromMap(usage, "input_tokens") +
		intFromMap(usage, "cache_creation_input_tokens") +
		intFromMap(usage, "cache_read_input_tokens")
	output := intFromMap(usage, "output_tokens")
	var costUSD float64
	if v, ok := evt["total_cost_usd"].(float64); ok {
		costUSD = v
	}
	return &tokenUsage{
		InputTokens:  input,
		OutputTokens: output,
		TotalTokens:  input + output,
		CostUSD:      costUSD,
	}
}

func (p *ClaudeProvider) parseClaudeDisplayEvents(line []byte) []DisplayEvent {
	// Replace NUL bytes with a space to avoid JSON parse errors on malformed input.
	line = bytes.ReplaceAll(line, []byte{0x00}, []byte(" "))

	var evt struct {
		Type    string `json:"type"`
		Message *struct {
			Content []struct {
				Type  string         `json:"type"`
				Text  string         `json:"text"`
				ID    string         `json:"id"`
				Name  string         `json:"name"`
				Input map[string]any `json:"input"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &evt); err != nil {
		return nil
	}
	if evt.Type != "assistant" || evt.Message == nil {
		return nil
	}

	var events []DisplayEvent
	for _, block := range evt.Message.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				events = append(events, DisplayEvent{
					Type: "assistant",
					Kind: EventText,
					Text: block.Text,
				})
			}
		case "tool_use":
			events = append(events, DisplayEvent{
				Type: "assistant",
				Kind: EventToolUse,
				Name: block.Name,
				Arg:  extractArgPreviewFromMap(block.Input),
				ID:   block.ID,
			})
		}
	}
	return events
}
