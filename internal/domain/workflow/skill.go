package workflow

import (
	"fmt"
	"strings"
)

// Skill holds the loaded content and metadata for a skill.
type Skill struct {
	Name      string
	Content   string
	Location  string
	Resources []string
}

// SkillReference is a value object representing a skill declaration in YAML.
// Either Name (discovery-based) or Path (explicit) is set, never both.
type SkillReference struct {
	Name string
	Path string
}

// IsPathBased returns true when the reference uses an explicit file path.
func (s SkillReference) IsPathBased() bool {
	return s.Path != ""
}

// SkillNotFoundError is returned when a skill cannot be resolved by name or path.
type SkillNotFoundError struct {
	Name        string
	SearchPaths []string
}

func (e *SkillNotFoundError) Error() string {
	return fmt.Sprintf("skill %q not found in search paths: %s", e.Name, strings.Join(e.SearchPaths, ", "))
}
