package workflow_test

import (
	"errors"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkflow_Validate_StateReferenceErrors tests all branches of StateReferenceError.Error()
// Covers: initial, on_success, on_failure, transition, loop_body, on_complete, default
func TestWorkflow_Validate_StateReferenceErrors(t *testing.T) {
	tests := []struct {
		name    string
		wf      workflow.Workflow
		wantErr bool
		errMsg  string // exact error message to match
	}{
		{
			name: "initial state not found - StateReferenceError initial branch",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "nonexistent",
				Steps: map[string]*workflow.Step{
					"end": {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: true,
			errMsg:  "initial state 'nonexistent' not found in steps",
		},
		{
			name: "on_success references unknown state - StateReferenceError on_success branch",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {
						Name:      "start",
						Type:      workflow.StepTypeCommand,
						Command:   "echo",
						OnSuccess: "unknown_success",
						OnFailure: "end",
					},
					"end": {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: true,
			errMsg:  "step 'start': on_success references unknown state 'unknown_success'",
		},
		{
			name: "on_failure references unknown state - StateReferenceError on_failure branch",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {
						Name:      "start",
						Type:      workflow.StepTypeCommand,
						Command:   "echo",
						OnSuccess: "end",
						OnFailure: "unknown_failure",
					},
					"end": {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: true,
			errMsg:  "step 'start': on_failure references unknown state 'unknown_failure'",
		},
		{
			name: "transition references unknown state - StateReferenceError transition branch",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {
						Name:    "start",
						Type:    workflow.StepTypeCommand,
						Command: "echo",
						Transitions: []workflow.Transition{
							{When: "true", Goto: "unknown_transition"},
						},
					},
					"end": {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: true,
			errMsg:  "step 'start': transition references unknown state 'unknown_transition'",
		},
		{
			name: "loop body references unknown step - StateReferenceError loop_body branch",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "loop_step",
				Steps: map[string]*workflow.Step{
					"loop_step": {
						Name:    "loop_step",
						Type:    workflow.StepTypeCommand,
						Command: "echo",
						Loop: &workflow.LoopConfig{
							Type:  workflow.LoopTypeForEach,
							Items: "{{inputs.items}}",
							Body:  []string{"unknown_body_step"},
						},
						OnSuccess: "end",
					},
					"end": {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: true,
			errMsg:  "step 'loop_step': loop body references unknown step 'unknown_body_step'",
		},
		{
			name: "loop on_complete references unknown state - StateReferenceError on_complete branch",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "loop_step",
				Steps: map[string]*workflow.Step{
					"body_step": {
						Name:      "body_step",
						Type:      workflow.StepTypeCommand,
						Command:   "echo",
						OnSuccess: "end",
					},
					"loop_step": {
						Name:    "loop_step",
						Type:    workflow.StepTypeCommand,
						Command: "echo",
						Loop: &workflow.LoopConfig{
							Type:       workflow.LoopTypeForEach,
							Items:      "{{inputs.items}}",
							Body:       []string{"body_step"},
							OnComplete: "unknown_complete",
						},
						OnSuccess: "end",
					},
					"end": {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: true,
			errMsg:  "step 'loop_step': on_complete references unknown state 'unknown_complete'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.wf.Validate(nil, nil)
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, tt.errMsg, err.Error())

				// Verify it's a StateReferenceError
				var stateRefErr *workflow.StateReferenceError
				assert.True(t, errors.As(err, &stateRefErr))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestWorkflow_Validate_MultipleErrors tests error accumulation paths
// Previously only single errors were tested - now testing multiple validation failures
func TestWorkflow_Validate_MultipleValidationPaths(t *testing.T) {
	tests := []struct {
		name    string
		wf      workflow.Workflow
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid workflow with transitions instead of legacy on_success/on_failure",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {
						Name:    "start",
						Type:    workflow.StepTypeCommand,
						Command: "echo",
						Transitions: []workflow.Transition{
							{When: "{{context.exit_code}} == 0", Goto: "end"},
						},
					},
					"end": {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: false,
		},
		{
			name: "command step with neither transitions nor legacy on_success/on_failure",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {
						Name:    "start",
						Type:    workflow.StepTypeCommand,
						Command: "echo",
						// Missing both Transitions and OnSuccess/OnFailure
					},
					"end": {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: true,
			errMsg:  "step 'start': command step must have OnSuccess/OnFailure or Transitions",
		},
		{
			name: "empty workflow name",
			wf: workflow.Workflow{
				Name:    "",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {Name: "start", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: true,
			errMsg:  "workflow name is required",
		},
		{
			name: "empty initial state",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "",
				Steps: map[string]*workflow.Step{
					"start": {Name: "start", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: true,
			errMsg:  "initial state is required",
		},
		{
			name: "no terminal state exists",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {
						Name:      "start",
						Type:      workflow.StepTypeCommand,
						Command:   "echo",
						OnSuccess: "middle",
					},
					"middle": {
						Name:      "middle",
						Type:      workflow.StepTypeCommand,
						Command:   "echo",
						OnSuccess: "start",
					},
				},
			},
			wantErr: true,
			errMsg:  "at least one terminal state is required",
		},
		{
			name: "workflow with both legacy and conditional transitions - valid",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {
						Name:      "start",
						Type:      workflow.StepTypeCommand,
						Command:   "echo",
						OnSuccess: "end",
						OnFailure: "end",
					},
					"end": {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: false,
		},
		{
			name: "loop body step can have transitions outside workflow (F048 exemption)",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "loop_step",
				Steps: map[string]*workflow.Step{
					"body_step": {
						Name:    "body_step",
						Type:    workflow.StepTypeCommand,
						Command: "echo",
						Transitions: []workflow.Transition{
							{When: "true", Goto: "external_target"}, // Not in workflow - should be allowed for loop body
						},
					},
					"loop_step": {
						Name:    "loop_step",
						Type:    workflow.StepTypeCommand,
						Command: "echo",
						Loop: &workflow.LoopConfig{
							Type:  workflow.LoopTypeForEach,
							Items: "{{inputs.items}}",
							Body:  []string{"body_step"},
						},
						OnSuccess: "end",
					},
					"end": {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: false, // F048: loop body steps can transition to external targets
		},
		{
			name: "non-loop body step with invalid transition - should fail",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {
						Name:    "start",
						Type:    workflow.StepTypeCommand,
						Command: "echo",
						Transitions: []workflow.Transition{
							{When: "true", Goto: "nonexistent"},
						},
					},
					"end": {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: true,
			errMsg:  "step 'start': transition references unknown state 'nonexistent'",
		},
		{
			name: "transition validation failure propagated",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {
						Name:    "start",
						Type:    workflow.StepTypeCommand,
						Command: "echo",
						Transitions: []workflow.Transition{
							{When: "true", Goto: ""}, // Invalid: empty Goto
						},
					},
					"end": {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: true,
			errMsg:  "step 'start': transition 0: transition goto is required",
		},
		{
			name: "step validation failure propagated",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {
						Name: "", // Invalid: empty name
						Type: workflow.StepTypeCommand,
					},
					"end": {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: true,
			// Error message will be wrapped: "step 'start': step name is required"
		},
		{
			name: "valid loop with on_complete",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "loop_step",
				Steps: map[string]*workflow.Step{
					"body_step": {
						Name:      "body_step",
						Type:      workflow.StepTypeCommand,
						Command:   "echo",
						OnSuccess: "end",
					},
					"loop_step": {
						Name:    "loop_step",
						Type:    workflow.StepTypeCommand,
						Command: "echo",
						Loop: &workflow.LoopConfig{
							Type:       workflow.LoopTypeForEach,
							Items:      "{{inputs.items}}",
							Body:       []string{"body_step"},
							OnComplete: "success_step",
						},
						OnSuccess: "end",
					},
					"success_step": {
						Name:      "success_step",
						Type:      workflow.StepTypeCommand,
						Command:   "echo complete",
						OnSuccess: "end",
					},
					"end": {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.wf.Validate(nil, nil)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestWorkflow_Validate_EdgeCases tests edge cases in validation logic
func TestWorkflow_Validate_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		wf      workflow.Workflow
		wantErr bool
		errMsg  string
	}{
		{
			name: "empty steps map",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps:   map[string]*workflow.Step{},
			},
			wantErr: true,
			errMsg:  "initial state 'start' not found in steps",
		},
		{
			name: "nil steps map",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps:   nil,
			},
			wantErr: true,
			errMsg:  "initial state 'start' not found in steps",
		},
		{
			name: "single terminal step - minimal valid workflow",
			wf: workflow.Workflow{
				Name:    "minimal",
				Initial: "end",
				Steps: map[string]*workflow.Step{
					"end": {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: false,
		},
		{
			name: "loop with empty body - should fail step validation",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "loop_step",
				Steps: map[string]*workflow.Step{
					"loop_step": {
						Name: "loop_step",
						Type: workflow.StepTypeForEach, // Use proper loop type
						Loop: &workflow.LoopConfig{
							Type:  workflow.LoopTypeForEach,
							Items: "{{inputs.items}}",
							Body:  []string{}, // Empty body
						},
					},
					"end": {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: true, // Step.Validate should catch this
			errMsg:  "body is required for loop steps",
		},
		{
			name: "multiple transitions - all valid",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {
						Name:    "start",
						Type:    workflow.StepTypeCommand,
						Command: "echo",
						Transitions: []workflow.Transition{
							{When: "{{context.exit_code}} == 0", Goto: "success"},
							{When: "{{context.exit_code}} == 1", Goto: "failure"},
							{When: "true", Goto: "end"},
						},
					},
					"success": {Name: "success", Type: workflow.StepTypeTerminal},
					"failure": {Name: "failure", Type: workflow.StepTypeTerminal},
					"end":     {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple transitions - one invalid",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {
						Name:    "start",
						Type:    workflow.StepTypeCommand,
						Command: "echo",
						Transitions: []workflow.Transition{
							{When: "{{context.exit_code}} == 0", Goto: "success"},
							{When: "{{context.exit_code}} == 1", Goto: "invalid_state"},
						},
					},
					"success": {Name: "success", Type: workflow.StepTypeTerminal},
					"end":     {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: true,
			errMsg:  "step 'start': transition references unknown state 'invalid_state'",
		},
		{
			name: "loop with multiple body steps - all valid",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "loop_step",
				Steps: map[string]*workflow.Step{
					"step1": {
						Name:      "step1",
						Type:      workflow.StepTypeCommand,
						Command:   "echo 1",
						OnSuccess: "step2",
					},
					"step2": {
						Name:      "step2",
						Type:      workflow.StepTypeCommand,
						Command:   "echo 2",
						OnSuccess: "end",
					},
					"loop_step": {
						Name:    "loop_step",
						Type:    workflow.StepTypeCommand,
						Command: "echo loop",
						Loop: &workflow.LoopConfig{
							Type:  workflow.LoopTypeForEach,
							Items: "{{inputs.items}}",
							Body:  []string{"step1", "step2"},
						},
						OnSuccess: "end",
					},
					"end": {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: false,
		},
		{
			name: "loop with multiple body steps - one invalid",
			wf: workflow.Workflow{
				Name:    "test",
				Initial: "loop_step",
				Steps: map[string]*workflow.Step{
					"step1": {
						Name:      "step1",
						Type:      workflow.StepTypeCommand,
						Command:   "echo 1",
						OnSuccess: "end",
					},
					"loop_step": {
						Name:    "loop_step",
						Type:    workflow.StepTypeCommand,
						Command: "echo loop",
						Loop: &workflow.LoopConfig{
							Type:  workflow.LoopTypeForEach,
							Items: "{{inputs.items}}",
							Body:  []string{"step1", "invalid_step"},
						},
						OnSuccess: "end",
					},
					"end": {Name: "end", Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: true,
			errMsg:  "step 'loop_step': loop body references unknown step 'invalid_step'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.wf.Validate(nil, nil)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestStateReferenceError_AllBranches explicitly tests all Error() branches
func TestStateReferenceError_AllBranches(t *testing.T) {
	tests := []struct {
		name     string
		err      *workflow.StateReferenceError
		expected string
	}{
		{
			name: "initial field",
			err: &workflow.StateReferenceError{
				StepName:        "",
				ReferencedState: "missing_initial",
				AvailableStates: []string{"step1", "step2"},
				Field:           "initial",
			},
			expected: "initial state 'missing_initial' not found in steps",
		},
		{
			name: "on_success field",
			err: &workflow.StateReferenceError{
				StepName:        "my_step",
				ReferencedState: "unknown_success",
				AvailableStates: []string{"step1", "step2"},
				Field:           "on_success",
			},
			expected: "step 'my_step': on_success references unknown state 'unknown_success'",
		},
		{
			name: "on_failure field",
			err: &workflow.StateReferenceError{
				StepName:        "my_step",
				ReferencedState: "unknown_failure",
				AvailableStates: []string{"step1", "step2"},
				Field:           "on_failure",
			},
			expected: "step 'my_step': on_failure references unknown state 'unknown_failure'",
		},
		{
			name: "transition field",
			err: &workflow.StateReferenceError{
				StepName:        "my_step",
				ReferencedState: "unknown_transition",
				AvailableStates: []string{"step1", "step2"},
				Field:           "transition",
			},
			expected: "step 'my_step': transition references unknown state 'unknown_transition'",
		},
		{
			name: "loop_body field",
			err: &workflow.StateReferenceError{
				StepName:        "loop_step",
				ReferencedState: "unknown_body",
				AvailableStates: []string{"step1", "step2"},
				Field:           "loop_body",
			},
			expected: "step 'loop_step': loop body references unknown step 'unknown_body'",
		},
		{
			name: "on_complete field",
			err: &workflow.StateReferenceError{
				StepName:        "loop_step",
				ReferencedState: "unknown_complete",
				AvailableStates: []string{"step1", "step2"},
				Field:           "on_complete",
			},
			expected: "step 'loop_step': on_complete references unknown state 'unknown_complete'",
		},
		{
			name: "default case - unknown field",
			err: &workflow.StateReferenceError{
				StepName:        "my_step",
				ReferencedState: "unknown_state",
				AvailableStates: []string{"step1", "step2"},
				Field:           "some_unknown_field",
			},
			expected: "step 'my_step': references unknown state 'unknown_state'",
		},
		{
			name: "default case - empty field",
			err: &workflow.StateReferenceError{
				StepName:        "my_step",
				ReferencedState: "unknown_state",
				AvailableStates: []string{"step1", "step2"},
				Field:           "",
			},
			expected: "step 'my_step': references unknown state 'unknown_state'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

// TestStateReferenceError_AvailableStatesField verifies AvailableStates is populated
func TestStateReferenceError_AvailableStatesField(t *testing.T) {
	wf := workflow.Workflow{
		Name:    "test",
		Initial: "nonexistent",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeTerminal},
			"step2": {Name: "step2", Type: workflow.StepTypeTerminal},
			"step3": {Name: "step3", Type: workflow.StepTypeTerminal},
		},
	}

	err := wf.Validate(nil, nil)
	require.Error(t, err)

	var stateRefErr *workflow.StateReferenceError
	require.True(t, errors.As(err, &stateRefErr))

	// AvailableStates should contain all step names (order may vary due to map iteration)
	assert.Len(t, stateRefErr.AvailableStates, 3)
	assert.Contains(t, stateRefErr.AvailableStates, "step1")
	assert.Contains(t, stateRefErr.AvailableStates, "step2")
	assert.Contains(t, stateRefErr.AvailableStates, "step3")
	assert.Equal(t, "nonexistent", stateRefErr.ReferencedState)
	assert.Equal(t, "initial", stateRefErr.Field)
}
