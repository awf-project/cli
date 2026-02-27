package pluginmgr

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
)

// --- Constructor Tests ---

func TestNewRPCPluginManager(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)

	if manager == nil {
		t.Fatal("NewRPCPluginManager() returned nil")
	}
	if manager.loader != loader {
		t.Error("NewRPCPluginManager() did not set loader")
	}
	if manager.plugins == nil {
		t.Error("NewRPCPluginManager() did not initialize plugins map")
	}
}

func TestNewRPCPluginManager_NilLoader(t *testing.T) {
	manager := NewRPCPluginManager(nil)

	if manager == nil {
		t.Fatal("NewRPCPluginManager(nil) returned nil")
	}
	if manager.loader != nil {
		t.Error("NewRPCPluginManager(nil) should set loader to nil")
	}
	if manager.plugins == nil {
		t.Error("NewRPCPluginManager(nil) should still initialize plugins map")
	}
}

// --- Discover Tests ---

func TestRPCPluginManager_Discover_ReturnsNotImplemented(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	ctx := context.Background()

	plugins, err := manager.Discover(ctx)

	// Current stub returns ErrRPCNotImplemented
	if !errors.Is(err, ErrRPCNotImplemented) {
		t.Errorf("Discover() error = %v, want ErrRPCNotImplemented", err)
	}
	if plugins != nil {
		t.Errorf("Discover() plugins = %v, want nil for stub", plugins)
	}
}

func TestRPCPluginManager_Discover_WithValidLoader(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath) // Configure plugins directory
	ctx := context.Background()

	plugins, err := manager.Discover(ctx)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	// Should return plugins from fixturesPath
	if plugins == nil {
		t.Error("Discover() returned nil plugins")
	}

	// Should find at least 2 valid plugins
	if len(plugins) < 2 {
		t.Errorf("Discover() found %d plugins, want at least 2", len(plugins))
	}

	// All returned plugins should be stored in manager
	for _, p := range plugins {
		if p.Manifest == nil || p.Manifest.Name == "" {
			continue
		}
		info, found := manager.Get(p.Manifest.Name)
		if !found {
			t.Errorf("Plugin %q not found in manager after Discover", p.Manifest.Name)
		}
		if info != p {
			t.Errorf("Plugin %q info mismatch after Discover", p.Manifest.Name)
		}
	}
}

func TestRPCPluginManager_Discover_ContextCancellation(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := manager.Discover(ctx)
	if err == nil {
		t.Fatal("Discover() error = nil, want error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Discover() error = %v, want context.Canceled", err)
	}
}

func TestRPCPluginManager_Discover_ContextTimeout(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond) // Ensure timeout expires

	_, err := manager.Discover(ctx)
	if err == nil {
		t.Fatal("Discover() error = nil, want error for timed out context")
	}
}

// --- Load Tests ---

func TestRPCPluginManager_Load_ReturnsNotImplemented(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	ctx := context.Background()

	err := manager.Load(ctx, "test-plugin")

	// Current stub returns ErrRPCNotImplemented
	if !errors.Is(err, ErrRPCNotImplemented) {
		t.Errorf("Load() error = %v, want ErrRPCNotImplemented", err)
	}
}

func TestRPCPluginManager_Load_ValidPlugin(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	err := manager.Load(ctx, "valid-simple")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// After loading, plugin should be accessible via Get
	info, found := manager.Get("valid-simple")
	if !found {
		t.Fatal("Get() found = false, want true after Load")
	}
	if info == nil {
		t.Fatal("Get() info = nil, want non-nil after Load")
	}
	if info.Status != pluginmodel.StatusLoaded {
		t.Errorf("Status = %q, want %q", info.Status, pluginmodel.StatusLoaded)
	}
}

func TestRPCPluginManager_Load_NonExistentPlugin(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	err := manager.Load(ctx, "nonexistent-plugin")
	if err == nil {
		t.Fatal("Load() error = nil, want error for non-existent plugin")
	}
}

func TestRPCPluginManager_Load_EmptyName(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	err := manager.Load(ctx, "")
	if err == nil {
		t.Fatal("Load() error = nil, want error for empty name")
	}
	// Error should indicate plugin name is required
	var mgrErr *RPCManagerError
	if errors.As(err, &mgrErr) {
		if mgrErr.Op != "load" {
			t.Errorf("RPCManagerError.Op = %q, want %q", mgrErr.Op, "load")
		}
	}
}

