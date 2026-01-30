// Package plugin provides domain entities for the plugin system.
package plugin

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
)

// ErrNotImplemented indicates a stub method that needs implementation.
var ErrNotImplemented = errors.New("not implemented")

// namePattern enforces manifest name validation rules.
// Names must start with a lowercase letter followed by lowercase letters, digits, or hyphens.
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

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
// It validates name format, version, AWFVersion, capabilities, and config fields.
func (m *Manifest) Validate() error {
	// 1. Name validation
	if m.Name == "" {
		return errors.New("name cannot be empty")
	}
	if !namePattern.MatchString(m.Name) {
		return fmt.Errorf("invalid name %q: must match pattern ^[a-z][a-z0-9-]*$", m.Name)
	}

	// 2. Version validation
	if m.Version == "" {
		return errors.New("version cannot be empty")
	}

	// 3. AWFVersion validation
	if m.AWFVersion == "" {
		return errors.New("awf_version cannot be empty")
	}

	// 4. Capabilities validation
	for _, cap := range m.Capabilities {
		if !slices.Contains(ValidCapabilities, cap) {
			return fmt.Errorf("invalid capability %q: must be one of %v", cap, ValidCapabilities)
		}
	}

	// 5. Config validation - validate each config field
	for name, cf := range m.Config {
		if err := validateConfigField(name, &cf); err != nil {
			return fmt.Errorf("config field %q: %w", name, err)
		}
	}

	return nil
}

// validateConfigField validates a single configuration field.
// It checks that the type is valid, enum constraints are only on strings,
// and default values match the declared type.
func validateConfigField(name string, cf *ConfigField) error {
	// 1. Type validation: must be non-empty and in ValidConfigTypes
	if cf.Type == "" {
		return fmt.Errorf("type cannot be empty")
	}
	if !slices.Contains(ValidConfigTypes, cf.Type) {
		return fmt.Errorf("invalid type %q: must be one of %v", cf.Type, ValidConfigTypes)
	}

	// 2. Enum validation: only allowed on string types
	if len(cf.Enum) > 0 && cf.Type != ConfigTypeString {
		return fmt.Errorf("enum is only allowed on string type, got %q", cf.Type)
	}

	// 3. Default value type validation: must match declared Type (if not nil)
	if cf.Default != nil {
		if err := validateDefaultType(cf.Type, cf.Default); err != nil {
			return err
		}
	}

	return nil
}

// validateDefaultType checks if the default value type matches the declared config field type.
// Supports JSON unmarshaling behavior where numbers become float64.
func validateDefaultType(configType string, defaultValue any) error {
	switch configType {
	case ConfigTypeString:
		if _, ok := defaultValue.(string); !ok {
			return fmt.Errorf("default value type mismatch: expected string, got %T", defaultValue)
		}
	case ConfigTypeInteger:
		// JSON decoding may produce float64 for integer types, so accept both int and float64
		switch defaultValue.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float64:
			// Valid integer types
		default:
			return fmt.Errorf("default value type mismatch: expected integer or float64, got %T", defaultValue)
		}
	case ConfigTypeBoolean:
		if _, ok := defaultValue.(bool); !ok {
			return fmt.Errorf("default value type mismatch: expected bool, got %T", defaultValue)
		}
	}
	return nil
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
