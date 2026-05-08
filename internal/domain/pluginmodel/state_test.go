package pluginmodel_test

import (
	"encoding/json"
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

func TestNewPluginState_ChecksumEmptyAndZeroByDefault(t *testing.T) {
	state := pluginmodel.NewPluginState()

	assert.Empty(t, state.Checksum)
	assert.Zero(t, state.ChecksumAt)
}

func TestPluginState_JSONRoundtrip_PreservesChecksumFields(t *testing.T) {
	state := &pluginmodel.PluginState{
		Enabled:    true,
		Checksum:   "a3f5b2c8d1e4f7a0b3c6d9e2f5a8b1c4d7e0f3a6b9c2d5e8f1a4b7c0d3e6f9a2",
		ChecksumAt: 1735689600,
	}

	data, err := json.Marshal(state)
	require.NoError(t, err)

	var restored pluginmodel.PluginState
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, state.Checksum, restored.Checksum)
	assert.Equal(t, state.ChecksumAt, restored.ChecksumAt)
}

func TestPluginState_JSONUnmarshal_BackwardCompatWithoutChecksumFields(t *testing.T) {
	payload := `{"enabled":true,"config":{"key":"value"}}`

	var state pluginmodel.PluginState
	err := json.Unmarshal([]byte(payload), &state)
	require.NoError(t, err)

	assert.Empty(t, state.Checksum)
	assert.Zero(t, state.ChecksumAt)
}

func TestPluginState_JSONMarshal_OmitsChecksumFieldsWhenEmpty(t *testing.T) {
	state := pluginmodel.NewPluginState()

	data, err := json.Marshal(state)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	_, hasChecksum := raw["checksum"]
	_, hasChecksumAt := raw["checksum_at"]
	assert.False(t, hasChecksum, "checksum field should be omitted when empty")
	assert.False(t, hasChecksumAt, "checksum_at field should be omitted when zero")
}
