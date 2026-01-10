package ui_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

// =============================================================================
// CLIInputCollector Tests (F046)
// =============================================================================

// TestCLIInputCollector_PromptForInput_RequiredString tests prompting for a required string input.
// Component: cli_input_collector
// Feature: F046
// User Story: US1 - Prompt for Required Inputs
func TestCLIInputCollector_PromptForInput_RequiredString(t *testing.T) {
	// Arrange: Setup stdin with valid string input
	stdin := strings.NewReader("test-value\n")
	stdout := new(bytes.Buffer)
	colorizer := ui.NewColorizer(false)
	collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

	input := &workflow.Input{
		Name:        "project_name",
		Type:        "string",
		Description: "Name of the project",
		Required:    true,
	}

	// Act: Prompt for input
	value, err := collector.PromptForInput(input)

	// Assert: Should return the input value
	require.NoError(t, err)
	assert.Equal(t, "test-value", value, "should return the provided string value")
	assert.Contains(t, stdout.String(), "project_name", "should display input name in prompt")
	assert.Contains(t, stdout.String(), "Name of the project", "should display description")
}

// TestCLIInputCollector_PromptForInput_RequiredInteger tests prompting for a required integer input.
// Component: cli_input_collector
// Feature: F046
// User Story: US1
func TestCLIInputCollector_PromptForInput_RequiredInteger(t *testing.T) {
	// Arrange
	stdin := strings.NewReader("42\n")
	stdout := new(bytes.Buffer)
	colorizer := ui.NewColorizer(false)
	collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

	input := &workflow.Input{
		Name:        "timeout",
		Type:        "integer",
		Description: "Timeout in seconds",
		Required:    true,
	}

	// Act
	value, err := collector.PromptForInput(input)

	// Assert: Should coerce string to integer
	require.NoError(t, err)
	assert.Equal(t, 42, value, "should coerce string '42' to integer 42")
}

// TestCLIInputCollector_PromptForInput_RequiredBoolean tests prompting for a required boolean input.
// Component: cli_input_collector
// Feature: F046
// User Story: US1
func TestCLIInputCollector_PromptForInput_RequiredBoolean(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"true value", "true\n", true},
		{"false value", "false\n", false},
		{"yes as true", "yes\n", true},
		{"no as false", "no\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			stdin := strings.NewReader(tt.input)
			stdout := new(bytes.Buffer)
			colorizer := ui.NewColorizer(false)
			collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

			input := &workflow.Input{
				Name:     "enabled",
				Type:     "boolean",
				Required: true,
			}

			// Act
			value, err := collector.PromptForInput(input)

			// Assert: Should coerce string to boolean
			require.NoError(t, err)
			assert.Equal(t, tt.expected, value, "should coerce string to boolean")
		})
	}
}

// TestCLIInputCollector_PromptForInput_EnumSmall tests enum selection with <= 9 options.
// Component: cli_input_collector
// Feature: F046
// User Story: US1-AC2 - Display enum options
func TestCLIInputCollector_PromptForInput_EnumSmall(t *testing.T) {
	// Arrange: Enum with 3 options, user selects option 2
	stdin := strings.NewReader("2\n")
	stdout := new(bytes.Buffer)
	colorizer := ui.NewColorizer(false)
	collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

	input := &workflow.Input{
		Name:        "environment",
		Type:        "string",
		Description: "Deployment environment",
		Required:    true,
		Validation: &workflow.InputValidation{
			Enum: []string{"dev", "staging", "prod"},
		},
	}

	// Act
	value, err := collector.PromptForInput(input)

	// Assert: Should display numbered list and return selected value
	require.NoError(t, err)
	assert.Equal(t, "staging", value, "should return 'staging' for selection 2")
	assert.Contains(t, stdout.String(), "1. dev", "should display option 1")
	assert.Contains(t, stdout.String(), "2. staging", "should display option 2")
	assert.Contains(t, stdout.String(), "3. prod", "should display option 3")
}

