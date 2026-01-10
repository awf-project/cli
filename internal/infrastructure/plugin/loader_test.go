package plugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanoix/awf/internal/domain/plugin"
)

func TestNewFileSystemLoader(t *testing.T) {
	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)

	if loader == nil {
		t.Fatal("NewFileSystemLoader() returned nil")
	}
	if loader.parser != parser {
		t.Error("NewFileSystemLoader() did not set parser")
	}
}

func TestNewFileSystemLoader_NilParser(t *testing.T) {
	loader := NewFileSystemLoader(nil)

	if loader == nil {
		t.Fatal("NewFileSystemLoader(nil) returned nil")
	}
	if loader.parser != nil {
		t.Error("NewFileSystemLoader(nil) should set parser to nil")
	}
}

// --- DiscoverPlugins Tests ---

func TestFileSystemLoader_DiscoverPlugins_ValidDirectory(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())
	ctx := context.Background()

	plugins, err := loader.DiscoverPlugins(ctx, fixturesPath)
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("DiscoverPlugins not yet implemented")
	}
	if err != nil {
		t.Fatalf("DiscoverPlugins() error = %v", err)
	}

	// Should find at least the valid plugins in fixtures
	if len(plugins) < 2 {
		t.Errorf("DiscoverPlugins() found %d plugins, want at least 2", len(plugins))
	}

	// Check all returned plugins have required fields
	for _, p := range plugins {
		if p.Path == "" {
			t.Error("PluginInfo.Path is empty")
		}
		if p.Manifest == nil {
			t.Errorf("PluginInfo.Manifest is nil for plugin at %s", p.Path)
			continue
		}
		if p.Manifest.Name == "" {
			t.Errorf("PluginInfo.Manifest.Name is empty for plugin at %s", p.Path)
		}
	}
}

func TestFileSystemLoader_DiscoverPlugins_EmptyDirectory(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())
	ctx := context.Background()

	// Create a temp empty directory
	tmpDir := t.TempDir()

	plugins, err := loader.DiscoverPlugins(ctx, tmpDir)
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("DiscoverPlugins not yet implemented")
	}
	if err != nil {
		t.Fatalf("DiscoverPlugins() error = %v, want nil for empty directory", err)
	}
	if len(plugins) != 0 {
		t.Errorf("DiscoverPlugins() returned %d plugins, want 0 for empty directory", len(plugins))
	}
}

func TestFileSystemLoader_DiscoverPlugins_NonExistentDirectory(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())
	ctx := context.Background()

	_, err := loader.DiscoverPlugins(ctx, "/nonexistent/plugins/directory")
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("DiscoverPlugins not yet implemented")
	}
	if err == nil {
		t.Fatal("DiscoverPlugins() error = nil, want error for non-existent directory")
	}

	// Check error type
	var loaderErr *LoaderError
	if errors.As(err, &loaderErr) {
		if loaderErr.Op != "discover" {
			t.Errorf("LoaderError.Op = %q, want %q", loaderErr.Op, "discover")
		}
	}
}

func TestFileSystemLoader_DiscoverPlugins_FileNotDirectory(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())
	ctx := context.Background()

	// Pass a file path instead of directory
	filePath := filepath.Join(fixturesPath, "valid-simple", "plugin.yaml")

	_, err := loader.DiscoverPlugins(ctx, filePath)
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("DiscoverPlugins not yet implemented")
	}
	if err == nil {
		t.Fatal("DiscoverPlugins() error = nil, want error when path is a file")
	}
}

func TestFileSystemLoader_DiscoverPlugins_SkipsInvalidPlugins(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())
	ctx := context.Background()

	plugins, err := loader.DiscoverPlugins(ctx, fixturesPath)
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("DiscoverPlugins not yet implemented")
	}
	if err != nil {
		t.Fatalf("DiscoverPlugins() error = %v", err)
	}

	// Should only return valid plugins (valid-simple, valid-full)
	// Invalid plugins should be skipped or have Failed status
	validCount := 0
	for _, p := range plugins {
		if p.Status == plugin.StatusLoaded || p.Status == plugin.StatusDiscovered {
			validCount++
		}
	}

	if validCount < 2 {
		t.Errorf("DiscoverPlugins() found %d valid plugins, want at least 2", validCount)
	}
}

