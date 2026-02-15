package application_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/awf/internal/application"
	domerrors "github.com/awf-project/awf/internal/domain/errors"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create a basic workflow with agent step and terminal state
func newTestWorkflowWithPromptFile(name, sourceDir, promptFile string) *workflow.Workflow {
	return &workflow.Workflow{
		Name:      name,
		Initial:   "agent_step",
		SourceDir: sourceDir,
		Steps: map[string]*workflow.Step{
			"agent_step": {
				Name: "agent_step",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:   "claude",
					PromptFile: promptFile,
				},
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}
}

func TestValidateWorkflow_PromptFileValidation_HappyPath(t *testing.T) {
	tempDir := t.TempDir()

	promptFile := filepath.Join(tempDir, "prompt.md")
	err := os.WriteFile(promptFile, []byte("# Test Prompt\n{{.inputs.name}}"), 0o600)
	require.NoError(t, err)

	repo := newMockRepository()
	repo.workflows["test"] = newTestWorkflowWithPromptFile("test", tempDir, "prompt.md")

	svc := application.NewWorkflowService(
		repo,
		newMockStateStore(),
		newMockExecutor(),
		&mockLogger{},
		newMockExpressionValidator(),
	)

	err = svc.ValidateWorkflow(context.Background(), "test")
	assert.NoError(t, err)
}

func TestValidateWorkflow_PromptFileValidation_MissingFile(t *testing.T) {
	tempDir := t.TempDir()

	repo := newMockRepository()
	repo.workflows["test"] = newTestWorkflowWithPromptFile("test", tempDir, "nonexistent.md")

	svc := application.NewWorkflowService(
		repo,
		newMockStateStore(),
		newMockExecutor(),
		&mockLogger{},
		newMockExpressionValidator(),
	)

	err := svc.ValidateWorkflow(context.Background(), "test")
	require.Error(t, err)

	var structErr *domerrors.StructuredError
	require.ErrorAs(t, err, &structErr)
	assert.Equal(t, domerrors.ErrorCodeUserInputMissingFile, structErr.Code)
	assert.Contains(t, structErr.Message, "prompt_file")
	assert.Contains(t, structErr.Message, "nonexistent.md")
}

func TestValidateWorkflow_PromptFileValidation_UnreadableFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	tempDir := t.TempDir()

	promptFile := filepath.Join(tempDir, "unreadable.md")
	err := os.WriteFile(promptFile, []byte("# Test"), 0o000)
	require.NoError(t, err)
	defer os.Chmod(promptFile, 0o600)

	repo := newMockRepository()
	repo.workflows["test"] = newTestWorkflowWithPromptFile("test", tempDir, "unreadable.md")

	svc := application.NewWorkflowService(
		repo,
		newMockStateStore(),
		newMockExecutor(),
		&mockLogger{},
		newMockExpressionValidator(),
	)

	err = svc.ValidateWorkflow(context.Background(), "test")
	require.Error(t, err)

	var structErr *domerrors.StructuredError
	require.ErrorAs(t, err, &structErr)
	assert.Equal(t, domerrors.ErrorCodeUserInputMissingFile, structErr.Code)
}

func TestValidateWorkflow_PromptFileValidation_AbsolutePath(t *testing.T) {
	tempDir := t.TempDir()

	promptFile := filepath.Join(tempDir, "absolute.md")
	err := os.WriteFile(promptFile, []byte("# Absolute Path Test"), 0o600)
	require.NoError(t, err)

	repo := newMockRepository()
	repo.workflows["test"] = newTestWorkflowWithPromptFile("test", "/some/other/dir", promptFile)

	svc := application.NewWorkflowService(
		repo,
		newMockStateStore(),
		newMockExecutor(),
		&mockLogger{},
		newMockExpressionValidator(),
	)

	err = svc.ValidateWorkflow(context.Background(), "test")
	assert.NoError(t, err)
}

