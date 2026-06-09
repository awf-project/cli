package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolverWire_RoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		canonical string
		wireType  string
	}{
		{
			name:      "CLI identity",
			canonical: "pack/workflow",
			wireType:  "cli",
		},
		{
			name:      "CLI with nested workflow",
			canonical: "my-pack/nested/workflow",
			wireType:  "cli",
		},
		{
			name:      "HTTP converts forward slashes",
			canonical: "pack/workflow",
			wireType:  "http",
		},
		{
			name:      "ACP uses colon separator",
			canonical: "pack/workflow",
			wireType:  "acp",
		},
		{
			name:      "MCP uses underscore separator",
			canonical: "pack/workflow",
			wireType:  "mcp",
		},
		{
			name:      "ACP with nested workflow",
			canonical: "pack/nested/workflow",
			wireType:  "acp",
		},
		{
			name:      "MCP with nested workflow",
			canonical: "pack/nested/workflow",
			wireType:  "mcp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var encoded string
			var decoded string

			switch tt.wireType {
			case "cli":
				// CLI: identity transform
				encoded = WireFromCLI(tt.canonical)
				decoded = WireToCLI(encoded)

			case "http":
				// HTTP: canonical is "pack/workflow", HTTP represents it appropriately
				encoded = WireFromHTTP(tt.canonical)
				decoded = WireToHTTP(encoded)

			case "acp":
				// ACP: colon separator (pack:workflow)
				encoded = WireFromACP(tt.canonical)
				decoded = WireToACP(encoded)

			case "mcp":
				// MCP: underscore separator (pack_workflow)
				encoded = WireFromMCP(tt.canonical)
				decoded = WireToMCP(encoded)
			}

			// Round-trip should return to original canonical form
			assert.Equal(t, tt.canonical, decoded,
				"round-trip through %s wire should preserve canonical form", tt.wireType)
		})
	}
}

func TestResolverWire_CLIFormat(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantOutput string
	}{
		{
			name:       "CLI returns input unchanged",
			input:      "pack/workflow",
			wantOutput: "pack/workflow",
		},
		{
			name:       "CLI preserves nested paths",
			input:      "pack/nested/deep/workflow",
			wantOutput: "pack/nested/deep/workflow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WireFromCLI(tt.input)
			assert.Equal(t, tt.wantOutput, result)

			result2 := WireToCLI(tt.input)
			assert.Equal(t, tt.wantOutput, result2)
		})
	}
}

func TestResolverWire_ACPFormat(t *testing.T) {
	tests := []struct {
		name          string
		canonical     string
		wantACP       string
		wantCanonical string
	}{
		{
			name:          "ACP converts slashes to colons",
			canonical:     "pack/workflow",
			wantACP:       "pack:workflow",
			wantCanonical: "pack/workflow",
		},
		{
			name:          "ACP with nested path",
			canonical:     "pack/nested/workflow",
			wantACP:       "pack:nested:workflow",
			wantCanonical: "pack/nested/workflow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acpForm := WireFromACP(tt.canonical)
			assert.Equal(t, tt.wantACP, acpForm)

			canonical := WireToACP(acpForm)
			assert.Equal(t, tt.wantCanonical, canonical)
		})
	}
}

func TestResolverWire_MCPFormat(t *testing.T) {
	tests := []struct {
		name          string
		canonical     string
		wantMCP       string
		wantCanonical string
	}{
		{
			name:          "MCP converts slashes to underscores",
			canonical:     "pack/workflow",
			wantMCP:       "pack_workflow",
			wantCanonical: "pack/workflow",
		},
		{
			name:          "MCP with nested path",
			canonical:     "pack/nested/workflow",
			wantMCP:       "pack_nested_workflow",
			wantCanonical: "pack/nested/workflow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mcpForm := WireFromMCP(tt.canonical)
			assert.Equal(t, tt.wantMCP, mcpForm)

			canonical := WireToMCP(mcpForm)
			assert.Equal(t, tt.wantCanonical, canonical)
		})
	}
}