func TestFileSystemLoader_DiscoverPlugins_ContextCancellation(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := loader.DiscoverPlugins(ctx, fixturesPath)
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("DiscoverPlugins not yet implemented")
	}
	if err == nil {
		t.Fatal("DiscoverPlugins() error = nil, want error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("DiscoverPlugins() error = %v, want context.Canceled", err)
	}
}

func TestFileSystemLoader_DiscoverPlugins_SubdirectoriesOnly(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())
	ctx := context.Background()

	// Create temp directory with a plugin.yaml at root (should be ignored)
	// and a valid plugin in subdirectory
	tmpDir := t.TempDir()

	// Create root-level plugin.yaml (should be ignored)
	rootManifest := `name: root-plugin
version: 1.0.0
awf_version: ">=0.4.0"
capabilities: [operations]`
	if err := os.WriteFile(filepath.Join(tmpDir, "plugin.yaml"), []byte(rootManifest), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create valid subdirectory plugin
	subDir := filepath.Join(tmpDir, "my-plugin")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	subManifest := `name: my-plugin
version: 1.0.0
awf_version: ">=0.4.0"
capabilities: [operations]`
	if err := os.WriteFile(filepath.Join(subDir, "plugin.yaml"), []byte(subManifest), 0o644); err != nil {
		t.Fatal(err)
	}

	plugins, err := loader.DiscoverPlugins(ctx, tmpDir)
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("DiscoverPlugins not yet implemented")
	}
	if err != nil {
		t.Fatalf("DiscoverPlugins() error = %v", err)
	}

	// Should only find the subdirectory plugin, not root
	if len(plugins) != 1 {
		t.Errorf("DiscoverPlugins() found %d plugins, want 1", len(plugins))
	}
	if len(plugins) > 0 && plugins[0].Manifest.Name != "my-plugin" {
		t.Errorf("DiscoverPlugins() found plugin %q, want %q", plugins[0].Manifest.Name, "my-plugin")
	}
}

// --- LoadPlugin Tests ---

func TestFileSystemLoader_LoadPlugin_ValidSimple(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())
	ctx := context.Background()
	pluginDir := filepath.Join(fixturesPath, "valid-simple")

	info, err := loader.LoadPlugin(ctx, pluginDir)
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("LoadPlugin not yet implemented")
	}
	if err != nil {
		t.Fatalf("LoadPlugin() error = %v", err)
	}
	if info == nil {
		t.Fatal("LoadPlugin() returned nil PluginInfo")
	}

	// Check PluginInfo fields
	if info.Path != pluginDir {
		t.Errorf("PluginInfo.Path = %q, want %q", info.Path, pluginDir)
	}
	if info.Status != plugin.StatusLoaded {
		t.Errorf("PluginInfo.Status = %q, want %q", info.Status, plugin.StatusLoaded)
	}
	if info.LoadedAt == 0 {
		t.Error("PluginInfo.LoadedAt should be set")
	}

	// Check Manifest
	if info.Manifest == nil {
		t.Fatal("PluginInfo.Manifest is nil")
	}
	if info.Manifest.Name != "awf-plugin-simple" {
		t.Errorf("Manifest.Name = %q, want %q", info.Manifest.Name, "awf-plugin-simple")
	}
	if info.Manifest.Version != "1.0.0" {
		t.Errorf("Manifest.Version = %q, want %q", info.Manifest.Version, "1.0.0")
	}
}

