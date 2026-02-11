package plugin

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/vanoix/awf/internal/domain/plugin"
)

// ManifestParseError represents an error during plugin manifest parsing.
type ManifestParseError struct {
	File    string // file path
	Field   string // field path (e.g., "config.webhook_url")
	Message string // error message
	Cause   error  // underlying error
}

// Error implements the error interface.
func (e *ManifestParseError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s: %s", e.File, e.Field, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.File, e.Message)
}

// Unwrap returns the underlying error.
func (e *ManifestParseError) Unwrap() error {
	return e.Cause
}

// NewManifestParseError creates a new ManifestParseError with field and message.
func NewManifestParseError(file, field, message string) *ManifestParseError {
	return &ManifestParseError{
		File:    file,
		Field:   field,
		Message: message,
	}
}

// WrapManifestParseError wraps an existing error as a ManifestParseError.
func WrapManifestParseError(file string, cause error) *ManifestParseError {
	return &ManifestParseError{
		File:    file,
		Message: cause.Error(),
		Cause:   cause,
	}
}

// yamlManifest is the YAML representation of a plugin manifest.
type yamlManifest struct {
	Name         string                     `yaml:"name"`
	Version      string                     `yaml:"version"`
	Description  string                     `yaml:"description"`
	AWFVersion   string                     `yaml:"awf_version"`
	Author       string                     `yaml:"author"`
	License      string                     `yaml:"license"`
	Homepage     string                     `yaml:"homepage"`
	Capabilities []string                   `yaml:"capabilities"`
	Config       map[string]yamlConfigField `yaml:"config"`
}

// yamlConfigField is the YAML representation of a plugin configuration field.
type yamlConfigField struct {
	Type        string   `yaml:"type"`
	Required    bool     `yaml:"required"`
	Default     any      `yaml:"default"`
	Description string   `yaml:"description"`
	Enum        []string `yaml:"enum"`
}

// ManifestParser parses plugin manifests from YAML files.
type ManifestParser struct{}

// NewManifestParser creates a new ManifestParser.
func NewManifestParser() *ManifestParser {
	return &ManifestParser{}
}

// ParseFile reads and parses a plugin manifest from a file path.
func (p *ManifestParser) ParseFile(path string) (*plugin.Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, WrapManifestParseError(path, err)
	}
	defer file.Close()

	manifest, err := p.parse(file, path)
	if err != nil {
		return nil, err
	}

	return manifest, nil
}

// Parse reads and parses a plugin manifest from an io.Reader.
func (p *ManifestParser) Parse(r io.Reader) (*plugin.Manifest, error) {
	return p.parse(r, "<reader>")
}

// parse is the internal implementation that parses from a reader with a source identifier.
func (p *ManifestParser) parse(r io.Reader, source string) (*plugin.Manifest, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, WrapManifestParseError(source, err)
	}

	if len(data) == 0 {
		return nil, NewManifestParseError(source, "", "empty manifest file")
	}

	var yamlM yamlManifest
	if err := yaml.Unmarshal(data, &yamlM); err != nil {
		return nil, WrapManifestParseError(source, err)
	}

	// Validate required fields
	if err := validateYAMLManifest(&yamlM, source); err != nil {
		return nil, err
	}

	return mapToDomain(&yamlM), nil
}

// validateYAMLManifest checks that all required fields are present.
func validateYAMLManifest(m *yamlManifest, source string) error {
	if m.Name == "" {
		return NewManifestParseError(source, "name", "required field missing")
	}
	if m.Version == "" {
		return NewManifestParseError(source, "version", "required field missing")
	}
	if m.AWFVersion == "" {
		return NewManifestParseError(source, "awf_version", "required field missing")
	}
	return nil
}

// mapToDomain converts a yamlManifest to a domain Manifest.
func mapToDomain(m *yamlManifest) *plugin.Manifest {
	manifest := &plugin.Manifest{
		Name:         m.Name,
		Version:      m.Version,
		Description:  m.Description,
		AWFVersion:   m.AWFVersion,
		Author:       m.Author,
		License:      m.License,
		Homepage:     m.Homepage,
		Capabilities: m.Capabilities,
	}

	if len(m.Config) > 0 {
		manifest.Config = make(map[string]plugin.ConfigField, len(m.Config))
		for name, field := range m.Config {
			manifest.Config[name] = plugin.ConfigField{
				Type:        field.Type,
				Required:    field.Required,
				Default:     field.Default,
				Description: field.Description,
				Enum:        field.Enum,
			}
		}
	}

	return manifest
}