func TestRPCPluginManager_Load_ContextCancellation(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := manager.Load(ctx, "valid-simple")
	if err == nil {
		t.Fatal("Load() error = nil, want error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Load() error = %v, want context.Canceled", err)
	}
}

func TestRPCPluginManager_Load_AlreadyLoaded(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	// Load once
	err := manager.Load(ctx, "valid-simple")
	if err != nil {
		t.Fatalf("First Load() error = %v", err)
	}

	// Load again - should succeed silently (idempotent)
	err = manager.Load(ctx, "valid-simple")
	if err != nil {
		t.Fatalf("Second Load() error = %v, want nil (idempotent)", err)
	}

	// Plugin should still be loaded
	info, found := manager.Get("valid-simple")
	if !found {
		t.Error("Get() found = false after second Load")
	}
	if info.Status != pluginmodel.StatusLoaded {
		t.Errorf("Status = %q, want %q after second Load", info.Status, pluginmodel.StatusLoaded)
	}
}

// --- Init Tests ---

func TestRPCPluginManager_Init_ReturnsNotImplemented(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	ctx := context.Background()

	err := manager.Init(ctx, "test-plugin", nil)

	// Returns ErrRPCNotImplemented when loader not configured
	if !errors.Is(err, ErrRPCNotImplemented) {
		t.Errorf("Init() error = %v, want ErrRPCNotImplemented", err)
	}
}

func TestRPCPluginManager_Init_WithConfig(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	config := map[string]any{
		"webhook_url": "https://hooks.slack.com/...",
		"channel":     "#alerts",
	}

	// Must load before init
	err := manager.Load(ctx, "valid-full")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	err = manager.Init(ctx, "valid-full", config)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// After init, plugin status should be Running
	info, found := manager.Get("valid-full")
	if !found {
		t.Fatal("Get() found = false after Init")
	}
	if info.Status != pluginmodel.StatusRunning {
		t.Errorf("Status = %q, want %q after Init", info.Status, pluginmodel.StatusRunning)
	}
}

func TestRPCPluginManager_Init_NotLoaded(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	err := manager.Init(ctx, "not-loaded-plugin", nil)
	if err == nil {
		t.Fatal("Init() error = nil, want error for plugin not loaded")
	}
	var mgrErr *RPCManagerError
	if errors.As(err, &mgrErr) {
		if mgrErr.Op != "init" {
			t.Errorf("RPCManagerError.Op = %q, want %q", mgrErr.Op, "init")
		}
	}
}

func TestRPCPluginManager_Init_EmptyName(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	err := manager.Init(ctx, "", nil)
	if err == nil {
		t.Fatal("Init() error = nil, want error for empty name")
	}
	var mgrErr *RPCManagerError
	if errors.As(err, &mgrErr) {
		if mgrErr.Op != "init" {
			t.Errorf("RPCManagerError.Op = %q, want %q", mgrErr.Op, "init")
		}
	}
}

func TestRPCPluginManager_Init_NilConfig(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	err := manager.Load(ctx, "valid-simple")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Init with nil config should work for plugins without required config
	err = manager.Init(ctx, "valid-simple", nil)
	if err != nil {
		t.Fatalf("Init() error = %v, want nil for plugin without required config", err)
	}
}

func TestRPCPluginManager_Init_ContextCancellation(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := manager.Init(ctx, "valid-simple", nil)
	if err == nil {
		t.Fatal("Init() error = nil, want error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Init() error = %v, want context.Canceled", err)
	}
}

func TestRPCPluginManager_Init_AlreadyRunning(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	// Load and init
	if err := manager.Load(ctx, "valid-simple"); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := manager.Init(ctx, "valid-simple", nil); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Init again - should succeed silently (idempotent)
	err := manager.Init(ctx, "valid-simple", nil)
	if err != nil {
		t.Fatalf("Second Init() error = %v, want nil (idempotent)", err)
	}
}

// --- Shutdown Tests ---

