package api

import "github.com/awf-project/cli/internal/domain/ports"

// isRepoScope reports whether the given scope sentinel maps to one of the
// CompositeRepository's own sources (env / local / global), as opposed to a
// vendor pack name. These scopes never prefix the workflow identifier — the
// bare name is enough for WorkflowService.GetWorkflow to resolve via the
// repository, which already searches all three sources in priority order.
//
// Deriving the set from the ports.SourceXxx constants keeps the URL grammar
// in lockstep with the domain definitions; adding a new WorkflowSource will
// force a compile-time update here rather than a silent desync.
func isRepoScope(scope string) bool {
	switch ports.WorkflowSource(scope) {
	case ports.SourceEnv, ports.SourceLocal, ports.SourceGlobal:
		return true
	}
	return false
}

// recomposeIdentifier reconstructs the canonical workflow identifier from the
// (scope, name) path components of the HTTP routes.
//
// The grammar of `/api/workflows/{scope}/{name}` decomposes the identifier
// used by the application layer:
//   - scope in {"local", "global", "env"} → identifier is the bare workflow
//     name (e.g. "deploy-prod"); the CompositeRepository resolves it across
//     its configured sources.
//   - any other scope → identifier is "scope/name" (e.g. "speckit/specify");
//     this is how pack-sourced workflows are addressed by WorkflowService.
//
// This helper is the single source of truth for the URL ↔ identifier mapping
// shared by workflow and execution handlers.
func recomposeIdentifier(scope, name string) string {
	if isRepoScope(scope) {
		return name
	}
	return scope + "/" + name
}
