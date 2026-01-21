package workflow_test

import (
	"testing"

	"github.com/vanoix/awf/internal/domain/workflow"
)

// =============================================================================
// Step Agent Tests Extracted from step_test.go
// Component: C013 - Domain Test File Splitting
// Tests: 20 (8 CallWorkflow + 12 Agent)
// Source: internal/domain/workflow/step_test.go
// =============================================================================

// =============================================================================
// CallWorkflow Step Tests (F023)
// =============================================================================

func TestCallWorkflowStepValidation(t *testing.T) {
	tests := []struct {
		name    string
		step    workflow.Step
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid call_workflow step",
			step: workflow.Step{
				Name: "call_analyzer",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "analyze-file",
					Inputs: map[string]string{
						"file_path": "{{inputs.source_file}}",
					},
					Outputs: map[string]string{
						"analysis_result": "result",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid call_workflow step with timeout",
			step: workflow.Step{
				Name: "call_with_timeout",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "long-running-workflow",
					Timeout:  600,
				},
			},
			wantErr: false,
		},
		{
			name: "valid call_workflow step minimal config",
			step: workflow.Step{
				Name: "call_simple",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "simple-workflow",
				},
			},
			wantErr: false,
		},
		{
			name: "call_workflow step without config",
			step: workflow.Step{
				Name: "bad_call",
				Type: workflow.StepTypeCallWorkflow,
			},
			wantErr: true,
			errMsg:  "call_workflow config is required",
		},
		{
			name: "call_workflow step with nil config",
			step: workflow.Step{
				Name:         "nil_config",
				Type:         workflow.StepTypeCallWorkflow,
				CallWorkflow: nil,
			},
			wantErr: true,
			errMsg:  "call_workflow config is required",
		},
		{
			name: "call_workflow step with empty workflow name",
			step: workflow.Step{
				Name: "empty_workflow",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "",
				},
			},
			wantErr: true,
			errMsg:  "workflow name is required",
		},
		{
			name: "call_workflow step with negative timeout",
			step: workflow.Step{
				Name: "negative_timeout",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "some-workflow",
					Timeout:  -1,
				},
			},
			wantErr: true,
			errMsg:  "timeout must be non-negative",
		},
		{
			name: "call_workflow step with zero timeout (uses default)",
			step: workflow.Step{
				Name: "zero_timeout",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "some-workflow",
					Timeout:  0,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Step.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestCallWorkflowStepCreation(t *testing.T) {
	step := workflow.Step{
		Name:        "invoke_analyzer",
		Type:        workflow.StepTypeCallWorkflow,
		Description: "Call the file analyzer sub-workflow",
		Timeout:     120,
		OnSuccess:   "aggregate",
		OnFailure:   "handle_error",
		CallWorkflow: &workflow.CallWorkflowConfig{
			Workflow: "analyze-single-file",
			Inputs: map[string]string{
				"file_path":  "{{loop.item}}",
				"max_tokens": "{{inputs.max_tokens}}",
			},
			Outputs: map[string]string{
				"result": "analysis_result",
			},
			Timeout: 300,
		},
	}

	if step.Name != "invoke_analyzer" {
		t.Errorf("expected name 'invoke_analyzer', got '%s'", step.Name)
	}
	if step.Type != workflow.StepTypeCallWorkflow {
		t.Errorf("expected type StepTypeCallWorkflow, got '%v'", step.Type)
	}
	if step.CallWorkflow == nil {
		t.Fatal("expected CallWorkflow to be set")
	}
	if step.CallWorkflow.Workflow != "analyze-single-file" {
		t.Errorf("expected workflow 'analyze-single-file', got '%s'", step.CallWorkflow.Workflow)
	}
	if len(step.CallWorkflow.Inputs) != 2 {
		t.Errorf("expected 2 inputs, got %d", len(step.CallWorkflow.Inputs))
	}
	if step.CallWorkflow.Inputs["file_path"] != "{{loop.item}}" {
		t.Errorf("expected input file_path '{{loop.item}}', got '%s'", step.CallWorkflow.Inputs["file_path"])
	}
	if len(step.CallWorkflow.Outputs) != 1 {
		t.Errorf("expected 1 output, got %d", len(step.CallWorkflow.Outputs))
	}
	if step.CallWorkflow.Outputs["result"] != "analysis_result" {
		t.Errorf("expected output 'result' -> 'analysis_result', got '%s'", step.CallWorkflow.Outputs["result"])
	}
	if step.CallWorkflow.Timeout != 300 {
		t.Errorf("expected timeout 300, got %d", step.CallWorkflow.Timeout)
	}
	if step.OnSuccess != "aggregate" {
		t.Errorf("expected OnSuccess 'aggregate', got '%s'", step.OnSuccess)
	}
	if step.OnFailure != "handle_error" {
		t.Errorf("expected OnFailure 'handle_error', got '%s'", step.OnFailure)
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("valid call_workflow step should not return error: %v", err)
	}
}

func TestCallWorkflowStepWithHooks(t *testing.T) {
	step := workflow.Step{
		Name: "call_with_hooks",
		Type: workflow.StepTypeCallWorkflow,
		CallWorkflow: &workflow.CallWorkflowConfig{
			Workflow: "sub-workflow",
		},
		Hooks: workflow.StepHooks{
			Pre:  workflow.Hook{{Log: "Calling sub-workflow"}},
			Post: workflow.Hook{{Log: "Sub-workflow completed"}},
		},
	}

	if len(step.Hooks.Pre) != 1 {
		t.Errorf("expected 1 pre hook, got %d", len(step.Hooks.Pre))
	}
	if len(step.Hooks.Post) != 1 {
		t.Errorf("expected 1 post hook, got %d", len(step.Hooks.Post))
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("call_workflow step with hooks should be valid: %v", err)
	}
}

func TestCallWorkflowStepWithRetry(t *testing.T) {
	step := workflow.Step{
		Name: "call_with_retry",
		Type: workflow.StepTypeCallWorkflow,
		CallWorkflow: &workflow.CallWorkflowConfig{
			Workflow: "flaky-workflow",
		},
		Retry: &workflow.RetryConfig{
			MaxAttempts:    3,
			InitialDelayMs: 1000,
			Backoff:        "exponential",
			Multiplier:     2.0,
		},
	}

	if step.Retry == nil {
		t.Fatal("expected Retry to be set")
	}
	if step.Retry.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts 3, got %d", step.Retry.MaxAttempts)
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("call_workflow step with retry should be valid: %v", err)
	}
}

func TestCallWorkflowStepWithContinueOnError(t *testing.T) {
	step := workflow.Step{
		Name: "optional_call",
		Type: workflow.StepTypeCallWorkflow,
		CallWorkflow: &workflow.CallWorkflowConfig{
			Workflow: "optional-workflow",
		},
		ContinueOnError: true,
	}

	if !step.ContinueOnError {
		t.Error("expected ContinueOnError to be true")
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("call_workflow step with ContinueOnError should be valid: %v", err)
	}
}

func TestCallWorkflowTypeConstant(t *testing.T) {
	// Verify the constant is defined correctly
	if workflow.StepTypeCallWorkflow != "call_workflow" {
		t.Errorf("StepTypeCallWorkflow should be 'call_workflow', got '%s'", workflow.StepTypeCallWorkflow)
	}

	// Verify it stringifies correctly
	if workflow.StepTypeCallWorkflow.String() != "call_workflow" {
		t.Errorf("StepTypeCallWorkflow.String() should be 'call_workflow', got '%s'", workflow.StepTypeCallWorkflow.String())
	}
}

func TestCallWorkflowStepWithEmptyInputsOutputs(t *testing.T) {
	// Steps can have empty inputs/outputs - workflow may not need any
	step := workflow.Step{
		Name: "call_no_io",
		Type: workflow.StepTypeCallWorkflow,
		CallWorkflow: &workflow.CallWorkflowConfig{
			Workflow: "standalone-workflow",
			Inputs:   map[string]string{},
			Outputs:  map[string]string{},
		},
	}

	if step.CallWorkflow.Inputs == nil {
		t.Error("expected Inputs to be initialized")
	}
	if step.CallWorkflow.Outputs == nil {
		t.Error("expected Outputs to be initialized")
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("call_workflow step with empty inputs/outputs should be valid: %v", err)
	}
}

func TestCallWorkflowStepWithTemplateInterpolation(t *testing.T) {
	// Test that template expressions in inputs are accepted
	step := workflow.Step{
		Name: "call_with_templates",
		Type: workflow.StepTypeCallWorkflow,
		CallWorkflow: &workflow.CallWorkflowConfig{
			Workflow: "process-file",
			Inputs: map[string]string{
				"file":      "{{inputs.source_file}}",
				"output":    "{{states.prepare.output}}",
				"env_value": "{{env.API_KEY}}",
				"combined":  "prefix-{{inputs.name}}-suffix",
			},
		},
	}

	if len(step.CallWorkflow.Inputs) != 4 {
		t.Errorf("expected 4 inputs, got %d", len(step.CallWorkflow.Inputs))
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("call_workflow step with template inputs should be valid: %v", err)
	}
}

// =============================================================================
// Agent Step Tests (F039)
// Component: step_type_extension
// Feature: 39 - AI Agent Step Type
// =============================================================================

func TestStepTypeAgent_String(t *testing.T) {
	if got := workflow.StepTypeAgent.String(); got != "agent" {
		t.Errorf("StepTypeAgent.String() = %s, want %s", got, "agent")
	}
}

func TestStep_Validate_AgentType_HappyPath(t *testing.T) {
	tests := []struct {
		name string
		step workflow.Step
	}{
		{
			name: "valid agent step with all fields",
			step: workflow.Step{
				Name: "ask_claude",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Analyze {{inputs.data}}",
					Options: map[string]any{
						"model":       "claude-3-5-sonnet-20241022",
						"temperature": 0.7,
						"max_tokens":  1000,
					},
					Timeout: 60,
				},
			},
		},
		{
			name: "valid agent step with minimal fields",
			step: workflow.Step{
				Name: "simple_agent",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "codex",
					Prompt:   "Generate code for {{inputs.task}}",
				},
			},
		},
		{
			name: "valid agent step with custom provider",
			step: workflow.Step{
				Name: "custom_agent",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "custom",
					Prompt:   "Process {{inputs.text}}",
					Command:  "python3 custom_agent.py --prompt={{prompt}}",
					Timeout:  120,
				},
			},
		},
		{
			name: "valid agent step with gemini provider",
			step: workflow.Step{
				Name: "gemini_agent",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "gemini",
					Prompt:   "Summarize {{inputs.article}}",
					Options: map[string]any{
						"model": "gemini-pro",
					},
				},
			},
		},
		{
			name: "valid agent step with zero timeout (uses default)",
			step: workflow.Step{
				Name: "default_timeout",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "opencode",
					Prompt:   "Review {{inputs.code}}",
					Timeout:  0,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate()
			if err != nil {
				t.Errorf("Step.Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestStep_Validate_AgentType_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		step    workflow.Step
		wantErr bool
		errMsg  string
	}{
		{
			name: "agent step with empty options map",
			step: workflow.Step{
				Name: "agent_no_options",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Test prompt",
					Options:  map[string]any{},
				},
			},
			wantErr: false,
		},
		{
			name: "agent step with nil options",
			step: workflow.Step{
				Name: "agent_nil_options",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Test prompt",
					Options:  nil,
				},
			},
			wantErr: false,
		},
		{
			name: "agent step with very long prompt",
			step: workflow.Step{
				Name: "long_prompt",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   string(make([]byte, 10000)) + "{{inputs.data}}",
				},
			},
			wantErr: false,
		},
		{
			name: "agent step with complex nested options",
			step: workflow.Step{
				Name: "complex_options",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Process {{inputs.data}}",
					Options: map[string]any{
						"model":       "claude-3-5-sonnet-20241022",
						"temperature": 0.7,
						"max_tokens":  1000,
						"metadata": map[string]any{
							"user_id": "123",
							"tags":    []string{"test", "ai"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "agent step with max timeout value",
			step: workflow.Step{
				Name: "max_timeout",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Long task {{inputs.data}}",
					Timeout:  3600,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Step.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg {
					t.Errorf("Step.Validate() error message = %q, want %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestStep_Validate_AgentType_ErrorHandling(t *testing.T) {
	tests := []struct {
		name    string
		step    workflow.Step
		wantErr bool
		errMsg  string
	}{
		{
			name: "agent step without agent config",
			step: workflow.Step{
				Name:  "no_config",
				Type:  workflow.StepTypeAgent,
				Agent: nil,
			},
			wantErr: true,
			errMsg:  "agent config is required for agent-type steps",
		},
		{
			name: "agent step with missing provider",
			step: workflow.Step{
				Name: "no_provider",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "",
					Prompt:   "Test prompt",
				},
			},
			wantErr: true,
		},
		{
			name: "agent step with missing prompt",
			step: workflow.Step{
				Name: "no_prompt",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "",
				},
			},
			wantErr: true,
		},
		{
			name: "agent step with negative timeout",
			step: workflow.Step{
				Name: "negative_timeout",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Test prompt",
					Timeout:  -60,
				},
			},
			wantErr: true,
		},
		{
			name: "agent step with empty name",
			step: workflow.Step{
				Name: "",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Test prompt",
				},
			},
			wantErr: true,
			errMsg:  "step name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Step.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg {
					t.Errorf("Step.Validate() error message = %q, want %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestStep_Validate_AgentType_WithTransitions(t *testing.T) {
	step := workflow.Step{
		Name: "agent_with_transitions",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Analyze {{inputs.data}}",
		},
		Transitions: workflow.Transitions{
			{
				When: "{{states.agent_with_transitions.output}} == 'success'",
				Goto: "next_step",
			},
			{
				When: "{{states.agent_with_transitions.output}} == 'failure'",
				Goto: "error_handler",
			},
		},
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("agent step with transitions should be valid: %v", err)
	}

	if len(step.Transitions) != 2 {
		t.Errorf("expected 2 transitions, got %d", len(step.Transitions))
	}
}

func TestStep_Validate_AgentType_WithRetry(t *testing.T) {
	step := workflow.Step{
		Name: "agent_with_retry",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Analyze {{inputs.data}}",
			Timeout:  30,
		},
		Retry: &workflow.RetryConfig{
			MaxAttempts:    3,
			InitialDelayMs: 1000,
			MaxDelayMs:     10000,
			Backoff:        "exponential",
			Multiplier:     2.0,
		},
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("agent step with retry config should be valid: %v", err)
	}

	if step.Retry.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts 3, got %d", step.Retry.MaxAttempts)
	}
}

func TestStep_Validate_AgentType_WithCapture(t *testing.T) {
	step := workflow.Step{
		Name: "agent_with_capture",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Generate report for {{inputs.data}}",
		},
		Capture: &workflow.CaptureConfig{
			Stdout:   "agent_output",
			Stderr:   "agent_errors",
			MaxSize:  "5MB",
			Encoding: "utf-8",
		},
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("agent step with capture config should be valid: %v", err)
	}

	if step.Capture.Stdout != "agent_output" {
		t.Errorf("expected Stdout 'agent_output', got %s", step.Capture.Stdout)
	}
}

func TestStep_Validate_AgentType_WithHooks(t *testing.T) {
	step := workflow.Step{
		Name: "agent_with_hooks",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Process {{inputs.data}}",
		},
		Hooks: workflow.StepHooks{
			Pre:  workflow.Hook{{Command: "echo 'Starting agent execution'"}},
			Post: workflow.Hook{{Command: "echo 'Agent execution completed'"}},
		},
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("agent step with hooks should be valid: %v", err)
	}

	if len(step.Hooks.Pre) != 1 {
		t.Errorf("expected 1 pre hook, got %d", len(step.Hooks.Pre))
	}
}

func TestStep_Validate_AgentType_WithTimeout(t *testing.T) {
	tests := []struct {
		name    string
		step    workflow.Step
		wantErr bool
	}{
		{
			name: "agent step with valid timeout",
			step: workflow.Step{
				Name: "agent_timeout",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Quick task",
					Timeout:  30,
				},
				Timeout: 60, // step-level timeout
			},
			wantErr: false,
		},
		{
			name: "agent step with only agent timeout",
			step: workflow.Step{
				Name: "agent_only_timeout",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Quick task",
					Timeout:  45,
				},
			},
			wantErr: false,
		},
		{
			name: "agent step with only step timeout",
			step: workflow.Step{
				Name: "step_only_timeout",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "claude",
					Prompt:   "Quick task",
				},
				Timeout: 90,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Step.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStep_Validate_AgentType_WithContinueOnError(t *testing.T) {
	step := workflow.Step{
		Name: "agent_continue_on_error",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Try to process {{inputs.data}}",
		},
		ContinueOnError: true,
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("agent step with continue_on_error should be valid: %v", err)
	}

	if !step.ContinueOnError {
		t.Errorf("expected ContinueOnError true, got false")
	}
}

func TestStep_Validate_AgentType_WithDependsOn(t *testing.T) {
	step := workflow.Step{
		Name: "dependent_agent",
		Type: workflow.StepTypeAgent,
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Analyze results from {{states.step1.output}} and {{states.step2.output}}",
		},
		DependsOn: []string{"step1", "step2"},
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("agent step with dependencies should be valid: %v", err)
	}

	if len(step.DependsOn) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(step.DependsOn))
	}
}

