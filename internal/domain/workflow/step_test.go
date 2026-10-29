package workflow_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	"github.com/awf-project/cli/internal/infrastructure/expression"
	"github.com/awf-project/cli/internal/infrastructure/logger"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStep_TimeoutPresence exercises the public dry-run path so the projection
// contract cannot be satisfied merely by declaring matching domain fields.
func TestStep_TimeoutPresence(t *testing.T) {
	workflowDir := t.TempDir()
	const workflowYAML = `name: timeout-presence
version: "1.0.0"
states:
  initial: omitted
  omitted:
    type: step
    command: echo omitted
    on_success: explicit_zero
  explicit_zero:
    type: step
    command: echo zero
    timeout: 0s
    on_success: configured
  configured:
    type: step
    command: echo configured
    timeout: 45s
    on_success: agent
  agent:
    type: agent
    provider: claude
    prompt: summarize
    timeout: 30s
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowDir, "timeout-presence.yaml"), []byte(workflowYAML), 0o600))

	repo := repository.NewYAMLRepository(workflowDir)
	service := application.NewWorkflowService(repo, store.NewJSONStore(t.TempDir()), executor.NewShellExecutor(), logger.NopLogger{}, nil)
	dryRun := application.NewDryRunExecutor(service, interpolation.NewTemplateResolver(), expression.NewExprEvaluator(), logger.NopLogger{})

	plan, err := dryRun.Execute(context.Background(), "timeout-presence", nil)
	require.NoError(t, err)

	steps := make(map[string]workflow.DryRunStep, len(plan.Steps))
	for _, step := range plan.Steps {
		steps[step.Name] = step
	}

	tests := []struct {
		name        string
		stepName    string
		wantTimeout int
		wantSet     bool
		wantAgent   int
	}{
		{name: "omitted step timeout", stepName: "omitted", wantTimeout: 0, wantSet: false},
		{name: "explicit zero step timeout", stepName: "explicit_zero", wantTimeout: 0, wantSet: true},
		{name: "positive step timeout", stepName: "configured", wantTimeout: 45, wantSet: true},
		{name: "agent timeout preserves presence", stepName: "agent", wantTimeout: 0, wantSet: true, wantAgent: 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step, ok := steps[tt.stepName]
			require.True(t, ok, "dry-run plan must include %q", tt.stepName)
			assert.Equal(t, tt.wantTimeout, step.Timeout)
			assert.Equal(t, tt.wantSet, step.TimeoutSet)
			if tt.wantAgent > 0 {
				require.NotNil(t, step.Agent)
				assert.Equal(t, tt.wantAgent, step.Agent.Timeout)
			}
		})
	}
}

func TestIsQualifiedStepType(t *testing.T) {
	for _, name := range []string{"database.query", "awf-plugin-db.execute_query", "fixture.wait", "command", "", ".wait", "fixture.", "a.b.c", "fixture.bad/type", "fixture.bad type"} {
		t.Run(name, func(t *testing.T) {
			want := name == "database.query" || name == "awf-plugin-db.execute_query" || name == "fixture.wait"
			assert.Equal(t, want, workflow.IsQualifiedStepType(name))
		})
	}
}
