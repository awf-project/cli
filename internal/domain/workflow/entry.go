package workflow

// WorkflowEntry represents a workflow from any source (local, global, pack)
// for listing purposes. It carries enough metadata for display without loading
// the full workflow definition.
type WorkflowEntry struct {
	Name        string // "my-workflow" or "pack-name/workflow-name"
	Source      string // "local", "global", "env", "pack"
	Version     string
	Description string
}
