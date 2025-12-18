package plugin

// PluginStatus represents the lifecycle state of a plugin.
type PluginStatus string

const (
	StatusDiscovered  PluginStatus = "discovered"  // Found in plugins/ directory
	StatusLoaded      PluginStatus = "loaded"      // Manifest parsed successfully
	StatusInitialized PluginStatus = "initialized" // Init() completed
	StatusRunning     PluginStatus = "running"     // Active and serving requests
	StatusStopped     PluginStatus = "stopped"     // Shutdown() completed
	StatusFailed      PluginStatus = "failed"      // Error occurred
	StatusDisabled    PluginStatus = "disabled"    // Manually disabled by user
)

// PluginInfo holds runtime state for a loaded plugin.
type PluginInfo struct {
	Manifest      *Manifest
	Status        PluginStatus
	Path          string // Filesystem path to plugin directory
	Error         error  // Last error if status is Failed
	LoadedAt      int64  // Unix timestamp when loaded
	InitializedAt int64  // Unix timestamp when initialized
}

// IsActive returns true if the plugin is running.
func (p *PluginInfo) IsActive() bool {
	return p.Status == StatusRunning
}

// CanLoad returns true if the plugin can be loaded.
func (p *PluginInfo) CanLoad() bool {
	return p.Status == StatusDiscovered || p.Status == StatusStopped || p.Status == StatusFailed
}
