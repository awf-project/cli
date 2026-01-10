package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/plugin"
	"github.com/vanoix/awf/internal/domain/ports"
)

// --- Interface compliance tests ---

func TestJSONPluginStateStore_ImplementsInterface(t *testing.T) {
	var _ ports.PluginStateStore = (*JSONPluginStateStore)(nil)
}

// --- Constructor tests ---

func TestNewJSONPluginStateStore(t *testing.T) {
	store := NewJSONPluginStateStore("/test/path")

	if store == nil {
		t.Fatal("NewJSONPluginStateStore() returned nil")
	}
	if store.basePath != "/test/path" {
		t.Errorf("basePath = %q, want %q", store.basePath, "/test/path")
	}
	if store.states == nil {
		t.Error("states map should be initialized")
	}
}

func TestNewJSONPluginStateStore_EmptyPath(t *testing.T) {
	store := NewJSONPluginStateStore("")

	if store == nil {
		t.Fatal("NewJSONPluginStateStore(\"\") returned nil")
	}
	if store.basePath != "" {
		t.Errorf("basePath = %q, want empty", store.basePath)
	}
}

func TestJSONPluginStateStore_BasePath(t *testing.T) {
	store := NewJSONPluginStateStore("/custom/path")

	if store.BasePath() != "/custom/path" {
		t.Errorf("BasePath() = %q, want %q", store.BasePath(), "/custom/path")
	}
}

// --- Save tests ---

func TestJSONPluginStateStore_Save_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	err := store.Save(ctx)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("Save not yet implemented")
	}
	require.NoError(t, err)

	filePath := filepath.Join(tmpDir, "plugins.json")
	_, statErr := os.Stat(filePath)
	assert.NoError(t, statErr, "plugins.json should be created")
}

func TestJSONPluginStateStore_Save_PersistsStates(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	// Set up some state
	err := store.SetEnabled(ctx, "plugin-a", false)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetEnabled not yet implemented")
	}
	require.NoError(t, err)

	err = store.SetConfig(ctx, "plugin-b", map[string]any{"key": "value"})
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetConfig not yet implemented")
	}
	require.NoError(t, err)

	// Save
	err = store.Save(ctx)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("Save not yet implemented")
	}
	require.NoError(t, err)

	// Load in new store instance
	store2 := NewJSONPluginStateStore(tmpDir)
	err = store2.Load(ctx)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("Load not yet implemented")
	}
	require.NoError(t, err)

	// Verify states persisted
	assert.False(t, store2.IsEnabled("plugin-a"), "plugin-a should be disabled")
	config := store2.GetConfig("plugin-b")
	assert.Equal(t, "value", config["key"], "plugin-b config should be persisted")
}

func TestJSONPluginStateStore_Save_EmptyStates(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	err := store.Save(ctx)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("Save not yet implemented")
	}
	require.NoError(t, err)

	// File should exist and be valid JSON
	filePath := filepath.Join(tmpDir, "plugins.json")
	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "{", "should be valid JSON")
}

func TestJSONPluginStateStore_Save_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "nested", "storage")
	store := NewJSONPluginStateStore(nestedPath)
	ctx := context.Background()

	err := store.Save(ctx)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("Save not yet implemented")
	}
	require.NoError(t, err)

	_, statErr := os.Stat(nestedPath)
	assert.NoError(t, statErr, "nested directory should be created")
}

func TestJSONPluginStateStore_Save_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	err := store.Save(ctx)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("Save not yet implemented")
	}
	require.NoError(t, err)

	// No temp files should remain
	tmpFiles, globErr := filepath.Glob(filepath.Join(tmpDir, "*.tmp"))
	require.NoError(t, globErr)
	assert.Empty(t, tmpFiles, "temp files should be cleaned up")
}

func TestJSONPluginStateStore_Save_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	require.NoError(t, os.MkdirAll(readOnlyDir, 0o755))
	require.NoError(t, os.Chmod(readOnlyDir, 0o444))
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0o755) })

	store := NewJSONPluginStateStore(readOnlyDir)
	ctx := context.Background()

	err := store.Save(ctx)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("Save not yet implemented")
	}
	assert.Error(t, err, "Save should fail on read-only directory")
}

