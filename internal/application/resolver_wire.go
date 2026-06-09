package application

import "strings"

// Wire adapters are stateless pure functions (D6). Each converts between a
// per-interface identifier encoding and the canonical "pack/workflow" form.
// They are NOT methods on Resolver to enable 4-row table-driven round-trip tests (NFR-003).

// WireFromCLI returns the identifier unchanged; the CLI already uses canonical "/" format.
func WireFromCLI(s string) string { return s }

// WireFromHTTP converts an HTTP path identifier to canonical form.
// HTTP uses the same "/" separator as the canonical form.
func WireFromHTTP(s string) string { return s }

// WireFromACP converts a canonical identifier to ACP ":" form.
func WireFromACP(s string) string { return strings.ReplaceAll(s, "/", ":") }

// WireFromMCP converts a canonical identifier to MCP "_" form.
func WireFromMCP(s string) string { return strings.ReplaceAll(s, "/", "_") }

// WireToCLI returns the canonical identifier unchanged for CLI output.
func WireToCLI(s string) string { return s }

// WireToHTTP converts an HTTP identifier to canonical form.
func WireToHTTP(s string) string { return s }

// WireToACP converts an ACP ":" identifier to canonical "/" form.
func WireToACP(s string) string { return strings.ReplaceAll(s, ":", "/") }

// WireToMCP converts an MCP "_" identifier to canonical "/" form.
func WireToMCP(s string) string { return strings.ReplaceAll(s, "_", "/") }
