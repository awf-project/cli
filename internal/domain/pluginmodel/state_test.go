package pluginmodel_test

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPluginState_DefaultsEnabledWithEmptyConfig(t *testing.T) {
	state := pluginmodel.NewPluginState()

	require.NotNil(t, state)
	assert.True(t, state.Enabled)
	assert.NotNil(t, state.Config)
	assert.Empty(t, state.Config)
}

func TestNewPluginState_SourceDataNilByDefault(t *testing.T) {
	state := pluginmodel.NewPluginState()

	assert.Nil(t, state.SourceData)
}

func TestPluginState_SourceDataAcceptsArbitraryMetadata(t *testing.T) {
	state := pluginmodel.NewPluginState()
	state.SourceData = map[string]any{
		"owner":       "awf-project",
		"repo":        "awf-plugin-echo",
		"stars":       42,
		"description": "Echo plugin for AWF",
	}

	assert.Equal(t, "awf-project", state.SourceData["owner"])
	assert.Equal(t, "awf-plugin-echo", state.SourceData["repo"])
	assert.Equal(t, 42, state.SourceData["stars"])
}
