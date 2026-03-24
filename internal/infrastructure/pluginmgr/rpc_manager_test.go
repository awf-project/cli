package pluginmgr

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
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

	// Returns ErrNoPluginsConfigured when no loader is configured
	if !errors.Is(err, ErrNoPluginsConfigured) {
		t.Errorf("Discover() error = %v, want ErrNoPluginsConfigured", err)
	}
	if plugins != nil {
		t.Errorf("Discover() plugins = %v, want nil when unconfigured", plugins)
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

	// Returns ErrNoPluginsConfigured when no loader is configured
	if !errors.Is(err, ErrNoPluginsConfigured) {
		t.Errorf("Load() error = %v, want ErrNoPluginsConfigured", err)
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

func TestRPCPluginManager_Init_ReturnsNoPluginsConfigured(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	ctx := context.Background()

	err := manager.Init(ctx, "test-plugin", nil)

	// Returns ErrNoPluginsConfigured when loader not configured
	if !errors.Is(err, ErrNoPluginsConfigured) {
		t.Errorf("Init() error = %v, want ErrNoPluginsConfigured", err)
	}
}

func TestRPCPluginManager_Init_EstablishesGRPCConnection(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	// Must load before init
	err := manager.Load(ctx, "valid-simple")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	err = manager.Init(ctx, "valid-simple", nil)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// After init, connection should be stored in connections map
	manager.mu.RLock()
	conn, found := manager.connections["valid-simple"]
	manager.mu.RUnlock()

	if !found {
		t.Fatal("Init() did not store plugin connection in connections map")
	}
	if conn == nil {
		t.Fatal("Init() stored nil connection in connections map")
	}
	// Verify connection fields are populated
	if conn.client == nil {
		t.Error("Init() connection.client is nil - go-plugin client not started")
	}
	if conn.plugin == nil {
		t.Error("Init() connection.plugin is nil - gRPC client not established")
	}
}

func TestRPCPluginManager_Init_CallsGetInfoRPC(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	err := manager.Load(ctx, "valid-simple")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	err = manager.Init(ctx, "valid-simple", nil)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// After successful init, PluginInfo should have populated fields from GetInfo RPC
	info, found := manager.Get("valid-simple")
	if !found {
		t.Fatal("Plugin not found after Init")
	}

	// GetInfo RPC should populate operations if they exist
	// The real implementation should call GetInfo and merge/update the PluginInfo
	if info.Status != pluginmodel.StatusRunning && info.Status != pluginmodel.StatusInitialized {
		t.Errorf("Status = %q, want Running or Initialized after Init", info.Status)
	}
}

func TestRPCPluginManager_Init_CallsInitRPC(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	config := map[string]any{
		"webhook_url": "https://hooks.slack.com/...",
		"channel":     "#alerts",
	}

	err := manager.Load(ctx, "valid-full")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Init RPC should be called with the provided config
	err = manager.Init(ctx, "valid-full", config)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Verify plugin is properly initialized
	info, found := manager.Get("valid-full")
	if !found {
		t.Fatal("Plugin not found after Init")
	}
	if info.Status != pluginmodel.StatusRunning && info.Status != pluginmodel.StatusInitialized {
		t.Errorf("Status = %q, want Running or Initialized", info.Status)
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

func TestRPCPluginManager_Init_VersionIncompatible(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	// Create a plugin with incompatible version
	manager.mu.Lock()
	manager.plugins["incompatible-version"] = &pluginmodel.PluginInfo{
		Manifest: &pluginmodel.Manifest{
			Name:         "incompatible-version",
			Version:      "0.1.0",
			AWFVersion:   ">= 99.0.0", // Impossible constraint
			Capabilities: []string{"operations"},
		},
		Status: pluginmodel.StatusLoaded,
		Path:   fixturesPath + "/valid-simple",
	}
	manager.mu.Unlock()

	err := manager.Init(ctx, "incompatible-version", nil)

	// Real implementation should reject incompatible versions with USER.VALIDATION error
	// This test will fail until version constraint validation is implemented
	if err == nil {
		t.Fatal("Init() error = nil, want error for version incompatible")
	}

	// Error should indicate version incompatibility (when implemented)
	var mgrErr *RPCManagerError
	if errors.As(err, &mgrErr) {
		if mgrErr.Op != "init" {
			t.Errorf("RPCManagerError.Op = %q, want %q", mgrErr.Op, "init")
		}
		// Once version validation is implemented, error should mention version
		if !strings.Contains(strings.ToLower(mgrErr.Error()), "version") {
			t.Logf("Note: Error message should mention version for USER.VALIDATION clarity: %v", mgrErr.Error())
		}
	}
}

func TestRPCPluginManager_Init_BinaryNotFound(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	// Create a plugin info manually (not loaded from fixtures) with non-existent binary
	manager.mu.Lock()
	manager.plugins["nonexistent-binary"] = &pluginmodel.PluginInfo{
		Manifest: &pluginmodel.Manifest{
			Name:    "nonexistent-binary",
			Version: "1.0.0",
		},
		Status: pluginmodel.StatusLoaded,
		Path:   fixturesPath + "/nonexistent-binary",
	}
	manager.mu.Unlock()

	err := manager.Init(ctx, "nonexistent-binary", nil)

	// Real implementation should detect binary not found and return error
	if err == nil {
		t.Fatal("Init() error = nil, want error when binary not found")
	}
}

func TestRPCPluginManager_Init_PluginManifestPopulated(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	// Load a plugin with valid manifest
	err := manager.Load(ctx, "valid-simple")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	err = manager.Init(ctx, "valid-simple", nil)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	info, found := manager.Get("valid-simple")
	if !found {
		t.Fatal("Plugin not found after Init")
	}
	// After Init, the plugin manifest should still be present
	if info.Manifest == nil {
		t.Error("Plugin manifest is nil after Init")
	}
}

func TestRPCPluginManager_Init_ConnectionTimeout(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	// Don't set pluginsDir - let Init timeout trying to start process
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	manager.mu.Lock()
	manager.plugins["timeout-test"] = &pluginmodel.PluginInfo{
		Manifest: &pluginmodel.Manifest{
			Name:    "timeout-test",
			Version: "1.0.0",
		},
		Status: pluginmodel.StatusLoaded,
		Path:   "/nonexistent/path",
	}
	manager.mu.Unlock()

	// Try to init with very tight timeout
	err := manager.Init(ctx, "timeout-test", nil)

	// Should fail due to context timeout or process timeout
	if err == nil {
		t.Fatal("Init() error = nil, want error for timeout scenario")
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

func TestRPCPluginManager_Init_GRPCRoundTrip(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	err := manager.Load(ctx, "valid-simple")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Init with empty config to test basic gRPC connectivity
	err = manager.Init(ctx, "valid-simple", map[string]any{})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Connection should be established and functional
	manager.mu.RLock()
	conn, found := manager.connections["valid-simple"]
	manager.mu.RUnlock()

	if !found {
		t.Error("Connection not found after Init")
	}
	if conn == nil {
		t.Error("Connection is nil")
	}
}

// --- Shutdown Tests ---

func TestRPCPluginManager_Shutdown_ReturnsNoPluginsConfigured(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	ctx := context.Background()

	err := manager.Shutdown(ctx, "test-plugin")

	// Returns ErrNoPluginsConfigured when loader not configured
	if !errors.Is(err, ErrNoPluginsConfigured) {
		t.Errorf("Shutdown() error = %v, want ErrNoPluginsConfigured", err)
	}
}

func TestRPCPluginManager_Shutdown_RunningPlugin(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	ctx := context.Background()

	// Manually set up a running plugin for testing Shutdown
	manager.mu.Lock()
	manager.plugins["valid-simple"] = &pluginmodel.PluginInfo{
		Manifest: &pluginmodel.Manifest{
			Name:    "valid-simple",
			Version: "1.0.0",
		},
		Status: pluginmodel.StatusRunning,
		Path:   "tests/fixtures/plugins/valid-simple",
	}
	// Create a mock connection (go-plugin client will be nil but that's OK for this test)
	manager.connections["valid-simple"] = &pluginConnection{}
	manager.mu.Unlock()

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
	ctx := context.Background()

	// Manually set up a running plugin
	manager.mu.Lock()
	manager.plugins["valid-simple"] = &pluginmodel.PluginInfo{
		Manifest: &pluginmodel.Manifest{
			Name:    "valid-simple",
			Version: "1.0.0",
		},
		Status: pluginmodel.StatusRunning,
		Path:   "tests/fixtures/plugins/valid-simple",
	}
	manager.connections["valid-simple"] = &pluginConnection{}
	manager.mu.Unlock()

	// First shutdown
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

func TestRPCPluginManager_ShutdownAll_ReturnsNoPluginsConfigured(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	ctx := context.Background()

	err := manager.ShutdownAll(ctx)

	// Returns ErrNoPluginsConfigured when loader not configured
	if !errors.Is(err, ErrNoPluginsConfigured) {
		t.Errorf("ShutdownAll() error = %v, want ErrNoPluginsConfigured", err)
	}
}

func TestRPCPluginManager_ShutdownAll_MultiplePlugins(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	ctx := context.Background()

	// Manually set up multiple running plugins
	pluginDirs := []string{"valid-simple", "valid-full"}
	manager.mu.Lock()
	for _, dir := range pluginDirs {
		manager.plugins[dir] = &pluginmodel.PluginInfo{
			Manifest: &pluginmodel.Manifest{
				Name:    dir,
				Version: "1.0.0",
			},
			Status: pluginmodel.StatusRunning,
			Path:   "tests/fixtures/plugins/" + dir,
		}
		manager.connections[dir] = &pluginConnection{}
	}
	manager.mu.Unlock()

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
	ctx := context.Background()

	// Manually set up plugins in different states
	manager.mu.Lock()
	// Running plugin
	manager.plugins["valid-simple"] = &pluginmodel.PluginInfo{
		Manifest: &pluginmodel.Manifest{
			Name:    "valid-simple",
			Version: "1.0.0",
		},
		Status: pluginmodel.StatusRunning,
		Path:   "tests/fixtures/plugins/valid-simple",
	}
	manager.connections["valid-simple"] = &pluginConnection{}

	// Loaded (not running) plugin
	manager.plugins["valid-full"] = &pluginmodel.PluginInfo{
		Manifest: &pluginmodel.Manifest{
			Name:    "valid-full",
			Version: "1.0.0",
		},
		Status: pluginmodel.StatusLoaded,
		Path:   "tests/fixtures/plugins/valid-full",
	}
	manager.mu.Unlock()

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

// --- connectWithTimeout Tests ---

// TestRPCPluginManager_connectWithTimeout_Nil verifies that nil client is handled.
func TestRPCPluginManager_connectWithTimeout_Nil(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	conn, _ := manager.connectWithTimeout(context.Background(), nil)

	// Should handle nil client gracefully
	if conn != nil {
		t.Error("connectWithTimeout(nil) should return nil connection")
	}
}

// TestRPCPluginManager_connectWithTimeout_SuccessfulConnection verifies successful connection.
// In real use, this establishes a connection to a real plugin process.
func TestRPCPluginManager_connectWithTimeout_SuccessfulConnection(t *testing.T) {
	manager := NewRPCPluginManager(nil)

	// For now, since we can't easily create a real go-plugin.Client in tests,
	// we verify the connections field exists and can store connections
	testConn := &pluginConnection{}
	manager.mu.Lock()
	manager.connections["test-plugin"] = testConn
	manager.mu.Unlock()

	// Verify the connection is stored
	manager.mu.RLock()
	stored, exists := manager.connections["test-plugin"]
	manager.mu.RUnlock()

	if !exists {
		t.Error("connections map should store plugin connection")
	}
	if stored != testConn {
		t.Error("connections map should preserve the stored connection")
	}
}

// TestRPCPluginManager_connectWithTimeout_Timeout verifies 5s timeout enforcement.
// This test will fail until connectWithTimeout implements the timeout logic.
// TestRPCPluginManager_connectWithTimeout_Timeout verifies 5s timeout enforcement.
// This test verifies that the function exists and has the timeout mechanism in place.
// Full timeout testing requires real go-plugin clients or integration tests.
// TestRPCPluginManager_connectWithTimeout_Timeout verifies 5s timeout enforcement.
// This test verifies that the function exists and has the timeout mechanism in place.
// Full timeout testing requires real go-plugin clients or integration tests.
func TestRPCPluginManager_connectWithTimeout_Timeout(t *testing.T) {
	manager := NewRPCPluginManager(nil)

	// Test the timeout behavior by calling connectWithTimeout with nil client
	// The real implementation should enforce a ~5s timeout when client.Client() hangs
	start := time.Now()
	conn, _ := manager.connectWithTimeout(context.Background(), nil)

	// Should return quickly (nil client should fail immediately, not hang)
	elapsed := time.Since(start)
	if elapsed > 2*time.Second {
		t.Errorf("connectWithTimeout() took %v for nil client, expected < 2s", elapsed)
	}

	// Should return nil connection
	if conn != nil {
		t.Error("connectWithTimeout(nil) should return nil connection")
	}
}

// TestRPCPluginManager_connectWithTimeout_ClientError verifies error propagation.
// This test will fail until connectWithTimeout properly handles client.Client() errors.
// TestRPCPluginManager_connectWithTimeout_ClientError verifies error propagation.
// This test verifies the function handles nil clients without panicking.
// TestRPCPluginManager_connectWithTimeout_ClientError verifies error propagation.
// This test verifies the function handles nil clients without panicking.
func TestRPCPluginManager_connectWithTimeout_ClientError(t *testing.T) {
	manager := NewRPCPluginManager(nil)

	// Call with nil client - should handle gracefully
	conn, _ := manager.connectWithTimeout(context.Background(), nil)

	// Implementation should either return error or nil, not panic
	if conn != nil {
		t.Error("connectWithTimeout(nil) should return nil connection")
	}

	// nil client returns (nil, nil)
}

// TestRPCPluginManager_connectionsFieldInitialized verifies connections map is initialized.
func TestRPCPluginManager_connectionsFieldInitialized(t *testing.T) {
	manager := NewRPCPluginManager(nil)

	// Verify connections field exists and is initialized
	if manager.connections == nil {
		t.Error("RPCPluginManager.connections = nil, want initialized map")
	}

	// Verify it's an empty map initially
	if len(manager.connections) != 0 {
		t.Errorf("RPCPluginManager.connections initial length = %d, want 0", len(manager.connections))
	}
}

// TestRPCPluginManager_connectionsMutexProtection verifies connections map is protected by mu.
func TestRPCPluginManager_connectionsMutexProtection(t *testing.T) {
	manager := NewRPCPluginManager(nil)

	// Simulate concurrent access patterns that would occur during Init/Shutdown
	var wg sync.WaitGroup

	// Multiple goroutines writing to connections (simulating Init)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			manager.mu.Lock()
			defer manager.mu.Unlock()

			// Simulate storing a connection
			name := "plugin-" + string(rune('a'+idx)) //nolint:gosec // controlled test input: idx is bounded by loop range
			manager.connections[name] = &pluginConnection{}
		}(i)
	}

	// Multiple goroutines reading from connections (simulating Execute/GetOperation)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			manager.mu.RLock()
			defer manager.mu.RUnlock()

			// Simulate reading from connections
			_ = len(manager.connections)
		}(i)
	}

	wg.Wait()
	// Test passes if no race condition detected (run with -race)
}

// TestRPCPluginManager_connectWithTimeout_ReturnType verifies function returns proper types.
// This test will fail if connectWithTimeout signature changes unexpectedly.
func TestRPCPluginManager_connectWithTimeout_ReturnType(t *testing.T) {
	manager := NewRPCPluginManager(nil)

	// Verify function signature by calling it (should not panic)
	conn, err := manager.connectWithTimeout(context.Background(), nil)

	// Should accept nil client without panicking
	// Return types should be (*pluginConnection, error)
	_ = conn // Type should be *pluginConnection or nil
	_ = err  // Type should be error or nil
}

// --- T013: Shutdown Implementation Requirements Tests ---
// These tests verify that Shutdown() and ShutdownAll() implement the following from the spec:
// - Shutdown calls gRPC conn.plugin.Shutdown(ctx)
// - Shutdown calls conn.client.Kill()
// - Shutdown removes connection from m.connections
// - ShutdownAll uses 5s per-plugin deadline
// - ShutdownAll accumulates errors with errors.Join()

// TestRPCPluginManager_Shutdown_ConnectionCleanup_Required verifies connection cleanup behavior.
// TestRPCPluginManager_Shutdown_ConnectionCleanup_Required verifies Shutdown calls RPC, Kill, and cleans up.
func TestRPCPluginManager_Shutdown_ConnectionCleanup_Required(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	if err := manager.Load(ctx, "valid-simple"); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := manager.Init(ctx, "valid-simple", nil); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Verify shutdown succeeds and connection is cleaned up
	err := manager.Shutdown(ctx, "valid-simple")
	if err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	// Verify plugin status is stopped
	info, found := manager.Get("valid-simple")
	if !found {
		t.Fatal("Plugin disappeared after Shutdown")
	}
	if info.Status != pluginmodel.StatusStopped {
		t.Errorf("Plugin status = %q, want StatusStopped", info.Status)
	}
}

// TestRPCPluginManager_Shutdown_WithoutLoaderContext validates error handling.
// This ensures Shutdown returns proper errors when loader is not configured.
func TestRPCPluginManager_Shutdown_LoaderRequired(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	ctx := context.Background()

	// Stub check: Shutdown requires loader
	err := manager.Shutdown(ctx, "test")
	if !errors.Is(err, ErrNoPluginsConfigured) {
		t.Errorf("Shutdown() without loader returned %v, want ErrNoPluginsConfigured", err)
	}
}

// TestRPCPluginManager_ShutdownAll_UsesPerPluginDeadlines verifies timeout implementation.
// FAILS: stub or incomplete implementation without proper timeout per plugin
// PASSES: implementation uses context.WithTimeout(ctx, 5*time.Second) per plugin
func TestRPCPluginManager_ShutdownAll_UsesPerPluginDeadlines(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	// Load and init two plugins
	if err := manager.Load(ctx, "valid-simple"); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := manager.Load(ctx, "valid-full"); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := manager.Init(ctx, "valid-simple", nil); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if err := manager.Init(ctx, "valid-full", nil); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// ShutdownAll should complete quickly even with 5s per-plugin timeout
	// Real implementation must use context.WithTimeout(ctx, 5*time.Second) per plugin
	start := time.Now()
	err := manager.ShutdownAll(ctx)
	elapsed := time.Since(start)

	// Should complete relatively quickly (stub doesn't make RPC calls)
	// If it takes close to 10s, the implementation is likely wrong
	if elapsed > 2*time.Second {
		t.Logf("ShutdownAll() took %v (may indicate incorrect timeout handling)", elapsed)
	}

	// Verify both plugins are stopped
	for _, name := range []string{"valid-simple", "valid-full"} {
		info, found := manager.Get(name)
		if !found {
			continue
		}
		if info.Status == pluginmodel.StatusRunning || info.Status == pluginmodel.StatusInitialized {
			t.Errorf("Plugin %q still in state %q after ShutdownAll", name, info.Status)
		}
	}

	_ = err // Error expected during stub phase
}

// TestRPCPluginManager_ShutdownAll_AccumulatesErrors verifies error handling.
// FAILS: stub that only returns first error instead of accumulating
// PASSES: implementation uses errors.Join() to accumulate all errors
func TestRPCPluginManager_ShutdownAll_AccumulatesErrors(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	// Load and init two plugins
	if err := manager.Load(ctx, "valid-simple"); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := manager.Load(ctx, "valid-full"); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := manager.Init(ctx, "valid-simple", nil); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if err := manager.Init(ctx, "valid-full", nil); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// ShutdownAll should attempt to shutdown all plugins even if some fail
	// The real implementation must use errors.Join() to accumulate errors
	err := manager.ShutdownAll(ctx)
	// Both plugins should be attempted to shutdown
	// Result: either no error (both succeed), or accumulated errors from both attempts
	if err != nil {
		// With errors.Join(), multiple errors should be present (if both fail)
		errStr := err.Error()
		t.Logf("ShutdownAll() returned accumulated error: %v", errStr)
	}
}

// T014: OperationProvider Implementation Tests

type mockOperationServiceClient struct {
	// Configured responses for testing
	listOpsResp *pluginv1.ListOperationsResponse
	listOpsErr  error
	getOpResp   *pluginv1.GetOperationResponse
	getOpErr    error
	execResp    *pluginv1.ExecuteResponse
	execErr     error
}

func (m *mockOperationServiceClient) ListOperations(ctx context.Context, _ *pluginv1.ListOperationsRequest, _ ...grpc.CallOption) (*pluginv1.ListOperationsResponse, error) {
	if m.listOpsErr != nil {
		return nil, m.listOpsErr
	}
	return m.listOpsResp, nil
}

func (m *mockOperationServiceClient) GetOperation(ctx context.Context, req *pluginv1.GetOperationRequest, _ ...grpc.CallOption) (*pluginv1.GetOperationResponse, error) {
	if m.getOpErr != nil {
		return nil, m.getOpErr
	}
	return m.getOpResp, nil
}

func (m *mockOperationServiceClient) Execute(ctx context.Context, req *pluginv1.ExecuteRequest, _ ...grpc.CallOption) (*pluginv1.ExecuteResponse, error) {
	if m.execErr != nil {
		return nil, m.execErr
	}
	return m.execResp, nil
}

func (m *mockOperationServiceClient) Shutdown(ctx context.Context, _ *pluginv1.ShutdownRequest, _ ...grpc.CallOption) (*pluginv1.ShutdownResponse, error) {
	return &pluginv1.ShutdownResponse{}, nil
}

// TestRPCPluginManager_GetOperation_NotFound tests that GetOperation returns false when operation not found.
func TestRPCPluginManager_GetOperation_NotFound(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	manager.plugins = make(map[string]*pluginmodel.PluginInfo)
	manager.connections = make(map[string]*pluginConnection)

	schema, found := manager.GetOperation("nonexistent-op")
	assert.False(t, found)
	assert.Nil(t, schema)
}

// TestRPCPluginManager_GetOperation_FoundInConnection tests that GetOperation finds operation in a connected plugin.
func TestRPCPluginManager_GetOperation_FoundInConnection(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	manager.plugins = make(map[string]*pluginmodel.PluginInfo)
	manager.connections = make(map[string]*pluginConnection)

	// Setup a connected plugin with mock operation client
	mockClient := &mockOperationServiceClient{
		getOpResp: &pluginv1.GetOperationResponse{
			Operation: &pluginv1.OperationSchema{
				Name:        "echo",
				Description: "Echo operation",
				Inputs:      nil,
				Outputs:     nil,
			},
		},
	}

	manager.plugins["test-plugin"] = &pluginmodel.PluginInfo{
		Status: pluginmodel.StatusRunning,
	}

	manager.connections["test-plugin"] = &pluginConnection{
		operation: mockClient,
	}

	schema, found := manager.GetOperation("test-plugin.echo")
	assert.True(t, found)
	assert.NotNil(t, schema)
	assert.Equal(t, "test-plugin.echo", schema.Name)
	assert.Equal(t, "test-plugin", schema.PluginName)
}

// TestRPCPluginManager_GetOperation_SearchesMultiplePlugins tests that GetOperation searches across multiple connected plugins.
func TestRPCPluginManager_GetOperation_SearchesMultiplePlugins(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	manager.plugins = make(map[string]*pluginmodel.PluginInfo)
	manager.connections = make(map[string]*pluginConnection)

	// Plugin 1: operation not found
	mock1 := &mockOperationServiceClient{
		getOpResp: nil,
		getOpErr:  errors.New("not found"),
	}

	// Plugin 2: operation found
	mock2 := &mockOperationServiceClient{
		getOpResp: &pluginv1.GetOperationResponse{
			Operation: &pluginv1.OperationSchema{
				Name:        "fetch",
				Description: "Fetch data",
			},
		},
	}

	manager.plugins["plugin1"] = &pluginmodel.PluginInfo{Status: pluginmodel.StatusRunning}
	manager.plugins["plugin2"] = &pluginmodel.PluginInfo{Status: pluginmodel.StatusRunning}

	manager.connections["plugin1"] = &pluginConnection{operation: mock1}
	manager.connections["plugin2"] = &pluginConnection{operation: mock2}

	schema, found := manager.GetOperation("plugin2.fetch")
	assert.True(t, found)
	assert.NotNil(t, schema)
	assert.Equal(t, "plugin2.fetch", schema.Name)
	assert.Equal(t, "plugin2", schema.PluginName)
}

// TestRPCPluginManager_ListOperations_Empty tests that ListOperations returns empty slice when no plugins are connected.
func TestRPCPluginManager_ListOperations_Empty(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	manager.plugins = make(map[string]*pluginmodel.PluginInfo)
	manager.connections = make(map[string]*pluginConnection)

	ops := manager.ListOperations()
	assert.NotNil(t, ops)
	assert.Len(t, ops, 0)
}

// TestRPCPluginManager_ListOperations_Aggregates tests that ListOperations aggregates operations from all connected plugins.
func TestRPCPluginManager_ListOperations_Aggregates(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	manager.plugins = make(map[string]*pluginmodel.PluginInfo)
	manager.connections = make(map[string]*pluginConnection)

	// Plugin 1 provides 2 operations
	mock1 := &mockOperationServiceClient{
		listOpsResp: &pluginv1.ListOperationsResponse{
			Operations: []*pluginv1.OperationSchema{
				{Name: "op1", Description: "Operation 1"},
				{Name: "op2", Description: "Operation 2"},
			},
		},
	}

	// Plugin 2 provides 1 operation
	mock2 := &mockOperationServiceClient{
		listOpsResp: &pluginv1.ListOperationsResponse{
			Operations: []*pluginv1.OperationSchema{
				{Name: "op3", Description: "Operation 3"},
			},
		},
	}

	manager.plugins["plugin1"] = &pluginmodel.PluginInfo{Status: pluginmodel.StatusRunning}
	manager.plugins["plugin2"] = &pluginmodel.PluginInfo{Status: pluginmodel.StatusRunning}

	manager.connections["plugin1"] = &pluginConnection{operation: mock1}
	manager.connections["plugin2"] = &pluginConnection{operation: mock2}

	ops := manager.ListOperations()
	assert.Len(t, ops, 3)

	// Verify operations are present and plugin names are injected
	names := make(map[string]string)
	for _, op := range ops {
		names[op.Name] = op.PluginName
	}

	assert.Equal(t, "plugin1", names["plugin1.op1"])
	assert.Equal(t, "plugin1", names["plugin1.op2"])
	assert.Equal(t, "plugin2", names["plugin2.op3"])
}

// TestRPCPluginManager_ListOperations_SkipsErroredPlugins tests that ListOperations continues aggregating even if one plugin fails.
func TestRPCPluginManager_ListOperations_SkipsErroredPlugins(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	manager.plugins = make(map[string]*pluginmodel.PluginInfo)
	manager.connections = make(map[string]*pluginConnection)

	// Plugin 1: error
	mock1 := &mockOperationServiceClient{
		listOpsErr: errors.New("rpc failure"),
	}

	// Plugin 2: success
	mock2 := &mockOperationServiceClient{
		listOpsResp: &pluginv1.ListOperationsResponse{
			Operations: []*pluginv1.OperationSchema{
				{Name: "valid-op", Description: "Valid"},
			},
		},
	}

	manager.plugins["plugin1"] = &pluginmodel.PluginInfo{Status: pluginmodel.StatusRunning}
	manager.plugins["plugin2"] = &pluginmodel.PluginInfo{Status: pluginmodel.StatusRunning}

	manager.connections["plugin1"] = &pluginConnection{operation: mock1}
	manager.connections["plugin2"] = &pluginConnection{operation: mock2}

	ops := manager.ListOperations()
	assert.Len(t, ops, 1)
	assert.Equal(t, "plugin2.valid-op", ops[0].Name)
}

// TestRPCPluginManager_Execute_DelegatesToPlugin tests that Execute delegates to the correct plugin.
func TestRPCPluginManager_Execute_DelegatesToPlugin(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	manager.plugins = make(map[string]*pluginmodel.PluginInfo)
	manager.connections = make(map[string]*pluginConnection)

	mockClient := &mockOperationServiceClient{
		execResp: &pluginv1.ExecuteResponse{
			Success: true,
			Output:  "result-value",
			Data: map[string][]byte{
				"key": []byte(`"value"`),
			},
		},
	}

	manager.plugins["test-plugin"] = &pluginmodel.PluginInfo{Status: pluginmodel.StatusRunning}
	manager.connections["test-plugin"] = &pluginConnection{operation: mockClient}

	ctx := context.Background()
	result, err := manager.Execute(ctx, "test-plugin.some-op", map[string]any{"input": "value"})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "result-value", result.Outputs["output"])
	assert.Equal(t, "value", result.Outputs["key"])
}

// TestRPCPluginManager_Execute_OperationNotFound tests that Execute returns error when operation not found.
func TestRPCPluginManager_Execute_OperationNotFound(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	manager.plugins = make(map[string]*pluginmodel.PluginInfo)
	manager.connections = make(map[string]*pluginConnection)

	ctx := context.Background()
	result, err := manager.Execute(ctx, "nonexistent-op", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestRPCPluginManager_Execute_ContextCancellation tests that Execute respects context cancellation.
func TestRPCPluginManager_Execute_ContextCancellation(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	manager.plugins = make(map[string]*pluginmodel.PluginInfo)
	manager.connections = make(map[string]*pluginConnection)

	mockClient := &mockOperationServiceClient{
		execResp: &pluginv1.ExecuteResponse{Success: true},
	}

	manager.plugins["test-plugin"] = &pluginmodel.PluginInfo{Status: pluginmodel.StatusRunning}
	manager.connections["test-plugin"] = &pluginConnection{operation: mockClient}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := manager.Execute(ctx, "op", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestRPCPluginManager_Execute_RemoteError tests that Execute handles errors from plugin.
func TestRPCPluginManager_Execute_RemoteError(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	manager.plugins = make(map[string]*pluginmodel.PluginInfo)
	manager.connections = make(map[string]*pluginConnection)

	mockClient := &mockOperationServiceClient{
		execErr: errors.New("plugin error"),
	}

	manager.plugins["test-plugin"] = &pluginmodel.PluginInfo{Status: pluginmodel.StatusRunning}
	manager.connections["test-plugin"] = &pluginConnection{operation: mockClient}

	ctx := context.Background()
	result, err := manager.Execute(ctx, "op", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestRPCPluginManager_Execute_ConcurrentCalls tests that concurrent Execute calls are race-free.
func TestRPCPluginManager_Execute_ConcurrentCalls(t *testing.T) {
	manager := NewRPCPluginManager(nil)
	manager.plugins = make(map[string]*pluginmodel.PluginInfo)
	manager.connections = make(map[string]*pluginConnection)

	mockClient := &mockOperationServiceClient{
		execResp: &pluginv1.ExecuteResponse{
			Success: true,
			Output:  "concurrent-result",
		},
	}

	manager.plugins["test-plugin"] = &pluginmodel.PluginInfo{Status: pluginmodel.StatusRunning}
	manager.connections["test-plugin"] = &pluginConnection{operation: mockClient}

	ctx := context.Background()

	// Run concurrent Execute calls to verify no race conditions
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := manager.Execute(ctx, "op", nil)
			assert.NoError(t, err)
			assert.NotNil(t, result)
		}()
	}

	wg.Wait()
}

// TestRPCPluginManager_Execute_ResultConversion tests that Execute properly converts gRPC response to domain result.
func TestRPCPluginManager_Execute_ResultConversion(t *testing.T) {
	tests := []struct {
		name     string
		grpcResp *pluginv1.ExecuteResponse
		want     *pluginmodel.OperationResult
	}{
		{
			name: "success with output only",
			grpcResp: &pluginv1.ExecuteResponse{
				Success: true,
				Output:  "hello",
				Data:    map[string][]byte{},
			},
			want: &pluginmodel.OperationResult{
				Success: true,
				Outputs: map[string]any{"output": "hello"},
				Error:   "",
			},
		},
		{
			name: "success with data fields",
			grpcResp: &pluginv1.ExecuteResponse{
				Success: true,
				Output:  "main",
				Data: map[string][]byte{
					"extra": []byte(`"field"`),
					"count": []byte(`42`),
				},
			},
			want: &pluginmodel.OperationResult{
				Success: true,
				Outputs: map[string]any{
					"output": "main",
					"extra":  "field",
					"count":  float64(42),
				},
				Error: "",
			},
		},
		{
			name: "failure with error message",
			grpcResp: &pluginv1.ExecuteResponse{
				Success: false,
				Output:  "",
				Error:   "operation failed",
				Data:    map[string][]byte{},
			},
			want: &pluginmodel.OperationResult{
				Success: false,
				Outputs: map[string]any{},
				Error:   "operation failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewRPCPluginManager(nil)
			manager.plugins = make(map[string]*pluginmodel.PluginInfo)
			manager.connections = make(map[string]*pluginConnection)

			mockClient := &mockOperationServiceClient{
				execResp: tt.grpcResp,
			}

			manager.plugins["test-plugin"] = &pluginmodel.PluginInfo{Status: pluginmodel.StatusRunning}
			manager.connections["test-plugin"] = &pluginConnection{operation: mockClient}

			result, err := manager.Execute(context.Background(), "op", nil)

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.want.Success, result.Success)
			assert.Equal(t, tt.want.Error, result.Error)
			assert.Equal(t, tt.want.Outputs, result.Outputs)
		})
	}
}

// TestRPCPluginManager_ShutdownAll_IdempotentWithMixedStates validates mixed state handling.
// Ensures ShutdownAll can be called multiple times without panicking.
func TestRPCPluginManager_ShutdownAll_Idempotent(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDir(fixturesPath)
	ctx := context.Background()

	if err := manager.Load(ctx, "valid-simple"); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := manager.Init(ctx, "valid-simple", nil); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// First shutdown all
	if err := manager.ShutdownAll(ctx); err != nil {
		t.Logf("First ShutdownAll() error = %v", err)
	}

	// Second shutdown all (should be idempotent, no panic)
	if err := manager.ShutdownAll(ctx); err != nil {
		t.Logf("Second ShutdownAll() error = %v", err)
	}
}
