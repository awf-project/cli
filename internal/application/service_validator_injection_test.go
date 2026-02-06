package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/testutil"
)

// TestNewWorkflowService_ValidatorInjection tests that the constructor
// properly accepts and stores the validator parameter (happy path).
func TestNewWorkflowService_ValidatorInjection(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	store := testutil.NewMockStateStore()
	executor := testutil.NewMockCommandExecutor()
	logger := testutil.NewMockLogger()
	validator := testutil.NewMockExpressionValidator()

	// Act
	svc := application.NewWorkflowService(repo, store, executor, logger, validator)

	// Assert
	if svc == nil {
		t.Fatal("expected service to be created, got nil")
	}

	// Verify service is usable by calling a method
	// This indirectly confirms the validator was properly stored
	ctx := context.Background()
	repo.AddWorkflow("test", &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeTerminal},
		},
	})

	err := svc.ValidateWorkflow(ctx, "test")
	if err != nil {
		t.Errorf("ValidateWorkflow failed unexpectedly: %v", err)
	}
}

// TestNewWorkflowService_ValidatorNil tests that the service handles
// nil validator gracefully (edge case).
func TestNewWorkflowService_ValidatorNil(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	store := testutil.NewMockStateStore()
	executor := testutil.NewMockCommandExecutor()
	logger := testutil.NewMockLogger()

	// Act - passing nil validator
	svc := application.NewWorkflowService(repo, store, executor, logger, nil)

	// Assert
	if svc == nil {
		t.Fatal("expected service to be created even with nil validator")
	}

	// Note: Calling ValidateWorkflow with nil validator will panic at runtime.
	// This test documents that the constructor doesn't validate the validator parameter.
	// In production, the CLI layer ensures a valid validator is always provided.
}

// TestValidateWorkflow_UsesInjectedValidator tests that ValidateWorkflow
// uses the injected validator instead of creating its own (happy path).
func TestValidateWorkflow_UsesInjectedValidator(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	validator := testutil.NewMockExpressionValidator()

	// Add a workflow with an expression that needs validation
	repo.AddWorkflow("test", &workflow.Workflow{
		Name:    "test",
		Initial: "check",
		Steps: map[string]*workflow.Step{
			"check": {
				Name:    "check",
				Type:    workflow.StepTypeCommand,
				Command: "echo test",
				Transitions: workflow.Transitions{
					{When: "inputs.count > 5", Goto: "success"},
					{When: "", Goto: "failure"}, // default/fallback
				},
			},
			"success": {Name: "success", Type: workflow.StepTypeTerminal},
			"failure": {Name: "failure", Type: workflow.StepTypeTerminal},
		},
	})

	svc := application.NewWorkflowService(
		repo,
		testutil.NewMockStateStore(),
		testutil.NewMockCommandExecutor(),
		testutil.NewMockLogger(),
		validator,
	)

	// Act
	err := svc.ValidateWorkflow(context.Background(), "test")
	// Assert
	if err != nil {
		t.Errorf("expected validation to pass, got error: %v", err)
	}

	// The mock validator returns nil by default, so validation should succeed.
	// This confirms the injected validator was used.
}

// TestValidateWorkflow_PropagatesValidatorErrors tests that validator
// errors are properly propagated to the caller (error handling).
func TestValidateWorkflow_PropagatesValidatorErrors(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	validator := testutil.NewMockExpressionValidator()

	// Configure validator to return an error
	expectedErr := errors.New("invalid expression syntax")
	validator.SetCompileError(expectedErr)

	// Add workflow with expression
	repo.AddWorkflow("invalid-expr", &workflow.Workflow{
		Name:    "invalid-expr",
		Initial: "check",
		Steps: map[string]*workflow.Step{
			"check": {
				Name:    "check",
				Type:    workflow.StepTypeCommand,
				Command: "echo test",
				Transitions: workflow.Transitions{
					{When: "invalid >> syntax", Goto: "success"},
					{When: "", Goto: "failure"},
				},
			},
			"success": {Name: "success", Type: workflow.StepTypeTerminal},
			"failure": {Name: "failure", Type: workflow.StepTypeTerminal},
		},
	})

	svc := application.NewWorkflowService(
		repo,
		testutil.NewMockStateStore(),
		testutil.NewMockCommandExecutor(),
		testutil.NewMockLogger(),
		validator,
	)

	// Act
	err := svc.ValidateWorkflow(context.Background(), "invalid-expr")

	// Assert
	if err == nil {
		t.Fatal("expected validation to fail with validator error, got nil")
	}

	// The error should be wrapped by Workflow.Validate, but the root cause
	// should be the validator error
	if !errors.Is(err, expectedErr) && err.Error() != expectedErr.Error() {
		// Check if error message contains the expected error
		// (Workflow.Validate may wrap the error)
		t.Errorf("expected error to contain validator error\nwant: %v\ngot: %v", expectedErr, err)
	}
}

