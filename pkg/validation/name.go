// Package validation provides shared validation primitives for pack and workflow names.
//
// This package is the single source of truth for name validation across all layers
// (CLI, TUI, infrastructure/workflowpkg). Centralizing the regex here ensures that
// every path-construction call site applies the same guard before filepath.Join.
//
// # Name rule
//
// Valid names match ^[a-z][a-z0-9-]*$:
//   - Start with a lowercase ASCII letter.
//   - Contain only lowercase ASCII letters, digits, and hyphens.
//   - No dots, slashes, underscores, spaces, or uppercase letters.
//
// This rule is stricter than strictly necessary for correctness, but the strictness
// is intentional: it makes path-traversal attacks structurally impossible because
// ".." and "/" are both rejected, so filepath.Join(baseDir, name) can never escape
// baseDir for any name that passes ValidateName.
package validation

import (
	"fmt"
	"regexp"
)

// nameRegex is the single authoritative pattern for pack and workflow names.
// Kept unexported to force callers through ValidateName, which produces a
// consistent error message.
var nameRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// ValidateName returns a non-nil error when name does not conform to the pack /
// workflow naming rule (^[a-z][a-z0-9-]*$).
//
// Because the rule forbids ".", "/" and "..", a name that passes this check is
// safe to use as a single path component in filepath.Join without further guards.
func ValidateName(name string) error {
	if !nameRegex.MatchString(name) {
		return fmt.Errorf("invalid name %q: must match ^[a-z][a-z0-9-]*$", name)
	}
	return nil
}
