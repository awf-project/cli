package interpolation

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
)

// SerializeLoopItem converts a loop item value to a string representation.
// For primitive types (string, int, float, bool), returns the value as-is.
// For complex types (maps, slices, structs), marshals to JSON.
// Returns error if JSON marshaling fails (e.g., circular references, unsupported types).
//
// This function is used to ensure loop items in templates are properly serialized
// when passed to sub-workflows via call_workflow.
func SerializeLoopItem(item any) (string, error) {
	// Handle nil explicitly
	if item == nil {
		return "null", nil
	}

	// Type switch: primitives pass through, complex types get JSON marshaled
	switch v := item.(type) {
	case string:
		// Strings pass through unchanged (backward compatibility)
		return v, nil

	case bool:
		return fmt.Sprintf("%t", v), nil

	case float32:
		// Use strconv.FormatFloat for efficient formatting without scientific notation
		// 'f' format avoids scientific notation, -1 precision uses minimum needed digits
		return strconv.FormatFloat(float64(v), 'f', -1, 32), nil

	case float64:
		// Use strconv.FormatFloat for efficient formatting without scientific notation
		// 'f' format avoids scientific notation, -1 precision uses minimum needed digits
		return strconv.FormatFloat(v, 'f', -1, 64), nil

	default:
		// Use reflection to check if value is any integer type
		// This handles int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64
		rv := reflect.ValueOf(item)
		kind := rv.Kind()
		if kind >= reflect.Int && kind <= reflect.Int64 {
			// Signed integer types
			return fmt.Sprintf("%d", rv.Int()), nil
		}
		if kind >= reflect.Uint && kind <= reflect.Uint64 {
			// Unsigned integer types
			return fmt.Sprintf("%d", rv.Uint()), nil
		}

		// Complex types (maps, slices, structs): marshal to JSON
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			// Graceful degradation per ADR-004: return descriptive type name
			// instead of memory address (which is not useful for workflow inputs)
			// This handles unsupported types like channels, functions
			// Note: Callers should implement their own logging if needed
			typeName := reflect.TypeOf(v).String()
			return fmt.Sprintf("<unsupported:%s>", typeName), nil
		}
		return string(jsonBytes), nil
	}
}