func TestRPCPluginManager_Shutdown_ReturnsNotImplemented(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	ctx := context.Background()

	err := manager.Shutdown(ctx, "test-plugin")

	// Returns ErrRPCNotImplemented when loader not configured
	if !errors.Is(err, ErrRPCNotImplemented) {
		t.Errorf("Shutdown() error = %v, want ErrRPCNotImplemented", err)
	}
}

func TestRPCPluginManager_Shutdown_RunningPlugin(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	// Load and init first
	if err := manager.Load(ctx, "valid-simple"); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := manager.Init(ctx, "valid-simple", nil); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Verify plugin is running
	info, _ := manager.Get("valid-simple")
	if info.Status != pluginmodel.StatusRunning {
		t.Fatalf("Plugin status = %q, want %q before shutdown", info.Status, pluginmodel.StatusRunning)
	}

	err := manager.Shutdown(ctx, "valid-simple")
	if err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	// After shutdown, plugin should be stopped
	info, found := manager.Get("valid-simple")
	if !found {
		t.Fatal("Plugin not found after Shutdown")
	}
	if info.Status == pluginmodel.StatusRunning {
		t.Error("Plugin still running after Shutdown")
	}
	if info.Status != pluginmodel.StatusStopped {
		t.Errorf("Status = %q, want %q after Shutdown", info.Status, pluginmodel.StatusStopped)
	}
}

func TestRPCPluginManager_Shutdown_NotRunning(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	// Shutdown non-existent plugin - should succeed silently (no-op)
	err := manager.Shutdown(ctx, "not-running-plugin")
	if err != nil {
		t.Fatalf("Shutdown() error = %v, want nil for non-existent plugin", err)
	}
}

func TestRPCPluginManager_Shutdown_EmptyName(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	err := manager.Shutdown(ctx, "")
	if err == nil {
		t.Fatal("Shutdown() error = nil, want error for empty name")
	}
	var mgrErr *RPCManagerError
	if errors.As(err, &mgrErr) {
		if mgrErr.Op != "shutdown" {
			t.Errorf("RPCManagerError.Op = %q, want %q", mgrErr.Op, "shutdown")
		}
	}
}

func TestRPCPluginManager_Shutdown_ContextCancellation(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := manager.Shutdown(ctx, "valid-simple")
	if err == nil {
		t.Fatal("Shutdown() error = nil, want error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Shutdown() error = %v, want context.Canceled", err)
	}
}

func TestRPCPluginManager_Shutdown_AlreadyStopped(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	// Load, init, shutdown
	if err := manager.Load(ctx, "valid-simple"); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := manager.Init(ctx, "valid-simple", nil); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if err := manager.Shutdown(ctx, "valid-simple"); err != nil {
		t.Fatalf("First Shutdown() error = %v", err)
	}

	// Shutdown again - should succeed silently (idempotent)
	err := manager.Shutdown(ctx, "valid-simple")
	if err != nil {
		t.Fatalf("Second Shutdown() error = %v, want nil (idempotent)", err)
	}
}

// --- ShutdownAll Tests ---

func TestRPCPluginManager_ShutdownAll_ReturnsNotImplemented(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	ctx := context.Background()

	err := manager.ShutdownAll(ctx)

	// Returns ErrRPCNotImplemented when loader not configured
	if !errors.Is(err, ErrRPCNotImplemented) {
		t.Errorf("ShutdownAll() error = %v, want ErrRPCNotImplemented", err)
	}
}

func TestRPCPluginManager_ShutdownAll_MultiplePlugins(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	// Load and init multiple plugins
	pluginDirs := []string{"valid-simple", "valid-full"}
	for _, dir := range pluginDirs {
		if err := manager.Load(ctx, dir); err != nil {
			t.Fatalf("Load(%s) error = %v", dir, err)
		}
		if err := manager.Init(ctx, dir, nil); err != nil {
			t.Fatalf("Init(%s) error = %v", dir, err)
		}
	}

	// Verify plugins are running
	for _, dir := range pluginDirs {
		info, _ := manager.Get(dir)
		if info.Status != pluginmodel.StatusRunning {
			t.Fatalf("Plugin %q status = %q, want %q before ShutdownAll", dir, info.Status, pluginmodel.StatusRunning)
		}
	}

	err := manager.ShutdownAll(ctx)
	if err != nil {
		t.Fatalf("ShutdownAll() error = %v", err)
	}

	// All plugins should be stopped
	for _, dir := range pluginDirs {
		info, found := manager.Get(dir)
		if !found {
			t.Errorf("Plugin %q not found after ShutdownAll", dir)
			continue
		}
		if info.Status == pluginmodel.StatusRunning {
			t.Errorf("Plugin %q still running after ShutdownAll", dir)
		}
		if info.Status != pluginmodel.StatusStopped {
			t.Errorf("Plugin %q status = %q, want %q after ShutdownAll", dir, info.Status, pluginmodel.StatusStopped)
		}
	}
}