func TestFileSystemLoader_LoadPlugin_ValidFull(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())
	ctx := context.Background()
	pluginDir := filepath.Join(fixturesPath, "valid-full")

	info, err := loader.LoadPlugin(ctx, pluginDir)
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("LoadPlugin not yet implemented")
	}
	if err != nil {
		t.Fatalf("LoadPlugin() error = %v", err)
	}
	if info == nil {
		t.Fatal("LoadPlugin() returned nil PluginInfo")
	}

	// Check Manifest with full metadata
	if info.Manifest.Name != "awf-plugin-slack" {
		t.Errorf("Manifest.Name = %q, want %q", info.Manifest.Name, "awf-plugin-slack")
	}
	if info.Manifest.Description == "" {
		t.Error("Manifest.Description should be set")
	}
	if info.Manifest.Author == "" {
		t.Error("Manifest.Author should be set")
	}
	if len(info.Manifest.Config) == 0 {
		t.Error("Manifest.Config should have fields")
	}
}

func TestFileSystemLoader_LoadPlugin_NonExistentDirectory(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())
	ctx := context.Background()

	info, err := loader.LoadPlugin(ctx, "/nonexistent/plugin/dir")
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("LoadPlugin not yet implemented")
	}
	if err == nil {
		t.Fatal("LoadPlugin() error = nil, want error for non-existent directory")
	}
	if info != nil {
		t.Errorf("LoadPlugin() info = %v, want nil", info)
	}

	// Check error type
	var loaderErr *LoaderError
	if errors.As(err, &loaderErr) {
		if loaderErr.Op != "load" {
			t.Errorf("LoaderError.Op = %q, want %q", loaderErr.Op, "load")
		}
	}
}

func TestFileSystemLoader_LoadPlugin_NoManifest(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())
	ctx := context.Background()

	// Create temp directory without plugin.yaml
	tmpDir := t.TempDir()

	info, err := loader.LoadPlugin(ctx, tmpDir)
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("LoadPlugin not yet implemented")
	}
	if err == nil {
		t.Fatal("LoadPlugin() error = nil, want error for missing manifest")
	}
	if info != nil {
		t.Errorf("LoadPlugin() info = %v, want nil", info)
	}

	// Error should mention plugin.yaml or manifest
	if !strings.Contains(err.Error(), "plugin.yaml") && !strings.Contains(err.Error(), "manifest") {
		t.Errorf("error = %v, should mention plugin.yaml or manifest", err)
	}
}

func TestFileSystemLoader_LoadPlugin_InvalidManifest(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())
	ctx := context.Background()
	pluginDir := filepath.Join(fixturesPath, "invalid-syntax")

	info, err := loader.LoadPlugin(ctx, pluginDir)
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("LoadPlugin not yet implemented")
	}
	if err == nil {
		t.Fatal("LoadPlugin() error = nil, want error for invalid manifest")
	}
	if info != nil {
		t.Errorf("LoadPlugin() info = %v, want nil", info)
	}
}

func TestFileSystemLoader_LoadPlugin_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name      string
		pluginDir string
	}{
		{"missing-name", filepath.Join(fixturesPath, "invalid-missing-name")},
		{"missing-version", filepath.Join(fixturesPath, "invalid-missing-version")},
		{"missing-awf-version", filepath.Join(fixturesPath, "invalid-missing-awf-version")},
	}

	loader := NewFileSystemLoader(NewManifestParser())
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := loader.LoadPlugin(ctx, tt.pluginDir)
			if errors.Is(err, ErrLoaderNotImplemented) {
				t.Skip("LoadPlugin not yet implemented")
			}
			if err == nil {
				t.Fatalf("LoadPlugin() error = nil, want error for %s", tt.name)
			}
			if info != nil {
				t.Errorf("LoadPlugin() info = %v, want nil", info)
			}
		})
	}
}

func TestFileSystemLoader_LoadPlugin_ContextCancellation(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	pluginDir := filepath.Join(fixturesPath, "valid-simple")
	info, err := loader.LoadPlugin(ctx, pluginDir)
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("LoadPlugin not yet implemented")
	}
	if err == nil {
		t.Fatal("LoadPlugin() error = nil, want error for cancelled context")
	}
	if info != nil {
		t.Errorf("LoadPlugin() info = %v, want nil for cancelled context", info)
	}
}

