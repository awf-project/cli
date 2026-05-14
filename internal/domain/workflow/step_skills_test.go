package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/awf-project/cli/internal/domain/workflow"
)

func TestStepSkillsField_Exists(t *testing.T) {
	step := workflow.Step{
		Name:   "agent-step",
		Type:   workflow.StepTypeAgent,
		Agent:  &workflow.AgentConfig{Provider: "claude"},
		Skills: []workflow.SkillReference{},
	}

	assert.NotNil(t, step.Skills)
	assert.IsType(t, []workflow.SkillReference{}, step.Skills)
}

func TestStepSkillsField_MultipleSkills(t *testing.T) {
	skills := []workflow.SkillReference{
		{Name: "skill1"},
		{Name: "skill2"},
		{Path: "/path/to/skill3"},
	}

	step := workflow.Step{
		Name:   "agent-step",
		Type:   workflow.StepTypeAgent,
		Agent:  &workflow.AgentConfig{Provider: "claude"},
		Skills: skills,
	}

	assert.Equal(t, 3, len(step.Skills))
	assert.Equal(t, "skill1", step.Skills[0].Name)
	assert.Equal(t, "skill2", step.Skills[1].Name)
	assert.Equal(t, "/path/to/skill3", step.Skills[2].Path)
}

func TestStepSkillsField_AfterAgentField(t *testing.T) {
	step := workflow.Step{
		Name:   "agent-step",
		Type:   workflow.StepTypeAgent,
		Agent:  &workflow.AgentConfig{Provider: "claude"},
		Skills: []workflow.SkillReference{{Name: "test-skill"}},
	}

	assert.NotNil(t, step.Agent)
	assert.Equal(t, "claude", step.Agent.Provider)
	assert.NotNil(t, step.Skills)
	assert.Equal(t, 1, len(step.Skills))
}

func TestStepSkillsField_EmptySlice(t *testing.T) {
	step := workflow.Step{
		Name:   "agent-step",
		Type:   workflow.StepTypeAgent,
		Agent:  &workflow.AgentConfig{Provider: "claude"},
		Skills: []workflow.SkillReference{},
	}

	assert.NotNil(t, step.Skills)
	assert.Equal(t, 0, len(step.Skills))
}

func TestSkillReference_IsPathBased(t *testing.T) {
	tests := []struct {
		name     string
		ref      workflow.SkillReference
		wantPath bool
	}{
		{
			name:     "name-based reference",
			ref:      workflow.SkillReference{Name: "skill1"},
			wantPath: false,
		},
		{
			name:     "path-based reference",
			ref:      workflow.SkillReference{Path: "/path/to/skill"},
			wantPath: true,
		},
		{
			name:     "empty reference",
			ref:      workflow.SkillReference{},
			wantPath: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ref.IsPathBased()
			assert.Equal(t, tt.wantPath, got)
		})
	}
}

func TestStepValidation_WithSkills(t *testing.T) {
	tests := []struct {
		name    string
		step    workflow.Step
		wantErr bool
	}{
		{
			name: "agent step with skills",
			step: workflow.Step{
				Name:   "agent-step",
				Type:   workflow.StepTypeAgent,
				Agent:  &workflow.AgentConfig{Provider: "claude", Prompt: "test prompt"},
				Skills: []workflow.SkillReference{{Name: "skill1"}},
			},
			wantErr: false,
		},
		{
			name: "agent step with multiple skills",
			step: workflow.Step{
				Name:  "agent-step",
				Type:  workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{Provider: "claude", Prompt: "test prompt"},
				Skills: []workflow.SkillReference{
					{Name: "skill1"},
					{Path: "/path/to/skill2"},
				},
			},
			wantErr: false,
		},
		{
			name: "command step with skills (allowed, no validation error)",
			step: workflow.Step{
				Name:    "cmd-step",
				Type:    workflow.StepTypeCommand,
				Command: "echo hello",
				Skills:  []workflow.SkillReference{{Name: "skill1"}},
			},
			wantErr: false,
		},
	}

	mockValidator := workflow.ExpressionCompiler(func(_ string) error {
		return nil
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate(mockValidator, nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStepSkillsIndependentOfStepType(t *testing.T) {
	stepTypes := []workflow.StepType{
		workflow.StepTypeCommand,
		workflow.StepTypeParallel,
		workflow.StepTypeAgent,
		workflow.StepTypeOperation,
	}

	for _, st := range stepTypes {
		t.Run(string(st), func(t *testing.T) {
			step := workflow.Step{
				Name:   "test",
				Type:   st,
				Skills: []workflow.SkillReference{{Name: "test-skill"}},
			}

			assert.Equal(t, 1, len(step.Skills))
			assert.Equal(t, "test-skill", step.Skills[0].Name)
		})
	}
}
