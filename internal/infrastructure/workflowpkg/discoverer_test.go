package workflowpkg_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/infrastructure/workflowpkg"
)

func TestPackDiscovererAdapter_DiscoverWorkflows_EmptyDirs(t *testing.T) {
	adapter := workflowpkg.NewPackDiscovererAdapter(nil)
	entries, err := adapter.DiscoverWorkflows(context.Background())
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestPackDiscovererAdapter_DiscoverWorkflows_FindsPacks(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "mypack")
	require.NoError(t, os.MkdirAll(filepath.Join(packDir, "workflows"), 0o755))

	manifest := `name: mypack
version: "1.0.0"
description: "A test pack"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - hello
`
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(manifest), 0o644))

	wfYAML := `name: hello
description: "Hello workflow"
initial: start
steps:
  start:
    type: terminal
    status: success
    message: done
`
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "workflows", "hello.yaml"), []byte(wfYAML), 0o644))

	stateJSON := `{"name":"mypack","enabled":true,"source_data":{"repository":"owner/mypack","version":"1.0.0"}}`
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "state.json"), []byte(stateJSON), 0o644))

	adapter := workflowpkg.NewPackDiscovererAdapter([]string{dir})
	entries, err := adapter.DiscoverWorkflows(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "mypack/hello", entries[0].Name)
	assert.Equal(t, "pack", entries[0].Source)
	assert.Equal(t, "1.0.0", entries[0].Version)
}

func TestPackDiscovererAdapter_DiscoverWorkflows_SkipsDisabledPacks(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "disabled")
	require.NoError(t, os.MkdirAll(filepath.Join(packDir, "workflows"), 0o755))

	manifest := `name: disabled
version: "1.0.0"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - hello
`
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(manifest), 0o644))

	wfYAML := `name: hello
description: "Hello"
initial: start
steps:
  start:
    type: terminal
    status: success
    message: done
`
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "workflows", "hello.yaml"), []byte(wfYAML), 0o644))

	stateJSON := `{"name":"disabled","enabled":false,"source_data":{"repository":"owner/disabled","version":"1.0.0"}}`
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "state.json"), []byte(stateJSON), 0o644))

	adapter := workflowpkg.NewPackDiscovererAdapter([]string{dir})
	entries, err := adapter.DiscoverWorkflows(context.Background())
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestPackDiscovererAdapter_DiscoverWorkflows_DeduplicatesByName(t *testing.T) {
	localDir := t.TempDir()
	globalDir := t.TempDir()

	for _, dir := range []string{localDir, globalDir} {
		packDir := filepath.Join(dir, "shared")
		require.NoError(t, os.MkdirAll(filepath.Join(packDir, "workflows"), 0o755))
		manifest := `name: shared
version: "1.0.0"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - hello
`
		require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(manifest), 0o644))
		wfYAML := `name: hello
initial: start
steps:
  start:
    type: terminal
    status: success
    message: done
`
		require.NoError(t, os.WriteFile(filepath.Join(packDir, "workflows", "hello.yaml"), []byte(wfYAML), 0o644))
		stateJSON := `{"name":"shared","enabled":true,"source_data":{"repository":"owner/shared","version":"1.0.0"}}`
		require.NoError(t, os.WriteFile(filepath.Join(packDir, "state.json"), []byte(stateJSON), 0o644))
	}

	adapter := workflowpkg.NewPackDiscovererAdapter([]string{localDir, globalDir})
	entries, err := adapter.DiscoverWorkflows(context.Background())
	require.NoError(t, err)
	assert.Len(t, entries, 1, "duplicate pack should be deduplicated")
}

func TestPackDiscovererAdapter_LoadsWorkflowDescription(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "descpack")
	require.NoError(t, os.MkdirAll(filepath.Join(packDir, "workflows"), 0o755))

	manifest := `name: descpack
version: "2.0.0"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - greet
`
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(manifest), 0o644))

	wfYAML := `name: greet
description: "Greet the user"
initial: start
steps:
  start:
    type: terminal
    status: success
    message: hello
`
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "workflows", "greet.yaml"), []byte(wfYAML), 0o644))

	stateJSON := `{"name":"descpack","enabled":true,"source_data":{"repository":"owner/descpack","version":"2.0.0"}}`
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "state.json"), []byte(stateJSON), 0o644))

	adapter := workflowpkg.NewPackDiscovererAdapter([]string{dir})
	entries, err := adapter.DiscoverWorkflows(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "Greet the user", entries[0].Description)
}

// TestPackDiscovererAdapter_DiscoverWorkflows_SkipsPackWithInvalidName verifies that
// a pack whose manifest declares a name with path-traversal characters is silently
// skipped and does not appear in the results. No panic, no path escape.
func TestPackDiscovererAdapter_DiscoverWorkflows_SkipsPackWithInvalidName(t *testing.T) {
	dir := t.TempDir()
	// The directory name must be a valid OS path component; the evil name is in
	// the manifest content, which is what nameRegex checks.
	packDir := filepath.Join(dir, "evilpack")
	require.NoError(t, os.MkdirAll(filepath.Join(packDir, "workflows"), 0o755))

	// manifest.Name contains path traversal; Validate will reject it.
	evilManifest := `name: "../../evil"
version: "1.0.0"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - hello
`
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(evilManifest), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "workflows", "hello.yaml"), []byte("name: hello\n"), 0o644))
	stateJSON := `{"name":"evilpack","enabled":true,"source_data":{"repository":"owner/evilpack","version":"1.0.0"}}`
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "state.json"), []byte(stateJSON), 0o644))

	adapter := workflowpkg.NewPackDiscovererAdapter([]string{dir})
	entries, err := adapter.DiscoverWorkflows(context.Background())

	require.NoError(t, err, "invalid pack name must not cause an error — it is silently skipped")
	assert.Empty(t, entries, "pack with invalid name must not produce entries")
}

