package repository

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// C069: Plugin Extensibility - Config Mapping Tests

func TestMapStep_Config_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		yamlStep     yamlStep
		expectedLen  int
		expectations map[string]any
	}{
		{
			name: "basic config mapping",
			yamlStep: yamlStep{
				Type:    "command",
				Command: "echo hello",
				Config: map[string]any{
					"timeout": 30,
					"enabled": true,
				},
			},
			expectedLen: 2,
			expectations: map[string]any{
				"timeout": 30,
				"enabled": true,
			},
		},
		{
			name: "config with nested objects",
			yamlStep: yamlStep{
				Type:    "agent",
				Command: "test",
				Config: map[string]any{
					"database": map[string]any{
						"host": "localhost",
						"port": 5432,
					},
					"cache": map[string]any{
						"ttl": 3600,
					},
				},
			},
			expectedLen: 2,
			expectations: map[string]any{
				"database": map[string]any{
					"host": "localhost",
					"port": 5432,
				},
				"cache": map[string]any{
					"ttl": 3600,
				},
			},
		},
		{
			name: "config with arrays",
			yamlStep: yamlStep{
				Type:    "command",
				Command: "test",
				Config: map[string]any{
					"tags": []any{"tag1", "tag2", "tag3"},
					"servers": []any{
						map[string]any{"host": "server1"},
						map[string]any{"host": "server2"},
					},
				},
			},
			expectedLen: 2,
		},
		{
			name: "config with heterogeneous types",
			yamlStep: yamlStep{
				Type:    "command",
				Command: "test",
				Config: map[string]any{
					"str_val":   "hello",
					"int_val":   42,
					"float_val": 3.14,
					"bool_val":  false,
					"nil_val":   nil,
				},
			},
			expectedLen: 5,
			expectations: map[string]any{
				"str_val":   "hello",
				"int_val":   42,
				"float_val": 3.14,
				"bool_val":  false,
				"nil_val":   nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step, err := mapStep("test.yaml", "test-step", &tt.yamlStep)

			require.NoError(t, err)
			require.NotNil(t, step)
			assert.Equal(t, tt.expectedLen, len(step.Config))

			for key, expectedVal := range tt.expectations {
				assert.Equal(t, expectedVal, step.Config[key])
			}
		})
	}
}

func TestMapStep_Config_EmptyConfig(t *testing.T) {
	yamlStep := &yamlStep{
		Type:    "command",
		Command: "echo test",
		Config:  make(map[string]any),
	}

	step, err := mapStep("test.yaml", "test-step", yamlStep)

	require.NoError(t, err)
	require.NotNil(t, step.Config)
	assert.Len(t, step.Config, 0)
}

func TestMapStep_Config_NilConfig(t *testing.T) {
	yamlStep := &yamlStep{
		Type:    "command",
		Command: "echo test",
		Config:  nil,
	}

	step, err := mapStep("test.yaml", "test-step", yamlStep)

	require.NoError(t, err)
	assert.Nil(t, step.Config)
}

func TestMapStep_Config_OperationStep(t *testing.T) {
	yamlStep := &yamlStep{
		Type:      "operation",
		Operation: "github.create_issue",
		Config: map[string]any{
			"repository": "awf-project/cli",
			"title":      "Test Issue",
			"body":       "Issue body",
		},
	}

	step, err := mapStep("test.yaml", "create-issue", yamlStep)

	require.NoError(t, err)
	assert.Equal(t, workflow.StepTypeOperation, step.Type)
	assert.Len(t, step.Config, 3)
	assert.Equal(t, "awf-project/cli", step.Config["repository"])
}

