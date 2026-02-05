package ports_test

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// Component: T001
// Feature: C049

// =============================================================================
// Mock Implementations
// =============================================================================

// mockStepPresenter is a test implementation of StepPresenter interface
type mockStepPresenter struct {
	showHeaderCalled      bool
	showStepDetailsCalled bool
	showExecutingCalled   bool
	showStepResultCalled  bool
	lastWorkflowName      string
	lastStepInfo          *workflow.InteractiveStepInfo
	lastExecutingStep     string
	lastStepState         *workflow.StepState
	lastNextStep          string
	callCount             int
}

func newMockStepPresenter() *mockStepPresenter {
	return &mockStepPresenter{}
}

func (m *mockStepPresenter) ShowHeader(workflowName string) {
	m.showHeaderCalled = true
	m.lastWorkflowName = workflowName
	m.callCount++
}

func (m *mockStepPresenter) ShowStepDetails(info *workflow.InteractiveStepInfo) {
	m.showStepDetailsCalled = true
	m.lastStepInfo = info
	m.callCount++
}

func (m *mockStepPresenter) ShowExecuting(stepName string) {
	m.showExecutingCalled = true
	m.lastExecutingStep = stepName
	m.callCount++
}

func (m *mockStepPresenter) ShowStepResult(state *workflow.StepState, nextStep string) {
	m.showStepResultCalled = true
	m.lastStepState = state
	m.lastNextStep = nextStep
	m.callCount++
}

// mockStatusPresenter is a test implementation of StatusPresenter interface
type mockStatusPresenter struct {
	showAbortedCalled   bool
	showSkippedCalled   bool
	showCompletedCalled bool
	showErrorCalled     bool
	lastSkippedStep     string
	lastSkippedNext     string
	lastStatus          workflow.ExecutionStatus
	lastError           error
	callCount           int
}

func newMockStatusPresenter() *mockStatusPresenter {
	return &mockStatusPresenter{}
}

func (m *mockStatusPresenter) ShowAborted() {
	m.showAbortedCalled = true
	m.callCount++
}

func (m *mockStatusPresenter) ShowSkipped(stepName, nextStep string) {
	m.showSkippedCalled = true
	m.lastSkippedStep = stepName
	m.lastSkippedNext = nextStep
	m.callCount++
}

func (m *mockStatusPresenter) ShowCompleted(status workflow.ExecutionStatus) {
	m.showCompletedCalled = true
	m.lastStatus = status
	m.callCount++
}

func (m *mockStatusPresenter) ShowError(err error) {
	m.showErrorCalled = true
	m.lastError = err
	m.callCount++
}

// mockUserInteraction is a test implementation of UserInteraction interface
type mockUserInteraction struct {
	promptActionCalled bool
	editInputCalled    bool
	showContextCalled  bool
	lastHasRetry       bool
	lastInputName      string
	lastCurrentValue   any
	lastContext        *workflow.RuntimeContext
	returnAction       workflow.InteractiveAction
	returnActionError  error
	returnInputValue   any
	returnInputError   error
	callCount          int
}

func newMockUserInteraction() *mockUserInteraction {
	return &mockUserInteraction{
		returnAction:     workflow.ActionContinue,
		returnInputValue: "default",
	}
}

func (m *mockUserInteraction) PromptAction(hasRetry bool) (workflow.InteractiveAction, error) {
	m.promptActionCalled = true
	m.lastHasRetry = hasRetry
	m.callCount++
	return m.returnAction, m.returnActionError
}

func (m *mockUserInteraction) EditInput(name string, current any) (any, error) {
	m.editInputCalled = true
	m.lastInputName = name
	m.lastCurrentValue = current
	m.callCount++
	return m.returnInputValue, m.returnInputError
}

func (m *mockUserInteraction) ShowContext(ctx *workflow.RuntimeContext) {
	m.showContextCalled = true
	m.lastContext = ctx
	m.callCount++
}

// mockInteractivePrompt is a test implementation of InteractivePrompt interface (composite)
type mockInteractivePrompt struct {
	*mockStepPresenter
	*mockStatusPresenter
	*mockUserInteraction
}

func newMockInteractivePrompt() *mockInteractivePrompt {
	return &mockInteractivePrompt{
		mockStepPresenter:   newMockStepPresenter(),
		mockStatusPresenter: newMockStatusPresenter(),
		mockUserInteraction: newMockUserInteraction(),
	}
}

