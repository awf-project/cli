package workflowpkg_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/infrastructure/workflowpkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseManifest_ValidYAML tests parsing a valid manifest.yaml.
func TestParseManifest_ValidYAML(t *testing.T) {
	yamlData := []byte(`
name: speckit
version: "1.2.0"
description: "Spec-driven development workflow pack"
author: "awf-project"
license: "MIT"
awf_version: ">=0.5.0"
workflows:
  - specify
  - clarify
plugins:
  security-validator: ">=1.0.0"
`)

	manifest, err := workflowpkg.ParseManifest(yamlData)

	require.NoError(t, err)
	require.NotNil(t, manifest)
	assert.Equal(t, "speckit", manifest.Name)
	assert.Equal(t, "1.2.0", manifest.Version)
	assert.Equal(t, "Spec-driven development workflow pack", manifest.Description)
	assert.Equal(t, "awf-project", manifest.Author)
	assert.Equal(t, "MIT", manifest.License)
	assert.Equal(t, ">=0.5.0", manifest.AWFVersion)
	assert.Len(t, manifest.Workflows, 2)
	assert.Contains(t, manifest.Workflows, "specify")
	assert.Contains(t, manifest.Workflows, "clarify")
	assert.Len(t, manifest.Plugins, 1)
	assert.Equal(t, ">=1.0.0", manifest.Plugins["security-validator"])
}

// TestParseManifest_MinimalManifest tests parsing with only required fields.
func TestParseManifest_MinimalManifest(t *testing.T) {
	yamlData := []byte(`
name: minimal
version: "1.0.0"
description: "Minimal pack"
author: "test"
awf_version: ">=0.1.0"
workflows:
  - test
`)

	manifest, err := workflowpkg.ParseManifest(yamlData)

	require.NoError(t, err)
	require.NotNil(t, manifest)
	assert.Equal(t, "minimal", manifest.Name)
	assert.Empty(t, manifest.License)
	assert.Len(t, manifest.Plugins, 0)
}

// TestParseManifest_InvalidYAML tests parsing malformed YAML.
func TestParseManifest_InvalidYAML(t *testing.T) {
	yamlData := []byte(`
name: test
version: "1.0.0
invalid: [unclosed array
`)

	manifest, err := workflowpkg.ParseManifest(yamlData)

	assert.Error(t, err)
	assert.Nil(t, manifest)
}

// TestParseManifest_EmptyData tests parsing empty byte slice.
func TestParseManifest_EmptyData(t *testing.T) {
	yamlData := []byte("")

	manifest, err := workflowpkg.ParseManifest(yamlData)

	assert.Error(t, err)
	assert.Nil(t, manifest)
}

// TestValidate_ValidManifest tests validation of a valid manifest with existing workflow files.
func TestValidate_ValidManifest(t *testing.T) {
	// Setup: create temporary pack directory with workflows
	packDir := t.TempDir()
	workflowsDir := filepath.Join(packDir, "workflows")
	require.NoError(t, os.Mkdir(workflowsDir, 0o755))

	// Create workflow files
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "specify.yaml"), []byte("test"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "clarify.yaml"), []byte("test"), 0o644))

	manifest := &workflowpkg.Manifest{
		Name:        "speckit",
		Version:     "1.2.0",
		Description: "Test pack",
		Author:      "test",
		AWFVersion:  ">=0.5.0",
		Workflows:   []string{"specify", "clarify"},
		Plugins:     map[string]string{},
	}

	err := manifest.Validate(packDir)

	assert.NoError(t, err)
}

// TestValidate_InvalidName tests validation fails for invalid pack name.
func TestValidate_InvalidName(t *testing.T) {
	tests := []struct {
		name        string
		invalidName string
	}{
		{name: "starts with digit", invalidName: "1pack"},
		{name: "contains uppercase", invalidName: "MyPack"},
		{name: "contains underscore", invalidName: "my_pack"},
		{name: "contains space", invalidName: "my pack"},
		{name: "contains dot", invalidName: "my.pack"},
		{name: "empty name", invalidName: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packDir := t.TempDir()
			workflowsDir := filepath.Join(packDir, "workflows")
			require.NoError(t, os.Mkdir(workflowsDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "test.yaml"), []byte("test"), 0o644))

			manifest := &workflowpkg.Manifest{
				Name:        tt.invalidName,
				Version:     "1.0.0",
				Description: "Test",
				Author:      "test",
				AWFVersion:  ">=0.1.0",
				Workflows:   []string{"test"},
			}

			err := manifest.Validate(packDir)

			assert.Error(t, err, "expected validation to fail for name %q", tt.invalidName)
		})
	}
}

