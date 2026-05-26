package workflow

import (
	"testing"

	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPProxyConfig_Validate_DisabledProxy(t *testing.T) {
	cfg := &MCPProxyConfig{
		Enable:            false,
		InterceptBuiltins: false,
		PluginTools:       []PluginToolExpose{},
	}

	errs := cfg.Validate()

	assert.Empty(t, errs, "should return empty slice when proxy is disabled")
}

func TestMCPProxyConfig_Validate_ValidCase2_InterceptBuiltinsOnly(t *testing.T) {
	cfg := &MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
		PluginTools:       []PluginToolExpose{},
	}

	errs := cfg.Validate()

	assert.Empty(t, errs, "should be valid when intercept_builtins is true with no plugin_tools")
}

func TestMCPProxyConfig_Validate_ValidCase3_WithPluginTools(t *testing.T) {
	cfg := &MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
		PluginTools: []PluginToolExpose{
			{
				Plugin: "k8s",
				Expose: []string{"apply", "get"},
			},
		},
	}

	errs := cfg.Validate()

	assert.Empty(t, errs, "should be valid when enable=true, intercept_builtins=true with plugin_tools")
}

func TestMCPProxyConfig_Validate_ValidCase4_InterceptFalseWithPluginTools(t *testing.T) {
	cfg := &MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: false,
		PluginTools: []PluginToolExpose{
			{
				Plugin: "k8s",
				Expose: []string{"apply"},
			},
		},
	}

	errs := cfg.Validate()

	assert.Empty(t, errs, "should be valid when enable=true, intercept_builtins=false with plugin_tools")
}

func TestMCPProxyConfig_Validate_EmptyProxy_Error(t *testing.T) {
	cfg := &MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: false,
		PluginTools:       []PluginToolExpose{},
	}

	errs := cfg.Validate()

	require.Len(t, errs, 1, "should return one error for empty proxy")
	assert.Equal(t, ValidationCode(domerrors.ErrorCodeUserMCPProxyEmptyProxy), errs[0].Code)
	assert.Equal(t, ValidationLevelError, errs[0].Level)
}

func TestMCPProxyConfig_Validate_DuplicatePluginNameCollision(t *testing.T) {
	cfg := &MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
		PluginTools: []PluginToolExpose{
			{
				Plugin: "k8s",
				Expose: []string{"apply"},
			},
			{
				Plugin: "k8s",
				Expose: []string{"get"},
			},
		},
	}

	errs := cfg.Validate()

	require.Len(t, errs, 1, "should return one error for duplicate plugin")
	assert.Equal(t, ValidationCode(domerrors.ErrorCodeUserMCPProxyNameCollision), errs[0].Code)
	assert.Equal(t, ValidationLevelError, errs[0].Level)
}

func TestMCPProxyConfig_Validate_MultiplePluginsNoCollision(t *testing.T) {
	cfg := &MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
		PluginTools: []PluginToolExpose{
			{
				Plugin: "k8s",
				Expose: []string{"apply"},
			},
			{
				Plugin: "docker",
				Expose: []string{"run", "stop"},
			},
			{
				Plugin: "git",
				Expose: []string{"clone"},
			},
		},
	}

	errs := cfg.Validate()

	assert.Empty(t, errs, "should be valid with multiple unique plugin names")
}

func TestMCPProxyConfig_Validate_ThreePluginsWithDuplicateInMiddle(t *testing.T) {
	cfg := &MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
		PluginTools: []PluginToolExpose{
			{
				Plugin: "k8s",
				Expose: []string{"apply"},
			},
			{
				Plugin: "docker",
				Expose: []string{"run"},
			},
			{
				Plugin: "k8s",
				Expose: []string{"get"},
			},
		},
	}

	errs := cfg.Validate()

	require.Len(t, errs, 1, "should detect duplicate even with other plugins between")
	assert.Equal(t, ValidationCode(domerrors.ErrorCodeUserMCPProxyNameCollision), errs[0].Code)
}
