// Package plugin provides domain entities for the plugin system.
package plugin

import "errors"

// ErrNotImplemented indicates a stub method that needs implementation.
var ErrNotImplemented = errors.New("not implemented")

// Valid capability names that plugins can declare.
const (
	CapabilityOperations = "operations"
	CapabilityCommands   = "commands"
	CapabilityValidators = "validators"
)

// ValidCapabilities lists all recognized plugin capabilities.
var ValidCapabilities = []string{
	CapabilityOperations,
	CapabilityCommands,
	CapabilityValidators,
}

// Valid configuration field types.
const (
	ConfigTypeString  = "string"
	ConfigTypeInteger = "integer"
	ConfigTypeBoolean = "boolean"
)

// ValidConfigTypes lists all recognized configuration field types.
var ValidConfigTypes = []string{
	ConfigTypeString,
	ConfigTypeInteger,
	ConfigTypeBoolean,
}

// Manifest represents plugin metadata loaded from plugin.yaml.
type Manifest struct {
	Name         string                 // Plugin unique identifier (required, alphanumeric + hyphens)
	Version      string                 // Semantic version (required, e.g., "1.0.0")
	Description  string                 // Human-readable description (optional)
	AWFVersion   string                 // Version constraint (required, e.g., ">=0.4.0")
	Author       string                 // Plugin author name/email (optional)
	License      string                 // SPDX license identifier (optional)
	Homepage     string                 // Plugin documentation URL (optional)
	Capabilities []string               // List of capabilities: "operations", "commands", "validators"
	Config       map[string]ConfigField // Configuration schema (optional)
}

// ConfigField defines a configuration parameter for a plugin.
type ConfigField struct {
	Type        string   // "string", "integer", "boolean"
	Required    bool     // Whether field is required (default: false)
	Default     any      // Default value (must match Type)
	Description string   // Field documentation
	Enum        []string // Allowed values (for string type only)
}

// Validate checks if the manifest is valid.
// Stub: returns ErrNotImplemented.
func (m *Manifest) Validate() error {
	return ErrNotImplemented
}

// HasCapability checks if the plugin declares a specific capability.
func (m *Manifest) HasCapability(capability string) bool {
	for _, cap := range m.Capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}