func TestMapStep_Config_AgentStep(t *testing.T) {
	yamlStep := &yamlStep{
		Type:     "agent",
		Provider: "claude",
		Prompt:   "test prompt",
		Config: map[string]any{
			"model":       "claude-3-5-sonnet",
			"temperature": 0.7,
		},
	}

	step, err := mapStep("test.yaml", "agent-step", yamlStep)

	require.NoError(t, err)
	assert.Equal(t, workflow.StepTypeAgent, step.Type)
	assert.NotNil(t, step.Config)
	assert.Equal(t, "claude-3-5-sonnet", step.Config["model"])
	assert.Equal(t, 0.7, step.Config["temperature"])
}

func TestMapStep_Config_ParallelStep(t *testing.T) {
	yamlStep := &yamlStep{
		Type:     "parallel",
		Parallel: []string{"step1", "step2"},
		Config: map[string]any{
			"max_concurrent": 5,
			"timeout":        300,
		},
	}

	step, err := mapStep("test.yaml", "parallel-step", yamlStep)

	require.NoError(t, err)
	assert.Equal(t, workflow.StepTypeParallel, step.Type)
	assert.Equal(t, 5, step.Config["max_concurrent"])
	assert.Equal(t, 300, step.Config["timeout"])
}

func TestMapStep_Config_LoopStep(t *testing.T) {
	yamlStep := &yamlStep{
		Type:  "for_each",
		Items: "{{inputs.items}}",
		Body:  []string{"inner-step"},
		Config: map[string]any{
			"batch_size":     10,
			"parallel_items": true,
		},
	}

	step, err := mapStep("test.yaml", "loop-step", yamlStep)

	require.NoError(t, err)
	assert.Equal(t, workflow.StepTypeForEach, step.Type)
	assert.Equal(t, 10, step.Config["batch_size"])
	assert.Equal(t, true, step.Config["parallel_items"])
}

func TestMapStep_Config_ComplexNesting(t *testing.T) {
	yamlStep := &yamlStep{
		Type:    "command",
		Command: "test",
		Config: map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"level3": map[string]any{
						"value": "deep",
						"count": 42,
					},
				},
			},
		},
	}

	step, err := mapStep("test.yaml", "nested-step", yamlStep)

	require.NoError(t, err)
	l1 := step.Config["level1"].(map[string]any)
	l2 := l1["level2"].(map[string]any)
	l3 := l2["level3"].(map[string]any)
	assert.Equal(t, "deep", l3["value"])
	assert.Equal(t, 42, l3["count"])
}

func TestMapStep_Config_PreservesAllDataTypes(t *testing.T) {
	config := map[string]any{
		"string": "value",
		"int":    42,
		"float":  3.14,
		"bool":   true,
		"nil":    nil,
		"list":   []any{1, 2, 3},
		"dict":   map[string]any{"key": "value"},
	}

	yamlStep := &yamlStep{
		Type:    "command",
		Command: "test",
		Config:  config,
	}

	step, err := mapStep("test.yaml", "preserve-types", yamlStep)

	require.NoError(t, err)

	assert.Equal(t, "value", step.Config["string"])
	assert.Equal(t, 42, step.Config["int"])
	assert.Equal(t, 3.14, step.Config["float"])
	assert.Equal(t, true, step.Config["bool"])
	assert.Nil(t, step.Config["nil"])
	assert.Equal(t, []any{1, 2, 3}, step.Config["list"])
	assert.Equal(t, map[string]any{"key": "value"}, step.Config["dict"])
}

func TestMapStep_Config_CommandStepWithValidationConfig(t *testing.T) {
	yamlStep := &yamlStep{
		Type:    "command",
		Command: "validate",
		Config: map[string]any{
			"schema": "json_schema",
			"strict": true,
			"rules": []any{
				map[string]any{"field": "name", "type": "string"},
				map[string]any{"field": "age", "type": "number"},
			},
		},
	}

	step, err := mapStep("test.yaml", "validator", yamlStep)

	require.NoError(t, err)
	assert.NotNil(t, step.Config)
	assert.Equal(t, "json_schema", step.Config["schema"])
	assert.Equal(t, true, step.Config["strict"])
	rules := step.Config["rules"].([]any)
	assert.Len(t, rules, 2)
}