// TestCLIInputCollector_PromptForInput_EnumLarge tests enum with > 9 options (freetext fallback).
// Component: cli_input_collector
// Feature: F046
// User Story: US1
func TestCLIInputCollector_PromptForInput_EnumLarge(t *testing.T) {
	// Arrange: Enum with 12 options, fallback to freetext validation
	stdin := strings.NewReader("option5\n")
	stdout := new(bytes.Buffer)
	colorizer := ui.NewColorizer(false)
	collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

	input := &workflow.Input{
		Name:     "region",
		Type:     "string",
		Required: true,
		Validation: &workflow.InputValidation{
			Enum: []string{
				"us-east-1", "us-west-1", "us-west-2", "eu-west-1", "eu-central-1",
				"ap-southeast-1", "ap-southeast-2", "ap-northeast-1", "option5", "option10", "option11", "option12",
			},
		},
	}

	// Act
	value, err := collector.PromptForInput(input)

	// Assert: Should use freetext with validation for large enums
	require.NoError(t, err)
	assert.Equal(t, "option5", value, "should validate freetext against enum")
	assert.Contains(t, stdout.String(), "Available values:", "should list available enum values")
}

// TestCLIInputCollector_PromptForInput_EnumInvalidThenValid tests retry on invalid enum selection.
// Component: cli_input_collector
// Feature: F046
// User Story: US1-AC3 - Validate enum selection
func TestCLIInputCollector_PromptForInput_EnumInvalidThenValid(t *testing.T) {
	// Arrange: Invalid selection (4) then valid selection (2)
	stdin := strings.NewReader("4\n2\n")
	stdout := new(bytes.Buffer)
	colorizer := ui.NewColorizer(false)
	collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

	input := &workflow.Input{
		Name:     "environment",
		Type:     "string",
		Required: true,
		Validation: &workflow.InputValidation{
			Enum: []string{"dev", "staging", "prod"},
		},
	}

	// Act
	value, err := collector.PromptForInput(input)

	// Assert: Should show error and re-prompt
	require.NoError(t, err)
	assert.Equal(t, "staging", value, "should accept valid selection after error")
	assert.Contains(t, stdout.String(), "Error:", "should display error message for invalid selection")
	assert.Contains(t, stdout.String(), "invalid", "should explain why selection was invalid")
}

// TestCLIInputCollector_PromptForInput_OptionalWithDefault tests skipping optional input with default.
// Component: cli_input_collector
// Feature: F046
// User Story: US2-AC2 - Use default values
func TestCLIInputCollector_PromptForInput_OptionalWithDefault(t *testing.T) {
	// Arrange: Empty input for optional field with default
	stdin := strings.NewReader("\n")
	stdout := new(bytes.Buffer)
	colorizer := ui.NewColorizer(false)
	collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

	input := &workflow.Input{
		Name:        "timeout",
		Type:        "integer",
		Description: "Timeout in seconds",
		Required:    false,
		Default:     30,
	}

	// Act
	value, err := collector.PromptForInput(input)

	// Assert: Should return default value for empty input
	require.NoError(t, err)
	assert.Equal(t, 30, value, "should return default value when input is empty")
	assert.Contains(t, stdout.String(), "default: 30", "should display default value in prompt")
}

// TestCLIInputCollector_PromptForInput_OptionalWithoutDefault tests skipping optional input without default.
// Component: cli_input_collector
// Feature: F046
// User Story: US2-AC1 - Skip optional inputs
func TestCLIInputCollector_PromptForInput_OptionalWithoutDefault(t *testing.T) {
	// Arrange: Empty input for optional field without default
	stdin := strings.NewReader("\n")
	stdout := new(bytes.Buffer)
	colorizer := ui.NewColorizer(false)
	collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

	input := &workflow.Input{
		Name:        "description",
		Type:        "string",
		Description: "Optional description",
		Required:    false,
	}

	// Act
	value, err := collector.PromptForInput(input)

	// Assert: Should return nil for empty optional input without default
	require.NoError(t, err)
	assert.Nil(t, value, "should return nil when optional input is empty and no default")
	assert.Contains(t, stdout.String(), "optional", "should indicate input is optional")
}

