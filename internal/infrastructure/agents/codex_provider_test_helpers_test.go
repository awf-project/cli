package agents

import "encoding/json"

// ndjsonLine builds a single Codex NDJSON `item.completed` envelope wrapping an
// assistant_message with the given text. Shared across the codex provider test
// files (unit + delegation) so the envelope shape lives in one place.
//
// The text is JSON-escaped via json.Marshal so callers may pass quotes,
// backslashes, or control characters without producing a malformed envelope —
// matching the project convention of testing handlers with special characters.
func ndjsonLine(text string) string {
	escaped, _ := json.Marshal(text) //nolint:errcheck // json.Marshal never fails for a Go string value
	return `{"type":"item.completed","item":{"item_type":"assistant_message","text":` + string(escaped) + `}}`
}
