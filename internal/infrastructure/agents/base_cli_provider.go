package agents

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/logger"
	"github.com/awf-project/cli/pkg/display"
)

var (
	execPathOnce sync.Once
	execPath     string
)

const cliProviderEnvOptionKey = "__awf_cli_provider_env"

type cliEnvExecutor interface {
	RunWithEnv(ctx context.Context, name string, env map[string]string, stdoutW, stderrW io.Writer, args ...string) (stdout, stderr []byte, err error)
}

func resolvedExecutable() string {
	execPathOnce.Do(func() {
		exe, err := os.Executable()
		if err != nil {
			execPath = os.Args[0]
			return
		}
		resolved, err := filepath.EvalSymlinks(exe)
		if err != nil {
			execPath = exe
			return
		}
		execPath = resolved
	})
	return execPath
}

func mcpServeCommand(configPath string) []string {
	return []string{resolvedExecutable(), "mcp-serve", "--config=" + configPath}
}

func cliProviderEnv(options map[string]any) (map[string]string, error) {
	if options == nil {
		return nil, nil
	}
	raw, ok := options[cliProviderEnvOptionKey]
	if !ok || raw == nil {
		return nil, nil
	}
	env, ok := raw.(map[string]string)
	if !ok {
		return nil, fmt.Errorf("invalid provider environment option type %T", raw)
	}
	return env, nil
}

func runCLIWithProviderEnv(ctx context.Context, executor ports.CLIExecutor, binary string, env map[string]string, stdout, stderr io.Writer, args ...string) (stdoutBytes, stderrBytes []byte, err error) {
	if len(env) == 0 {
		return executor.Run(ctx, binary, stdout, stderr, args...)
	}
	envExec, ok := executor.(cliEnvExecutor)
	if !ok {
		return nil, nil, fmt.Errorf("CLI executor does not support provider environment overrides")
	}
	return envExec.RunWithEnv(ctx, binary, env, stdout, stderr, args...)
}

type fallbackTokenizer struct{}

func (fallbackTokenizer) CountTokens(text string) (int, error) { return len(text) / 4, nil }
func (fallbackTokenizer) CountTurnsTokens(turns []string) (int, error) {
	n := 0
	for _, t := range turns {
		n += len(t)
	}
	return n / 4, nil
}
func (fallbackTokenizer) IsEstimate() bool  { return true }
func (fallbackTokenizer) ModelName() string { return "fallback" }

type tokenUsage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	CostUSD      float64
}

// noopMCPCleanup is a no-op cleanup for providers that have no MCP side-effects.
func noopMCPCleanup() error { return nil }

// cliProviderHooks captures provider-specific behavior as function values.
// Optional hooks (extractTextContent, validateOptions, parseDisplayEvents, extractTokenUsage, mcpInjector) may be nil.
type cliProviderHooks struct {
	buildExecuteArgs      func(prompt string, options map[string]any) ([]string, error)
	buildConversationArgs func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error)
	extractSessionID      func(output string) (string, error)
	extractTextContent    func(output string) string
	validateOptions       func(options map[string]any) error
	parseDisplayEvents    DisplayEventParser
	extractTokenUsage     func(rawOutput string) *tokenUsage
	// mcpInjector appends provider-specific MCP flags to args and optionally mutates
	// options (e.g. prepending a system_prompt for Codex/OpenCode coexistence mode).
	// ctx is the parent context of the agent execution; injectors that spawn sub-processes
	// (e.g. gemini mcp add, opencode mcp add) should derive a timeout from ctx rather than
	// context.Background() so that a cancelled parent propagates cancellation correctly.
	// For cleanup closures that must run after parent cancellation (mcp remove), use
	// context.Background() inside the closure directly.
	// Returns:
	//   - newArgs:    the augmented args slice (never mutates the input slice)
	//   - newOptions: merged options map; callers replace their local options map with this
	//   - cleanup:    invoked AFTER the agent process exits (e.g. opencode mcp remove)
	//   - err:        non-nil aborts provider execution before spawning the CLI
	//
	// Providers without side-effects return (newArgs, options, noopMCPCleanup, nil).
	// Called only when cfg != nil && cfg.Enable && hooks.mcpInjector != nil.
	mcpInjector func(ctx context.Context, args []string, cfg *workflow.MCPProxyConfig, mcpConfigPath string, options map[string]any) (newArgs []string, newOptions map[string]any, cleanup func() error, err error)
}

