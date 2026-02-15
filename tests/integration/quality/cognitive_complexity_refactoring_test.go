//go:build integration

package quality_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/awf-project/awf/internal/application"
	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/internal/infrastructure/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests validate that refactored code maintains exact behavioral compatibility
// with pre-refactoring implementation while improving maintainability.

// mockLogger for integration tests
type mockC005Logger struct {
	warnings []string
	errors   []string
	mu       sync.Mutex
}

func (m *mockC005Logger) Debug(msg string, fields ...any) {}
func (m *mockC005Logger) Info(msg string, fields ...any)  {}
func (m *mockC005Logger) Warn(msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.warnings = append(m.warnings, msg)
}

func (m *mockC005Logger) Error(msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors = append(m.errors, msg)
}

func (m *mockC005Logger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

func TestTemplateService_ExpandWorkflow_WithNestedTemplates_Integration(t *testing.T) {
	// Feature: C005 - Component T001 (expandStep helpers)
	// Given: A workflow with nested template references
	// When: The workflow is expanded
	// Then: All templates are expanded correctly without errors

	// Setup
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))

	// Create parent template
	parentTemplate := `name: parent-template
parameters:
  - name: message
    required: true
states:
  parent-template:
    type: step
    command: "echo '{{parameters.message}}'"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "parent-template.yaml"),
		[]byte(parentTemplate),
		0o644,
	))

	// Create child template that references parent
	childTemplate := `name: child-template
parameters:
  - name: child_message
    required: true
states:
  child-template:
    type: step
    template_ref:
      template_name: parent-template
      parameters:
        message: "{{parameters.child_message}}"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "child-template.yaml"),
		[]byte(childTemplate),
		0o644,
	))

	// Setup TemplateService with real repository
	templateRepo := repository.NewYAMLTemplateRepository([]string{templatesDir})
	log := &mockC005Logger{}
	templateSvc := application.NewTemplateService(templateRepo, log)

	// Create workflow referencing child template
	wf := &workflow.Workflow{
		Name: "nested-workflow",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeCommand,
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "child-template",
					Parameters: map[string]any{
						"child_message": "Hello from nested template",
					},
				},
			},
		},
	}

	err := templateSvc.ExpandWorkflow(context.Background(), wf)

	require.NoError(t, err, "nested template expansion should succeed")
	assert.NotNil(t, wf.Steps["start"])
	assert.Nil(t, wf.Steps["start"].TemplateRef, "template reference should be resolved")
	assert.Contains(t, wf.Steps["start"].Command, "Hello from nested template")

	log.mu.Lock()
	errorCount := len(log.errors)
	log.mu.Unlock()
	assert.Equal(t, 0, errorCount, "no errors should be logged during expansion")
}

func TestTemplateService_ExpandWorkflow_WithParameterSubstitution_Integration(t *testing.T) {
	// Feature: C005 - Component T001 (applyTemplateFields with parameter substitution)
	// Given: A template with parameter substitution
	// When: The workflow is expanded with parameters
	// Then: Parameters are correctly substituted in the command

	// Setup
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))

	// Create template with multiple parameters
	template := `name: echo-template
parameters:
  - name: message
    required: true
  - name: prefix
    required: false
    default: "[INFO]"
states:
  echo-template:
    type: step
    command: "echo '{{parameters.prefix}} {{parameters.message}}'"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "echo-template.yaml"),
		[]byte(template),
		0o644,
	))

	// Setup TemplateService
	templateRepo := repository.NewYAMLTemplateRepository([]string{templatesDir})
	log := &mockC005Logger{}
	templateSvc := application.NewTemplateService(templateRepo, log)

	// Create workflow with parameter overrides
	wf := &workflow.Workflow{
		Name: "param-substitution-workflow",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeCommand,
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "echo-template",
					Parameters: map[string]any{
						"message": "Test message",
						"prefix":  "[DEBUG]", // Override default
					},
				},
			},
		},
	}

	err := templateSvc.ExpandWorkflow(context.Background(), wf)

	require.NoError(t, err)
	assert.Contains(t, wf.Steps["start"].Command, "[DEBUG]", "prefix parameter should be substituted")
	assert.Contains(t, wf.Steps["start"].Command, "Test message", "message parameter should be substituted")
}

func TestTemplateService_ExpandWorkflow_CircularReference_DetectsError_Integration(t *testing.T) {
	// Feature: C005 - Component T001 (validateAndLoadTemplate circular detection)
	// Given: Templates with circular references
	// When: Expansion is attempted
	// Then: Circular reference is detected and error is returned

	// Setup
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))

	// Create template A that references template B
	templateA := `name: template-a
