//go:build integration

// Feature: C001 - validateRules Cognitive Complexity Reduction
// This test suite validates that the refactored validateRules function
// maintains 100% backward compatibility while reducing cognitive complexity
// from 31 to ≤20 through extraction of type-checked validator wrappers.

package validation_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/awf/pkg/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateRules_HappyPath validates normal usage with all validation types
func TestValidateRules_HappyPath(t *testing.T) {
	tests := []struct {
		name        string
		definitions []validation.Input
		inputs      map[string]any
	}{
		{
			name: "pattern validation success",
			definitions: []validation.Input{
				{
					Name:     "email",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						Pattern: `^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`,
					},
				},
			},
			inputs: map[string]any{
				"email": "user@example.com",
			},
		},
		{
			name: "enum validation success",
			definitions: []validation.Input{
				{
					Name:     "level",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						Enum: []string{"debug", "info", "warn", "error"},
					},
				},
			},
			inputs: map[string]any{
				"level": "info",
			},
		},
		{
			name: "range validation success",
			definitions: []validation.Input{
				{
					Name:     "port",
					Type:     "integer",
					Required: true,
					Validation: &validation.Rules{
						Min: intPtr(1024),
						Max: intPtr(65535),
					},
				},
			},
			inputs: map[string]any{
				"port": 8080,
			},
		},
		{
			name: "combined validations success",
			definitions: []validation.Input{
				{
					Name:     "email",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						Pattern: `^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`,
					},
				},
				{
					Name:     "level",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						Enum: []string{"debug", "info", "warn", "error"},
					},
				},
				{
					Name:     "port",
					Type:     "integer",
					Required: true,
					Validation: &validation.Rules{
						Min: intPtr(1024),
						Max: intPtr(65535),
					},
				},
			},
			inputs: map[string]any{
				"email": "admin@example.com",
				"level": "debug",
				"port":  3000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateInputs(tt.inputs, tt.definitions)
			require.NoError(t, err, "validation should succeed")
		})
	}
}