// TestCLIInputCollector_PromptForInput_RequiredEmptyThenValid tests retry on empty required input.
// Component: cli_input_collector
// Feature: F046
// User Story: US3-AC2 - Error recovery
func TestCLIInputCollector_PromptForInput_RequiredEmptyThenValid(t *testing.T) {
	// Arrange: Empty input first, then valid input
	stdin := strings.NewReader("\nvalid-value\n")
	stdout := new(bytes.Buffer)
	colorizer := ui.NewColorizer(false)
	collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

	input := &workflow.Input{
		Name:     "project_name",
		Type:     "string",
		Required: true,
	}

	// Act
	value, err := collector.PromptForInput(input)

	// Assert: Should show error and re-prompt
	require.NoError(t, err)
	assert.Equal(t, "valid-value", value, "should accept valid value after error")
	assert.Contains(t, stdout.String(), "Error:", "should display error for empty required input")
	assert.Contains(t, stdout.String(), "required", "should explain that field is required")
}

// TestCLIInputCollector_PromptForInput_ValidationPattern tests pattern validation.
// Component: cli_input_collector
// Feature: F046
// User Story: US3-AC1 - Validation with error messages
func TestCLIInputCollector_PromptForInput_ValidationPattern(t *testing.T) {
	// Arrange: Invalid pattern first, then valid
	stdin := strings.NewReader("abc123\nABCDEF\n")
	stdout := new(bytes.Buffer)
	colorizer := ui.NewColorizer(false)
	collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

	input := &workflow.Input{
		Name:     "code",
		Type:     "string",
		Required: true,
		Validation: &workflow.InputValidation{
			Pattern: "^[A-Z]+$",
		},
	}

	// Act
	value, err := collector.PromptForInput(input)

	// Assert: Should validate pattern and re-prompt on failure
	require.NoError(t, err)
	assert.Equal(t, "ABCDEF", value, "should accept value matching pattern")
	assert.Contains(t, stdout.String(), "Error:", "should display validation error")
	assert.Contains(t, stdout.String(), "pattern", "should mention pattern constraint")
}

// TestCLIInputCollector_PromptForInput_ValidationMinMax tests min/max validation for integers.
// Component: cli_input_collector
// Feature: F046
// User Story: US3-AC1 - Validation with error messages
func TestCLIInputCollector_PromptForInput_ValidationMinMax(t *testing.T) {
	minVal := 1
	maxVal := 100

	// Arrange: Value below min, then above max, then valid
	stdin := strings.NewReader("0\n150\n50\n")
	stdout := new(bytes.Buffer)
	colorizer := ui.NewColorizer(false)
	collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

	input := &workflow.Input{
		Name:     "port",
		Type:     "integer",
		Required: true,
		Validation: &workflow.InputValidation{
			Min: &minVal,
			Max: &maxVal,
		},
	}

	// Act
	value, err := collector.PromptForInput(input)

	// Assert: Should validate range and re-prompt on failure
	require.NoError(t, err)
	assert.Equal(t, 50, value, "should accept value within range")
	output := stdout.String()
	assert.Contains(t, output, "Error:", "should display validation errors")
	assert.Contains(t, output, "minimum", "should mention minimum constraint for value below min")
	assert.Contains(t, output, "maximum", "should mention maximum constraint for value above max")
}

// TestCLIInputCollector_PromptForInput_EOF tests handling of EOF (Ctrl+D).
// Component: cli_input_collector
// Feature: F046
// User Story: US3-AC3 - Graceful cancellation
func TestCLIInputCollector_PromptForInput_EOF(t *testing.T) {
	// Arrange: Empty reader to simulate EOF
	stdin := strings.NewReader("")
	stdout := new(bytes.Buffer)
	colorizer := ui.NewColorizer(false)
	collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

	input := &workflow.Input{
		Name:     "project_name",
		Type:     "string",
		Required: true,
	}

	// Act
	value, err := collector.PromptForInput(input)

	// Assert: Should return error on EOF
	require.Error(t, err)
	assert.Nil(t, value, "should return nil value on EOF")
	assert.Contains(t, err.Error(), "cancelled", "error should indicate cancellation")
}

