//go:build integration

package cli_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: F110

func TestRootVersionOutputAndLegacyVersionCommand_Integration(t *testing.T) {
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--version"})

	err := cmd.Execute()
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.Len(t, lines, 3)
	assert.Regexp(t, `^awf version .+`, lines[0])
	assert.Regexp(t, `^commit: .+`, lines[1])
	assert.Regexp(t, `^built: .+`, lines[2])

	cmd = cli.NewRootCommand()
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"version"})

	err = cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown command "version"`)
}

func TestVerboseShorthandIsRejected_Integration(t *testing.T) {
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"-v", "--version"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown shorthand flag: 'v'")
}

func TestUpgradePositionalVersionCheck_Integration(t *testing.T) {
	originalVersion := cli.Version
	cli.Version = "0.4.0"
	t.Cleanup(func() { cli.Version = originalVersion })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.URL.Path, "/releases")
		releases := []map[string]any{
			{"tag_name": "v1.0.0", "prerelease": false},
			{"tag_name": "v0.5.0", "prerelease": false},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(releases) //nolint:errcheck // controlled test response
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"upgrade", "0.5.0", "--check"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Update available: v0.5.0")
	assert.NotContains(t, out.String(), "v1.0.0")
}

func TestUpgradeRemovedVersionFlagDoesNotResolveReleases_Integration(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv("GITHUB_API_URL", server.URL)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"upgrade", "--version", "0.5.0"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag: --version")
	assert.Zero(t, hits.Load())
}

func TestInstallCommandsRejectInvalidVersionSuffixBeforeReleaseLookup_Integration(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "plugin latest",
			args:    []string{"plugin", "install", "testorg/awf-plugin-test-plugin@latest"},
			wantErr: `invalid release version "latest": version: invalid format "latest"`,
		},
		{
			name:    "plugin range",
			args:    []string{"plugin", "install", "testorg/awf-plugin-test-plugin@>=1.0.0"},
			wantErr: `invalid release version ">=1.0.0": version: invalid format ">=1.0.0"`,
		},
		{
			name:    "workflow empty suffix",
			args:    []string{"workflow", "install", "testorg/awf-workflow-speckit@"},
			wantErr: `invalid release version "": version: empty string`,
		},
		{
			name:    "workflow range",
			args:    []string{"workflow", "install", "testorg/awf-workflow-speckit@>=1.0.0"},
			wantErr: `invalid release version ">=1.0.0": version: invalid format ">=1.0.0"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			t.Setenv("GITHUB_API_URL", server.URL)

			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Zero(t, hits.Load())
		})
	}
}
