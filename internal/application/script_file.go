package application

import (
	"context"

	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/pkg/interpolation"
)

func loadScriptFile(
	ctx context.Context,
	scriptFile string,
	wf *workflow.Workflow,
	intCtx *interpolation.Context,
) (string, error) {
	return loadExternalFile(ctx, scriptFile, wf, intCtx)
}