func TestRPCPluginManager_ShutdownAll_NoPlugins(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	// Should succeed (no-op) when no plugins are loaded
	err := manager.ShutdownAll(ctx)
	if err != nil {
		t.Fatalf("ShutdownAll() error = %v, want nil for empty manager", err)
	}
}

func TestRPCPluginManager_ShutdownAll_ContextCancellation(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := manager.ShutdownAll(ctx)
	if err == nil {
		t.Fatal("ShutdownAll() error = nil, want error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("ShutdownAll() error = %v, want context.Canceled", err)
	}
}

func TestRPCPluginManager_ShutdownAll_MixedStates(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	// Load two plugins, init only one
	if err := manager.Load(ctx, "valid-simple"); err != nil {
		t.Fatalf("Load(valid-simple) error = %v", err)
	}
	if err := manager.Load(ctx, "valid-full"); err != nil {
		t.Fatalf("Load(valid-full) error = %v", err)
	}
	if err := manager.Init(ctx, "valid-simple", nil); err != nil {
		t.Fatalf("Init(valid-simple) error = %v", err)
	}

	// One running, one loaded
	err := manager.ShutdownAll(ctx)
	if err != nil {
		t.Fatalf("ShutdownAll() error = %v", err)
	}

	// Running plugin should be stopped
	info, _ := manager.Get("valid-simple")
	if info.Status == pluginmodel.StatusRunning {
		t.Error("Running plugin still running after ShutdownAll")
	}
}

// --- Get Tests ---

func TestRPCPluginManager_Get_NotFound(t *testing.T) {
	manager := NewRPCPluginManager(nil)

	info, found := manager.Get("nonexistent")

	if found {
		t.Error("Get() found = true, want false for nonexistent plugin")
	}
	if info != nil {
		t.Errorf("Get() info = %v, want nil for nonexistent plugin", info)
	}
}

func TestRPCPluginManager_Get_EmptyName(t *testing.T) {
	manager := NewRPCPluginManager(nil)

	info, found := manager.Get("")

	if found {
		t.Error("Get() found = true, want false for empty name")
	}
	if info != nil {
		t.Errorf("Get() info = %v, want nil for empty name", info)
	}
}

func TestRPCPluginManager_Get_AfterDirectInsert(t *testing.T) {
	// Test the Get implementation directly by inserting into the map
	manager := NewRPCPluginManager(nil)

	testInfo := &pluginmodel.PluginInfo{
		Manifest: &pluginmodel.Manifest{
			Name:    "test-plugin",
			Version: "1.0.0",
		},
		Status: pluginmodel.StatusLoaded,
		Path:   "/plugins/test-plugin",
	}

	// Directly insert into the map (simulating what Load would do)
	manager.mu.Lock()
	manager.plugins["test-plugin"] = testInfo
	manager.mu.Unlock()

	info, found := manager.Get("test-plugin")
	if !found {
		t.Fatal("Get() found = false, want true")
	}
	if info == nil {
		t.Fatal("Get() info = nil, want non-nil")
	}
	if info.Manifest.Name != "test-plugin" {
		t.Errorf("Get() info.Manifest.Name = %q, want %q", info.Manifest.Name, "test-plugin")
	}
}

// --- List Tests ---

func TestRPCPluginManager_List_Empty(t *testing.T) {
	manager := NewRPCPluginManager(nil)

	plugins := manager.List()

	if plugins == nil {
		t.Fatal("List() returned nil, want empty slice")
	}
	if len(plugins) != 0 {
		t.Errorf("List() returned %d plugins, want 0", len(plugins))
	}
}

func TestRPCPluginManager_List_AfterDirectInsert(t *testing.T) {
	manager := NewRPCPluginManager(nil)

	// Insert test plugins
	testPlugins := []*pluginmodel.PluginInfo{
		{
			Manifest: &pluginmodel.Manifest{Name: "plugin-a", Version: "1.0.0"},
			Status:   pluginmodel.StatusLoaded,
		},
		{
			Manifest: &pluginmodel.Manifest{Name: "plugin-b", Version: "2.0.0"},
			Status:   pluginmodel.StatusRunning,
		},
	}

	manager.mu.Lock()
	for _, p := range testPlugins {
		manager.plugins[p.Manifest.Name] = p
	}
	manager.mu.Unlock()

	plugins := manager.List()

	if len(plugins) != 2 {
		t.Fatalf("List() returned %d plugins, want 2", len(plugins))
	}

	// Check all plugins are returned (order may vary due to map iteration)
	names := make(map[string]bool)
	for _, p := range plugins {
		names[p.Manifest.Name] = true
	}
	if !names["plugin-a"] || !names["plugin-b"] {
		t.Error("List() did not return all plugins")
	}
}

func TestRPCPluginManager_List_ReturnsNewSlice(t *testing.T) {
	manager := NewRPCPluginManager(nil)

	// Insert a test plugin
	manager.mu.Lock()
	manager.plugins["test"] = &pluginmodel.PluginInfo{
		Manifest: &pluginmodel.Manifest{Name: "test"},
	}
	manager.mu.Unlock()

	list1 := manager.List()
	list2 := manager.List()

	// Modifying list1 should not affect list2
	if len(list1) > 0 {
		list1[0] = nil
	}
	if len(list2) > 0 && list2[0] == nil {
		t.Error("List() returns shared slice, want independent copy")
	}
}

// --- Concurrency Tests ---

func TestRPCPluginManager_ConcurrentGet(t *testing.T) {
	manager := NewRPCPluginManager(nil)

	// Insert test plugin
	manager.mu.Lock()
	manager.plugins["concurrent-test"] = &pluginmodel.PluginInfo{
		Manifest: &pluginmodel.Manifest{Name: "concurrent-test"},
	}
	manager.mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = manager.Get("concurrent-test")
		}()
	}
	wg.Wait()
	// Test passes if no race condition detected
}

