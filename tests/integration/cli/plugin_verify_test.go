//go:build integration

package cli_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: F091
// TestPluginVerifyCommand_VerifyAllInstalledPlugins tests the happy path:
// install plugin with checksum → run verify → reports PASS
func TestPluginVerifyCommand_VerifyAllInstalledPlugins(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tempDir := t.TempDir()
	storageDir := filepath.Join(tempDir, "storage")
	pluginsDir := filepath.Join(tempDir, "plugins")

	require.NoError(t, os.MkdirAll(storageDir, 0o755))
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	// Create a test plugin binary
	pluginName := "test-verify"
	pluginDir := filepath.Join(pluginsDir, pluginName)
	pluginBinary := filepath.Join(pluginDir, "awf-plugin-"+pluginName)
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))

	testBinaryContent := []byte("test plugin binary content for verify")
	require.NoError(t, os.WriteFile(pluginBinary, testBinaryContent, 0o755))

	// Initialize state store and compute checksum
	ctx := context.Background()
	store := pluginmgr.NewJSONPluginStateStore(storageDir)

	// Initialize plugin state for the test plugin
	store.SetSourceData(ctx, pluginName, map[string]any{"version": "1.0.0"})

	// Compute and store checksum
	hash := sha256.Sum256(testBinaryContent)
	expectedChecksum := fmt.Sprintf("%x", hash[:])
	require.NoError(t, store.SetChecksum(pluginName, expectedChecksum))
	require.NoError(t, store.Save(ctx))

	// Run verify command - use t.Setenv for inherited environment
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/awf", "plugin", "verify", "--storage", storageDir)
	cmd.Env = os.Environ() // Inherit environment with AWF_PLUGINS_PATH
	cmd.Dir = filepath.Join(os.Getenv("PWD"), "../../..")

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Should succeed (exit code 0)
	require.NoError(t, err, "verify command should succeed: %s", outputStr)

	// Should report PASS
	assert.Contains(t, outputStr, "PASS", "verify should report PASS for matching checksum")
	assert.Contains(t, outputStr, expectedChecksum, "verify should show the checksum")
	assert.Contains(t, outputStr, pluginName, "verify should report plugin name")
}

// Feature: F091
// TestPluginVerifyCommand_VerifyNamedPlugins tests verifying specific plugins
func TestPluginVerifyCommand_VerifyNamedPlugins(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tempDir := t.TempDir()
	storageDir := filepath.Join(tempDir, "storage")
	pluginsDir := filepath.Join(tempDir, "plugins")

	require.NoError(t, os.MkdirAll(storageDir, 0o755))
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	// Create two test plugins
	plugin1Name, plugin1Checksum := "plugin-one", createTestPlugin(t, pluginsDir, "plugin-one", "content one")
	plugin2Name, plugin2Checksum := "plugin-two", createTestPlugin(t, pluginsDir, "plugin-two", "content two")

	// Store checksums
	ctx := context.Background()
	store := pluginmgr.NewJSONPluginStateStore(storageDir)
	store.SetSourceData(ctx, plugin1Name, map[string]any{})
	store.SetSourceData(ctx, plugin2Name, map[string]any{})
	require.NoError(t, store.SetChecksum(plugin1Name, plugin1Checksum))
	require.NoError(t, store.SetChecksum(plugin2Name, plugin2Checksum))
	require.NoError(t, store.Save(ctx))

	// Set plugin path for command execution
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	// Verify only plugin-one
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/awf", "plugin", "verify", "--storage", storageDir, "plugin-one")
	cmd.Env = os.Environ()
	cmd.Dir = filepath.Join(os.Getenv("PWD"), "../../..")

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	require.NoError(t, err, "verify command should succeed: %s", outputStr)
	assert.Contains(t, outputStr, "plugin-one", "should list plugin-one")
	assert.Contains(t, outputStr, "PASS", "plugin-one should pass verification")

	// Verify multiple plugins by name
	cmd = exec.CommandContext(ctx, "go", "run", "./cmd/awf", "plugin", "verify", "--storage", storageDir, "plugin-one", "plugin-two")
	cmd.Env = os.Environ()
	cmd.Dir = filepath.Join(os.Getenv("PWD"), "../../..")

	output, err = cmd.CombinedOutput()
	outputStr = string(output)

	require.NoError(t, err, "verify both plugins: %s", outputStr)
	assert.Contains(t, outputStr, "plugin-one", "should list plugin-one")
	assert.Contains(t, outputStr, "plugin-two", "should list plugin-two")
}

