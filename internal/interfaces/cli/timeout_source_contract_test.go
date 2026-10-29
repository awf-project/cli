package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type timeoutSourceState struct {
	Timeout string `yaml:"timeout"`
}

// TestMaintainedTimeoutSources_PreserveIntentionalLimits keeps explicit bounds
// in maintained fixtures distinct from the unlimited omitted-timeout default.
func TestMaintainedTimeoutSources_PreserveIntentionalLimits(t *testing.T) {
	templatePath := filepath.Join("..", "..", "..", "tests", "fixtures", "templates", "ai-analyze.yaml")
	templateData, err := os.ReadFile(templatePath) //nolint:gosec // Fixed repository fixture path.
	require.NoError(t, err)

	var template struct {
		Parameters []struct {
			Name    string `yaml:"name"`
			Default string `yaml:"default"`
		} `yaml:"parameters"`
		States map[string]yaml.Node `yaml:"states"`
	}
	require.NoError(t, yaml.Unmarshal(templateData, &template))
	var analyzeTemplateState timeoutSourceState
	analyzeTemplateNode := template.States["analyze"]
	require.NoError(t, analyzeTemplateNode.Decode(&analyzeTemplateState))
	assert.Equal(t, "120s", analyzeTemplateState.Timeout)

	templateDefaults := make(map[string]string, len(template.Parameters))
	for _, parameter := range template.Parameters {
		templateDefaults[parameter.Name] = parameter.Default
	}
	assert.Equal(t, "120s", templateDefaults["timeout"])

	workflowPath := filepath.Join("..", "..", "..", "tests", "fixtures", "workflows", "example-simple.yaml")
	workflowData, err := os.ReadFile(workflowPath) //nolint:gosec // Fixed repository fixture path.
	require.NoError(t, err)

	var workflow struct {
		States map[string]yaml.Node `yaml:"states"`
	}
	require.NoError(t, yaml.Unmarshal(workflowData, &workflow))

	for name, wantTimeout := range map[string]string{
		"validate": "5s",
		"extract":  "30s",
		"analyze":  "10s",
	} {
		var state timeoutSourceState
		stateNode := workflow.States[name]
		require.NoError(t, stateNode.Decode(&state))
		assert.Equal(t, wantTimeout, state.Timeout, "state %q must retain its explicit timeout", name)
	}
}
