package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/pkg/registry"
)

func TestParseExactReleaseTarget_AcceptsEmptyRawTargetOnlyWhenOptional(t *testing.T) {
	target, err := parseExactReleaseTarget("", true)
	require.NoError(t, err)

	assert.False(t, target.HasVersion)
	assert.Equal(t, registry.Version{}, target.Version)
	assert.Empty(t, target.Tag)
}

func TestParseExactReleaseTarget_RejectsEmptyRawTargetWhenExplicitVersionRequired(t *testing.T) {
	_, err := parseExactReleaseTarget("", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid release version "": version: empty string`)
}

func TestParseExactReleaseTarget_AcceptsExactSemVerTargets(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "parseExactReleaseTarget 1.2.3 returns exact release target",
			raw:  "1.2.3",
		},
		{
			name: "parseExactReleaseTarget v1.2.3 accepts v prefix",
			raw:  "v1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := parseExactReleaseTarget(tt.raw, false)
			require.NoError(t, err)

			assert.True(t, target.HasVersion)
			assert.Equal(t, registry.Version{Major: 1, Minor: 2, Patch: 3}, target.Version)
			assert.Equal(t, registry.NormalizeTag("v1.2.3"), target.Tag)
		})
	}
}

func TestParseExactReleaseTarget_RejectsNonExactTargetsBeforeReleaseLookup(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantMessage string
	}{
		{
			name:        "parseExactReleaseTarget latest returns invalid format",
			raw:         "latest",
			wantMessage: `invalid release version "latest": version: invalid format "latest"`,
		},
		{
			name:        "parseExactReleaseTarget range returns invalid format",
			raw:         ">=1.0.0",
			wantMessage: `invalid release version ">=1.0.0": version: invalid format ">=1.0.0"`,
		},
		{
			name:        "parseExactReleaseTarget partial version returns invalid format",
			raw:         "1.2",
			wantMessage: `invalid release version "1.2": version: invalid format "1.2"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseExactReleaseTarget(tt.raw, false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMessage)
		})
	}
}

func TestParseInstallReleaseTarget(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantRepo    string
		wantVersion bool
		wantTag     string
		wantErr     string
	}{
		{
			name:     "unversioned owner repo",
			source:   "owner/repo",
			wantRepo: "owner/repo",
		},
		{
			name:        "exact version suffix",
			source:      "owner/repo@1.2.3",
			wantRepo:    "owner/repo",
			wantVersion: true,
			wantTag:     "1.2.3",
		},
		{
			name:        "v-prefixed exact version suffix",
			source:      "owner/repo@v1.2.3",
			wantRepo:    "owner/repo",
			wantVersion: true,
			wantTag:     "1.2.3",
		},
		{
			name:    "colon version syntax",
			source:  "owner/repo:1.2.3",
			wantErr: "owner/repo:version syntax is not supported; use owner/repo@version",
		},
		{
			name:    "empty version suffix",
			source:  "owner/repo@",
			wantErr: `invalid release version "": version: empty string`,
		},
		{
			name:    "range version suffix",
			source:  "owner/repo@>=1.0.0",
			wantErr: `invalid release version ">=1.0.0": version: invalid format ">=1.0.0"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRepo, gotTarget, err := parseInstallReleaseTarget(tt.source)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantRepo, gotRepo)
			assert.Equal(t, tt.wantVersion, gotTarget.HasVersion)
			assert.Equal(t, tt.wantTag, gotTarget.Tag)
		})
	}
}

func TestSelectExactRelease_NoVersionSelection(t *testing.T) {
	releases := []registry.Release{
		{TagName: "v1.3.0-beta.1", Prerelease: true},
		{TagName: "v1.2.3", Prerelease: false},
		{TagName: "v1.1.0", Prerelease: false},
	}

	t.Run("selectExactRelease returns latest stable release when prereleases are excluded", func(t *testing.T) {
		target, err := parseExactReleaseTarget("", true)
		require.NoError(t, err)

		release, err := selectExactRelease(releases, target, false)
		require.NoError(t, err)
		assert.Equal(t, "v1.2.3", release.TagName)
	})

	t.Run("selectExactRelease includes prereleases only when caller allows prereleases", func(t *testing.T) {
		target, err := parseExactReleaseTarget("", true)
		require.NoError(t, err)

		release, err := selectExactRelease(releases, target, true)
		require.NoError(t, err)
		assert.Equal(t, "v1.3.0-beta.1", release.TagName)
	})
}

func TestSelectExactRelease_ReturnsExactMatchingReleaseRegardlessOfVPrefix(t *testing.T) {
	tests := []struct {
		name       string
		rawTarget  string
		releaseTag string
	}{
		{
			name:       "selectExactRelease matches bare target to v-prefixed release tag",
			rawTarget:  "1.2.3",
			releaseTag: "v1.2.3",
		},
		{
			name:       "selectExactRelease matches v-prefixed target to bare release tag",
			rawTarget:  "v1.2.3",
			releaseTag: "1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := parseExactReleaseTarget(tt.rawTarget, false)
			require.NoError(t, err)

			release, err := selectExactRelease([]registry.Release{
				{TagName: "v1.4.0", Prerelease: false},
				{TagName: tt.releaseTag, Prerelease: false},
			}, target, false)
			require.NoError(t, err)
			assert.Equal(t, tt.releaseTag, release.TagName)
		})
	}
}

func TestSelectExactRelease_ReturnsExactNotFoundError(t *testing.T) {
	target, err := parseExactReleaseTarget("1.2.3", false)
	require.NoError(t, err)

	_, err = selectExactRelease([]registry.Release{
		{TagName: "v1.4.0", Prerelease: false},
	}, target, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "release version 1.2.3 not found")
}

func TestSelectExactRelease_ReturnsNoStableReleasesFound(t *testing.T) {
	target, err := parseExactReleaseTarget("", true)
	require.NoError(t, err)

	_, err = selectExactRelease([]registry.Release{
		{TagName: "v1.3.0-beta.1", Prerelease: true},
		{TagName: "v1.2.3-rc.1", Prerelease: true},
	}, target, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no stable releases found")
}
