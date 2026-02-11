package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vanoix/awf/internal/domain/workflow"
)

// =============================================================================
// Token Estimation - Consolidated from 5 providers
// =============================================================================

// estimateTokens provides rough token estimation based on character count.
// Previously duplicated as estimateTokens, estimateCodexTokens, estimateGeminiTokens,
// estimateOpenCodeTokens, estimateCustomTokens.
func estimateTokens(output string) int {
	// Rough estimation: ~4 characters per token
	return len(output) / 4
}

// =============================================================================
// State Cloning - Consolidated from 3 providers
// =============================================================================

// cloneState creates a shallow copy of ConversationState to avoid mutating the original.
// Previously duplicated as cloneState, cloneCodexState, cloneGeminiState.
func cloneState(state *workflow.ConversationState) *workflow.ConversationState {
	if state == nil {
		return nil
	}

	// Create new state with copied turns slice
	turns := make([]workflow.Turn, len(state.Turns))
	copy(turns, state.Turns)

	return &workflow.ConversationState{
		Turns:       turns,
		TotalTurns:  state.TotalTurns,
		TotalTokens: state.TotalTokens,
		StoppedBy:   state.StoppedBy,
	}
}

// =============================================================================
// Type-Checked Option Getters - C001 Pattern
// =============================================================================

// getStringOption extracts a string option from the options map.
// Returns the value and true if found and correctly typed, otherwise zero value and false.
func getStringOption(options map[string]any, key string) (string, bool) {
	if options == nil {
		return "", false
	}
	val, ok := options[key].(string)
	return val, ok
}

// getIntOption extracts an int option from the options map.
// Returns the value and true if found and correctly typed, otherwise zero value and false.
func getIntOption(options map[string]any, key string) (int, bool) {
	if options == nil {
		return 0, false
	}
	val, ok := options[key].(int)
	return val, ok
}

// getFloatOption extracts a float64 option from the options map.
// Returns the value and true if found and correctly typed, otherwise zero value and false.
func getFloatOption(options map[string]any, key string) (float64, bool) {
	if options == nil {
		return 0, false
	}
	val, ok := options[key].(float64)
	return val, ok
}

// getBoolOption extracts a bool option from the options map.
// Returns the value and true if found and correctly typed, otherwise zero value and false.
func getBoolOption(options map[string]any, key string) (value, found bool) {
	if options == nil {
		return false, false
	}
	val, ok := options[key].(bool)
	return val, ok
}

// =============================================================================
// Validation Helpers - Consolidated from 6+ occurrences
// =============================================================================

// validatePrompt validates that a prompt is non-empty.
// Previously duplicated 6x across providers.
func validatePrompt(prompt string) error {
	if strings.TrimSpace(prompt) == "" {
		return fmt.Errorf("prompt cannot be empty")
	}
	return nil
}

// validateContext checks if the context has been cancelled.
// Previously duplicated 6x across providers with provider-specific error prefix.
func validateContext(ctx context.Context, providerName string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s provider: %w", providerName, err)
	}
	return nil
}

// validateState validates that a conversation state is non-nil.
func validateState(state *workflow.ConversationState) error {
	if state == nil {
		return fmt.Errorf("conversation state cannot be nil")
	}
	return nil
}

// =============================================================================
// Metadata Extractors - From Claude provider
// =============================================================================

// getWorkflowID extracts workflow ID from options for logging.
func getWorkflowID(options map[string]any) string {
	if id, ok := getStringOption(options, "workflowID"); ok {
		return id
	}
	return "unknown"
}

// getStepName extracts step name from options for logging.
func getStepName(options map[string]any) string {
	if name, ok := getStringOption(options, "stepName"); ok {
		return name
	}
	return "unknown"
}

// =============================================================================
// Token Estimation Loop - Consolidated from 3 providers
// =============================================================================

// estimateInputTokens calculates total input tokens from conversation turns.
// Previously duplicated in Claude, Codex, and Gemini ExecuteConversation.
func estimateInputTokens(turns []workflow.Turn, excludeLastN int) int {
	inputTokens := 0
	limit := len(turns) - excludeLastN
	if limit < 0 {
		limit = 0
	}
	for i := 0; i < limit; i++ {
		if turns[i].Tokens == 0 {
			turns[i].Tokens = estimateTokens(turns[i].Content)
		}
		inputTokens += turns[i].Tokens
	}
	return inputTokens
}

// =============================================================================
// JSON Response Parsing - Consolidated from Claude and Gemini
// =============================================================================

// parseJSONResponse attempts to parse JSON from raw output.
// Returns the parsed map and nil error on success, nil and error on failure.
func parseJSONResponse(output []byte) (map[string]any, error) {
	var jsonResp map[string]any
	if err := json.Unmarshal(output, &jsonResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w", err)
	}
	return jsonResp, nil
}

// tryParseJSONResponse attempts to parse JSON without returning error.
// Returns parsed map or nil if parsing fails (for heuristic JSON detection).
func tryParseJSONResponse(output string) map[string]any {
	trimmed := strings.TrimSpace(output)
	if !strings.HasPrefix(trimmed, "{") {
		return nil
	}
	var jsonResp map[string]any
	if err := json.Unmarshal([]byte(trimmed), &jsonResp); err != nil {
		return nil
	}
	return jsonResp
}
