package workflowpkg

import (
	"fmt"
	"time"
)

// PackSource represents the origin and installation metadata of an external workflow pack.
// Serialized to PackState.SourceData for persistence.
type PackSource struct {
	Repository  string // e.g., "owner/awf-workflow-example"
	Version     string // e.g., "1.0.0"
	InstalledAt time.Time
	UpdatedAt   time.Time
}

// PackInfo holds metadata about an installed workflow pack, parsed from manifest.yaml.
type PackInfo struct {
	Name        string            // Pack name (lowercase, alphanumeric with dashes)
	Version     string            // Semver version
	Description string            // Human-readable description
	Author      string            // Author contact
	License     string            // License identifier
	Workflows   map[string]string // Workflow filename → path mapping
	Plugins     map[string]string // Plugin name → version requirement mapping
}

// PackState represents the persisted state of an installed workflow pack.
// Stored in workflow-packs/<name>/state.json.
type PackState struct {
	Name       string         `json:"name"`
	Enabled    bool           `json:"enabled"`
	SourceData map[string]any `json:"source_data"`
}

// SourceDataFromPackSource converts a PackSource to a map[string]any for persistence.
func SourceDataFromPackSource(source *PackSource) (map[string]any, error) {
	if source == nil {
		return nil, fmt.Errorf("source cannot be nil")
	}
	return map[string]any{
		"repository":   source.Repository,
		"version":      source.Version,
		"installed_at": source.InstalledAt,
		"updated_at":   source.UpdatedAt,
	}, nil
}

// PackSourceFromSourceData converts persisted map[string]any back to PackSource.
// Returns error if data is nil, missing required fields, or type mismatches.
func PackSourceFromSourceData(data map[string]any) (*PackSource, error) {
	if data == nil {
		return nil, fmt.Errorf("source data cannot be nil")
	}

	// Extract and validate repository
	repoVal, ok := data["repository"]
	if !ok {
		return nil, fmt.Errorf("missing required field: repository")
	}
	repo, ok := repoVal.(string)
	if !ok {
		return nil, fmt.Errorf("repository must be a string, got %T", repoVal)
	}

	// Extract and validate version
	versionVal, ok := data["version"]
	if !ok {
		return nil, fmt.Errorf("missing required field: version")
	}
	version, ok := versionVal.(string)
	if !ok {
		return nil, fmt.Errorf("version must be a string, got %T", versionVal)
	}

	// Extract optional installed_at (may be time.Time or string after JSON roundtrip)
	var installedAt time.Time
	if iVal, ok := data["installed_at"]; ok {
		installedAt = parseTimeValue(iVal)
	}

	// Extract optional updated_at (may be time.Time or string after JSON roundtrip)
	var updatedAt time.Time
	if uVal, ok := data["updated_at"]; ok {
		updatedAt = parseTimeValue(uVal)
	}

	return &PackSource{
		Repository:  repo,
		Version:     version,
		InstalledAt: installedAt,
		UpdatedAt:   updatedAt,
	}, nil
}

// parseTimeValue handles both direct time.Time objects and RFC3339 strings
// (the latter occur after JSON serialization roundtrips).
func parseTimeValue(val any) time.Time {
	switch v := val.(type) {
	case time.Time:
		return v
	case string:
		t, _ := time.Parse(time.RFC3339, v)
		return t
	}
	return time.Time{}
}
