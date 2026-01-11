// Package validation provides input validation for workflow inputs.
package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Input defines an input parameter for validation.
// Mirrors workflow.Input to decouple pkg from internal/domain.
type Input struct {
	Name       string
	Type       string // "string", "integer", "boolean"
	Required   bool
	Validation *Rules
}

// Rules defines validation rules for an input.
type Rules struct {
	Pattern       string   // regex pattern for strings
	Enum          []string // allowed values
	Min           *int     // minimum for integers
	Max           *int     // maximum for integers
	FileExists    bool     // file must exist
	FileExtension []string // allowed file extensions
}

// ValidationError represents a collection of validation errors.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0]
	}
	return fmt.Sprintf("%d errors:\n  - %s", len(e.Errors), strings.Join(e.Errors, "\n  - "))
}

// ValidateInputs validates all inputs against their definitions.
// Returns aggregated errors (not fail-fast).
func ValidateInputs(inputs map[string]any, definitions []Input) error {
	if len(definitions) == 0 {
		return nil
	}

	var errs []string

	for _, def := range definitions {
		value, provided := inputs[def.Name]

		// check required
		if err := validateRequired(def.Name, value, def.Required); err != nil {
			if def.Required {
				errs = append(errs, err.Error())
			}
			// skip other validations if value not provided
			if !provided || value == nil {
				continue
			}
		}

		// skip validation if not provided and not required
		if !provided || value == nil {
			continue
		}

		// validate type and coerce if needed
		if def.Type != "" {
			coerced, err := validateType(def.Name, value, def.Type)
			if err != nil {
				errs = append(errs, err.Error())
				continue // skip other validations if type is invalid
			}
			value = coerced
		}

		// apply validation rules if defined
		if def.Validation != nil {
			valErrs := validateRules(def.Name, value, def.Type, def.Validation)
			errs = append(errs, valErrs...)
		}
	}

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

// validateRules applies validation rules to a value.
func validateRules(name string, value any, inputType string, rules *Rules) []string {
	var errs []string

	// pattern validation (for strings)
	if rules.Pattern != "" {
		if err := validatePatternWithType(name, value, rules.Pattern); err != nil {
			errs = append(errs, err.Error())
		}
	}

	// enum validation (for strings)
	if len(rules.Enum) > 0 {
		if err := validateEnumWithType(name, value, rules.Enum); err != nil {
			errs = append(errs, err.Error())
		}
	}

	// range validation (for integers)
	if rules.Min != nil || rules.Max != nil {
		if err := validateRangeWithType(name, value, rules.Min, rules.Max); err != nil {
			errs = append(errs, err.Error())
		}
	}

	// file validation (for strings representing paths)
	if rules.FileExists {
		if err := validateFileExistsWithType(name, value); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(rules.FileExtension) > 0 {
		if err := validateFileExtensionWithType(name, value, rules.FileExtension); err != nil {
			errs = append(errs, err.Error())
		}
	}

	return errs
}

// validatePatternWithType validates pattern rule with type checking.
// Returns nil if value is not a string (skips validation).
func validatePatternWithType(name string, value any, pattern string) error {
	// Skip validation if value is not a string
	strVal, ok := value.(string)
	if !ok {
		return nil
	}
	return validatePattern(name, strVal, pattern)
}

// validateEnumWithType validates enum rule with type checking.
// Returns nil if value is not a string (skips validation).
func validateEnumWithType(name string, value any, enum []string) error {
	// Skip validation if value is not a string
	strVal, ok := value.(string)
	if !ok {
		return nil
	}
	return validateEnum(name, strVal, enum)
}

// validateRangeWithType validates range rule with type checking.
// Returns nil if value is not an int (skips validation).
func validateRangeWithType(name string, value any, minVal, maxVal *int) error {
	// Skip validation if value is not an int
	intVal, ok := value.(int)
	if !ok {
		return nil
	}
	return validateRange(name, intVal, minVal, maxVal)
}

// validateFileExistsWithType validates file existence with type checking.
// Returns nil if value is not a string (skips validation).
func validateFileExistsWithType(name string, value any) error {
	// Skip validation if value is not a string
	strVal, ok := value.(string)
	if !ok {
		return nil
	}
	// Empty string is an error when value is explicitly provided as string
	if strVal == "" {
		return fmt.Errorf("inputs.%s: file path cannot be empty", name)
	}
	return validateFileExists(name, strVal)
}

// validateFileExtensionWithType validates file extension with type checking.
// Returns nil if value is not a string (skips validation).
func validateFileExtensionWithType(name string, value any, extensions []string) error {
	// Skip validation if value is not a string
	strVal, ok := value.(string)
	if !ok {
		return nil
	}

	// Handle empty extensions list - no validation needed
	if len(extensions) == 0 || strVal == "" {
		return nil
	}

	// Get file extension
	ext := filepath.Ext(strVal)
	if ext == "" {
		return fmt.Errorf("inputs.%s: file %q has no extension, allowed: %v", name, strVal, extensions)
	}

	// Case-sensitive comparison (unlike the atomic validator which is case-insensitive)
	for _, allowed := range extensions {
		if ext == allowed {
			return nil
		}
	}

	return fmt.Errorf("inputs.%s: file extension %q not in allowed extensions %v", name, ext, extensions)
}

// validateRequired checks if a required input is present and non-nil.
func validateRequired(name string, value any, required bool) error {
	if !required {
		return nil
	}

	if value == nil {
		return fmt.Errorf("inputs.%s: required but not provided", name)
	}

	// check for empty string
	if strVal, ok := value.(string); ok && strVal == "" {
		return fmt.Errorf("inputs.%s: required but empty", name)
	}

	return nil
}

// validateType checks if the input value matches the expected type.
// Returns the coerced value if type conversion is needed.
func validateType(name string, value any, expectedType string) (any, error) {
	if value == nil {
		return nil, fmt.Errorf("inputs.%s: cannot validate nil value as type %s", name, expectedType)
	}

	switch expectedType {
	case "string":
		return coerceToString(name, value)
	case "integer":
		return coerceToInt(name, value)
	case "boolean":
		return coerceToBool(name, value)
	case "":
		// empty type accepts any value
		return value, nil
	default:
		return nil, fmt.Errorf("inputs.%s: unknown type %q", name, expectedType)
	}
}

// coerceToString converts a value to string.
func coerceToString(name string, value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case int:
		return strconv.Itoa(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

// coerceToInt converts a value to int.
func coerceToInt(name string, value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		i, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("inputs.%s: cannot convert %q to integer", name, v)
		}
		return i, nil
	default:
		return 0, fmt.Errorf("inputs.%s: cannot convert %T to integer", name, value)
	}
}

// coerceToBool converts a value to bool.
func coerceToBool(name string, value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		lower := strings.ToLower(v)
		switch lower {
		case "true", "yes", "1":
			return true, nil
		case "false", "no", "0":
			return false, nil
		default:
			return false, fmt.Errorf("inputs.%s: cannot convert %q to boolean", name, v)
		}
	default:
		return false, fmt.Errorf("inputs.%s: cannot convert %T to boolean", name, value)
	}
}

// validatePattern checks if a string value matches the regex pattern.
func validatePattern(name, value, pattern string) error {
	if pattern == "" {
		return nil
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("inputs.%s: invalid regex pattern %q: %w", name, pattern, err)
	}

	if !re.MatchString(value) {
		return fmt.Errorf("inputs.%s: value %q does not match pattern %q", name, value, pattern)
	}

	return nil
}

// validateEnum checks if a string value is in the allowed list.
func validateEnum(name, value string, allowed []string) error {
	if len(allowed) == 0 {
		return nil
	}

	for _, a := range allowed {
		if value == a {
			return nil
		}
	}

	return fmt.Errorf("inputs.%s: value %q not in allowed values %v", name, value, allowed)
}

// validateRange checks if an integer value is within min/max bounds.
func validateRange(name string, value int, minVal, maxVal *int) error {
	if minVal != nil && value < *minVal {
		return fmt.Errorf("inputs.%s: value %d is below minimum %d", name, value, *minVal)
	}

	if maxVal != nil && value > *maxVal {
		return fmt.Errorf("inputs.%s: value %d exceeds maximum %d", name, value, *maxVal)
	}

	return nil
}

// validateFileExists checks if the file at the given path exists.
// Empty paths are skipped (valid for optional file inputs).
func validateFileExists(name, path string) error {
	if path == "" {
		return nil // skip validation for empty optional paths
	}

	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("inputs.%s: file does not exist: %s", name, path)
		}
		return fmt.Errorf("inputs.%s: cannot access file: %s: %w", name, path, err)
	}

	return nil
}

// validateFileExtension checks if the file has an allowed extension.
// Empty paths are skipped (valid for optional file inputs).
func validateFileExtension(name, path string, allowed []string) error {
	if len(allowed) == 0 || path == "" {
		return nil
	}

	ext := filepath.Ext(path)
	if ext == "" {
		return fmt.Errorf("inputs.%s: file %q has no extension, allowed: %v", name, path, allowed)
	}

	extLower := strings.ToLower(ext)
	for _, a := range allowed {
		if strings.EqualFold(extLower, a) {
			return nil
		}
	}

	return fmt.Errorf("inputs.%s: file extension %q not in allowed extensions %v", name, ext, allowed)
}

// Ensure ValidationError implements error interface.
var _ error = (*ValidationError)(nil)
