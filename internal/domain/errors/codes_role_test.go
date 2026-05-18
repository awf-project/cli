package errors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorCodeUserInputMissingRole_Exists(t *testing.T) {
	assert.Equal(t, ErrorCode("USER.INPUT.MISSING_ROLE"), ErrorCodeUserInputMissingRole)
}

func TestErrorCodeUserInputMissingRole_IsValid(t *testing.T) {
	assert.True(t, ErrorCodeUserInputMissingRole.IsValid())
}

func TestErrorCodeUserInputMissingRole_Category(t *testing.T) {
	assert.Equal(t, "USER", ErrorCodeUserInputMissingRole.Category())
}

func TestErrorCodeUserInputMissingRole_Subcategory(t *testing.T) {
	assert.Equal(t, "INPUT", ErrorCodeUserInputMissingRole.Subcategory())
}

func TestErrorCodeUserInputMissingRole_Specific(t *testing.T) {
	assert.Equal(t, "MISSING_ROLE", ErrorCodeUserInputMissingRole.Specific())
}

func TestErrorCodeUserInputMissingRole_ExitCode(t *testing.T) {
	assert.Equal(t, 1, ErrorCodeUserInputMissingRole.ExitCode())
}

func TestErrorCodeUserInputMissingRole_InUserInputCategory(t *testing.T) {
	assert.Equal(t, "USER", ErrorCodeUserInputMissingRole.Category())
	assert.Equal(t, "INPUT", ErrorCodeUserInputMissingRole.Subcategory())
}
