package agents

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/logger"
)

// CodexProvider implements AgentProvider for Codex CLI.
// Invokes: codex exec --json "prompt"
type CodexProvider struct {
	logger   ports.Logger
	executor ports.CLIExecutor
}

func NewCodexProvider() *CodexProvider {
	return &CodexProvider{
		logger:   logger.NopLogger{},
		executor: NewExecCLIExecutor(),
	}
}

func NewCodexProviderWithOptions(opts ...CodexProviderOption) *CodexProvider {
	p := &CodexProvider{
		logger:   logger.NopLogger{},
		executor: NewExecCLIExecutor(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *CodexProvider) Execute(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
	startedAt := time.Now()

	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("codex provider: %w", err)
	}

	args := []string{"exec", "--json", prompt}

	if language, ok := getStringOption(options, "language"); ok {
		args = append(args, "--language", language)
	}
	if model, ok := getStringOption(options, "model"); ok {
		args = append(args, "--model", model)
	}
	if skipPerms, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skipPerms {
		args = append(args, "--yolo")
	}

	stdoutBytes, stderrBytes, err := p.executor.Run(ctx, "codex", stdout, stderr, args...)
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("codex execution failed: %w", err)
	}

	output := make([]byte, 0, len(stdoutBytes)+len(stderrBytes))
	output = append(output, stdoutBytes...)
	output = append(output, stderrBytes...)
	outputStr := string(output)
	result := &workflow.AgentResult{
		Provider:    "codex",
		Output:      outputStr,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		Tokens:      estimateTokens(outputStr),
	}

	return result, nil
}

func (p *CodexProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
	startedAt := time.Now()

	if state == nil {
		return nil, errors.New("conversation state cannot be nil")
	}

	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("codex provider: %w", err)
	}

	workingState := cloneState(state)

	userTurn := workflow.NewTurn(workflow.TurnRoleUser, prompt)
	if err := workingState.AddTurn(userTurn); err != nil {
		return nil, fmt.Errorf("failed to add user turn: %w", err)
	}

	// Only session IDs with the "codex-" prefix (issued by the Codex CLI) use the
	// resume subcommand. Unknown-format IDs skip resume but still suppress system
	// prompt (the session is ongoing, even if not resumable by subcommand).
	isResume := strings.HasPrefix(workingState.SessionID, "codex-")
	if !isResume && workingState.SessionID != "" {
		// NFR-002: log only a prefix of the session ID to avoid leaking full value.
		prefixLen := min(10, len(workingState.SessionID))
		p.logger.Debug("session ID does not have codex- prefix, skipping resume",
			"session_id_prefix", workingState.SessionID[:prefixLen])
	}

	var args []string
	if isResume {
		args = []string{"resume", workingState.SessionID, "--json", prompt}
	} else {
		args = []string{"exec", "--json", prompt}
		// First turn only (no session yet): pass system prompt if provided
		if workingState.SessionID == "" {
			if sysPrompt, ok := getStringOption(options, "system_prompt"); ok && sysPrompt != "" {
				args = append(args, "--system-prompt", sysPrompt)
			}
		}
	}

	if model, ok := getStringOption(options, "model"); ok {
		args = append(args, "--model", model)
	}
	if language, ok := getStringOption(options, "language"); ok {
		args = append(args, "--language", language)
	}
	if skipPerms, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skipPerms {
		args = append(args, "--yolo")
	}

	stdoutBytes, stderrBytes, err := p.executor.Run(ctx, "codex", stdout, stderr, args...)
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("codex execution failed: %w", err)
	}

	output := make([]byte, 0, len(stdoutBytes)+len(stderrBytes))
	output = append(output, stdoutBytes...)
	output = append(output, stderrBytes...)
	outputStr := string(output)
	if outputStr == "" {
		outputStr = " "
	}

	// Extract session ID for future resume turns; log and continue if not found.
	if sessionID, extractErr := extractSessionIDFromLines(outputStr); extractErr == nil {
		workingState.SessionID = sessionID
	} else {
		workingState.SessionID = ""
	}

	assistantTurn := workflow.NewTurn(workflow.TurnRoleAssistant, outputStr)
	assistantTurn.Tokens = estimateTokens(outputStr)
	if err := workingState.AddTurn(assistantTurn); err != nil {
		return nil, fmt.Errorf("failed to add assistant turn: %w", err)
	}

	inputTokens := estimateInputTokens(workingState.Turns, 1)

	result := &workflow.ConversationResult{
		Provider:        "codex",
		State:           workingState,
		Output:          outputStr,
		TokensInput:     inputTokens,
		TokensOutput:    assistantTurn.Tokens,
		TokensTotal:     inputTokens + assistantTurn.Tokens,
		TokensEstimated: true,
		StartedAt:       startedAt,
		CompletedAt:     completedAt,
	}

	return result, nil
}

// Name returns the provider identifier.
func (p *CodexProvider) Name() string {
	return "codex"
}

// Validate checks if the Codex CLI is installed and accessible.
func (p *CodexProvider) Validate() error {
	_, err := exec.LookPath("codex")
	if err != nil {
		return fmt.Errorf("codex CLI not found in PATH: %w", err)
	}
	return nil
}
