package plugin

import (
	"errors"
	"fmt"
	"slices"
)

// Valid input types for operation parameters.
const (
	InputTypeString  = "string"
	InputTypeInteger = "integer"
	InputTypeBoolean = "boolean"
	InputTypeArray   = "array"
	InputTypeObject  = "object"
)

// ValidInputTypes lists all recognized input types.
var ValidInputTypes = []string{
	InputTypeString,
	InputTypeInteger,
	InputTypeBoolean,
	InputTypeArray,
	InputTypeObject,
}

// ValidValidationRules lists all recognized validation rules for input parameters.
var ValidValidationRules = []string{
	"url",
	"email",
}

// OperationSchema defines a plugin-provided operation.
type OperationSchema struct {
	Name        string                 // Operation name (e.g., "slack.send")
	Description string                 // Human-readable description
	Inputs      map[string]InputSchema // Input parameters
	Outputs     []string               // Output field names
	PluginName  string                 // Owning plugin name
}

// Validate checks if the operation schema is valid.
func (o *OperationSchema) Validate() error {
	// Validate Name field - must be non-empty
	if o.Name == "" {
		return errors.New("operation name cannot be empty")
	}

	// Check if Name contains only whitespace
	trimmedName := ""
	for _, r := range o.Name {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			trimmedName = o.Name
			break
		}
	}
	if trimmedName == "" {
		return errors.New("operation name cannot be empty")
	}

	// Validate PluginName field - must be non-empty
	if o.PluginName == "" {
		return errors.New("plugin name cannot be empty")
	}

	// Check if PluginName contains only whitespace
	trimmedPluginName := ""
	for _, r := range o.PluginName {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			trimmedPluginName = o.PluginName
			break
		}
	}
	if trimmedPluginName == "" {
		return errors.New("plugin name cannot be empty")
	}

	// Validate all input schemas
	for name, inputSchema := range o.Inputs {
		if err := inputSchema.Validate(); err != nil {
			return fmt.Errorf("invalid input schema for %q: %w", name, err)
		}
	}

	// Check for duplicate outputs and empty strings
	if len(o.Outputs) > 0 {
		seen := make(map[string]bool)
		for _, output := range o.Outputs {
			if output == "" {
				return errors.New("output name cannot be empty")
			}
			if seen[output] {
				return fmt.Errorf("duplicate output name: %q", output)
			}
			seen[output] = true
		}
	}

	return nil
}

// GetRequiredInputs returns a list of required input parameter names.
func (o *OperationSchema) GetRequiredInputs() []string {
	// Initialize empty slice to ensure we never return nil
	result := []string{}

	// Iterate through inputs and collect required ones
	for name, schema := range o.Inputs {
		if schema.Required {
			result = append(result, name)
		}
	}

	return result
}

// InputSchema defines an input parameter for an operation.
type InputSchema struct {
	Type        string // "string", "integer", "boolean", "array", "object"
	Required    bool
	Default     any
	Description string
	Validation  string // Optional validation rule (e.g., "url", "email")
}

// Validate checks if the input schema is valid.
func (i *InputSchema) Validate() error {
	// Check if type is empty
	if i.Type == "" {
		return errors.New("input schema type cannot be empty")
	}

	// Check if type is valid
	if !i.IsValidType() {
		return fmt.Errorf("invalid input schema type %q: must be one of %v", i.Type, ValidInputTypes)
	}

	// Check validation rule if present
	if i.Validation != "" && !slices.Contains(ValidValidationRules, i.Validation) {
		return fmt.Errorf("invalid validation rule %q: must be one of %v", i.Validation, ValidValidationRules)
	}

	// Check default value type matches declared type if default is provided
	if i.Default != nil {
		if err := i.validateDefaultType(); err != nil {
			return err
		}
	}

	return nil
}

// validateDefaultType checks if the default value type matches the declared input type.
func (i *InputSchema) validateDefaultType() error {
	switch i.Type {
	case InputTypeString:
		if _, ok := i.Default.(string); !ok {
			return fmt.Errorf("default value type mismatch: expected string, got %T", i.Default)
		}
	case InputTypeInteger:
		// JSON decoding may produce float64 for integer types, so accept both int and float64
		switch i.Default.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float64:
			// Valid integer types
		default:
			return fmt.Errorf("default value type mismatch: expected integer or float64, got %T", i.Default)
		}
	case InputTypeBoolean:
		if _, ok := i.Default.(bool); !ok {
			return fmt.Errorf("default value type mismatch: expected bool, got %T", i.Default)
		}
	case InputTypeArray:
		// Check for slice type (any element type)
		switch i.Default.(type) {
		case []any, []string, []int, []bool:
			// Valid slice types
		default:
			return fmt.Errorf("default value type mismatch: expected slice, got %T", i.Default)
		}
	case InputTypeObject:
		// Check for map type (any key/value type)
		switch i.Default.(type) {
		case map[string]any, map[string]string:
			// Valid map types
		default:
			return fmt.Errorf("default value type mismatch: expected map, got %T", i.Default)
		}
	}
	return nil
}

// IsValidType checks if the input type is a recognized type.
func (i *InputSchema) IsValidType() bool {
	return slices.Contains(ValidInputTypes, i.Type)
}

// OperationResult holds the result of executing a plugin operation.
type OperationResult struct {
	Success bool
	Outputs map[string]any
	Error   string
}

// IsSuccess returns true if the operation completed successfully.
func (r *OperationResult) IsSuccess() bool {
	return r.Success
}

// HasError returns true if the operation has an error message.
func (r *OperationResult) HasError() bool {
	return r.Error != ""
}

// GetOutput retrieves a specific output value by key.
func (r *OperationResult) GetOutput(key string) (any, bool) {
	if r.Outputs == nil {
		return nil, false
	}
	val, ok := r.Outputs[key]
	return val, ok
}