// baseCLIProvider encapsulates the shared Execute and ExecuteConversation
// orchestration logic for all CLI-based agent providers.
type baseCLIProvider struct {
	name      string
	binary    string
	executor  ports.CLIExecutor
	logger    ports.Logger
	tokenizer ports.Tokenizer
	hooks     cliProviderHooks
}

func newBaseCLIProvider(name, binary string, executor ports.CLIExecutor, log ports.Logger, hooks cliProviderHooks) *baseCLIProvider {
	if log == nil {
		log = logger.NopLogger{}
	}
	return &baseCLIProvider{
		name:      name,
		binary:    binary,
		executor:  executor,
		logger:    log,
		tokenizer: fallbackTokenizer{},
		hooks:     hooks,
	}
}

// combineOutput merges stdout and stderr bytes into a single string.
// When one side is empty, conversion is done directly without extra allocation.
func combineOutput(stdoutBytes, stderrBytes []byte) string {
	if len(stderrBytes) == 0 {
		return string(stdoutBytes)
	}
	if len(stdoutBytes) == 0 {
		return string(stderrBytes)
	}
	return string(stdoutBytes) + string(stderrBytes)
}

func wantsRawDisplay(options map[string]any) bool {
	v, ok := getStringOption(options, "output_format")
	return ok && v == "json"
}

func (b *baseCLIProvider) applyStreamFilter(ctx context.Context, stdout io.Writer, rawDisplay bool) (io.Writer, *StreamFilterWriter) {
	if b.hooks.parseDisplayEvents == nil || rawDisplay {
		return stdout, nil
	}
	// When a per-step renderer is injected (ACP entry point), it owns the entire
	// agent stream (text + reasoning + tool events). Discard the inner writer so the
	// same text is not emitted twice; executor.Run still captures stdout independently.
	if r := display.RendererFromContext(ctx); r != nil {
		f := NewStreamFilterWriterWithParser(io.Discard, b.hooks.parseDisplayEvents, DisplayEventRenderer(r), b.logger)
		return f, f
	}
	if stdout != nil {
		f := NewStreamFilterWriterWithParser(stdout, b.hooks.parseDisplayEvents, nil, b.logger)
		return f, f
	}
	// stdout is nil but a parser is present: route through a filter backed by
	// io.Discard so that display events are still parsed (and emitted to any
	// future renderer wired into the filter). Without this the event stream is
	// lost silently because the filter is never created.
	f := NewStreamFilterWriterWithParser(io.Discard, b.hooks.parseDisplayEvents, nil, b.logger)
	return f, f
}