func TestStep_Validate_AgentType_CompleteWorkflow(t *testing.T) {
	// Test a complete agent step with all possible configurations
	step := workflow.Step{
		Name:        "comprehensive_agent",
		Type:        workflow.StepTypeAgent,
		Description: "Comprehensive AI agent with all features",
		Agent: &workflow.AgentConfig{
			Provider: "claude",
			Prompt:   "Analyze and process {{inputs.data}} considering {{inputs.context}}",
			Options: map[string]any{
				"model":            "claude-3-5-sonnet-20241022",
				"temperature":      0.7,
				"max_tokens":       2000,
				"top_p":            0.9,
				"stop_sequences":   []string{"\n\nHuman:"},
				"presence_penalty": 0.0,
			},
			Timeout: 120,
		},
		Timeout: 180,
		Retry: &workflow.RetryConfig{
			MaxAttempts:    3,
			InitialDelayMs: 2000,
			MaxDelayMs:     30000,
			Backoff:        "exponential",
			Multiplier:     2.0,
			Jitter:         0.1,
		},
		Capture: &workflow.CaptureConfig{
			Stdout:   "agent_response",
			Stderr:   "agent_errors",
			MaxSize:  "10MB",
			Encoding: "utf-8",
		},
		Hooks: workflow.StepHooks{
			Pre:  workflow.Hook{{Command: "echo 'Preparing agent execution'"}},
			Post: workflow.Hook{{Command: "echo 'Agent execution finished'"}},
		},
		Transitions: workflow.Transitions{
			{
				When: "{{states.comprehensive_agent.output}} contains 'approved'",
				Goto: "approval_step",
			},
			{
				When: "{{states.comprehensive_agent.output}} contains 'rejected'",
				Goto: "rejection_step",
			},
		},
		DependsOn:       []string{"prepare_data", "load_context"},
		ContinueOnError: false,
	}

	err := step.Validate()
	if err != nil {
		t.Errorf("comprehensive agent step should be valid: %v", err)
	}

	// Verify all configurations are set
	if step.Agent.Provider != "claude" {
		t.Errorf("expected provider 'claude', got %s", step.Agent.Provider)
	}
	if step.Agent.Timeout != 120 {
		t.Errorf("expected agent timeout 120, got %d", step.Agent.Timeout)
	}
	if step.Timeout != 180 {
		t.Errorf("expected step timeout 180, got %d", step.Timeout)
	}
	if step.Retry.MaxAttempts != 3 {
		t.Errorf("expected 3 retry attempts, got %d", step.Retry.MaxAttempts)
	}
	if step.Capture.Stdout != "agent_response" {
		t.Errorf("expected stdout capture 'agent_response', got %s", step.Capture.Stdout)
	}
	if len(step.Hooks.Pre) != 1 {
		t.Errorf("expected 1 pre hook, got %d", len(step.Hooks.Pre))
	}
	if len(step.Transitions) != 2 {
		t.Errorf("expected 2 transitions, got %d", len(step.Transitions))
	}
	if len(step.DependsOn) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(step.DependsOn))
	}
}

// =============================================================================
// Helper Functions
// Note: containsString is defined in step_loop_test.go (shared across step tests)
// =============================================================================