// =============================================================================
// Interface Compliance Tests
// =============================================================================

func TestStepPresenterInterface(t *testing.T) {
	// Verify interface compliance
	var _ ports.StepPresenter = (*mockStepPresenter)(nil)
}

func TestStatusPresenterInterface(t *testing.T) {
	// Verify interface compliance
	var _ ports.StatusPresenter = (*mockStatusPresenter)(nil)
}

func TestUserInteractionInterface(t *testing.T) {
	// Verify interface compliance
	var _ ports.UserInteraction = (*mockUserInteraction)(nil)
}

func TestInteractivePromptInterface(t *testing.T) {
	// Verify that composite interface is satisfied by embedding all three
	var _ ports.InteractivePrompt = (*mockInteractivePrompt)(nil)

	// Verify that InteractivePrompt embeds all three focused interfaces
	var _ ports.StepPresenter = ports.InteractivePrompt(nil)
	var _ ports.StatusPresenter = ports.InteractivePrompt(nil)
	var _ ports.UserInteraction = ports.InteractivePrompt(nil)
}

// =============================================================================
// StepPresenter Tests - Happy Path
// =============================================================================

func TestStepPresenter_ShowHeader_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		workflowName string
	}{
		{"simple workflow name", "deploy-app"},
		{"workflow with spaces", "Deploy Production App"},
		{"workflow with dashes", "ci-cd-pipeline"},
		{"workflow with version", "deploy-v1.2.3"},
		{"empty workflow name", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			presenter := newMockStepPresenter()

			// Act
			presenter.ShowHeader(tt.workflowName)

			// Assert
			if !presenter.showHeaderCalled {
				t.Error("ShowHeader should be called")
			}
			if presenter.lastWorkflowName != tt.workflowName {
				t.Errorf("expected workflow name '%s', got '%s'", tt.workflowName, presenter.lastWorkflowName)
			}
			if presenter.callCount != 1 {
				t.Errorf("expected 1 call, got %d", presenter.callCount)
			}
		})
	}
}

func TestStepPresenter_ShowStepDetails_HappyPath(t *testing.T) {
	// Arrange
	presenter := newMockStepPresenter()
	stepInfo := &workflow.InteractiveStepInfo{
		Name:  "validate-input",
		Index: 1,
		Total: 5,
		Step: &workflow.Step{
			Name: "validate-input",
			Type: workflow.StepTypeCommand,
		},
		Command:     "test -f input.txt",
		Transitions: []string{"→ on_success: process", "→ on_failure: error"},
	}

	// Act
	presenter.ShowStepDetails(stepInfo)

	// Assert
	if !presenter.showStepDetailsCalled {
		t.Error("ShowStepDetails should be called")
	}
	if presenter.lastStepInfo != stepInfo {
		t.Error("should store the provided step info")
	}
	if presenter.lastStepInfo.Name != "validate-input" {
		t.Errorf("expected step name 'validate-input', got '%s'", presenter.lastStepInfo.Name)
	}
	if presenter.callCount != 1 {
		t.Errorf("expected 1 call, got %d", presenter.callCount)
	}
}

func TestStepPresenter_ShowExecuting_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		stepName string
	}{
		{"simple step", "build"},
		{"step with dash", "run-tests"},
		{"step with underscore", "deploy_prod"},
		{"long step name", "validate-configuration-and-dependencies"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			presenter := newMockStepPresenter()

			// Act
			presenter.ShowExecuting(tt.stepName)

			// Assert
			if !presenter.showExecutingCalled {
				t.Error("ShowExecuting should be called")
			}
			if presenter.lastExecutingStep != tt.stepName {
				t.Errorf("expected step name '%s', got '%s'", tt.stepName, presenter.lastExecutingStep)
			}
		})
	}
}

func TestStepPresenter_ShowStepResult_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		state    *workflow.StepState
		nextStep string
	}{
		{
			name: "success with output",
			state: &workflow.StepState{
				Status:   workflow.StatusCompleted,
				Output:   "Build completed successfully",
				ExitCode: 0,
			},
			nextStep: "deploy",
		},
		{
			name: "failure with error",
			state: &workflow.StepState{
				Status:   workflow.StatusFailed,
				Output:   "",
				Error:    "command not found",
				ExitCode: 127,
			},
			nextStep: "cleanup",
		},
		{
			name: "success with empty output",
			state: &workflow.StepState{
				Status:   workflow.StatusCompleted,
				Output:   "",
				ExitCode: 0,
			},
			nextStep: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			presenter := newMockStepPresenter()

			// Act
			presenter.ShowStepResult(tt.state, tt.nextStep)

			// Assert
			if !presenter.showStepResultCalled {
				t.Error("ShowStepResult should be called")
			}
			if presenter.lastStepState != tt.state {
				t.Error("should store the provided step state")
			}
			if presenter.lastNextStep != tt.nextStep {
				t.Errorf("expected next step '%s', got '%s'", tt.nextStep, presenter.lastNextStep)
			}
		})
	}
}

