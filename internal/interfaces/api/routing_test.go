package api

import "testing"

func TestRecomposeIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		scope    string
		input    string
		expected string
	}{
		{
			name:     "local scope returns bare name",
			scope:    "local",
			input:    "deploy-prod",
			expected: "deploy-prod",
		},
		{
			name:     "global scope returns bare name (regression: CompositeRepository resolves globally)",
			scope:    "global",
			input:    "audit",
			expected: "audit",
		},
		{
			name:     "env scope returns bare name (regression: env source is also a repo scope)",
			scope:    "env",
			input:    "custom",
			expected: "custom",
		},
		{
			name:     "pack scope composes scope/name",
			scope:    "speckit",
			input:    "specify",
			expected: "speckit/specify",
		},
		{
			name:     "arbitrary pack name composes scope/name",
			scope:    "acme",
			input:    "deploy",
			expected: "acme/deploy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := recomposeIdentifier(tt.scope, tt.input)
			if got != tt.expected {
				t.Errorf("recomposeIdentifier(%q, %q) = %q, want %q", tt.scope, tt.input, got, tt.expected)
			}
		})
	}
}
