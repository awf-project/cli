package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/logger"
)

// CopilotProvider implements AgentProvider for GitHub Copilot CLI.
// Invokes: copilot -p "prompt" --output-format=json --silent
type CopilotProvider struct {
	base      *baseCLIProvider
	logger    ports.Logger
	executor  ports.CLIExecutor
	tokenizer ports.Tokenizer
}

func NewCopilotProvider() *CopilotProvider {
	p := &CopilotProvider{
		logger:   logger.NopLogger{},
		executor: NewExecCLIExecutor(),
	}
	p.base = p.newBase()
	return p
}

func NewCopilotProviderWithOptions(opts ...CopilotProviderOption) *CopilotProvider {
	p := &CopilotProvider{
		logger:   logger.NopLogger{},
		executor: NewExecCLIExecutor(),
	}
	for _, opt := range opts {
		opt(p)
	}
	p.base = p.newBase()
	return p
}

func (p *CopilotProvider) newBase() *baseCLIProvider {
	b := newBaseCLIProvider("github_copilot", "copilot", p.executor, p.logger, cliProviderHooks{
		buildExecuteArgs:      p.buildCopilotExecuteArgs,
		buildConversationArgs: p.buildCopilotConversationArgs,
		extractSessionID:      p.extractCopilotSessionID,
		extractTextContent:    p.extractCopilotTextContent,
		validateOptions:       validateCopilotOptions,
		parseDisplayEvents:    p.parseCopilotDisplayEvents,
		extractTokenUsage:     p.extractCopilotTokenUsage,
		mcpInjector:           p.copilotMCPInjector,
	})
	if p.tokenizer != nil {
		b.tokenizer = p.tokenizer
	}
	return b
}

func (p *CopilotProvider) Execute(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
	result, rawOutput, err := p.base.execute(ctx, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}
	if extracted := p.extractCopilotTextContent(rawOutput); extracted != "" {
		result.Output = extracted
		if result.TokensEstimated {
			tokens, _ := p.base.tokenizer.CountTokens(extracted) //nolint:errcheck // ApproximationTokenizer never errors with a valid ratio
			result.Tokens = tokens
		}
	}
	return result, nil
}

