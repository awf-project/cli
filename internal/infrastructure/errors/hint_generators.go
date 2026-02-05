// Package errfmt provides infrastructure implementations for error formatting and hint generation.
// This package implements the error formatting ports defined in the domain layer.
package errfmt

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vanoix/awf/internal/domain/errors"
	"github.com/vanoix/awf/pkg/stringutil"
)

// FileNotFoundHintGenerator examines file-not-found errors and generates actionable hints
// such as "did you mean?" suggestions for similar files and commands to list available files.
//
// Detection:
//   - Matches errors with code USER.INPUT.MISSING_FILE
//   - Extracts file path from error Details["path"]
//
// Hints generated:
//   - Similar filename suggestions (using Levenshtein distance)
//   - Command to list available files (e.g., "Run 'awf list' to see workflows")
//   - Working directory verification prompt
//
// Implementation notes:
//   - Uses os.ReadDir to scan directory for similar files
//   - Returns empty slice if directory read fails (graceful degradation)
//   - Limits suggestions to 3 closest matches to avoid overwhelming user
//   - Thread-safe: no shared mutable state
func FileNotFoundHintGenerator(err *errors.StructuredError) []errors.Hint {
	// Defensive: handle nil error
	if err == nil {
		return []errors.Hint{}
	}

	// Only handle USER.INPUT.MISSING_FILE errors
	if err.Code != errors.ErrorCodeUserInputMissingFile {
		return []errors.Hint{}
	}

	// Extract path from error details
	if err.Details == nil {
		return []errors.Hint{}
	}

	pathValue, exists := err.Details["path"]
	if !exists {
		return []errors.Hint{}
	}

	// Ensure path is a string
	path, ok := pathValue.(string)
	if !ok || path == "" {
		return []errors.Hint{}
	}

	// Extract directory and filename
	dir := filepath.Dir(path)
	missingFilename := filepath.Base(path)

	// Read directory to find similar files
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		// Directory doesn't exist or can't be read - return generic hints
		return []errors.Hint{
			{Message: "Run 'awf list' to see available workflows"},
		}
	}

	// Find similar YAML files
	type candidate struct {
		filename string
		distance int
	}
	candidates := make([]candidate, 0, len(entries))

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		// Only consider YAML files
		ext := strings.ToLower(filepath.Ext(filename))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		// Calculate distance
		distance := stringutil.LevenshteinDistance(missingFilename, filename)
		candidates = append(candidates, candidate{
			filename: filename,
			distance: distance,
		})
	}

	// Sort by distance (closest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].distance < candidates[j].distance
	})

	// Build hints
	var hints []errors.Hint

	// Add "did you mean?" suggestions for top 3 closest matches
	maxSuggestions := 3
	for i, c := range candidates {
		if i >= maxSuggestions {
			break
		}
		// Only suggest if reasonably similar (distance < 50% of filename length)
		maxDistance := len(missingFilename) / 2
		if c.distance <= maxDistance {
			hints = append(hints, errors.Hint{
				Message: "Did you mean '" + c.filename + "'?",
			})
		}
	}

	// Always suggest 'awf list' command if no similar files found
	if len(hints) == 0 {
		hints = append(hints, errors.Hint{
			Message: "Run 'awf list' to see available workflows",
		})
	}

	// Limit total hints to 3
	if len(hints) > 3 {
		hints = hints[:3]
	}

	return hints
}