func TestStepPresenter_MethodSequence(t *testing.T) {
	// Test typical method call sequence for step lifecycle
	// Arrange
	presenter := newMockStepPresenter()

	// Act - typical sequence
	presenter.ShowHeader("test-workflow")
	presenter.ShowStepDetails(&workflow.InteractiveStepInfo{Name: "step1"})
	presenter.ShowExecuting("step1")
	presenter.ShowStepResult(&workflow.StepState{Status: workflow.StatusCompleted}, "step2")

	// Assert
	if presenter.callCount != 4 {
		t.Errorf("expected 4 calls in sequence, got %d", presenter.callCount)
	}
	if !presenter.showHeaderCalled || !presenter.showStepDetailsCalled ||
		!presenter.showExecutingCalled || !presenter.showStepResultCalled {
		t.Error("all methods should be called in lifecycle sequence")
	}
}

// =============================================================================
// StatusPresenter Tests - Happy Path
// =============================================================================

func TestStatusPresenter_ShowAborted_HappyPath(t *testing.T) {
	// Arrange
	presenter := newMockStatusPresenter()

	// Act
	presenter.ShowAborted()

	// Assert
	if !presenter.showAbortedCalled {
		t.Error("ShowAborted should be called")
	}
	if presenter.callCount != 1 {
		t.Errorf("expected 1 call, got %d", presenter.callCount)
	}
}

func TestStatusPresenter_ShowSkipped_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		stepName string
		nextStep string
	}{
		{"skip to next step", "validation", "deployment"},
		{"skip to terminal", "optional-check", "completed"},
		{"skip with empty next", "cleanup", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			presenter := newMockStatusPresenter()

			// Act
			presenter.ShowSkipped(tt.stepName, tt.nextStep)

			// Assert
			if !presenter.showSkippedCalled {
				t.Error("ShowSkipped should be called")
			}
			if presenter.lastSkippedStep != tt.stepName {
				t.Errorf("expected skipped step '%s', got '%s'", tt.stepName, presenter.lastSkippedStep)
			}
			if presenter.lastSkippedNext != tt.nextStep {
				t.Errorf("expected next step '%s', got '%s'", tt.nextStep, presenter.lastSkippedNext)
			}
		})
	}
}

func TestStatusPresenter_ShowCompleted_HappyPath(t *testing.T) {
	tests := []struct {
		name   string
		status workflow.ExecutionStatus
	}{
		{"success status", workflow.StatusCompleted},
		{"failed status", workflow.StatusFailed},
		{"cancelled status", workflow.StatusCancelled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			presenter := newMockStatusPresenter()

			// Act
			presenter.ShowCompleted(tt.status)

			// Assert
			if !presenter.showCompletedCalled {
				t.Error("ShowCompleted should be called")
			}
			if presenter.lastStatus != tt.status {
				t.Errorf("expected status '%v', got '%v'", tt.status, presenter.lastStatus)
			}
		})
	}
}

func TestStatusPresenter_ShowError_HappyPath(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"simple error", errors.New("validation failed")},
		{"workflow error", errors.New("step execution failed")},
		{"nil error", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			presenter := newMockStatusPresenter()

			// Act
			presenter.ShowError(tt.err)

			// Assert
			if !presenter.showErrorCalled {
				t.Error("ShowError should be called")
			}
			if presenter.lastError != tt.err {
				t.Errorf("expected error '%v', got '%v'", tt.err, presenter.lastError)
			}
		})
	}
}

