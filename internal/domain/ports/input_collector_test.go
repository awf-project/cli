package ports_test

import (
	"errors"
	"testing"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
)

// Component: input_collector_port
// Feature: F046

// mockInputCollector is a test implementation of the InputCollector interface
// for use in application service tests and contract validation.
type mockInputCollector struct {
	// inputs stores prompts that were requested
	inputs []*workflow.Input
	// values maps input name to value to return
	values map[string]any
	// errors maps input name to error to return
	errors map[string]error
	// callCount tracks number of PromptForInput calls
	callCount int
}

func newMockInputCollector() *mockInputCollector {
	return &mockInputCollector{
		inputs: make([]*workflow.Input, 0),
		values: make(map[string]any),
		errors: make(map[string]error),
	}
}

func (m *mockInputCollector) PromptForInput(input *workflow.Input) (any, error) {
	m.inputs = append(m.inputs, input)
	m.callCount++

	if err, ok := m.errors[input.Name]; ok {
		return nil, err
	}

	if val, ok := m.values[input.Name]; ok {
		return val, nil
	}

	// Default behavior: return default value for optional, error for required
	if !input.Required && input.Default != nil {
		return input.Default, nil
	}

	if !input.Required {
		return nil, nil
	}

	return nil, errors.New("no mock value configured")
}

// TestInputCollectorInterface verifies that mockInputCollector implements the port interface.
func TestInputCollectorInterface(t *testing.T) {
	var _ ports.InputCollector = (*mockInputCollector)(nil)
}

