package pluginmodel

type PluginState struct {
	Enabled    bool           `json:"enabled"`
	Config     map[string]any `json:"config,omitempty"`
	DisabledAt int64          `json:"disabled_at,omitempty"`
}

func NewPluginState() *PluginState {
	return &PluginState{
		Enabled: true,
		Config:  make(map[string]any),
	}
}
