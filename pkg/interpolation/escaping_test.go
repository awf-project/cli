package interpolation_test

import (
	"testing"

	"github.com/awf-project/awf/pkg/interpolation"
	"github.com/stretchr/testify/assert"
)

func TestShellEscape(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string unchanged",
			input:    "simple",
			expected: "simple",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "''",
		},
		{
			name:     "with space",
			input:    "with space",
			expected: "'with space'",
		},
		{
			name:     "with tab",
			input:    "with\ttab",
			expected: "'with\ttab'",
		},
		{
			name:     "with newline",
			input:    "with\nnewline",
			expected: "'with\nnewline'",
		},
		{
			name:     "with single quote",
			input:    "it's quoted",
			expected: `'it'\''s quoted'`,
		},
		{
			name:     "command substitution",
			input:    "$(rm -rf /)",
			expected: "'$(rm -rf /)'",
		},
		{
			name:     "backtick substitution",
			input:    "`whoami`",
			expected: "'`whoami`'",
		},
		{
			name:     "semicolon",
			input:    "a;b",
			expected: "'a;b'",
		},
		{
			name:     "pipe",
			input:    "a|b",
			expected: "'a|b'",
		},
		{
			name:     "ampersand",
			input:    "a&b",
			expected: "'a&b'",
		},
		{
			name:     "redirect greater",
			input:    "a>b",
			expected: "'a>b'",
		},
		{
			name:     "redirect less",
			input:    "a<b",
			expected: "'a<b'",
		},
		{
			name:     "glob asterisk",
			input:    "a*b",
			expected: "'a*b'",
		},
		{
			name:     "glob question",
			input:    "a?b",
			expected: "'a?b'",
		},
		{
			name:     "dollar sign",
			input:    "$HOME",
			expected: "'$HOME'",
		},
		{
			name:     "double quotes",
			input:    `"hello"`,
			expected: `'"hello"'`,
		},
		{
			name:     "backslash",
			input:    `a\b`,
			expected: `'a\b'`,
		},
		{
			name:     "exclamation mark",
			input:    "hello!",
			expected: "'hello!'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := interpolation.ShellEscape(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestNoEscape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with space", "with space"},
		{"$(rm -rf /)", "$(rm -rf /)"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := interpolation.NoEscape(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}
