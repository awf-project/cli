package interpolation_test

import (
	"bytes"
	"encoding/json"
	"log"
	"testing"

	"github.com/awf-project/awf/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Item: T001
// Feature: F047

// TestSerializeLoopItem_Primitives verifies that primitive types pass through unchanged.
// These are the common scalar types that don't need JSON marshaling.
func TestSerializeLoopItem_Primitives(t *testing.T) {
	tests := []struct {
		name string
		item any
		want string
	}{
		{
			name: "string unchanged",
			item: "hello",
			want: "hello",
		},
		{
			name: "empty string",
			item: "",
			want: "",
		},
		{
			name: "string with spaces",
			item: "hello world",
			want: "hello world",
		},
		{
			name: "string with special characters",
			item: "hello@world.com",
			want: "hello@world.com",
		},
		{
			name: "integer",
			item: 42,
			want: "42",
		},
		{
			name: "integer zero",
			item: 0,
			want: "0",
		},
		{
			name: "negative integer",
			item: -100,
			want: "-100",
		},
		{
			name: "float64",
			item: 3.14,
			want: "3.14",
		},
		{
			name: "float64 zero",
			item: 0.0,
			want: "0",
		},
		{
			name: "negative float",
			item: -2.5,
			want: "-2.5",
		},
		{
			name: "bool true",
			item: true,
			want: "true",
		},
		{
			name: "bool false",
			item: false,
			want: "false",
		},
		{
			name: "nil",
			item: nil,
			want: "null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := interpolation.SerializeLoopItem(tt.item)
			require.NoError(t, err, "SerializeLoopItem should not error for primitive types")
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestSerializeLoopItem_Maps verifies that map[string]any gets marshaled to JSON objects.
// This is the primary use case from F047 bug report.
func TestSerializeLoopItem_Maps(t *testing.T) {
	tests := []struct {
		name string
		item any
		want string
	}{
		{
			name: "simple map",
			item: map[string]any{"name": "S1"},
			want: `{"name":"S1"}`,
		},
		{
			name: "map with multiple fields",
			item: map[string]any{
				"name": "S1",
				"type": "fix",
			},
			want: `{"name":"S1","type":"fix"}`,
		},
		{
			name: "map with mixed types",
			item: map[string]any{
				"name":  "S1",
				"count": 42,
				"valid": true,
			},
			want: `{"count":42,"name":"S1","valid":true}`,
		},
		{
			name: "empty map",
			item: map[string]any{},
			want: `{}`,
		},
		{
			name: "map with nil value",
			item: map[string]any{"value": nil},
			want: `{"value":null}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := interpolation.SerializeLoopItem(tt.item)
			require.NoError(t, err, "SerializeLoopItem should not error for maps")

			// Parse both as JSON to compare structure (order-independent)
			var gotJSON, wantJSON any
			require.NoError(t, json.Unmarshal([]byte(got), &gotJSON))
			require.NoError(t, json.Unmarshal([]byte(tt.want), &wantJSON))
			assert.Equal(t, wantJSON, gotJSON)
		})
	}
}

// TestSerializeLoopItem_Slices verifies that slices get marshaled to JSON arrays.
func TestSerializeLoopItem_Slices(t *testing.T) {
	tests := []struct {
		name string
		item any
		want string
	}{
		{
			name: "slice of strings",
			item: []string{"a", "b", "c"},
			want: `["a","b","c"]`,
		},
		{
			name: "slice of integers",
			item: []int{1, 2, 3},
			want: `[1,2,3]`,
		},
		{
			name: "slice of any",
			item: []any{"hello", 42, true},
			want: `["hello",42,true]`,
		},
		{
			name: "empty slice",
			item: []any{},
			want: `[]`,
		},
		{
			name: "slice with nil",
			item: []any{nil},
			want: `[null]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := interpolation.SerializeLoopItem(tt.item)
			require.NoError(t, err, "SerializeLoopItem should not error for slices")
			assert.JSONEq(t, tt.want, got)
		})
	}
}

// TestSerializeLoopItem_Nested verifies deeply nested structures serialize correctly.
// This tests the exact use case from F047: objects with nested arrays.
func TestSerializeLoopItem_Nested(t *testing.T) {
	tests := []struct {
		name string
		item any
		want string
	}{
		{
			name: "map with slice value (F047 exact case)",
			item: map[string]any{
				"name":  "S1",
				"type":  "fix",
				"files": []string{"a.go"},
			},
			want: `{"name":"S1","type":"fix","files":["a.go"]}`,
		},
		{
			name: "slice of maps",
			item: []any{
				map[string]any{"name": "S1"},
				map[string]any{"name": "S2"},
			},
			want: `[{"name":"S1"},{"name":"S2"}]`,
		},
		{
			name: "deeply nested map",
			item: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"value": "deep",
					},
				},
			},
			want: `{"level1":{"level2":{"value":"deep"}}}`,
		},
		{
			name: "deeply nested slice",
			item: []any{
				[]any{
					[]any{1, 2},
				},
			},
			want: `[[[1,2]]]`,
		},
		{
			name: "complex mixed nesting",
			item: map[string]any{
				"items": []any{
					map[string]any{
						"name":  "item1",
						"tags":  []string{"a", "b"},
						"count": 5,
					},
					map[string]any{
						"name":  "item2",
						"tags":  []string{"c"},
						"count": 3,
					},
				},
			},
			want: `{"items":[{"name":"item1","tags":["a","b"],"count":5},{"name":"item2","tags":["c"],"count":3}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := interpolation.SerializeLoopItem(tt.item)
			require.NoError(t, err, "SerializeLoopItem should not error for nested structures")
			assert.JSONEq(t, tt.want, got)
		})
	}
}

// TestSerializeLoopItem_Structs verifies that custom structs serialize to JSON.
func TestSerializeLoopItem_Structs(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	type Task struct {
		Title     string   `json:"title"`
		Completed bool     `json:"completed"`
		Tags      []string `json:"tags,omitempty"`
	}

	tests := []struct {
		name string
		item any
		want string
	}{
		{
			name: "simple struct",
			item: Person{Name: "Alice", Age: 30},
			want: `{"name":"Alice","age":30}`,
		},
		{
			name: "struct with slice",
			item: Task{
				Title:     "Fix bug",
				Completed: true,
				Tags:      []string{"urgent", "backend"},
			},
			want: `{"title":"Fix bug","completed":true,"tags":["urgent","backend"]}`,
		},
		{
			name: "struct with omitempty",
			item: Task{
				Title:     "Review PR",
				Completed: false,
			},
			want: `{"title":"Review PR","completed":false}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := interpolation.SerializeLoopItem(tt.item)
			require.NoError(t, err, "SerializeLoopItem should not error for structs")
			assert.JSONEq(t, tt.want, got)
		})
	}
}

