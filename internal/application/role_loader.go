package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
)

// ResolveAgentRole resolves a role reference to an AgentRole.
// An empty ref is a no-op and returns (nil, nil).
// References containing path separators or starting with '.', '~', '/' are treated as paths;
// everything else is resolved by name through the repository.
func ResolveAgentRole(ctx context.Context, repo ports.AgentRoleRepository, ref, workflowDir string) (*workflow.AgentRole, error) {
	if ref == "" {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !isRolePathRef(ref) {
		role, err := repo.Load(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("resolve role %q: %w", ref, err)
		}
		return role, nil
	}
	absPath, err := expandRolePath(ref, workflowDir)
	if err != nil {
		return nil, fmt.Errorf("resolve role %q: %w", ref, err)
	}
	role, err := repo.LoadFromPath(ctx, absPath)
	if err != nil {
		return nil, fmt.Errorf("resolve role %q: %w", ref, err)
	}
	return role, nil
}

// isRolePathRef reports whether ref should be treated as a filesystem path.
// Refs containing path separators or starting with '.', '~', '/' are paths;
// everything else is a symbolic name for repo.Load.
func isRolePathRef(ref string) bool {
	if ref == "" {
		return false
	}
	return strings.ContainsAny(ref, `/\`) || ref[0] == '.' || ref[0] == '~'
}

// expandRolePath converts a path ref to an absolute path.
func expandRolePath(ref, workflowDir string) (string, error) {
	if strings.HasPrefix(ref, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(homeDir, ref[2:]), nil
	}
	if ref == "~" {
		return os.UserHomeDir()
	}
	if filepath.IsAbs(ref) {
		return filepath.Clean(ref), nil
	}
	return filepath.Join(workflowDir, ref), nil
}

// BuildRoleSystemPrompt resolves the agent role template, loads role content
// from the repository, and composes the final system prompt.
func BuildRoleSystemPrompt(
	ctx context.Context,
	repo ports.AgentRoleRepository,
	resolver interpolation.Resolver,
	step *workflow.Step,
	workflowDir string,
	intCtx *interpolation.Context,
) (string, error) {
	if step.Agent == nil {
		return "", nil
	}
	roleRef := step.Agent.Role
	if roleRef != "" {
		resolved, err := resolver.Resolve(roleRef, intCtx)
		if err != nil {
			return "", fmt.Errorf("resolve role template: %w", err)
		}
		roleRef = resolved
	}
	if roleRef == "" {
		return ComposeSystemPrompt("", step.Agent.SystemPrompt), nil
	}
	if repo == nil {
		return "", domainerrors.NewUserError(
			domainerrors.ErrorCodeUserInputMissingRole,
			fmt.Sprintf("agent role %q requires a configured role repository; set one via SetAgentRoleRepository", roleRef),
			map[string]any{"role": roleRef},
			nil,
		)
	}
	role, err := ResolveAgentRole(ctx, repo, roleRef, workflowDir)
	if err != nil {
		return "", err
	}
	if role == nil {
		return ComposeSystemPrompt("", step.Agent.SystemPrompt), nil
	}
	return ComposeSystemPrompt(role.Content, step.Agent.SystemPrompt), nil
}

// ComposeSystemPrompt concatenates role content and an inline system_prompt.
// Composition order: role + "\n\n" + inline. Empty parts are omitted.
func ComposeSystemPrompt(roleContent, inline string) string {
	if roleContent == "" {
		return inline
	}
	if inline == "" {
		return roleContent
	}
	return roleContent + "\n\n" + inline
}