func TestResolverWire_HTTPFormat(t *testing.T) {
	tests := []struct {
		name          string
		canonical     string
		wantHTTP      string
		wantCanonical string
	}{
		{
			name:          "HTTP round-trip preserves format",
			canonical:     "pack/workflow",
			wantCanonical: "pack/workflow",
		},
		{
			name:          "HTTP with nested path",
			canonical:     "pack/nested/workflow",
			wantCanonical: "pack/nested/workflow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpForm := WireFromHTTP(tt.canonical)
			canonical := WireToHTTP(httpForm)
			assert.Equal(t, tt.wantCanonical, canonical)
		})
	}
}

func TestResolverWire_AllFormatsConvertFromCanonical(t *testing.T) {
	canonical := "pack/workflow"

	// Test that all FromX functions accept canonical format
	cliForm := WireFromCLI(canonical)
	assert.NotEmpty(t, cliForm)

	httpForm := WireFromHTTP(canonical)
	assert.NotEmpty(t, httpForm)

	acpForm := WireFromACP(canonical)
	assert.NotEmpty(t, acpForm)

	mcpForm := WireFromMCP(canonical)
	assert.NotEmpty(t, mcpForm)
}

func TestResolverWire_AllFormatsConvertToCanonical(t *testing.T) {
	canonical := "pack/workflow"

	// All wire formats should convert back to canonical
	cliCanonical := WireToCLI(canonical)
	assert.Equal(t, canonical, cliCanonical)

	httpCanonical := WireToHTTP(canonical)
	assert.Equal(t, canonical, httpCanonical)

	acpCanonical := WireToACP("pack:workflow")
	assert.Equal(t, canonical, acpCanonical)

	mcpCanonical := WireToMCP("pack_workflow")
	assert.Equal(t, canonical, mcpCanonical)
}

func TestResolverWire_MultipleIdentifiersRoundTrip(t *testing.T) {
	identifiers := []string{
		"simple/workflow",
		"pack-name/workflow-name",
		"pack/namespace/workflow",
		"pack/deep/nested/workflow",
	}

	for _, id := range identifiers {
		t.Run(id, func(t *testing.T) {
			// Test each wire format preserves through round-trip
			cliResult := WireToCLI(WireFromCLI(id))
			assert.Equal(t, id, cliResult, "CLI round-trip failed")

			httpResult := WireToHTTP(WireFromHTTP(id))
			assert.Equal(t, id, httpResult, "HTTP round-trip failed")

			acpEncoded := WireFromACP(id)
			acpResult := WireToACP(acpEncoded)
			assert.Equal(t, id, acpResult, "ACP round-trip failed")

			mcpEncoded := WireFromMCP(id)
			mcpResult := WireToMCP(mcpEncoded)
			assert.Equal(t, id, mcpResult, "MCP round-trip failed")
		})
	}
}

func TestResolverWire_CanonicalFormat(t *testing.T) {
	tests := []struct {
		name       string
		canonical  string
		expectCLI  string
		expectHTTP string
		expectACP  string
		expectMCP  string
	}{
		{
			name:       "simple pack/workflow",
			canonical:  "pack/workflow",
			expectCLI:  "pack/workflow",
			expectHTTP: "pack/workflow",
			expectACP:  "pack:workflow",
			expectMCP:  "pack_workflow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := WireFromCLI(tt.canonical)
			assert.Equal(t, tt.expectCLI, cli)

			http := WireFromHTTP(tt.canonical)
			assert.Equal(t, tt.expectHTTP, http)

			acp := WireFromACP(tt.canonical)
			assert.Equal(t, tt.expectACP, acp)

			mcp := WireFromMCP(tt.canonical)
			assert.Equal(t, tt.expectMCP, mcp)
		})
	}
}

func TestResolverWire_EmptyStringHandling(t *testing.T) {
	// Test that wire functions handle empty strings (may return empty or identity)
	cliResult := WireFromCLI("")
	assert.NotPanics(t, func() { _ = cliResult })

	httpResult := WireFromHTTP("")
	assert.NotPanics(t, func() { _ = httpResult })

	acpResult := WireFromACP("")
	assert.NotPanics(t, func() { _ = acpResult })

	mcpResult := WireFromMCP("")
	assert.NotPanics(t, func() { _ = mcpResult })
}