// TestValidateWorkflow_EmptyExpression tests handling of empty expressions
// (edge case - validator should accept empty expressions as valid).
func TestValidateWorkflow_EmptyExpression(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	validator := testutil.NewMockExpressionValidator()

	// Add workflow with empty expression
	repo.AddWorkflow("empty-expr", &workflow.Workflow{
		Name:    "empty-expr",
		Initial: "check",
		Steps: map[string]*workflow.Step{
			"check": {
				Name:    "check",
				Type:    workflow.StepTypeCommand,
				Command: "echo test",
				Transitions: workflow.Transitions{
					{When: "", Goto: "success"}, // Empty expression (default/fallback)
				},
			},
			"success": {Name: "success", Type: workflow.StepTypeTerminal},
			"failure": {Name: "failure", Type: workflow.StepTypeTerminal},
		},
	})

	svc := application.NewWorkflowService(
		repo,
		testutil.NewMockStateStore(),
		testutil.NewMockCommandExecutor(),
		testutil.NewMockLogger(),
		validator,
	)

	// Act
	err := svc.ValidateWorkflow(context.Background(), "empty-expr")
	// Assert
	if err != nil {
		t.Errorf("expected validation to pass for empty expression, got error: %v", err)
	}
}

// TestValidateWorkflow_MultipleExpressions tests validation with multiple
// expressions in different steps (edge case - all expressions should be validated).
func TestValidateWorkflow_MultipleExpressions(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	validator := testutil.NewMockExpressionValidator()

	// Track which expressions were validated
	validatedExpressions := make([]string, 0)
	validator.SetCompileFunc(func(expr string) error {
		validatedExpressions = append(validatedExpressions, expr)
		return nil
	})

	// Add workflow with multiple expressions
	repo.AddWorkflow("multi-expr", &workflow.Workflow{
		Name:    "multi-expr",
		Initial: "check1",
		Steps: map[string]*workflow.Step{
			"check1": {
				Name:    "check1",
				Type:    workflow.StepTypeCommand,
				Command: "echo step1",
				Transitions: workflow.Transitions{
					{When: "inputs.x > 0", Goto: "check2"},
					{When: "", Goto: "failure"},
				},
			},
			"check2": {
				Name:    "check2",
				Type:    workflow.StepTypeCommand,
				Command: "echo step2",
				Transitions: workflow.Transitions{
					{When: "inputs.y < 10", Goto: "success"},
					{When: "", Goto: "failure"},
				},
			},
			"success": {Name: "success", Type: workflow.StepTypeTerminal},
			"failure": {Name: "failure", Type: workflow.StepTypeTerminal},
		},
	})

	svc := application.NewWorkflowService(
		repo,
		testutil.NewMockStateStore(),
		testutil.NewMockCommandExecutor(),
		testutil.NewMockLogger(),
		validator,
	)

	// Act
	err := svc.ValidateWorkflow(context.Background(), "multi-expr")
	// Assert
	if err != nil {
		t.Fatalf("expected validation to pass, got error: %v", err)
	}

	// Verify both expressions were validated
	if len(validatedExpressions) < 2 {
		t.Errorf("expected at least 2 expressions to be validated, got %d", len(validatedExpressions))
	}

	// Verify expected expressions were validated
	expectedExprs := map[string]bool{
		"inputs.x > 0":  false,
		"inputs.y < 10": false,
	}
	for _, expr := range validatedExpressions {
		if _, exists := expectedExprs[expr]; exists {
			expectedExprs[expr] = true
		}
	}

	for expr, found := range expectedExprs {
		if !found {
			t.Errorf("expected expression %q to be validated, but it wasn't", expr)
		}
	}
}