// InvalidStateHintGenerator examines invalid state reference errors and generates
// actionable hints such as "did you mean?" suggestions for similar state names.
//
// Detection:
//   - Matches errors with code WORKFLOW.VALIDATION.MISSING_STATE
//   - Extracts referenced state name from error Details["state"]
//   - Extracts available states from error Details["available_states"]
//
// Hints generated:
//   - Similar state name suggestions (using Levenshtein distance)
//   - List of all available states if no close match found
//
// Implementation notes:
//   - Uses Levenshtein distance to find closest matching state names
//   - Returns empty slice if no state context available
//   - Limits suggestions to 3 closest matches to avoid overwhelming user
//   - Thread-safe: no shared mutable state
func InvalidStateHintGenerator(err *errors.StructuredError) []errors.Hint {
	// Defensive: handle nil error
	if err == nil {
		return []errors.Hint{}
	}

	// Only handle WORKFLOW.VALIDATION.MISSING_STATE errors
	if err.Code != errors.ErrorCodeWorkflowValidationMissingState {
		return []errors.Hint{}
	}

	// Extract state and available_states from error details
	if err.Details == nil {
		return []errors.Hint{}
	}

	// Extract missing state name
	stateValue, stateExists := err.Details["state"]
	if !stateExists {
		return []errors.Hint{}
	}

	// Ensure state is a string
	missingState, ok := stateValue.(string)
	if !ok || missingState == "" {
		return []errors.Hint{}
	}

	// Extract available states
	availableStatesValue, availableExists := err.Details["available_states"]
	if !availableExists {
		return []errors.Hint{}
	}

	// Ensure available_states is a slice of strings
	availableStatesInterface, ok := availableStatesValue.([]any)
	if !ok {
		// Try []string type assertion as well
		availableStatesStr, ok := availableStatesValue.([]string)
		if !ok {
			return []errors.Hint{}
		}
		// Convert []string to []any for unified handling
		availableStatesInterface = make([]any, len(availableStatesStr))
		for i, s := range availableStatesStr {
			availableStatesInterface[i] = s
		}
	}

	// Convert to []string, validating each element
	availableStates := make([]string, 0, len(availableStatesInterface))
	for _, stateInterface := range availableStatesInterface {
		stateStr, ok := stateInterface.(string)
		if !ok {
			// Skip non-string elements
			continue
		}
		availableStates = append(availableStates, stateStr)
	}

	// If no valid states found, return empty
	if len(availableStates) == 0 {
		return []errors.Hint{}
	}

	// Calculate Levenshtein distance for each available state
	type candidate struct {
		stateName string
		distance  int
	}
	candidates := make([]candidate, 0, len(availableStates))

	for _, stateName := range availableStates {
		distance := stringutil.LevenshteinDistance(missingState, stateName)
		candidates = append(candidates, candidate{
			stateName: stateName,
			distance:  distance,
		})
	}

	// Sort by distance (closest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].distance < candidates[j].distance
	})

	// Build hints
	var hints []errors.Hint

	// Add "did you mean?" suggestions for top 3 closest matches
	maxSuggestions := 3
	for i, c := range candidates {
		if i >= maxSuggestions {
			break
		}
		// Only suggest if reasonably similar (distance < 50% of state name length)
		maxDistance := len(missingState) / 2
		if c.distance <= maxDistance {
			hints = append(hints, errors.Hint{
				Message: "Did you mean '" + c.stateName + "'?",
			})
		}
	}

	// If no close matches, list all available states
	if len(hints) == 0 {
		// Build a list message
		statesMessage := "Available states: " + strings.Join(availableStates, ", ")
		hints = append(hints, errors.Hint{
			Message: statesMessage,
		})
	}

	// Limit total hints to 3
	if len(hints) > 3 {
		hints = hints[:3]
	}

	return hints
}

// YAMLSyntaxHintGenerator examines YAML parsing errors and generates actionable hints
// such as line/column pointers and expected format guidance.
//
// Detection:
//   - Matches errors with code WORKFLOW.PARSE.YAML_SYNTAX
//   - Extracts line/column information from error Details
//
// Hints generated:
//   - Line and column position of syntax error
//   - Expected YAML format guidance
//   - Common YAML syntax mistake suggestions
//
// Implementation notes:
//   - Extracts line/column from error details if available
//   - Returns generic YAML syntax guidance if no position info
//   - Thread-safe: no shared mutable state
func YAMLSyntaxHintGenerator(err *errors.StructuredError) []errors.Hint {
	// Defensive: handle nil error
	if err == nil {
		return []errors.Hint{}
	}

	// Only handle WORKFLOW.PARSE.YAML_SYNTAX errors
	if err.Code != errors.ErrorCodeWorkflowParseYAMLSyntax {
		return []errors.Hint{}
	}

	var hints []errors.Hint

	// Extract line and column information if available
	var line, column int
	var hasLine, hasColumn bool

	if err.Details != nil {
		// Extract line with type conversion
		if lineValue, exists := err.Details["line"]; exists && lineValue != nil {
			switch v := lineValue.(type) {
			case int:
				line = v
				hasLine = true
			case float64:
				line = int(v)
				hasLine = true
			}
		}

		// Extract column with type conversion
		if columnValue, exists := err.Details["column"]; exists && columnValue != nil {
			switch v := columnValue.(type) {
			case int:
				column = v
				hasColumn = true
			case float64:
				column = int(v)
				hasColumn = true
			}
		}
	}

	// Add position-specific hint if line/column available
	switch {
	case hasLine && hasColumn:
		hints = append(hints, errors.Hint{
			Message: "Check line " + formatInt(line) + ", column " + formatInt(column) + " in your YAML file",
		})
	case hasLine:
		hints = append(hints, errors.Hint{
			Message: "Check line " + formatInt(line) + " in your YAML file",
		})
	case hasColumn:
		hints = append(hints, errors.Hint{
			Message: "Check column " + formatInt(column) + " in your YAML file",
		})
	}

	// Add common YAML syntax tips
	hints = append(hints,
		errors.Hint{
			Message: "Common YAML issues: ensure proper indentation with spaces (not tabs)",
		},
		errors.Hint{
			Message: "Check that colons have spaces after them (key: value, not key:value)",
		},
		errors.Hint{
			Message: "Verify list items start with a dash followed by a space (- item)",
		},
	)

	// Limit to 5 hints maximum
	if len(hints) > 5 {
		hints = hints[:5]
	}

	return hints
}

