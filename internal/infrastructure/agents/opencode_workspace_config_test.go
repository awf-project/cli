//go:build !windows

package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

// TestAddOpenCodeMCPServer_EmptyWorkspace verifies that a fresh workspace gets
// a new opencode.json with our entry, and cleanup deletes the file entirely.
func TestAddOpenCodeMCPServer_EmptyWorkspace(t *testing.T) {
	dir := t.TempDir()

	cleanup, err := addOpenCodeMCPServer(dir, "awf-proxy-test01", []string{"/usr/bin/awf", "mcp-serve", "--config", "/tmp/c.json"})
	require.NoError(t, err)
	require.NotNil(t, cleanup)

	configPath := filepath.Join(dir, "opencode.json")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var top map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &top))
	require.Contains(t, top, "mcp", "mcp key must exist after add")

	var mcpMap map[string]opencodeMCPEntry
	require.NoError(t, json.Unmarshal(top["mcp"], &mcpMap))
	entry, ok := mcpMap["awf-proxy-test01"]
	require.True(t, ok, "our server entry must be present")
	assert.Equal(t, "local", entry.Type)
	assert.Equal(t, []string{"/usr/bin/awf", "mcp-serve", "--config", "/tmp/c.json"}, entry.Command)
	assert.True(t, entry.Enabled)

	// Cleanup should remove the file since we created it from scratch.
	require.NoError(t, cleanup())
	_, statErr := os.Stat(configPath)
	assert.True(t, os.IsNotExist(statErr), "opencode.json must be deleted after cleanup on a from-scratch file")
}

// TestAddOpenCodeMCPServer_OpenCodeInjectsSchemaPostAdd reproduces the real-world
// scenario where opencode itself rewrites our from-scratch opencode.json after we
// added our entry — typically annotating it with "$schema". Cleanup must still
// delete the file because createdByUs == true and no user content can have
// reached this artifact via legitimate edits during a single workflow step.
func TestAddOpenCodeMCPServer_OpenCodeInjectsSchemaPostAdd(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")

	cleanup, err := addOpenCodeMCPServer(dir, "awf-proxy-injected01", []string{"/bin/awf", "mcp-serve"})
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var top map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &top))
	mutated := map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"mcp":     json.RawMessage(top["mcp"]),
	}
	writeJSON(t, configPath, mutated)

	require.NoError(t, cleanup())
	_, statErr := os.Stat(configPath)
	assert.True(t, os.IsNotExist(statErr),
		"file must be deleted even when opencode injected $schema, because createdByUs==true")
}

// TestAddOpenCodeMCPServer_PreExistingFileWithSchemaAndUserKeys verifies that
// existing top-level keys ($schema, model, etc.) survive the merge and cleanup
// removes only our entry, leaving the file intact.
func TestAddOpenCodeMCPServer_PreExistingFileWithSchemaAndUserKeys(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")

	// Pre-populate with user content.
	initial := map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"model":   "gpt-4o",
	}
	writeJSON(t, configPath, initial)

	cleanup, err := addOpenCodeMCPServer(dir, "awf-proxy-schema01", []string{"/bin/awf", "mcp-serve"})
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var top map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &top))

	// User keys must be preserved.
	assert.Contains(t, top, "$schema", "$schema must be preserved")
	assert.Contains(t, top, "model", "model must be preserved")
	assert.Contains(t, top, "mcp", "mcp key must exist")

	// Our entry must be present.
	var mcpMap map[string]opencodeMCPEntry
	require.NoError(t, json.Unmarshal(top["mcp"], &mcpMap))
	assert.Contains(t, mcpMap, "awf-proxy-schema01")

	// Cleanup removes only our entry; user keys survive.
	require.NoError(t, cleanup())
	data, err = os.ReadFile(configPath)
	require.NoError(t, err, "file must still exist after cleanup — user has $schema + model")

	// Use a fresh map to avoid json.Unmarshal merging into stale state.
	var topAfter map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &topAfter))
	assert.Contains(t, topAfter, "$schema", "$schema must still be present after cleanup")
	assert.Contains(t, topAfter, "model", "model must still be present after cleanup")

	var mcpAfter map[string]opencodeMCPEntry
	if raw, ok := topAfter["mcp"]; ok {
		_ = json.Unmarshal(raw, &mcpAfter)
	}
	assert.NotContains(t, mcpAfter, "awf-proxy-schema01", "our entry must be removed by cleanup")
}