func TestStatusPresenter_TerminalStateExclusivity(t *testing.T) {
	// Test that each terminal state method can be called independently
	// Arrange
	tests := []struct {
		name   string
		action func(*mockStatusPresenter)
		check  func(*mockStatusPresenter) bool
	}{
		{"aborted only", func(p *mockStatusPresenter) { p.ShowAborted() }, func(p *mockStatusPresenter) bool { return p.showAbortedCalled }},
		{"skipped only", func(p *mockStatusPresenter) { p.ShowSkipped("step", "next") }, func(p *mockStatusPresenter) bool { return p.showSkippedCalled }},
		{"completed only", func(p *mockStatusPresenter) { p.ShowCompleted(workflow.StatusCompleted) }, func(p *mockStatusPresenter) bool { return p.showCompletedCalled }},
		{"error only", func(p *mockStatusPresenter) { p.ShowError(errors.New("test")) }, func(p *mockStatusPresenter) bool { return p.showErrorCalled }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			presenter := newMockStatusPresenter()

			// Act
			tt.action(presenter)

			// Assert
			if !tt.check(presenter) {
				t.Errorf("expected %s to be called", tt.name)
			}
			if presenter.callCount != 1 {
				t.Errorf("expected only 1 call for terminal state, got %d", presenter.callCount)
			}
		})
	}
}

// =============================================================================
// UserInteraction Tests - Happy Path
// =============================================================================

func TestUserInteraction_PromptAction_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		hasRetry     bool
		returnAction workflow.InteractiveAction
	}{
		{"continue without retry", false, workflow.ActionContinue},
		{"continue with retry", true, workflow.ActionContinue},
		{"skip without retry", false, workflow.ActionSkip},
		{"abort with retry", true, workflow.ActionAbort},
		{"inspect without retry", false, workflow.ActionInspect},
		{"edit input with retry", true, workflow.ActionEdit},
		{"retry available", true, workflow.ActionRetry},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			interaction := newMockUserInteraction()
			interaction.returnAction = tt.returnAction

			// Act
			action, err := interaction.PromptAction(tt.hasRetry)
			// Assert
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !interaction.promptActionCalled {
				t.Error("PromptAction should be called")
			}
			if interaction.lastHasRetry != tt.hasRetry {
				t.Errorf("expected hasRetry=%v, got %v", tt.hasRetry, interaction.lastHasRetry)
			}
			if action != tt.returnAction {
				t.Errorf("expected action %v, got %v", tt.returnAction, action)
			}
		})
	}
}

func TestUserInteraction_EditInput_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		inputName    string
		currentValue any
		returnValue  any
	}{
		{"string input", "workflow_name", "old-name", "new-name"},
		{"int input", "max_count", 5, 10},
		{"bool input", "debug", false, true},
		{"empty current", "name", "", "value"},
		{"nil current", "optional", nil, "set"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			interaction := newMockUserInteraction()
			interaction.returnInputValue = tt.returnValue

			// Act
			newValue, err := interaction.EditInput(tt.inputName, tt.currentValue)
			// Assert
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !interaction.editInputCalled {
				t.Error("EditInput should be called")
			}
			if interaction.lastInputName != tt.inputName {
				t.Errorf("expected input name '%s', got '%s'", tt.inputName, interaction.lastInputName)
			}
			if interaction.lastCurrentValue != tt.currentValue {
				t.Errorf("expected current value %v, got %v", tt.currentValue, interaction.lastCurrentValue)
			}
			if newValue != tt.returnValue {
				t.Errorf("expected return value %v, got %v", tt.returnValue, newValue)
			}
		})
	}
}

func TestUserInteraction_ShowContext_HappyPath(t *testing.T) {
	tests := []struct {
		name    string
		context *workflow.RuntimeContext
	}{
		{
			name: "context with inputs",
			context: &workflow.RuntimeContext{
				Inputs: map[string]any{
					"name":  "test-workflow",
					"count": 5,
				},
			},
		},
		{
			name: "context with states",
			context: &workflow.RuntimeContext{
				States: map[string]workflow.RuntimeStepState{
					"step1": {Output: "success", ExitCode: 0},
				},
			},
		},
		{
			name:    "empty context",
			context: &workflow.RuntimeContext{},
		},
		{
			name:    "nil context",
			context: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			interaction := newMockUserInteraction()

			// Act
			interaction.ShowContext(tt.context)

			// Assert
			if !interaction.showContextCalled {
				t.Error("ShowContext should be called")
			}
			if interaction.lastContext != tt.context {
				t.Error("should store the provided context")
			}
		})
	}
}

