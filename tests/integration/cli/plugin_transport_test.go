//go:build integration

// Feature: C067
package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginTransport_EchoPluginLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, ".awf", "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	echoBinPath := buildEchoPlugin(t, pluginsDir)
	require.True(t, fileExists(echoBinPath), "echo plugin binary should exist")
	setupPluginManifest(t, pluginsDir, "awf-plugin-echo")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	manager := setupPluginManager(t, pluginsDir)

	// Execute with required input only
	result, err := manager.Execute(ctx, "echo", map[string]any{"text": "hello"})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "hello", result.Outputs["output"])
	assert.Equal(t, "hello", result.Outputs["text"])

	// Execute with optional prefix
	result, err = manager.Execute(ctx, "echo", map[string]any{"text": "world", "prefix": "hello"})
	require.NoError(t, err)
	assert.Equal(t, "hello world", result.Outputs["output"])
	assert.Equal(t, "hello", result.Outputs["prefix"])

	err = manager.Shutdown(ctx, "awf-plugin-echo")
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
	assert.True(t, pluginProcessTerminated(echoBinPath), "plugin process should be terminated after Shutdown")
}

func TestPluginTransport_ConcurrentExecute(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, ".awf", "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	buildEchoPlugin(t, pluginsDir)
	setupPluginManifest(t, pluginsDir, "awf-plugin-echo")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	manager := setupPluginManager(t, pluginsDir)
	defer manager.ShutdownAll(ctx) //nolint:errcheck // test cleanup

	// 20 concurrent goroutines per AC: "Concurrent Execute calls produce no races"
	const goroutines = 20
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	results := make([]string, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			input := map[string]any{"text": fmt.Sprintf("msg-%d", idx)}
			res, err := manager.Execute(ctx, "echo", input)
			errs[idx] = err
			if res != nil {
				if out, ok := res.Outputs["output"].(string); ok {
					results[idx] = out
				}
			}
		}(i)
	}
	wg.Wait()

	for i := range goroutines {
		require.NoError(t, errs[i], "goroutine %d should not error", i)
		assert.Equal(t, fmt.Sprintf("msg-%d", i), results[i], "goroutine %d output mismatch", i)
	}
}

func TestPluginTransport_BinaryNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, ".awf", "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	// Manifest but no binary
	setupPluginManifest(t, pluginsDir, "awf-plugin-echo")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)
	manager := pluginmgr.NewRPCPluginManager(loader)
	manager.SetPluginsDir(pluginsDir)

	plugins, err := manager.Discover(ctx)
	require.NoError(t, err)
	require.Len(t, plugins, 1)

	require.NoError(t, manager.Load(ctx, plugins[0].Manifest.Name))

	err = manager.Init(ctx, "awf-plugin-echo", nil)
	require.Error(t, err, "Init should fail when binary not found")
}

func TestPluginTransport_VersionMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, ".awf", "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	buildEchoPlugin(t, pluginsDir)

	// Overwrite manifest with incompatible version constraint
	echoPluginDir := filepath.Join(pluginsDir, "awf-plugin-echo")
	manifestContent := `name: awf-plugin-echo
version: 1.0.0
awf_version: "<0.2.0"
capabilities:
  - operations
`
	require.NoError(t, os.WriteFile(filepath.Join(echoPluginDir, "plugin.yaml"), []byte(manifestContent), 0o644))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Manual discover+load (not setupPluginManager which auto-inits)
	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)
	manager := pluginmgr.NewRPCPluginManager(loader)
	manager.SetPluginsDir(pluginsDir)

	plugins, err := manager.Discover(ctx)
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	require.NoError(t, manager.Load(ctx, plugins[0].Manifest.Name))

	err = manager.Init(ctx, "awf-plugin-echo", nil)
	require.Error(t, err, "Init should fail on version mismatch")
	assert.Contains(t, err.Error(), "version")
}

func TestPluginTransport_ShutdownAll(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, ".awf", "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	// Build same echo plugin source into two distinct plugin directories
	buildPlugin(t, pluginsDir, "awf-plugin-echo", "awf-plugin-echo-1")
	buildPlugin(t, pluginsDir, "awf-plugin-echo", "awf-plugin-echo-2")
	setupPluginManifest(t, pluginsDir, "awf-plugin-echo-1")
	setupPluginManifest(t, pluginsDir, "awf-plugin-echo-2")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	manager := setupPluginManager(t, pluginsDir)
	require.NoError(t, manager.Init(ctx, "awf-plugin-echo-1", nil))
	require.NoError(t, manager.Init(ctx, "awf-plugin-echo-2", nil))

	err := manager.ShutdownAll(ctx)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
}