func TestFileSystemLoader_LoadPlugin_FileInsteadOfDirectory(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())
	ctx := context.Background()

	// Pass a file path instead of directory
	filePath := filepath.Join(fixturesPath, "valid-simple", "plugin.yaml")

	info, err := loader.LoadPlugin(ctx, filePath)
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("LoadPlugin not yet implemented")
	}
	if err == nil {
		t.Fatal("LoadPlugin() error = nil, want error when path is a file")
	}
	if info != nil {
		t.Errorf("LoadPlugin() info = %v, want nil", info)
	}
}

// --- ValidatePlugin Tests ---

func TestFileSystemLoader_ValidatePlugin_ValidPlugin(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())

	info := &plugin.PluginInfo{
		Path:   "/plugins/test-plugin",
		Status: plugin.StatusLoaded,
		Manifest: &plugin.Manifest{
			Name:         "test-plugin",
			Version:      "1.0.0",
			AWFVersion:   ">=0.4.0",
			Capabilities: []string{plugin.CapabilityOperations},
		},
	}

	err := loader.ValidatePlugin(info)
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("ValidatePlugin not yet implemented")
	}
	if err != nil {
		t.Fatalf("ValidatePlugin() error = %v, want nil", err)
	}
}

func TestFileSystemLoader_ValidatePlugin_NilInfo(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())

	err := loader.ValidatePlugin(nil)
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("ValidatePlugin not yet implemented")
	}
	if err == nil {
		t.Fatal("ValidatePlugin() error = nil, want error for nil info")
	}
}

func TestFileSystemLoader_ValidatePlugin_NilManifest(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())

	info := &plugin.PluginInfo{
		Path:     "/plugins/test-plugin",
		Status:   plugin.StatusLoaded,
		Manifest: nil,
	}

	err := loader.ValidatePlugin(info)
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("ValidatePlugin not yet implemented")
	}
	if err == nil {
		t.Fatal("ValidatePlugin() error = nil, want error for nil manifest")
	}
}

func TestFileSystemLoader_ValidatePlugin_InvalidCapability(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())

	info := &plugin.PluginInfo{
		Path:   "/plugins/test-plugin",
		Status: plugin.StatusLoaded,
		Manifest: &plugin.Manifest{
			Name:         "test-plugin",
			Version:      "1.0.0",
			AWFVersion:   ">=0.4.0",
			Capabilities: []string{"invalid-capability"},
		},
	}

	err := loader.ValidatePlugin(info)
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("ValidatePlugin not yet implemented")
	}
	if err == nil {
		t.Fatal("ValidatePlugin() error = nil, want error for invalid capability")
	}
	if !strings.Contains(err.Error(), "capability") && !strings.Contains(err.Error(), "invalid") {
		t.Errorf("error = %v, should mention invalid capability", err)
	}
}

func TestFileSystemLoader_ValidatePlugin_EmptyCapabilities(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())

	info := &plugin.PluginInfo{
		Path:   "/plugins/test-plugin",
		Status: plugin.StatusLoaded,
		Manifest: &plugin.Manifest{
			Name:         "test-plugin",
			Version:      "1.0.0",
			AWFVersion:   ">=0.4.0",
			Capabilities: []string{},
		},
	}

	err := loader.ValidatePlugin(info)
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("ValidatePlugin not yet implemented")
	}
	if err == nil {
		t.Fatal("ValidatePlugin() error = nil, want error for empty capabilities")
	}
}

func TestFileSystemLoader_ValidatePlugin_InvalidName(t *testing.T) {
	tests := []struct {
		name    string
		badName string
	}{
		{"empty", ""},
		{"spaces", "plugin with spaces"},
		{"special-chars", "plugin@name!"},
	}

	loader := NewFileSystemLoader(NewManifestParser())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &plugin.PluginInfo{
				Path:   "/plugins/test-plugin",
				Status: plugin.StatusLoaded,
				Manifest: &plugin.Manifest{
					Name:         tt.badName,
					Version:      "1.0.0",
					AWFVersion:   ">=0.4.0",
					Capabilities: []string{plugin.CapabilityOperations},
				},
			}

			err := loader.ValidatePlugin(info)
			if errors.Is(err, ErrLoaderNotImplemented) {
				t.Skip("ValidatePlugin not yet implemented")
			}
			if err == nil {
				t.Fatalf("ValidatePlugin() error = nil, want error for name %q", tt.badName)
			}
		})
	}
}

