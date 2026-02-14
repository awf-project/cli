//go:build integration

package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

//
// These tests verify that the sub-workflow YAML fixtures are:
// 1. Present in the expected location
// 2. Valid YAML syntax
// 3. Have the expected structure (name, version, states, etc.)
//
// Note: These tests only validate YAML structure. Full workflow validation
// will fail until the call_workflow step type is implemented.

const subworkflowFixturesPath = "../fixtures/workflows"

// rawWorkflow is a minimal struct for validating YAML structure without
// requiring the call_workflow step type to be implemented.
type rawWorkflow struct {
	Name        string                 `yaml:"name"`
	Version     string                 `yaml:"version"`
	Description string                 `yaml:"description"`
	Inputs      []rawInput             `yaml:"inputs"`
	Outputs     []rawOutput            `yaml:"outputs"`
	States      map[string]interface{} `yaml:"states"`
}

type rawInput struct {
	Name        string      `yaml:"name"`
	Type        string      `yaml:"type"`
	Required    bool        `yaml:"required"`
	Default     interface{} `yaml:"default"`
	Description string      `yaml:"description"`
}

type rawOutput struct {
	Name        string `yaml:"name"`
	From        string `yaml:"from"`
	Description string `yaml:"description"`
}

// TestSubworkflowFixtures_Exist verifies all required fixture files are present.
func TestSubworkflowFixtures_Exist(t *testing.T) {
	requiredFixtures := []string{
		"subworkflow-simple.yaml",
		"subworkflow-child.yaml",
		"subworkflow-nested-a.yaml",
		"subworkflow-nested-b.yaml",
		"subworkflow-nested-c.yaml",
		"subworkflow-circular.yaml",
		"subworkflow-circular-a.yaml",
		"subworkflow-circular-b.yaml",
	}

	for _, fixture := range requiredFixtures {
		t.Run(fixture, func(t *testing.T) {
			path := filepath.Join(subworkflowFixturesPath, fixture)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("fixture file does not exist: %s", path)
			}
		})
	}
}

// TestSubworkflowFixtures_ValidYAML verifies all fixtures are syntactically valid YAML.
func TestSubworkflowFixtures_ValidYAML(t *testing.T) {
	fixtures := []string{
		"subworkflow-simple.yaml",
		"subworkflow-child.yaml",
		"subworkflow-nested-a.yaml",
		"subworkflow-nested-b.yaml",
		"subworkflow-nested-c.yaml",
		"subworkflow-circular.yaml",
		"subworkflow-circular-a.yaml",
		"subworkflow-circular-b.yaml",
	}

	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			path := filepath.Join(subworkflowFixturesPath, fixture)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read fixture: %v", err)
			}

			var wf rawWorkflow
			if err := yaml.Unmarshal(data, &wf); err != nil {
				t.Errorf("invalid YAML syntax: %v", err)
			}
		})
	}
}

// TestSubworkflowFixtures_RequiredFields verifies fixtures have required workflow fields.
func TestSubworkflowFixtures_RequiredFields(t *testing.T) {
	fixtures := []struct {
		filename    string
		wantName    string
		wantVersion string
	}{
		{"subworkflow-simple.yaml", "subworkflow-simple", "1.0.0"},
		{"subworkflow-child.yaml", "subworkflow-child", "1.0.0"},
		{"subworkflow-nested-a.yaml", "subworkflow-nested-a", "1.0.0"},
		{"subworkflow-nested-b.yaml", "subworkflow-nested-b", "1.0.0"},
		{"subworkflow-nested-c.yaml", "subworkflow-nested-c", "1.0.0"},
		{"subworkflow-circular.yaml", "subworkflow-circular", "1.0.0"},
		{"subworkflow-circular-a.yaml", "subworkflow-circular-a", "1.0.0"},
		{"subworkflow-circular-b.yaml", "subworkflow-circular-b", "1.0.0"},
	}

	for _, tt := range fixtures {
		t.Run(tt.filename, func(t *testing.T) {
			path := filepath.Join(subworkflowFixturesPath, tt.filename)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read fixture: %v", err)
			}

			var wf rawWorkflow
			if err := yaml.Unmarshal(data, &wf); err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}

			if wf.Name != tt.wantName {
				t.Errorf("name = %q, want %q", wf.Name, tt.wantName)
			}
			if wf.Version != tt.wantVersion {
				t.Errorf("version = %q, want %q", wf.Version, tt.wantVersion)
			}
			if wf.States == nil {
				t.Error("states is nil, want non-nil")
			}
		})
	}
}