// TestValidateRules_EdgeCases validates boundary conditions and edge cases
func TestValidateRules_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		definitions []validation.Input
		inputs      map[string]any
		wantErr     bool
		wantErrs    []string // partial error message matches
	}{
		{
			name: "empty string with required",
			definitions: []validation.Input{
				{
					Name:     "optional",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						Pattern: `^[a-z]*$`, // allows empty
					},
				},
			},
			inputs: map[string]any{
				"optional": "",
			},
			wantErr:  true,
			wantErrs: []string{"optional"},
		},
		{
			name: "range boundary min",
			definitions: []validation.Input{
				{
					Name:     "port",
					Type:     "integer",
					Required: true,
					Validation: &validation.Rules{
						Min: intPtr(1024),
						Max: intPtr(65535),
					},
				},
			},
			inputs: map[string]any{
				"port": 1024, // exact min
			},
			wantErr:  false,
			wantErrs: nil,
		},
		{
			name: "range boundary max",
			definitions: []validation.Input{
				{
					Name:     "port",
					Type:     "integer",
					Required: true,
					Validation: &validation.Rules{
						Min: intPtr(1024),
						Max: intPtr(65535),
					},
				},
			},
			inputs: map[string]any{
				"port": 65535, // exact max
			},
			wantErr:  false,
			wantErrs: nil,
		},
		{
			name: "range below min",
			definitions: []validation.Input{
				{
					Name:     "port",
					Type:     "integer",
					Required: true,
					Validation: &validation.Rules{
						Min: intPtr(1024),
						Max: intPtr(65535),
					},
				},
			},
			inputs: map[string]any{
				"port": 1023,
			},
			wantErr:  true,
			wantErrs: []string{"port", "1024"},
		},
		{
			name: "range above max",
			definitions: []validation.Input{
				{
					Name:     "port",
					Type:     "integer",
					Required: true,
					Validation: &validation.Rules{
						Min: intPtr(1024),
						Max: intPtr(65535),
					},
				},
			},
			inputs: map[string]any{
				"port": 65536,
			},
			wantErr:  true,
			wantErrs: []string{"port", "65535"},
		},
		{
			name: "unicode in pattern",
			definitions: []validation.Input{
				{
					Name:     "name",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						Pattern: `^[\p{L}\s]+$`, // allows unicode letters
					},
				},
			},
			inputs: map[string]any{
				"name": "José María",
			},
			wantErr:  false,
			wantErrs: nil,
		},
		{
			name: "case sensitive enum",
			definitions: []validation.Input{
				{
					Name:     "level",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						Enum: []string{"DEBUG", "INFO", "WARN", "ERROR"},
					},
				},
			},
			inputs: map[string]any{
				"level": "debug", // lowercase
			},
			wantErr:  true,
			wantErrs: []string{"level"},
		},
		{
			name: "multiple rules all failing",
			definitions: []validation.Input{
				{
					Name:     "email",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						Pattern: `^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`,
						Enum:    []string{"admin@example.com", "user@example.com"},
					},
				},
			},
			inputs: map[string]any{
				"email": "invalid-email",
			},
			wantErr:  true,
			wantErrs: []string{"email"},
		},
		{
			name: "zero value integer with range",
			definitions: []validation.Input{
				{
					Name:     "count",
					Type:     "integer",
					Required: true,
					Validation: &validation.Rules{
						Min: intPtr(0),
						Max: intPtr(100),
					},
				},
			},
			inputs: map[string]any{
				"count": 0, // zero is valid
			},
			wantErr:  false,
			wantErrs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateInputs(tt.inputs, tt.definitions)

			if tt.wantErr {
				require.Error(t, err, "validation should fail")
				var valErr *validation.ValidationError
				require.True(t, errors.As(err, &valErr), "error should be ValidationError")

				for _, errMatch := range tt.wantErrs {
					found := false
					for _, actualErr := range valErr.Errors {
						if containsString(actualErr, errMatch) {
							found = true
							break
						}
					}
					assert.True(t, found, "expected error containing '%s' not found in: %v", errMatch, valErr.Errors)
				}
			} else {
				require.NoError(t, err, "validation should succeed")
			}
		})
	}
}

// TestValidateRules_ErrorHandling validates invalid inputs are rejected properly
func TestValidateRules_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		definitions []validation.Input
		inputs      map[string]any
		wantErrs    []string
	}{
		{
			name: "pattern mismatch after type coercion",
			definitions: []validation.Input{
				{
					Name:     "value",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						Pattern: `^[a-z]+$`,
					},
				},
			},
			inputs: map[string]any{
				"value": 12345, // int coerced to "12345", fails pattern
			},
			wantErrs: []string{"value", "pattern"},
		},
		{
			name: "enum mismatch after type coercion",
			definitions: []validation.Input{
				{
					Name:     "status",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						Enum: []string{"active", "inactive"},
					},
				},
			},
			inputs: map[string]any{
				"status": 123, // int coerced to "123", not in enum
			},
			wantErrs: []string{"status"},
		},
		{
			name: "non-coercible type for range validation",
			definitions: []validation.Input{
				{
					Name:     "port",
					Type:     "integer",
					Required: true,
					Validation: &validation.Rules{
						Min: intPtr(1024),
						Max: intPtr(65535),
					},
				},
			},
			inputs: map[string]any{
				"port": []int{8080}, // array cannot be coerced to int
			},
			wantErrs: []string{"port", "convert"},
		},
		{
			name: "invalid pattern regex",
			definitions: []validation.Input{
				{
					Name:     "value",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						Pattern: `^[invalid(`,
					},
				},
			},
			inputs: map[string]any{
				"value": "test",
			},
			wantErrs: []string{"value"},
		},
		{
			name: "enum not in list",
			definitions: []validation.Input{
				{
					Name:     "color",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						Enum: []string{"red", "green", "blue"},
					},
				},
			},
			inputs: map[string]any{
				"color": "yellow",
			},
			wantErrs: []string{"color"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateInputs(tt.inputs, tt.definitions)

			require.Error(t, err, "validation should fail")
			var valErr *validation.ValidationError
			require.True(t, errors.As(err, &valErr), "error should be ValidationError")

			for _, errMatch := range tt.wantErrs {
				found := false
				for _, actualErr := range valErr.Errors {
					if containsString(actualErr, errMatch) {
						found = true
						break
					}
				}
				assert.True(t, found, "expected error containing '%s' not found in: %v", errMatch, valErr.Errors)
			}
		})
	}
}

