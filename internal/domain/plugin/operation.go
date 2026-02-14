package plugin

import (
	"errors"
	"fmt"
	"slices"
)

const (
	InputTypeString  = "string"
	InputTypeInteger = "integer"
	InputTypeBoolean = "boolean"
	InputTypeArray   = "array"
	InputTypeObject  = "object"
)

var ValidInputTypes = []string{
	InputTypeString,
	InputTypeInteger,
	InputTypeBoolean,
	InputTypeArray,
	InputTypeObject,
}

var ValidValidationRules = []string{
	"url",
	"email",
}

type OperationSchema struct {
	Name        string
	Description string
	Inputs      map[string]InputSchema
	Outputs     []string
	PluginName  string
}

func (o *OperationSchema) Validate() error {
	if o.Name == "" {
		return errors.New("operation name cannot be empty")
	}

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

	if o.PluginName == "" {
		return errors.New("plugin name cannot be empty")
	}

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

	for name, inputSchema := range o.Inputs {
		if err := inputSchema.Validate(); err != nil {
			return fmt.Errorf("invalid input schema for %q: %w", name, err)
		}
	}

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

func (o *OperationSchema) GetRequiredInputs() []string {
	result := []string{}

	for name, schema := range o.Inputs {
		if schema.Required {
			result = append(result, name)
		}
	}

	return result
}

type InputSchema struct {
	Type        string
	Required    bool
	Default     any
	Description string
	Validation  string
}

func (i *InputSchema) Validate() error {
	if i.Type == "" {
		return errors.New("input schema type cannot be empty")
	}

	if !i.IsValidType() {
		return fmt.Errorf("invalid input schema type %q: must be one of %v", i.Type, ValidInputTypes)
	}

	if i.Validation != "" && !slices.Contains(ValidValidationRules, i.Validation) {
		return fmt.Errorf("invalid validation rule %q: must be one of %v", i.Validation, ValidValidationRules)
	}

	if i.Default != nil {
		if err := i.validateDefaultType(); err != nil {
			return err
		}
	}

	return nil
}

func (i *InputSchema) validateDefaultType() error {
	switch i.Type {
	case InputTypeString:
		if _, ok := i.Default.(string); !ok {
			return fmt.Errorf("default value type mismatch: expected string, got %T", i.Default)
		}
	case InputTypeInteger:
		switch i.Default.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float64:
		default:
			return fmt.Errorf("default value type mismatch: expected integer or float64, got %T", i.Default)
		}
	case InputTypeBoolean:
		if _, ok := i.Default.(bool); !ok {
			return fmt.Errorf("default value type mismatch: expected bool, got %T", i.Default)
		}
	case InputTypeArray:
		switch i.Default.(type) {
		case []any, []string, []int, []bool:
		default:
			return fmt.Errorf("default value type mismatch: expected slice, got %T", i.Default)
		}
	case InputTypeObject:
		switch i.Default.(type) {
		case map[string]any, map[string]string:
		default:
			return fmt.Errorf("default value type mismatch: expected map, got %T", i.Default)
		}
	}
	return nil
}

func (i *InputSchema) IsValidType() bool {
	return slices.Contains(ValidInputTypes, i.Type)
}

type OperationResult struct {
	Success bool
	Outputs map[string]any
	Error   string
}

func (r *OperationResult) IsSuccess() bool {
	return r.Success
}

func (r *OperationResult) HasError() bool {
	return r.Error != ""
}

func (r *OperationResult) GetOutput(key string) (any, bool) {
	if r.Outputs == nil {
		return nil, false
	}
	val, ok := r.Outputs[key]
	return val, ok
}