// execute runs the provider-specific CLI command and returns the AgentResult,
// the raw output string (for Response field population by callers), and any error.
func (b *baseCLIProvider) execute(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, string, error) {
	startedAt := time.Now()

	if strings.TrimSpace(prompt) == "" {
		return nil, "", errors.New("prompt cannot be empty")
	}

	if err := ctx.Err(); err != nil {
		return nil, "", fmt.Errorf("%s provider: %w", b.name, err)
	}

	if b.hooks.validateOptions != nil {
		if err := b.hooks.validateOptions(options); err != nil {
			return nil, "", err
		}
	}

	args, err := b.hooks.buildExecuteArgs(prompt, options)
	if err != nil {
		return nil, "", err
	}

	mcpCleanup := func() error { return nil }
	if b.hooks.mcpInjector != nil {
		if cfg, ok := options[workflow.MCPProxyConfigKey].(*workflow.MCPProxyConfig); ok && cfg != nil && cfg.Enable {
			path, _ := getStringOption(options, workflow.MCPProxyConfigPathKey)
			newArgs, newOpts, cleanup, injErr := b.hooks.mcpInjector(ctx, args, cfg, path, options)
			if injErr != nil {
				return nil, "", fmt.Errorf("%s mcp injector: %w", b.name, injErr)
			}
			args = newArgs
			options = newOpts
			mcpCleanup = cleanup
		}
	}
	defer func() {
		if cleanupErr := mcpCleanup(); cleanupErr != nil {
			b.logger.Warn("mcp cleanup failed", "error", cleanupErr)
		}
	}()

	rawDisplay := wantsRawDisplay(options)
	wrappedStdout, filter := b.applyStreamFilter(ctx, stdout, rawDisplay)
	env, envErr := cliProviderEnv(options)
	if envErr != nil {
		return nil, "", envErr
	}
	stdoutBytes, stderrBytes, err := runCLIWithProviderEnv(ctx, b.executor, b.binary, env, wrappedStdout, stderr, args...)
	completedAt := time.Now()
	if filter != nil {
		_ = filter.Flush()
	}

	if err != nil {
		return nil, "", fmt.Errorf("%s execution failed: %w", b.name, err)
	}

	rawOutput := combineOutput(stdoutBytes, stderrBytes)

	outputStr := rawOutput
	if outputStr == "" {
		outputStr = " "
	}

	var displayOutput string
	if !rawDisplay {
		displayOutput = extractDisplayTextFromEvents(rawOutput, b.hooks.parseDisplayEvents)
	}

	var outputTokens int
	hasRealTokens := false
	if b.hooks.extractTokenUsage != nil {
		if usage := b.hooks.extractTokenUsage(rawOutput); usage != nil {
			outputTokens = usage.TotalTokens
			hasRealTokens = true
		}
	}
	if !hasRealTokens {
		outputTokens, _ = b.tokenizer.CountTokens(outputStr) //nolint:errcheck // ApproximationTokenizer never errors with a valid ratio
	}

	result := &workflow.AgentResult{
		Provider:        b.name,
		Output:          outputStr,
		RawOutput:       rawOutput,
		DisplayOutput:   displayOutput,
		StartedAt:       startedAt,
		CompletedAt:     completedAt,
		Tokens:          outputTokens,
		TokensEstimated: !hasRealTokens && b.tokenizer.IsEstimate(),
	}

	return result, rawOutput, nil
}