// TestValidateRules_FileValidation validates file-based validation rules
func TestValidateRules_FileValidation(t *testing.T) {
	// create temp files for testing
	tmpDir := t.TempDir()
	validFile := filepath.Join(tmpDir, "valid.txt")
	yamlFile := filepath.Join(tmpDir, "config.yaml")
	jsonFile := filepath.Join(tmpDir, "data.json")

	require.NoError(t, os.WriteFile(validFile, []byte("content"), 0o644))
	require.NoError(t, os.WriteFile(yamlFile, []byte("key: value"), 0o644))
	require.NoError(t, os.WriteFile(jsonFile, []byte("{}"), 0o644))

	tests := []struct {
		name        string
		definitions []validation.Input
		inputs      map[string]any
		wantErr     bool
		wantErrs    []string
	}{
		{
			name: "file exists validation success",
			definitions: []validation.Input{
				{
					Name:     "config",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						FileExists: true,
					},
				},
			},
			inputs: map[string]any{
				"config": validFile,
			},
			wantErr:  false,
			wantErrs: nil,
		},
		{
			name: "file exists validation failure",
			definitions: []validation.Input{
				{
					Name:     "config",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						FileExists: true,
					},
				},
			},
			inputs: map[string]any{
				"config": filepath.Join(tmpDir, "nonexistent.txt"),
			},
			wantErr:  true,
			wantErrs: []string{"config", "does not exist"},
		},
		{
			name: "file extension validation success",
			definitions: []validation.Input{
				{
					Name:     "config",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						FileExtension: []string{".yaml", ".yml"},
					},
				},
			},
			inputs: map[string]any{
				"config": yamlFile,
			},
			wantErr:  false,
			wantErrs: nil,
		},
		{
			name: "file extension validation failure",
			definitions: []validation.Input{
				{
					Name:     "config",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						FileExtension: []string{".yaml", ".yml"},
					},
				},
			},
			inputs: map[string]any{
				"config": jsonFile,
			},
			wantErr:  true,
			wantErrs: []string{"config"},
		},
		{
			name: "combined file validations",
			definitions: []validation.Input{
				{
					Name:     "config",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						FileExists:    true,
						FileExtension: []string{".yaml", ".yml"},
					},
				},
			},
			inputs: map[string]any{
				"config": yamlFile,
			},
			wantErr:  false,
			wantErrs: nil,
		},
		{
			name: "file validation with non-existent coerced path",
			definitions: []validation.Input{
				{
					Name:     "config",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						FileExists: true,
					},
				},
			},
			inputs: map[string]any{
				"config": 12345, // int coerced to "12345", file doesn't exist
			},
			wantErr:  true,
			wantErrs: []string{"config", "does not exist"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateInputs(tt.inputs, tt.definitions)

			if tt.wantErr {
				require.Error(t, err, "validation should fail")
				var valErr *validation.ValidationError
				require.True(t, errors.As(err, &valErr), "error should be ValidationError")

				for _, errMatch := range tt.wantErrs {
					found := false
					for _, actualErr := range valErr.Errors {
						if containsString(actualErr, errMatch) {
							found = true
							break
						}
					}
					assert.True(t, found, "expected error containing '%s' not found in: %v", errMatch, valErr.Errors)
				}
			} else {
				require.NoError(t, err, "validation should succeed")
			}
		})
	}
}

