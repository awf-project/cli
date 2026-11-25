package repository

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMapToDomain_Requires(t *testing.T) {
	var y yamlWorkflow
	require.NoError(t, yaml.Unmarshal([]byte(`
name: needs
requires:
  plugins:
    nexus: ">=0.1.0"
  agent_plugins:
    claude: ["egm@egm"]
states:
  initial: done
  done:
    type: terminal
`), &y))

	wf, err := mapToDomain("/workflows/needs.yaml", &y)

	require.NoError(t, err)
	assert.Equal(t, workflow.Requirements{
		Plugins:      map[string]string{"nexus": ">=0.1.0"},
		AgentPlugins: map[string][]string{"claude": {"egm@egm"}},
	}, wf.Requires)
}

func TestMapToDomain_NoRequires(t *testing.T) {
	var y yamlWorkflow
	require.NoError(t, yaml.Unmarshal([]byte("name: plain\nstates:\n  initial: done\n  done:\n    type: terminal\n"), &y))

	wf, err := mapToDomain("/workflows/plain.yaml", &y)

	require.NoError(t, err)
	assert.True(t, wf.Requires.IsEmpty())
}
