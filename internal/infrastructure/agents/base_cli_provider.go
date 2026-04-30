package agents

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/logger"
)

// cliProviderHooks captures provider-specific behavior as function values.
// Optional hooks (extractTextContent, validateOptions, parseStreamLine) may be nil.
type cliProviderHooks struct {
	buildExecuteArgs      func(prompt string, options map[string]any) ([]string, error)
	buildConversationArgs func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error)
	extractSessionID      func(output string) (string, error)
	extractTextContent    func(output string) string
	validateOptions       func(options map[string]any) error
	parseStreamLine       LineExtractor
}

// baseCLIProvider encapsulates the shared Execute and ExecuteConversation
// orchestration logic for all CLI-based agent providers.
type baseCLIProvider struct {
	name     string
	binary   string
	executor ports.CLIExecutor
	logger   ports.Logger
	hooks    cliProviderHooks
}

func newBaseCLIProvider(name, binary string, executor ports.CLIExecutor, log ports.Logger, hooks cliProviderHooks) *baseCLIProvider {
	if log == nil {
		log = logger.NopLogger{}
	}
	return &baseCLIProvider{
		name:     name,
		binary:   binary,
		executor: executor,
		logger:   log,
		hooks:    hooks,
	}
}

// combineOutput merges stdout and stderr bytes into a single string.
func combineOutput(stdoutBytes, stderrBytes []byte) string {
	output := make([]byte, 0, len(stdoutBytes)+len(stderrBytes))
	output = append(output, stdoutBytes...)
	output = append(output, stderrBytes...)
	return string(output)
}

func wantsRawDisplay(options map[string]any) bool {
	v, ok := getStringOption(options, "output_format")
	return ok && v == "json"
}

func (b *baseCLIProvider) applyStreamFilter(stdout io.Writer, rawDisplay bool) (io.Writer, *StreamFilterWriter) {
	if b.hooks.parseStreamLine != nil && !rawDisplay && stdout != nil {
		f := NewStreamFilterWriter(stdout, b.hooks.parseStreamLine, b.logger)
		return f, f
	}
	return stdout, nil
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

	rawDisplay := wantsRawDisplay(options)
	wrappedStdout, filter := b.applyStreamFilter(stdout, rawDisplay)
	stdoutBytes, stderrBytes, err := b.executor.Run(ctx, b.binary, wrappedStdout, stderr, args...)
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
		displayOutput = extractDisplayText(rawOutput, b.hooks.parseStreamLine)
	}

	result := &workflow.AgentResult{
		Provider:        b.name,
		Output:          outputStr,
		DisplayOutput:   displayOutput,
		StartedAt:       startedAt,
		CompletedAt:     completedAt,
		Tokens:          estimateTokens(outputStr),
		TokensEstimated: true,
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

	userTurn := workflow.NewTurn(workflow.TurnRoleUser, prompt)
	if addErr := workingState.AddTurn(userTurn); addErr != nil {
		return nil, "", fmt.Errorf("failed to add user turn: %w", addErr)
	}

	rawDisplay := wantsRawDisplay(options)
	wrappedStdout, filter := b.applyStreamFilter(stdout, rawDisplay)
	stdoutBytes, stderrBytes, err := b.executor.Run(ctx, b.binary, wrappedStdout, stderr, args...)
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
	assistantTurn.Tokens = estimateTokens(outputStr)
	if addErr := workingState.AddTurn(assistantTurn); addErr != nil {
		return nil, "", fmt.Errorf("failed to add assistant turn: %w", addErr)
	}

	sessionID, err := b.hooks.extractSessionID(rawOutput)
	if err != nil {
		b.logger.Debug("session ID extraction failed, continuing stateless", "error", err)
	}
	workingState.SessionID = sessionID

	inputTokens := estimateInputTokens(workingState.Turns, 1)

	var displayOutput string
	if !rawDisplay {
		displayOutput = extractDisplayText(rawOutput, b.hooks.parseStreamLine)
	}

	result := &workflow.ConversationResult{
		Provider:        b.name,
		State:           workingState,
		Output:          outputStr,
		DisplayOutput:   displayOutput,
		TokensInput:     inputTokens,
		TokensOutput:    assistantTurn.Tokens,
		TokensTotal:     inputTokens + assistantTurn.Tokens,
		TokensEstimated: true,
		StartedAt:       startedAt,
		CompletedAt:     completedAt,
	}

	return result, rawOutput, nil
}
