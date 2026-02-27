package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteSingleStep_Success(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["test"] = &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()
	executor.results["echo hello"] = &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	result, err := execSvc.ExecuteSingleStep(context.Background(), "test", "start", nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "start", result.StepName)
	assert.Equal(t, "hello\n", result.Output)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, workflow.StatusCompleted, result.Status)
}

func TestExecuteSingleStep_StepNotFound(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["test"] = &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	_, err := execSvc.ExecuteSingleStep(context.Background(), "test", "nonexistent", nil, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExecuteSingleStep_WorkflowNotFound(t *testing.T) {
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	_, err := execSvc.ExecuteSingleStep(context.Background(), "nonexistent", "step", nil, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExecuteSingleStep_WithInputs(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["input-test"] = &workflow.Workflow{
		Name:    "input-test",
		Initial: "process",
		Inputs:  []workflow.Input{{Name: "data", Type: "string"}},
		Steps: map[string]*workflow.Step{
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{inputs.data}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()
	executor.results["echo test-value"] = &ports.CommandResult{Stdout: "test-value\n", ExitCode: 0}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	// Use a resolver that actually interpolates
	resolver := &interpolatingMockResolver{
		mapping: map[string]string{
			"echo {{inputs.data}}": "echo test-value",
		},
	}
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, resolver, nil)

	inputs := map[string]any{"data": "test-value"}
	result, err := execSvc.ExecuteSingleStep(context.Background(), "input-test", "process", inputs, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "test-value\n", result.Output)
}

func TestExecuteSingleStep_WithMocks(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["mock-test"] = &workflow.Workflow{
		Name:    "mock-test",
		Initial: "fetch",
		Steps: map[string]*workflow.Step{
			"fetch": {
				Name:      "fetch",
				Type:      workflow.StepTypeCommand,
				Command:   "curl http://api",
				OnSuccess: "process",
			},
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{states.fetch.output}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()
	executor.results["echo mocked-data"] = &ports.CommandResult{Stdout: "mocked-data\n", ExitCode: 0}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	// Use a resolver that interpolates the mocked state
	resolver := &interpolatingMockResolver{
		mapping: map[string]string{
			"echo {{states.fetch.output}}": "echo mocked-data",
		},
	}
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, resolver, nil)

	// Mock the output of the "fetch" step
	mocks := map[string]string{
		"states.fetch.output": "mocked-data",
	}

	result, err := execSvc.ExecuteSingleStep(context.Background(), "mock-test", "process", nil, mocks)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "mocked-data\n", result.Output)
}

func TestExecuteSingleStep_ExecutesHooks(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["hook-test"] = &workflow.Workflow{
		Name:    "hook-test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo main",
				OnSuccess: "done",
				Hooks: workflow.StepHooks{
					Pre:  workflow.Hook{{Command: "echo pre-hook"}},
					Post: workflow.Hook{{Command: "echo post-hook"}},
				},
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := &hookTrackingExecutor{
		executedCommands: []string{},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	_, err := execSvc.ExecuteSingleStep(context.Background(), "hook-test", "start", nil, nil)

	require.NoError(t, err)
	// Verify hooks were executed
	assert.Contains(t, executor.executedCommands, "echo pre-hook")
	assert.Contains(t, executor.executedCommands, "echo post-hook")
}

func TestExecuteSingleStep_TerminalStepError(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["terminal-test"] = &workflow.Workflow{
		Name:    "terminal-test",
		Initial: "done",
		Steps: map[string]*workflow.Step{
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	_, err := execSvc.ExecuteSingleStep(context.Background(), "terminal-test", "done", nil, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminal")
}

func TestExecuteSingleStep_CommandFails(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["fail-test"] = &workflow.Workflow{
		Name:    "fail-test",
		Initial: "fail",
		Steps: map[string]*workflow.Step{
			"fail": {
				Name:      "fail",
				Type:      workflow.StepTypeCommand,
				Command:   "exit 1",
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	executor.results["exit 1"] = &ports.CommandResult{ExitCode: 1, Stderr: "command failed"}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	result, err := execSvc.ExecuteSingleStep(context.Background(), "fail-test", "fail", nil, nil)

	// Single step execution returns result even on failure (no state machine transition)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.ExitCode)
	assert.Equal(t, workflow.StatusFailed, result.Status)
	assert.Equal(t, "command failed", result.Stderr)
}

// hookTrackingExecutor records all executed commands
type hookTrackingExecutor struct {
	executedCommands []string
}

func (h *hookTrackingExecutor) Execute(ctx context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	h.executedCommands = append(h.executedCommands, cmd.Program)
	return &ports.CommandResult{ExitCode: 0, Stdout: "ok"}, nil
}

// interpolatingMockResolver resolves templates based on a predefined mapping
type interpolatingMockResolver struct {
	mapping map[string]string
}

func (r *interpolatingMockResolver) Resolve(template string, ctx *interpolation.Context) (string, error) {
	if resolved, ok := r.mapping[template]; ok {
		return resolved, nil
	}
	return template, nil
}

// Feature: C054 - Increase Application Layer Test Coverage
// Component: T008 - Extend ExecuteSingleStep tests
//
// These tests target uncovered paths in ExecuteSingleStep to increase coverage
// from 62.3% to 80%+. Focus areas:
// - Template service expansion errors (lines 46-48)
// - Dir interpolation (success and error paths, lines 94-103)
// - Step timeout enforcement (lines 72-76)
//
// Tests follow existing manual mock wiring pattern (pre-C012) to maintain
// consistency with existing tests in this file. New tests are additive only.

func TestExecuteSingleStep_TemplateExpansionError(t *testing.T) {
	tests := []struct {
		name          string
		workflow      *workflow.Workflow
		expectedError string
	}{
		{
			name: "template service returns expansion error",
			workflow: &workflow.Workflow{
				Name:    "template-error-test",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {
						Name:      "start",
						Type:      workflow.StepTypeCommand,
						Command:   "echo test",
						OnSuccess: "done",
					},
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
				},
			},
			expectedError: "expand templates: load template failing-template: template not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()

			// Add template reference to trigger template expansion
			wfWithTemplate := *tt.workflow
			wfWithTemplate.Steps["start"].TemplateRef = &workflow.WorkflowTemplateRef{
				TemplateName: "failing-template",
			}
			repo.workflows[tt.workflow.Name] = &wfWithTemplate

			// Create template service with failing template repository
			templateRepo := &mockTemplateRepository{
				getError: errors.New("template not found"),
			}
			templateSvc := application.NewTemplateService(templateRepo, &mockLogger{})

			wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{}, nil)
			execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)
			execSvc.SetTemplateService(templateSvc)

			result, err := execSvc.ExecuteSingleStep(context.Background(), tt.workflow.Name, "start", nil, nil)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestExecuteSingleStep_DirInterpolation(t *testing.T) {
	tests := []struct {
		name           string
		workdir        string
		expectedDir    string
		commandMapping map[string]string
	}{
		{
			name:        "dir interpolated from input",
			workdir:     "/tmp/workspace",
			expectedDir: "/tmp/workspace",
			commandMapping: map[string]string{
				"pwd": "pwd",
			},
		},
		{
			name:        "dir interpolated with nested path",
			workdir:     "/home/user/project",
			expectedDir: "/home/user/project",
			commandMapping: map[string]string{
				"ls -la": "ls -la",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			repo.workflows["dir-test"] = &workflow.Workflow{
				Name:    "dir-test",
				Initial: "work",
				Inputs:  []workflow.Input{{Name: "workdir", Type: "string"}},
				Steps: map[string]*workflow.Step{
					"work": {
						Name:      "work",
						Type:      workflow.StepTypeCommand,
						Command:   "pwd",
						Dir:       "{{inputs.workdir}}",
						OnSuccess: "done",
					},
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			executor := &dirCapturingExecutor{
				capturedDir: "",
				result:      &ports.CommandResult{Stdout: tt.expectedDir + "\n", ExitCode: 0},
			}

			// Resolver that handles Dir interpolation
			resolver := &interpolatingMockResolver{
				mapping: map[string]string{
					"{{inputs.workdir}}": tt.expectedDir,
					"pwd":                "pwd",
				},
			}

			wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
			execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, resolver, nil)

			inputs := map[string]any{"workdir": tt.workdir}
			result, err := execSvc.ExecuteSingleStep(context.Background(), "dir-test", "work", inputs, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, workflow.StatusCompleted, result.Status)
			assert.Equal(t, tt.expectedDir, executor.capturedDir, "command should be executed in interpolated directory")
		})
	}
}

func TestExecuteSingleStep_DirInterpolationError(t *testing.T) {
	tests := []struct {
		name          string
		dir           string
		resolverError error
		expectedError string
	}{
		{
			name:          "dir interpolation fails with invalid template",
			dir:           "{{inputs.missing}}",
			resolverError: errors.New("undefined variable: inputs.missing"),
			expectedError: "interpolate dir:",
		},
		{
			name:          "dir interpolation fails with malformed template",
			dir:           "{{invalid",
			resolverError: errors.New("unclosed template"),
			expectedError: "interpolate dir:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			repo.workflows["dir-error-test"] = &workflow.Workflow{
				Name:    "dir-error-test",
				Initial: "work",
				Steps: map[string]*workflow.Step{
					"work": {
						Name:      "work",
						Type:      workflow.StepTypeCommand,
						Command:   "pwd",
						Dir:       tt.dir,
						OnSuccess: "done",
					},
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			// Resolver that returns error for dir, but succeeds for command
			resolver := &selectiveErrorResolver{
				commandMapping: map[string]string{
					"pwd": "pwd",
				},
				dirError: tt.resolverError,
			}

			executor := newMockExecutor()
			wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
			execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, resolver, nil)

			result, err := execSvc.ExecuteSingleStep(context.Background(), "dir-error-test", "work", nil, nil)

			require.Error(t, err)
			require.NotNil(t, result, "result should be returned even on error")
			assert.Contains(t, err.Error(), tt.expectedError)
			assert.Equal(t, workflow.StatusFailed, result.Status)
			assert.Contains(t, result.Error, "interpolate dir:")
		})
	}
}

func TestExecuteSingleStep_StepTimeout(t *testing.T) {
	tests := []struct {
		name           string
		timeout        int
		executorDelay  time.Duration
		expectTimeout  bool
		expectedStatus workflow.ExecutionStatus
	}{
		{
			name:           "command completes within timeout",
			timeout:        2,
			executorDelay:  100 * time.Millisecond,
			expectTimeout:  false,
			expectedStatus: workflow.StatusCompleted,
		},
		{
			name:           "command exceeds timeout",
			timeout:        1,
			executorDelay:  2 * time.Second,
			expectTimeout:  true,
			expectedStatus: workflow.StatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			repo.workflows["timeout-test"] = &workflow.Workflow{
				Name:    "timeout-test",
				Initial: "slow",
				Steps: map[string]*workflow.Step{
					"slow": {
						Name:      "slow",
						Type:      workflow.StepTypeCommand,
						Command:   "sleep 10",
						Timeout:   tt.timeout,
						OnSuccess: "done",
					},
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			// Executor that simulates delay
			executor := &delayedExecutor{
				delay: tt.executorDelay,
			}

			wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
			execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

			result, err := execSvc.ExecuteSingleStep(context.Background(), "timeout-test", "slow", nil, nil)

			if tt.expectTimeout {
				// Timeout results in failed status with context deadline error
				require.NoError(t, err, "ExecuteSingleStep returns result, not error")
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedStatus, result.Status)
				assert.Contains(t, result.Error, "context deadline exceeded")
			} else {
				// Success case
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedStatus, result.Status)
			}
		})
	}
}

// dirCapturingExecutor captures the Dir field from Command to verify interpolation
type dirCapturingExecutor struct {
	capturedDir string
	result      *ports.CommandResult
}

func (d *dirCapturingExecutor) Execute(ctx context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	d.capturedDir = cmd.Dir
	return d.result, nil
}

// selectiveErrorResolver returns error for dir interpolation but succeeds for commands
type selectiveErrorResolver struct {
	commandMapping map[string]string
	dirError       error
}

func (s *selectiveErrorResolver) Resolve(template string, ctx *interpolation.Context) (string, error) {
	// Check if this is a dir template (contains inputs.workdir or similar patterns)
	if s.dirError != nil && (strings.Contains(template, "{{inputs.") || strings.Contains(template, "{{invalid")) {
		return "", s.dirError
	}

	// Otherwise resolve normally using mapping
	if resolved, ok := s.commandMapping[template]; ok {
		return resolved, nil
	}
	return template, nil
}

// delayedExecutor simulates command execution delay for timeout testing
type delayedExecutor struct {
	delay time.Duration
}

func (d *delayedExecutor) Execute(ctx context.Context, cmd *ports.Command) (*ports.CommandResult, error) {
	select {
	case <-time.After(d.delay):
		return &ports.CommandResult{Stdout: "completed", ExitCode: 0}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// mockTemplateRepository is a failing template repository for testing template expansion errors
type mockTemplateRepository struct {
	getError error
}

func (m *mockTemplateRepository) GetTemplate(ctx context.Context, name string) (*workflow.Template, error) {
	return nil, m.getError
}

func (m *mockTemplateRepository) ListTemplates(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockTemplateRepository) Exists(ctx context.Context, name string) bool {
	return false
}