func TestJSONPluginStateStore_Save_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := store.Save(ctx)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("Save not yet implemented")
	}
	if err == nil {
		t.Log("Save succeeded despite cancelled context - acceptable if operation is fast")
	}
}

// --- Load tests ---

func TestJSONPluginStateStore_Load_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	// Create a valid plugins.json file
	data := `{"plugin-a":{"enabled":false,"disabled_at":1234567890},"plugin-b":{"enabled":true,"config":{"key":"value"}}}`
	filePath := filepath.Join(tmpDir, "plugins.json")
	require.NoError(t, os.WriteFile(filePath, []byte(data), 0o600))

	err := store.Load(ctx)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("Load not yet implemented")
	}
	require.NoError(t, err)

	// Verify loaded states
	assert.False(t, store.IsEnabled("plugin-a"), "plugin-a should be disabled")
	assert.True(t, store.IsEnabled("plugin-b"), "plugin-b should be enabled")
	config := store.GetConfig("plugin-b")
	assert.Equal(t, "value", config["key"])
}

func TestJSONPluginStateStore_Load_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	err := store.Load(ctx)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("Load not yet implemented")
	}
	// Should succeed with empty state (no file = no persisted state)
	assert.NoError(t, err, "Load should not error for non-existent file")
}

func TestJSONPluginStateStore_Load_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	// Create invalid JSON
	filePath := filepath.Join(tmpDir, "plugins.json")
	require.NoError(t, os.WriteFile(filePath, []byte("not valid json{"), 0o600))

	err := store.Load(ctx)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("Load not yet implemented")
	}
	assert.Error(t, err, "Load should error on invalid JSON")
}

func TestJSONPluginStateStore_Load_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	// Create empty file
	filePath := filepath.Join(tmpDir, "plugins.json")
	require.NoError(t, os.WriteFile(filePath, []byte(""), 0o600))

	err := store.Load(ctx)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("Load not yet implemented")
	}
	// Empty file should either error or be handled gracefully
	// Implementation decides the behavior
}

func TestJSONPluginStateStore_Load_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	// Create unreadable file
	filePath := filepath.Join(tmpDir, "plugins.json")
	require.NoError(t, os.WriteFile(filePath, []byte("{}"), 0o600))
	require.NoError(t, os.Chmod(filePath, 0o000))
	t.Cleanup(func() { _ = os.Chmod(filePath, 0o644) })

	err := store.Load(ctx)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("Load not yet implemented")
	}
	assert.Error(t, err, "Load should fail on unreadable file")
}

func TestJSONPluginStateStore_Load_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := store.Load(ctx)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("Load not yet implemented")
	}
	// Should either error or succeed quickly
}

// --- SetEnabled tests ---

func TestJSONPluginStateStore_SetEnabled_DisablePlugin(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	err := store.SetEnabled(ctx, "test-plugin", false)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetEnabled not yet implemented")
	}
	require.NoError(t, err)

	assert.False(t, store.IsEnabled("test-plugin"), "plugin should be disabled")
}

func TestJSONPluginStateStore_SetEnabled_EnablePlugin(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	// First disable
	err := store.SetEnabled(ctx, "test-plugin", false)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetEnabled not yet implemented")
	}
	require.NoError(t, err)

	// Then enable
	err = store.SetEnabled(ctx, "test-plugin", true)
	require.NoError(t, err)

	assert.True(t, store.IsEnabled("test-plugin"), "plugin should be enabled")
}

func TestJSONPluginStateStore_SetEnabled_SetsDisabledAt(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	err := store.SetEnabled(ctx, "test-plugin", false)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetEnabled not yet implemented")
	}
	require.NoError(t, err)

	state := store.GetState("test-plugin")
	if state == nil {
		t.Fatal("GetState returned nil after SetEnabled")
	}
	assert.NotZero(t, state.DisabledAt, "DisabledAt should be set when disabling")
}

func TestJSONPluginStateStore_SetEnabled_ClearsDisabledAtOnEnable(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	// Disable first
	err := store.SetEnabled(ctx, "test-plugin", false)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetEnabled not yet implemented")
	}
	require.NoError(t, err)

	// Enable
	err = store.SetEnabled(ctx, "test-plugin", true)
	require.NoError(t, err)

	state := store.GetState("test-plugin")
	if state == nil {
		t.Fatal("GetState returned nil after SetEnabled")
	}
	assert.Zero(t, state.DisabledAt, "DisabledAt should be cleared when enabling")
}

