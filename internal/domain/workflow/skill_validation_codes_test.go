package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSkillValidationCodes_Exist(t *testing.T) {
	tests := []struct {
		name     string
		code     ValidationCode
		expected string
	}{
		{
			name:     "ErrSkillNotFound",
			code:     ErrSkillNotFound,
			expected: "skill_not_found",
		},
		{
			name:     "ErrSkillMissingSkillMD",
			code:     ErrSkillMissingSkillMD,
			expected: "skill_missing_skillmd",
		},
		{
			name:     "ErrSkillEmptyContent",
			code:     ErrSkillEmptyContent,
			expected: "skill_empty_content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, string(tt.code))
			assert.Equal(t, tt.expected, string(tt.code))
		})
	}
}

func TestSkillValidationCodes_UniqueValues(t *testing.T) {
	codes := []ValidationCode{
		ErrSkillNotFound,
		ErrSkillMissingSkillMD,
		ErrSkillEmptyContent,
	}

	seen := make(map[string]bool)
	for _, code := range codes {
		strCode := string(code)
		assert.False(t, seen[strCode], "duplicate validation code: %s", strCode)
		seen[strCode] = true
	}
}

func TestValidationError_SkillNotFound(t *testing.T) {
	err := ValidationError{
		Level:   ValidationLevelError,
		Code:    ErrSkillNotFound,
		Message: "skill 'helpers' not found in skill repository",
		Path:    "states.agent_step.skills[0]",
	}

	assert.True(t, err.IsError())
	assert.Equal(t, ErrSkillNotFound, err.Code)
	assert.Contains(t, err.Error(), "error")
	assert.Contains(t, err.Error(), "states.agent_step.skills[0]")
}

func TestValidationError_SkillMissingSKILLMD(t *testing.T) {
	err := ValidationError{
		Level:   ValidationLevelWarning,
		Code:    ErrSkillMissingSkillMD,
		Message: "skill 'utils' missing SKILL.md documentation",
		Path:    "states.agent_step.skills[1]",
	}

	assert.False(t, err.IsError())
	assert.Equal(t, ValidationLevelWarning, err.Level)
	assert.Equal(t, ErrSkillMissingSkillMD, err.Code)
}

func TestValidationError_SkillEmptyContent(t *testing.T) {
	err := ValidationError{
		Level:   ValidationLevelError,
		Code:    ErrSkillEmptyContent,
		Message: "skill 'fetch' has empty content",
		Path:    "states.agent_step.skills[0]",
	}

	assert.True(t, err.IsError())
	assert.Equal(t, ErrSkillEmptyContent, err.Code)
	assert.Contains(t, err.Error(), "skill 'fetch' has empty content")
}

func TestValidationResult_WithSkillErrors(t *testing.T) {
	result := &ValidationResult{}

	result.AddError(ErrSkillNotFound, "states.agent.skills[0]", "skill 'helpers' not found")
	result.AddError(ErrSkillEmptyContent, "states.agent.skills[1]", "skill content is empty")
	result.AddWarning(ErrSkillMissingSkillMD, "states.agent.skills[0]", "missing documentation")

	assert.True(t, result.HasErrors())
	assert.True(t, result.HasWarnings())
	assert.Equal(t, 2, len(result.Errors))
	assert.Equal(t, 1, len(result.Warnings))
	assert.Equal(t, 3, len(result.AllIssues()))
}