// Feature: F091
// TestPluginVerifyCommand_UpdateFlagStoresChecksum tests --update flag
func TestPluginVerifyCommand_UpdateFlagStoresChecksum(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tempDir := t.TempDir()
	storageDir := filepath.Join(tempDir, "storage")
	pluginsDir := filepath.Join(tempDir, "plugins")

	require.NoError(t, os.MkdirAll(storageDir, 0o755))
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	// Create plugin WITHOUT stored checksum
	pluginName := "plugin-no-checksum"
	pluginBinary := filepath.Join(pluginsDir, pluginName, "awf-plugin-"+pluginName)
	require.NoError(t, os.MkdirAll(filepath.Dir(pluginBinary), 0o755))

	testContent := []byte("plugin without checksum")
	require.NoError(t, os.WriteFile(pluginBinary, testContent, 0o755))

	// Initialize state store without checksum
	ctx := context.Background()
	store := pluginmgr.NewJSONPluginStateStore(storageDir)
	store.SetSourceData(ctx, pluginName, map[string]any{})
	require.NoError(t, store.Save(ctx))

	// Set plugin path for command execution
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	// Run verify --update to store checksum
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/awf", "plugin", "verify", "--storage", storageDir, "--update")
	cmd.Env = os.Environ()
	cmd.Dir = filepath.Join(os.Getenv("PWD"), "../../..")

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	require.NoError(t, err, "verify --update should succeed: %s", outputStr)
	assert.Contains(t, outputStr, "UPDATED", "should report UPDATED status")

	// Verify checksum was stored by loading state again
	store2 := pluginmgr.NewJSONPluginStateStore(storageDir)
	require.NoError(t, store2.Load(ctx))

	storedHash, _, exists := store2.GetChecksum(pluginName)
	require.True(t, exists, "checksum should be stored after --update")

	// Compute expected checksum to verify it matches
	expectedHash := fmt.Sprintf("%x", sha256.Sum256(testContent))
	assert.Equal(t, expectedHash, storedHash, "stored checksum should match computed checksum")
}

// Feature: F091
// TestPluginStateStore_ChecksumRoundtrip tests that checksums persist through Save/Load
func TestPluginStateStore_ChecksumRoundtrip(t *testing.T) {
	tempDir := t.TempDir()
	ctx := context.Background()

	// Create store and set checksum
	store1 := pluginmgr.NewJSONPluginStateStore(tempDir)
	pluginName := "test-roundtrip"
	expectedChecksum := "a1b2c3d4e5f6"
	expectedTimestamp := time.Now().Unix()

	store1.SetSourceData(ctx, pluginName, map[string]any{"version": "1.0"})
	require.NoError(t, store1.SetChecksum(pluginName, expectedChecksum))
	require.NoError(t, store1.Save(ctx))

	// Load in new store instance and verify
	store2 := pluginmgr.NewJSONPluginStateStore(tempDir)
	require.NoError(t, store2.Load(ctx))

	retrievedChecksum, retrievedTimestamp, exists := store2.GetChecksum(pluginName)

	assert.True(t, exists, "checksum should exist after Save/Load roundtrip")
	assert.Equal(t, expectedChecksum, retrievedChecksum, "checksum should match")
	assert.True(t, retrievedTimestamp > 0, "checksum timestamp should be set")
	assert.True(t, retrievedTimestamp >= expectedTimestamp, "timestamp should be current time or later")
}