func TestJSONPluginStateStore_SetEnabled_EmptyName(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	err := store.SetEnabled(ctx, "", false)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetEnabled not yet implemented")
	}
	// Should either error or accept empty name - implementation decides
}

func TestJSONPluginStateStore_SetEnabled_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := store.SetEnabled(ctx, "test-plugin", false)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetEnabled not yet implemented")
	}
	// Should either error or succeed quickly
}

// --- IsEnabled tests ---

func TestJSONPluginStateStore_IsEnabled_UnknownPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)

	// Unknown plugins should be enabled by default
	assert.True(t, store.IsEnabled("unknown-plugin"), "unknown plugins should be enabled by default")
}

func TestJSONPluginStateStore_IsEnabled_DisabledPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	err := store.SetEnabled(ctx, "disabled-plugin", false)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetEnabled not yet implemented")
	}
	require.NoError(t, err)

	assert.False(t, store.IsEnabled("disabled-plugin"))
}

func TestJSONPluginStateStore_IsEnabled_EnabledPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	// Explicitly enable (should be redundant but explicit)
	err := store.SetEnabled(ctx, "enabled-plugin", true)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetEnabled not yet implemented")
	}
	require.NoError(t, err)

	assert.True(t, store.IsEnabled("enabled-plugin"))
}

func TestJSONPluginStateStore_IsEnabled_EmptyName(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)

	// Empty name should return default (enabled)
	assert.True(t, store.IsEnabled(""))
}

// --- GetConfig tests ---

func TestJSONPluginStateStore_GetConfig_UnknownPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)

	config := store.GetConfig("unknown-plugin")
	assert.Nil(t, config, "unknown plugins should have nil config")
}

func TestJSONPluginStateStore_GetConfig_ExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	testConfig := map[string]any{
		"string_key": "value",
		"int_key":    42,
		"bool_key":   true,
	}

	err := store.SetConfig(ctx, "test-plugin", testConfig)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetConfig not yet implemented")
	}
	require.NoError(t, err)

	config := store.GetConfig("test-plugin")
	assert.Equal(t, "value", config["string_key"])
	// Note: JSON unmarshaling converts int to float64
}

func TestJSONPluginStateStore_GetConfig_EmptyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	err := store.SetConfig(ctx, "test-plugin", map[string]any{})
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetConfig not yet implemented")
	}
	require.NoError(t, err)

	config := store.GetConfig("test-plugin")
	assert.NotNil(t, config, "empty config should not be nil")
	assert.Empty(t, config, "config should be empty")
}

// --- SetConfig tests ---

func TestJSONPluginStateStore_SetConfig_NewPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	testConfig := map[string]any{"key": "value"}
	err := store.SetConfig(ctx, "new-plugin", testConfig)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetConfig not yet implemented")
	}
	require.NoError(t, err)

	config := store.GetConfig("new-plugin")
	assert.Equal(t, "value", config["key"])
}

func TestJSONPluginStateStore_SetConfig_OverwriteExisting(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	// Set initial config
	err := store.SetConfig(ctx, "test-plugin", map[string]any{"old": "value"})
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetConfig not yet implemented")
	}
	require.NoError(t, err)

	// Overwrite with new config
	err = store.SetConfig(ctx, "test-plugin", map[string]any{"new": "data"})
	require.NoError(t, err)

	config := store.GetConfig("test-plugin")
	assert.Nil(t, config["old"], "old config should be overwritten")
	assert.Equal(t, "data", config["new"])
}

func TestJSONPluginStateStore_SetConfig_NilConfig(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	err := store.SetConfig(ctx, "test-plugin", nil)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetConfig not yet implemented")
	}
	// Should either clear config or error - implementation decides
}

func TestJSONPluginStateStore_SetConfig_PreservesEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	// Disable plugin first
	err := store.SetEnabled(ctx, "test-plugin", false)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetEnabled not yet implemented")
	}
	require.NoError(t, err)

	// Set config
	err = store.SetConfig(ctx, "test-plugin", map[string]any{"key": "value"})
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetConfig not yet implemented")
	}
	require.NoError(t, err)

	// Verify enabled state preserved
	assert.False(t, store.IsEnabled("test-plugin"), "enabled state should be preserved after SetConfig")
}