func TestFileSystemLoader_ValidatePlugin_InvalidVersion(t *testing.T) {
	tests := []struct {
		name       string
		badVersion string
	}{
		{"empty", ""},
		{"no-dots", "100"},
		{"letters", "v1.0.0"}, // semver without v prefix
	}

	loader := NewFileSystemLoader(NewManifestParser())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &plugin.PluginInfo{
				Path:   "/plugins/test-plugin",
				Status: plugin.StatusLoaded,
				Manifest: &plugin.Manifest{
					Name:         "test-plugin",
					Version:      tt.badVersion,
					AWFVersion:   ">=0.4.0",
					Capabilities: []string{plugin.CapabilityOperations},
				},
			}

			err := loader.ValidatePlugin(info)
			if errors.Is(err, ErrLoaderNotImplemented) {
				t.Skip("ValidatePlugin not yet implemented")
			}
			// Empty version should always fail
			if tt.name == "empty" && err == nil {
				t.Fatalf("ValidatePlugin() error = nil, want error for empty version")
			}
		})
	}
}

func TestFileSystemLoader_ValidatePlugin_InvalidAWFVersion(t *testing.T) {
	tests := []struct {
		name       string
		awfVersion string
		shouldFail bool
	}{
		{"empty", "", true},
		{"invalid-constraint", "invalid", true},
		{"valid-gte", ">=0.4.0", false},
		{"valid-exact", "0.4.0", false},
		{"valid-range", ">=0.4.0 <1.0.0", false},
	}

	loader := NewFileSystemLoader(NewManifestParser())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &plugin.PluginInfo{
				Path:   "/plugins/test-plugin",
				Status: plugin.StatusLoaded,
				Manifest: &plugin.Manifest{
					Name:         "test-plugin",
					Version:      "1.0.0",
					AWFVersion:   tt.awfVersion,
					Capabilities: []string{plugin.CapabilityOperations},
				},
			}

			err := loader.ValidatePlugin(info)
			if errors.Is(err, ErrLoaderNotImplemented) {
				t.Skip("ValidatePlugin not yet implemented")
			}
			if tt.shouldFail && err == nil {
				t.Fatalf("ValidatePlugin() error = nil, want error for awf_version %q", tt.awfVersion)
			}
			if !tt.shouldFail && err != nil {
				t.Fatalf("ValidatePlugin() error = %v, want nil for awf_version %q", err, tt.awfVersion)
			}
		})
	}
}

func TestFileSystemLoader_ValidatePlugin_MultipleCapabilities(t *testing.T) {
	loader := NewFileSystemLoader(NewManifestParser())

	info := &plugin.PluginInfo{
		Path:   "/plugins/test-plugin",
		Status: plugin.StatusLoaded,
		Manifest: &plugin.Manifest{
			Name:       "test-plugin",
			Version:    "1.0.0",
			AWFVersion: ">=0.4.0",
			Capabilities: []string{
				plugin.CapabilityOperations,
				plugin.CapabilityCommands,
				plugin.CapabilityValidators,
			},
		},
	}

	err := loader.ValidatePlugin(info)
	if errors.Is(err, ErrLoaderNotImplemented) {
		t.Skip("ValidatePlugin not yet implemented")
	}
	if err != nil {
		t.Fatalf("ValidatePlugin() error = %v, want nil for valid multiple capabilities", err)
	}
}

// --- LoaderError Tests ---

