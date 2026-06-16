package application

import (
	"fmt"
	"strings"
)

// Wire adapters are stateless pure functions (D6). Each converts between a
// per-interface identifier encoding and the canonical "pack/workflow" form.
// They are NOT methods on Resolver to enable table-driven round-trip tests (NFR-003).
//
// Naming convention:
//   - EncodeFor<Protocol> converts a canonical "/" identifier to the protocol's native form.
//   - DecodeFrom<Protocol> converts the protocol's native form back to canonical "/" form.
//
// CLI and HTTP carry identifiers in canonical "/" form natively, so no encode/decode
// functions are provided for them — callers pass the identifier through unchanged.
//
// # Bijectivity constraint
//
// MCP uses "_" as the pack/workflow separator; ACP uses ":".
// The encoding is bijective only when the canonical identifier does NOT contain the
// target separator character:
//
//   - EncodeForMCP is bijective iff the canonical form contains no "_".
//     Counterexample: "my_pack/wf" and "my/pack_wf" both encode to "my_pack_wf".
//   - EncodeForACP is bijective iff the canonical form contains no ":".
//     Counterexample: "pack:x/wf" and "pack/x:wf" both encode to "pack:x:wf".
//
// Use the validated variants (ValidateAndEncodeForMCP / ValidateAndEncodeForACP)
// at any trust boundary where the canonical identifier comes from user input. The
// plain Encode/Decode functions are kept as string→string for backward compatibility
// and test usage; they do not error but the round-trip is only valid when the
// bijectivity constraint is satisfied.
//
// # Round-trip guarantee
//
// DecodeFromMCP(EncodeForMCP(s)) == s when s contains no "_".
// DecodeFromACP(EncodeForACP(s)) when s contains no ":".

// EncodeForMCP converts a canonical "pack/workflow" identifier to MCP "pack_workflow" form.
//
// The encoding replaces every "/" with "_". It is bijective only when the canonical
// identifier contains no underscores — see the package-level bijectivity constraint.
// Use ValidateAndEncodeForMCP when encoding user-supplied identifiers.
func EncodeForMCP(s string) string { return strings.ReplaceAll(s, "/", "_") }

// DecodeFromMCP converts an MCP "pack_workflow" identifier to canonical "pack/workflow" form.
//
// The decoding replaces the first "_" with "/" to recover the canonical form.
// It is bijective only when the original canonical identifier contained no underscores —
// see the package-level bijectivity constraint.
func DecodeFromMCP(s string) string {
	idx := strings.Index(s, "_")
	if idx < 0 {
		return s // no separator: bare workflow name, return as-is
	}
	// Replace only the first "_" to recover the canonical "/" separator; remaining
	// underscores are part of the pack or workflow name and are preserved verbatim.
	return s[:idx] + "/" + s[idx+1:]
}

// EncodeForACP converts a canonical "pack/workflow" identifier to ACP "pack:workflow" form.
//
// The encoding replaces every "/" with ":". It is bijective only when the canonical
// identifier contains no colons — see the package-level bijectivity constraint.
// Use ValidateAndEncodeForACP when encoding user-supplied identifiers.
func EncodeForACP(s string) string { return strings.ReplaceAll(s, "/", ":") }

// DecodeFromACP converts an ACP "pack:workflow" identifier to canonical "pack/workflow" form.
//
// The decoding replaces the first ":" with "/" to recover the canonical form.
// It is bijective only when the original canonical identifier contained no colons —
// see the package-level bijectivity constraint.
func DecodeFromACP(s string) string {
	idx := strings.Index(s, ":")
	if idx < 0 {
		return s // no separator: bare workflow name, return as-is
	}
	// Replace only the first ":" to recover the canonical "/" separator; remaining
	// colons are part of the pack or workflow name and are preserved verbatim.
	return s[:idx] + "/" + s[idx+1:]
}

// ValidateAndEncodeForMCP validates that the canonical identifier satisfies the MCP
// bijectivity constraint (no "_" characters) and then encodes it to MCP form.
//
// Returns an error when the canonical identifier contains an underscore, because the
// MCP encoding would not be reversible: EncodeForMCP("a_b/c") == EncodeForMCP("a/b_c").
// Pack and workflow names MUST NOT contain underscores when used over the MCP protocol.
func ValidateAndEncodeForMCP(s string) (string, error) {
	if strings.Contains(s, "_") {
		return "", fmt.Errorf("ValidateAndEncodeForMCP: canonical identifier %q contains reserved MCP separator '_'; pack and workflow names must not contain underscores for MCP encoding", s)
	}
	return EncodeForMCP(s), nil
}

// ValidateAndEncodeForACP validates that the canonical identifier satisfies the ACP
// bijectivity constraint (no ":" characters) and then encodes it to ACP form.
//
// Returns an error when the canonical identifier contains a colon, because the
// ACP encoding would not be reversible: EncodeForACP("a:b/c") == EncodeForACP("a/b:c").
// Pack and workflow names MUST NOT contain colons when used over the ACP protocol.
func ValidateAndEncodeForACP(s string) (string, error) {
	if strings.Contains(s, ":") {
		return "", fmt.Errorf("ValidateAndEncodeForACP: canonical identifier %q contains reserved ACP separator ':'; pack and workflow names must not contain colons for ACP encoding", s)
	}
	return EncodeForACP(s), nil
}