func TestJSONPluginStateStore_SetConfig_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := store.SetConfig(ctx, "test-plugin", map[string]any{"key": "value"})
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetConfig not yet implemented")
	}
	// Should either error or succeed quickly
}

// --- GetState tests ---

func TestJSONPluginStateStore_GetState_UnknownPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)

	state := store.GetState("unknown-plugin")
	assert.Nil(t, state, "unknown plugins should return nil state")
}

func TestJSONPluginStateStore_GetState_ExistingPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	err := store.SetEnabled(ctx, "test-plugin", false)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetEnabled not yet implemented")
	}
	require.NoError(t, err)

	state := store.GetState("test-plugin")
	require.NotNil(t, state)
	assert.False(t, state.Enabled)
}

func TestJSONPluginStateStore_GetState_WithConfig(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	err := store.SetConfig(ctx, "test-plugin", map[string]any{"key": "value"})
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetConfig not yet implemented")
	}
	require.NoError(t, err)

	state := store.GetState("test-plugin")
	require.NotNil(t, state)
	assert.Equal(t, "value", state.Config["key"])
}

// --- ListDisabled tests ---

func TestJSONPluginStateStore_ListDisabled_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)

	disabled := store.ListDisabled()
	assert.Empty(t, disabled, "should return empty list when no plugins disabled")
}

func TestJSONPluginStateStore_ListDisabled_SingleDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	err := store.SetEnabled(ctx, "disabled-plugin", false)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetEnabled not yet implemented")
	}
	require.NoError(t, err)

	disabled := store.ListDisabled()
	assert.Len(t, disabled, 1)
	assert.Contains(t, disabled, "disabled-plugin")
}

func TestJSONPluginStateStore_ListDisabled_MultipleDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	for _, name := range []string{"plugin-a", "plugin-b", "plugin-c"} {
		err := store.SetEnabled(ctx, name, false)
		if errors.Is(err, ErrStateStoreNotImplemented) {
			t.Skip("SetEnabled not yet implemented")
		}
		require.NoError(t, err)
	}

	disabled := store.ListDisabled()
	assert.Len(t, disabled, 3)
	assert.Contains(t, disabled, "plugin-a")
	assert.Contains(t, disabled, "plugin-b")
	assert.Contains(t, disabled, "plugin-c")
}

func TestJSONPluginStateStore_ListDisabled_MixedEnabledDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	// Disable some plugins
	err := store.SetEnabled(ctx, "disabled-1", false)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetEnabled not yet implemented")
	}
	require.NoError(t, err)

	err = store.SetEnabled(ctx, "disabled-2", false)
	require.NoError(t, err)

	// Enable a plugin (explicitly)
	err = store.SetEnabled(ctx, "enabled-1", true)
	require.NoError(t, err)

	disabled := store.ListDisabled()
	assert.Len(t, disabled, 2)
	assert.Contains(t, disabled, "disabled-1")
	assert.Contains(t, disabled, "disabled-2")
	assert.NotContains(t, disabled, "enabled-1")
}

// --- Concurrency tests ---

func TestJSONPluginStateStore_ConcurrentSetEnabled(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("plugin-%d", n)
			err := store.SetEnabled(ctx, name, n%2 == 0)
			if errors.Is(err, ErrStateStoreNotImplemented) {
				return // Skip if not implemented
			}
		}(i)
	}

	wg.Wait()

	// Verify no race conditions
	disabled := store.ListDisabled()
	_ = disabled // Should not panic
}

func TestJSONPluginStateStore_ConcurrentSetConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("plugin-%d", n)
			config := map[string]any{"iteration": n}
			err := store.SetConfig(ctx, name, config)
			if errors.Is(err, ErrStateStoreNotImplemented) {
				return // Skip if not implemented
			}
		}(i)
	}

	wg.Wait()
}

func TestJSONPluginStateStore_ConcurrentReadWrite(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	const iterations = 50
	var wg sync.WaitGroup

	// Writers
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("plugin-%d", n%5)
			err := store.SetEnabled(ctx, name, n%2 == 0)
			if errors.Is(err, ErrStateStoreNotImplemented) {
				return
			}
		}(i)
	}

	// Readers
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("plugin-%d", n%5)
			_ = store.IsEnabled(name)
			_ = store.GetConfig(name)
			_ = store.GetState(name)
			_ = store.ListDisabled()
		}(i)
	}

	wg.Wait()
}

