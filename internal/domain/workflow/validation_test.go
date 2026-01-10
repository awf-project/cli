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
	v := InputValidation{Enum: []string{"dev", "staging", "prod"}}
	assert.Equal(t, []string{"dev", "staging", "prod"}, v.Enum)
}

func TestInputValidation_MinMax(t *testing.T) {
	minVal, maxVal := 1, 100
	v := InputValidation{Min: &minVal, Max: &maxVal}
	assert.Equal(t, 1, *v.Min)
	assert.Equal(t, 100, *v.Max)
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
