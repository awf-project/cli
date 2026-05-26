package workflow

import (
	"errors"
	"testing"

	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStep_Validate_MCPProxyNil(t *testing.T) {
	step := &Step{
		Name:     "test_step",
		Type:     StepTypeCommand,
		Command:  "echo hello",
		MCPProxy: nil,
	}

	err := step.Validate(nil, nil)

	assert.NoError(t, err, "should validate successfully when MCPProxy is nil")
}

func TestStep_Validate_MCPProxyValidConfig(t *testing.T) {
	step := &Step{
		Name: "agent_with_proxy",
		Type: StepTypeAgent,
		Agent: &AgentConfig{
			Provider: "claude",
			Prompt:   "test prompt",
		},
		MCPProxy: &MCPProxyConfig{
			Enable:            true,
			InterceptBuiltins: true,
			PluginTools: []PluginToolExpose{
				{
					Plugin: "k8s",
					Expose: []string{"apply"},
				},
			},
		},
	}

	err := step.Validate(nil, nil)

	assert.NoError(t, err, "should validate successfully with valid MCPProxy config")
}

func TestStep_Validate_MCPProxyEmptyProxyError(t *testing.T) {
	step := &Step{
		Name: "agent_with_bad_proxy",
		Type: StepTypeAgent,
		Agent: &AgentConfig{
			Provider: "claude",
			Prompt:   "test prompt",
		},
		MCPProxy: &MCPProxyConfig{
			Enable:            true,
			InterceptBuiltins: false,
			PluginTools:       []PluginToolExpose{},
		},
	}

	err := step.Validate(nil, nil)

	require.Error(t, err, "should return error for empty proxy")
	var structErr *domerrors.StructuredError
	require.True(t, errors.As(err, &structErr), "error must be a *domerrors.StructuredError")
	assert.Equal(t, domerrors.ErrorCodeUserMCPProxyEmptyProxy, structErr.Code)
}

func TestStep_Validate_MCPProxyNameCollisionError(t *testing.T) {
	step := &Step{
		Name: "agent_with_collision",
		Type: StepTypeAgent,
		Agent: &AgentConfig{
			Provider: "claude",
			Prompt:   "test prompt",
		},
		MCPProxy: &MCPProxyConfig{
			Enable:            true,
			InterceptBuiltins: true,
			PluginTools: []PluginToolExpose{
				{
					Plugin: "k8s",
					Expose: []string{"apply"},
				},
				{
					Plugin: "k8s",
					Expose: []string{"get"},
				},
			},
		},
	}

	err := step.Validate(nil, nil)

	require.Error(t, err, "should return error for duplicate plugin")
	var structErr *domerrors.StructuredError
	require.True(t, errors.As(err, &structErr), "error must be a *domerrors.StructuredError")
	assert.Equal(t, domerrors.ErrorCodeUserMCPProxyNameCollision, structErr.Code)
}

func TestStep_Validate_CommandStepWithMCPProxy(t *testing.T) {
	step := &Step{
		Name:    "command_step",
		Type:    StepTypeCommand,
		Command: "echo hello",
		MCPProxy: &MCPProxyConfig{
			Enable:            true,
			InterceptBuiltins: true,
			PluginTools:       []PluginToolExpose{},
		},
	}

	err := step.Validate(nil, nil)

	assert.NoError(t, err, "should validate command step with valid MCPProxy")
}

func TestStep_Validate_MCPProxyDisabledWithBadConfig(t *testing.T) {
	step := &Step{
		Name: "agent_step",
		Type: StepTypeAgent,
		Agent: &AgentConfig{
			Provider: "claude",
			Prompt:   "test prompt",
		},
		MCPProxy: &MCPProxyConfig{
			Enable:            false,
			InterceptBuiltins: false,
			PluginTools:       []PluginToolExpose{},
		},
	}

	err := step.Validate(nil, nil)

	assert.NoError(t, err, "should validate when MCPProxy is disabled, even with other bad fields")
}

func TestStep_Validate_MCPProxyWithMultiplePlugins(t *testing.T) {
	step := &Step{
		Name: "agent_with_multiple",
		Type: StepTypeAgent,
		Agent: &AgentConfig{
			Provider: "claude",
			Prompt:   "test prompt",
		},
		MCPProxy: &MCPProxyConfig{
			Enable:            true,
			InterceptBuiltins: true,
			PluginTools: []PluginToolExpose{
				{
					Plugin: "k8s",
					Expose: []string{"apply", "get"},
				},
				{
					Plugin: "docker",
					Expose: []string{"run", "stop"},
				},
				{
					Plugin: "git",
					Expose: []string{"clone", "pull"},
				},
			},
		},
	}

	err := step.Validate(nil, nil)

	assert.NoError(t, err, "should validate with multiple unique plugins")
}

// TestStep_Validate_MCPProxyError_IsStructuredError is a regression test for
// bug #2/#3: Step.Validate must return a *domerrors.StructuredError (not a raw
// ValidationError or a WORKFLOW.PARSE.YAML_SYNTAX error) so the load pipeline
// can propagate the original USER.MCP_PROXY.* code to the formatter.
func TestStep_Validate_MCPProxyError_IsStructuredError(t *testing.T) {
	step := &Step{
		Name: "agent_with_empty_proxy",
		Type: StepTypeAgent,
		Agent: &AgentConfig{
			Provider: "claude",
			Prompt:   "test prompt",
		},
		MCPProxy: &MCPProxyConfig{
			Enable:            true,
			InterceptBuiltins: false,
			PluginTools:       []PluginToolExpose{},
		},
	}

	err := step.Validate(nil, nil)
	require.Error(t, err, "should return error for empty proxy")

	// The error must be (or wrap) a *domerrors.StructuredError with the exact
	// USER.MCP_PROXY.EMPTY_PROXY code so the infrastructure load pipeline can
	// detect and propagate it without converting it to WORKFLOW.PARSE.YAML_SYNTAX.
	var structErr *domerrors.StructuredError
	require.True(t, errors.As(err, &structErr),
		"error must be or wrap *domerrors.StructuredError; got %T: %v", err, err)
	assert.Equal(t, domerrors.ErrorCodeUserMCPProxyEmptyProxy, structErr.Code,
		"StructuredError code must be USER.MCP_PROXY.EMPTY_PROXY")
}

// TestStep_Validate_MCPProxyMultipleErrors_BothVisibleViaJoin is a regression
// test for bug #4: when MCPProxyConfig.Validate returns multiple errors, ALL of
// them must be reachable in the joined error returned by Step.Validate, not just
// the first one.
//
// Two simultaneous conditions: NAME_COLLISION (duplicate plugin in plugin_tools)
// is the only case that can fire alongside itself (two duplicates at once). We
// use a config where EMPTY_PROXY fires for a second step at the Workflow level
// (tested separately in TestWorkflow_Validate_MCPProxyErrors_AllStepsChecked).
func TestStep_Validate_MCPProxyNameCollision_IsStructuredError(t *testing.T) {
	step := &Step{
		Name: "agent_with_collision",
		Type: StepTypeAgent,
		Agent: &AgentConfig{
			Provider: "claude",
			Prompt:   "test prompt",
		},
		MCPProxy: &MCPProxyConfig{
			Enable:            true,
			InterceptBuiltins: true,
			PluginTools: []PluginToolExpose{
				{Plugin: "k8s", Expose: []string{"apply"}},
				{Plugin: "k8s", Expose: []string{"get"}},
			},
		},
	}

	err := step.Validate(nil, nil)
	require.Error(t, err, "should return error for duplicate plugin")

	var structErr *domerrors.StructuredError
	require.True(t, errors.As(err, &structErr),
		"error must be or wrap *domerrors.StructuredError; got %T: %v", err, err)
	assert.Equal(t, domerrors.ErrorCodeUserMCPProxyNameCollision, structErr.Code,
		"StructuredError code must be USER.MCP_PROXY.NAME_COLLISION")
}