// TestSubworkflowFixtures_ChildHasInputsAndOutputs verifies child workflow has proper I/O.
func TestSubworkflowFixtures_ChildHasInputsAndOutputs(t *testing.T) {
	path := filepath.Join(subworkflowFixturesPath, "subworkflow-child.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	var wf rawWorkflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	// Verify inputs
	if len(wf.Inputs) < 1 {
		t.Errorf("inputs count = %d, want at least 1", len(wf.Inputs))
	}

	// Check for 'message' input
	foundMessage := false
	for _, input := range wf.Inputs {
		if input.Name == "message" {
			foundMessage = true
			if !input.Required {
				t.Error("message input should be required")
			}
			break
		}
	}
	if !foundMessage {
		t.Error("expected 'message' input not found")
	}

	// Verify outputs
	if len(wf.Outputs) < 1 {
		t.Errorf("outputs count = %d, want at least 1", len(wf.Outputs))
	}

	// Check for 'result' output
	foundResult := false
	for _, output := range wf.Outputs {
		if output.Name == "result" {
			foundResult = true
			if output.From == "" {
				t.Error("result output should have 'from' field")
			}
			break
		}
	}
	if !foundResult {
		t.Error("expected 'result' output not found")
	}
}

// TestSubworkflowFixtures_SimpleParentHasCallWorkflow verifies parent has call_workflow step.
func TestSubworkflowFixtures_SimpleParentHasCallWorkflow(t *testing.T) {
	path := filepath.Join(subworkflowFixturesPath, "subworkflow-simple.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	var wf rawWorkflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	// Look for call_child step
	callChild, ok := wf.States["call_child"]
	if !ok {
		t.Fatal("call_child state not found")
	}

	stepMap, ok := callChild.(map[string]interface{})
	if !ok {
		t.Fatalf("call_child is not a map, got %T", callChild)
	}

	// Verify it's a call_workflow type
	stepType, ok := stepMap["type"].(string)
	if !ok {
		t.Fatal("call_child.type not found or not a string")
	}
	if stepType != "call_workflow" {
		t.Errorf("call_child.type = %q, want 'call_workflow'", stepType)
	}

	// Verify workflow reference
	workflow, ok := stepMap["workflow"].(string)
	if !ok {
		t.Fatal("call_child.workflow not found or not a string")
	}
	if workflow != "subworkflow-child" {
		t.Errorf("call_child.workflow = %q, want 'subworkflow-child'", workflow)
	}

	// Verify inputs mapping
	inputs, ok := stepMap["inputs"].(map[string]interface{})
	if !ok {
		t.Fatal("call_child.inputs not found or not a map")
	}
	if _, ok := inputs["message"]; !ok {
		t.Error("call_child.inputs.message not found")
	}

	// Verify outputs mapping
	outputs, ok := stepMap["outputs"].(map[string]interface{})
	if !ok {
		t.Fatal("call_child.outputs not found or not a map")
	}
	if _, ok := outputs["result"]; !ok {
		t.Error("call_child.outputs.result not found")
	}
}

// TestSubworkflowFixtures_NestedChain verifies the A -> B -> C nesting chain.
func TestSubworkflowFixtures_NestedChain(t *testing.T) {
	// Verify A calls B
	t.Run("A_calls_B", func(t *testing.T) {
		path := filepath.Join(subworkflowFixturesPath, "subworkflow-nested-a.yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read fixture: %v", err)
		}

		var wf rawWorkflow
		if err := yaml.Unmarshal(data, &wf); err != nil {
			t.Fatalf("failed to parse YAML: %v", err)
		}

		callB, ok := wf.States["call_b"]
		if !ok {
			t.Fatal("call_b state not found in nested-a")
		}

		stepMap := callB.(map[string]interface{})
		if stepMap["workflow"] != "subworkflow-nested-b" {
			t.Errorf("call_b.workflow = %v, want 'subworkflow-nested-b'", stepMap["workflow"])
		}
	})

	// Verify B calls C
	t.Run("B_calls_C", func(t *testing.T) {
		path := filepath.Join(subworkflowFixturesPath, "subworkflow-nested-b.yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read fixture: %v", err)
		}

		var wf rawWorkflow
		if err := yaml.Unmarshal(data, &wf); err != nil {
			t.Fatalf("failed to parse YAML: %v", err)
		}

		callC, ok := wf.States["call_c"]
		if !ok {
			t.Fatal("call_c state not found in nested-b")
		}

		stepMap := callC.(map[string]interface{})
		if stepMap["workflow"] != "subworkflow-nested-c" {
			t.Errorf("call_c.workflow = %v, want 'subworkflow-nested-c'", stepMap["workflow"])
		}
	})

	// Verify C is a leaf (no call_workflow steps)
	t.Run("C_is_leaf", func(t *testing.T) {
		path := filepath.Join(subworkflowFixturesPath, "subworkflow-nested-c.yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read fixture: %v", err)
		}

		var wf rawWorkflow
		if err := yaml.Unmarshal(data, &wf); err != nil {
			t.Fatalf("failed to parse YAML: %v", err)
		}

		// Check no state has type call_workflow
		for name, state := range wf.States {
			if name == "initial" {
				continue // skip initial key
			}
			stepMap, ok := state.(map[string]interface{})
			if !ok {
				continue
			}
			if stepType, ok := stepMap["type"].(string); ok && stepType == "call_workflow" {
				t.Errorf("C should be leaf but found call_workflow in state %q", name)
			}
		}
	})
}