func TestValidateWorkflow_PromptFileValidation_MultipleAgentSteps(t *testing.T) {
	tempDir := t.TempDir()

	prompt1 := filepath.Join(tempDir, "prompt1.md")
	prompt2 := filepath.Join(tempDir, "prompt2.md")
	err := os.WriteFile(prompt1, []byte("# Prompt 1"), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(prompt2, []byte("# Prompt 2"), 0o600)
	require.NoError(t, err)

	repo := newMockRepository()
	repo.workflows["test"] = &workflow.Workflow{
		Name:      "test",
		Initial:   "step1",
		SourceDir: tempDir,
		Steps: map[string]*workflow.Step{
			"step1": {
				Name: "step1",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:   "claude",
					PromptFile: "prompt1.md",
				},
				OnSuccess: "step2",
			},
			"step2": {
				Name: "step2",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:   "gemini",
					PromptFile: "prompt2.md",
				},
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	svc := application.NewWorkflowService(
		repo,
		newMockStateStore(),
		newMockExecutor(),
		&mockLogger{},
		newMockExpressionValidator(),
	)

	err = svc.ValidateWorkflow(context.Background(), "test")
	assert.NoError(t, err)
}

func TestValidateWorkflow_PromptFileValidation_NoAgentSteps(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["test"] = &workflow.Workflow{
		Name:      "test",
		Initial:   "shell_step",
		SourceDir: "/tmp",
		Steps: map[string]*workflow.Step{
			"shell_step": {
				Name:      "shell_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 'hello'",
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	svc := application.NewWorkflowService(
		repo,
		newMockStateStore(),
		newMockExecutor(),
		&mockLogger{},
		newMockExpressionValidator(),
	)

	err := svc.ValidateWorkflow(context.Background(), "test")
	assert.NoError(t, err)
}

func TestValidateWorkflow_PromptFileValidation_EmptyPromptFile(t *testing.T) {
	tempDir := t.TempDir()

	repo := newMockRepository()
	repo.workflows["test"] = &workflow.Workflow{
		Name:      "test",
		Initial:   "agent_step",
		SourceDir: tempDir,
		Steps: map[string]*workflow.Step{
			"agent_step": {
				Name: "agent_step",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:   "claude",
					Prompt:     "inline prompt",
					PromptFile: "",
				},
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	svc := application.NewWorkflowService(
		repo,
		newMockStateStore(),
		newMockExecutor(),
		&mockLogger{},
		newMockExpressionValidator(),
	)

	err := svc.ValidateWorkflow(context.Background(), "test")
	assert.NoError(t, err)
}

func TestValidateWorkflow_PromptFileValidation_DirectoryInsteadOfFile(t *testing.T) {
	tempDir := t.TempDir()

	subDir := filepath.Join(tempDir, "subdir")
	err := os.Mkdir(subDir, 0o755)
	require.NoError(t, err)

	repo := newMockRepository()
	repo.workflows["test"] = newTestWorkflowWithPromptFile("test", tempDir, "subdir")

	svc := application.NewWorkflowService(
		repo,
		newMockStateStore(),
		newMockExecutor(),
		&mockLogger{},
		newMockExpressionValidator(),
	)

	err = svc.ValidateWorkflow(context.Background(), "test")
	require.Error(t, err)

	var structErr *domerrors.StructuredError
	require.ErrorAs(t, err, &structErr)
	assert.Equal(t, domerrors.ErrorCodeUserInputMissingFile, structErr.Code)
}

func TestValidateWorkflow_PromptFileValidation_NestedPath(t *testing.T) {
	tempDir := t.TempDir()

	nestedDir := filepath.Join(tempDir, "prompts", "analyze")
	err := os.MkdirAll(nestedDir, 0o755)
	require.NoError(t, err)

	promptFile := filepath.Join(nestedDir, "research.md")
	err = os.WriteFile(promptFile, []byte("# Research Prompt"), 0o600)
	require.NoError(t, err)

	repo := newMockRepository()
	repo.workflows["test"] = newTestWorkflowWithPromptFile("test", tempDir, "prompts/analyze/research.md")

	svc := application.NewWorkflowService(
		repo,
		newMockStateStore(),
		newMockExecutor(),
		&mockLogger{},
		newMockExpressionValidator(),
	)

	err = svc.ValidateWorkflow(context.Background(), "test")
	assert.NoError(t, err)
}

func TestValidateWorkflow_PromptFileValidation_ErrorIncludesDetails(t *testing.T) {
	tempDir := t.TempDir()

	similarFile := filepath.Join(tempDir, "prompt_template.md")
	err := os.WriteFile(similarFile, []byte("# Similar"), 0o600)
	require.NoError(t, err)

	repo := newMockRepository()
	repo.workflows["test"] = newTestWorkflowWithPromptFile("test", tempDir, "prompt_tempalte.md")

	svc := application.NewWorkflowService(
		repo,
		newMockStateStore(),
		newMockExecutor(),
		&mockLogger{},
		newMockExpressionValidator(),
	)

	err = svc.ValidateWorkflow(context.Background(), "test")
	require.Error(t, err)

	var structErr *domerrors.StructuredError
	require.ErrorAs(t, err, &structErr)

	assert.NotNil(t, structErr.Details)
	pathDetail, hasPath := structErr.Details["path"]
	assert.True(t, hasPath, "error details should include path field for FileNotFoundHintGenerator")
	assert.NotEmpty(t, pathDetail)
}

func TestValidateWorkflow_PromptFileValidation_TemplateExpressionSkipped(t *testing.T) {
	// Paths containing template expressions (e.g. {{.awf.prompts_dir}}) cannot be
	// validated statically — they are resolved at runtime via interpolation.
	repo := newMockRepository()
	repo.workflows["test"] = newTestWorkflowWithPromptFile(
		"test", "/nonexistent/source", "{{.awf.prompts_dir}}/commit/commit-message.md",
	)

	svc := application.NewWorkflowService(
		repo,
		newMockStateStore(),
		newMockExecutor(),
		&mockLogger{},
		newMockExpressionValidator(),
	)

	err := svc.ValidateWorkflow(context.Background(), "test")
	assert.NoError(t, err, "paths with template expressions should be skipped during static validation")
}

func TestValidateWorkflow_PromptFileValidation_MultipleFilesFirstFailureReported(t *testing.T) {
	tempDir := t.TempDir()

	prompt2 := filepath.Join(tempDir, "prompt2.md")
	err := os.WriteFile(prompt2, []byte("# Valid"), 0o600)
	require.NoError(t, err)

	repo := newMockRepository()
	repo.workflows["test"] = &workflow.Workflow{
		Name:      "test",
		Initial:   "step1",
		SourceDir: tempDir,
		Steps: map[string]*workflow.Step{
			"step1": {
				Name: "step1",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:   "claude",
					PromptFile: "missing1.md",
				},
				OnSuccess: "step2",
			},
			"step2": {
				Name: "step2",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:   "gemini",
					PromptFile: "prompt2.md",
				},
				OnSuccess: "step3",
			},
			"step3": {
				Name: "step3",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:   "claude",
					PromptFile: "missing3.md",
				},
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	svc := application.NewWorkflowService(
		repo,
		newMockStateStore(),
		newMockExecutor(),
		&mockLogger{},
		newMockExpressionValidator(),
	)

	err = svc.ValidateWorkflow(context.Background(), "test")
	require.Error(t, err)

	errMsg := err.Error()
	assert.True(t,
		strings.Contains(errMsg, "missing1.md") || strings.Contains(errMsg, "missing3.md"),
		"error should mention at least one missing file",
	)
}
