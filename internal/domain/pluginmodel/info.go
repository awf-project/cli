package pluginmodel

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
	StatusBuiltin     PluginStatus = "builtin"     // Built-in provider registered at startup
)

// PluginType distinguishes built-in providers from external plugins.
type PluginType string

const (
	PluginTypeBuiltin  PluginType = "builtin"
	PluginTypeExternal PluginType = "external"
)

type PluginInfo struct {
	Manifest      *Manifest
	Status        PluginStatus
	Type          PluginType
	Path          string
	Error         error
	LoadedAt      int64
	InitializedAt int64
	Operations    []string
	StepTypes     []string
}

func (p *PluginInfo) IsActive() bool {
	return p.Status == StatusRunning || p.Status == StatusBuiltin
}

func (p *PluginInfo) CanLoad() bool {
	return p.Status == StatusDiscovered || p.Status == StatusStopped || p.Status == StatusFailed
}
