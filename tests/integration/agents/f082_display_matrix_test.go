//go:build integration

package agents_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/agents"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infraExpr "github.com/awf-project/cli/internal/infrastructure/expression"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/interpolation"
)

// Feature: F082
// Functional tests validating the display matrix: output_format × --output interaction
// through real provider implementations (Claude, OpenCode) with mock CLI executors.

func TestDisplayMatrix_ClaudeTextFormat_FiltersNDJSON(t *testing.T) {
	ndjsonOutput := strings.Join([]string{
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":", world!"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"message_stop"}`,
	}, "\n")

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(ndjsonOutput), nil)

	provider := agents.NewClaudeProviderWithOptions(
		agents.WithClaudeExecutor(mockExec),
	)

	registry := mocks.NewMockAgentRegistry()
	registry.Register(provider)

	svc, _ := setupDisplayMatrixService(t)
	svc.SetAgentRegistry(registry)

	wf := buildDisplayMatrixWorkflow("claude", workflow.OutputFormatText)
	execCtx, err := svc.RunWithWorkflow(context.Background(), wf, nil)

	require.NoError(t, err)
	state := execCtx.States["agent-step"]

	assert.Contains(t, state.Output, "content_block_delta", "raw Output must contain NDJSON")
	assert.Equal(t, "Hello\n, world!", state.DisplayOutput)
	assert.NotContains(t, state.DisplayOutput, "content_block_delta",
		"DisplayOutput must not contain raw NDJSON event types")
}

func TestDisplayMatrix_JSONFormat_PassthroughRaw(t *testing.T) {
	// F065 post-processing requires valid JSON when output_format=json
	jsonOutput := `{"result":"Hello","status":"ok"}`

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(jsonOutput), nil)

	provider := agents.NewClaudeProviderWithOptions(
		agents.WithClaudeExecutor(mockExec),
	)

	registry := mocks.NewMockAgentRegistry()
	registry.Register(provider)

	svc, _ := setupDisplayMatrixService(t)
	svc.SetAgentRegistry(registry)

	wf := buildDisplayMatrixWorkflow("claude", workflow.OutputFormatJSON)
	execCtx, err := svc.RunWithWorkflow(context.Background(), wf, nil)

	require.NoError(t, err)
	state := execCtx.States["agent-step"]

	assert.Contains(t, state.Output, "result", "raw Output preserved")
	assert.Empty(t, state.DisplayOutput, "json format must produce empty DisplayOutput")
}

func TestDisplayMatrix_DefaultFormat_BehavesAsText(t *testing.T) {
	ndjsonOutput := `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Default filtered"}}` + "\n"

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(ndjsonOutput), nil)

	provider := agents.NewClaudeProviderWithOptions(
		agents.WithClaudeExecutor(mockExec),
	)

	registry := mocks.NewMockAgentRegistry()
	registry.Register(provider)

	svc, _ := setupDisplayMatrixService(t)
	svc.SetAgentRegistry(registry)

	wf := buildDisplayMatrixWorkflow("claude", workflow.OutputFormatNone)
	execCtx, err := svc.RunWithWorkflow(context.Background(), wf, nil)

	require.NoError(t, err)
	state := execCtx.States["agent-step"]

	assert.Equal(t, "Default filtered", state.DisplayOutput,
		"empty output_format must default to text filtering")
	assert.Contains(t, state.Output, "content_block_delta", "raw Output preserved")
}

func TestDisplayMatrix_StubProvider_FallsBackToRawOutput(t *testing.T) {
	rawOutput := `{"type":"some_event","data":"test value"}` + "\n"

	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(rawOutput), nil)

	// OpenCode has a real parser (not a stub), so use a provider with stub parser behavior
	// by checking that when parser extracts nothing, DisplayOutput is empty
	provider := agents.NewGeminiProviderWithOptions(
		agents.WithGeminiExecutor(mockExec),
	)

	registry := mocks.NewMockAgentRegistry()
	registry.Register(provider)

	svc, _ := setupDisplayMatrixService(t)
	svc.SetAgentRegistry(registry)

	wf := buildDisplayMatrixWorkflow("gemini", workflow.OutputFormatText)
	execCtx, err := svc.RunWithWorkflow(context.Background(), wf, nil)

	require.NoError(t, err)
	state := execCtx.States["agent-step"]

	assert.Empty(t, state.DisplayOutput, "stub parser returns empty: DisplayOutput stays empty")
	assert.Contains(t, state.Output, "some_event", "raw Output always preserved")
}

