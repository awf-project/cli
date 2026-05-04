package workflowpkg_test

import (
	"context"
	"os"
	"path/filepath"
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