// TestAddOpenCodeMCPServer_PreExistingMCPEntry verifies our entry is added alongside
// a pre-existing mcp entry, and cleanup removes only ours.
func TestAddOpenCodeMCPServer_PreExistingMCPEntry(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")

	initial := map[string]any{
		"mcp": map[string]any{
			"user-server": map[string]any{
				"type":    "local",
				"command": []string{"/usr/local/bin/my-server"},
				"enabled": true,
			},
		},
	}
	writeJSON(t, configPath, initial)

	cleanup, err := addOpenCodeMCPServer(dir, "awf-proxy-sibling01", []string{"/bin/awf", "mcp-serve"})
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var top map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &top))
	var mcpMap map[string]opencodeMCPEntry
	require.NoError(t, json.Unmarshal(top["mcp"], &mcpMap))

	assert.Contains(t, mcpMap, "user-server", "pre-existing entry must be preserved")
	assert.Contains(t, mcpMap, "awf-proxy-sibling01", "our entry must be added")

	// Cleanup removes only ours; user-server survives.
	require.NoError(t, cleanup())
	data, err = os.ReadFile(configPath)
	require.NoError(t, err, "file must persist — user-server still present")

	// Use a fresh map to avoid json.Unmarshal merging into stale state.
	var topAfter map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &topAfter))
	var mcpAfter map[string]opencodeMCPEntry
	require.NoError(t, json.Unmarshal(topAfter["mcp"], &mcpAfter))
	assert.Contains(t, mcpAfter, "user-server", "user-server must survive cleanup")
	assert.NotContains(t, mcpAfter, "awf-proxy-sibling01", "our entry must be removed")
}

// TestAddOpenCodeMCPServer_IdempotentCleanup verifies that calling cleanup twice
// is a no-op and returns nil both times.
func TestAddOpenCodeMCPServer_IdempotentCleanup(t *testing.T) {
	dir := t.TempDir()

	cleanup, err := addOpenCodeMCPServer(dir, "awf-proxy-idem01", []string{"/bin/awf", "mcp-serve"})
	require.NoError(t, err)

	require.NoError(t, cleanup(), "first cleanup must succeed")
	require.NoError(t, cleanup(), "second cleanup must be no-op and return nil")
}

// TestAddOpenCodeMCPServer_ConcurrentSafety spawns N goroutines each adding a
// uniquely-named entry, then verifies all N entries landed correctly, then runs
// all N cleanups and verifies the file is gone (created-from-scratch scenario).
func TestAddOpenCodeMCPServer_ConcurrentSafety(t *testing.T) {
	t.Parallel()
	const n = 8
	dir := t.TempDir()

	names := make([]string, n)
	for i := range names {
		names[i] = mcpProxyNamePrefix + randShortID(8)
	}

	cleanups := make([]func() error, n)
	var mu sync.Mutex

	var g errgroup.Group
	for i, name := range names {
		g.Go(func() error {
			cleanup, err := addOpenCodeMCPServer(dir, name, []string{"/bin/awf", "mcp-serve", "--config", "/tmp/c.json"})
			if err != nil {
				return err
			}
			mu.Lock()
			cleanups[i] = cleanup
			mu.Unlock()
			return nil
		})
	}
	require.NoError(t, g.Wait(), "all concurrent adds must succeed")

	// Verify all entries are present.
	configPath := filepath.Join(dir, "opencode.json")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var top map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &top))
	var mcpMap map[string]opencodeMCPEntry
	require.NoError(t, json.Unmarshal(top["mcp"], &mcpMap))
	for _, name := range names {
		assert.Contains(t, mcpMap, name, "entry %s must be present after concurrent adds", name)
	}

	// Run all cleanups concurrently.
	var cg errgroup.Group
	for _, cleanup := range cleanups {
		cg.Go(cleanup)
	}
	require.NoError(t, cg.Wait(), "all concurrent cleanups must succeed")

	// File was created from scratch with no user keys — it must be gone.
	_, statErr := os.Stat(configPath)
	assert.True(t, os.IsNotExist(statErr), "opencode.json must be deleted after all cleanups on a from-scratch file")
}

// TestAcquireWorkspaceLock verifies that a contended advisory flock times out at
// its documented bound, then leaves no lock behind once the original owner
// releases it. A successful subsequent acquisition proves the timeout cleanup
// does not retain the lock file descriptor.
func TestAcquireWorkspaceLock_AcquireAndRelease(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "test.lock")
	assert.Equal(t, 5*time.Second, workspaceLockTimeout)

	held, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = held.Close() })
	require.NoError(t, syscall.Flock(int(held.Fd()), syscall.LOCK_EX)) //nolint:gosec // controlled test descriptor

	// Keep the owner lock held for the complete call. This deterministically
	// exercises the helper's bounded contention path rather than racing a
	// concurrent release against the attempted acquisition.
	startedAt := time.Now()
	_, timedOutRelease, timeoutErr := acquireWorkspaceLock(lockPath)
	elapsed := time.Since(startedAt)
	require.Nil(t, timedOutRelease, "a timed-out acquisition must not return a release function")
	require.EqualError(t, timeoutErr, "timed out acquiring lock after 5s")
	assert.GreaterOrEqual(t, elapsed, workspaceLockTimeout, "contended acquisition must wait through its timeout")
	assert.Less(t, elapsed, workspaceLockTimeout+time.Second, "contended acquisition must remain bounded")

	require.NoError(t, held.Close())
	_, release2, err := acquireWorkspaceLock(lockPath)
	require.NoError(t, err, "acquisition after timeout cleanup and owner release must succeed")
	require.NotNil(t, release2)
	release2()

	_, release3, err := acquireWorkspaceLock(lockPath)
	require.NoError(t, err, "release callback must unlock the workspace for later callers")
	require.NotNil(t, release3)
	release3()
}

// writeJSON marshals v and writes it to path, failing the test on any error.
func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o600))
}