func TestJSONPluginStateStore_ConcurrentSaveLoad(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	const iterations = 20
	var wg sync.WaitGroup

	// Savers
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go func(n int) {
			defer wg.Done()
			// Set some state first
			name := fmt.Sprintf("plugin-%d", n)
			_ = store.SetEnabled(ctx, name, n%2 == 0)
			err := store.Save(ctx)
			if errors.Is(err, ErrStateStoreNotImplemented) {
				return
			}
		}(i)
	}

	// Loaders
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			err := store.Load(ctx)
			if errors.Is(err, ErrStateStoreNotImplemented) {
				return
			}
		}()
	}

	wg.Wait()

	// Final state should be consistent
	disabled := store.ListDisabled()
	_ = disabled // Should not panic
}

// --- Domain entity tests ---

func TestNewPluginState(t *testing.T) {
	state := plugin.NewPluginState()

	if state == nil {
		t.Fatal("NewPluginState() returned nil")
	}
	if !state.Enabled {
		t.Error("new PluginState should be enabled by default")
	}
	if state.Config == nil {
		t.Error("new PluginState should have initialized Config map")
	}
	if state.DisabledAt != 0 {
		t.Error("new PluginState should have zero DisabledAt")
	}
}

func TestPluginState_Fields(t *testing.T) {
	state := &plugin.PluginState{
		Enabled:    false,
		Config:     map[string]any{"key": "value"},
		DisabledAt: 1234567890,
	}

	assert.False(t, state.Enabled)
	assert.Equal(t, "value", state.Config["key"])
	assert.Equal(t, int64(1234567890), state.DisabledAt)
}

// --- Table-driven edge case tests ---

func TestJSONPluginStateStore_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		pluginName string
		wantErr    bool
	}{
		{"normal-name", "my-plugin", false},
		{"hyphenated", "my-awesome-plugin", false},
		{"underscored", "my_plugin", false},
		{"with-numbers", "plugin123", false},
		{"empty-name", "", false}, // Empty name handled gracefully
		{"special-chars", "plugin@name", false},
		{"unicode", "plugin-日本語", false},
		{"very-long-name", "this-is-a-very-long-plugin-name-that-might-cause-issues-with-file-paths-or-storage-systems-123456789", false},
	}

	tmpDir := t.TempDir()
	store := NewJSONPluginStateStore(tmpDir)
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.SetEnabled(ctx, tt.pluginName, false)
			if errors.Is(err, ErrStateStoreNotImplemented) {
				t.Skip("SetEnabled not yet implemented")
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("SetEnabled() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				assert.False(t, store.IsEnabled(tt.pluginName))
			}
		})
	}
}

// --- Roundtrip test ---

func TestJSONPluginStateStore_Roundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create and populate store
	store1 := NewJSONPluginStateStore(tmpDir)

	err := store1.SetEnabled(ctx, "plugin-a", false)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetEnabled not yet implemented")
	}
	require.NoError(t, err)

	err = store1.SetEnabled(ctx, "plugin-b", true)
	require.NoError(t, err)

	err = store1.SetConfig(ctx, "plugin-c", map[string]any{
		"string": "value",
		"number": 42,
		"bool":   true,
		"nested": map[string]any{"inner": "data"},
	})
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("SetConfig not yet implemented")
	}
	require.NoError(t, err)

	// Save
	err = store1.Save(ctx)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("Save not yet implemented")
	}
	require.NoError(t, err)

	// Load in new store instance
	store2 := NewJSONPluginStateStore(tmpDir)
	err = store2.Load(ctx)
	if errors.Is(err, ErrStateStoreNotImplemented) {
		t.Skip("Load not yet implemented")
	}
	require.NoError(t, err)

	// Verify all states match
	assert.False(t, store2.IsEnabled("plugin-a"), "plugin-a should be disabled")
	assert.True(t, store2.IsEnabled("plugin-b"), "plugin-b should be enabled")

	config := store2.GetConfig("plugin-c")
	require.NotNil(t, config)
	assert.Equal(t, "value", config["string"])

	disabled := store2.ListDisabled()
	assert.Len(t, disabled, 1)
	assert.Contains(t, disabled, "plugin-a")
}
