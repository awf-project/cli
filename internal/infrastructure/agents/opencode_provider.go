package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/logger"
)

// OpenCodeProvider implements AgentProvider for OpenCode CLI.
// Invokes: opencode run "prompt"
type OpenCodeProvider struct {
	base      *baseCLIProvider
	logger    ports.Logger
	executor  ports.CLIExecutor
	tokenizer ports.Tokenizer
}

func NewOpenCodeProvider() *OpenCodeProvider {
	p := &OpenCodeProvider{
		logger:   logger.NopLogger{},
		executor: NewExecCLIExecutor(),
	}
	p.base = p.newBase()
	return p
}

func NewOpenCodeProviderWithOptions(opts ...OpenCodeProviderOption) *OpenCodeProvider {
	p := &OpenCodeProvider{
		logger:   logger.NopLogger{},
		executor: NewExecCLIExecutor(),
	}
	for _, opt := range opts {
		opt(p)
	}
	p.base = p.newBase()
	return p
}

func (p *OpenCodeProvider) newBase() *baseCLIProvider {
	b := newBaseCLIProvider("opencode", "opencode", p.executor, p.logger, cliProviderHooks{
		buildExecuteArgs:      p.buildExecuteArgs,
		buildConversationArgs: p.buildConversationArgs,
		extractSessionID:      p.extractSessionID,
		validateOptions:       validateOpenCodeOptions,
		parseDisplayEvents:    p.parseOpencodeDisplayEvents,
		extractTokenUsage:     p.extractOpenCodeTokenUsage,
		mcpInjector:           p.opencodeMCPInjector,
	})
	if p.tokenizer != nil {
		b.tokenizer = p.tokenizer
	}
	return b
}

