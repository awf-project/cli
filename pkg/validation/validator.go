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

		if err := validateRequired(def.Name, value, def.Required); err != nil {
			if def.Required {
				errs = append(errs, err.Error())
			}
			if !provided || value == nil {
				continue
			}
		}

		if !provided || value == nil {
			continue
		}

		if def.Type != "" {
			coerced, err := validateType(def.Name, value, def.Type)
			if err != nil {
				errs = append(errs, err.Error())
				continue
			}
			value = coerced
		}

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

func validateRules(name string, value any, inputType string, rules *Rules) []string {
	var errs []string

	if rules.Pattern != "" {
		if err := validatePatternWithType(name, value, rules.Pattern); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(rules.Enum) > 0 {
		if err := validateEnumWithType(name, value, rules.Enum); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if rules.Min != nil || rules.Max != nil {
		if err := validateRangeWithType(name, value, rules.Min, rules.Max); err != nil {
			errs = append(errs, err.Error())
		}
	}

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

func validatePatternWithType(name string, value any, pattern string) error {
	strVal, ok := value.(string)
	if !ok {
		return nil
	}
	return validatePattern(name, strVal, pattern)
}

func validateEnumWithType(name string, value any, enum []string) error {
	strVal, ok := value.(string)
	if !ok {
		return nil
	}
	return validateEnum(name, strVal, enum)
}

func validateRangeWithType(name string, value any, minVal, maxVal *int) error {
	intVal, ok := value.(int)
	if !ok {
		return nil
	}
	return validateRange(name, intVal, minVal, maxVal)
}

func validateFileExistsWithType(name string, value any) error {
	strVal, ok := value.(string)
	if !ok {
		return nil
	}
	if strVal == "" {
		return fmt.Errorf("inputs.%s: file path cannot be empty", name)
	}
	return validateFileExists(name, strVal)
}

func validateFileExtensionWithType(name string, value any, extensions []string) error {
	strVal, ok := value.(string)
	if !ok {
		return nil
	}

	if len(extensions) == 0 || strVal == "" {
		return nil
	}

	ext := filepath.Ext(strVal)
	if ext == "" {
		return fmt.Errorf("inputs.%s: file %q has no extension, allowed: %v", name, strVal, extensions)
	}

	for _, allowed := range extensions {
		if ext == allowed {
			return nil
		}
	}

	return fmt.Errorf("inputs.%s: file extension %q not in allowed extensions %v", name, ext, extensions)
}

func validateRequired(name string, value any, required bool) error {
	if !required {
		return nil
	}

	if value == nil {
		return fmt.Errorf("inputs.%s: required but not provided", name)
	}

	if strVal, ok := value.(string); ok && strVal == "" {
		return fmt.Errorf("inputs.%s: required but empty", name)
	}

	return nil
}

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
		return value, nil
	default:
		return nil, fmt.Errorf("inputs.%s: unknown type %q", name, expectedType)
	}
}

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

func validateRange(name string, value int, minVal, maxVal *int) error {
	if minVal != nil && value < *minVal {
		return fmt.Errorf("inputs.%s: value %d is below minimum %d", name, value, *minVal)
	}

	if maxVal != nil && value > *maxVal {
		return fmt.Errorf("inputs.%s: value %d exceeds maximum %d", name, value, *maxVal)
	}

	return nil
}

func validateFileExists(name, path string) error {
	if path == "" {
		return nil
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

var _ error = (*ValidationError)(nil)