// executeConversation runs the provider-specific CLI command in conversation mode
// and returns the ConversationResult, the raw output string, and any error.
//
//nolint:gocognit // Conversation orchestration manages multi-turn state, session extraction, token estimation, and output transformation in a single pipeline.
func (b *baseCLIProvider) executeConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, string, error) {
	startedAt := time.Now()

	if state == nil {
		return nil, "", errors.New("conversation state cannot be nil")
	}

	if strings.TrimSpace(prompt) == "" {
		return nil, "", errors.New("prompt cannot be empty")
	}

	if err := ctx.Err(); err != nil {
		return nil, "", fmt.Errorf("%s provider: %w", b.name, err)
	}

	if b.hooks.validateOptions != nil {
		if err := b.hooks.validateOptions(options); err != nil {
			return nil, "", err
		}
	}

	workingState := cloneState(state)

	args, err := b.hooks.buildConversationArgs(workingState, prompt, options)
	if err != nil {
		return nil, "", err
	}

	// F099: apply MCP injector when configured — mirrors the same pattern in execute().
	// The injector is invoked after buildConversationArgs so all provider-specific flags
	// are already in args before MCP flags are appended. newOptions may include a mutated
	// system_prompt for Codex/OpenCode coexistence mode.
	mcpCleanup := func() error { return nil }
	if b.hooks.mcpInjector != nil {
		if cfg, ok := options[workflow.MCPProxyConfigKey].(*workflow.MCPProxyConfig); ok && cfg != nil && cfg.Enable {
			path, _ := getStringOption(options, workflow.MCPProxyConfigPathKey)
			newArgs, newOpts, cleanup, injErr := b.hooks.mcpInjector(ctx, args, cfg, path, options)
			if injErr != nil {
				return nil, "", fmt.Errorf("%s mcp injector: %w", b.name, injErr)
			}
			args = newArgs
			options = newOpts
			mcpCleanup = cleanup
		}
	}
	defer func() {
		if cleanupErr := mcpCleanup(); cleanupErr != nil {
			b.logger.Warn("mcp cleanup failed", "error", cleanupErr)
		}
	}()

	userTurn := workflow.NewTurn(workflow.TurnRoleUser, prompt)
	if addErr := workingState.AddTurn(userTurn); addErr != nil {
		return nil, "", fmt.Errorf("failed to add user turn: %w", addErr)
	}

	rawDisplay := wantsRawDisplay(options)
	wrappedStdout, filter := b.applyStreamFilter(ctx, stdout, rawDisplay)
	env, envErr := cliProviderEnv(options)
	if envErr != nil {
		return nil, "", envErr
	}
	stdoutBytes, stderrBytes, err := runCLIWithProviderEnv(ctx, b.executor, b.binary, env, wrappedStdout, stderr, args...)
	completedAt := time.Now()
	if filter != nil {
		_ = filter.Flush()
	}

	if err != nil {
		return nil, "", fmt.Errorf("%s execution failed: %w", b.name, err)
	}

	rawOutput := combineOutput(stdoutBytes, stderrBytes)

	outputStr := rawOutput
	if b.hooks.extractTextContent != nil {
		if extracted := b.hooks.extractTextContent(rawOutput); extracted != "" {
			outputStr = extracted
		} else {
			outputStr = strings.TrimSpace(rawOutput)
		}
	}
	if outputStr == "" {
		outputStr = " "
	}

	assistantTurn := workflow.NewTurn(workflow.TurnRoleAssistant, outputStr)

	var assistantTokens, inputTokens int
	hasRealTokens := false
	if b.hooks.extractTokenUsage != nil {
		if usage := b.hooks.extractTokenUsage(rawOutput); usage != nil {
			assistantTokens = usage.OutputTokens
			inputTokens = usage.InputTokens
			hasRealTokens = true
		}
	}
	if !hasRealTokens {
		assistantTokens, _ = b.tokenizer.CountTokens(outputStr) //nolint:errcheck // ApproximationTokenizer never errors with a valid ratio
	}
	assistantTurn.Tokens = assistantTokens

	if addErr := workingState.AddTurn(assistantTurn); addErr != nil {
		return nil, "", fmt.Errorf("failed to add assistant turn: %w", addErr)
	}

	sessionID, err := b.hooks.extractSessionID(rawOutput)
	if err != nil {
		b.logger.Debug("session ID extraction failed, continuing stateless", "error", err)
	}
	workingState.SessionID = sessionID

	if !hasRealTokens {
		limit := len(workingState.Turns) - 1
		turnContents := make([]string, 0, limit)
		for _, t := range workingState.Turns[0:limit] {
			turnContents = append(turnContents, t.Content)
		}
		inputTokens, _ = b.tokenizer.CountTurnsTokens(turnContents) //nolint:errcheck // ApproximationTokenizer never errors with a valid ratio
	}

	var displayOutput string
	if !rawDisplay {
		displayOutput = extractDisplayTextFromEvents(rawOutput, b.hooks.parseDisplayEvents)
	}

	result := &workflow.ConversationResult{
		Provider:        b.name,
		State:           workingState,
		Output:          outputStr,
		RawOutput:       rawOutput,
		DisplayOutput:   displayOutput,
		TokensInput:     inputTokens,
		TokensOutput:    assistantTurn.Tokens,
		TokensTotal:     inputTokens + assistantTurn.Tokens,
		TokensEstimated: !hasRealTokens && b.tokenizer.IsEstimate(),
		StartedAt:       startedAt,
		CompletedAt:     completedAt,
	}

	return result, rawOutput, nil
}