// Execute invokes the OpenCode CLI with the given prompt and options.
func (p *OpenCodeProvider) Execute(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
	result, rawOutput, err := p.base.execute(ctx, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}

	// CLI is forced to --format json (NDJSON), so stdout interleaves
	// step_start/text/step_finish events. Aggregate text parts unconditionally —
	// leaving NDJSON in state.Output breaks any downstream JSON post-processing.
	if extracted := extractDisplayTextFromEvents(rawOutput, p.parseOpencodeDisplayEvents); extracted != "" {
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

// ExecuteConversation invokes the OpenCode CLI with conversation history for multi-turn interactions.
func (p *OpenCodeProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
	result, _, err := p.base.executeConversation(ctx, state, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// buildExecuteArgs constructs CLI arguments for a single-turn Execute call.
// opencode CLI syntax: opencode run "prompt" --format <json|default> [--model X] [--framework F] [--verbose] [--output DIR]
func (p *OpenCodeProvider) buildExecuteArgs(prompt string, options map[string]any) ([]string, error) {
	args := []string{"run", prompt}
	// Always request NDJSON: session ID extraction, display filter, and raw
	// display (output_format: json) all consume stream-json events. For
	// output_format: text, the F082 display filter extracts assistant text
	// from the "text" events (consistent with Claude's stream-json approach).
	args = append(args, "--format", "json")

	if model, ok := getStringOption(options, "model"); ok {
		args = append(args, "--model", model)
	}

	if skipPerms, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skipPerms {
		args = append(args, "--dangerously-skip-permissions")
	}

	args = applyOpenCodeCLIOptions(args, options)
	return args, nil
}

// buildConversationArgs constructs CLI arguments for a multi-turn ExecuteConversation call.
// Applies session resume (-s sessionID) or continuation fallback (-c) when prior turns exist,
// and inlines system_prompt into the first turn's message (opencode CLI has no --system-prompt flag).
func (p *OpenCodeProvider) buildConversationArgs(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
	effectivePrompt := prompt
	if len(state.Turns) == 0 {
		effectivePrompt = buildFirstTurnPrompt(prompt, options)
	}

	args := []string{"run", effectivePrompt}
	// Always request NDJSON: session ID extraction, display filter, and raw
	// display (output_format: json) all consume stream-json events. For
	// output_format: text, the F082 display filter extracts assistant text
	// from the "text" events (consistent with Claude's stream-json approach).
	args = append(args, "--format", "json")

	if model, ok := getStringOption(options, "model"); ok {
		args = append(args, "--model", model)
	}

	if skipPerms, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skipPerms {
		args = append(args, "--dangerously-skip-permissions")
	}

	switch {
	case state.SessionID != "":
		args = append(args, "-s", state.SessionID)
	case len(state.Turns) > 0:
		// Prior turns exist but session ID was lost (extraction failed); use -c to continue last session.
		args = append(args, "-c")
	}

	args = applyOpenCodeCLIOptions(args, options)
	return args, nil
}

// applyOpenCodeCLIOptions appends shared OpenCode flag mappings (framework,
// verbose, output_dir) to an existing argv slice.
func applyOpenCodeCLIOptions(args []string, options map[string]any) []string {
	if options == nil {
		return args
	}
	if framework, ok := options["framework"].(string); ok {
		args = append(args, "--framework", framework)
	}
	if verbose, ok := options["verbose"].(bool); ok && verbose {
		args = append(args, "--verbose")
	}
	if outputDir, ok := options["output_dir"].(string); ok {
		args = append(args, "--output", outputDir)
	}
	return args
}

// Name returns the provider identifier.
func (p *OpenCodeProvider) Name() string {
	return "opencode"
}

// Validate checks if the OpenCode CLI is installed and accessible.
func (p *OpenCodeProvider) Validate() error {
	_, err := exec.LookPath("opencode")
	if err != nil {
		return fmt.Errorf("opencode CLI not found in PATH: %w", err)
	}
	return nil
}

// validateOpenCodeOptions validates provider-specific options.
func validateOpenCodeOptions(options map[string]any) error {
	if options == nil {
		return nil
	}

	if outputDir, exists := options["output_dir"]; exists {
		if _, ok := outputDir.(string); !ok {
			return errors.New("output_dir must be a string")
		}
	}

	if verbose, exists := options["verbose"]; exists {
		if _, ok := verbose.(bool); !ok {
			return errors.New("verbose must be a boolean")
		}
	}

	return nil
}

func (p *OpenCodeProvider) extractStepStartEvent(output string) (map[string]any, error) {
	evt := findFirstNDJSONEvent(output, "step_start")
	if evt == nil {
		return nil, errors.New("no step_start event found in output")
	}
	return evt, nil
}

func (p *OpenCodeProvider) extractSessionID(output string) (string, error) {
	event, err := p.extractStepStartEvent(output)
	if err != nil {
		return "", err
	}

	sessionID, ok := event["sessionID"].(string)
	if !ok || sessionID == "" {
		return "", errors.New("sessionID not found or not a string in step_start event")
	}

	return sessionID, nil
}

func (p *OpenCodeProvider) extractOpenCodeTokenUsage(rawOutput string) *tokenUsage {
	evt := findFirstNDJSONEvent(rawOutput, "step_finish")
	if evt == nil {
		return nil
	}
	part, ok := evt["part"].(map[string]any)
	if !ok {
		return nil
	}
	tokens, ok := part["tokens"].(map[string]any)
	if !ok {
		return nil
	}
	cost, _ := part["cost"].(float64) //nolint:errcheck // type assertion; zero-value fallback is intentional
	return &tokenUsage{
		InputTokens:  intFromMap(tokens, "input"),
		OutputTokens: intFromMap(tokens, "output"),
		TotalTokens:  intFromMap(tokens, "total"),
		CostUSD:      cost,
	}
}

func (p *OpenCodeProvider) opencodeMCPInjector(_ context.Context, args []string, cfg *workflow.MCPProxyConfig, mcpConfigPath string, options map[string]any) (newArgs []string, newOptions map[string]any, cleanup func() error, err error) {
	if cfg == nil {
		return args, options, noopMCPCleanup, nil
	}

	// Generate a unique registration name to prevent collisions when multiple AWF
	// processes run concurrently. Each invocation of this injector owns exactly
	// one registration keyed by this name; the cleanup closure captures name so
	// it removes only its own registration, never another run's.
	name := mcpProxyNamePrefix + randShortID(8)

	// opencode 1.15.3 `opencode mcp add` is a TUI-only command — not scriptable.
	// The only reliable per-invocation mechanism is writing to ./opencode.json in
	// the workspace directory (opencode checks workspace config at startup and
	// gives it precedence over user-global config). We write only to opencode.json
	// (not opencode.jsonc) to avoid clobbering hand-edited user files with comments.
	workspaceDir, err := os.Getwd()
	if err != nil {
		return nil, options, noopMCPCleanup, fmt.Errorf("opencode mcp config: get working directory: %w", err)
	}

	serveCmd := mcpServeCommand(mcpConfigPath)
	removeCleanup, err := addOpenCodeMCPServer(workspaceDir, name, serveCmd)
	if err != nil {
		return nil, options, noopMCPCleanup, fmt.Errorf("opencode mcp config: %w", err)
	}

	// Clone options so we don't mutate the caller's map.
	newOpts := make(map[string]any, len(options)+1)
	maps.Copy(newOpts, options)

	if cfg.InterceptBuiltins {
		p.logger.Warn("mcp_proxy on provider=opencode runs in coexistence mode; built-in tools are not blocked")

		// Prepend MCP-only instruction to system_prompt (coexistence mitigation — T011 AC).
		// This guides the model to prefer MCP tools when intercept_builtins=true.
		const mcpOnlyPrefix = "Use only MCP tools, never built-in tools. "
		existing, _ := getStringOption(newOpts, "system_prompt")
		newOpts["system_prompt"] = mcpOnlyPrefix + existing
	}

	// OpenCode receives MCP server configuration via the workspace opencode.json
	// written above. Do NOT append --mcp-config here: OpenCode's --mcp-config flag
	// expects its own native format, not the AWF internal proxy config.
	newArgs = make([]string, len(args))
	copy(newArgs, args)

	return newArgs, newOpts, removeCleanup, nil
}

func (p *OpenCodeProvider) parseOpencodeDisplayEvents(line []byte) []DisplayEvent {
	// Escape NUL bytes to JSON unicode sequence so json.Unmarshal preserves them
	// in decoded string fields while avoiding parse errors.
	sanitized := bytes.ReplaceAll(line, []byte{0x00}, []byte{0x5c, 0x75, 0x30, 0x30, 0x30, 0x30})

	var evt struct {
		Type string `json:"type"`
		Part *struct {
			Text  string         `json:"text"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
		} `json:"part"`
	}
	if err := json.Unmarshal(sanitized, &evt); err != nil {
		return nil
	}
	if evt.Type == "" {
		return nil
	}
	if evt.Type == "text" {
		text := ""
		if evt.Part != nil {
			text = evt.Part.Text
		}
		return []DisplayEvent{{Type: evt.Type, Kind: EventText, Text: text}}
	}
	if evt.Type == "tool_use" {
		name := ""
		arg := ""
		if evt.Part != nil {
			name = evt.Part.Name
			arg = extractArgPreviewFromMap(evt.Part.Input)
		}
		return []DisplayEvent{{Type: evt.Type, Kind: EventToolUse, Name: name, Arg: arg, ID: ""}}
	}
	return nil
}
