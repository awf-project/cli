package agents

import (
	"encoding/json"
	"strings"

	"github.com/awf-project/cli/internal/domain/workflow"
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
		SessionID:   state.SessionID,
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

// findFirstNDJSONEvent scans NDJSON output and returns the first parsed event
// whose "type" field equals eventType. Returns nil if no match is found.
func findFirstNDJSONEvent(output, eventType string) map[string]any {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var evt map[string]any
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}
		if t, ok := evt["type"].(string); ok && t == eventType {
			return evt
		}
	}
	return nil
}
