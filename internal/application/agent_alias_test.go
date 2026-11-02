package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/awf-project/cli/internal/infrastructure/agents"
)

func TestMergeAliasOptions(t *testing.T) {
	alias := map[string]any{"model": "opus", "effort": "high"}
	step := map[string]any{"model": "sonnet", "allowed_tools": "Read"}

	got := mergeAliasOptions(alias, step)

	assert.Equal(t, map[string]any{"model": "sonnet", "effort": "high", "allowed_tools": "Read"}, got, "the step wins on a conflict")
	assert.Equal(t, map[string]any{"model": "opus", "effort": "high"}, alias, "alias options are not modified")
	assert.Equal(t, map[string]any{"model": "sonnet", "allowed_tools": "Read"}, step, "step options are not modified")
}

func TestMergeAliasOptions_NoAlias(t *testing.T) {
	step := map[string]any{"model": "sonnet"}
	assert.Equal(t, step, mergeAliasOptions(nil, step))
	assert.Nil(t, mergeAliasOptions(nil, nil))
}

func TestMergeAliasOptions_NoStepOptions(t *testing.T) {
	assert.Equal(t, map[string]any{"model": "opus"}, mergeAliasOptions(map[string]any{"model": "opus"}, nil))
}

func TestResolveAgentAlias(t *testing.T) {
	r := agents.NewAgentRegistry()
	r.SetAliases(map[string]agents.Alias{"einstein": {Provider: "claude", Options: map[string]any{"model": "opus"}}})

	name, opts := resolveAgentAlias(r, "einstein")
	assert.Equal(t, "claude", name)
	assert.Equal(t, map[string]any{"model": "opus"}, opts)

	name, opts = resolveAgentAlias(r, "codex")
	assert.Equal(t, "codex", name)
	assert.Nil(t, opts)
}

func TestAgentAliasNames(t *testing.T) {
	r := agents.NewAgentRegistry()
	assert.Empty(t, agentAliasNames(r))

	r.SetAliases(map[string]agents.Alias{"b": {Provider: "claude"}, "a": {Provider: "codex"}})
	assert.Equal(t, []string{"a", "b"}, agentAliasNames(r))
}
