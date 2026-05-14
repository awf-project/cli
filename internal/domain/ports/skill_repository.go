package ports

import (
	"context"

	"github.com/awf-project/cli/internal/domain/workflow"
)

type SkillRepository interface {
	Load(ctx context.Context, name string) (*workflow.Skill, error)
	LoadFromPath(ctx context.Context, absolutePath string) (*workflow.Skill, error)
}
