package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestResolverWire_ACPRoundTrip verifies ACP encode/decode behavior.
// DecodeFromACP replaces only the FIRST ":" — this is the documented bijectivity
// constraint for multi-segment paths (see resolver_wire.go package-level comment).
// A true round-trip is only guaranteed for two-segment canonical identifiers.
func TestResolverWire_ACPRoundTrip(t *testing.T) {
	tests := []struct {
		name        string
		canonical   string
		wantACP     string
		wantDecoded string // what DecodeFromACP(wantACP) yields (not always == canonical)
	}{
		{
			name:        "simple pack/workflow",
			canonical:   "pack/workflow",
			wantACP:     "pack:workflow",
			wantDecoded: "pack/workflow", // true round-trip for two-segment paths
		},
		{
			name:        "nested path",
			canonical:   "pack/nested/workflow",
			wantACP:     "pack:nested:workflow",
			wantDecoded: "pack/nested:workflow", // first ":" restored; remainder preserved
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acpForm := EncodeForACP(tt.canonical)
			assert.Equal(t, tt.wantACP, acpForm, "EncodeForACP")

			decoded := DecodeFromACP(acpForm)
			assert.Equal(t, tt.wantDecoded, decoded, "DecodeFromACP")
		})
	}
}

// TestResolverWire_MCPRoundTrip verifies MCP encode/decode behavior.
// DecodeFromMCP replaces only the FIRST "_" — this is the documented bijectivity
// constraint for multi-segment paths (see resolver_wire.go package-level comment).
// A true round-trip is only guaranteed for two-segment canonical identifiers.
func TestResolverWire_MCPRoundTrip(t *testing.T) {
	tests := []struct {
		name        string
		canonical   string
		wantMCP     string
		wantDecoded string // what DecodeFromMCP(wantMCP) yields (not always == canonical)
	}{
		{
			name:        "simple pack/workflow",
			canonical:   "pack/workflow",
			wantMCP:     "pack_workflow",
			wantDecoded: "pack/workflow", // true round-trip for two-segment paths
		},
		{
			name:        "nested path",
			canonical:   "pack/nested/workflow",
			wantMCP:     "pack_nested_workflow",
			wantDecoded: "pack/nested_workflow", // first "_" restored; remainder preserved
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mcpForm := EncodeForMCP(tt.canonical)
			assert.Equal(t, tt.wantMCP, mcpForm, "EncodeForMCP")

			decoded := DecodeFromMCP(mcpForm)
			assert.Equal(t, tt.wantDecoded, decoded, "DecodeFromMCP")
		})
	}
}

// TestResolverWire_TwoSegmentRoundTrip confirms exact round-trip for two-segment
// canonical identifiers (the only case where bijectivity is guaranteed).
func TestResolverWire_TwoSegmentRoundTrip(t *testing.T) {
	identifiers := []string{
		"simple/workflow",
		"pack-name/workflow-name",
	}

	for _, id := range identifiers {
		t.Run(id, func(t *testing.T) {
			acpResult := DecodeFromACP(EncodeForACP(id))
			assert.Equal(t, id, acpResult, "ACP round-trip failed")

			mcpResult := DecodeFromMCP(EncodeForMCP(id))
			assert.Equal(t, id, mcpResult, "MCP round-trip failed")
		})
	}
}

// TestResolverWire_MultipleIdentifiersRoundTrip documents the lossy decode behavior
// for multi-segment paths. Only the first separator is restored (bijectivity constraint).
func TestResolverWire_MultipleIdentifiersRoundTrip(t *testing.T) {
	tests := []struct {
		id      string
		wantACP string
		wantMCP string
	}{
		// two-segment: true round-trip
		{"simple/workflow", "simple/workflow", "simple/workflow"},
		{"pack-name/workflow-name", "pack-name/workflow-name", "pack-name/workflow-name"},
		// multi-segment: first separator only restored
		{"pack/namespace/workflow", "pack/namespace:workflow", "pack/namespace_workflow"},
		{"pack/deep/nested/workflow", "pack/deep:nested:workflow", "pack/deep_nested_workflow"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			acpResult := DecodeFromACP(EncodeForACP(tt.id))
			assert.Equal(t, tt.wantACP, acpResult, "ACP decode")

			mcpResult := DecodeFromMCP(EncodeForMCP(tt.id))
			assert.Equal(t, tt.wantMCP, mcpResult, "MCP decode")
		})
	}
}

// TestResolverWire_EncodeFormatConversions verifies that encoding produces the
// expected protocol-specific separator for a known canonical identifier.
func TestResolverWire_EncodeFormatConversions(t *testing.T) {
	tests := []struct {
		name      string
		canonical string
		expectACP string
		expectMCP string
	}{
		{
			name:      "simple pack/workflow",
			canonical: "pack/workflow",
			expectACP: "pack:workflow",
			expectMCP: "pack_workflow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acp := EncodeForACP(tt.canonical)
			assert.Equal(t, tt.expectACP, acp)

			mcp := EncodeForMCP(tt.canonical)
			assert.Equal(t, tt.expectMCP, mcp)
		})
	}
}

// TestResolverWire_EmptyStringHandling verifies that encode/decode functions
// never panic on empty input.
func TestResolverWire_EmptyStringHandling(t *testing.T) {
	assert.NotPanics(t, func() { _ = EncodeForACP("") })
	assert.NotPanics(t, func() { _ = DecodeFromACP("") })
	assert.NotPanics(t, func() { _ = EncodeForMCP("") })
	assert.NotPanics(t, func() { _ = DecodeFromMCP("") })
}