func TestUserInteraction_InteractiveLoopPattern(t *testing.T) {
	// Test typical interactive loop: prompt → action → possibly edit → possibly inspect
	// Arrange
	interaction := newMockUserInteraction()

	// Act - simulate interactive loop
	_, _ = interaction.PromptAction(true)
	_, _ = interaction.EditInput("name", "old")
	interaction.ShowContext(&workflow.RuntimeContext{})
	_, _ = interaction.PromptAction(false)

	// Assert
	if interaction.callCount != 4 {
		t.Errorf("expected 4 calls in interactive loop, got %d", interaction.callCount)
	}
	if !interaction.promptActionCalled || !interaction.editInputCalled || !interaction.showContextCalled {
		t.Error("all interaction methods should be callable in sequence")
	}
}

// =============================================================================
// InteractivePrompt (Composite) Tests - Happy Path
// =============================================================================

func TestInteractivePrompt_CompositeEmbedding_HappyPath(t *testing.T) {
	// Test that composite interface can access all embedded interface methods
	// Arrange
	prompt := newMockInteractivePrompt()

	// Act - Call methods from all three embedded interfaces
	prompt.ShowHeader("test-workflow")
	prompt.ShowAborted()
	_, _ = prompt.PromptAction(false)

	// Assert
	if !prompt.showHeaderCalled {
		t.Error("StepPresenter methods should be accessible")
	}
	if !prompt.showAbortedCalled {
		t.Error("StatusPresenter methods should be accessible")
	}
	if !prompt.promptActionCalled {
		t.Error("UserInteraction methods should be accessible")
	}
}

