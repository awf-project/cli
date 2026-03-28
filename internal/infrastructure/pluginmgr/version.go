package pluginmgr

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version comparison operators supported by the constraint parser.
const (
	OpEqual          = "==" // Exact match
	OpNotEqual       = "!=" // Not equal
	OpGreater        = ">"  // Greater than
	OpGreaterOrEqual = ">=" // Greater than or equal
	OpLess           = "<"  // Less than
	OpLessOrEqual    = "<=" // Less than or equal
	OpTilde          = "~"  // Compatible with (allows patch updates)
	OpCaret          = "^"  // Compatible with (allows minor updates)
)

// Version represents a parsed semantic version.
type Version struct {
	Major      int    // Major version number
	Minor      int    // Minor version number
	Patch      int    // Patch version number
	Prerelease string // Prerelease identifier (e.g., "alpha.1")
}

// versionRegex matches semver strings: X.Y.Z or X.Y.Z-prerelease
// Does not allow leading zeros (except for 0 itself)
var versionRegex = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([a-zA-Z0-9]+(?:\.[a-zA-Z0-9]+)*))?$`)

// String returns the string representation of the version.
func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}
	return s
}

// Compare compares this version to another.
// Returns -1 if v < other, 0 if v == other, 1 if v > other.
func (v Version) Compare(other Version) int {
	// Compare major
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}

	// Compare minor
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}

	// Compare patch
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}

	// Compare prerelease
	// A version without prerelease is greater than one with prerelease
	// E.g., 1.0.0 > 1.0.0-alpha
	if v.Prerelease == "" && other.Prerelease != "" {
		return 1
	}
	if v.Prerelease != "" && other.Prerelease == "" {
		return -1
	}
	if v.Prerelease == other.Prerelease {
		return 0
	}

	// Both have prerelease, compare lexicographically
	if v.Prerelease < other.Prerelease {
		return -1
	}
	return 1
}

// Constraint represents a single version constraint (e.g., ">=0.4.0").
type Constraint struct {
	Operator string  // One of the Op* constants
	Version  Version // The version to compare against
}

// Check tests if a version satisfies this constraint.
func (c Constraint) Check(v Version) bool {
	cmp := v.Compare(c.Version)

	switch c.Operator {
	case OpEqual:
		return cmp == 0
	case OpNotEqual:
		return cmp != 0
	case OpGreater:
		return cmp > 0
	case OpGreaterOrEqual:
		return cmp >= 0
	case OpLess:
		return cmp < 0
	case OpLessOrEqual:
		return cmp <= 0
	case OpTilde:
		// Tilde: allows patch updates (~1.2.0 means >=1.2.0 <1.3.0)
		// Must have same major and minor, patch >= constraint patch
		if v.Major != c.Version.Major || v.Minor != c.Version.Minor {
			return false
		}
		return v.Patch >= c.Version.Patch
	case OpCaret:
		// Caret: allows minor updates for 1.x.x, patch updates for 0.x.x
		// ^1.2.0 means >=1.2.0 <2.0.0
		// ^0.2.0 means >=0.2.0 <0.3.0 (special case for 0.x)
		if v.Major != c.Version.Major {
			return false
		}
		if c.Version.Major == 0 {
			// For 0.x versions, caret only allows patch updates
			if v.Minor != c.Version.Minor {
				return false
			}
			return v.Patch >= c.Version.Patch
		}
		// For 1.x+ versions, caret allows minor updates
		if v.Minor < c.Version.Minor {
			return false
		}
		if v.Minor == c.Version.Minor {
			return v.Patch >= c.Version.Patch
		}
		return true
	default:
		return false
	}
}

// Constraints represents multiple version constraints that all must be satisfied.
type Constraints []Constraint

// Check tests if a version satisfies all constraints.
func (cs Constraints) Check(v Version) bool {
	// Empty constraints always satisfied
	if len(cs) == 0 {
		return true
	}
	for _, c := range cs {
		if !c.Check(v) {
			return false
		}
	}
	return true
}

// String returns the string representation of the constraints.
func (cs Constraints) String() string {
	if len(cs) == 0 {
		return ""
	}
	parts := make([]string, len(cs))
	for i, c := range cs {
		parts[i] = c.Operator + c.Version.String()
	}
	return strings.Join(parts, " ")
}

// ParseVersion parses a version string into a Version struct.
// Accepts formats: "1.0.0", "1.0.0-alpha.1"
func ParseVersion(s string) (Version, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Version{}, errors.New("version: empty string")
	}

	matches := versionRegex.FindStringSubmatch(s)
	if matches == nil {
		return Version{}, fmt.Errorf("version: invalid format %q", s)
	}

	// Parse is safe because regex already validated these are digits
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return Version{}, fmt.Errorf("version: invalid major %q", matches[1])
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return Version{}, fmt.Errorf("version: invalid minor %q", matches[2])
	}
	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return Version{}, fmt.Errorf("version: invalid patch %q", matches[3])
	}
	prerelease := ""
	if len(matches) > 4 {
		prerelease = matches[4]
	}

	return Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: prerelease,
	}, nil
}

// constraintRegex matches constraint strings with optional operator
var constraintRegex = regexp.MustCompile(`^\s*(==|!=|>=|<=|>|<|~|\^)?\s*(.+)$`)

// ParseConstraint parses a constraint string into a Constraint struct.
// Accepts formats: ">=0.4.0", "~1.2.0", "^2.0.0", "1.0.0" (implies ==)
func ParseConstraint(s string) (Constraint, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Constraint{}, errors.New("constraint: empty string")
	}

	matches := constraintRegex.FindStringSubmatch(s)
	if matches == nil {
		return Constraint{}, fmt.Errorf("constraint: invalid format %q", s)
	}

	operator := matches[1]
	versionStr := strings.TrimSpace(matches[2])

	// Default to equal if no operator
	if operator == "" {
		operator = OpEqual
	}

	version, err := ParseVersion(versionStr)
	if err != nil {
		return Constraint{}, fmt.Errorf("constraint: %w", err)
	}

	return Constraint{
		Operator: operator,
		Version:  version,
	}, nil
}

// ParseConstraints parses a constraint string that may contain multiple constraints.
// Constraints are separated by spaces or commas.
// Examples: ">=0.4.0 <1.0.0", ">=0.4.0, <1.0.0"
func ParseConstraints(s string) (Constraints, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("constraints: empty string")
	}

	// Split by comma or spaces, handling multiple delimiters
	// First replace commas with spaces, then split by whitespace
	s = strings.ReplaceAll(s, ",", " ")
	parts := strings.Fields(s)

	if len(parts) == 0 {
		return nil, errors.New("constraints: no valid constraints found")
	}

	// Preallocate for expected constraints
	constraints := make(Constraints, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		c, err := ParseConstraint(part)
		if err != nil {
			return nil, fmt.Errorf("constraints: %w", err)
		}
		constraints = append(constraints, c)
	}

	if len(constraints) == 0 {
		return nil, errors.New("constraints: no valid constraints found")
	}

	return constraints, nil
}

// CheckVersionConstraint checks if a version string satisfies a constraint string.
// This is the main entry point for version compatibility checking.
func CheckVersionConstraint(constraintStr, versionStr string) (bool, error) {
	if constraintStr == "" {
		return false, errors.New("version check: empty constraint")
	}
	if versionStr == "" {
		return false, errors.New("version check: empty version")
	}

	constraints, err := ParseConstraints(constraintStr)
	if err != nil {
		return false, err
	}

	version, err := ParseVersion(versionStr)
	if err != nil {
		return false, err
	}

	return constraints.Check(version), nil
}

// IsCompatible checks if the current AWF version is compatible with a plugin's
// AWF version constraint. This is a convenience function.
func IsCompatible(awfVersionConstraint, currentAWFVersion string) (bool, error) {
	return CheckVersionConstraint(awfVersionConstraint, currentAWFVersion)
}

// NormalizeTag strips a leading "v" prefix from a GitHub release tag.
func NormalizeTag(tag string) string {
	return strings.TrimPrefix(tag, "v")
}

// IsPrerelease returns true if this version has a prerelease identifier.
func (v Version) IsPrerelease() bool {
	return v.Prerelease != ""
}