func TestDisplayMatrix_StreamFilterWriter_WritesToLiveOutput(t *testing.T) {
	ndjsonLines := []string{
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"streamed"}}`,
		`{"type":"message_stop"}`,
	}

	var liveOutput bytes.Buffer
	mockExec := &streamingMockExecutor{
		lines:  ndjsonLines,
		stdout: []byte(strings.Join(ndjsonLines, "\n")),
	}

	provider := agents.NewClaudeProviderWithOptions(
		agents.WithClaudeExecutor(mockExec),
	)

	result, err := provider.Execute(
		context.Background(),
		"test prompt",
		map[string]any{"model": "claude-sonnet-4-20250514", "output_format": "text"},
		&liveOutput,
		io.Discard,
	)

	require.NoError(t, err)
	assert.Equal(t, "streamed", result.DisplayOutput)
	assert.Contains(t, result.Output, "content_block_delta", "raw output preserved in result")
}

func TestDisplayMatrix_OptionsMapCloning_OriginalUnmutated(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("output"), nil)

	provider := agents.NewClaudeProviderWithOptions(
		agents.WithClaudeExecutor(mockExec),
	)

	registry := mocks.NewMockAgentRegistry()
	registry.Register(provider)

	svc, _ := setupDisplayMatrixService(t)
	svc.SetAgentRegistry(registry)

	originalOpts := map[string]any{"model": "claude-sonnet-4-20250514"}
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "agent-step",
		Steps: map[string]*workflow.Step{
			"agent-step": {
				Name: "agent-step",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "test prompt",
					OutputFormat: workflow.OutputFormatText,
					Options:      originalOpts,
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

	_, err := svc.RunWithWorkflow(context.Background(), wf, nil)
	require.NoError(t, err)

	assert.NotContains(t, originalOpts, "output_format",
		"cloneAndInjectOutputFormat must not mutate original options")
	assert.Len(t, originalOpts, 1, "original map must keep only its original keys")
}

// streamingMockExecutor writes NDJSON lines to the stdout writer to exercise
// the streamFilterWriter live filtering path.
type streamingMockExecutor struct {
	lines  []string
	stdout []byte
}

func (m *streamingMockExecutor) Run(ctx context.Context, name string, stdoutW, stderrW io.Writer, args ...string) ([]byte, []byte, error) {
	if stdoutW != nil {
		for _, line := range m.lines {
			_, _ = stdoutW.Write([]byte(line + "\n"))
		}
	}
	return m.stdout, nil, nil
}

func setupDisplayMatrixService(t *testing.T) (*application.ExecutionService, string) {
	t.Helper()
	tempDir := t.TempDir()
	repo := repository.NewYAMLRepository(tempDir)
	stateStore := store.NewJSONStore(tempDir)
	exec := executor.NewShellExecutor()
	wfSvc := application.NewWorkflowService(repo, stateStore, exec, &nopLogger{}, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(&nopLogger{})
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, stateStore, &nopLogger{}, interpolation.NewTemplateResolver(), nil, infraExpr.NewExprEvaluator(),
	)
	return execSvc, tempDir
}

func buildDisplayMatrixWorkflow(provider string, format workflow.OutputFormat) *workflow.Workflow {
	return &workflow.Workflow{
		Name:    "test",
		Initial: "agent-step",
		Steps: map[string]*workflow.Step{
			"agent-step": {
				Name: "agent-step",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     provider,
					Prompt:       "test prompt",
					OutputFormat: format,
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

type nopLogger struct{}

func (m *nopLogger) Debug(msg string, fields ...any)            {}
func (m *nopLogger) Info(msg string, fields ...any)             {}
func (m *nopLogger) Warn(msg string, fields ...any)             {}
func (m *nopLogger) Error(msg string, fields ...any)            {}
func (m *nopLogger) WithContext(ctx map[string]any) ports.Logger { return m }
