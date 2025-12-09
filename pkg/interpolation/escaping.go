package interpolation

import "strings"

// ShellEscape escapes a string for safe use in shell commands.
// Wraps in single quotes and escapes embedded single quotes.
func ShellEscape(s string) string {
	if s == "" {
		return "''"
	}
	if !needsEscaping(s) {
		return s
	}
	// Escape single quotes: ' -> '\''
	escaped := strings.ReplaceAll(s, "'", `'\''`)
	return "'" + escaped + "'"
}

func needsEscaping(s string) bool {
	for _, c := range s {
		switch c {
		case ' ', '\t', '\n', '"', '\'', '\\', '$', '`', '!', '*', '?',
			'[', ']', '(', ')', '{', '}', '<', '>', '|', '&', ';':
			return true
		}
	}
	return false
}

// NoEscape returns the string unchanged.
// Use for cases where escaping is not needed (e.g., in non-shell contexts).
func NoEscape(s string) string {
	return s
}