// TestValidateRules_Integration validates full workflow execution with validation
func TestValidateRules_Integration(t *testing.T) {
	// define complex input schema with all validation types
	definitions := []validation.Input{
		{
			Name:     "email",
			Type:     "string",
			Required: true,
			Validation: &validation.Rules{
				Pattern: `^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`,
			},
		},
		{
			Name:     "log_level",
			Type:     "string",
			Required: true,
			Validation: &validation.Rules{
				Enum: []string{"DEBUG", "INFO", "WARN", "ERROR"},
			},
		},
		{
			Name:     "port",
			Type:     "integer",
			Required: true,
			Validation: &validation.Rules{
				Min: intPtr(1024),
				Max: intPtr(65535),
			},
		},
		{
			Name:     "max_connections",
			Type:     "integer",
			Required: false,
			Validation: &validation.Rules{
				Min: intPtr(1),
				Max: intPtr(1000),
			},
		},
	}

	tests := []struct {
		name     string
		inputs   map[string]any
		wantErr  bool
		errCount int
	}{
		{
			name: "all valid inputs",
			inputs: map[string]any{
				"email":     "admin@example.com",
				"log_level": "INFO",
				"port":      8080,
			},
			wantErr:  false,
			errCount: 0,
		},
		{
			name: "all valid with optional",
			inputs: map[string]any{
				"email":           "admin@example.com",
				"log_level":       "DEBUG",
				"port":            3000,
				"max_connections": 500,
			},
			wantErr:  false,
			errCount: 0,
		},
		{
			name: "multiple validation failures",
			inputs: map[string]any{
				"email":           "invalid-email",
				"log_level":       "TRACE", // not in enum
				"port":            80,      // below min
				"max_connections": 2000,    // above max
			},
			wantErr:  true,
			errCount: 4,
		},
		{
			name: "missing required field",
			inputs: map[string]any{
				"email":     "admin@example.com",
				"log_level": "INFO",
				// port missing
			},
			wantErr:  true,
			errCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateInputs(tt.inputs, definitions)

			if tt.wantErr {
				require.Error(t, err, "validation should fail")
				var valErr *validation.ValidationError
				require.True(t, errors.As(err, &valErr), "error should be ValidationError")
				assert.Equal(t, tt.errCount, len(valErr.Errors), "error count mismatch: %v", valErr.Errors)
			} else {
				require.NoError(t, err, "validation should succeed")
			}
		})
	}
}

// TestValidateRules_BackwardCompatibility ensures refactoring maintains exact behavior
func TestValidateRules_BackwardCompatibility(t *testing.T) {
	// Test scenarios that should work identically before and after refactoring
	tests := []struct {
		name        string
		definitions []validation.Input
		inputs      map[string]any
		wantErr     bool
		errContains []string
	}{
		{
			name: "nil rules should skip validation",
			definitions: []validation.Input{
				{
					Name:       "value",
					Type:       "string",
					Required:   false,
					Validation: nil,
				},
			},
			inputs: map[string]any{
				"value": "anything",
			},
			wantErr:     false,
			errContains: nil,
		},
		{
			name: "empty rules should skip validation",
			definitions: []validation.Input{
				{
					Name:       "value",
					Type:       "string",
					Required:   false,
					Validation: &validation.Rules{},
				},
			},
			inputs: map[string]any{
				"value": "anything",
			},
			wantErr:     false,
			errContains: nil,
		},
		{
			name: "multiple rules with partial match",
			definitions: []validation.Input{
				{
					Name:     "value",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						Pattern: `^[a-z]+$`,
						Enum:    []string{"abc", "def", "ghi"},
					},
				},
			},
			inputs: map[string]any{
				"value": "abc",
			},
			wantErr:     false,
			errContains: nil,
		},
		{
			name: "type coercion before validation",
			definitions: []validation.Input{
				{
					Name:     "value",
					Type:     "string",
					Required: true,
					Validation: &validation.Rules{
						Pattern: `^\d+$`,
					},
				},
			},
			inputs: map[string]any{
				"value": 123, // int coerced to string
			},
			wantErr:     false,
			errContains: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateInputs(tt.inputs, tt.definitions)

			if tt.wantErr {
				require.Error(t, err)
				for _, contains := range tt.errContains {
					assert.Contains(t, err.Error(), contains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Helper functions

func intPtr(i int) *int {
	return &i
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
