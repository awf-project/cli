package skills

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "empty string input returns empty string",
			content:  "",
			expected: "",
		},
		{
			name:     "content with no delimiters returns entire content unchanged",
			content:  "Just some body content\nwith multiple lines\nand no frontmatter",
			expected: "Just some body content\nwith multiple lines\nand no frontmatter",
		},
		{
			name:     "valid frontmatter returns body content",
			content:  "---\nname: test\n---\nbody content",
			expected: "body content",
		},
		{
			name:     "frontmatter only with no body returns empty string",
			content:  "---\nname: test\n---\n",
			expected: "",
		},
		{
			name:     "first delimiter present but no closing delimiter returns entire content unchanged",
			content:  "---\nname: test\nno closing delimiter\nbody content",
			expected: "---\nname: test\nno closing delimiter\nbody content",
		},
		{
			name:     "content not starting with delimiter returns entire content unchanged",
			content:  "leading text\n---\nname: test\n---\nbody",
			expected: "leading text\n---\nname: test\n---\nbody",
		},
		{
			name:     "multiple delimiters: only first pair treated as delimiters, third is body content",
			content:  "---\nname: test\n---\nbody content\n---\nmore content",
			expected: "body content\n---\nmore content",
		},
		{
			name:     "leading whitespace in body is trimmed",
			content:  "---\nname: test\n---\n  \n  body content",
			expected: "body content",
		},
		{
			name:     "trailing whitespace in body is trimmed",
			content:  "---\nname: test\n---\nbody content  \n  ",
			expected: "body content",
		},
		{
			name:     "both leading and trailing whitespace in body is trimmed",
			content:  "---\nname: test\n---\n   \nbody content with spaces\n   ",
			expected: "body content with spaces",
		},
		{
			name:     "frontmatter with complex YAML structure",
			content:  "---\nname: complex\ndescription: test skill\ntags:\n  - tag1\n  - tag2\n---\nActual body content here",
			expected: "Actual body content here",
		},
		{
			name:     "body with internal line breaks preserved",
			content:  "---\nname: test\n---\nline 1\nline 2\nline 3",
			expected: "line 1\nline 2\nline 3",
		},
		{
			name:     "single delimiter only (not at start) returns entire content unchanged",
			content:  "content\n---\nbody",
			expected: "content\n---\nbody",
		},
		{
			name:     "two delimiters without proper opening",
			content:  "content\n---\n---\nbody",
			expected: "content\n---\n---\nbody",
		},
		{
			name:     "tabs and mixed whitespace in body trimmed",
			content:  "---\nname: test\n---\n\t  \nbody\n  \t",
			expected: "body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripFrontmatter(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}
