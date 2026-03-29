package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

// C069: Plugin Extensibility - YAML Config Parsing Tests

func TestYamlStep_Config_UnmarshalFromYAML(t *testing.T) {
	yamlContent := `
type: command
command: echo hello
config:
  timeout: 30
  enabled: true
  threshold: 0.95
`

	var step yamlStep
	err := yaml.Unmarshal([]byte(yamlContent), &step)

	assert.NoError(t, err)
	assert.NotNil(t, step.Config)
	assert.Equal(t, 30, step.Config["timeout"])
	assert.Equal(t, true, step.Config["enabled"])
	assert.Equal(t, 0.95, step.Config["threshold"])
}

func TestYamlStep_Config_EmptyConfig(t *testing.T) {
	yamlContent := `
type: command
command: test
config: {}
`

	var step yamlStep
	err := yaml.Unmarshal([]byte(yamlContent), &step)

	assert.NoError(t, err)
	assert.NotNil(t, step.Config)
	assert.Len(t, step.Config, 0)
}

func TestYamlStep_Config_NoConfig(t *testing.T) {
	yamlContent := `
type: command
command: test
`

	var step yamlStep
	err := yaml.Unmarshal([]byte(yamlContent), &step)

	assert.NoError(t, err)
	assert.Nil(t, step.Config)
}

func TestYamlStep_Config_NestedStructure(t *testing.T) {
	yamlContent := `
type: agent
provider: claude
prompt: test
config:
  database:
    host: localhost
    port: 5432
    credentials:
      username: user
      password: pass
  cache:
    ttl: 3600
    enabled: true
`

	var step yamlStep
	err := yaml.Unmarshal([]byte(yamlContent), &step)

	assert.NoError(t, err)
	assert.NotNil(t, step.Config)

	dbConfig := step.Config["database"].(map[string]any)
	assert.Equal(t, "localhost", dbConfig["host"])
	assert.Equal(t, 5432, dbConfig["port"])

	credsConfig := dbConfig["credentials"].(map[string]any)
	assert.Equal(t, "user", credsConfig["username"])
	assert.Equal(t, "pass", credsConfig["password"])

	cacheConfig := step.Config["cache"].(map[string]any)
	assert.Equal(t, 3600, cacheConfig["ttl"])
	assert.Equal(t, true, cacheConfig["enabled"])
}

func TestYamlStep_Config_WithArrays(t *testing.T) {
	yamlContent := `
type: command
command: test
config:
  tags:
    - tag1
    - tag2
    - tag3
  servers:
    - host: server1
      port: 8080
    - host: server2
      port: 8081
`

	var step yamlStep
	err := yaml.Unmarshal([]byte(yamlContent), &step)

	assert.NoError(t, err)
	assert.NotNil(t, step.Config)

	tags := step.Config["tags"].([]any)
	assert.Len(t, tags, 3)
	assert.Equal(t, "tag1", tags[0])

	servers := step.Config["servers"].([]any)
	assert.Len(t, servers, 2)
	server1 := servers[0].(map[string]any)
	assert.Equal(t, "server1", server1["host"])
	assert.Equal(t, 8080, server1["port"])
}

func TestYamlStep_Config_MixedTypes(t *testing.T) {
	yamlContent := `
type: command
command: test
config:
  string_val: hello
  int_val: 42
  float_val: 3.14
  bool_val: true
  null_val: null
  list_val:
    - item1
    - 2
    - true
  dict_val:
    nested: value
`

	var step yamlStep
	err := yaml.Unmarshal([]byte(yamlContent), &step)

	assert.NoError(t, err)
	assert.NotNil(t, step.Config)
	assert.Equal(t, "hello", step.Config["string_val"])
	assert.Equal(t, 42, step.Config["int_val"])
	assert.Equal(t, 3.14, step.Config["float_val"])
	assert.Equal(t, true, step.Config["bool_val"])
	assert.Nil(t, step.Config["null_val"])

	listVal := step.Config["list_val"].([]any)
	assert.Equal(t, "item1", listVal[0])
	assert.Equal(t, 2, listVal[1])
	assert.Equal(t, true, listVal[2])

	dictVal := step.Config["dict_val"].(map[string]any)
	assert.Equal(t, "value", dictVal["nested"])
}