// TestSubworkflowFixtures_CircularReference verifies A -> B -> A pattern.
func TestSubworkflowFixtures_CircularReference(t *testing.T) {
	// Verify A calls B
	t.Run("A_calls_B", func(t *testing.T) {
		path := filepath.Join(subworkflowFixturesPath, "subworkflow-circular-a.yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read fixture: %v", err)
		}

		var wf rawWorkflow
		if err := yaml.Unmarshal(data, &wf); err != nil {
			t.Fatalf("failed to parse YAML: %v", err)
		}

		callB, ok := wf.States["call_b"]
		if !ok {
			t.Fatal("call_b state not found in circular-a")
		}

		stepMap := callB.(map[string]interface{})
		if stepMap["workflow"] != "subworkflow-circular-b" {
			t.Errorf("call_b.workflow = %v, want 'subworkflow-circular-b'", stepMap["workflow"])
		}
	})

	// Verify B calls back to A (creates the cycle)
	t.Run("B_calls_A", func(t *testing.T) {
		path := filepath.Join(subworkflowFixturesPath, "subworkflow-circular-b.yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read fixture: %v", err)
		}

		var wf rawWorkflow
		if err := yaml.Unmarshal(data, &wf); err != nil {
			t.Fatalf("failed to parse YAML: %v", err)
		}

		callA, ok := wf.States["call_a"]
		if !ok {
			t.Fatal("call_a state not found in circular-b")
		}

		stepMap := callA.(map[string]interface{})
		if stepMap["workflow"] != "subworkflow-circular-a" {
			t.Errorf("call_a.workflow = %v, want 'subworkflow-circular-a'", stepMap["workflow"])
		}
	})
}

// TestSubworkflowFixtures_SelfCircularReference verifies A → A (direct self-call) pattern.
func TestSubworkflowFixtures_SelfCircularReference(t *testing.T) {
	path := filepath.Join(subworkflowFixturesPath, "subworkflow-circular.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	var wf rawWorkflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	// Verify the workflow calls itself
	callSelf, ok := wf.States["call_self"]
	if !ok {
		t.Fatal("call_self state not found in circular.yaml")
	}

	stepMap := callSelf.(map[string]interface{})

	// Verify it's a call_workflow type
	stepType, ok := stepMap["type"].(string)
	if !ok {
		t.Fatal("call_self.type not found or not a string")
	}
	if stepType != "call_workflow" {
		t.Errorf("call_self.type = %q, want 'call_workflow'", stepType)
	}

	// Verify it calls itself (same workflow name)
	workflow, ok := stepMap["workflow"].(string)
	if !ok {
		t.Fatal("call_self.workflow not found or not a string")
	}
	if workflow != wf.Name {
		t.Errorf("call_self.workflow = %q, want %q (self-reference)", workflow, wf.Name)
	}
}

// TestSubworkflowFixtures_TimeoutSpecified verifies call_workflow steps have timeout.
func TestSubworkflowFixtures_TimeoutSpecified(t *testing.T) {
	fixtures := []struct {
		filename string
		stepName string
		wantMin  int
	}{
		{"subworkflow-simple.yaml", "call_child", 1},
		{"subworkflow-nested-a.yaml", "call_b", 1},
		{"subworkflow-nested-b.yaml", "call_c", 1},
		{"subworkflow-circular.yaml", "call_self", 1},
		{"subworkflow-circular-a.yaml", "call_b", 1},
		{"subworkflow-circular-b.yaml", "call_a", 1},
	}

	for _, tt := range fixtures {
		t.Run(tt.filename, func(t *testing.T) {
			path := filepath.Join(subworkflowFixturesPath, tt.filename)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read fixture: %v", err)
			}

			var wf rawWorkflow
			if err := yaml.Unmarshal(data, &wf); err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}

			step, ok := wf.States[tt.stepName]
			if !ok {
				t.Fatalf("step %q not found", tt.stepName)
			}

			stepMap := step.(map[string]interface{})
			timeout, ok := stepMap["timeout"]
			if !ok {
				t.Errorf("timeout not specified for %s.%s", tt.filename, tt.stepName)
				return
			}

			// timeout should be a positive number
			var timeoutVal int
			switch v := timeout.(type) {
			case int:
				timeoutVal = v
			case float64:
				timeoutVal = int(v)
			default:
				t.Errorf("timeout is not a number: %T", timeout)
				return
			}

			if timeoutVal < tt.wantMin {
				t.Errorf("timeout = %d, want >= %d", timeoutVal, tt.wantMin)
			}
		})
	}
}
