package pluginmodel_test

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestCapabilityEvents_Value(t *testing.T) {
	assert.Equal(t, "events", pluginmodel.CapabilityEvents)
}

func TestValidCapabilities_ContainsEvents(t *testing.T) {
	assert.Contains(t, pluginmodel.ValidCapabilities, pluginmodel.CapabilityEvents)
}

func TestManifest_HasCapabilityEvents(t *testing.T) {
	m := pluginmodel.Manifest{
		Name:         "test-plugin",
		Version:      "1.0.0",
		AWFVersion:   ">=0.4.0",
		Capabilities: []string{pluginmodel.CapabilityEvents},
	}
	assert.True(t, m.HasCapability(pluginmodel.CapabilityEvents))
}

func TestManifestValidate_WithEventsCapability(t *testing.T) {
	tests := []struct {
		name     string
		manifest pluginmodel.Manifest
		wantErr  bool
	}{
		{
			name: "events capability with subscribe and emit",
			manifest: pluginmodel.Manifest{
				Name:         "test-plugin",
				Version:      "1.0.0",
				AWFVersion:   ">=0.4.0",
				Capabilities: []string{pluginmodel.CapabilityEvents},
				Events: pluginmodel.ManifestEvents{
					Subscribe: []string{"workflow.completed", "step.failed"},
					Emit:      []string{"notification.sent"},
				},
			},
			wantErr: false,
		},
		{
			name: "events capability with empty subscribe and emit",
			manifest: pluginmodel.Manifest{
				Name:         "test-plugin",
				Version:      "1.0.0",
				AWFVersion:   ">=0.4.0",
				Capabilities: []string{pluginmodel.CapabilityEvents},
				Events:       pluginmodel.ManifestEvents{},
			},
			wantErr: false,
		},
		{
			name: "events capability alongside other capabilities",
			manifest: pluginmodel.Manifest{
				Name:       "test-plugin",
				Version:    "1.0.0",
				AWFVersion: ">=0.4.0",
				Capabilities: []string{
					pluginmodel.CapabilityOperations,
					pluginmodel.CapabilityEvents,
				},
				Events: pluginmodel.ManifestEvents{
					Subscribe: []string{"workflow.completed"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestManifestValidate_WithoutEventsCapabilityButEventsField(t *testing.T) {
	m := pluginmodel.Manifest{
		Name:       "test-plugin",
		Version:    "1.0.0",
		AWFVersion: ">=0.4.0",
		Events: pluginmodel.ManifestEvents{
			Subscribe: []string{"workflow.completed"},
			Emit:      []string{"notification.sent"},
		},
	}
	err := m.Validate()
	assert.NoError(t, err, "events field without events capability should still validate")
}

func TestManifestEvents_YAMLUnmarshal(t *testing.T) {
	input := `
name: test-plugin
version: 1.0.0
awf_version: ">=0.4.0"
capabilities:
  - events
events:
  subscribe:
    - workflow.completed
    - step.failed
  emit:
    - notification.sent
    - audit.logged
`
	var m pluginmodel.Manifest
	require.NoError(t, yaml.Unmarshal([]byte(input), &m))
	assert.Equal(t, []string{"workflow.completed", "step.failed"}, m.Events.Subscribe)
	assert.Equal(t, []string{"notification.sent", "audit.logged"}, m.Events.Emit)
}

func TestManifestEvents_YAMLUnmarshal_EmptyLists(t *testing.T) {
	input := `
name: test-plugin
version: 1.0.0
awf_version: ">=0.4.0"
events:
  subscribe: []
  emit: []
`
	var m pluginmodel.Manifest
	require.NoError(t, yaml.Unmarshal([]byte(input), &m))
	assert.Empty(t, m.Events.Subscribe)
	assert.Empty(t, m.Events.Emit)
}

func TestManifestEvents_YAMLUnmarshal_NoEventsField(t *testing.T) {
	input := `
name: test-plugin
version: 1.0.0
awf_version: ">=0.4.0"
`
	var m pluginmodel.Manifest
	require.NoError(t, yaml.Unmarshal([]byte(input), &m))
	assert.Nil(t, m.Events.Subscribe)
	assert.Nil(t, m.Events.Emit)
}
