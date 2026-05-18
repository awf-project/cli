package workflow

import (
	"fmt"
	"strings"
)

// AgentRole holds the loaded content and metadata for an agent role.
type AgentRole struct {
	Name       string
	SourcePath string
	Content    string
}

// AgentRoleNotFoundError is returned when an agent role cannot be resolved by name or path.
type AgentRoleNotFoundError struct {
	Name        string
	SearchPaths []string
	Underlying  error
}

func (e *AgentRoleNotFoundError) Error() string {
	return fmt.Sprintf("agent role %q not found in search paths: %s", e.Name, strings.Join(e.SearchPaths, ", "))
}

func (e *AgentRoleNotFoundError) Unwrap() error {
	return e.Underlying
}