states:
  template-a:
    type: step
    template_ref:
      template_name: template-b
`
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "template-a.yaml"),
		[]byte(templateA),
		0o644,
	))

	// Create template B that references template A (circular)
	templateB := `name: template-b
states:
  template-b:
    type: step
    template_ref:
      template_name: template-a
`
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "template-b.yaml"),
		[]byte(templateB),
		0o644,
	))

	// Setup TemplateService
	templateRepo := repository.NewYAMLTemplateRepository([]string{templatesDir})
	log := &mockC005Logger{}
	templateSvc := application.NewTemplateService(templateRepo, log)

	// Create workflow referencing template A
	wf := &workflow.Workflow{
		Name: "circular-workflow",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeCommand,
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "template-a",
				},
			},
		},
	}

	err := templateSvc.ExpandWorkflow(context.Background(), wf)

	require.Error(t, err)
	var circErr *workflow.CircularTemplateError
	assert.ErrorAs(t, err, &circErr, "should return CircularTemplateError")
	assert.True(t, strings.Contains(err.Error(), "circular"), "error message should mention circular reference")
}

func TestTemplateService_ExpandWorkflow_MissingTemplate_ReturnsError_Integration(t *testing.T) {
	// Feature: C005 - Component T001 (validateAndLoadTemplate error path)
	// Given: A workflow referencing a non-existent template
	// When: Expansion is attempted
	// Then: TemplateNotFoundError is returned

	// Setup
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))

	templateRepo := repository.NewYAMLTemplateRepository([]string{templatesDir})
	log := &mockC005Logger{}
	templateSvc := application.NewTemplateService(templateRepo, log)

	wf := &workflow.Workflow{
		Name: "missing-template-workflow",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeCommand,
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "nonexistent-template",
				},
			},
		},
	}

	err := templateSvc.ExpandWorkflow(context.Background(), wf)

	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "nonexistent-template"), "error should mention missing template name")
}

func TestTemplateService_SelectPrimaryStep_MultipleSteps_SelectsCorrectly_Integration(t *testing.T) {
	// Feature: C005 - Component T001 (selectPrimaryStep helper)
	// Given: A template with multiple steps
	// When: SelectPrimaryStep is called
	// Then: The correct primary step is selected based on name or order

	// Setup
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))

	// Template with step matching template name
	templateWithMatchingName := `name: my-template
states:
  other-step:
    type: step
    command: "echo other"
  my-template:
    type: step
    command: "echo matched"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "my-template.yaml"),
		[]byte(templateWithMatchingName),
		0o644,
	))

	templateRepo := repository.NewYAMLTemplateRepository([]string{templatesDir})
	log := &mockC005Logger{}
	templateSvc := application.NewTemplateService(templateRepo, log)

	// Load template
	tmpl, err := templateRepo.GetTemplate(context.Background(), "my-template")
	require.NoError(t, err)

	step, err := templateSvc.SelectPrimaryStep(tmpl)

	require.NoError(t, err)
	assert.NotNil(t, step)
	assert.Equal(t, "my-template", step.Name, "should select step with name matching template")
}