// TestSerializeLoopItem_Unicode verifies special characters and unicode handle correctly.
func TestSerializeLoopItem_Unicode(t *testing.T) {
	tests := []struct {
		name string
		item any
		want string
	}{
		{
			name: "map with unicode",
			item: map[string]any{"greeting": "hello 世界"},
			want: `{"greeting":"hello 世界"}`,
		},
		{
			name: "map with emoji",
			item: map[string]any{"status": "✅ done"},
			want: `{"status":"✅ done"}`,
		},
		{
			name: "map with special chars",
			item: map[string]any{
				"path":    "/tmp/file.txt",
				"command": "echo 'hello'",
			},
			want: `{"path":"/tmp/file.txt","command":"echo 'hello'"}`,
		},
		{
			name: "map with newlines and tabs",
			item: map[string]any{"text": "line1\nline2\ttabbed"},
			want: `{"text":"line1\nline2\ttabbed"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := interpolation.SerializeLoopItem(tt.item)
			require.NoError(t, err, "SerializeLoopItem should not error for unicode")
			assert.JSONEq(t, tt.want, got)
		})
	}
}

// TestSerializeLoopItem_EdgeCases verifies boundary conditions and special values.
func TestSerializeLoopItem_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		item any
		want string
	}{
		{
			name: "zero-value map",
			item: map[string]any{},
			want: `{}`,
		},
		{
			name: "zero-value slice",
			item: []any{},
			want: `[]`,
		},
		{
			name: "nil slice",
			item: []any(nil),
			want: `null`,
		},
		{
			name: "nil map",
			item: map[string]any(nil),
			want: `null`,
		},
		{
			name: "map with all zero values",
			item: map[string]any{
				"str":   "",
				"num":   0,
				"bool":  false,
				"null":  nil,
				"slice": []any{},
				"map":   map[string]any{},
			},
			want: `{"str":"","num":0,"bool":false,"null":null,"slice":[],"map":{}}`,
		},
		{
			name: "very large integer",
			item: 9223372036854775807, // max int64
			want: "9223372036854775807",
		},
		{
			name: "very small integer",
			item: -9223372036854775808, // min int64
			want: "-9223372036854775808",
		},
		{
			name: "scientific notation float",
			item: 1.23e10,
			want: "12300000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := interpolation.SerializeLoopItem(tt.item)
			require.NoError(t, err, "SerializeLoopItem should not error for edge cases")

			// For JSON values, use JSONEq; for primitives, use string equality
			if tt.want[0] == '{' || tt.want[0] == '[' || tt.want == "null" {
				assert.JSONEq(t, tt.want, got)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestSerializeLoopItem_JSONUnmarshalled tests the exact output from loop_executor.ParseItems().
// This reproduces the F047 bug scenario: JSON unmarshal -> SerializeLoopItem -> should get original JSON.
func TestSerializeLoopItem_JSONUnmarshalled(t *testing.T) {
	tests := []struct {
		name     string
		jsonIn   string // Original JSON from ParseItems
		wantJSON string // Expected JSON output
	}{
		{
			name:     "F047 reproduction case",
			jsonIn:   `[{"name":"S1","type":"fix","files":["a.go"]}]`,
			wantJSON: `{"name":"S1","type":"fix","files":["a.go"]}`,
		},
		{
			name:     "multiple items array",
			jsonIn:   `[{"name":"S1"},{"name":"S2"}]`,
			wantJSON: `{"name":"S1"}`, // First item
		},
		{
			name:     "complex nested structure",
			jsonIn:   `[{"data":{"nested":{"value":42}}}]`,
			wantJSON: `{"data":{"nested":{"value":42}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate ParseItems behavior: unmarshal JSON array
			var items []any
			err := json.Unmarshal([]byte(tt.jsonIn), &items)
			require.NoError(t, err, "Test setup: unmarshal JSON")
			require.NotEmpty(t, items, "Test setup: array should have items")

			// Take first item (simulating loop iteration)
			item := items[0]

			// This is the call that was broken in F047
			got, err := interpolation.SerializeLoopItem(item)
			require.NoError(t, err, "SerializeLoopItem should not error")

			// Verify we get valid JSON back, not "map[...]" format
			assert.JSONEq(t, tt.wantJSON, got, "Should produce valid JSON, not Go map format")
		})
	}
}

// TestSerializeLoopItem_Errors verifies error handling for unsupported types.
// Per ADR-004, we should gracefully degrade rather than panic.
func TestSerializeLoopItem_Errors(t *testing.T) {
	tests := []struct {
		name      string
		item      any
		wantErr   bool
		wantValue string // Expected fallback value if applicable
	}{
		{
			name:    "channel type (unsupported by json.Marshal)",
			item:    make(chan int),
			wantErr: false, // Graceful degradation per ADR-004
		},
		{
			name:    "function type (unsupported by json.Marshal)",
			item:    func() {},
			wantErr: false, // Graceful degradation per ADR-004
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := interpolation.SerializeLoopItem(tt.item)

			if tt.wantErr {
				assert.Error(t, err, "Should return error for unsupported types")
			} else {
				// Per ADR-004: graceful degradation
				assert.NoError(t, err, "Should not error (graceful degradation)")
				assert.NotEmpty(t, got, "Should return non-empty fallback value")
			}
		})
	}
}

