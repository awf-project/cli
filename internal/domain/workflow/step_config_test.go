package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// C069: Plugin Extensibility - Step Config Field Tests

func TestStep_Config_DefaultEmpty(t *testing.T) {
	step := &Step{
		Name: "test-step",
		Type: StepTypeCommand,
	}

	assert.Nil(t, step.Config)
}

func TestStep_Config_CanStoreValues(t *testing.T) {
	config := map[string]any{
		"timeout":   30,
		"retries":   3,
		"enabled":   true,
		"threshold": 0.95,
		"tags":      []string{"a", "b"},
		"nested":    map[string]any{"key": "value"},
	}

	step := &Step{
		Name:   "test-step",
		Type:   StepTypeCommand,
		Config: config,
	}

	assert.Equal(t, config, step.Config)
	assert.Equal(t, 30, step.Config["timeout"])
	assert.Equal(t, 3, step.Config["retries"])
	assert.Equal(t, true, step.Config["enabled"])
	assert.Equal(t, 0.95, step.Config["threshold"])
}

func TestStep_Config_EmptyMap(t *testing.T) {
	config := make(map[string]any)
	step := &Step{
		Name:   "test-step",
		Type:   StepTypeCommand,
		Config: config,
	}

	assert.NotNil(t, step.Config)
	assert.Len(t, step.Config, 0)
}

func TestStep_Config_HeterogeneousTypes(t *testing.T) {
	config := map[string]any{
		"str_val":   "hello",
		"int_val":   42,
		"float_val": 3.14,
		"bool_val":  false,
		"nil_val":   nil,
	}

	step := &Step{
		Name:   "test-step",
		Type:   StepTypeCommand,
		Config: config,
	}

	assert.Equal(t, "hello", step.Config["str_val"])
	assert.Equal(t, 42, step.Config["int_val"])
	assert.Equal(t, 3.14, step.Config["float_val"])
	assert.Equal(t, false, step.Config["bool_val"])
	assert.Nil(t, step.Config["nil_val"])
}

func TestStep_Config_DifferentStepTypes(t *testing.T) {
	tests := []struct {
		name     string
		stepType StepType
		config   map[string]any
	}{
		{
			name:     "command step with config",
			stepType: StepTypeCommand,
			config:   map[string]any{"key": "value"},
		},
		{
			name:     "agent step with config",
			stepType: StepTypeAgent,
			config:   map[string]any{"api_key": "secret"},
		},
		{
			name:     "operation step with config",
			stepType: StepTypeOperation,
			config:   map[string]any{"endpoint": "/v1/api"},
		},
		{
			name:     "parallel step with config",
			stepType: StepTypeParallel,
			config:   map[string]any{"concurrent": 5},
		},
		{
			name:     "for_each step with config",
			stepType: StepTypeForEach,
			config:   map[string]any{"batch_size": 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &Step{
				Name:   "test-step",
				Type:   tt.stepType,
				Config: tt.config,
			}

			assert.Equal(t, tt.config, step.Config)
			assert.Equal(t, tt.stepType, step.Type)
		})
	}
}

func TestStep_Config_NestedStructures(t *testing.T) {
	config := map[string]any{
		"database": map[string]any{
			"host":     "localhost",
			"port":     5432,
			"username": "user",
			"password": "pass",
		},
		"cache": map[string]any{
			"ttl":     3600,
			"enabled": true,
		},
		"servers": []any{
			map[string]any{"name": "server1", "port": 8080},
			map[string]any{"name": "server2", "port": 8081},
		},
	}

	step := &Step{
		Name:   "test-step",
		Type:   StepTypeCommand,
		Config: config,
	}

	dbConfig := step.Config["database"].(map[string]any)
	assert.Equal(t, "localhost", dbConfig["host"])
	assert.Equal(t, 5432, dbConfig["port"])

	cacheConfig := step.Config["cache"].(map[string]any)
	assert.Equal(t, 3600, cacheConfig["ttl"])

	servers := step.Config["servers"].([]any)
	assert.Len(t, servers, 2)
}

func TestStep_Config_Readonly(t *testing.T) {
	initialConfig := map[string]any{"key": "initial"}
	step := &Step{
		Name:   "test-step",
		Type:   StepTypeCommand,
		Config: initialConfig,
	}

	step.Config["key"] = "modified"
	assert.Equal(t, "modified", step.Config["key"])
}
