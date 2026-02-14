package ports

import (
	"context"

	"github.com/vanoix/awf/internal/domain/workflow"
)

type WorkflowRepository interface {
	Load(ctx context.Context, name string) (*workflow.Workflow, error)
	List(ctx context.Context) ([]string, error)
	Exists(ctx context.Context, name string) (bool, error)
}

type TemplateRepository interface {
	GetTemplate(ctx context.Context, name string) (*workflow.Template, error)
	ListTemplates(ctx context.Context) ([]string, error)
	Exists(ctx context.Context, name string) bool
}
