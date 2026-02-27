// Package plugin provides domain entities for the plugin system.
package pluginmodel

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
)

var ErrNotImplemented = errors.New("not implemented")

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

const (
	CapabilityOperations = "operations"
	CapabilityCommands   = "commands"
	CapabilityValidators = "validators"
)

var ValidCapabilities = []string{
	CapabilityOperations,
	CapabilityCommands,
	CapabilityValidators,
}

const (
	ConfigTypeString  = "string"
	ConfigTypeInteger = "integer"
	ConfigTypeBoolean = "boolean"
)

var ValidConfigTypes = []string{
	ConfigTypeString,
	ConfigTypeInteger,
	ConfigTypeBoolean,
}

type Manifest struct {
	Name         string
	Version      string
	Description  string
	AWFVersion   string
	Author       string
	License      string
	Homepage     string
	Capabilities []string
	Config       map[string]ConfigField
}

type ConfigField struct {
	Type        string
	Required    bool
	Default     any
	Description string
	Enum        []string
}

func (m *Manifest) Validate() error {
	if m.Name == "" {
		return errors.New("name cannot be empty")
	}
	if !namePattern.MatchString(m.Name) {
		return fmt.Errorf("invalid name %q: must match pattern ^[a-z][a-z0-9-]*$", m.Name)
	}

	if m.Version == "" {
		return errors.New("version cannot be empty")
	}

	if m.AWFVersion == "" {
		return errors.New("awf_version cannot be empty")
	}

	for _, cap := range m.Capabilities {
		if !slices.Contains(ValidCapabilities, cap) {
			return fmt.Errorf("invalid capability %q: must be one of %v", cap, ValidCapabilities)
		}
	}

	for name, cf := range m.Config {
		if err := validateConfigField(name, &cf); err != nil {
			return fmt.Errorf("config field %q: %w", name, err)
		}
	}

	return nil
}

func validateConfigField(name string, cf *ConfigField) error {
	if cf.Type == "" {
		return fmt.Errorf("type cannot be empty")
	}
	if !slices.Contains(ValidConfigTypes, cf.Type) {
		return fmt.Errorf("invalid type %q: must be one of %v", cf.Type, ValidConfigTypes)
	}

	if len(cf.Enum) > 0 && cf.Type != ConfigTypeString {
		return fmt.Errorf("enum is only allowed on string type, got %q", cf.Type)
	}

	if cf.Default != nil {
		if err := validateDefaultType(cf.Type, cf.Default); err != nil {
			return err
		}
	}

	return nil
}

func validateDefaultType(configType string, defaultValue any) error {
	switch configType {
	case ConfigTypeString:
		if _, ok := defaultValue.(string); !ok {
			return fmt.Errorf("default value type mismatch: expected string, got %T", defaultValue)
		}
	case ConfigTypeInteger:
		switch defaultValue.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float64:
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