// Feature: F091
// TestPluginVerifyCommand_TamperedBinaryDetection tests that verify detects tampering
func TestPluginVerifyCommand_TamperedBinaryDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tempDir := t.TempDir()
	storageDir := filepath.Join(tempDir, "storage")
	pluginsDir := filepath.Join(tempDir, "plugins")

	require.NoError(t, os.MkdirAll(storageDir, 0o755))
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	// Create plugin with known checksum
	pluginName := "tamper-test"
	pluginBinary := filepath.Join(pluginsDir, pluginName, "awf-plugin-"+pluginName)
	require.NoError(t, os.MkdirAll(filepath.Dir(pluginBinary), 0o755))

	originalContent := []byte("original plugin binary")
	require.NoError(t, os.WriteFile(pluginBinary, originalContent, 0o755))

	// Store the checksum
	ctx := context.Background()
	store := pluginmgr.NewJSONPluginStateStore(storageDir)
	store.SetSourceData(ctx, pluginName, map[string]any{})

	originalChecksum := fmt.Sprintf("%x", sha256.Sum256(originalContent))
	require.NoError(t, store.SetChecksum(pluginName, originalChecksum))
	require.NoError(t, store.Save(ctx))

	// Tamper with the binary
	tamperedContent := append(originalContent, 0xFF)
	require.NoError(t, os.WriteFile(pluginBinary, tamperedContent, 0o755))

	// Set plugin path for command execution
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	// Run verify - should report FAIL
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/awf", "plugin", "verify", "--storage", storageDir, pluginName)
	cmd.Env = os.Environ()
	cmd.Dir = filepath.Join(os.Getenv("PWD"), "../../..")

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Command should fail (exit 1) when verification fails
	require.Error(t, err, "verify command should fail for tampered binary: %s", outputStr)

	// Should report FAIL with both hashes
	assert.Contains(t, outputStr, "FAIL", "should report FAIL for tampered binary")
	assert.Contains(t, outputStr, "expected="+originalChecksum, "should show expected hash")
	assert.NotContains(t, outputStr, "actual="+originalChecksum, "actual hash should differ from expected")
}

// Feature: F091
// TestPluginVerifyCommand_MissingChecksumReportsMissing tests handling of plugins without checksums
func TestPluginVerifyCommand_MissingChecksumReportsMissing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tempDir := t.TempDir()
	storageDir := filepath.Join(tempDir, "storage")
	pluginsDir := filepath.Join(tempDir, "plugins")

	require.NoError(t, os.MkdirAll(storageDir, 0o755))
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	// Create plugin WITHOUT checksum
	pluginName := "no-checksum-plugin"
	pluginBinary := filepath.Join(pluginsDir, pluginName, "awf-plugin-"+pluginName)
	require.NoError(t, os.MkdirAll(filepath.Dir(pluginBinary), 0o755))
	require.NoError(t, os.WriteFile(pluginBinary, []byte("test content"), 0o755))

	// Initialize state store without checksum
	ctx := context.Background()
	store := pluginmgr.NewJSONPluginStateStore(storageDir)
	store.SetSourceData(ctx, pluginName, map[string]any{})
	require.NoError(t, store.Save(ctx))

	// Set plugin path for command execution
	t.Setenv("AWF_PLUGINS_PATH", pluginsDir)

	// Run verify - should report MISSING
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/awf", "plugin", "verify", "--storage", storageDir, pluginName)
	cmd.Env = os.Environ()
	cmd.Dir = filepath.Join(os.Getenv("PWD"), "../../..")

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Command should fail when plugin is missing checksum
	require.Error(t, err, "verify command should fail for missing checksum: %s", outputStr)
	assert.Contains(t, outputStr, "MISSING", "should report MISSING for plugin without checksum")
	assert.Contains(t, outputStr, pluginName, "should show plugin name")
}

// Helper function to create a test plugin and return its checksum
func createTestPlugin(t *testing.T, pluginsDir, pluginName, content string) string {
	t.Helper()

	pluginDir := filepath.Join(pluginsDir, pluginName)
	pluginBinary := filepath.Join(pluginDir, "awf-plugin-"+pluginName)

	require.NoError(t, os.MkdirAll(pluginDir, 0o755))

	contentBytes := []byte(content)
	require.NoError(t, os.WriteFile(pluginBinary, contentBytes, 0o755))

	hash := sha256.Sum256(contentBytes)
	return fmt.Sprintf("%x", hash[:])
}

// Helper function to compute file checksum
func computePluginChecksum(t *testing.T, filePath string) string {
	t.Helper()

	file, err := os.Open(filePath)
	require.NoError(t, err)
	defer file.Close()

	hash := sha256.New()
	_, err = io.Copy(hash, file)
	require.NoError(t, err)

	return fmt.Sprintf("%x", hash.Sum(nil))
}