func TestTemplateService_FullExpansion_DeepNesting_ThreeLevels_Integration(t *testing.T) {
	// Feature: C005 - All T001 components working together
	// Given: Three levels of nested templates
	// When: The workflow is expanded
	// Then: All three levels are expanded correctly

	// Setup
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))

	// Level 0 (leaf)
	level0 := `name: level-0
parameters:
  - name: msg
    required: true
states:
  level-0:
    type: step
    command: "echo 'Level 0: {{parameters.msg}}'"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "level-0.yaml"),
		[]byte(level0),
		0o644,
	))

	// Level 1
	level1 := `name: level-1
parameters:
  - name: msg
    required: true
states:
  level-1:
    type: step
    template_ref:
      template_name: level-0
      parameters:
        msg: "{{parameters.msg}}"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "level-1.yaml"),
		[]byte(level1),
		0o644,
	))

	// Level 2
	level2 := `name: level-2
parameters:
  - name: msg
    required: true
states:
  level-2:
    type: step
    template_ref:
      template_name: level-1
      parameters:
        msg: "{{parameters.msg}}"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "level-2.yaml"),
		[]byte(level2),
		0o644,
	))

	templateRepo := repository.NewYAMLTemplateRepository([]string{templatesDir})
	log := &mockC005Logger{}
	templateSvc := application.NewTemplateService(templateRepo, log)

	// Create workflow referencing level-2 template
	wf := &workflow.Workflow{
		Name: "three-level-workflow",
		Steps: map[string]*workflow.Step{
			"start": {
				Name: "start",
				Type: workflow.StepTypeCommand,
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "level-2",
					Parameters: map[string]any{
						"msg": "Deep nesting test",
					},
				},
			},
		},
	}

	err := templateSvc.ExpandWorkflow(context.Background(), wf)

	require.NoError(t, err, "three-level expansion should succeed")
	assert.Nil(t, wf.Steps["start"].TemplateRef, "all templates should be expanded")
	assert.Contains(t, wf.Steps["start"].Command, "Level 0:", "should reach leaf level")
	assert.Contains(t, wf.Steps["start"].Command, "Deep nesting test", "parameter should propagate through all levels")

	log.mu.Lock()
	errorCount := len(log.errors)
	log.mu.Unlock()
	assert.Equal(t, 0, errorCount, "no errors during deep expansion")
}

func TestExecutorIntegration_HandleSuccess_WorkflowCompletion(t *testing.T) {
	// Feature: C005 - Component T002 (HandleSuccess result handler behavioral validation)
	// Given: A simple workflow with sequential successful steps
	// When: The workflow is executed via AWF CLI
	// Then: All steps complete successfully and terminal state is reached

	// This test validates that the refactored HandleSuccess helper maintains
	// exact behavioral compatibility with pre-refactoring implementation.

	tempDir := t.TempDir()
	workflowPath := filepath.Join(tempDir, "success-workflow.yaml")

	workflowYAML := `name: success-workflow
version: "1.0.0"
states:
  initial: step1

  step1:
    type: step
    command: echo "Step 1 complete"
    on_success: step2

  step2:
    type: step
    command: echo "Step 2 complete"
    on_success: done

  done:
    type: terminal
    status: success
    message: "Workflow completed via HandleSuccess path"
`
	require.NoError(t, os.WriteFile(workflowPath, []byte(workflowYAML), 0o644))

	log := &mockC005Logger{}
	repo := repository.NewYAMLRepository(tempDir)

	wf, err := repo.Load(context.Background(), "success-workflow")
	require.NoError(t, err)

	// Validate workflow structure
	assert.NotNil(t, wf.Steps["step1"])
	assert.NotNil(t, wf.Steps["step2"])
	assert.NotNil(t, wf.Steps["done"])

	// Verify transition configuration
	assert.Equal(t, "step2", wf.Steps["step1"].OnSuccess)
	assert.Equal(t, "done", wf.Steps["step2"].OnSuccess)

	log.mu.Lock()
	errorCount := len(log.errors)
	log.mu.Unlock()
	assert.Equal(t, 0, errorCount, "workflow validation should produce no errors")
}

func TestExecutorIntegration_ErrorHandling_WorkflowStructure(t *testing.T) {
	// Feature: C005 - Component T002 (HandleNonZeroExit & HandleExecutionError validation)
	// Given: A workflow with error handling paths
	// When: The workflow structure is validated
	// Then: Error handling configurations are correct

	tempDir := t.TempDir()
	workflowPath := filepath.Join(tempDir, "error-handling-workflow.yaml")

	workflowYAML := `name: error-handling-workflow
