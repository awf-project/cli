package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/infrastructure/config"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
)

func TestBuildBuiltinProviders_HappyPath(t *testing.T) {
	pluginSvc := application.NewPluginService(
		nil, // no RPCPluginManager
		pluginmgr.NewJSONPluginStateStore(t.TempDir()+"/plugins"),
		&mockLogger{},
	)
	projectCfg := &config.ProjectConfig{}
	logger := &mockLogger{}

	provider, err := buildBuiltinProviders(pluginSvc, projectCfg, logger)

	require.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestBuildBuiltinProviders_WithNilProjectConfig(t *testing.T) {
	pluginSvc := application.NewPluginService(
		nil, // no RPCPluginManager
		pluginmgr.NewJSONPluginStateStore(t.TempDir()+"/plugins"),
		&mockLogger{},
	)
	logger := &mockLogger{}

	provider, err := buildBuiltinProviders(pluginSvc, nil, logger)

	// Should handle nil config gracefully (use empty config defaults)
	require.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestBuildBuiltinProviders_WithProjectConfigInputs(t *testing.T) {
	pluginSvc := application.NewPluginService(
		nil, // no RPCPluginManager
		pluginmgr.NewJSONPluginStateStore(t.TempDir()+"/plugins"),
		&mockLogger{},
	)
	projectCfg := &config.ProjectConfig{
		Inputs: map[string]any{
			"test": "value",
		},
	}
	logger := &mockLogger{}

	provider, err := buildBuiltinProviders(pluginSvc, projectCfg, logger)

	require.NoError(t, err)
	assert.NotNil(t, provider)
}
