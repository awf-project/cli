package application

import (
	"context"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
)

func loadScriptFile(
	ctx context.Context,
	scriptFile string,
	wf *workflow.Workflow,
	intCtx *interpolation.Context,
) (string, error) {
	return loadExternalFile(ctx, scriptFile, wf, intCtx)
}
