package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/awf-project/awf/internal/domain/plugin"
)

// LoaderError represents an error during plugin loading operations.
type LoaderError struct {
	Path    string // plugin directory path
	Op      string // operation (discover, load, validate)
	Message string // error message
	Cause   error  // underlying error
}

// Error implements the error interface.
func (e *LoaderError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s: %s", e.Op, e.Path, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

// Unwrap returns the underlying error.
func (e *LoaderError) Unwrap() error {
	return e.Cause
}

// NewLoaderError creates a new LoaderError.
func NewLoaderError(op, path, message string) *LoaderError {
	return &LoaderError{
		Op:      op,
		Path:    path,
		Message: message,
	}
}

// WrapLoaderError wraps an existing error as a LoaderError.
func WrapLoaderError(op, path string, cause error) *LoaderError {
	return &LoaderError{
		Op:      op,
		Path:    path,
		Message: cause.Error(),
		Cause:   cause,
	}
}

// ManifestFileName is the expected filename for plugin manifests.
const ManifestFileName = "plugin.yaml"

// namePattern validates plugin names: alphanumeric, hyphens, underscores.
var namePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// versionPattern validates semver-like versions: X.Y.Z with optional prerelease.
var versionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?$`)

// awfVersionPattern validates AWF version constraints.
var awfVersionPattern = regexp.MustCompile(`^([<>=!]+)?\d+\.\d+\.\d+(\s+[<>=!]+\d+\.\d+\.\d+)*$`)

// FileSystemLoader implements PluginLoader for filesystem-based plugin discovery.
type FileSystemLoader struct {
	parser *ManifestParser
}

// NewFileSystemLoader creates a new FileSystemLoader with the given manifest parser.
func NewFileSystemLoader(parser *ManifestParser) *FileSystemLoader {
	return &FileSystemLoader{
		parser: parser,
	}
}

// DiscoverPlugins scans a directory for plugins and returns their info.
// Each subdirectory containing a plugin.yaml is considered a plugin.
func (l *FileSystemLoader) DiscoverPlugins(ctx context.Context, pluginsDir string) ([]*plugin.PluginInfo, error) {
	// Check context first
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("discover plugins: %w", err)
	}

	// Verify the directory exists and is a directory
	info, err := os.Stat(pluginsDir)
	if err != nil {
		return nil, WrapLoaderError("discover", pluginsDir, err)
	}
	if !info.IsDir() {
		return nil, NewLoaderError("discover", pluginsDir, "path is not a directory")
	}

	// Read directory entries
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, WrapLoaderError("discover", pluginsDir, err)
	}

	// Preallocate for expected number of plugins
	plugins := make([]*plugin.PluginInfo, 0, len(entries))

	for _, entry := range entries {
		// Check context on each iteration
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("discover plugins: %w", err)
		}

		// Skip non-directories
		if !entry.IsDir() {
			continue
		}

		subDir := filepath.Join(pluginsDir, entry.Name())
		manifestPath := filepath.Join(subDir, ManifestFileName)

		// Check if plugin.yaml exists
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			continue
		}

		// Try to load the plugin
		pluginInfo, err := l.LoadPlugin(ctx, subDir)
		if err != nil {
			// Skip invalid plugins silently during discovery
			continue
		}

		plugins = append(plugins, pluginInfo)
	}

	return plugins, nil
}

// LoadPlugin loads a single plugin from a directory path.
// Reads the plugin.yaml manifest and creates PluginInfo with status=StatusLoaded.
func (l *FileSystemLoader) LoadPlugin(ctx context.Context, pluginDir string) (*plugin.PluginInfo, error) {
	// Check context first
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("load plugin: %w", err)
	}

	// Verify the directory exists and is a directory
	info, err := os.Stat(pluginDir)
	if err != nil {
		return nil, WrapLoaderError("load", pluginDir, err)
	}
	if !info.IsDir() {
		return nil, NewLoaderError("load", pluginDir, "path is not a directory")
	}

	// Check for manifest file
	manifestPath := filepath.Join(pluginDir, ManifestFileName)
	if _, statErr := os.Stat(manifestPath); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, NewLoaderError("load", pluginDir, "plugin.yaml not found")
		}
		return nil, WrapLoaderError("load", pluginDir, statErr)
	}

	// Parse the manifest
	manifest, err := l.parser.ParseFile(manifestPath)
	if err != nil {
		return nil, WrapLoaderError("load", pluginDir, err)
	}

	// Create PluginInfo
	pluginInfo := &plugin.PluginInfo{
		Manifest: manifest,
		Status:   plugin.StatusLoaded,
		Path:     pluginDir,
		LoadedAt: time.Now().Unix(),
	}

	return pluginInfo, nil
}

// ValidatePlugin checks if a discovered plugin is valid and compatible.
// Validates manifest fields, capabilities, and AWF version constraint.
func (l *FileSystemLoader) ValidatePlugin(info *plugin.PluginInfo) error {
	if info == nil {
		return NewLoaderError("validate", "", "plugin info is nil")
	}

	if info.Manifest == nil {
		return NewLoaderError("validate", info.Path, "manifest is nil")
	}

	m := info.Manifest

	// Validate name
	if m.Name == "" {
		return NewLoaderError("validate", info.Path, "name is required")
	}
	if !namePattern.MatchString(m.Name) {
		return NewLoaderError("validate", info.Path, fmt.Sprintf("invalid name %q: must be alphanumeric with hyphens/underscores", m.Name))
	}

	// Validate version
	if m.Version == "" {
		return NewLoaderError("validate", info.Path, "version is required")
	}
	if !versionPattern.MatchString(m.Version) {
		return NewLoaderError("validate", info.Path, fmt.Sprintf("invalid version %q: must be semver format (X.Y.Z)", m.Version))
	}

	// Validate AWF version constraint
	if m.AWFVersion == "" {
		return NewLoaderError("validate", info.Path, "awf_version is required")
	}
	if !awfVersionPattern.MatchString(m.AWFVersion) {
		return NewLoaderError("validate", info.Path, fmt.Sprintf("invalid awf_version %q: must be a valid version constraint", m.AWFVersion))
	}

	// Validate capabilities
	if len(m.Capabilities) == 0 {
		return NewLoaderError("validate", info.Path, "at least one capability is required")
	}
	for _, capability := range m.Capabilities {
		if !isValidCapability(capability) {
			return NewLoaderError("validate", info.Path, fmt.Sprintf("invalid capability %q", capability))
		}
	}

	return nil
}

// isValidCapability checks if a capability string is valid.
func isValidCapability(capability string) bool {
	for _, valid := range plugin.ValidCapabilities {
		if capability == valid {
			return true
		}
	}
	return false
}