// TestSerializeLoopItem_BackwardCompatibility ensures existing string-based loops still work.
// This is critical: primitive types must pass through unchanged to avoid breaking existing workflows.
func TestSerializeLoopItem_BackwardCompatibility(t *testing.T) {
	tests := []struct {
		name string
		item any
		want string
	}{
		{
			name: "string item unchanged",
			item: "simple-string",
			want: "simple-string",
		},
		{
			name: "string with JSON-like content unchanged",
			item: `{"already":"json"}`,
			want: `{"already":"json"}`,
		},
		{
			name: "numeric string unchanged",
			item: "123",
			want: "123",
		},
		{
			name: "boolean string unchanged",
			item: "true",
			want: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := interpolation.SerializeLoopItem(tt.item)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got, "Primitive strings must pass through unchanged")
		})
	}
}

// Item: T002
// Feature: F047

// TestSerializeLoopItem_ErrorHandling_GracefulDegradation verifies that when json.Marshal fails,
// the function falls back gracefully without logging (per PR-65 Component S1: avoid bypassing structured logging).
func TestSerializeLoopItem_ErrorHandling_GracefulDegradation(t *testing.T) {
	tests := []struct {
		name string
		item any
	}{
		{
			name: "channel type falls back",
			item: make(chan int),
		},
		{
			name: "function type falls back",
			item: func() {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function
			got, err := interpolation.SerializeLoopItem(tt.item)

			// Verify graceful degradation (no error, non-empty result)
			assert.NoError(t, err, "Should not return error (graceful degradation per ADR-004)")
			assert.NotEmpty(t, got, "Should return non-empty fallback value")
		})
	}
}