func TestRPCPluginManager_ConcurrentList(t *testing.T) {
	manager := NewRPCPluginManager(nil)

	// Insert test plugins
	manager.mu.Lock()
	for i := 0; i < 10; i++ {
		name := "plugin-" + string(rune('a'+i))
		manager.plugins[name] = &pluginmodel.PluginInfo{
			Manifest: &pluginmodel.Manifest{Name: name},
		}
	}
	manager.mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = manager.List()
		}()
	}
	wg.Wait()
	// Test passes if no race condition detected
}

func TestRPCPluginManager_ConcurrentGetAndList(t *testing.T) {
	manager := NewRPCPluginManager(nil)

	// Insert test plugin
	manager.mu.Lock()
	manager.plugins["test"] = &pluginmodel.PluginInfo{
		Manifest: &pluginmodel.Manifest{Name: "test"},
	}
	manager.mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = manager.Get("test")
		}()
		go func() {
			defer wg.Done()
			_ = manager.List()
		}()
	}
	wg.Wait()
	// Test passes if no race condition detected
}

// --- RPCManagerError Tests ---

func TestRPCManagerError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *RPCManagerError
		contains []string
	}{
		{
			name: "with plugin name",
			err: &RPCManagerError{
				Op:      "load",
				Plugin:  "my-plugin",
				Message: "binary not found",
			},
			contains: []string{"load", "my-plugin", "binary not found"},
		},
		{
			name: "without plugin name",
			err: &RPCManagerError{
				Op:      "discover",
				Message: "directory not found",
			},
			contains: []string{"discover", "directory not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errStr := tt.err.Error()
			for _, s := range tt.contains {
				if !strings.Contains(errStr, s) {
					t.Errorf("Error() = %q, should contain %q", errStr, s)
				}
			}
		})
	}
}

