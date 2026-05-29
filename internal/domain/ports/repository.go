package ports

import (
	"context"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// WorkflowSource identifies the origin of a discovered workflow.
// It maps directly to the three discovery paths wired by the CLI:
//   - SourceEnv    — AWF_WORKFLOWS_PATH environment variable
//   - SourceLocal  — ./.awf/workflows/ (project-local)
//   - SourceGlobal — $XDG_CONFIG_HOME/awf/workflows/ (user-wide)
type WorkflowSource string

const (
	SourceEnv    WorkflowSource = "env"
	SourceLocal  WorkflowSource = "local"
	SourceGlobal WorkflowSource = "global"
)

// WorkflowInfo carries the minimal metadata returned by ListWithSource.
// It lives in the ports package so that the application layer and the domain
// port interface share the same type without importing infrastructure packages.
type WorkflowInfo struct {
	Name   string
	Source WorkflowSource
	Path   string
}

type WorkflowRepository interface {
	Load(ctx context.Context, name string) (*workflow.Workflow, error)
	List(ctx context.Context) ([]string, error)
	// ListWithSource returns workflow names together with their discovery source.
	// The ordering follows the same priority as List: earlier paths win for
	// duplicates and are listed first.
	ListWithSource(ctx context.Context) ([]WorkflowInfo, error)
	Exists(ctx context.Context, name string) (bool, error)
}

type TemplateRepository interface {
	GetTemplate(ctx context.Context, name string) (*workflow.Template, error)
	ListTemplates(ctx context.Context) ([]string, error)
	Exists(ctx context.Context, name string) bool
}