func (p *CopilotProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
	result, _, err := p.base.executeConversation(ctx, state, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (p *CopilotProvider) Name() string {
	return "github_copilot"
}

func (p *CopilotProvider) Validate() error {
	_, err := exec.LookPath("copilot")
	if err != nil {
		return fmt.Errorf("copilot CLI not found in PATH: %w", err)
	}
	return nil
}

func (p *CopilotProvider) buildCopilotExecuteArgs(prompt string, options map[string]any) ([]string, error) {
	args := []string{"-p", prompt, "--output-format=json", "--silent"}
	args = appendCopilotOptions(args, options)
	return args, nil
}

func (p *CopilotProvider) buildCopilotConversationArgs(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
	var args []string
	if state.SessionID != "" {
		args = []string{"--resume=" + state.SessionID, "-p", prompt, "--output-format=json", "--silent"}
	} else {
		effectivePrompt := buildFirstTurnPrompt(prompt, options)
		args = []string{"-p", effectivePrompt, "--output-format=json", "--silent"}
	}
	args = appendCopilotOptions(args, options)
	return args, nil
}

// appendCopilotOptions appends Copilot CLI flags from options; unknown keys are silently ignored.
func appendCopilotOptions(args []string, options map[string]any) []string {
	if model, ok := getStringOption(options, "model"); ok && model != "" {
		args = append(args, "--model="+model)
	}
	if mode, ok := getStringOption(options, "mode"); ok && mode != "" {
		args = append(args, "--mode="+mode)
	}
	if effort, ok := getStringOption(options, "effort"); ok && effort != "" {
		args = append(args, "--effort="+effort)
	}
	if tools, ok := options["allowed_tools"]; ok {
		for _, t := range toStringSlice(tools) {
			args = append(args, "--allow-tool="+t)
		}
	}
	if tools, ok := options["denied_tools"]; ok {
		for _, t := range toStringSlice(tools) {
			args = append(args, "--deny-tool="+t)
		}
	}
	if allow, ok := getBoolOption(options, "allow_all"); ok && allow {
		args = append(args, "--allow-all")
	} else if skip, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skip {
		// Alias for cross-provider compatibility: dangerously_skip_permissions maps to --allow-all
		args = append(args, "--allow-all")
	}
	return args
}

func toStringSlice(v any) []string {
	switch typed := v.(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func validateCopilotOptions(options map[string]any) error {
	var errs []string

	if mode, ok := getStringOption(options, "mode"); ok && mode != "" {
		switch mode {
		case "interactive", "plan", "autopilot":
		default:
			errs = append(errs, fmt.Sprintf("invalid mode %q: must be one of interactive, plan, autopilot", mode))
		}
	}

	if effort, ok := getStringOption(options, "effort"); ok && effort != "" {
		switch effort {
		case "low", "medium", "high":
		default:
			errs = append(errs, fmt.Sprintf("invalid effort %q: must be one of low, medium, high", effort))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func (p *CopilotProvider) extractCopilotSessionID(output string) (string, error) {
	if output == "" {
		return "", errors.New("empty output")
	}
	evt := findFirstNDJSONEvent(output, "result")
	if evt == nil {
		return "", errors.New("result event not found")
	}
	// Copilot CLI uses camelCase "sessionId" (not snake_case)
	sessionIDVal, ok := evt["sessionId"]
	if !ok || sessionIDVal == nil {
		return "", errors.New("sessionId missing")
	}
	if str, ok := sessionIDVal.(string); ok && str != "" {
		return str, nil
	}
	return "", errors.New("sessionId is not a non-empty string")
}

func (p *CopilotProvider) extractCopilotTokenUsage(rawOutput string) *tokenUsage {
	evt := findLastNDJSONEvent(rawOutput, "assistant.message")
	if evt == nil {
		return nil
	}
	data, ok := evt["data"].(map[string]any)
	if !ok {
		return nil
	}
	outputTokens := intFromMap(data, "outputTokens")
	if outputTokens == 0 {
		return nil
	}
	return &tokenUsage{
		OutputTokens: outputTokens,
		TotalTokens:  outputTokens,
	}
}

func (p *CopilotProvider) parseCopilotDisplayEvents(line []byte) []DisplayEvent {
	var evt struct {
		Type string         `json:"type"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(line, &evt); err != nil {
		return nil
	}
	switch evt.Type {
	case "assistant.message":
		if content, ok := evt.Data["content"].(string); ok && content != "" {
			return []DisplayEvent{{Type: evt.Type, Kind: EventText, Text: content}}
		}
	case "tool.execution_start":
		name, ok := evt.Data["toolName"].(string)
		if !ok {
			name = ""
		}
		return []DisplayEvent{{Type: evt.Type, Kind: EventToolUse, Name: name}}
	}
	return nil
}

// copilotMCPInjector appends Copilot-specific MCP flags to args.
//
// Copilot CLI's --additional-mcp-config flag accepts a JSON string or a file
// path (prefixed with `@`). It expects the standard `{"mcpServers": {...}}`
// shape; AWF's internal proxy config has a different shape, so this injector
// writes a small wrapper file mapping the server name "awf-proxy" to the spawn
// command `awf mcp-serve --config=<internal>`, and passes the WRAPPER path
// (prefixed with `@`) to --additional-mcp-config. The returned cleanup removes
// the wrapper file after Execute returns.
//
// Copilot has no equivalent to Claude's `--tools ""` flag, so full native-tool
// blocking is impossible. This injector therefore runs in COEXISTENCE mode
// like Codex/OpenCode:
//   - intercept_builtins=true: --additional-mcp-config @<wrapper> +
//     --disable-builtin-mcps (best-effort: blocks Copilot's bundled
//     github-mcp-server, but the native shell/edit/read tools remain
//     accessible), emits a WARN log, and prepends an MCP-only directive to
//     system_prompt as a mitigation guidance to the model.
//   - intercept_builtins=false: --additional-mcp-config @<wrapper> only.
func (p *CopilotProvider) copilotMCPInjector(_ context.Context, args []string, cfg *workflow.MCPProxyConfig, mcpConfigPath string, options map[string]any) (newArgs []string, newOptions map[string]any, cleanup func() error, err error) {
	if cfg == nil {
		return args, options, noopMCPCleanup, nil
	}

	wrapperPath, wrapperCleanup, werr := writeCopilotMCPWrapper(mcpConfigPath)
	if werr != nil {
		return nil, options, noopMCPCleanup, werr
	}

	newArgs = make([]string, len(args), len(args)+3)
	copy(newArgs, args)
	// The `@` prefix tells Copilot to read the MCP config from the given file path.
	newArgs = append(newArgs, "--additional-mcp-config", "@"+wrapperPath)

	// Clone options so we don't mutate the caller's map.
	newOpts := make(map[string]any, len(options)+1)
	maps.Copy(newOpts, options)

	if cfg.InterceptBuiltins {
		// Best-effort: disable Copilot's bundled github-mcp-server so the only
		// MCP surface is awf-proxy. This does NOT block native shell/edit tools.
		newArgs = append(newArgs, "--disable-builtin-mcps")

		p.logger.Warn("mcp_proxy on provider=copilot runs in coexistence mode; built-in tools are not blocked")

		// Prepend MCP-only instruction to system_prompt (coexistence mitigation).
		// Guides the model to prefer MCP tools when intercept_builtins=true but
		// native tool blocking is unavailable.
		const mcpOnlyPrefix = "Use only MCP tools, never built-in tools. "
		existing, _ := getStringOption(newOpts, "system_prompt")
		newOpts["system_prompt"] = mcpOnlyPrefix + existing
	}

	return newArgs, newOpts, wrapperCleanup, nil
}

// copilotMCPWrapperServer is one entry under "mcpServers" in the Copilot wrapper config.
type copilotMCPWrapperServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// copilotMCPWrapperConfig is the shape Copilot CLI expects for --additional-mcp-config.
type copilotMCPWrapperConfig struct {
	MCPServers map[string]copilotMCPWrapperServer `json:"mcpServers"`
}

// writeCopilotMCPWrapper writes a Copilot-compatible MCP config that maps the
// "awf-proxy" server name to "<awf-bin> mcp-serve --config=<internalConfigPath>",
// returns the wrapper file path and an idempotent cleanup that removes the file.
// The internal config path itself is owned by ProxyService and removed by its own
// cleanup; this function manages ONLY the wrapper file.
func writeCopilotMCPWrapper(internalConfigPath string) (path string, cleanup func() error, err error) {
	cmd := mcpServeCommand(internalConfigPath)
	if len(cmd) == 0 {
		return "", noopMCPCleanup, fmt.Errorf("copilot mcp wrapper: empty mcp-serve command")
	}

	wrapper := copilotMCPWrapperConfig{
		MCPServers: map[string]copilotMCPWrapperServer{
			"awf-proxy": {Type: "local", Command: cmd[0], Args: cmd[1:]},
		},
	}
	data, err := json.Marshal(wrapper)
	if err != nil {
		return "", noopMCPCleanup, fmt.Errorf("marshal copilot mcp wrapper: %w", err)
	}

	f, createErr := os.CreateTemp("", "awf-copilot-mcp-*.json")
	if createErr != nil {
		return "", noopMCPCleanup, fmt.Errorf("create copilot mcp wrapper: %w", createErr)
	}
	tmpPath := f.Name()
	if _, writeErr := f.Write(data); writeErr != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return "", noopMCPCleanup, fmt.Errorf("write copilot mcp wrapper: %w", writeErr)
	}
	if closeErr := f.Close(); closeErr != nil {
		_ = os.Remove(tmpPath)
		return "", noopMCPCleanup, fmt.Errorf("close copilot mcp wrapper: %w", closeErr)
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

// extractCopilotTextContent scans JSONL output for the last assistant.message event
// and returns its data.content field. Falls back to raw output when not found.
func (p *CopilotProvider) extractCopilotTextContent(output string) string {
	var lastContent string
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var evt map[string]any
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}
		if t, ok := evt["type"].(string); ok && t == "assistant.message" {
			if data, ok := evt["data"].(map[string]any); ok {
				if content, ok := data["content"].(string); ok && content != "" {
					lastContent = content
				}
			}
		}
	}
	if lastContent != "" {
		return lastContent
	}
	return output
}
