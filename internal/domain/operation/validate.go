package operation

import (
	"fmt"
	"net/mail"
	"net/url"
	"sort"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
)

// ValidateInputs validates runtime inputs against an operation schema.
//
// This function performs the following validations:
// - Required fields: Checks that all required inputs are provided and non-empty
// - Type correctness: Validates that each input matches its declared type (string, integer, boolean, array, object)
// - Default values: Applies default values for optional inputs that are not provided
// - Type coercion: Handles JSON float64-to-int coercion for integer types
// - Validation rules: Applies validation rules (e.g., "url", "email") when the Validation field is set
//
// The inputs map is modified in-place to apply default values for missing optional parameters.
//
// Returns ErrInvalidInputs if validation fails, with details in the error message.
func ValidateInputs(schema *pluginmodel.OperationSchema, inputs map[string]any) error {
	if schema == nil {
		return fmt.Errorf("%w: schema cannot be nil", ErrInvalidInputs)
	}

	if inputs == nil {
		return fmt.Errorf("%w: inputs map cannot be nil", ErrInvalidInputs)
	}

	// Collect all errors instead of failing fast
	var errors []string

	// Get sorted list of input names for deterministic iteration
	inputNames := getSortedInputNames(schema.Inputs)

	// Validate each input
	for _, inputName := range inputNames {
		inputSchema := schema.Inputs[inputName]
		value, exists := inputs[inputName]

		if err := validateInput(inputName, inputSchema, value, exists, inputs); err != nil {
			errors = append(errors, err...)
		}
	}

	return formatValidationErrors(errors)
}

// getSortedInputNames returns a sorted list of input names for deterministic iteration
func getSortedInputNames(inputs map[string]pluginmodel.InputSchema) []string {
	names := make([]string, 0, len(inputs))
	for name := range inputs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// validateInput validates a single input field and returns any errors
func validateInput(name string, schema pluginmodel.InputSchema, value any, exists bool, inputs map[string]any) []string {
	var errors []string

	// Check required fields
	if schema.Required {
		if !exists || value == nil || (value == "" && schema.Type == pluginmodel.InputTypeString) {
			return []string{fmt.Sprintf("required field %q is missing or empty", name)}
		}
	}

	// Apply default values for missing optional fields
	if !exists || value == nil {
		if schema.Default != nil {
			inputs[name] = schema.Default
			value = schema.Default
		} else {
			return nil // No value and no default, skip validation
		}
	}

	// Validate type correctness
	if err := validateType(name, value, schema.Type); err != nil {
		return []string{err.Error()}
	}

	// Apply validation rules if set
	if schema.Validation != "" {
		if err := validateRule(name, value, schema.Validation); err != nil {
			errors = append(errors, err.Error())
		}
	}

	return errors
}

// formatValidationErrors formats multiple validation errors into a single error message
func formatValidationErrors(errors []string) error {
	if len(errors) == 0 {
		return nil
	}

	if len(errors) == 1 {
		return fmt.Errorf("%w: %s", ErrInvalidInputs, errors[0])
	}

	// Multiple errors: join with semicolons
	var errMsg string
	for i, e := range errors {
		if i == 0 {
			errMsg = e
		} else {
			errMsg += "; " + e
		}
	}
	return fmt.Errorf("%w: %s", ErrInvalidInputs, errMsg)
}

// validateType checks if the value matches the expected type, handling JSON float64-to-int coercion
func validateType(name string, value any, expectedType string) error {
	switch expectedType {
	case pluginmodel.InputTypeString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field %q: expected string, got %T", name, value)
		}

	case pluginmodel.InputTypeInteger:
		switch v := value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			// Valid integer types
		case float64:
			// JSON unmarshaling produces float64 for all numbers
			// Allow float64 if it represents a whole number
			if v != float64(int64(v)) {
				return fmt.Errorf("field %q: float64 value %v has fractional part, cannot coerce to integer", name, v)
			}
		default:
			return fmt.Errorf("field %q: expected integer, got %T", name, value)
		}

	case pluginmodel.InputTypeBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("field %q: expected boolean, got %T", name, value)
		}

	case pluginmodel.InputTypeArray:
		// Check if value is a slice type
		switch value.(type) {
		case []any, []string, []int, []bool:
			// Valid array types
		default:
			return fmt.Errorf("field %q: expected array, got %T", name, value)
		}

	case pluginmodel.InputTypeObject:
		// Check if value is a map type
		switch value.(type) {
		case map[string]any, map[string]string:
			// Valid object types
		default:
			return fmt.Errorf("field %q: expected object, got %T", name, value)
		}

	default:
		return fmt.Errorf("field %q: unknown type %q", name, expectedType)
	}

	return nil
}

// validateRule applies validation rules like "url" and "email"
func validateRule(name string, value any, rule string) error {
	// Validation rules only apply to string values
	strValue, ok := value.(string)
	if !ok {
		// Skip validation for non-string types
		return nil
	}

	switch rule {
	case "url":
		if _, err := url.ParseRequestURI(strValue); err != nil {
			return fmt.Errorf("field %q: invalid URL %q", name, strValue)
		}
		// Also check that it has a valid scheme
		u, err := url.Parse(strValue)
		if err != nil {
			return fmt.Errorf("field %q: invalid URL %q", name, strValue)
		}
		if u.Scheme == "" {
			return fmt.Errorf("field %q: URL %q missing scheme", name, strValue)
		}

	case "email":
		if _, err := mail.ParseAddress(strValue); err != nil {
			return fmt.Errorf("field %q: invalid email %q", name, strValue)
		}

	default:
		// Unknown validation rules are ignored (schema validation should catch this earlier)
		return nil
	}

	return nil
}