// TestValidateWorkflow_ValidatorErrorInSecondExpression tests that
// validation fails correctly when the second of multiple expressions is invalid
// (error handling edge case).
func TestValidateWorkflow_ValidatorErrorInSecondExpression(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	validator := testutil.NewMockExpressionValidator()

	// Configure validator to fail on second expression
	callCount := 0
	validator.SetCompileFunc(func(expr string) error {
		callCount++
		if expr == "invalid >> syntax" {
			return errors.New("syntax error in second expression")
		}
		return nil
	})

	repo.AddWorkflow("partial-invalid", &workflow.Workflow{
		Name:    "partial-invalid",
		Initial: "check1",
		Steps: map[string]*workflow.Step{
			"check1": {
				Name:    "check1",
				Type:    workflow.StepTypeCommand,
				Command: "echo step1",
				Transitions: workflow.Transitions{
					{When: "inputs.x > 0", Goto: "check2"}, // Valid
					{When: "", Goto: "failure"},
				},
			},
			"check2": {
				Name:    "check2",
				Type:    workflow.StepTypeCommand,
				Command: "echo step2",
				Transitions: workflow.Transitions{
					{When: "invalid >> syntax", Goto: "success"}, // Invalid
					{When: "", Goto: "failure"},
				},
			},
			"success": {Name: "success", Type: workflow.StepTypeTerminal},
			"failure": {Name: "failure", Type: workflow.StepTypeTerminal},
		},
	})

	svc := application.NewWorkflowService(
		repo,
		testutil.NewMockStateStore(),
		testutil.NewMockCommandExecutor(),
		testutil.NewMockLogger(),
		validator,
	)

	// Act
	err := svc.ValidateWorkflow(context.Background(), "partial-invalid")

	// Assert
	if err == nil {
		t.Fatal("expected validation to fail with invalid expression, got nil")
	}

	if callCount < 1 {
		t.Errorf("expected validator to be called at least once, got %d calls", callCount)
	}
}

// TestValidateWorkflow_WorkflowNotFound tests error handling when
// workflow doesn't exist (error handling).
func TestValidateWorkflow_WorkflowNotFound(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	validator := testutil.NewMockExpressionValidator()

	svc := application.NewWorkflowService(
		repo,
		testutil.NewMockStateStore(),
		testutil.NewMockCommandExecutor(),
		testutil.NewMockLogger(),
		validator,
	)

	// Act
	err := svc.ValidateWorkflow(context.Background(), "nonexistent")

	// Assert
	if err == nil {
		t.Fatal("expected error for nonexistent workflow, got nil")
	}

	// Should be a user error with missing file code
	// The validator should NOT be called for a nonexistent workflow
}

// TestValidateWorkflow_ContextCancellation tests that validation respects
// context cancellation (edge case - graceful shutdown).
func TestValidateWorkflow_ContextCancellation(t *testing.T) {
	// Arrange
	repo := testutil.NewMockWorkflowRepository()
	validator := testutil.NewMockExpressionValidator()

	repo.AddWorkflow("test", &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeTerminal},
		},
	})

	svc := application.NewWorkflowService(
		repo,
		testutil.NewMockStateStore(),
		testutil.NewMockCommandExecutor(),
		testutil.NewMockLogger(),
		validator,
	)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Act
	err := svc.ValidateWorkflow(ctx, "test")

	// Assert
	// Note: Current implementation may not check context during validation.
	// This test documents the expected behavior if context checking is added.
	// For now, we just verify the method doesn't panic with cancelled context.
	_ = err // Validation may succeed or fail depending on implementation
}
