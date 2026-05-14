package errors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorCodeUserInputMissingSkill_Exists(t *testing.T) {
	assert.Equal(t, ErrorCode("USER.INPUT.MISSING_SKILL"), ErrorCodeUserInputMissingSkill)
}

func TestErrorCodeUserInputMissingSkill_IsValid(t *testing.T) {
	assert.True(t, ErrorCodeUserInputMissingSkill.IsValid())
}

func TestErrorCodeUserInputMissingSkill_Category(t *testing.T) {
	assert.Equal(t, "USER", ErrorCodeUserInputMissingSkill.Category())
}

func TestErrorCodeUserInputMissingSkill_Subcategory(t *testing.T) {
	assert.Equal(t, "INPUT", ErrorCodeUserInputMissingSkill.Subcategory())
}

func TestErrorCodeUserInputMissingSkill_Specific(t *testing.T) {
	assert.Equal(t, "MISSING_SKILL", ErrorCodeUserInputMissingSkill.Specific())
}

func TestErrorCodeUserInputMissingSkill_Format(t *testing.T) {
	code := ErrorCodeUserInputMissingSkill
	assert.Equal(t, "USER.INPUT.MISSING_SKILL", string(code))
}

func TestErrorCodeUserInputMissingSkill_InUserInputCategory(t *testing.T) {
	assert.Equal(t, "USER", ErrorCodeUserInputMissingSkill.Category())
	otherCodes := []ErrorCode{
		ErrorCodeUserInputMissingFile,
		ErrorCodeUserInputInvalidFormat,
		ErrorCodeUserInputValidationFailed,
	}
	for _, code := range otherCodes {
		assert.Equal(t, "USER", code.Category())
		assert.Equal(t, "INPUT", code.Subcategory())
	}
}
