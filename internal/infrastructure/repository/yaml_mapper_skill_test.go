package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// mapSkillRefs Tests - String Elements (Name-Based References)

func TestMapSkillRefs_StringElement(t *testing.T) {
	skills := []any{"go-conventions"}

	got, err := mapSkillRefs(skills)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "go-conventions", got[0].Name)
	assert.Equal(t, "", got[0].Path)
}

func TestMapSkillRefs_MultipleStringElements(t *testing.T) {
	skills := []any{"go-conventions", "python-style"}

	got, err := mapSkillRefs(skills)

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "go-conventions", got[0].Name)
	assert.Equal(t, "python-style", got[1].Name)
}

// mapSkillRefs Tests - Map Elements (Path-Based References)

func TestMapSkillRefs_MapElementWithPath(t *testing.T) {
	skills := []any{map[string]any{"path": "./custom/audit"}}

	got, err := mapSkillRefs(skills)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "", got[0].Name)
	assert.Equal(t, "./custom/audit", got[0].Path)
}

func TestMapSkillRefs_MultipleMapElements(t *testing.T) {
	skills := []any{
		map[string]any{"path": "./custom/audit"},
		map[string]any{"path": "/abs/path/skill"},
	}

	got, err := mapSkillRefs(skills)

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "./custom/audit", got[0].Path)
	assert.Equal(t, "/abs/path/skill", got[1].Path)
}

// mapSkillRefs Tests - Mixed Elements

func TestMapSkillRefs_MixedNameAndPath(t *testing.T) {
	skills := []any{"go-conventions", map[string]any{"path": "./custom/audit"}}

	got, err := mapSkillRefs(skills)

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "go-conventions", got[0].Name)
	assert.Equal(t, "./custom/audit", got[1].Path)
}

func TestMapSkillRefs_MixedMultipleElements(t *testing.T) {
	skills := []any{
		"skill1",
		map[string]any{"path": "./path1"},
		"skill2",
		map[string]any{"path": "./path2"},
	}

	got, err := mapSkillRefs(skills)

	require.NoError(t, err)
	require.Len(t, got, 4)
	assert.Equal(t, "skill1", got[0].Name)
	assert.Equal(t, "./path1", got[1].Path)
	assert.Equal(t, "skill2", got[2].Name)
	assert.Equal(t, "./path2", got[3].Path)
}

// mapSkillRefs Tests - Empty and Nil Lists

func TestMapSkillRefs_NilList(t *testing.T) {
	var skills []any

	got, err := mapSkillRefs(skills)

	assert.NoError(t, err)
	assert.Nil(t, got)
}

func TestMapSkillRefs_EmptyList(t *testing.T) {
	skills := []any{}

	got, err := mapSkillRefs(skills)

	assert.NoError(t, err)
	assert.Nil(t, got)
}

// mapSkillRefs Tests - Validation Errors

func TestMapSkillRefs_EmptyStringElement(t *testing.T) {
	skills := []any{""}

	_, err := mapSkillRefs(skills)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill[0]")
	assert.Contains(t, err.Error(), "empty skill name")
}

func TestMapSkillRefs_EmptyStringInMixedList(t *testing.T) {
	skills := []any{"go-conventions", ""}

	_, err := mapSkillRefs(skills)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill[1]")
	assert.Contains(t, err.Error(), "empty skill name")
}

func TestMapSkillRefs_MapMissingPathKey(t *testing.T) {
	skills := []any{map[string]any{"name": "something"}}

	_, err := mapSkillRefs(skills)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill[0]")
	assert.Contains(t, err.Error(), "must have 'path' key")
}

func TestMapSkillRefs_MapMissingPathKeyInMixedList(t *testing.T) {
	skills := []any{"skill1", map[string]any{"name": "something"}}

	_, err := mapSkillRefs(skills)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill[1]")
	assert.Contains(t, err.Error(), "must have 'path' key")
}

func TestMapSkillRefs_EmptyPathValue(t *testing.T) {
	skills := []any{map[string]any{"path": ""}}

	_, err := mapSkillRefs(skills)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill[0]")
	assert.Contains(t, err.Error(), "path must be a non-empty string")
}

func TestMapSkillRefs_EmptyPathValueInMixedList(t *testing.T) {
	skills := []any{"go-conventions", map[string]any{"path": ""}}

	_, err := mapSkillRefs(skills)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill[1]")
	assert.Contains(t, err.Error(), "path must be a non-empty string")
}

func TestMapSkillRefs_InvalidTypeInt(t *testing.T) {
	skills := []any{42}

	_, err := mapSkillRefs(skills)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill[0]")
	assert.Contains(t, err.Error(), "invalid type")
}

func TestMapSkillRefs_InvalidTypeBool(t *testing.T) {
	skills := []any{true}

	_, err := mapSkillRefs(skills)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill[0]")
	assert.Contains(t, err.Error(), "invalid type")
}

func TestMapSkillRefs_InvalidTypeInMixedList(t *testing.T) {
	skills := []any{"go-conventions", 42}

	_, err := mapSkillRefs(skills)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill[1]")
	assert.Contains(t, err.Error(), "invalid type")
}

// mapStep Integration Tests - Skills Field Population

func TestMapStep_SkillsStringElement(t *testing.T) {
	y := &yamlStep{
		Type:    "agent",
		Command: "test",
		Skills:  []any{"go-conventions"},
	}

	got, err := mapStep("test.yaml", "agent_step", y)

	require.NoError(t, err)
	require.Len(t, got.Skills, 1)
	assert.Equal(t, "go-conventions", got.Skills[0].Name)
}

