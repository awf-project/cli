package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHookAction_Empty(t *testing.T) {
	action := HookAction{}
	assert.Empty(t, action.Log)
	assert.Empty(t, action.Command)
}

func TestHookAction_Log(t *testing.T) {
	action := HookAction{Log: "Starting workflow"}
	assert.Equal(t, "Starting workflow", action.Log)
	assert.Empty(t, action.Command)
}

func TestHookAction_Command(t *testing.T) {
	action := HookAction{Command: "echo hello"}
	assert.Empty(t, action.Log)
	assert.Equal(t, "echo hello", action.Command)
}

func TestHook_Empty(t *testing.T) {
	hook := Hook{}
	assert.Len(t, hook, 0)
}

func TestHook_WithActions(t *testing.T) {
	hook := Hook{
		{Log: "Starting"},
		{Command: "echo hello"},
	}
	assert.Len(t, hook, 2)
	assert.Equal(t, "Starting", hook[0].Log)
	assert.Equal(t, "echo hello", hook[1].Command)
}

func TestWorkflowHooks_Empty(t *testing.T) {
	hooks := WorkflowHooks{}
	assert.Nil(t, hooks.WorkflowStart)
	assert.Nil(t, hooks.WorkflowEnd)
	assert.Nil(t, hooks.WorkflowError)
	assert.Nil(t, hooks.WorkflowCancel)
}

func TestWorkflowHooks_WithHooks(t *testing.T) {
	hooks := WorkflowHooks{
		WorkflowStart: Hook{{Log: "Starting"}},
		WorkflowEnd:   Hook{{Log: "Completed"}},
		WorkflowError: Hook{{Log: "Error: {{error.message}}"}},
	}
	assert.Len(t, hooks.WorkflowStart, 1)
	assert.Len(t, hooks.WorkflowEnd, 1)
	assert.Len(t, hooks.WorkflowError, 1)
	assert.Nil(t, hooks.WorkflowCancel)
}

func TestStepHooks_Empty(t *testing.T) {
	hooks := StepHooks{}
	assert.Nil(t, hooks.Pre)
	assert.Nil(t, hooks.Post)
}

func TestStepHooks_WithHooks(t *testing.T) {
	hooks := StepHooks{
		Pre:  Hook{{Log: "Before step"}},
		Post: Hook{{Log: "After step"}},
	}
	assert.Len(t, hooks.Pre, 1)
	assert.Len(t, hooks.Post, 1)
}