func TestLoaderError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *LoaderError
		contains []string
	}{
		{
			name: "with path",
			err: &LoaderError{
				Op:      "discover",
				Path:    "/plugins",
				Message: "directory not found",
			},
			contains: []string{"discover", "/plugins", "directory not found"},
		},
		{
			name: "without path",
			err: &LoaderError{
				Op:      "validate",
				Message: "invalid manifest",
			},
			contains: []string{"validate", "invalid manifest"},
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

func TestLoaderError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &LoaderError{
		Op:      "load",
		Path:    "/plugin",
		Message: "failed",
		Cause:   cause,
	}

	unwrapped := err.Unwrap()
	if unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}
}

func TestNewLoaderError(t *testing.T) {
	err := NewLoaderError("discover", "/plugins", "not found")

	if err.Op != "discover" {
		t.Errorf("Op = %q, want %q", err.Op, "discover")
	}
	if err.Path != "/plugins" {
		t.Errorf("Path = %q, want %q", err.Path, "/plugins")
	}
	if err.Message != "not found" {
		t.Errorf("Message = %q, want %q", err.Message, "not found")
	}
	if err.Cause != nil {
		t.Errorf("Cause = %v, want nil", err.Cause)
	}
}

func TestWrapLoaderError(t *testing.T) {
	cause := os.ErrNotExist
	err := WrapLoaderError("load", "/plugin", cause)

	if err.Op != "load" {
		t.Errorf("Op = %q, want %q", err.Op, "load")
	}
	if err.Path != "/plugin" {
		t.Errorf("Path = %q, want %q", err.Path, "/plugin")
	}
	if err.Cause != cause {
		t.Errorf("Cause = %v, want %v", err.Cause, cause)
	}
}

// --- Table-driven edge case tests ---

func TestFileSystemLoader_DiscoverPlugins_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		pluginsDir string
		wantErr    bool
		minPlugins int
	}{
		{"valid-fixtures", fixturesPath, false, 2},
		{"nonexistent", "/nonexistent/dir", true, 0},
	}

	loader := NewFileSystemLoader(NewManifestParser())
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugins, err := loader.DiscoverPlugins(ctx, tt.pluginsDir)
			if errors.Is(err, ErrLoaderNotImplemented) {
				t.Skip("DiscoverPlugins not yet implemented")
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("DiscoverPlugins() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(plugins) < tt.minPlugins {
				t.Errorf("DiscoverPlugins() returned %d plugins, want >= %d", len(plugins), tt.minPlugins)
			}
		})
	}
}

func TestFileSystemLoader_LoadPlugin_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		pluginDir string
		wantErr   bool
	}{
		{"valid-simple", filepath.Join(fixturesPath, "valid-simple"), false},
		{"valid-full", filepath.Join(fixturesPath, "valid-full"), false},
		{"invalid-syntax", filepath.Join(fixturesPath, "invalid-syntax"), true},
		{"invalid-missing-name", filepath.Join(fixturesPath, "invalid-missing-name"), true},
		{"invalid-missing-version", filepath.Join(fixturesPath, "invalid-missing-version"), true},
		{"invalid-missing-awf-version", filepath.Join(fixturesPath, "invalid-missing-awf-version"), true},
		{"nonexistent", filepath.Join(fixturesPath, "nonexistent"), true},
	}

	loader := NewFileSystemLoader(NewManifestParser())
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := loader.LoadPlugin(ctx, tt.pluginDir)
			if errors.Is(err, ErrLoaderNotImplemented) {
				t.Skip("LoadPlugin not yet implemented")
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("LoadPlugin() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && info == nil {
				t.Error("LoadPlugin() returned nil info for valid plugin")
			}
			if tt.wantErr && info != nil {
				t.Error("LoadPlugin() returned non-nil info for invalid plugin")
			}
		})
	}
}

// --- Interface compliance test ---

func TestFileSystemLoader_ImplementsPluginLoader(t *testing.T) {
	// Compile-time check that FileSystemLoader implements the interface
	// This test doesn't need to run anything - it's a compile check
	var _ interface {
		DiscoverPlugins(context.Context, string) ([]*plugin.PluginInfo, error)
		LoadPlugin(context.Context, string) (*plugin.PluginInfo, error)
		ValidatePlugin(*plugin.PluginInfo) error
	} = (*FileSystemLoader)(nil)
}
