package workflow

// Requirements lists what a workflow needs outside its own files, declared under `requires:`.
type Requirements struct {
	// Plugins maps an awf plugin name to a version constraint; an empty constraint accepts any version.
	Plugins map[string]string
	// AgentPlugins maps an agent provider to the plugins of that agent CLI, e.g. "claude" → ["egm@egm"].
	AgentPlugins map[string][]string
}

// IsEmpty reports whether the workflow declares no requirement.
func (r Requirements) IsEmpty() bool {
	return len(r.Plugins) == 0 && len(r.AgentPlugins) == 0
}