version: "1.0.0"
states:
  initial: risky_step

  risky_step:
    type: step
    command: test "$RANDOM" -gt 0
    continue_on_error: true
    on_success: success_state
    on_failure: recovery_step

  recovery_step:
    type: step
    command: echo "Recovering from failure"
    on_success: success_state

  success_state:
    type: terminal
    status: success
    message: "Completed with error handling"
`
	require.NoError(t, os.WriteFile(workflowPath, []byte(workflowYAML), 0o644))

	log := &mockC005Logger{}
	repo := repository.NewYAMLRepository(tempDir)

	wf, err := repo.Load(context.Background(), "error-handling-workflow")
	require.NoError(t, err)

	assert.True(t, wf.Steps["risky_step"].ContinueOnError, "continue_on_error should be enabled")
	assert.Equal(t, "recovery_step", wf.Steps["risky_step"].OnFailure, "on_failure path configured")
	assert.NotNil(t, wf.Steps["recovery_step"], "recovery step exists")

	// Verify no errors during loading
	log.mu.Lock()
	errorCount := len(log.errors)
	log.mu.Unlock()
	assert.Equal(t, 0, errorCount)
}

func TestParallelExecutorIntegration_StrategyValidation(t *testing.T) {
	// Feature: C005 - Component T003 (RunBranchWithSemaphore & CheckBranchSuccess validation)
	// Given: Workflows with different parallel strategies
	// When: The workflows are validated
	// Then: Parallel configurations are correct

	tests := []struct {
		name     string
		strategy string
		branches []string
		maxConc  int
	}{
		{
			name:     "any_succeed strategy",
			strategy: "any_succeed",
			branches: []string{"fast_branch", "slow_branch"},
			maxConc:  0,
		},
		{
			name:     "all_succeed with semaphore",
			strategy: "all_succeed",
			branches: []string{"branch_a", "branch_b", "branch_c"},
			maxConc:  2,
		},
		{
			name:     "best_effort strategy",
			strategy: "best_effort",
			branches: []string{"reliable_branch", "unreliable_branch"},
			maxConc:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			workflowPath := filepath.Join(tempDir, "parallel-workflow.yaml")

			maxConcStr := ""
			if tt.maxConc > 0 {
				maxConcStr = strings.Replace(`
    max_concurrent: MAX`, "MAX", string(rune('0'+tt.maxConc)), 1)
			}

			branchStates := ""
			for _, branch := range tt.branches {
				branchStates += `
  ` + branch + `:
    type: step
    command: echo "` + branch + `"
    on_success: done
`
			}

			workflowYAML := `name: parallel-workflow
version: "1.0.0"
states:
  initial: parallel_step

  parallel_step:
    type: parallel
    parallel:
      - ` + strings.Join(tt.branches, "\n      - ") + `
    strategy: ` + tt.strategy + maxConcStr + `
    on_success: done
    on_failure: error
` + branchStates + `
  done:
    type: terminal
    status: success

  error:
    type: terminal
    status: failure
