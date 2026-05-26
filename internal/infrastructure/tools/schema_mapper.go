package tools

import (
	"errors"
	"fmt"
	"slices"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
)

var (
	ErrUnknownOperation  = errors.New("unknown operation")
	ErrUnsupportedSchema = errors.New("unsupported schema type")
	ErrNilSchema         = errors.New("nil operation schema")
)

// MapOperationSchema converts a pluginmodel.OperationSchema to a JSON Schema document
// of shape {"type": "object", "properties": {...}, "required": [...]}.
// Supported primitive types: string, integer, boolean.
// Returns ErrUnsupportedSchema (wrapped) for array/object — no nested property schemas exist.
// Returns ErrNilSchema (wrapped) when s is nil — surfaced as an explicit error rather
// than a panic per the project rule "never panic on nil input in public infrastructure".
// The Validation field maps to JSON Schema format: "url" → "uri", "email" → "email".
// Accepts a pointer to avoid copying the 80-byte OperationSchema on every call; the function
// does not mutate s.
func MapOperationSchema(s *pluginmodel.OperationSchema) (map[string]any, error) {
	if s == nil {
		return nil, fmt.Errorf("MapOperationSchema: %w", ErrNilSchema)
	}
	properties := make(map[string]any, len(s.Inputs))
	required := make([]string, 0, len(s.Inputs))

	for name, input := range s.Inputs {
		if input.Type == pluginmodel.InputTypeArray || input.Type == pluginmodel.InputTypeObject {
			return nil, fmt.Errorf("%s: %w", name, ErrUnsupportedSchema)
		}

		prop := map[string]any{
			"type": input.Type,
		}

		if input.Description != "" {
			prop["description"] = input.Description
		}

		if input.Default != nil {
			prop["default"] = input.Default
		}

		switch input.Validation {
		case "url":
			prop["format"] = "uri"
		case "email":
			prop["format"] = "email"
		}

		properties[name] = prop

		if input.Required {
			required = append(required, name)
		}
	}

	// Sort required fields for deterministic output. Iterating over s.Inputs (a map)
	// yields keys in non-deterministic order; agents comparing tools/list responses
	// across calls would see spurious diffs without this sort.
	slices.Sort(required)

	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}, nil
}