// MissingInputHintGenerator examines missing input variable errors and generates
// actionable hints such as listing required inputs and providing example values.
//
// Detection:
//   - Matches errors with code USER.INPUT.VALIDATION_FAILED
//   - Extracts missing input variable name from error Details["input"]
//   - Extracts required inputs list from error Details["required_inputs"]
//
// Hints generated:
//   - Message indicating which input is missing
//   - List of all required inputs with example values
//   - Command example showing how to provide inputs
//
// Implementation notes:
//   - Extracts input name and required inputs from error details
//   - Returns generic hint if no input context available
//   - Thread-safe: no shared mutable state
func MissingInputHintGenerator(err *errors.StructuredError) []errors.Hint {
	// Defensive: handle nil error
	if err == nil {
		return []errors.Hint{}
	}

	// Only handle USER.INPUT.VALIDATION_FAILED errors
	if err.Code != errors.ErrorCodeUserInputValidationFailed {
		return []errors.Hint{}
	}

	// Extract data from error details
	if err.Details == nil {
		return []errors.Hint{}
	}

	// Extract missing and required inputs
	missingInputs := extractMissingInputs(err.Details)
	requiredInputs := extractRequiredInputs(err.Details)

	// Build hints
	return buildMissingInputHints(missingInputs, requiredInputs)
}

// extractMissingInputs extracts missing input names from error details
func extractMissingInputs(details map[string]any) []string {
	var missingInputs []string

	// Try single missing_input first
	if missingInputValue, exists := details["missing_input"]; exists {
		if missingInput, ok := missingInputValue.(string); ok && missingInput != "" {
			missingInputs = append(missingInputs, missingInput)
		}
	}

	// Try multiple missing_inputs
	if missingInputsValue, exists := details["missing_inputs"]; exists {
		switch v := missingInputsValue.(type) {
		case []string:
			missingInputs = append(missingInputs, v...)
		case []any:
			for _, item := range v {
				if str, ok := item.(string); ok && str != "" {
					missingInputs = append(missingInputs, str)
				}
			}
		}
	}

	return missingInputs
}

// extractRequiredInputs extracts required input names from error details
func extractRequiredInputs(details map[string]any) []string {
	var requiredInputs []string

	if requiredInputsValue, exists := details["required_inputs"]; exists {
		switch v := requiredInputsValue.(type) {
		case []string:
			requiredInputs = v
		case []any:
			for _, item := range v {
				if str, ok := item.(string); ok && str != "" {
					requiredInputs = append(requiredInputs, str)
				}
			}
		}
	}

	return requiredInputs
}

// buildMissingInputHints constructs hint messages from missing and required inputs
func buildMissingInputHints(missingInputs, requiredInputs []string) []errors.Hint {
	hints := []errors.Hint{}

	// If we have specific missing input(s), mention them
	if len(missingInputs) > 0 {
		if len(missingInputs) == 1 {
			hints = append(hints, errors.Hint{
				Message: "Missing required input: '" + missingInputs[0] + "'",
			})
		} else {
			hints = append(hints, errors.Hint{
				Message: "Missing required inputs: " + strings.Join(missingInputs, ", "),
			})
		}
	}

	// If we have required inputs, list them
	if len(requiredInputs) > 0 {
		hints = append(hints, errors.Hint{
			Message: "Required inputs: " + strings.Join(requiredInputs, ", "),
		})
	}

	// Provide example usage
	if len(missingInputs) > 0 {
		exampleInput := missingInputs[0]
		hints = append(hints, errors.Hint{
			Message: "Example: awf run workflow.yaml --input " + exampleInput + "=<value>",
		})
	} else if len(requiredInputs) > 0 {
		exampleInput := requiredInputs[0]
		hints = append(hints, errors.Hint{
			Message: "Example: awf run workflow.yaml --input " + exampleInput + "=<value>",
		})
	}

	// Provide example values
	if len(missingInputs) > 0 {
		hints = append(hints, errors.Hint{
			Message: "Provide values for all required inputs using --input flags",
		})
	}

	// Limit total hints to 5
	if len(hints) > 5 {
		hints = hints[:5]
	}

	return hints
}

// formatInt converts an integer to a string for display
func formatInt(n int) string {
	// Simple conversion - using a manual implementation to avoid importing strconv
	if n == 0 {
		return "0"
	}

	// Handle negative numbers
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}

	// Convert to string by building digits in reverse
	var digits []byte
	for n > 0 {
		digit := n % 10
		digits = append(digits, byte('0'+digit))
		n /= 10
	}

	// Reverse the digits
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}

	// Add negative sign if needed
	if negative {
		return "-" + string(digits)
	}

	return string(digits)
}

