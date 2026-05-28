package workflow

import (
	"fmt"
	"strings"
)

// AgentRoleSizeWarnBytes is the AGENTS.md size (in bytes) above which a
// context-window warning is emitted. Single source of truth for both the
// infrastructure loader and the CLI validator.
const AgentRoleSizeWarnBytes = 500 * 1024

// AgentRole holds the loaded content and metadata for an agent role.
type AgentRole struct {
	Name       string
	SourcePath string
	Content    string
	// RawSizeBytes is the size of the AGENTS.md file as read from disk (before
	// frontmatter stripping). Captured at load time so consumers can warn on
	// oversized files without re-stat'ing the source.
	RawSizeBytes int64
}

// AgentRoleNotFoundError is returned when an agent role cannot be resolved by name or path.
type AgentRoleNotFoundError struct {
	Name        string
	SearchPaths []string
	Underlying  error
	// IsPathRef indicates whether the lookup was triggered by a path-based reference
	// (LoadFromPath) rather than a symbolic name (Load). Consumers such as the CLI
	// validator use this to distinguish "role directory exists but lacks AGENTS.md"
	// from "role name not found in search paths", without relying on the number of
	// search paths as a heuristic.
	IsPathRef bool
}

func (e *AgentRoleNotFoundError) Error() string {
	return fmt.Sprintf("agent role %q not found in search paths: %s", e.Name, strings.Join(e.SearchPaths, ", "))
}

func (e *AgentRoleNotFoundError) Unwrap() error {
	return e.Underlying
}
