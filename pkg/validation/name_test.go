package validation_test

import (
	"testing"

	"github.com/awf-project/cli/pkg/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateName_Valid(t *testing.T) {
	valid := []string{
		"mypack",
		"my-pack",
		"my-pack-123",
		"a",
		"speckit",
		"hello-world",
		"abc123",
	}
	for _, name := range valid {
		t.Run(name, func(t *testing.T) {
			err := validation.ValidateName(name)
			assert.NoError(t, err, "expected %q to be valid", name)
		})
	}
}

func TestValidateName_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		errContains string
	}{
		{
			name:        "empty string",
			input:       "",
			errContains: "invalid name",
		},
		{
			name:        "path traversal with ..",
			input:       "../etc/passwd",
			errContains: "invalid name",
		},
		{
			name:        "multiple path traversal segments",
			input:       "../../etc/passwd",
			errContains: "invalid name",
		},
		{
			name:        "starts with digit",
			input:       "1pack",
			errContains: "invalid name",
		},
		{
			name:        "uppercase letters",
			input:       "MyPack",
			errContains: "invalid name",
		},
		{
			name:        "underscore",
			input:       "my_pack",
			errContains: "invalid name",
		},
		{
			name:        "slash separator",
			input:       "pack/workflow",
			errContains: "invalid name",
		},
		{
			name:        "absolute path",
			input:       "/etc/passwd",
			errContains: "invalid name",
		},
		{
			name:        "dot prefix",
			input:       ".hidden",
			errContains: "invalid name",
		},
		{
			name:        "space in name",
			input:       "my pack",
			errContains: "invalid name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateName(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

// TestValidateName_RejectsDotDot is the security-critical case:
// a name containing ".." must always be rejected, regardless of surrounding
// characters, to prevent path traversal via filepath.Join(baseDir, name).
func TestValidateName_RejectsDotDot(t *testing.T) {
	traversalAttempts := []string{
		"..",
		"../",
		"../etc",
		"../../etc/passwd",
		"a/../b",
		"pack-..foo",
	}
	for _, attempt := range traversalAttempts {
		t.Run(attempt, func(t *testing.T) {
			err := validation.ValidateName(attempt)
			require.Error(t, err, "path traversal attempt %q must be rejected", attempt)
		})
	}
}