func TestRPCManagerError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &RPCManagerError{
		Op:      "init",
		Plugin:  "test-plugin",
		Message: "failed",
		Cause:   cause,
	}

	unwrapped := err.Unwrap()
	if unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}
}

func TestRPCManagerError_Unwrap_NilCause(t *testing.T) {
	err := &RPCManagerError{
		Op:      "shutdown",
		Plugin:  "test-plugin",
		Message: "not running",
	}

	unwrapped := err.Unwrap()
	if unwrapped != nil {
		t.Errorf("Unwrap() = %v, want nil", unwrapped)
	}
}

func TestNewRPCManagerError(t *testing.T) {
	err := NewRPCManagerError("load", "my-plugin", "not found")

	if err.Op != "load" {
		t.Errorf("Op = %q, want %q", err.Op, "load")
	}
	if err.Plugin != "my-plugin" {
		t.Errorf("Plugin = %q, want %q", err.Plugin, "my-plugin")
	}
	if err.Message != "not found" {
		t.Errorf("Message = %q, want %q", err.Message, "not found")
	}
	if err.Cause != nil {
		t.Errorf("Cause = %v, want nil", err.Cause)
	}
}

func TestWrapRPCManagerError(t *testing.T) {
	cause := errors.New("io error")
	err := WrapRPCManagerError("init", "my-plugin", cause)

	if err.Op != "init" {
		t.Errorf("Op = %q, want %q", err.Op, "init")
	}
	if err.Plugin != "my-plugin" {
		t.Errorf("Plugin = %q, want %q", err.Plugin, "my-plugin")
	}
	if err.Message != "io error" {
		t.Errorf("Message = %q, want %q", err.Message, "io error")
	}
	if err.Cause != cause {
		t.Errorf("Cause = %v, want %v", err.Cause, cause)
	}
}

func TestRPCManagerError_ErrorsIs(t *testing.T) {
	cause := errors.New("original error")
	err := WrapRPCManagerError("load", "plugin", cause)

	if !errors.Is(err, cause) {
		t.Error("errors.Is(err, cause) = false, want true")
	}
}

// --- Interface Compliance Tests ---

func TestRPCPluginManager_ImplementsPluginManager(t *testing.T) {
	// Compile-time check that RPCPluginManager implements the interface
	var _ interface {
		Discover(context.Context) ([]*pluginmodel.PluginInfo, error)
		Load(context.Context, string) error
		Init(context.Context, string, map[string]any) error
		Shutdown(context.Context, string) error
		ShutdownAll(context.Context) error
		Get(string) (*pluginmodel.PluginInfo, bool)
		List() []*pluginmodel.PluginInfo
	} = (*RPCPluginManager)(nil)
}

// --- Edge Case Tests ---

func TestRPCPluginManager_Get_SpecialCharacters(t *testing.T) {
	manager := NewRPCPluginManager(nil)

	// Test with various edge case names
	testCases := []string{
		"plugin-with-dashes",
		"plugin_with_underscores",
		"plugin123",
		"UPPERCASE",
		"MixedCase",
	}

	for _, name := range testCases {
		info, found := manager.Get(name)
		if found {
			t.Errorf("Get(%q) found = true, want false", name)
		}
		if info != nil {
			t.Errorf("Get(%q) info = %v, want nil", name, info)
		}
	}
}

func TestRPCPluginManager_List_Order(t *testing.T) {
	manager := NewRPCPluginManager(nil)

	// Insert plugins in specific order
	names := []string{"zebra", "alpha", "middle"}
	manager.mu.Lock()
	for _, name := range names {
		manager.plugins[name] = &pluginmodel.PluginInfo{
			Manifest: &pluginmodel.Manifest{Name: name},
		}
	}
	manager.mu.Unlock()

	plugins := manager.List()

	// List doesn't guarantee order (map iteration), just completeness
	if len(plugins) != 3 {
		t.Fatalf("List() returned %d plugins, want 3", len(plugins))
	}

	// Verify all plugins are present
	foundNames := make(map[string]bool)
	for _, p := range plugins {
		foundNames[p.Manifest.Name] = true
	}
	for _, name := range names {
		if !foundNames[name] {
			t.Errorf("List() missing plugin %q", name)
		}
	}
}

