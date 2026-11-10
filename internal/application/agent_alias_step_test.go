package application_test

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/agents"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
)

func TestExecuteAgentStep_ProviderNamesAnAlias(t *testing.T) {
	var gotOptions map[string]any
	provider := testmocks.NewMockAgentProvider("test-provider")
	provider.SetExecuteFunc(func(_ context.Context, _ string, options map[string]any, _, _ io.Writer) (*workflow.AgentResult, error) {
		gotOptions = options
		return &workflow.AgentResult{Output: "ok"}, nil
	})
	registry := agents.NewAgentRegistry()
	require.NoError(t, registry.Register(provider))
	registry.SetAliases(map[string]agents.Alias{
		"reviewer": {Provider: "test-provider", Options: map[string]any{"model": "large", "effort": "high"}},
	})

	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeAgent,
				Agent:     &workflow.AgentConfig{Provider: "reviewer", Prompt: "p", Options: map[string]any{"model": "small"}},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}
	svc, _ := NewTestHarness(t).WithWorkflow("test", wf).Build()
	svc.SetAgentRegistry(registry)

	_, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "test-run")

	require.NoError(t, err)
	assert.Equal(t, "small", gotOptions["model"], "the step option wins over the alias option")
	assert.Equal(t, "high", gotOptions["effort"], "the alias option reaches the provider")
	assert.Equal(t, map[string]any{"model": "small"}, wf.Steps["step1"].Agent.Options, "the step options are not modified")
}

// runAliasStep runs one agent step whose provider names the alias "reviewer" and returns the options the provider
// received and the logger of the run.
func runAliasStep(t *testing.T, alias, step, inputs map[string]any) (map[string]any, *testmocks.MockLogger) {
	t.Helper()
	var got map[string]any
	provider := testmocks.NewMockAgentProvider("test-provider")
	provider.SetExecuteFunc(func(_ context.Context, _ string, options map[string]any, _, _ io.Writer) (*workflow.AgentResult, error) {
		got = options
		return &workflow.AgentResult{Output: "ok"}, nil
	})
	registry := agents.NewAgentRegistry()
	require.NoError(t, registry.Register(provider))
	registry.SetAliases(map[string]agents.Alias{"reviewer": {Provider: "test-provider", Options: alias}})

	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeAgent,
				Agent:     &workflow.AgentConfig{Provider: "reviewer", Prompt: "p", Options: step},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}
	svc, mocks := NewTestHarness(t).WithWorkflow("test", wf).Build()
	svc.SetAgentRegistry(registry)

	_, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, inputs, "test-run")
	require.NoError(t, err)
	return got, mocks.Logger
}

func TestExecuteAgentStep_AliasOptionsAreInterpolated(t *testing.T) {
	got, _ := runAliasStep(t,
		map[string]any{
			"model":         "{{.inputs.size}}",
			"enabled_tools": []any{"read", "{{.inputs.tool}}"},
			"nested":        map[string]any{"key": "{{.inputs.size}}"},
		},
		nil,
		map[string]any{"size": "large", "tool": "grep"},
	)

	assert.Equal(t, "large", got["model"])
	assert.Equal(t, []any{"read", "grep"}, got["enabled_tools"])
	assert.Equal(t, map[string]any{"key": "large"}, got["nested"])
}

func TestExecuteAgentStep_AliasKeepsTheTypesOfDocumentedOptions(t *testing.T) {
	got, _ := runAliasStep(t,
		map[string]any{"max_turns": 4, "max_price": 1.5, "trust": true, "add_dirs": []any{"docs", "src"}, "agent_profile": "plan"},
		map[string]any{"max_turns": 8},
		nil,
	)

	assert.Equal(t, 8, got["max_turns"], "the step wins")
	assert.Equal(t, 1.5, got["max_price"])
	assert.Equal(t, true, got["trust"])
	assert.Equal(t, []any{"docs", "src"}, got["add_dirs"])
	assert.Equal(t, "plan", got["agent_profile"])
}

func TestExecuteAgentStep_AliasSkipPermissionsStillWarns(t *testing.T) {
	tests := []struct {
		name       string
		alias      map[string]any
		step       map[string]any
		expectWarn bool
	}{
		{"enabled by the alias", map[string]any{"dangerously_skip_permissions": true}, nil, true},
		{"enabled by the step", nil, map[string]any{"dangerously_skip_permissions": true}, true},
		{"alias enables, step disables", map[string]any{"dangerously_skip_permissions": true}, map[string]any{"dangerously_skip_permissions": false}, false},
		{"never enabled", map[string]any{"model": "large"}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, logger := runAliasStep(t, tt.alias, tt.step, nil)

			var warned *testmocks.LogMessage
			for _, m := range logger.GetMessagesByLevel("WARN") {
				if m.Msg == "dangerously_skip_permissions enabled" {
					warned = &m
				}
			}
			if !tt.expectWarn {
				assert.Nil(t, warned, "no warning expected")
				return
			}
			require.NotNil(t, warned, "the audit warning must be emitted")
			assert.Contains(t, warned.Fields, "provider")
			assert.Contains(t, warned.Fields, "test-provider")
			assert.Contains(t, warned.Fields, "reviewer", "the warning names the alias")
		})
	}
}