// TestCLIInputCollector_PromptForInput_TypeCoercionInteger tests type coercion for integers.
// Component: cli_input_collector
// Feature: F046
// User Story: US1
func TestCLIInputCollector_PromptForInput_TypeCoercionInteger(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		expected    any
	}{
		{"valid integer", "123\n", false, 123},
		{"negative integer", "-42\n", false, -42},
		{"zero", "0\n", false, 0},
		{"invalid integer then valid", "abc\n42\n", false, 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			stdin := strings.NewReader(tt.input)
			stdout := new(bytes.Buffer)
			colorizer := ui.NewColorizer(false)
			collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

			input := &workflow.Input{
				Name:     "count",
				Type:     "integer",
				Required: true,
			}

			// Act
			value, err := collector.PromptForInput(input)

			// Assert
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, value, "should coerce to correct integer value")
			}
		})
	}
}

// TestCLIInputCollector_PromptForInput_ValidationFileExists tests file existence validation.
// Component: cli_input_collector
// Feature: F046
// User Story: US3
func TestCLIInputCollector_PromptForInput_ValidationFileExists(t *testing.T) {
	// Arrange: Create temporary file in current directory for testing
	tmpFile := "test_input_file.txt"
	f, err := os.Create(tmpFile)
	require.NoError(t, err)
	f.Close()
	defer os.Remove(tmpFile)

	// Non-existent file then existent file (within allowed directory)
	stdin := strings.NewReader("/nonexistent/file.txt\n" + tmpFile + "\n")
	stdout := new(bytes.Buffer)
	colorizer := ui.NewColorizer(false)
	collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

	input := &workflow.Input{
		Name:     "config_file",
		Type:     "string",
		Required: true,
		Validation: &workflow.InputValidation{
			FileExists: true,
		},
	}

	// Act
	value, err := collector.PromptForInput(input)

	// Assert: Should validate file existence
	require.NoError(t, err)
	assert.Equal(t, tmpFile, value, "should accept existing file")
	assert.Contains(t, stdout.String(), "Error:", "should display error for non-existent file")
	assert.Contains(t, stdout.String(), "file", "should mention file constraint")
}

// TestCLIInputCollector_PromptForInput_ValidationFileExtension tests file extension validation.
// Component: cli_input_collector
// Feature: F046
// User Story: US3
func TestCLIInputCollector_PromptForInput_ValidationFileExtension(t *testing.T) {
	// Arrange: Wrong extension then correct extension
	stdin := strings.NewReader("config.txt\nconfig.yaml\n")
	stdout := new(bytes.Buffer)
	colorizer := ui.NewColorizer(false)
	collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

	input := &workflow.Input{
		Name:     "workflow_file",
		Type:     "string",
		Required: true,
		Validation: &workflow.InputValidation{
			FileExtension: []string{".yaml", ".yml"},
		},
	}

	// Act
	value, err := collector.PromptForInput(input)

	// Assert: Should validate file extension
	require.NoError(t, err)
	assert.Equal(t, "config.yaml", value, "should accept file with valid extension")
	assert.Contains(t, stdout.String(), "Error:", "should display error for invalid extension")
	assert.Contains(t, stdout.String(), "extension", "should mention extension constraint")
}

// TestCLIInputCollector_PromptForInput_DisplayFormatting tests that prompts are properly formatted.
// Component: cli_input_collector
// Feature: F046
// User Story: US1
func TestCLIInputCollector_PromptForInput_DisplayFormatting(t *testing.T) {
	// Arrange
	stdin := strings.NewReader("test\n")
	stdout := new(bytes.Buffer)
	colorizer := ui.NewColorizer(false)
	collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

	input := &workflow.Input{
		Name:        "api_key",
		Type:        "string",
		Description: "API key for authentication",
		Required:    true,
	}

	// Act
	_, err := collector.PromptForInput(input)

	// Assert: Should display well-formatted prompt
	require.NoError(t, err)
	output := stdout.String()
	assert.Contains(t, output, "api_key", "should display input name")
	assert.Contains(t, output, "API key for authentication", "should display description")
	assert.Contains(t, output, "string", "should display type")
	assert.Contains(t, output, "required", "should indicate required status")
}