// TestPackDiscovererAdapter_DiscoverWorkflows_SkipsPackWithInvalidWorkflowName verifies
// that a pack with a valid name but a manifest declaring invalid workflow names (including
// path traversal) is silently skipped and does not appear in the results.
func TestPackDiscovererAdapter_DiscoverWorkflows_SkipsPackWithInvalidWorkflowName(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "safepack")
	require.NoError(t, os.MkdirAll(filepath.Join(packDir, "workflows"), 0o755))

	// Pack name is valid but workflow name contains path traversal.
	// manifest.Validate now rejects invalid workflow names, so DiscoverPacks
	// will skip this pack entirely.
	evilManifest := `name: safepack
version: "1.0.0"
author: "test"
awf_version: ">=0.5.0"
workflows:
  - "../../etc/passwd"
`
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(evilManifest), 0o644))
	stateJSON := `{"name":"safepack","enabled":true,"source_data":{"repository":"owner/safepack","version":"1.0.0"}}`
	require.NoError(t, os.WriteFile(filepath.Join(packDir, "state.json"), []byte(stateJSON), 0o644))

	adapter := workflowpkg.NewPackDiscovererAdapter([]string{dir})
	entries, err := adapter.DiscoverWorkflows(context.Background())

	require.NoError(t, err, "invalid workflow name must not cause an error — pack is silently skipped")
	assert.Empty(t, entries, "pack with invalid workflow name must not produce entries")

	// Verify no entry name contains the traversal string.
	for _, e := range entries {
		assert.NotContains(t, e.Name, "..", "entry name must not contain path traversal")
		assert.NotContains(t, e.Workflow, "..", "workflow field must not contain path traversal")
	}
}

// TestPackDiscovererAdapter_DiscoverWorkflows_PopulatesScopeAndWorkflowFields covers both single and multiple
// workflows per pack to ensure Scope=packName, Workflow=wfName, Name=packName/wfName, Source="pack".
func TestPackDiscovererAdapter_DiscoverWorkflows_PopulatesScopeAndWorkflowFields(t *testing.T) {
	tests := []struct {
		name         string
		packName     string
		workflows    []string
		wantScope    string
		wantWorkflow string
	}{
		{
			name:         "single workflow in pack",
			packName:     "acme",
			workflows:    []string{"deploy"},
			wantScope:    "acme",
			wantWorkflow: "deploy",
		},
		{
			name:         "multiple workflows in pack",
			packName:     "vendors",
			workflows:    []string{"build", "test", "deploy"},
			wantScope:    "vendors",
			wantWorkflow: "build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			packDir := filepath.Join(dir, tt.packName)
			require.NoError(t, os.MkdirAll(filepath.Join(packDir, "workflows"), 0o755))

			var workflowsYAML strings.Builder
			for _, wf := range tt.workflows {
				fmt.Fprintf(&workflowsYAML, "  - %s\n", wf)
			}

			manifest := fmt.Sprintf(`name: %s
version: "1.0.0"
author: "test"
awf_version: ">=0.5.0"
workflows:
%s`, tt.packName, workflowsYAML.String())
			require.NoError(t, os.WriteFile(filepath.Join(packDir, "manifest.yaml"), []byte(manifest), 0o644))

			for _, wf := range tt.workflows {
				wfYAML := fmt.Sprintf(`name: %s
initial: start
steps:
  start:
    type: terminal
    status: success
    message: ok
`, wf)
				require.NoError(t, os.WriteFile(
					filepath.Join(packDir, "workflows", wf+".yaml"),
					[]byte(wfYAML),
					0o644,
				))
			}

			stateJSON := fmt.Sprintf(`{"name":%q,"enabled":true,"source_data":{"repository":"owner/%s","version":"1.0.0"}}`, tt.packName, tt.packName)
			require.NoError(t, os.WriteFile(filepath.Join(packDir, "state.json"), []byte(stateJSON), 0o644))

			adapter := workflowpkg.NewPackDiscovererAdapter([]string{dir})
			entries, err := adapter.DiscoverWorkflows(context.Background())

			require.NoError(t, err)
			require.NotEmpty(t, entries, "should discover at least one workflow")

			entry := entries[0]
			assert.Equal(t, tt.packName+"/"+tt.wantWorkflow, entry.Name, "Name should be pack/workflow")
			assert.Equal(t, tt.wantScope, entry.Scope, "Scope should be pack name")
			assert.Equal(t, tt.wantWorkflow, entry.Workflow, "Workflow should be workflow name")
			assert.Equal(t, "pack", entry.Source, "Source should be pack")
		})
	}
}
