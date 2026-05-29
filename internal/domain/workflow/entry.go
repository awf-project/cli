package workflow

// WorkflowEntry represents a workflow from any source (local, global, env, pack)
// for listing purposes. It carries enough metadata for display without loading
// the full workflow definition.
//
// Fields:
//   - Name        — display identifier; "workflow-name" for local/global/env,
//     "packName/workflowName" for pack-sourced entries.
//   - Source      — provenance string matching the discovery origin:
//     "local" (./.awf/workflows/), "global" ($XDG_CONFIG_HOME/awf/workflows/),
//     "env" (AWF_WORKFLOWS_PATH), or "pack".
//   - Scope       — human-readable grouping label. Equals Source for non-pack
//     entries ("local", "global", "env"). For pack-sourced entries it is the
//     pack vendor name (e.g. "acme") rather than "pack".
//   - Workflow    — bare workflow name (no pack prefix); equals Name for
//     local/global/env entries, the filename part for pack entries.
//   - Version     — optional semver string carried from the pack manifest.
//   - Description — optional one-line summary surfaced in listings.
type WorkflowEntry struct {
	Name        string // "my-workflow" or "pack-name/workflow-name"
	Source      string // "local", "global", "env", "pack"
	Scope       string // "local", "global", "env", or pack vendor name
	Workflow    string // local part: plain name for non-pack, wfName for pack
	Version     string
	Description string
}