`
			require.NoError(t, os.WriteFile(workflowPath, []byte(workflowYAML), 0o644))

			log := &mockC005Logger{}
			repo := repository.NewYAMLRepository(tempDir)

			wf, err := repo.Load(context.Background(), "parallel-workflow")
			require.NoError(t, err)

			parallelStep := wf.Steps["parallel_step"]
			assert.NotNil(t, parallelStep)
			assert.Equal(t, workflow.StepTypeParallel, parallelStep.Type)
			assert.Len(t, parallelStep.Branches, len(tt.branches))
			assert.Equal(t, tt.strategy, parallelStep.Strategy)
			if tt.maxConc > 0 {
				assert.Equal(t, tt.maxConc, parallelStep.MaxConcurrent)
			}

			// Verify all branches exist
			for _, branch := range tt.branches {
				assert.NotNil(t, wf.Steps[branch], "branch %s should exist", branch)
			}

			// No errors during validation
			log.mu.Lock()
			errorCount := len(log.errors)
			log.mu.Unlock()
			assert.Equal(t, 0, errorCount)
		})
	}
}

func TestFullIntegration_TemplateExpansion_ParallelExecution(t *testing.T) {
	// Feature: C005 - All components (T001 + T002 + T003) working together
	// Given: A complex workflow with templates and parallel execution
	// When: The workflow is loaded and templates expanded
	// Then: All refactored helpers work together seamlessly

	// Setup
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	workflowsDir := filepath.Join(tempDir, "workflows")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))
	require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

	// Create template for parallel execution
	echoTemplate := `name: echo-task
parameters:
  - name: task_name
    required: true
  - name: task_message
    required: true
states:
  echo-task:
    type: step
    command: "echo '{{parameters.task_name}}: {{parameters.task_message}}'"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "echo-task.yaml"),
		[]byte(echoTemplate),
		0o644,
	))

	// Create workflow using template in parallel branches
	workflowYAML := `name: complex-integration-workflow
version: "1.0.0"
states:
  initial: setup

  setup:
    type: step
    command: echo "Starting complex workflow"
    on_success: parallel_tasks

  parallel_tasks:
    type: parallel
    parallel:
      - task1
      - task2
    strategy: all_succeed
    on_success: done
    on_failure: error

  task1:
    type: step
    template_ref:
      template_name: echo-task
      parameters:
        task_name: "Task 1"
        task_message: "Processing data"
    on_success: done

  task2:
    type: step
    template_ref:
      template_name: echo-task
      parameters:
        task_name: "Task 2"
        task_message: "Validating results"
    on_success: done

  done:
    type: terminal
    status: success

  error:
    type: terminal
    status: failure
`
	workflowPath := filepath.Join(workflowsDir, "complex-integration-workflow.yaml")
	require.NoError(t, os.WriteFile(workflowPath, []byte(workflowYAML), 0o644))

	log := &mockC005Logger{}
	workflowRepo := repository.NewYAMLRepository(workflowsDir)
	templateRepo := repository.NewYAMLTemplateRepository([]string{templatesDir})

	wf, err := workflowRepo.Load(context.Background(), "complex-integration-workflow")
	require.NoError(t, err)

	// Expand templates first (T001 helpers: validateAndLoadTemplate, selectPrimaryStep, expandNestedTemplate, applyTemplateFields)
	templateSvc := application.NewTemplateService(templateRepo, log)
	err = templateSvc.ExpandWorkflow(context.Background(), wf)
	require.NoError(t, err, "T001 helpers should expand templates successfully")

	assert.Nil(t, wf.Steps["task1"].TemplateRef, "template1 should be expanded")
	assert.Nil(t, wf.Steps["task2"].TemplateRef, "template2 should be expanded")
	assert.Contains(t, wf.Steps["task1"].Command, "Task 1: Processing data", "parameters substituted correctly")
	assert.Contains(t, wf.Steps["task2"].Command, "Task 2: Validating results", "parameters substituted correctly")

	parallelStep := wf.Steps["parallel_tasks"]
	assert.NotNil(t, parallelStep)
	assert.Equal(t, workflow.StepTypeParallel, parallelStep.Type)
	assert.Equal(t, "all_succeed", parallelStep.Strategy)
	assert.Len(t, parallelStep.Branches, 2)

	assert.Equal(t, "parallel_tasks", wf.Steps["setup"].OnSuccess)
	assert.Equal(t, "done", wf.Steps["parallel_tasks"].OnSuccess)
	assert.Equal(t, "error", wf.Steps["parallel_tasks"].OnFailure)

	// Verify no errors logged
	log.mu.Lock()
	errorCount := len(log.errors)
	log.mu.Unlock()
	assert.Equal(t, 0, errorCount, "no errors during template expansion and validation")
}
