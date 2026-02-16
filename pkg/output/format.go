// Package output provides format-specific post-processing for agent step outputs.
// It supports JSON validation/parsing and markdown code fence stripping.
package output

import (
	"encoding/json"
	"fmt"
	"regexp"
)

var (
	codeFenceRegex = regexp.MustCompile(`(?s)^\s*` + "```" + `[a-zA-Z0-9]*\r?\n(.*?)\r?\n` + "```")
	fenceMarker    = regexp.MustCompile("```")
	endsWithFence  = regexp.MustCompile(`\n` + "```" + `\s*$`)
)

// StripCodeFences removes outermost markdown code fences from input.
// Returns the inner content or the original input if no fences found.
func StripCodeFences(input string) string {
	matches := codeFenceRegex.FindStringSubmatch(input)
	if len(matches) > 1 {
		content := matches[1]
		fenceCount := len(fenceMarker.FindAllString(content, -1))
		if fenceCount > 0 && endsWithFence.MatchString(input) {
			greedyRegex := regexp.MustCompile(`(?s)^\s*` + "```" + `[a-zA-Z0-9]*\r?\n(.*)\r?\n` + "```" + `\s*$`)
			greedyMatches := greedyRegex.FindStringSubmatch(input)
			if len(greedyMatches) > 1 {
				return greedyMatches[1]
			}
		}
		return content
	}
	return input
}

// ValidateAndParseJSON validates JSON syntax and parses into map or slice.
// Returns parsed data as map[string]any or []any, or error if invalid.
func ValidateAndParseJSON(input string) (any, error) {
	var result any
	if err := json.Unmarshal([]byte(input), &result); err != nil {
		preview := input
		if len(input) > 200 {
			preview = input[:200]
		}
		return nil, fmt.Errorf("invalid JSON: %w (content: %s)", err, preview)
	}
	return result, nil
}

// ProcessOutputFormat applies format-specific processing to agent output.
// For "json": strips fences then validates/parses JSON.
// For "text": strips fences only.
// For "": returns output unchanged with nil parsed data.
func ProcessOutputFormat(output, format string) (processed string, parsed any, err error) {
	switch format {
	case "json":
		processed = StripCodeFences(output)
		parsed, err = ValidateAndParseJSON(processed)
		return processed, parsed, err
	case "text":
		processed = StripCodeFences(output)
		return processed, nil, nil
	default:
		return output, nil, nil
	}
}
