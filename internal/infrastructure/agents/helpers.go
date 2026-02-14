package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vanoix/awf/internal/domain/workflow"
)

func estimateTokens(output string) int {
	return len(output) / 4
}

func cloneState(state *workflow.ConversationState) *workflow.ConversationState {
	if state == nil {
		return nil
	}

	turns := make([]workflow.Turn, len(state.Turns))
	copy(turns, state.Turns)

	return &workflow.ConversationState{
		Turns:       turns,
		TotalTurns:  state.TotalTurns,
		TotalTokens: state.TotalTokens,
		StoppedBy:   state.StoppedBy,
	}
}

func getStringOption(options map[string]any, key string) (string, bool) {
	if options == nil {
		return "", false
	}
	val, ok := options[key].(string)
	return val, ok
}

func getIntOption(options map[string]any, key string) (int, bool) {
	if options == nil {
		return 0, false
	}
	val, ok := options[key].(int)
	return val, ok
}

func getFloatOption(options map[string]any, key string) (float64, bool) {
	if options == nil {
		return 0, false
	}
	val, ok := options[key].(float64)
	return val, ok
}

func getBoolOption(options map[string]any, key string) (value, found bool) {
	if options == nil {
		return false, false
	}
	val, ok := options[key].(bool)
	return val, ok
}

func validatePrompt(prompt string) error {
	if strings.TrimSpace(prompt) == "" {
		return fmt.Errorf("prompt cannot be empty")
	}
	return nil
}

func validateContext(ctx context.Context, providerName string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s provider: %w", providerName, err)
	}
	return nil
}

func validateState(state *workflow.ConversationState) error {
	if state == nil {
		return fmt.Errorf("conversation state cannot be nil")
	}
	return nil
}

func getWorkflowID(options map[string]any) string {
	if id, ok := getStringOption(options, "workflowID"); ok {
		return id
	}
	return "unknown"
}

func getStepName(options map[string]any) string {
	if name, ok := getStringOption(options, "stepName"); ok {
		return name
	}
	return "unknown"
}

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

func parseJSONResponse(output []byte) (map[string]any, error) {
	var jsonResp map[string]any
	if err := json.Unmarshal(output, &jsonResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w", err)
	}
	return jsonResp, nil
}

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
