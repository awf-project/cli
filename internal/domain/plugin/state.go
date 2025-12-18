package plugin

// PluginState represents the persisted state for a single plugin.
type PluginState struct {
	Enabled    bool           `json:"enabled"`
	Config     map[string]any `json:"config,omitempty"`
	DisabledAt int64          `json:"disabled_at,omitempty"` // Unix timestamp when disabled
}

// NewPluginState creates a new PluginState with default values.
func NewPluginState() *PluginState {
	return &PluginState{
		Enabled: true, // Plugins are enabled by default
		Config:  make(map[string]any),
	}
}