// TestSerializeLoopItem_ErrorHandling_SuccessfulSerialization verifies
// that serialization succeeds normally for supported complex types.
func TestSerializeLoopItem_ErrorHandling_SuccessfulSerialization(t *testing.T) {
	tests := []struct {
		name string
		item any
	}{
		{
			name: "map serializes successfully",
			item: map[string]any{"key": "value"},
		},
		{
			name: "slice serializes successfully",
			item: []string{"a", "b", "c"},
		},
		{
			name: "struct serializes successfully",
			item: struct {
				Name string `json:"name"`
			}{Name: "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function
			got, err := interpolation.SerializeLoopItem(tt.item)

			// Verify successful serialization
			require.NoError(t, err)
			require.NotEmpty(t, got)
		})
	}
}

// TestSerializeLoopItem_ErrorHandling_FallbackValue verifies that the fallback
// returns descriptive type names instead of memory addresses.
// Component S3: Improved fallback to use reflect.TypeOf(v).String() for clarity.
func TestSerializeLoopItem_ErrorHandling_FallbackValue(t *testing.T) {
	tests := []struct {
		name         string
		item         any
		wantContains string // The fallback should contain this substring
	}{
		{
			name:         "channel fallback contains type name",
			item:         make(chan int),
			wantContains: "chan int", // Descriptive type name
		},
		{
			name:         "function fallback contains func keyword",
			item:         func() {},
			wantContains: "func()", // Descriptive type name
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function
			got, err := interpolation.SerializeLoopItem(tt.item)

			// Verify graceful degradation
			require.NoError(t, err)
			require.NotEmpty(t, got)

			// Verify fallback format uses descriptive type names
			assert.Contains(t, got, tt.wantContains,
				"Fallback value should contain descriptive type name")
			assert.Contains(t, got, "<unsupported:",
				"Fallback value should be wrapped in <unsupported:...> format")
		})
	}
}

// TestSerializeLoopItem_ErrorHandling_MultipleCalls verifies that multiple
// failed serializations all gracefully degrade.
func TestSerializeLoopItem_ErrorHandling_MultipleCalls(t *testing.T) {
	// Make multiple calls with unsupported types
	items := []any{
		make(chan int),
		func() {},
		make(chan string),
	}

	for _, item := range items {
		got, err := interpolation.SerializeLoopItem(item)
		require.NoError(t, err)
		require.NotEmpty(t, got)
	}
}

// TestSerializeLoopItem_ErrorHandling_PrimitivesNeverLog verifies that
// primitive types never trigger warning logs (they use the type switch fast path).
func TestSerializeLoopItem_ErrorHandling_PrimitivesNeverLog(t *testing.T) {
	// Capture log output
	var logBuf bytes.Buffer
	originalOutput := log.Writer()
	log.SetOutput(&logBuf)
	defer log.SetOutput(originalOutput)

	primitives := []any{
		"string",
		42,
		3.14,
		true,
		nil,
	}

	for _, item := range primitives {
		_, err := interpolation.SerializeLoopItem(item)
		require.NoError(t, err)
	}

	// Verify NO logs were produced
	logOutput := logBuf.String()
	assert.Empty(t, logOutput,
		"Primitives should never produce logs (they use fast path)")
}