// CommandFailureHintGenerator examines command execution errors and generates
// actionable hints such as permission checks and command availability verification.
//
// Detection:
//   - Matches errors with code EXECUTION.COMMAND.FAILED
//   - Extracts exit code from error Details["exit_code"]
//   - Extracts command from error Details["command"]
//
// Hints generated:
//   - Permission checks for exit code 126
//   - Command not found suggestions for exit code 127
//   - Generic troubleshooting for other exit codes
//
// Implementation notes:
//   - Uses exit code to determine specific failure mode
//   - Returns empty slice if no command context available
//   - Thread-safe: no shared mutable state
func CommandFailureHintGenerator(err *errors.StructuredError) []errors.Hint {
	// Defensive: handle nil error
	if err == nil {
		return []errors.Hint{}
	}

	// Only handle EXECUTION.COMMAND.FAILED errors
	if err.Code != errors.ErrorCodeExecutionCommandFailed {
		return []errors.Hint{}
	}

	// Extract exit code and command from error details
	if err.Details == nil {
		return []errors.Hint{}
	}

	// If Details is empty, return empty hints
	if len(err.Details) == 0 {
		return []errors.Hint{}
	}

	// Extract exit code with type conversion
	var exitCode int
	var hasExitCode bool

	if exitCodeValue, exists := err.Details["exit_code"]; exists && exitCodeValue != nil {
		switch v := exitCodeValue.(type) {
		case int:
			exitCode = v
			hasExitCode = true
		case float64:
			exitCode = int(v)
			hasExitCode = true
		case string:
			// Handle string exit codes by attempting conversion
			// For robustness, but don't fail if conversion doesn't work
			_ = v // Skip string conversion as it's not a common case
		}
	}

	// Extract command (optional, for context)
	var command string
	if commandValue, exists := err.Details["command"]; exists && commandValue != nil {
		if cmdStr, ok := commandValue.(string); ok {
			command = cmdStr
		}
	}

	// Build hints based on exit code
	// Always return non-nil slice for consistency
	hints := []errors.Hint{}

	if !hasExitCode {
		// No exit code available, return empty (non-nil) hints
		return hints
	}

	// Generate specific hints based on exit code
	switch exitCode {
	case 126:
		// Permission denied
		hints = append(hints,
			errors.Hint{
				Message: "Permission denied: check if the command has execute permissions",
			},
			errors.Hint{
				Message: "Verify you have permission to execute the command",
			},
		)
		if command != "" {
			hints = append(hints, errors.Hint{
				Message: "Try: chmod +x " + command,
			})
		}

	case 127:
		// Command not found
		hints = append(hints,
			errors.Hint{
				Message: "Command not found: the command may not be installed or not in PATH",
			},
			errors.Hint{
				Message: "Check if the command is installed and available in your PATH",
			},
		)
		if command != "" {
			// Extract just the command name (first word)
			cmdName := extractCommandName(command)
			hints = append(hints, errors.Hint{
				Message: "Try: which " + cmdName,
			})
		}

	case 130:
		// SIGINT (Ctrl+C)
		hints = append(hints,
			errors.Hint{
				Message: "Command was interrupted (exit code 130 = SIGINT)",
			},
			errors.Hint{
				Message: "The command may have been cancelled by the user or a signal",
			},
		)

	case 137:
		// SIGKILL (out of memory or killed)
		hints = append(hints,
			errors.Hint{
				Message: "Command was killed (exit code 137 = SIGKILL)",
			},
			errors.Hint{
				Message: "This may indicate an out-of-memory condition or resource limit",
			},
		)

	case 2:
		// Misuse of shell builtin
		hints = append(hints,
			errors.Hint{
				Message: "Misuse of shell builtin (exit code 2)",
			},
			errors.Hint{
				Message: "Check command syntax and arguments",
			},
		)

	case 0:
		// Exit code 0 (success) in error context is unusual
		// Still provide hints for clarity
		hints = append(hints, errors.Hint{
			Message: "Command reported exit code 0 (success) in an error context",
		})

	default:
		// Generic hints for all other exit codes
		hints = append(hints, errors.Hint{
			Message: "Command failed with exit code " + formatInt(exitCode),
		})
		if command != "" {
			hints = append(hints, errors.Hint{
				Message: "Check command output and logs for details",
			})
		}
	}

	// Limit total hints to 5
	if len(hints) > 5 {
		hints = hints[:5]
	}

	return hints
}

// extractCommandName extracts the first word from a command string
// e.g., "docker-compose up" -> "docker-compose"
func extractCommandName(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}

	// Find first space or special character
	for i, ch := range command {
		if ch == ' ' || ch == '\t' || ch == '\n' {
			return command[:i]
		}
	}

	return command
}
