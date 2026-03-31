package workflowpkg

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackSourceRoundtrip(t *testing.T) {
	tests := []struct {
		name   string
		source *PackSource
	}{
		{
			name: "valid pack source",
			source: &PackSource{
				Repository:  "owner/awf-workflow-example",
				Version:     "1.0.0",
				InstalledAt: time.Date(2024, 3, 1, 10, 0, 0, 0, time.UTC),
				UpdatedAt:   time.Date(2024, 3, 2, 15, 30, 0, 0, time.UTC),
			},
		},
		{
			name: "minimal pack source",
			source: &PackSource{
				Repository: "owner/repo",
				Version:    "0.1.0",
			},
		},
		{
			name: "pack source with zero timestamps",
			source: &PackSource{
				Repository:  "acme/workflow-prod",
				Version:     "2.3.4",
				InstalledAt: time.Time{},
				UpdatedAt:   time.Time{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode to map[string]any via SourceDataFromPackSource
			sourceData, err := SourceDataFromPackSource(tt.source)
			require.NoError(t, err)
			require.NotNil(t, sourceData)

			// Decode back via PackSourceFromSourceData
			recovered, err := PackSourceFromSourceData(sourceData)
			require.NoError(t, err)

			// Verify roundtrip preserves all fields
			assert.Equal(t, tt.source.Repository, recovered.Repository)
			assert.Equal(t, tt.source.Version, recovered.Version)
			assert.Equal(t, tt.source.InstalledAt, recovered.InstalledAt)
			assert.Equal(t, tt.source.UpdatedAt, recovered.UpdatedAt)
		})
	}
}

func TestSourceDataFromPackSource(t *testing.T) {
	tests := []struct {
		name      string
		source    *PackSource
		wantKeys  []string
		wantError bool
	}{
		{
			name: "valid source produces expected keys",
			source: &PackSource{
				Repository:  "owner/repo",
				Version:     "1.0.0",
				InstalledAt: time.Date(2024, 3, 1, 10, 0, 0, 0, time.UTC),
				UpdatedAt:   time.Date(2024, 3, 2, 11, 0, 0, 0, time.UTC),
			},
			wantKeys: []string{"repository", "version", "installed_at", "updated_at"},
		},
		{
			name:      "nil source returns error",
			source:    nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantError {
				_, err := SourceDataFromPackSource(tt.source)
				assert.Error(t, err)
				return
			}

			data, err := SourceDataFromPackSource(tt.source)
			require.NoError(t, err)
			for _, key := range tt.wantKeys {
				assert.Contains(t, data, key, "key %q missing from source data", key)
			}
		})
	}
}

func TestPackSourceFromSourceData(t *testing.T) {
	tests := []struct {
		name        string
		sourceData  map[string]any
		wantErr     bool
		wantErrType string
	}{
		{
			name: "valid source data",
			sourceData: map[string]any{
				"repository":   "owner/repo",
				"version":      "1.0.0",
				"installed_at": time.Date(2024, 3, 1, 10, 0, 0, 0, time.UTC),
				"updated_at":   time.Date(2024, 3, 2, 11, 0, 0, 0, time.UTC),
			},
		},
		{
			name:       "nil source data",
			sourceData: nil,
			wantErr:    true,
		},
		{
			name: "missing repository",
			sourceData: map[string]any{
				"version": "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "missing version",
			sourceData: map[string]any{
				"repository": "owner/repo",
			},
			wantErr: true,
		},
		{
			name: "repository type mismatch",
			sourceData: map[string]any{
				"repository": 12345,
				"version":    "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "version type mismatch",
			sourceData: map[string]any{
				"repository": "owner/repo",
				"version":    []string{"1.0.0"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := PackSourceFromSourceData(tt.sourceData)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.sourceData["repository"], result.Repository)
			assert.Equal(t, tt.sourceData["version"], result.Version)
		})
	}
}

func TestPackInfoCreation(t *testing.T) {
	tests := []struct {
		name     string
		info     *PackInfo
		validate func(*PackInfo) error
	}{
		{
			name: "valid pack info with all fields",
			info: &PackInfo{
				Name:        "example-pack",
				Version:     "1.0.0",
				Description: "Example workflow pack",
				Author:      "author@example.com",
				License:     "MIT",
				Workflows:   map[string]string{"workflow1": "workflow1.yaml"},
				Plugins:     map[string]string{"plugin1": "1.0.0"},
			},
		},
		{
			name: "pack info with minimal fields",
			info: &PackInfo{
				Name:    "minimal-pack",
				Version: "0.1.0",
			},
		},
		{
			name: "pack info with empty workflows and plugins",
			info: &PackInfo{
				Name:        "empty-pack",
				Version:     "2.0.0",
				Description: "A pack with no workflows or plugins",
				Workflows:   map[string]string{},
				Plugins:     map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.info)
			assert.NotEmpty(t, tt.info.Name)
			assert.NotEmpty(t, tt.info.Version)
		})
	}
}

func TestPackStateCreation(t *testing.T) {
	tests := []struct {
		name  string
		state *PackState
	}{
		{
			name: "pack state with source data",
			state: &PackState{
				Name:    "example-pack",
				Enabled: true,
				SourceData: map[string]any{
					"repository":   "owner/repo",
					"version":      "1.0.0",
					"installed_at": time.Now(),
				},
			},
		},
		{
			name: "pack state disabled",
			state: &PackState{
				Name:    "disabled-pack",
				Enabled: false,
			},
		},
		{
			name: "pack state with empty source data",
			state: &PackState{
				Name:       "local-pack",
				Enabled:    true,
				SourceData: map[string]any{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.state)
			assert.NotEmpty(t, tt.state.Name)
			assert.IsType(t, true, tt.state.Enabled)
		})
	}
}

func TestPackStateJSON(t *testing.T) {
	state := &PackState{
		Name:    "test-pack",
		Enabled: true,
		SourceData: map[string]any{
			"repository": "owner/repo",
			"version":    "1.0.0",
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(state)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Unmarshal back
	var recovered PackState
	err = json.Unmarshal(data, &recovered)
	require.NoError(t, err)

	// Verify fields match
	assert.Equal(t, state.Name, recovered.Name)
	assert.Equal(t, state.Enabled, recovered.Enabled)
	assert.Equal(t, state.SourceData, recovered.SourceData)
}

func TestSourceDataJSONRoundtrip(t *testing.T) {
	original := &PackSource{
		Repository:  "acme/awf-workflow-datapipeline",
		Version:     "1.2.3",
		InstalledAt: time.Date(2024, 3, 1, 10, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2024, 3, 2, 14, 30, 0, 0, time.UTC),
	}

	// Convert to source data
	sourceData, err := SourceDataFromPackSource(original)
	require.NoError(t, err)

	// Simulate JSON serialization/deserialization (as would happen in state.json)
	jsonBytes, err := json.Marshal(sourceData)
	require.NoError(t, err)

	var deserializedData map[string]any
	err = json.Unmarshal(jsonBytes, &deserializedData)
	require.NoError(t, err)

	// Convert back from deserialized data
	recovered, err := PackSourceFromSourceData(deserializedData)
	require.NoError(t, err)

	// Verify exact equality
	assert.Equal(t, original.Repository, recovered.Repository)
	assert.Equal(t, original.Version, recovered.Version)
	assert.Equal(t, original.InstalledAt, recovered.InstalledAt)
	assert.Equal(t, original.UpdatedAt, recovered.UpdatedAt)
}
