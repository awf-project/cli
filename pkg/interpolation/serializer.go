package interpolation

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
)

// SerializeLoopItem converts a loop item value to a string representation.
// Primitives (string, int, float, bool) return as-is.
// Complex types (maps, slices, structs) marshal to JSON.
func SerializeLoopItem(item any) (string, error) {
	if item == nil {
		return "null", nil
	}

	switch v := item.(type) {
	case string:
		return v, nil

	case bool:
		return fmt.Sprintf("%t", v), nil

	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32), nil

	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil

	default:
		rv := reflect.ValueOf(item)
		kind := rv.Kind()
		if kind >= reflect.Int && kind <= reflect.Int64 {
			return fmt.Sprintf("%d", rv.Int()), nil
		}
		if kind >= reflect.Uint && kind <= reflect.Uint64 {
			return fmt.Sprintf("%d", rv.Uint()), nil
		}

		jsonBytes, err := json.Marshal(v)
		if err != nil {
			typeName := reflect.TypeOf(v).String()
			return fmt.Sprintf("<unsupported:%s>", typeName), nil
		}
		return string(jsonBytes), nil
	}
}
