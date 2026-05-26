package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/logger"
	"github.com/awf-project/cli/pkg/interpolation"
)

// GeminiProvider implements AgentProvider for Gemini CLI.
// Invokes: gemini -p "prompt"
type GeminiProvider struct {
	base              *baseCLIProvider
	logger            ports.Logger
	executor          ports.CLIExecutor
	cmdExecutor       ports.CommandExecutor
	tokenizer         ports.Tokenizer
	denyAllPolicyPath string
}

func NewGeminiProvider() *GeminiProvider {
	p := &GeminiProvider{
		logger:   logger.NopLogger{},
		executor: NewExecCLIExecutor(),
	}
	p.base = p.newBase()
	return p
}

func NewGeminiProviderWithOptions(opts ...GeminiProviderOption) *GeminiProvider {
	p := &GeminiProvider{
		logger:   logger.NopLogger{},
		executor: NewExecCLIExecutor(),
	}
	for _, opt := range opts {
		opt(p)
	}
	p.base = p.newBase()
	return p
}

func (p *GeminiProvider) newBase() *baseCLIProvider {
	b := newBaseCLIProvider("gemini", "gemini", p.executor, p.logger, cliProviderHooks{
		buildExecuteArgs:      p.buildExecuteArgs,
		buildConversationArgs: p.buildConversationArgs,
		extractSessionID:      p.extractSessionID,
		validateOptions:       validateGeminiOptions,
		parseDisplayEvents:    p.parseGeminiDisplayEvents,
		extractTokenUsage:     p.extractGeminiTokenUsage,
		mcpInjector:           p.geminiMCPInjector,
	})
	if p.tokenizer != nil {
		b.tokenizer = p.tokenizer
	}
	return b
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

	// CLI is forced to stream-json, so stdout interleaves init/message/result
	// events. Aggregate assistant content unconditionally — leaving NDJSON in
	// state.Output breaks any downstream JSON post-processing.
	if extracted := extractDisplayTextFromEvents(rawOutput, p.parseGeminiDisplayEvents); extracted != "" {
		result.Output = extracted
		if result.TokensEstimated {
			tokens, _ := p.base.tokenizer.CountTokens(extracted) //nolint:errcheck // ApproximationTokenizer never errors with a valid ratio
			result.Tokens = tokens
		}
	}

	userFormat, _ := getStringOption(options, "output_format")
	if userFormat == "json" || userFormat == "stream-json" {
		if jsonResp := tryParseJSONResponse(rawOutput); jsonResp != nil {
			result.Response = jsonResp
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

func prependGeminiGlobalFlags(args []string, options map[string]any) []string {
	if model, ok := getStringOption(options, "model"); ok {
		args = append([]string{"--model", model}, args...)
	}
	args = append([]string{"--output-format", "stream-json"}, args...)
	if skipPerms, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skipPerms {
		args = append([]string{"--approval-mode=yolo"}, args...)
	}
	return args
}

func (p *GeminiProvider) buildExecuteArgs(prompt string, options map[string]any) ([]string, error) {
	args := []string{"-p", prompt}

	// Always force stream-json NDJSON at the CLI level so the F082 display filter
	// and text extraction have a consistent wire format (F082, aligned with Claude).
	args = prependGeminiGlobalFlags(args, options)

	return args, nil
}

func (p *GeminiProvider) buildConversationArgs(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
	// Gemini CLI has no --system-prompt flag; inline the system prompt into
	// the first turn's message. Subsequent turns rely on --resume.
	var args []string
	if state != nil && state.SessionID != "" {
		args = []string{"--resume", state.SessionID, "-p", prompt}
	} else {
		args = []string{"-p", buildFirstTurnPrompt(prompt, options)}
	}

	// Force stream-json unconditionally for reliable session ID extraction.
	args = prependGeminiGlobalFlags(args, options)

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

func (p *GeminiProvider) extractGeminiTokenUsage(rawOutput string) *tokenUsage {
	evt := findFirstNDJSONEvent(rawOutput, "result")
	if evt == nil {
		return nil
	}
	stats, ok := evt["stats"].(map[string]any)
	if !ok {
		return nil
	}
	return &tokenUsage{
		InputTokens:  intFromMap(stats, "input_tokens"),
		OutputTokens: intFromMap(stats, "output_tokens"),
		TotalTokens:  intFromMap(stats, "total_tokens"),
	}
}

func (p *GeminiProvider) geminiMCPInjector(ctx context.Context, args []string, cfg *workflow.MCPProxyConfig, mcpConfigPath string, options map[string]any) (newArgs []string, newOptions map[string]any, cleanup func() error, err error) {
	if cfg == nil {
		return args, options, noopMCPCleanup, nil
	}

	if p.cmdExecutor == nil {
		return nil, options, noopMCPCleanup, fmt.Errorf("gemini mcp add: command executor not configured")
	}

	// Generate a unique registration name to prevent collisions when multiple AWF
	// processes run concurrently. Each invocation of this injector owns exactly
	// one registration keyed by this name; the cleanup closure captures name so
	// it removes only its own registration, never another run's.
	name := mcpProxyNamePrefix + randShortID(8)

	// Gemini MCP registration uses the `gemini mcp add <name> <cmd> [args...]`
	// subcommand (positional args, no -- separator unlike OpenCode).
	serveCmd := mcpServeCommand(mcpConfigPath)
	// interpolation.ShellEscape each argument to prevent shell injection from name or
	// any component of serveCmd (executable path, config path with special chars).
	quotedServeCmd := make([]string, len(serveCmd))
	for i, a := range serveCmd {
		quotedServeCmd[i] = interpolation.ShellEscape(a)
	}
	addProgram := "gemini mcp add " + interpolation.ShellEscape(name) + " " + strings.Join(quotedServeCmd, " ")

	// Derive timeout from parent ctx so a cancelled workflow propagates cancellation.
	// context.Background() is intentionally used in the cleanup closure (mcp remove)
	// so teardown runs even when the parent context has already been cancelled.
	addCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if _, err := p.cmdExecutor.Execute(addCtx, &ports.Command{Program: addProgram}); err != nil {
		return nil, options, noopMCPCleanup, fmt.Errorf("gemini mcp add: %w", err)
	}

	newArgs = make([]string, len(args), len(args)+4)
	copy(newArgs, args)

	// Full isolation: Gemini supports --allowed-mcp-server-names to whitelist servers
	// and --policy to deny all built-in tools. No system_prompt mutation needed.
	if cfg.InterceptBuiltins {
		newArgs = append(newArgs, "--allowed-mcp-server-names", name)
		if p.denyAllPolicyPath != "" {
			newArgs = append(newArgs, "--policy", p.denyAllPolicyPath)
		}
	}

	cmdExec := p.cmdExecutor
	var once sync.Once
	var removeErr error
	removeCleanup := func() error {
		once.Do(func() {
			// interpolation.ShellEscape: same injection defense as the add command above (F099-S1).
			_, removeErr = cmdExec.Execute(context.Background(), &ports.Command{
				Program: "gemini mcp remove " + interpolation.ShellEscape(name),
			})
		})
		return removeErr
	}

	// Gemini does not mutate system_prompt; return options unchanged.
	return newArgs, options, removeCleanup, nil
}

func (p *GeminiProvider) parseGeminiDisplayEvents(line []byte) []DisplayEvent {
	var evt struct {
		Type      string `json:"type"`
		Role      string `json:"role"`
		Content   string `json:"content"`
		ToolCalls []struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		} `json:"toolCalls"`
	}
	if err := json.Unmarshal(line, &evt); err != nil {
		return nil
	}
	if evt.Type != "message" || evt.Role != "assistant" {
		return nil
	}

	var out []DisplayEvent
	if evt.Content != "" {
		out = append(out, DisplayEvent{Type: evt.Role, Kind: EventText, Text: evt.Content})
	}
	for _, call := range evt.ToolCalls {
		out = append(out, DisplayEvent{
			Kind: EventToolUse,
			Name: call.Name,
			Arg:  extractArgPreviewFromMap(call.Arguments),
			ID:   "",
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
