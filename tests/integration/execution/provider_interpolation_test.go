//go:build integration

// Feature: B014

package execution_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infraExpr "github.com/awf-project/cli/internal/infrastructure/expression"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestExecSvc(tempDir string) *application.ExecutionService {
	log := &mockExecutionLogger{}
	repo := repository.NewYAMLRepository(tempDir)
	store := newMockStateStore()
	shellExec := executor.NewShellExecutor()
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, shellExec, log, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(log)
	return application.NewExecutionService(wfSvc, shellExec, parallelExec, store, log, resolver, nil)
}

func TestProviderInterpolation_AgentStep_Integration(t *testing.T) {
	tempDir := t.TempDir()

	workflowYAML := `name: provider-interp
version: "1.0.0"

inputs:
  - name: agent
    type: string
    required: true
    description: "Provider name to use"

states:
  initial: ask

  ask:
    type: agent
    provider: "{{inputs.agent}}"
    prompt: "Hello world"
    on_success: done
    on_failure: error

  done:
    type: terminal
    status: success

  error:
    type: terminal
    status: failure
`
	require.NoError(t, os.WriteFile(
		filepath.Join(tempDir, "provider-interp.yaml"),
		[]byte(workflowYAML), 0o644,
	))

	execSvc := newTestExecSvc(tempDir)

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider:    "claude",
			Output:      "Resolved provider executed",
			Tokens:      42,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		}, nil
	})
	_ = registry.Register(claude)
	execSvc.SetAgentRegistry(registry)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "provider-interp", map[string]any{"agent": "claude"})

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, "done", execCtx.CurrentStep)

	state, ok := execCtx.GetStepState("ask")
	require.True(t, ok)
	assert.Equal(t, "Resolved provider executed", state.Output)
}

func TestProviderInterpolation_ResolvedNotFound_Integration(t *testing.T) {
	tempDir := t.TempDir()

	workflowYAML := `name: provider-not-found
version: "1.0.0"

inputs:
  - name: agent
    type: string
    required: true

states:
  initial: ask

  ask:
    type: agent
    provider: "{{inputs.agent}}"
    prompt: "Hello"
    on_success: done
    on_failure: error

  done:
    type: terminal
    status: success

  error:
    type: terminal
    status: failure
`
	require.NoError(t, os.WriteFile(
		filepath.Join(tempDir, "provider-not-found.yaml"),
		[]byte(workflowYAML), 0o644,
	))

	execSvc := newTestExecSvc(tempDir)

	registry := mocks.NewMockAgentRegistry()
	execSvc.SetAgentRegistry(registry)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "provider-not-found", map[string]any{"agent": "nonexistent"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
}

func TestProviderInterpolation_LiteralProvider_Integration(t *testing.T) {
	tempDir := t.TempDir()

	workflowYAML := `name: literal-provider
version: "1.0.0"

states:
  initial: ask

  ask:
    type: agent
    provider: claude
    prompt: "Hello world"
    on_success: done

  done:
    type: terminal
    status: success
`
	require.NoError(t, os.WriteFile(
		filepath.Join(tempDir, "literal-provider.yaml"),
		[]byte(workflowYAML), 0o644,
	))

	execSvc := newTestExecSvc(tempDir)

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider:    "claude",
			Output:      "OK",
			Tokens:      10,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		}, nil
	})
	_ = registry.Register(claude)
	execSvc.SetAgentRegistry(registry)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "literal-provider", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
}