func TestInteractivePrompt_AllMethodsAccessible(t *testing.T) {
	// Test that all 11 original methods are accessible through composite
	// Arrange
	prompt := newMockInteractivePrompt()
	stepState := &workflow.StepState{Status: workflow.StatusCompleted}
	ctx := &workflow.RuntimeContext{}

	// Act - Call all 11 methods
	prompt.ShowHeader("workflow")
	prompt.ShowStepDetails(&workflow.InteractiveStepInfo{Name: "step"})
	prompt.ShowExecuting("step")
	prompt.ShowStepResult(stepState, "next")
	prompt.ShowAborted()
	prompt.ShowSkipped("step", "next")
	prompt.ShowCompleted(workflow.StatusCompleted)
	prompt.ShowError(nil)
	_, _ = prompt.PromptAction(false)
	_, _ = prompt.EditInput("name", "value")
	prompt.ShowContext(ctx)

	// Assert - All methods should be called
	if prompt.mockStepPresenter.callCount != 4 {
		t.Errorf("expected 4 StepPresenter calls, got %d", prompt.mockStepPresenter.callCount)
	}
	if prompt.mockStatusPresenter.callCount != 4 {
		t.Errorf("expected 4 StatusPresenter calls, got %d", prompt.mockStatusPresenter.callCount)
	}
	if prompt.mockUserInteraction.callCount != 3 {
		t.Errorf("expected 3 UserInteraction calls, got %d", prompt.mockUserInteraction.callCount)
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestStepPresenter_NilStepInfo(t *testing.T) {
	// Arrange
	presenter := newMockStepPresenter()

	// Act
	presenter.ShowStepDetails(nil)

	// Assert
	if !presenter.showStepDetailsCalled {
		t.Error("ShowStepDetails should handle nil gracefully")
	}
	if presenter.lastStepInfo != nil {
		t.Error("should store nil step info")
	}
}

func TestStepPresenter_NilStepState(t *testing.T) {
	// Arrange
	presenter := newMockStepPresenter()

	// Act
	presenter.ShowStepResult(nil, "next")

	// Assert
	if !presenter.showStepResultCalled {
		t.Error("ShowStepResult should handle nil state gracefully")
	}
	if presenter.lastStepState != nil {
		t.Error("should store nil step state")
	}
}

func TestStatusPresenter_EmptyStrings(t *testing.T) {
	// Arrange
	presenter := newMockStatusPresenter()

	// Act
	presenter.ShowSkipped("", "")

	// Assert
	if !presenter.showSkippedCalled {
		t.Error("ShowSkipped should handle empty strings")
	}
	if presenter.lastSkippedStep != "" || presenter.lastSkippedNext != "" {
		t.Error("should preserve empty string values")
	}
}

func TestUserInteraction_NilCurrentValue(t *testing.T) {
	// Arrange
	interaction := newMockUserInteraction()
	interaction.returnInputValue = "new-value"

	// Act
	newValue, err := interaction.EditInput("name", nil)
	// Assert
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if interaction.lastCurrentValue != nil {
		t.Error("should preserve nil current value")
	}
	if newValue != "new-value" {
		t.Error("should return new value even with nil current")
	}
}

func TestUserInteraction_LargeInputNames(t *testing.T) {
	// Test handling of very long input names
	// Arrange
	interaction := newMockUserInteraction()
	longName := "this_is_a_very_long_input_name_that_might_appear_in_complex_workflows_with_detailed_configuration"

	// Act
	_, _ = interaction.EditInput(longName, "value")

	// Assert
	if interaction.lastInputName != longName {
		t.Error("should handle long input names correctly")
	}
}

func TestStepPresenter_EmptyWorkflowName(t *testing.T) {
	// Arrange
	presenter := newMockStepPresenter()

	// Act
	presenter.ShowHeader("")

	// Assert
	if !presenter.showHeaderCalled {
		t.Error("ShowHeader should handle empty workflow name")
	}
	if presenter.lastWorkflowName != "" {
		t.Error("should preserve empty workflow name")
	}
}

func TestStepPresenter_EmptyStepName(t *testing.T) {
	// Arrange
	presenter := newMockStepPresenter()

	// Act
	presenter.ShowExecuting("")

	// Assert
	if !presenter.showExecutingCalled {
		t.Error("ShowExecuting should handle empty step name")
	}
	if presenter.lastExecutingStep != "" {
		t.Error("should preserve empty step name")
	}
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestUserInteraction_PromptAction_Error(t *testing.T) {
	// Arrange
	interaction := newMockUserInteraction()
	expectedErr := errors.New("user cancelled")
	interaction.returnActionError = expectedErr

	// Act
	action, err := interaction.PromptAction(false)

	// Assert
	if err == nil {
		t.Error("expected error from PromptAction")
	}
	if err != expectedErr {
		t.Errorf("expected error '%v', got '%v'", expectedErr, err)
	}
	if action != workflow.ActionContinue {
		t.Error("should return default action on error")
	}
}

func TestUserInteraction_EditInput_Error(t *testing.T) {
	// Arrange
	interaction := newMockUserInteraction()
	expectedErr := errors.New("invalid input")
	interaction.returnInputError = expectedErr

	// Act
	value, err := interaction.EditInput("name", "current")

	// Assert
	if err == nil {
		t.Error("expected error from EditInput")
	}
	if err != expectedErr {
		t.Errorf("expected error '%v', got '%v'", expectedErr, err)
	}
	if value != "default" {
		t.Error("should return default value on error")
	}
}

func TestUserInteraction_PromptAction_InvalidAction(t *testing.T) {
	// Test handling of invalid/unknown action values
	// Arrange
	interaction := newMockUserInteraction()
	interaction.returnAction = workflow.InteractiveAction("invalid") // invalid action

	// Act
	action, err := interaction.PromptAction(false)
	// Assert
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if action != workflow.InteractiveAction("invalid") {
		t.Error("should return the action value as-is, validation is caller's responsibility")
	}
}

func TestUserInteraction_EditInput_TypeMismatch(t *testing.T) {
	// Test handling when returned value differs from current value type
	// Arrange
	interaction := newMockUserInteraction()
	interaction.returnInputValue = "string-value"

	// Act
	newValue, err := interaction.EditInput("count", 42) // current is int, return is string
	// Assert
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if newValue != "string-value" {
		t.Error("should allow type changes through input editing")
	}
}

// =============================================================================
// Interface Segregation Validation Tests
// =============================================================================

func TestInterfaceSegregation_MethodCounts(t *testing.T) {
	// Verify that each interface has ≤4 methods as per ISP requirement
	tests := []struct {
		name          string
		interfaceType string
		maxMethods    int
	}{
		{"StepPresenter has ≤4 methods", "StepPresenter", 4},
		{"StatusPresenter has ≤4 methods", "StatusPresenter", 4},
		{"UserInteraction has ≤4 methods", "UserInteraction", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test documents the ISP compliance requirement
			// Actual method count validation would require AST parsing (see c049_isp_compliance_test.go)
			// Here we verify that mocks implement the expected methods
			switch tt.interfaceType {
			case "StepPresenter":
				p := newMockStepPresenter()
				p.ShowHeader("")
				p.ShowStepDetails(nil)
				p.ShowExecuting("")
				p.ShowStepResult(nil, "")
				if p.callCount != 4 {
					t.Errorf("StepPresenter should have exactly 4 methods, mock implements %d", p.callCount)
				}
			case "StatusPresenter":
				p := newMockStatusPresenter()
				p.ShowAborted()
				p.ShowSkipped("", "")
				p.ShowCompleted(workflow.StatusCompleted)
				p.ShowError(errors.New("test"))
				if p.callCount != 4 {
					t.Errorf("StatusPresenter should have exactly 4 methods, mock implements %d", p.callCount)
				}
			case "UserInteraction":
				p := newMockUserInteraction()
				_, _ = p.PromptAction(false)
				_, _ = p.EditInput("", nil)
				p.ShowContext(nil)
				if p.callCount != 3 {
					t.Errorf("UserInteraction should have ≤4 methods, mock implements %d", p.callCount)
				}
			}
		})
	}
}

func TestInterfaceSegregation_CompositePreservesBackwardCompatibility(t *testing.T) {
	// Verify that composite interface provides all 11 original methods
	// Arrange
	prompt := newMockInteractivePrompt()

	// Act - count accessible methods by calling all of them
	methodCount := 0

	// StepPresenter methods (4)
	prompt.ShowHeader("")
	methodCount++
	prompt.ShowStepDetails(nil)
	methodCount++
	prompt.ShowExecuting("")
	methodCount++
	prompt.ShowStepResult(nil, "")
	methodCount++

	// StatusPresenter methods (4)
	prompt.ShowAborted()
	methodCount++
	prompt.ShowSkipped("", "")
	methodCount++
	prompt.ShowCompleted(workflow.StatusCompleted)
	methodCount++
	prompt.ShowError(nil)
	methodCount++

	// UserInteraction methods (3)
	_, _ = prompt.PromptAction(false)
	methodCount++
	_, _ = prompt.EditInput("", nil)
	methodCount++
	prompt.ShowContext(nil)
	methodCount++

	// Assert
	if methodCount != 11 {
		t.Errorf("InteractivePrompt composite should expose 11 methods, found %d", methodCount)
	}
}

func TestInterfaceSegregation_FocusedInterfacesIndependent(t *testing.T) {
	// Verify that each focused interface can be used independently
	// Arrange
	stepPresenter := newMockStepPresenter()
	statusPresenter := newMockStatusPresenter()
	userInteraction := newMockUserInteraction()

	// Act - use each interface independently
	stepPresenter.ShowHeader("test")
	statusPresenter.ShowAborted()
	_, _ = userInteraction.PromptAction(false)

	// Assert - each should work without depending on others
	if !stepPresenter.showHeaderCalled {
		t.Error("StepPresenter should work independently")
	}
	if !statusPresenter.showAbortedCalled {
		t.Error("StatusPresenter should work independently")
	}
	if !userInteraction.promptActionCalled {
		t.Error("UserInteraction should work independently")
	}
}

// =============================================================================
// AST-Based Structural Validation (C049 ISP Compliance)
// =============================================================================

// interfaceInfo holds structural information about an interface declaration.
type interfaceInfo struct {
	Name          string
	MethodCount   int
	EmbeddedCount int
}

// TestC049_InterfaceStructure uses AST parsing to verify the structural
// properties of the refactored interfaces. This prevents regression by ensuring
// the interface split maintains the expected method counts.
func TestC049_InterfaceStructure(t *testing.T) {
	// Given: The interactive.go source file
	sourceFile := findInteractiveSourceFile(t)

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, sourceFile, nil, parser.AllErrors)
	require.NoError(t, err, "should parse interactive.go")

	// When: Extracting interface declarations
	interfaces := extractInterfaceDeclarations(node)

	// Then: Should have exactly 4 interfaces (3 focused + 1 composite)
	require.Len(t, interfaces, 4, "should have 4 interface types")

	// And: StepPresenter should have exactly 4 methods
	stepPresenter, exists := interfaces["StepPresenter"]
	require.True(t, exists, "StepPresenter interface should exist")
	assert.Equal(t, 4, stepPresenter.MethodCount,
		"StepPresenter should have 4 methods (ShowHeader, ShowStepDetails, ShowExecuting, ShowStepResult)")

	// And: StatusPresenter should have exactly 4 methods
	statusPresenter, exists := interfaces["StatusPresenter"]
	require.True(t, exists, "StatusPresenter interface should exist")
	assert.Equal(t, 4, statusPresenter.MethodCount,
		"StatusPresenter should have 4 methods (ShowAborted, ShowSkipped, ShowCompleted, ShowError)")

	// And: UserInteraction should have exactly 3 methods
	userInteraction, exists := interfaces["UserInteraction"]
	require.True(t, exists, "UserInteraction interface should exist")
	assert.Equal(t, 3, userInteraction.MethodCount,
		"UserInteraction should have 3 methods (PromptAction, EditInput, ShowContext)")

	// And: InteractivePrompt should embed all 3 focused interfaces (0 direct methods, 3 embedded types)
	interactivePrompt, exists := interfaces["InteractivePrompt"]
	require.True(t, exists, "InteractivePrompt interface should exist")
	assert.Equal(t, 0, interactivePrompt.MethodCount,
		"InteractivePrompt should have 0 direct methods (only embedded interfaces)")
	assert.Equal(t, 3, interactivePrompt.EmbeddedCount,
		"InteractivePrompt should embed exactly 3 interfaces")
}

// TestC049_FocusedInterfacesHaveFewMethods verifies ISP compliance by ensuring
// each focused interface has ≤4 methods.
func TestC049_FocusedInterfacesHaveFewMethods(t *testing.T) {
	// Given: The interactive.go source file
	sourceFile := findInteractiveSourceFile(t)

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, sourceFile, nil, parser.AllErrors)
	require.NoError(t, err, "should parse interactive.go")

	interfaces := extractInterfaceDeclarations(node)

	// When/Then: Each focused interface should have ≤4 methods
	focusedInterfaces := []string{"StepPresenter", "StatusPresenter", "UserInteraction"}
	for _, name := range focusedInterfaces {
		iface, exists := interfaces[name]
		require.True(t, exists, "%s should exist", name)
		assert.LessOrEqual(t, iface.MethodCount, 4,
			"%s should have ≤4 methods (ISP compliance)", name)
	}
}

