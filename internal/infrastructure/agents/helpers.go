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

// argPreviewKeys defines the ordered list of input map keys used to extract a
// human-readable preview for EventToolUse.Arg. The first matching key wins.
var argPreviewKeys = []string{"file_path", "command", "cmd", "query", "pattern"}

// truncateArg returns a preview of s capped at 40 Unicode characters.
// If s exceeds 40 characters it is truncated to 37 runes and the ellipsis
// character (U+2026) is appended, yielding exactly 40 visible characters.
func truncateArg(s string) string {
	const maxLen = 40
	const truncLen = 37
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:truncLen]) + "…"
}

// extractArgPreviewFromMap extracts a human-readable preview from a tool input map.
// It looks for the first recognized key (file_path, command, cmd, query, pattern)
// and returns its string value, truncated per truncateArg contract.
// Returns empty string if the map is nil or no recognized key is found.
func extractArgPreviewFromMap(m map[string]any) string {
	for _, key := range argPreviewKeys {
		if val, ok := m[key]; ok {
			if s, ok := val.(string); ok {
				return truncateArg(s)
			}
		}
	}
	return ""
}

// extractArgPreview parses a JSON-encoded tool arguments string and delegates to
// extractArgPreviewFromMap for key extraction and truncation.
// Returns empty string if the JSON is unparseable or no recognized key is found.
func extractArgPreview(arguments string) string {
	if arguments == "" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(arguments), &m); err != nil {
		return ""
	}
	return extractArgPreviewFromMap(m)
}

// parseToolCallArgPreview is an alias for extractArgPreview retained for
// compatibility with providers that use the longer name.
func parseToolCallArgPreview(arguments string) string {
	return extractArgPreview(arguments)
}