// --- Lifecycle State Transition Tests ---

func TestRPCPluginManager_LifecycleStates(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	// Test lifecycle: Discover -> Load -> Init -> Shutdown

	// Discover
	plugins, err := manager.Discover(ctx)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(plugins) == 0 {
		t.Fatal("Discover() returned no plugins")
	}

	// Find a valid plugin (use the directory name as stored in manager)
	var pluginKey string
	for _, p := range plugins {
		if p.Manifest != nil && p.Manifest.Name != "" {
			pluginKey = p.Manifest.Name
			break
		}
	}
	if pluginKey == "" {
		t.Fatal("No valid plugin found after Discover")
	}

	// After discover, status should be Discovered or Loaded
	info, found := manager.Get(pluginKey)
	if !found {
		t.Fatalf("Plugin %q not found after Discover", pluginKey)
	}
	if info.Status != pluginmodel.StatusDiscovered && info.Status != pluginmodel.StatusLoaded {
		t.Errorf("Status after Discover = %q, want Discovered or Loaded", info.Status)
	}

	// Load (should be no-op if already loaded via Discover)
	if err := manager.Load(ctx, pluginKey); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	info, _ = manager.Get(pluginKey)
	if info.Status != pluginmodel.StatusLoaded {
		t.Errorf("Status after Load = %q, want Loaded", info.Status)
	}

	// Init
	if err := manager.Init(ctx, pluginKey, nil); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	info, _ = manager.Get(pluginKey)
	if info.Status != pluginmodel.StatusRunning && info.Status != pluginmodel.StatusInitialized {
		t.Errorf("Status after Init = %q, want Running or Initialized", info.Status)
	}

	// Shutdown
	if err := manager.Shutdown(ctx, pluginKey); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	info, found = manager.Get(pluginKey)
	if found && info.Status == pluginmodel.StatusRunning {
		t.Error("Plugin still running after Shutdown")
	}
}

// --- Table-Driven Tests ---

func TestRPCPluginManager_Operations_EdgeCases(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)

	tests := []struct {
		name       string
		operation  string
		pluginName string
		wantErr    bool
	}{
		{"load-empty-name", "load", "", true},
		{"init-empty-name", "init", "", true},
		{"shutdown-empty-name", "shutdown", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewRPCPluginManager(loader)
			manager.SetPluginsDir(fixturesPath)
			ctx := context.Background()

			var err error
			switch tt.operation {
			case "load":
				err = manager.Load(ctx, tt.pluginName)
			case "init":
				err = manager.Init(ctx, tt.pluginName, nil)
			case "shutdown":
				err = manager.Shutdown(ctx, tt.pluginName)
			}

			// Empty names should always fail with appropriate error
			if tt.wantErr && err == nil {
				t.Errorf("%s() error = nil, want error for empty name", tt.operation)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("%s() error = %v, want nil", tt.operation, err)
			}
		})
	}
}

func TestRPCPluginManager_SetPluginsDir(t *testing.T) {
	manager := NewRPCPluginManager(nil)

	// Initially empty
	manager.mu.RLock()
	initial := manager.pluginsDir
	manager.mu.RUnlock()
	if initial != "" {
		t.Errorf("Initial pluginsDir = %q, want empty", initial)
	}

	// Set directory
	manager.SetPluginsDir("/custom/plugins")
	manager.mu.RLock()
	after := manager.pluginsDir
	manager.mu.RUnlock()
	if after != "/custom/plugins" {
		t.Errorf("After SetPluginsDir() pluginsDir = %q, want %q", after, "/custom/plugins")
	}
}

func TestRPCPluginManager_Discover_NonExistentDirectory(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir("/nonexistent/plugins/directory")
	ctx := context.Background()

	_, err := manager.Discover(ctx)
	if err == nil {
		t.Fatal("Discover() error = nil, want error for non-existent directory")
	}

	// Should be wrapped in RPCManagerError
	var mgrErr *RPCManagerError
	if errors.As(err, &mgrErr) {
		if mgrErr.Op != "discover" {
			t.Errorf("RPCManagerError.Op = %q, want %q", mgrErr.Op, "discover")
		}
	}
}