// TestC049_TotalMethodsPreserved verifies that the refactoring preserves all
// 11 original methods across the three focused interfaces.
func TestC049_TotalMethodsPreserved(t *testing.T) {
	// Given: The interactive.go source file
	sourceFile := findInteractiveSourceFile(t)

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, sourceFile, nil, parser.AllErrors)
	require.NoError(t, err, "should parse interactive.go")

	interfaces := extractInterfaceDeclarations(node)

	// When: Counting total methods across focused interfaces
	totalMethods := 0
	focusedInterfaces := []string{"StepPresenter", "StatusPresenter", "UserInteraction"}
	for _, name := range focusedInterfaces {
		iface, exists := interfaces[name]
		require.True(t, exists, "%s should exist", name)
		totalMethods += iface.MethodCount
	}

	// Then: Should have exactly 11 methods total (4 + 4 + 3)
	assert.Equal(t, 11, totalMethods,
		"total methods across focused interfaces should be 11 (original InteractivePrompt method count)")
}

// =============================================================================
// AST Helper Functions
// =============================================================================

// findInteractiveSourceFile locates the interactive.go source file.
func findInteractiveSourceFile(t *testing.T) string {
	t.Helper()

	// Navigate from test file location to source file
	testDir, err := filepath.Abs(".")
	require.NoError(t, err, "should get test directory")

	sourceFile := filepath.Join(testDir, "interactive.go")
	require.FileExists(t, sourceFile, "interactive.go should exist in same directory as test")

	return sourceFile
}

// extractInterfaceDeclarations parses an AST and extracts all interface
// declarations with their method counts and embedded type counts.
func extractInterfaceDeclarations(node *ast.File) map[string]interfaceInfo {
	interfaces := make(map[string]interfaceInfo)

	ast.Inspect(node, func(n ast.Node) bool {
		// Look for type declarations
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		// Check if it's an interface type
		interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
		if !ok {
			return true
		}

		// Count methods and embedded types
		methodCount := 0
		embeddedCount := 0

		for _, method := range interfaceType.Methods.List {
			if len(method.Names) == 0 {
				// Embedded interface (no method name)
				embeddedCount++
			} else {
				// Regular method
				methodCount++
			}
		}

		interfaces[typeSpec.Name.Name] = interfaceInfo{
			Name:          typeSpec.Name.Name,
			MethodCount:   methodCount,
			EmbeddedCount: embeddedCount,
		}

		return true
	})

	return interfaces
}