// TestCLIInputCollector_NewCLIInputCollector tests constructor initialization.
// Component: cli_input_collector
// Feature: F046
func TestCLIInputCollector_NewCLIInputCollector(t *testing.T) {
	// Arrange
	stdin := strings.NewReader("")
	stdout := new(bytes.Buffer)
	colorizer := ui.NewColorizer(false)

	// Act
	collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

	// Assert: Should create non-nil collector
	assert.NotNil(t, collector, "should create collector instance")

	// Verify collector implements InputCollector interface by calling a method
	input := &workflow.Input{
		Name:     "test",
		Type:     "string",
		Required: false,
	}
	stdin = strings.NewReader("\n")
	collector = ui.NewCLIInputCollector(stdin, stdout, colorizer)
	_, err := collector.PromptForInput(input)
	assert.NoError(t, err, "should implement InputCollector interface")
}

// TestCLIInputCollector_PromptForInput_EOFWithoutNewline tests EOF without trailing newline.
// Component: cli_input_collector
// Feature: F046
// User Story: US3-AC3
func TestCLIInputCollector_PromptForInput_EOFWithoutNewline(t *testing.T) {
	// Arrange: Reader returns EOF immediately without any data
	stdin := &errorReader{err: io.EOF}
	stdout := new(bytes.Buffer)
	colorizer := ui.NewColorizer(false)
	collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

	input := &workflow.Input{
		Name:     "project_name",
		Type:     "string",
		Required: true,
	}

	// Act
	value, err := collector.PromptForInput(input)

	// Assert: Should handle EOF gracefully
	require.Error(t, err)
	assert.Nil(t, value, "should return nil on EOF")
	assert.Contains(t, err.Error(), "cancelled", "should indicate cancellation")
}

// errorReader is a helper that always returns the specified error.
type errorReader struct {
	err error
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}

// TestCLIInputCollector_PromptForInput_EnumEdgeCaseExactly9Options tests enum with exactly 9 options.
// Component: cli_input_collector
// Feature: F046
// User Story: US1-AC2
func TestCLIInputCollector_PromptForInput_EnumEdgeCaseExactly9Options(t *testing.T) {
	// Arrange: Enum with exactly 9 options (boundary case)
	stdin := strings.NewReader("9\n")
	stdout := new(bytes.Buffer)
	colorizer := ui.NewColorizer(false)
	collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

	input := &workflow.Input{
		Name:     "option",
		Type:     "string",
		Required: true,
		Validation: &workflow.InputValidation{
			Enum: []string{"opt1", "opt2", "opt3", "opt4", "opt5", "opt6", "opt7", "opt8", "opt9"},
		},
	}

	// Act
	value, err := collector.PromptForInput(input)

	// Assert: Should display numbered list and accept selection 9
	require.NoError(t, err)
	assert.Equal(t, "opt9", value, "should return last option for selection 9")
	assert.Contains(t, stdout.String(), "9. opt9", "should display option 9")
}

// TestCLIInputCollector_PromptForInput_EnumEdgeCase10Options tests enum with 10 options (fallback).
// Component: cli_input_collector
// Feature: F046
// User Story: US1
func TestCLIInputCollector_PromptForInput_EnumEdgeCase10Options(t *testing.T) {
	// Arrange: Enum with 10 options (just over boundary, should use freetext)
	stdin := strings.NewReader("opt5\n")
	stdout := new(bytes.Buffer)
	colorizer := ui.NewColorizer(false)
	collector := ui.NewCLIInputCollector(stdin, stdout, colorizer)

	input := &workflow.Input{
		Name:     "option",
		Type:     "string",
		Required: true,
		Validation: &workflow.InputValidation{
			Enum: []string{"opt1", "opt2", "opt3", "opt4", "opt5", "opt6", "opt7", "opt8", "opt9", "opt10"},
		},
	}

	// Act
	value, err := collector.PromptForInput(input)

	// Assert: Should use freetext validation for 10+ options
	require.NoError(t, err)
	assert.Equal(t, "opt5", value, "should validate freetext against enum")
	assert.NotContains(t, stdout.String(), "5. opt5", "should NOT display numbered list for 10+ options")
}