// TestValidate_InvalidVersion tests validation fails for invalid semver version.
func TestValidate_InvalidVersion(t *testing.T) {
	tests := []struct {
		name           string
		invalidVersion string
	}{
		{name: "not semver", invalidVersion: "1.0"},
		{name: "non-numeric", invalidVersion: "abc.def.ghi"},
		{name: "empty string", invalidVersion: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packDir := t.TempDir()
			workflowsDir := filepath.Join(packDir, "workflows")
			require.NoError(t, os.Mkdir(workflowsDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "test.yaml"), []byte("test"), 0o644))

			manifest := &workflowpkg.Manifest{
				Name:        "valid",
				Version:     tt.invalidVersion,
				Description: "Test",
				Author:      "test",
				AWFVersion:  ">=0.1.0",
				Workflows:   []string{"test"},
			}

			err := manifest.Validate(packDir)

			assert.Error(t, err, "expected validation to fail for version %q", tt.invalidVersion)
		})
	}
}

// TestValidate_InvalidAWFVersionConstraint tests validation fails for invalid awf_version constraint.
func TestValidate_InvalidAWFVersionConstraint(t *testing.T) {
	tests := []struct {
		name              string
		invalidConstraint string
	}{
		{name: "malformed constraint", invalidConstraint: ">>0.5.0"},
		{name: "empty constraint", invalidConstraint: ""},
		{name: "invalid semver in constraint", invalidConstraint: ">=1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packDir := t.TempDir()
			workflowsDir := filepath.Join(packDir, "workflows")
			require.NoError(t, os.Mkdir(workflowsDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "test.yaml"), []byte("test"), 0o644))

			manifest := &workflowpkg.Manifest{
				Name:        "valid",
				Version:     "1.0.0",
				Description: "Test",
				Author:      "test",
				AWFVersion:  tt.invalidConstraint,
				Workflows:   []string{"test"},
			}

			err := manifest.Validate(packDir)

			assert.Error(t, err, "expected validation to fail for awf_version %q", tt.invalidConstraint)
		})
	}
}

// TestValidate_MissingWorkflowFile tests validation fails when workflow file is missing.
func TestValidate_MissingWorkflowFile(t *testing.T) {
	packDir := t.TempDir()
	workflowsDir := filepath.Join(packDir, "workflows")
	require.NoError(t, os.Mkdir(workflowsDir, 0o755))

	// Create only one of two workflow files
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "specify.yaml"), []byte("test"), 0o644))

	manifest := &workflowpkg.Manifest{
		Name:        "speckit",
		Version:     "1.2.0",
		Description: "Test",
		Author:      "test",
		AWFVersion:  ">=0.5.0",
		Workflows:   []string{"specify", "missing"},
	}

	err := manifest.Validate(packDir)

	assert.Error(t, err)
}

// TestValidate_NoWorkflowFiles tests validation fails when workflows directory doesn't exist.
func TestValidate_NoWorkflowFiles(t *testing.T) {
	packDir := t.TempDir()

	manifest := &workflowpkg.Manifest{
		Name:        "test",
		Version:     "1.0.0",
		Description: "Test",
		Author:      "test",
		AWFVersion:  ">=0.1.0",
		Workflows:   []string{"test"},
	}

	err := manifest.Validate(packDir)

	assert.Error(t, err)
}

// TestValidate_EmptyWorkflowsList tests validation fails when workflows list is empty.
func TestValidate_EmptyWorkflowsList(t *testing.T) {
	packDir := t.TempDir()
	workflowsDir := filepath.Join(packDir, "workflows")
	require.NoError(t, os.Mkdir(workflowsDir, 0o755))

	manifest := &workflowpkg.Manifest{
		Name:        "test",
		Version:     "1.0.0",
		Description: "Test",
		Author:      "test",
		AWFVersion:  ">=0.1.0",
		Workflows:   []string{},
	}

	err := manifest.Validate(packDir)

	assert.Error(t, err)
}