func TestMapStep_SkillsMapElement(t *testing.T) {
	y := &yamlStep{
		Type:   "agent",
		Skills: []any{map[string]any{"path": "./custom/skill"}},
	}

	got, err := mapStep("test.yaml", "agent_step", y)

	require.NoError(t, err)
	require.Len(t, got.Skills, 1)
	assert.Equal(t, "./custom/skill", got.Skills[0].Path)
}

func TestMapStep_SkillsMixedElements(t *testing.T) {
	y := &yamlStep{
		Type:   "agent",
		Skills: []any{"go-conventions", map[string]any{"path": "./custom/audit"}},
	}

	got, err := mapStep("test.yaml", "agent_step", y)

	require.NoError(t, err)
	require.Len(t, got.Skills, 2)
	assert.Equal(t, "go-conventions", got.Skills[0].Name)
	assert.Equal(t, "./custom/audit", got.Skills[1].Path)
}

func TestMapStep_SkillsNil(t *testing.T) {
	y := &yamlStep{
		Type:   "agent",
		Skills: nil,
	}

	got, err := mapStep("test.yaml", "agent_step", y)

	require.NoError(t, err)
	assert.Nil(t, got.Skills)
}

func TestMapStep_SkillsEmpty(t *testing.T) {
	y := &yamlStep{
		Type:   "agent",
		Skills: []any{},
	}

	got, err := mapStep("test.yaml", "agent_step", y)

	require.NoError(t, err)
	assert.Nil(t, got.Skills)
}

// mapStep Tests - Skills Error Wrapping (ParseError with field path)

func TestMapStep_SkillsErrorWrappedAsParseError_EmptyString(t *testing.T) {
	y := &yamlStep{
		Type:   "agent",
		Skills: []any{""},
	}

	_, err := mapStep("test.yaml", "agent_step", y)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "states.agent_step.skills")
	assert.Contains(t, err.Error(), "empty skill name")
}

func TestMapStep_SkillsErrorWrappedAsParseError_InvalidType(t *testing.T) {
	y := &yamlStep{
		Type:   "agent",
		Skills: []any{42},
	}

	_, err := mapStep("test.yaml", "agent_step", y)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "states.agent_step.skills")
	assert.Contains(t, err.Error(), "invalid type")
}

func TestMapStep_SkillsErrorWrappedAsParseError_MissingPath(t *testing.T) {
	y := &yamlStep{
		Type:   "agent",
		Skills: []any{map[string]any{"name": "skill"}},
	}

	_, err := mapStep("test.yaml", "agent_step", y)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "states.agent_step.skills")
	assert.Contains(t, err.Error(), "must have 'path' key")
}

// YAML Roundtrip Test - Parse from YAML bytes and verify Skills populated

func TestMapStep_YAMLRoundtrip_StringSkills(t *testing.T) {
	yamlBytes := []byte(`
type: agent
provider: claude
prompt: "test prompt"
skills:
  - go-conventions
  - python-style
`)

	var y yamlStep
	err := yaml.Unmarshal(yamlBytes, &y)
	require.NoError(t, err)
	require.NotNil(t, y.Skills)
	require.Len(t, y.Skills, 2)

	got, err := mapStep("test.yaml", "agent_step", &y)

	require.NoError(t, err)
	require.Len(t, got.Skills, 2)
	assert.Equal(t, "go-conventions", got.Skills[0].Name)
	assert.Equal(t, "python-style", got.Skills[1].Name)
}

func TestMapStep_YAMLRoundtrip_PathSkills(t *testing.T) {
	yamlBytes := []byte(`
type: agent
provider: claude
prompt: "test prompt"
skills:
  - path: ./custom/audit
  - path: /abs/path/skill
`)

	var y yamlStep
	err := yaml.Unmarshal(yamlBytes, &y)
	require.NoError(t, err)
	require.NotNil(t, y.Skills)
	require.Len(t, y.Skills, 2)

	got, err := mapStep("test.yaml", "agent_step", &y)

	require.NoError(t, err)
	require.Len(t, got.Skills, 2)
	assert.Equal(t, "./custom/audit", got.Skills[0].Path)
	assert.Equal(t, "/abs/path/skill", got.Skills[1].Path)
}

func TestMapStep_YAMLRoundtrip_MixedSkills(t *testing.T) {
	yamlBytes := []byte(`
type: agent
provider: claude
prompt: "test prompt"
skills:
  - go-conventions
  - path: ./custom/audit
  - python-style
`)

	var y yamlStep
	err := yaml.Unmarshal(yamlBytes, &y)
	require.NoError(t, err)
	require.NotNil(t, y.Skills)
	require.Len(t, y.Skills, 3)

	got, err := mapStep("test.yaml", "agent_step", &y)

	require.NoError(t, err)
	require.Len(t, got.Skills, 3)
	assert.Equal(t, "go-conventions", got.Skills[0].Name)
	assert.Equal(t, "./custom/audit", got.Skills[1].Path)
	assert.Equal(t, "python-style", got.Skills[2].Name)
}

func TestMapStep_YAMLRoundtrip_NoSkills(t *testing.T) {
	yamlBytes := []byte(`
type: agent
provider: claude
prompt: "test prompt"
`)

	var y yamlStep
	err := yaml.Unmarshal(yamlBytes, &y)
	require.NoError(t, err)
	assert.Nil(t, y.Skills)

	got, err := mapStep("test.yaml", "agent_step", &y)

	require.NoError(t, err)
	assert.Nil(t, got.Skills)
}
