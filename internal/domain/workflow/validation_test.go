package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInputValidation_Empty(t *testing.T) {
	v := InputValidation{}
	assert.Empty(t, v.Pattern)
	assert.Nil(t, v.Enum)
	assert.Nil(t, v.Min)
	assert.Nil(t, v.Max)
	assert.False(t, v.FileExists)
	assert.Nil(t, v.FileExtension)
}

func TestInputValidation_Pattern(t *testing.T) {
	v := InputValidation{Pattern: `^[a-z]+$`}
	assert.Equal(t, `^[a-z]+$`, v.Pattern)
}

func TestInputValidation_Enum(t *testing.T) {
	tests := []struct {
		name     string
		enum     []string
		expected []string
	}{
		{
			name:     "valid_values",
			enum:     []string{"dev", "staging", "prod"},
			expected: []string{"dev", "staging", "prod"},
		},
		{
			name:     "empty_enum_list",
			enum:     []string{},
			expected: []string{},
		},
		{
			name:     "single_value",
			enum:     []string{"prod"},
			expected: []string{"prod"},
		},
		{
			name:     "case_sensitive_values",
			enum:     []string{"dev", "DEV", "Dev"},
			expected: []string{"dev", "DEV", "Dev"},
		},
		{
			name:     "nil_enum",
			enum:     nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := InputValidation{Enum: tt.enum}
			assert.Equal(t, tt.expected, v.Enum)
		})
	}
}

func TestInputValidation_MinMax(t *testing.T) {
	tests := []struct {
		name        string
		min         *int
		max         *int
		expectedMin *int
		expectedMax *int
	}{
		{
			name:        "both_min_and_max",
			min:         intPtr(1),
			max:         intPtr(100),
			expectedMin: intPtr(1),
			expectedMax: intPtr(100),
		},
		{
			name:        "only_min",
			min:         intPtr(1),
			max:         nil,
			expectedMin: intPtr(1),
			expectedMax: nil,
		},
		{
			name:        "only_max",
			min:         nil,
			max:         intPtr(100),
			expectedMin: nil,
			expectedMax: intPtr(100),
		},
		{
			name:        "neither_min_nor_max",
			min:         nil,
			max:         nil,
			expectedMin: nil,
			expectedMax: nil,
		},
		{
			name:        "negative_min",
			min:         intPtr(-10),
			max:         intPtr(10),
			expectedMin: intPtr(-10),
			expectedMax: intPtr(10),
		},
		{
			name:        "zero_min",
			min:         intPtr(0),
			max:         intPtr(100),
			expectedMin: intPtr(0),
			expectedMax: intPtr(100),
		},
		{
			name:        "equal_min_max",
			min:         intPtr(50),
			max:         intPtr(50),
			expectedMin: intPtr(50),
			expectedMax: intPtr(50),
		},
		{
			name:        "large_range",
			min:         intPtr(-1000),
			max:         intPtr(1000),
			expectedMin: intPtr(-1000),
			expectedMax: intPtr(1000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := InputValidation{Min: tt.min, Max: tt.max}
			if tt.expectedMin == nil {
				assert.Nil(t, v.Min)
			} else {
				assert.NotNil(t, v.Min)
				assert.Equal(t, *tt.expectedMin, *v.Min)
			}
			if tt.expectedMax == nil {
				assert.Nil(t, v.Max)
			} else {
				assert.NotNil(t, v.Max)
				assert.Equal(t, *tt.expectedMax, *v.Max)
			}
		})
	}
}

// intPtr returns a pointer to an int value
func intPtr(i int) *int {
	return &i
}

func TestInputValidation_FileExists(t *testing.T) {
	v := InputValidation{FileExists: true}
	assert.True(t, v.FileExists)
}

func TestInputValidation_FileExtension(t *testing.T) {
	v := InputValidation{FileExtension: []string{".go", ".yaml"}}
	assert.Equal(t, []string{".go", ".yaml"}, v.FileExtension)
}

func TestInputValidation_Full(t *testing.T) {
	minVal := 0
	v := InputValidation{
		Pattern:       `^[a-zA-Z0-9_]+$`,
		Enum:          []string{"alpha", "beta"},
		Min:           &minVal,
		FileExists:    true,
		FileExtension: []string{".txt"},
	}
	assert.NotEmpty(t, v.Pattern)
	assert.Len(t, v.Enum, 2)
	assert.Equal(t, 0, *v.Min)
	assert.Nil(t, v.Max)
	assert.True(t, v.FileExists)
	assert.Len(t, v.FileExtension, 1)
}
