package ports

import (
	"context"

	"github.com/awf-project/cli/internal/domain/workflow"
)

type AgentRoleRepository interface {
	Load(ctx context.Context, name string) (*workflow.AgentRole, error)
	LoadFromPath(ctx context.Context, absolutePath string) (*workflow.AgentRole, error)
}
