package workflow_test

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
)

func TestExecutionContextGetInputsReturnsIndependentMap(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "workflow")
	ctx.SetInput("name", "original")
	snapshot := ctx.GetInputs()
	snapshot["name"] = "changed"
	snapshot["extra"] = true
	assert.Equal(t, map[string]any{"name": "original"}, ctx.GetInputs())
}
