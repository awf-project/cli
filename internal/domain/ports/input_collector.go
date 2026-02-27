package ports

import "github.com/awf-project/cli/internal/domain/workflow"

// InputCollector defines the contract for collecting missing workflow inputs interactively.
// This port is used for pre-execution input collection when required inputs are missing
// from command-line arguments.
//
// Key differences from InteractivePrompt:
//   - InputCollector: Pre-execution input collection (before workflow starts)
//   - InteractivePrompt: During-execution step control (while workflow runs)
//
// Implementation requirements:
//   - Display enum options as numbered list when Validation.Enum is present
//   - Validate input values against workflow.InputValidation constraints
//   - Re-prompt on validation errors with specific error messages
//   - Handle optional inputs by accepting empty values and using defaults
//   - Support graceful cancellation (Ctrl+C, Ctrl+D/EOF)
//
// Example usage:
//
//	collector := cli.NewCLIInputCollector(os.Stdin, os.Stdout, colorizer)
//	value, err := collector.PromptForInput(&workflow.Input{
//	    Name:        "environment",
//	    Type:        "string",
//	    Description: "Deployment environment",
//	    Required:    true,
//	    Validation: &workflow.InputValidation{
//	        Enum: []string{"dev", "staging", "prod"},
//	    },
//	})
//	if err != nil {
//	    return fmt.Errorf("input collection failed: %w", err)
//	}
type InputCollector interface {
	// PromptForInput prompts the user to provide a value for a single workflow input.
	//
	// Behavior by input type:
	//   - Required inputs: Empty value triggers error and re-prompt
	//   - Optional inputs: Empty value returns input.Default or nil
	//   - Enum inputs: Display numbered list (1-9) for selection
	//   - Validated inputs: Apply workflow.InputValidation constraints, re-prompt on error
	//
	// Type coercion is applied based on input.Type:
	//   - "string": Return value as-is
	//   - "integer": Parse string to int
	//   - "boolean": Parse "true"/"false" to bool
	//
	// Returns:
	//   - Validated input value (typed as any for flexibility)
	//   - Error if user cancels (Ctrl+C, EOF) or validation fails repeatedly
	//
	// Implementation notes:
	//   - Check stdin is terminal before calling (fail if non-interactive)
	//   - Handle io.EOF as cancellation, not panic
	//   - Display validation errors with constraint details
	PromptForInput(input *workflow.Input) (any, error)
}
