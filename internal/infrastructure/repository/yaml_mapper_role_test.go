package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMapAgentConfigFlat_RolePresent(t *testing.T) {
	y := &yamlStep{
		Type:     "agent",
		Provider: "claude",
		Prompt:   "hello",
		Role:     "go-senior",
	}

	got := mapAgentConfigFlat(y)

	require.NotNil(t, got)
	assert.Equal(t, "go-senior", got.Role)
}

func TestMapAgentConfigFlat_RoleAbsent(t *testing.T) {
	y := &yamlStep{
		Type:     "agent",
		Provider: "claude",
		Prompt:   "hello",
	}

	got := mapAgentConfigFlat(y)

	require.NotNil(t, got)
	assert.Equal(t, "", got.Role)
}

func TestMapAgentConfigFlat_RolePath(t *testing.T) {
	y := &yamlStep{
		Type:     "agent",
		Provider: "claude",
		Prompt:   "hello",
		Role:     "./roles/backend",
	}

	got := mapAgentConfigFlat(y)

	require.NotNil(t, got)
	assert.Equal(t, "./roles/backend", got.Role)
}

func TestMapAgentConfigFlat_RoleTemplateExpression(t *testing.T) {
	y := &yamlStep{
		Type:     "agent",
		Provider: "claude",
		Prompt:   "hello",
		Role:     "{{inputs.persona}}",
	}

	got := mapAgentConfigFlat(y)

	require.NotNil(t, got)
	assert.Equal(t, "{{inputs.persona}}", got.Role)
}

func TestMapStep_YAMLRoundtrip_RolePresent(t *testing.T) {
	yamlBytes := []byte(`
type: agent
provider: claude
prompt: "hello"
role: go-senior
`)

	var y yamlStep
	err := yaml.Unmarshal(yamlBytes, &y)
	require.NoError(t, err)
	assert.Equal(t, "go-senior", y.Role)

	got, err := mapStep("test.yaml", "agent_step", &y)

	require.NoError(t, err)
	require.NotNil(t, got.Agent)
	assert.Equal(t, "go-senior", got.Agent.Role)
}

func TestMapStep_YAMLRoundtrip_RoleAbsent(t *testing.T) {
	yamlBytes := []byte(`
type: agent
provider: claude
prompt: "hello"
`)

	var y yamlStep
	err := yaml.Unmarshal(yamlBytes, &y)
	require.NoError(t, err)
	assert.Equal(t, "", y.Role)

	got, err := mapStep("test.yaml", "agent_step", &y)

	require.NoError(t, err)
	require.NotNil(t, got.Agent)
	assert.Equal(t, "", got.Agent.Role)
}
