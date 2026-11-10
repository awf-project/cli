package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveAlias(t *testing.T) {
	r := NewAgentRegistry()
	r.SetAliases(map[string]Alias{
		"einstein": {Provider: "claude", Options: map[string]any{"model": "opus"}},
		"codex":    {Provider: "claude"},
	})

	provider, options := r.ResolveAlias("einstein")
	assert.Equal(t, "claude", provider)
	assert.Equal(t, map[string]any{"model": "opus"}, options)

	provider, options = r.ResolveAlias("gemini")
	assert.Equal(t, "gemini", provider, "a name that is not an alias is returned unchanged")
	assert.Nil(t, options)

	provider, _ = r.ResolveAlias("codex")
	assert.Equal(t, "claude", provider, "an alias shadows the provider of the same name")
}

func TestGetUnknownProviderListsAliases(t *testing.T) {
	r := NewAgentRegistry()
	_, err := r.Get("nope")
	require.Error(t, err)
	assert.Equal(t, `provider "nope" not found`, err.Error(), "no alias: message unchanged")

	r.SetAliases(map[string]Alias{"sklodowska": {Provider: "mistral_vibe"}, "einstein": {Provider: "claude"}})
	_, err = r.Get("nope")
	require.Error(t, err)
	assert.Equal(t, `provider "nope" not found (known aliases: einstein, sklodowska)`, err.Error())
}
