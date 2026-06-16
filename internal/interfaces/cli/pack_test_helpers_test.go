package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// createPackManifest creates manifest.yaml for a test pack.
func createPackManifest(t *testing.T, packDir, packName, version string, workflowNames []string) {
	t.Helper()

	manifest := `name: ` + packName + `
version: "` + version + `"
description: Test workflow pack
author: test-author
license: MIT
awf_version: ">=0.5.0"
workflows:
`
	for _, name := range workflowNames {
		manifest += `  - ` + name + "\n"
	}

	require.NoError(t, os.WriteFile(
		filepath.Join(packDir, "manifest.yaml"),
		[]byte(manifest),
		0o644,
	))
}

// createPackState creates state.json for a test pack.
func createPackState(t *testing.T, packDir, packName, version string, enabled bool) {
	t.Helper()

	stateJSON := `{
  "name": "` + packName + `",
  "enabled": ` + boolToJSON(enabled) + `,
  "source_data": {
    "repository": "https://github.com/test/` + packName + `",
    "version": "` + version + `",
    "installed_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
`
	require.NoError(t, os.WriteFile(
		filepath.Join(packDir, "state.json"),
		[]byte(stateJSON),
		0o644,
	))
}

// boolToJSON converts a boolean to JSON string.
func boolToJSON(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