// TestMockInputCollector_RequiredInput tests mock behavior with required inputs.
func TestMockInputCollector_RequiredInput(t *testing.T) {
	// F046: US1 - Required input collection
	mock := newMockInputCollector()
	mock.values["username"] = "testuser"

	input := &workflow.Input{
		Name:        "username",
		Type:        "string",
		Description: "User name",
		Required:    true,
	}

	value, err := mock.PromptForInput(input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if value != "testuser" {
		t.Errorf("expected 'testuser', got %v", value)
	}
	if mock.callCount != 1 {
		t.Errorf("expected 1 call, got %d", mock.callCount)
	}
}

// TestMockInputCollector_OptionalInputWithDefault tests mock behavior with optional inputs that have defaults.
func TestMockInputCollector_OptionalInputWithDefault(t *testing.T) {
	// F046: US2 - Optional input with default value
	mock := newMockInputCollector()
	// Don't set a value - should return default

	input := &workflow.Input{
		Name:        "timeout",
		Type:        "integer",
		Description: "Timeout in seconds",
		Required:    false,
		Default:     30,
	}

	value, err := mock.PromptForInput(input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if value != 30 {
		t.Errorf("expected default value 30, got %v", value)
	}
}

// TestMockInputCollector_OptionalInputNoDefault tests mock behavior with optional inputs without defaults.
func TestMockInputCollector_OptionalInputNoDefault(t *testing.T) {
	// F046: US2 - Optional input without default (empty input)
	mock := newMockInputCollector()

	input := &workflow.Input{
		Name:        "notes",
		Type:        "string",
		Description: "Optional notes",
		Required:    false,
	}

	value, err := mock.PromptForInput(input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if value != nil {
		t.Errorf("expected nil for skipped optional, got %v", value)
	}
}

// TestMockInputCollector_EnumInput tests mock behavior with enum-constrained inputs.
func TestMockInputCollector_EnumInput(t *testing.T) {
	// F046: US1-AC2 - Enum input selection
	mock := newMockInputCollector()
	mock.values["environment"] = "staging"

	input := &workflow.Input{
		Name:        "environment",
		Type:        "string",
		Description: "Deployment environment",
		Required:    true,
		Validation: &workflow.InputValidation{
			Enum: []string{"dev", "staging", "prod"},
		},
	}

	value, err := mock.PromptForInput(input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if value != "staging" {
		t.Errorf("expected 'staging', got %v", value)
	}
}

// TestMockInputCollector_ValidationError tests mock behavior when validation fails.
func TestMockInputCollector_ValidationError(t *testing.T) {
	// F046: US3-AC1 - Validation error handling
	mock := newMockInputCollector()
	mock.errors["email"] = errors.New("invalid email format")

	input := &workflow.Input{
		Name:        "email",
		Type:        "string",
		Description: "Email address",
		Required:    true,
		Validation: &workflow.InputValidation{
			Pattern: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$",
		},
	}

	_, err := mock.PromptForInput(input)
	if err == nil {
		t.Error("expected validation error, got nil")
	}
	if err.Error() != "invalid email format" {
		t.Errorf("expected 'invalid email format', got '%v'", err)
	}
}

// TestMockInputCollector_CancellationError tests mock behavior when user cancels.
func TestMockInputCollector_CancellationError(t *testing.T) {
	// F046: US3-AC3 - Graceful cancellation
	mock := newMockInputCollector()
	mock.errors["username"] = errors.New("input cancelled")

	input := &workflow.Input{
		Name:     "username",
		Type:     "string",
		Required: true,
	}

	_, err := mock.PromptForInput(input)
	if err == nil {
		t.Error("expected cancellation error, got nil")
	}
	if err.Error() != "input cancelled" {
		t.Errorf("expected 'input cancelled', got '%v'", err)
	}
}

// TestMockInputCollector_TypeCoercion tests mock behavior with different input types.
func TestMockInputCollector_TypeCoercion(t *testing.T) {
	tests := []struct {
		name      string
		inputType string
		mockValue any
		wantValue any
	}{
		{
			name:      "string type",
			inputType: "string",
			mockValue: "hello",
			wantValue: "hello",
		},
		{
			name:      "integer type",
			inputType: "integer",
			mockValue: 42,
			wantValue: 42,
		},
		{
			name:      "boolean type true",
			inputType: "boolean",
			mockValue: true,
			wantValue: true,
		},
		{
			name:      "boolean type false",
			inputType: "boolean",
			mockValue: false,
			wantValue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockInputCollector()
			mock.values["test_input"] = tt.mockValue

			input := &workflow.Input{
				Name:     "test_input",
				Type:     tt.inputType,
				Required: true,
			}

			value, err := mock.PromptForInput(input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if value != tt.wantValue {
				t.Errorf("expected %v, got %v", tt.wantValue, value)
			}
		})
	}
}

// TestMockInputCollector_MultipleInputs tests mock behavior with sequential input prompts.
func TestMockInputCollector_MultipleInputs(t *testing.T) {
	// F046: US1 - Collect multiple missing inputs
	mock := newMockInputCollector()
	mock.values["name"] = "Alice"
	mock.values["age"] = 30
	mock.values["city"] = "Paris"

	inputs := []*workflow.Input{
		{Name: "name", Type: "string", Required: true},
		{Name: "age", Type: "integer", Required: true},
		{Name: "city", Type: "string", Required: true},
	}

	for _, input := range inputs {
		value, err := mock.PromptForInput(input)
		if err != nil {
			t.Errorf("unexpected error for %s: %v", input.Name, err)
		}
		if value != mock.values[input.Name] {
			t.Errorf("expected %v for %s, got %v", mock.values[input.Name], input.Name, value)
		}
	}

	if mock.callCount != 3 {
		t.Errorf("expected 3 calls, got %d", mock.callCount)
	}
	if len(mock.inputs) != 3 {
		t.Errorf("expected 3 inputs recorded, got %d", len(mock.inputs))
	}
}

// TestMockInputCollector_InputsTracking tests that mock tracks all prompted inputs.
func TestMockInputCollector_InputsTracking(t *testing.T) {
	mock := newMockInputCollector()
	mock.values["input1"] = "value1"
	mock.values["input2"] = "value2"

	input1 := &workflow.Input{Name: "input1", Type: "string", Required: true}
	input2 := &workflow.Input{Name: "input2", Type: "string", Required: true}

	_, _ = mock.PromptForInput(input1)
	_, _ = mock.PromptForInput(input2)

	if len(mock.inputs) != 2 {
		t.Errorf("expected 2 tracked inputs, got %d", len(mock.inputs))
	}
	if mock.inputs[0].Name != "input1" {
		t.Errorf("expected first input name 'input1', got '%s'", mock.inputs[0].Name)
	}
	if mock.inputs[1].Name != "input2" {
		t.Errorf("expected second input name 'input2', got '%s'", mock.inputs[1].Name)
	}
}

// TestMockInputCollector_ValidationConstraints tests mock with various validation rules.
func TestMockInputCollector_ValidationConstraints(t *testing.T) {
	tests := []struct {
		name       string
		input      *workflow.Input
		mockValue  any
		mockError  error
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "pattern validation",
			input: &workflow.Input{
				Name:     "code",
				Type:     "string",
				Required: true,
				Validation: &workflow.InputValidation{
					Pattern: "^[A-Z]{3}$",
				},
			},
			mockValue: "ABC",
			wantErr:   false,
		},
		{
			name: "enum validation",
			input: &workflow.Input{
				Name:     "status",
				Type:     "string",
				Required: true,
				Validation: &workflow.InputValidation{
					Enum: []string{"active", "inactive", "pending"},
				},
			},
			mockValue: "active",
			wantErr:   false,
		},
		{
			name: "min/max validation",
			input: &workflow.Input{
				Name:     "count",
				Type:     "integer",
				Required: true,
				Validation: &workflow.InputValidation{
					Min: intPtr(1),
					Max: intPtr(100),
				},
			},
			mockValue: 50,
			wantErr:   false,
		},
		{
			name: "file exists validation",
			input: &workflow.Input{
				Name:     "config_file",
				Type:     "string",
				Required: true,
				Validation: &workflow.InputValidation{
					FileExists: true,
				},
			},
			mockError:  errors.New("file does not exist"),
			wantErr:    true,
			wantErrMsg: "file does not exist",
		},
		{
			name: "file extension validation",
			input: &workflow.Input{
				Name:     "script",
				Type:     "string",
				Required: true,
				Validation: &workflow.InputValidation{
					FileExtension: []string{".sh", ".bash"},
				},
			},
			mockValue: "script.sh",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockInputCollector()
			if tt.mockError != nil {
				mock.errors[tt.input.Name] = tt.mockError
			} else {
				mock.values[tt.input.Name] = tt.mockValue
			}

			value, err := mock.PromptForInput(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.wantErrMsg != "" && err.Error() != tt.wantErrMsg {
					t.Errorf("expected error '%s', got '%s'", tt.wantErrMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if value != tt.mockValue {
					t.Errorf("expected %v, got %v", tt.mockValue, value)
				}
			}
		})
	}
}

// TestMockInputCollector_EdgeCases tests mock behavior with edge cases.
func TestMockInputCollector_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		input     *workflow.Input
		mockValue any
		wantValue any
		wantErr   bool
	}{
		{
			name: "empty string for optional",
			input: &workflow.Input{
				Name:     "optional_str",
				Type:     "string",
				Required: false,
			},
			mockValue: "",
			wantValue: "",
			wantErr:   false,
		},
		{
			name: "zero integer for optional",
			input: &workflow.Input{
				Name:     "optional_int",
				Type:     "integer",
				Required: false,
			},
			mockValue: 0,
			wantValue: 0,
			wantErr:   false,
		},
		{
			name: "false boolean for optional",
			input: &workflow.Input{
				Name:     "optional_bool",
				Type:     "boolean",
				Required: false,
			},
			mockValue: false,
			wantValue: false,
			wantErr:   false,
		},
		{
			name: "long description",
			input: &workflow.Input{
				Name:        "long_desc",
				Type:        "string",
				Description: "This is a very long description that spans multiple lines and contains detailed information about what this input is used for and why it matters for the workflow execution process",
				Required:    true,
			},
			mockValue: "value",
			wantValue: "value",
			wantErr:   false,
		},
		{
			name: "many enum options",
			input: &workflow.Input{
				Name:     "many_options",
				Type:     "string",
				Required: true,
				Validation: &workflow.InputValidation{
					Enum: []string{"opt1", "opt2", "opt3", "opt4", "opt5", "opt6", "opt7", "opt8", "opt9", "opt10"},
				},
			},
			mockValue: "opt5",
			wantValue: "opt5",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockInputCollector()
			mock.values[tt.input.Name] = tt.mockValue

			value, err := mock.PromptForInput(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if value != tt.wantValue {
					t.Errorf("expected %v, got %v", tt.wantValue, value)
				}
			}
		})
	}
}

// Helper function to create int pointer for min/max validation
func intPtr(i int) *int {
	return &i
}
