package cli

import (
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegisterBuiltins_UsesProvidedVersion verifies version is passed correctly
func TestRegisterBuiltins_UsesProvidedVersion(t *testing.T) {
	mockStore := mocks.NewMockPluginStateStore()
	svc := application.NewPluginService(nil, mockStore, nil)

	testVersion := "custom-version-1.2.3"
	registerBuiltins(svc, testVersion)

	github, _ := svc.GetPlugin("github")
	notify, _ := svc.GetPlugin("notify")
	http, _ := svc.GetPlugin("http")

	assert.Equal(t, testVersion, github.Manifest.Version)
	assert.Equal(t, testVersion, notify.Manifest.Version)
	assert.Equal(t, testVersion, http.Manifest.Version)
}

// TestRegisterBuiltins_WithEmptyVersion tests registration with empty version
func TestRegisterBuiltins_WithEmptyVersion(t *testing.T) {
	mockStore := mocks.NewMockPluginStateStore()
	svc := application.NewPluginService(nil, mockStore, nil)

	registerBuiltins(svc, "")

	github, ok := svc.GetPlugin("github")
	require.True(t, ok)
	assert.Equal(t, "", github.Manifest.Version)
}

// TestRegisterBuiltins_TableDriven tests all providers with consistent structure
func TestRegisterBuiltins_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		providerName    string
		expectedDesc    string
		expectedOpCount int
		expectedOps     []string
	}{
		{
			name:            "github provider",
			providerName:    "github",
			expectedDesc:    "GitHub operation provider",
			expectedOpCount: 8,
			expectedOps: []string{
				"github.get_issue",
				"github.get_pr",
				"github.create_pr",
				"github.create_issue",
				"github.add_labels",
				"github.list_comments",
				"github.add_comment",
				"github.batch",
			},
		},
		{
			name:            "notify provider",
			providerName:    "notify",
			expectedDesc:    "Notification operation provider",
			expectedOpCount: 1,
			expectedOps:     []string{"notify.send"},
		},
		{
			name:            "http provider",
			providerName:    "http",
			expectedDesc:    "HTTP operation provider",
			expectedOpCount: 1,
			expectedOps:     []string{"http.request"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := mocks.NewMockPluginStateStore()
			svc := application.NewPluginService(nil, mockStore, nil)

			registerBuiltins(svc, "1.0.0")

			plugin, ok := svc.GetPlugin(tt.providerName)
			require.True(t, ok, "provider should be registered")
			assert.Equal(t, tt.providerName, plugin.Manifest.Name)
			assert.Equal(t, tt.expectedDesc, plugin.Manifest.Description)
			assert.Equal(t, pluginmodel.PluginTypeBuiltin, plugin.Type)
			assert.Len(t, plugin.Operations, tt.expectedOpCount)
			assert.Equal(t, tt.expectedOps, plugin.Operations)
		})
	}
}
