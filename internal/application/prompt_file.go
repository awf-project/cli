package application

import (
	"context"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
)

// loadPromptFile loads a prompt template file referenced by AgentConfig.PromptFile.
func loadPromptFile(
	ctx context.Context,
	promptFile string,
	wf *workflow.Workflow,
	intCtx *interpolation.Context,
) (string, error) {
	return loadExternalFile(ctx, promptFile, wf, intCtx)
}