// TestValidate_InvalidWorkflowName tests that workflow names with path traversal
// or invalid characters are rejected to prevent path traversal attacks.
func TestValidate_InvalidWorkflowName(t *testing.T) {
	tests := []struct {
		name         string
		workflowName string
	}{
		{name: "path traversal", workflowName: "../../etc/passwd"},
		{name: "absolute path", workflowName: "/etc/passwd"},
		{name: "starts with digit", workflowName: "1workflow"},
		{name: "contains uppercase", workflowName: "MyWorkflow"},
		{name: "contains underscore", workflowName: "my_workflow"},
		{name: "contains slash", workflowName: "pack/workflow"},
		{name: "empty name", workflowName: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packDir := t.TempDir()
			workflowsDir := filepath.Join(packDir, "workflows")
			require.NoError(t, os.Mkdir(workflowsDir, 0o755))

			manifest := &workflowpkg.Manifest{
				Name:        "valid",
				Version:     "1.0.0",
				Description: "Test",
				Author:      "test",
				AWFVersion:  ">=0.1.0",
				Workflows:   []string{tt.workflowName},
			}

			err := manifest.Validate(packDir)

			assert.Error(t, err, "expected validation to fail for workflow name %q", tt.workflowName)
			assert.Contains(t, err.Error(), "invalid workflow name", "error should mention invalid workflow name")
		})
	}
}

// TestValidate_ValidWorkflowNames tests that well-formed kebab-case names are accepted.
func TestValidate_ValidWorkflowNames(t *testing.T) {
	tests := []struct {
		name         string
		workflowName string
	}{
		{name: "simple lowercase", workflowName: "specify"},
		{name: "kebab-case", workflowName: "run-tests"},
		{name: "with digits", workflowName: "deploy-v2"},
		{name: "single letter", workflowName: "a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packDir := t.TempDir()
			workflowsDir := filepath.Join(packDir, "workflows")
			require.NoError(t, os.Mkdir(workflowsDir, 0o755))
			require.NoError(t, os.WriteFile(
				filepath.Join(workflowsDir, tt.workflowName+".yaml"),
				[]byte("test"),
				0o644,
			))

			manifest := &workflowpkg.Manifest{
				Name:        "valid",
				Version:     "1.0.0",
				Description: "Test",
				Author:      "test",
				AWFVersion:  ">=0.1.0",
				Workflows:   []string{tt.workflowName},
			}

			err := manifest.Validate(packDir)

			assert.NoError(t, err)
		})
	}
}

func TestValidate_ReservedScope(t *testing.T) {
	tests := []struct {
		name          string
		packName      string
		wantErr       bool
		wantCode      domerrors.ErrorCode
		wantPackName  string
		wantFormatErr bool
	}{
		{
			name:         "local is reserved",
			packName:     "local",
			wantErr:      true,
			wantCode:     domerrors.ErrorCodeUserInputValidationFailed,
			wantPackName: "local",
		},
		{
			name:         "global is reserved",
			packName:     "global",
			wantErr:      true,
			wantCode:     domerrors.ErrorCodeUserInputValidationFailed,
			wantPackName: "global",
		},
		{
			name:         "env is reserved",
			packName:     "env",
			wantErr:      true,
			wantCode:     domerrors.ErrorCodeUserInputValidationFailed,
			wantPackName: "env",
		},
		{
			name:     "localpack is not reserved",
			packName: "localpack",
			wantErr:  false,
		},
		{
			name:     "globalpack is not reserved",
			packName: "globalpack",
			wantErr:  false,
		},
		{
			name:     "envpack is not reserved",
			packName: "envpack",
			wantErr:  false,
		},
		{
			name:     "run is not reserved",
			packName: "run",
			wantErr:  false,
		},
		{
			name:     "validate is not reserved",
			packName: "validate",
			wantErr:  false,
		},
		{
			name:          "invalid format fails with format error not reserved error",
			packName:      "!!!",
			wantErr:       true,
			wantFormatErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packDir := t.TempDir()
			workflowsDir := filepath.Join(packDir, "workflows")
			require.NoError(t, os.Mkdir(workflowsDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "test.yaml"), []byte("test"), 0o644))

			manifest := &workflowpkg.Manifest{
				Name:        tt.packName,
				Version:     "1.0.0",
				Description: "Test",
				Author:      "test",
				AWFVersion:  ">=0.1.0",
				Workflows:   []string{"test"},
			}

			err := manifest.Validate(packDir)

			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)

			if tt.wantFormatErr {
				var structErr *domerrors.StructuredError
				assert.False(t, errors.As(err, &structErr), "format error must not be a StructuredError")
				return
			}

			var structErr *domerrors.StructuredError
			require.True(t, errors.As(err, &structErr), "expected *StructuredError")
			assert.Equal(t, tt.wantCode, structErr.Code)
			assert.Equal(t, tt.wantPackName, structErr.Details["pack_name"])
			assert.Equal(t, []string{"local", "global", "env"}, structErr.Details["reserved_tokens"])
		})
	}
}
