package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/logger"
)

// OpenCodeProvider implements AgentProvider for OpenCode CLI.
// Invokes: opencode run "prompt"
type OpenCodeProvider struct {
	base     *baseCLIProvider
	logger   ports.Logger
	executor ports.CLIExecutor
}

// NewOpenCodeProvider creates a new OpenCodeProvider.
// If no executor is provided, ExecCLIExecutor is used by default.
func NewOpenCodeProvider() *OpenCodeProvider {
	p := &OpenCodeProvider{
		logger:   logger.NopLogger{},
		executor: NewExecCLIExecutor(),
	}
	p.base = p.newBase()
	return p
}

// NewOpenCodeProviderWithOptions creates a new OpenCodeProvider with functional options.
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
	return newBaseCLIProvider("opencode", "opencode", p.executor, p.logger, cliProviderHooks{
		buildExecuteArgs:      p.buildExecuteArgs,
		buildConversationArgs: p.buildConversationArgs,
		extractSessionID:      p.extractSessionID,
		validateOptions:       validateOpenCodeOptions,
		parseStreamLine:       p.parseOpencodeStreamLine,
	})
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
	if extracted := extractDisplayText(rawOutput, p.parseOpencodeStreamLine); extracted != "" {
		result.Output = extracted
		result.Tokens = estimateTokens(extracted)
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
		// OpenCode has no equivalent flag; log at debug level so operators are aware the option was present but ignored.
		p.logger.Debug("dangerously_skip_permissions is not supported by OpenCode and will be ignored")
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
		if sysPrompt, ok := getStringOption(options, "system_prompt"); ok && sysPrompt != "" {
			effectivePrompt = sysPrompt + "\n\n" + prompt
		}
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
		p.logger.Debug("dangerously_skip_permissions is not supported by OpenCode and will be ignored")
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

// parseOpencodeStreamLine extracts displayable assistant text from OpenCode CLI's
// stream-json output. OpenCode CLI (`opencode run --format json`) emits one JSON
// object per line with these top-level types:
//   - "step_start"  — {sessionID, part} (ignored; session ID consumed separately)
//   - "text"        — {part:{text}} (surface the text field)
//   - "step_finish" — {sessionID, part:{tokens, cost}} (ignored)
//
// Only "text" events are surfaced to the live display. All other events (step
// metadata, tool use, reasoning blocks) are skipped.
func (p *OpenCodeProvider) parseOpencodeStreamLine(line []byte) string {
	var evt struct {
		Type string `json:"type"`
		Part struct {
			Text string `json:"text"`
		} `json:"part"`
	}
	if err := json.Unmarshal(line, &evt); err != nil {
		return ""
	}
	if evt.Type != "text" {
		return ""
	}
	return evt.Part.Text
}