func TestYamlStep_Config_OperationStep(t *testing.T) {
	yamlContent := `
type: operation
operation: github.create_issue
config:
  repository: awf-project/cli
  title: Test Issue
  body: Issue body content
  labels:
    - bug
    - priority-high
  assignees:
    - user1
    - user2
`

	var step yamlStep
	err := yaml.Unmarshal([]byte(yamlContent), &step)

	assert.NoError(t, err)
	assert.Equal(t, "operation", step.Type)
	assert.Equal(t, "github.create_issue", step.Operation)
	assert.NotNil(t, step.Config)
	assert.Equal(t, "awf-project/cli", step.Config["repository"])
	assert.Equal(t, "Test Issue", step.Config["title"])

	labels := step.Config["labels"].([]any)
	assert.Len(t, labels, 2)

	assignees := step.Config["assignees"].([]any)
	assert.Len(t, assignees, 2)
}

func TestYamlStep_Config_AllStepTypes(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		expectedLen int
	}{
		{
			name: "command step",
			yamlContent: `
type: command
command: test
config:
  param1: value1
  param2: 42
`,
			expectedLen: 2,
		},
		{
			name: "agent step",
			yamlContent: `
type: agent
provider: claude
prompt: test
config:
  model: claude-3
  temperature: 0.7
`,
			expectedLen: 2,
		},
		{
			name: "parallel step",
			yamlContent: `
type: parallel
parallel:
  - step1
  - step2
config:
  concurrent: 5
  timeout: 300
`,
			expectedLen: 2,
		},
		{
			name: "for_each step",
			yamlContent: `
type: for_each
items: "{{inputs.list}}"
body:
  - inner
config:
  batch_size: 10
  parallel: true
`,
			expectedLen: 2,
		},
		{
			name: "while step",
			yamlContent: `
type: while
while: "{{states.counter.output}} < 10"
body:
  - increment
config:
  check_interval: 1000
`,
			expectedLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var step yamlStep
			err := yaml.Unmarshal([]byte(tt.yamlContent), &step)

			assert.NoError(t, err)
			assert.NotNil(t, step.Config)
			assert.Len(t, step.Config, tt.expectedLen)
		})
	}
}

func TestYamlStep_Config_SpecialCharacters(t *testing.T) {
	yamlContent := `
type: command
command: test
config:
  key_with_underscore: value
  key-with-dash: value
  key.with.dots: value
  "quoted-key": value
  key@symbol: value
`

	var step yamlStep
	err := yaml.Unmarshal([]byte(yamlContent), &step)

	assert.NoError(t, err)
	assert.NotNil(t, step.Config)
	assert.Len(t, step.Config, 5)
	assert.Equal(t, "value", step.Config["key_with_underscore"])
	assert.Equal(t, "value", step.Config["key-with-dash"])
	assert.Equal(t, "value", step.Config["key.with.dots"])
	assert.Equal(t, "value", step.Config["quoted-key"])
	assert.Equal(t, "value", step.Config["key@symbol"])
}

func TestYamlStep_Config_LargeNestedStructure(t *testing.T) {
	yamlContent := `
type: command
command: test
config:
  level1:
    level2:
      level3:
        level4:
          level5:
            value: deep_value
            count: 99999
`

	var step yamlStep
	err := yaml.Unmarshal([]byte(yamlContent), &step)

	assert.NoError(t, err)
	assert.NotNil(t, step.Config)

	l1 := step.Config["level1"].(map[string]any)
	l2 := l1["level2"].(map[string]any)
	l3 := l2["level3"].(map[string]any)
	l4 := l3["level4"].(map[string]any)
	l5 := l4["level5"].(map[string]any)

	assert.Equal(t, "deep_value", l5["value"])
	assert.Equal(t, 99999, l5["count"])
}