func TestPluginTransport_ContextTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, ".awf", "plugins")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	echoPluginDir := filepath.Join(pluginsDir, "awf-plugin-echo")
	require.NoError(t, os.MkdirAll(echoPluginDir, 0o755))

	manifestContent := `name: awf-plugin-echo
version: 1.0.0
awf_version: ">=0.4.0"
capabilities:
  - operations
`
	require.NoError(t, os.WriteFile(filepath.Join(echoPluginDir, "plugin.yaml"), []byte(manifestContent), 0o644))

	// Hanging binary to trigger timeout (short sleep to limit test wall time)
	binPath := filepath.Join(echoPluginDir, "awf-plugin-echo")
	hangScript := "#!/bin/bash\nsleep 3"
	require.NoError(t, os.WriteFile(binPath, []byte(hangScript), 0o755))

	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)
	manager := pluginmgr.NewRPCPluginManager(loader)
	manager.SetPluginsDir(pluginsDir)

	discoverCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	plugins, err := manager.Discover(discoverCtx)
	cancel()
	require.NoError(t, err)
	require.Len(t, plugins, 1)

	loadCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	require.NoError(t, manager.Load(loadCtx, plugins[0].Manifest.Name))
	cancel()

	// 1s timeout — well under NFR-002's 5s hard deadline
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = manager.Init(ctx, "awf-plugin-echo", nil)
	require.Error(t, err, "Init should timeout")
}

// --- Helpers ---

func setupPluginManager(t *testing.T, pluginsDir string) *pluginmgr.RPCPluginManager {
	t.Helper()

	parser := pluginmgr.NewManifestParser()
	loader := pluginmgr.NewFileSystemLoader(parser)
	manager := pluginmgr.NewRPCPluginManager(loader)
	manager.SetPluginsDir(pluginsDir)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	plugins, err := manager.Discover(ctx)
	require.NoError(t, err, "Discover failed")

	for _, p := range plugins {
		name := p.Manifest.Name
		if err := manager.Load(ctx, name); err != nil {
			t.Fatalf("Load %s failed: %v", name, err)
		}
		if err := manager.Init(ctx, name, nil); err != nil {
			t.Fatalf("Init %s failed: %v", name, err)
		}
	}

	return manager
}

func buildEchoPlugin(t *testing.T, pluginsDir string) string {
	return buildPlugin(t, pluginsDir, "awf-plugin-echo", "awf-plugin-echo")
}

// buildPlugin compiles the plugin source at examples/plugins/<srcName> and places
// the binary at <pluginsDir>/<destName>/<destName>.
func buildPlugin(t *testing.T, pluginsDir, srcName, destName string) string {
	t.Helper()

	examplePath := filepath.Join("..", "..", "examples", "plugins", srcName)
	if _, err := os.Stat(examplePath); err != nil {
		t.Skipf("example plugin %s not found at %s", srcName, examplePath)
	}

	destDir := filepath.Join(pluginsDir, destName)
	require.NoError(t, os.MkdirAll(destDir, 0o755))

	binPath := filepath.Join(destDir, destName)
	cmd := exec.Command("go", "build", "-o", binPath, examplePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build plugin %s: %v\nstderr: %s", destName, err, stderr.String())
	}

	return binPath
}

func setupPluginManifest(t *testing.T, pluginsDir string, name string) {
	t.Helper()

	pluginDir := filepath.Join(pluginsDir, name)
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))

	manifestContent := fmt.Sprintf(`name: %s
version: 1.0.0
awf_version: ">=0.4.0"
capabilities:
  - operations
`, name)

	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.yaml"), []byte(manifestContent), 0o644))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func pluginProcessRunning(binPath string) bool {
	cmd := exec.Command("pgrep", "-f", binPath)
	err := cmd.Run()
	return err == nil
}

func pluginProcessTerminated(binPath string) bool {
	return !pluginProcessRunning(binPath)
}
