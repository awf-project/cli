package validation

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRequired(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		value    any
		required bool
		wantErr  bool
	}{
		{
			name:     "required present",
			input:    "email",
			value:    "test@example.com",
			required: true,
			wantErr:  false,
		},
		{
			name:     "required missing nil",
			input:    "email",
			value:    nil,
			required: true,
			wantErr:  true,
		},
		{
			name:     "required missing empty string",
			input:    "email",
			value:    "",
			required: true,
			wantErr:  true,
		},
		{
			name:     "optional missing",
			input:    "email",
			value:    nil,
			required: false,
			wantErr:  false,
		},
		{
			name:     "required zero int is valid",
			input:    "count",
			value:    0,
			required: true,
			wantErr:  false,
		},
		{
			name:     "required false bool is valid",
			input:    "enabled",
			value:    false,
			required: true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRequired(tt.input, tt.value, tt.required)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.input)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateType_String(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		value   any
		want    any
		wantErr bool
	}{
		{
			name:    "string value",
			input:   "name",
			value:   "hello",
			want:    "hello",
			wantErr: false,
		},
		{
			name:    "int coerced to string",
			input:   "name",
			value:   123,
			want:    "123",
			wantErr: false,
		},
		{
			name:    "bool coerced to string",
			input:   "name",
			value:   true,
			want:    "true",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateType(tt.input, tt.value, "string")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestValidateType_Integer(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		value   any
		want    any
		wantErr bool
	}{
		{
			name:    "int value",
			input:   "count",
			value:   42,
			want:    42,
			wantErr: false,
		},
		{
			name:    "int64 value",
			input:   "count",
			value:   int64(42),
			want:    42,
			wantErr: false,
		},
		{
			name:    "string coerced to int",
			input:   "count",
			value:   "42",
			want:    42,
			wantErr: false,
		},
		{
			name:    "float64 coerced to int",
			input:   "count",
			value:   42.0,
			want:    42,
			wantErr: false,
		},
		{
			name:    "invalid string",
			input:   "count",
			value:   "not-a-number",
			wantErr: true,
		},
		{
			name:    "bool cannot convert to int",
			input:   "count",
			value:   true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateType(tt.input, tt.value, "integer")
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.input)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestValidateType_Boolean(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		value   any
		want    any
		wantErr bool
	}{
		{
			name:    "bool true",
			input:   "enabled",
			value:   true,
			want:    true,
			wantErr: false,
		},
		{
			name:    "bool false",
			input:   "enabled",
			value:   false,
			want:    false,
			wantErr: false,
		},
		{
			name:    "string true",
			input:   "enabled",
			value:   "true",
			want:    true,
			wantErr: false,
		},
		{
			name:    "string false",
			input:   "enabled",
			value:   "false",
			want:    false,
			wantErr: false,
		},
		{
			name:    "string yes",
			input:   "enabled",
			value:   "yes",
			want:    true,
			wantErr: false,
		},
		{
			name:    "string no",
			input:   "enabled",
			value:   "no",
			want:    false,
			wantErr: false,
		},
		{
			name:    "string 1",
			input:   "enabled",
			value:   "1",
			want:    true,
			wantErr: false,
		},
		{
			name:    "string 0",
			input:   "enabled",
			value:   "0",
			want:    false,
			wantErr: false,
		},
		{
			name:    "invalid string",
			input:   "enabled",
			value:   "maybe",
			wantErr: true,
		},
		{
			name:    "int cannot convert to bool",
			input:   "enabled",
			value:   1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateType(tt.input, tt.value, "boolean")
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.input)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestValidatePattern(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		value   string
		pattern string
		wantErr bool
	}{
		{
			name:    "email valid",
			input:   "email",
			value:   "test@example.com",
			pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
			wantErr: false,
		},
		{
			name:    "email invalid",
			input:   "email",
			value:   "not-an-email",
			pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
			wantErr: true,
		},
		{
			name:    "simple pattern match",
			input:   "code",
			value:   "abc123",
			pattern: `^[a-z]+[0-9]+$`,
			wantErr: false,
		},
		{
			name:    "simple pattern no match",
			input:   "code",
			value:   "123abc",
			pattern: `^[a-z]+[0-9]+$`,
			wantErr: true,
		},
		{
			name:    "invalid regex",
			input:   "code",
			value:   "anything",
			pattern: `[invalid`,
			wantErr: true,
		},
		{
			name:    "empty pattern skips validation",
			input:   "code",
			value:   "anything",
			pattern: "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePattern(tt.input, tt.value, tt.pattern)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.input)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEnum(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		value   string
		allowed []string
		wantErr bool
	}{
		{
			name:    "value in list",
			input:   "env",
			value:   "staging",
			allowed: []string{"dev", "staging", "prod"},
			wantErr: false,
		},
		{
			name:    "value not in list",
			input:   "env",
			value:   "local",
			allowed: []string{"dev", "staging", "prod"},
			wantErr: true,
		},
		{
			name:    "empty list skips validation",
			input:   "env",
			value:   "anything",
			allowed: []string{},
			wantErr: false,
		},
		{
			name:    "nil list skips validation",
			input:   "env",
			value:   "anything",
			allowed: nil,
			wantErr: false,
		},
		{
			name:    "case sensitive",
			input:   "env",
			value:   "DEV",
			allowed: []string{"dev", "staging", "prod"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEnum(tt.input, tt.value, tt.allowed)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.input)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateRange(t *testing.T) {
	min1, max100 := 1, 100
	min0 := 0
	maxNeg := -1

	tests := []struct {
		name    string
		input   string
		value   int
		min     *int
		max     *int
		wantErr bool
	}{
		{
			name:    "value in range",
			input:   "count",
			value:   50,
			min:     &min1,
			max:     &max100,
			wantErr: false,
		},
		{
			name:    "value at min boundary",
			input:   "count",
			value:   1,
			min:     &min1,
			max:     &max100,
			wantErr: false,
		},
		{
			name:    "value at max boundary",
			input:   "count",
			value:   100,
			min:     &min1,
			max:     &max100,
			wantErr: false,
		},
		{
			name:    "value below min",
			input:   "count",
			value:   0,
			min:     &min1,
			max:     &max100,
			wantErr: true,
		},
		{
			name:    "value above max",
			input:   "count",
			value:   150,
			min:     &min1,
			max:     &max100,
			wantErr: true,
		},
		{
			name:    "min only",
			input:   "count",
			value:   1000,
			min:     &min1,
			max:     nil,
			wantErr: false,
		},
		{
			name:    "max only",
			input:   "count",
			value:   -1000,
			min:     nil,
			max:     &max100,
			wantErr: false,
		},
		{
			name:    "no constraints",
			input:   "count",
			value:   999999,
			min:     nil,
			max:     nil,
			wantErr: false,
		},
		{
			name:    "zero min",
			input:   "count",
			value:   0,
			min:     &min0,
			max:     &max100,
			wantErr: false,
		},
		{
			name:    "negative max",
			input:   "count",
			value:   -5,
			min:     nil,
			max:     &maxNeg,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRange(tt.input, tt.value, tt.min, tt.max)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.input)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFileExists(t *testing.T) {
	// Create a temp file for testing
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	require.NoError(t, err)
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "test-dir-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tests := []struct {
		name    string
		input   string
		path    string
		wantErr bool
	}{
		{
			name:    "file exists",
			input:   "config",
			path:    tmpFile.Name(),
			wantErr: false,
		},
		{
			name:    "file not exists",
			input:   "config",
			path:    "/nonexistent/path/to/file.txt",
			wantErr: true,
		},
		{
			name:    "directory exists",
			input:   "dir",
			path:    tmpDir,
			wantErr: false,
		},
		{
			name:    "empty path skipped",
			input:   "config",
			path:    "",
			wantErr: false, // empty paths are valid for optional inputs
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFileExists(tt.input, tt.path)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.input)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFileExtension(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		path    string
		allowed []string
		wantErr bool
	}{
		{
			name:    "valid extension",
			input:   "script",
			path:    "/path/to/file.py",
			allowed: []string{".py", ".js", ".go"},
			wantErr: false,
		},
		{
			name:    "invalid extension",
			input:   "script",
			path:    "/path/to/file.rb",
			allowed: []string{".py", ".js", ".go"},
			wantErr: true,
		},
		{
			name:    "no extension",
			input:   "script",
			path:    "/path/to/Makefile",
			allowed: []string{".py", ".js", ".go"},
			wantErr: true,
		},
		{
			name:    "empty allowed list skips validation",
			input:   "script",
			path:    "/path/to/file.xyz",
			allowed: []string{},
			wantErr: false,
		},
		{
			name:    "nil allowed list skips validation",
			input:   "script",
			path:    "/path/to/file.xyz",
			allowed: nil,
			wantErr: false,
		},
		{
			name:    "case insensitive extension",
			input:   "script",
			path:    "/path/to/file.PY",
			allowed: []string{".py", ".js", ".go"},
			wantErr: false,
		},
		{
			name:    "double extension uses last",
			input:   "script",
			path:    "/path/to/file.tar.gz",
			allowed: []string{".gz", ".zip"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFileExtension(tt.input, tt.path, tt.allowed)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.input)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateInputs(t *testing.T) {
	min1, max100 := 1, 100

	// Create a temp file for file_exists tests
	tmpFile, err := os.CreateTemp("", "test-*.go")
	require.NoError(t, err)
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	tests := []struct {
		name        string
		inputs      map[string]any
		definitions []Input
		wantErr     bool
		errCount    int // expected number of errors (if wantErr)
	}{
		{
			name: "all valid",
			inputs: map[string]any{
				"email": "test@example.com",
				"count": 50,
			},
			definitions: []Input{
				{
					Name:     "email",
					Type:     "string",
					Required: true,
					Validation: &Rules{
						Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
					},
				},
				{
					Name:     "count",
					Type:     "integer",
					Required: true,
					Validation: &Rules{
						Min: &min1,
						Max: &max100,
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "required missing",
			inputs: map[string]any{},
			definitions: []Input{
				{
					Name:     "email",
					Type:     "string",
					Required: true,
				},
			},
			wantErr:  true,
			errCount: 1,
		},
		{
			name: "multiple errors aggregated",
			inputs: map[string]any{
				"email": "not-an-email",
				"count": 150,
			},
			definitions: []Input{
				{
					Name:     "email",
					Type:     "string",
					Required: true,
					Validation: &Rules{
						Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
					},
				},
				{
					Name:     "count",
					Type:     "integer",
					Required: true,
					Validation: &Rules{
						Max: &max100,
					},
				},
			},
			wantErr:  true,
			errCount: 2,
		},
		{
			name:        "no definitions skips validation",
			inputs:      map[string]any{"anything": "value"},
			definitions: []Input{},
			wantErr:     false,
		},
		{
			name: "nil validation rules",
			inputs: map[string]any{
				"name": "test",
			},
			definitions: []Input{
				{
					Name:       "name",
					Type:       "string",
					Required:   true,
					Validation: nil,
				},
			},
			wantErr: false,
		},
		{
			name: "type coercion string to int",
			inputs: map[string]any{
				"count": "42",
			},
			definitions: []Input{
				{
					Name:     "count",
					Type:     "integer",
					Required: true,
				},
			},
			wantErr: false,
		},
		{
			name: "file validation",
			inputs: map[string]any{
				"script": tmpFile.Name(),
			},
			definitions: []Input{
				{
					Name:     "script",
					Type:     "string",
					Required: true,
					Validation: &Rules{
						FileExists:    true,
						FileExtension: []string{".go"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "file not exists",
			inputs: map[string]any{
				"script": "/nonexistent/file.go",
			},
			definitions: []Input{
				{
					Name:     "script",
					Type:     "string",
					Required: true,
					Validation: &Rules{
						FileExists: true,
					},
				},
			},
			wantErr:  true,
			errCount: 1,
		},
		{
			name: "enum validation",
			inputs: map[string]any{
				"env": "production",
			},
			definitions: []Input{
				{
					Name:     "env",
					Type:     "string",
					Required: true,
					Validation: &Rules{
						Enum: []string{"dev", "staging", "prod"},
					},
				},
			},
			wantErr:  true,
			errCount: 1,
		},
		{
			name: "optional input not provided",
			inputs: map[string]any{
				"name": "test",
			},
			definitions: []Input{
				{
					Name:     "name",
					Type:     "string",
					Required: true,
				},
				{
					Name:     "optional",
					Type:     "string",
					Required: false,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInputs(tt.inputs, tt.definitions)
			if tt.wantErr {
				assert.Error(t, err)
				var valErr *ValidationError
				if errors.As(err, &valErr) {
					assert.Len(t, valErr.Errors, tt.errCount)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	t.Run("empty errors", func(t *testing.T) {
		err := &ValidationError{}
		assert.Equal(t, "validation failed", err.Error())
	})

	t.Run("single error", func(t *testing.T) {
		err := &ValidationError{Errors: []string{"inputs.email: required"}}
		assert.Equal(t, "inputs.email: required", err.Error())
	})

	t.Run("multiple errors", func(t *testing.T) {
		err := &ValidationError{Errors: []string{
			"inputs.email: required",
			"inputs.count: exceeds max",
		}}
		expected := "2 errors:\n  - inputs.email: required\n  - inputs.count: exceeds max"
		assert.Equal(t, expected, err.Error())
	})
}

// =============================================================================
// Additional Edge Case Tests
// =============================================================================

func TestValidateType_UnknownType(t *testing.T) {
	// Unknown type should return error
	_, err := validateType("field", "value", "unknown_type")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "field")
	assert.Contains(t, err.Error(), "unknown_type")
}

func TestValidateType_EmptyType(t *testing.T) {
	// Empty type should be handled (default to string or error)
	got, err := validateType("field", "value", "")
	// Depending on implementation: could accept any value or require explicit type
	// For now, test that it doesn't panic
	_ = got
	_ = err
}

func TestValidateType_NilValue(t *testing.T) {
	// Type validation on nil value should be handled
	_, err := validateType("field", nil, "string")
	// nil cannot be converted to string - should error or skip
	// This depends on implementation - testing for no panic
	_ = err
}

func TestValidatePattern_EmptyValue(t *testing.T) {
	// Empty string should still be validated against pattern
	err := validatePattern("field", "", `^[a-z]+$`)
	// Empty string doesn't match ^[a-z]+$
	assert.Error(t, err)
}

func TestValidatePattern_ComplexRegex(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		pattern string
		wantErr bool
	}{
		{
			name:    "url pattern match",
			value:   "https://example.com/path",
			pattern: `^https?://[a-zA-Z0-9.-]+(/.*)?$`,
			wantErr: false,
		},
		{
			name:    "url pattern no match",
			value:   "ftp://example.com",
			pattern: `^https?://[a-zA-Z0-9.-]+(/.*)?$`,
			wantErr: true,
		},
		{
			name:    "semver pattern",
			value:   "1.2.3",
			pattern: `^[0-9]+\.[0-9]+\.[0-9]+$`,
			wantErr: false,
		},
		{
			name:    "semver with pre-release",
			value:   "1.2.3-beta",
			pattern: `^[0-9]+\.[0-9]+\.[0-9]+$`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePattern("field", tt.value, tt.pattern)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEnum_EmptyValue(t *testing.T) {
	// Empty string against enum
	err := validateEnum("field", "", []string{"a", "b", "c"})
	// Empty string is not in the list
	assert.Error(t, err)
}

func TestValidateEnum_SingleItemList(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		allowed []string
		wantErr bool
	}{
		{
			name:    "match single item",
			value:   "only",
			allowed: []string{"only"},
			wantErr: false,
		},
		{
			name:    "no match single item",
			value:   "other",
			allowed: []string{"only"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEnum("field", tt.value, tt.allowed)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateRange_NegativeValues(t *testing.T) {
	minNeg10, maxNeg1 := -10, -1

	tests := []struct {
		name    string
		value   int
		min     *int
		max     *int
		wantErr bool
	}{
		{
			name:    "negative in negative range",
			value:   -5,
			min:     &minNeg10,
			max:     &maxNeg1,
			wantErr: false,
		},
		{
			name:    "zero outside negative range",
			value:   0,
			min:     &minNeg10,
			max:     &maxNeg1,
			wantErr: true,
		},
		{
			name:    "negative at min boundary",
			value:   -10,
			min:     &minNeg10,
			max:     &maxNeg1,
			wantErr: false,
		},
		{
			name:    "negative at max boundary",
			value:   -1,
			min:     &minNeg10,
			max:     &maxNeg1,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRange("field", tt.value, tt.min, tt.max)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateRange_EqualMinMax(t *testing.T) {
	// When min == max, only that exact value is valid
	val := 42

	tests := []struct {
		name    string
		value   int
		wantErr bool
	}{
		{name: "exact match", value: 42, wantErr: false},
		{name: "below", value: 41, wantErr: true},
		{name: "above", value: 43, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRange("field", tt.value, &val, &val)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFileExists_Symlink(t *testing.T) {
	// Create a temp file and symlink
	tmpDir := t.TempDir()
	realFile := filepath.Join(tmpDir, "real.txt")
	linkFile := filepath.Join(tmpDir, "link.txt")

	err := os.WriteFile(realFile, []byte("content"), 0o644)
	require.NoError(t, err)

	err = os.Symlink(realFile, linkFile)
	require.NoError(t, err)

	// Both should exist
	err = validateFileExists("config", realFile)
	assert.NoError(t, err)

	err = validateFileExists("config", linkFile)
	assert.NoError(t, err)
}

func TestValidateFileExtension_CaseSensitivity(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		allowed []string
		wantErr bool
	}{
		{
			name:    "lowercase ext matches lowercase allowed",
			path:    "/path/file.go",
			allowed: []string{".go"},
			wantErr: false,
		},
		{
			name:    "uppercase ext matches lowercase allowed",
			path:    "/path/file.GO",
			allowed: []string{".go"},
			wantErr: false,
		},
		{
			name:    "mixed case ext matches lowercase allowed",
			path:    "/path/file.Go",
			allowed: []string{".go"},
			wantErr: false,
		},
		{
			name:    "lowercase ext matches uppercase allowed",
			path:    "/path/file.py",
			allowed: []string{".PY"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFileExtension("script", tt.path, tt.allowed)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFileExtension_HiddenFiles(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		allowed []string
		wantErr bool
	}{
		{
			name:    "hidden file with extension",
			path:    "/path/.gitignore",
			allowed: []string{".gitignore"},
			wantErr: false,
		},
		{
			name:    "hidden file no extension match",
			path:    "/path/.bashrc",
			allowed: []string{".txt"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFileExtension("script", tt.path, tt.allowed)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateInputs_CombinedValidationRules(t *testing.T) {
	// Test input with multiple validation rules (pattern + enum shouldn't both be set typically)
	// but test combined file_exists + file_extension
	tmpDir := t.TempDir()
	validFile := filepath.Join(tmpDir, "script.py")
	err := os.WriteFile(validFile, []byte("# python"), 0o644)
	require.NoError(t, err)

	invalidExtFile := filepath.Join(tmpDir, "script.rb")
	err = os.WriteFile(invalidExtFile, []byte("# ruby"), 0o644)
	require.NoError(t, err)

	tests := []struct {
		name        string
		inputs      map[string]any
		definitions []Input
		wantErr     bool
		errCount    int
	}{
		{
			name: "file exists with valid extension",
			inputs: map[string]any{
				"script": validFile,
			},
			definitions: []Input{
				{
					Name:     "script",
					Type:     "string",
					Required: true,
					Validation: &Rules{
						FileExists:    true,
						FileExtension: []string{".py"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "file exists with invalid extension",
			inputs: map[string]any{
				"script": invalidExtFile,
			},
			definitions: []Input{
				{
					Name:     "script",
					Type:     "string",
					Required: true,
					Validation: &Rules{
						FileExists:    true,
						FileExtension: []string{".py"},
					},
				},
			},
			wantErr:  true,
			errCount: 1, // extension validation fails
		},
		{
			name: "file not exists and invalid extension",
			inputs: map[string]any{
				"script": "/nonexistent/file.rb",
			},
			definitions: []Input{
				{
					Name:     "script",
					Type:     "string",
					Required: true,
					Validation: &Rules{
						FileExists:    true,
						FileExtension: []string{".py"},
					},
				},
			},
			wantErr:  true,
			errCount: 2, // both file_exists and extension fail
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInputs(tt.inputs, tt.definitions)
			if tt.wantErr {
				assert.Error(t, err)
				var valErr *ValidationError
				if errors.As(err, &valErr) {
					assert.Len(t, valErr.Errors, tt.errCount)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateInputs_AllValidationTypes(t *testing.T) {
	// Comprehensive test with all validation rule types on multiple inputs
	tmpFile, err := os.CreateTemp("", "test-*.py")
	require.NoError(t, err)
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	min1, max100 := 1, 100

	inputs := map[string]any{
		"email":  "test@example.com",
		"count":  50,
		"env":    "staging",
		"script": tmpFile.Name(),
	}

	definitions := []Input{
		{
			Name:     "email",
			Type:     "string",
			Required: true,
			Validation: &Rules{
				Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
			},
		},
		{
			Name:     "count",
			Type:     "integer",
			Required: true,
			Validation: &Rules{
				Min: &min1,
				Max: &max100,
			},
		},
		{
			Name:     "env",
			Type:     "string",
			Required: true,
			Validation: &Rules{
				Enum: []string{"dev", "staging", "prod"},
			},
		},
		{
			Name:     "script",
			Type:     "string",
			Required: true,
			Validation: &Rules{
				FileExists:    true,
				FileExtension: []string{".py"},
			},
		},
	}

	err = ValidateInputs(inputs, definitions)
	assert.NoError(t, err)
}

func TestValidateInputs_ExtraInputsIgnored(t *testing.T) {
	// Inputs not in definitions should be ignored
	inputs := map[string]any{
		"defined":   "value",
		"undefined": "extra_value",
	}

	definitions := []Input{
		{Name: "defined", Type: "string", Required: true},
	}

	err := ValidateInputs(inputs, definitions)
	assert.NoError(t, err)
}

func TestValidateInputs_NilInputsMap(t *testing.T) {
	definitions := []Input{
		{Name: "optional", Type: "string", Required: false},
	}

	// nil inputs map should work for optional fields
	err := ValidateInputs(nil, definitions)
	assert.NoError(t, err)
}

func TestValidateInputs_NilDefinitions(t *testing.T) {
	inputs := map[string]any{"key": "value"}

	// nil definitions should skip validation
	err := ValidateInputs(inputs, nil)
	assert.NoError(t, err)
}

// Feature: C001 - Type-Checked Validator Wrappers
// These tests verify that the wrapper functions properly handle type assertions
// and delegation to atomic validators.

func TestValidatePatternWithType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		value   any
		pattern string
		wantErr bool
	}{
		{
			name:    "string value with valid pattern",
			input:   "email",
			value:   "test@example.com",
			pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
			wantErr: false,
		},
		{
			name:    "string value with invalid pattern",
			input:   "email",
			value:   "not-an-email",
			pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
			wantErr: true,
		},
		{
			name:    "int value skips validation (returns nil)",
			input:   "port",
			value:   8080,
			pattern: `^\d+$`,
			wantErr: false,
		},
		{
			name:    "bool value skips validation (returns nil)",
			input:   "flag",
			value:   true,
			pattern: `^true$`,
			wantErr: false,
		},
		{
			name:    "nil value skips validation (returns nil)",
			input:   "optional",
			value:   nil,
			pattern: `.*`,
			wantErr: false,
		},
		{
			name:    "empty pattern with string value",
			input:   "code",
			value:   "anything",
			pattern: "",
			wantErr: false,
		},
		{
			name:    "invalid regex pattern with string value",
			input:   "code",
			value:   "test",
			pattern: `[invalid`,
			wantErr: true,
		},
		{
			name:    "complex pattern match",
			input:   "code",
			value:   "abc123",
			pattern: `^[a-z]+[0-9]+$`,
			wantErr: false,
		},
		{
			name:    "complex pattern no match",
			input:   "code",
			value:   "123abc",
			pattern: `^[a-z]+[0-9]+$`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePatternWithType(tt.input, tt.value, tt.pattern)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.input)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEnumWithType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		value   any
		enum    []string
		wantErr bool
	}{
		{
			name:    "string value in enum list",
			input:   "env",
			value:   "staging",
			enum:    []string{"dev", "staging", "prod"},
			wantErr: false,
		},
		{
			name:    "string value not in enum list",
			input:   "env",
			value:   "local",
			enum:    []string{"dev", "staging", "prod"},
			wantErr: true,
		},
		{
			name:    "int value skips validation (returns nil)",
			input:   "count",
			value:   42,
			enum:    []string{"1", "2", "3"},
			wantErr: false,
		},
		{
			name:    "bool value skips validation (returns nil)",
			input:   "flag",
			value:   false,
			enum:    []string{"true", "false"},
			wantErr: false,
		},
		{
			name:    "nil value skips validation (returns nil)",
			input:   "optional",
			value:   nil,
			enum:    []string{"a", "b"},
			wantErr: false,
		},
		{
			name:    "empty enum list with string value",
			input:   "env",
			value:   "anything",
			enum:    []string{},
			wantErr: false,
		},
		{
			name:    "nil enum list with string value",
			input:   "env",
			value:   "anything",
			enum:    nil,
			wantErr: false,
		},
		{
			name:    "case sensitive check",
			input:   "env",
			value:   "DEV",
			enum:    []string{"dev", "staging", "prod"},
			wantErr: true,
		},
		{
			name:    "single item enum list match",
			input:   "mode",
			value:   "single",
			enum:    []string{"single"},
			wantErr: false,
		},
		{
			name:    "single item enum list no match",
			input:   "mode",
			value:   "multi",
			enum:    []string{"single"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEnumWithType(tt.input, tt.value, tt.enum)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.input)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateRangeWithType(t *testing.T) {
	min1, max100 := 1, 100
	min0 := 0
	maxNeg := -1
	min10, max10 := 10, 10

	tests := []struct {
		name    string
		input   string
		value   any
		min     *int
		max     *int
		wantErr bool
	}{
		{
			name:    "int value in range",
			input:   "count",
			value:   50,
			min:     &min1,
			max:     &max100,
			wantErr: false,
		},
		{
			name:    "int value at min boundary",
			input:   "count",
			value:   1,
			min:     &min1,
			max:     &max100,
			wantErr: false,
		},
		{
			name:    "int value at max boundary",
			input:   "count",
			value:   100,
			min:     &min1,
			max:     &max100,
			wantErr: false,
		},
		{
			name:    "int value below min",
			input:   "count",
			value:   0,
			min:     &min1,
			max:     &max100,
			wantErr: true,
		},
		{
			name:    "int value above max",
			input:   "count",
			value:   150,
			min:     &min1,
			max:     &max100,
			wantErr: true,
		},
		{
			name:    "string value skips validation (returns nil)",
			input:   "port",
			value:   "8080",
			min:     &min1,
			max:     &max100,
			wantErr: false,
		},
		{
			name:    "bool value skips validation (returns nil)",
			input:   "flag",
			value:   true,
			min:     &min1,
			max:     &max100,
			wantErr: false,
		},
		{
			name:    "nil value skips validation (returns nil)",
			input:   "optional",
			value:   nil,
			min:     &min1,
			max:     &max100,
			wantErr: false,
		},
		{
			name:    "min only validation pass",
			input:   "count",
			value:   1000,
			min:     &min1,
			max:     nil,
			wantErr: false,
		},
		{
			name:    "max only validation pass",
			input:   "count",
			value:   -1000,
			min:     nil,
			max:     &max100,
			wantErr: false,
		},
		{
			name:    "no constraints",
			input:   "count",
			value:   999999,
			min:     nil,
			max:     nil,
			wantErr: false,
		},
		{
			name:    "zero as min boundary",
			input:   "count",
			value:   0,
			min:     &min0,
			max:     &max100,
			wantErr: false,
		},
		{
			name:    "negative max boundary",
			input:   "count",
			value:   -5,
			min:     nil,
			max:     &maxNeg,
			wantErr: false,
		},
		{
			name:    "equal min and max - value matches",
			input:   "exact",
			value:   10,
			min:     &min10,
			max:     &max10,
			wantErr: false,
		},
		{
			name:    "equal min and max - value below",
			input:   "exact",
			value:   9,
			min:     &min10,
			max:     &max10,
			wantErr: true,
		},
		{
			name:    "equal min and max - value above",
			input:   "exact",
			value:   11,
			min:     &min10,
			max:     &max10,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRangeWithType(tt.input, tt.value, tt.min, tt.max)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.input)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFileExistsWithType(t *testing.T) {
	// Create a temporary test file
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tests := []struct {
		name    string
		input   string
		value   any
		wantErr bool
	}{
		{
			name:    "string value with existing file",
			input:   "config",
			value:   tmpFile.Name(),
			wantErr: false,
		},
		{
			name:    "string value with non-existent file",
			input:   "config",
			value:   "/path/to/nonexistent/file.txt",
			wantErr: true,
		},
		{
			name:    "int value skips validation (returns nil)",
			input:   "file",
			value:   42,
			wantErr: false,
		},
		{
			name:    "bool value skips validation (returns nil)",
			input:   "file",
			value:   true,
			wantErr: false,
		},
		{
			name:    "nil value skips validation (returns nil)",
			input:   "optional",
			value:   nil,
			wantErr: false,
		},
		{
			name:    "empty string path",
			input:   "file",
			value:   "",
			wantErr: true,
		},
		{
			name:    "directory path (exists)",
			input:   "dir",
			value:   os.TempDir(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFileExistsWithType(tt.input, tt.value)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.input)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFileExtensionWithType(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		value      any
		extensions []string
		wantErr    bool
	}{
		{
			name:       "string value with matching extension",
			input:      "config",
			value:      "config.yaml",
			extensions: []string{".yaml", ".yml"},
			wantErr:    false,
		},
		{
			name:       "string value with non-matching extension",
			input:      "config",
			value:      "config.json",
			extensions: []string{".yaml", ".yml"},
			wantErr:    true,
		},
		{
			name:       "int value skips validation (returns nil)",
			input:      "file",
			value:      42,
			extensions: []string{".txt"},
			wantErr:    false,
		},
		{
			name:       "bool value skips validation (returns nil)",
			input:      "file",
			value:      false,
			extensions: []string{".txt"},
			wantErr:    false,
		},
		{
			name:       "nil value skips validation (returns nil)",
			input:      "optional",
			value:      nil,
			extensions: []string{".txt"},
			wantErr:    false,
		},
		{
			name:       "empty extensions list",
			input:      "file",
			value:      "anything.txt",
			extensions: []string{},
			wantErr:    false,
		},
		{
			name:       "nil extensions list",
			input:      "file",
			value:      "anything.txt",
			extensions: nil,
			wantErr:    false,
		},
		{
			name:       "case sensitive extension check",
			input:      "file",
			value:      "config.YAML",
			extensions: []string{".yaml"},
			wantErr:    true,
		},
		{
			name:       "single extension match",
			input:      "script",
			value:      "run.sh",
			extensions: []string{".sh"},
			wantErr:    false,
		},
		{
			name:       "file without extension",
			input:      "file",
			value:      "README",
			extensions: []string{".md"},
			wantErr:    true,
		},
		{
			name:       "hidden file with extension",
			input:      "dotfile",
			value:      ".gitignore",
			extensions: []string{".gitignore"},
			wantErr:    false,
		},
		{
			name:       "path with multiple dots",
			input:      "archive",
			value:      "backup.tar.gz",
			extensions: []string{".gz", ".zip"},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFileExtensionWithType(tt.input, tt.value, tt.extensions)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.input)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Ensure filepath is used
var _ = filepath.Base
