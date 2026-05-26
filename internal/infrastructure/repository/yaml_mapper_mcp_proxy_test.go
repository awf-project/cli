package repository

import (
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestMapMCPProxy_Struct exercises the struct-level mapMCPProxy helper, which converts
// a yamlMCPProxy value to a domain MCPProxyConfig. All variations of the pointer-based
// intercept_builtins field are covered in a single table.
func TestMapMCPProxy_Struct(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name                  string
		input                 *yamlMCPProxy
		wantNil               bool
		wantEnable            bool
		wantInterceptBuiltins bool
		wantPluginToolCount   int
		wantFirstPlugin       string
		wantFirstPluginExpose []string
	}{
		{
			name:    "nil input returns nil config",
			input:   nil,
			wantNil: true,
		},
		{
			name: "enable=true with nil intercept_builtins defaults to true",
			input: &yamlMCPProxy{
				Enable:            true,
				InterceptBuiltins: nil,
				PluginTools:       nil,
			},
			wantNil:               false,
			wantEnable:            true,
			wantInterceptBuiltins: true,
			wantPluginToolCount:   0,
		},
		{
			name: "enable=true intercept_builtins=true explicit",
			input: &yamlMCPProxy{
				Enable:            true,
				InterceptBuiltins: &trueVal,
				PluginTools: []yamlPluginToolExpose{
					{
						Plugin: "kubernetes",
						Expose: []string{"kubectl_apply", "kubectl_get"},
					},
				},
			},
			wantNil:               false,
			wantEnable:            true,
			wantInterceptBuiltins: true,
			wantPluginToolCount:   1,
			wantFirstPlugin:       "kubernetes",
			wantFirstPluginExpose: []string{"kubectl_apply", "kubectl_get"},
		},
		{
			name: "enable=true intercept_builtins=false explicit is respected",
			input: &yamlMCPProxy{
				Enable:            true,
				InterceptBuiltins: &falseVal,
				PluginTools:       nil,
			},
			wantNil:               false,
			wantEnable:            true,
			wantInterceptBuiltins: false,
			wantPluginToolCount:   0,
		},
		{
			name: "enable=false with nil intercept_builtins does NOT default to true",
			input: &yamlMCPProxy{
				Enable:            false,
				InterceptBuiltins: nil,
				PluginTools:       nil,
			},
			wantNil:               false,
			wantEnable:            false,
			wantInterceptBuiltins: false,
			wantPluginToolCount:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mapMCPProxy(tt.input)

			require.NoError(t, err)

			if tt.wantNil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			assert.Equal(t, tt.wantEnable, result.Enable)
			assert.Equal(t, tt.wantInterceptBuiltins, result.InterceptBuiltins)
			require.Len(t, result.PluginTools, tt.wantPluginToolCount)

			if tt.wantPluginToolCount > 0 {
				assert.Equal(t, tt.wantFirstPlugin, result.PluginTools[0].Plugin)
				assert.Equal(t, tt.wantFirstPluginExpose, result.PluginTools[0].Expose)
			}
		})
	}
}

// TestMapMCPProxy_UnknownKeys verifies that the YAML decoder reports unknown keys with
// the UNKNOWN_KEY error code. Multiple unknown keys must all be accumulated — not just
// the first one encountered.
func TestMapMCPProxy_UnknownKeys(t *testing.T) {
	tests := []struct {
		name         string
		yamlStr      string
		wantErrCode  string
		wantKeyNames []string
	}{
		{
			name: "single unknown key reports key name and error code",
			yamlStr: `
enable: true
policy: bogus
`,
			wantErrCode:  string(errors.ErrorCodeUserMCPProxyUnknownKey),
			wantKeyNames: []string{"policy"},
		},
		{
			name: "multiple unknown keys all reported in single error",
			yamlStr: `
enable: true
unknown_key1: value
unknown_key2: other
`,
			wantErrCode:  string(errors.ErrorCodeUserMCPProxyUnknownKey),
			wantKeyNames: []string{"unknown_key1", "unknown_key2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := unmarshalMCPProxyYAML(t, tt.yamlStr)

			require.Error(t, err, "unknown key should produce an error")
			assert.Contains(t, err.Error(), tt.wantErrCode,
				"error must reference the UNKNOWN_KEY code")
			for _, key := range tt.wantKeyNames {
				assert.Contains(t, err.Error(), key,
					"error must name the offending key: %s", key)
			}
		})
	}
}

// unmarshalMCPProxyYAML is a test helper that decodes a YAML string into a yamlMCPProxy
// via yaml.v3's node-based pipeline, exercising UnmarshalYAML directly.
func unmarshalMCPProxyYAML(t *testing.T, yamlStr string) error {
	t.Helper()
	// We decode into a wrapper struct so that yaml.v3 invokes UnmarshalYAML on the field.
	type wrapper struct {
		MCP yamlMCPProxy `yaml:"mcp_proxy"`
	}
	var b strings.Builder
	b.WriteString("mcp_proxy:\n")
	for line := range strings.SplitSeq(strings.TrimLeft(yamlStr, "\n"), "\n") {
		if line != "" {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	var w wrapper
	return yaml.Unmarshal([]byte(b.String()), &w)
}
