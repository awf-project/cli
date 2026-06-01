package ports

import "context"

// ACPClient is the domain port for agent→editor approval callbacks.
// Scope: one method only (F102 v1); fs/terminal methods are deferred per spec Decision #2(b).
type ACPClient interface {
	RequestPermission(ctx context.Context, req PermissionRequest) (PermissionResponse, error)
}

// PermissionRequest carries the data the editor needs to present a permission prompt.
type PermissionRequest struct {
	SessionID  string
	ToolCallID string
	Prompt     string
	Options    []PermissionOption
}

// PermissionOption represents a selectable choice in a permission prompt.
// Kind is "allow" or "deny".
type PermissionOption struct {
	ID    string
	Label string
	Kind  string
}

// PermissionResponse carries the user's selection.
// OptionID == "" means the prompt was cancelled without a selection.
type PermissionResponse struct {
	OptionID string
}
