//nolint:revive // Package name "errors" is intentional; fully qualified import path avoids stdlib conflict
package errors

// Hint represents an actionable suggestion to help users resolve errors.
// Hints are context-aware messages displayed alongside error output to guide users
// toward successful resolution without requiring external documentation.
//
// Design principles:
//   - Clear: Use plain language, avoid jargon
//   - Actionable: Suggest concrete steps, not vague advice
//   - Concise: One line when possible, max 2-3 lines
//   - Relevant: Generated from error context, not generic messages
//
// Example hints:
//   - "Did you mean 'my-workflow.yaml'?" (Levenshtein suggestion)
//   - "Run 'awf list' to see available workflows"
//   - "Expected: 'version: 1.0'" (format guidance)
//   - "Check file permissions with 'ls -l /path/to/file'"
//
// Hints are presentation-layer concerns and never stored in domain errors.
// Generated at render time by HintGenerator functions in the infrastructure layer.
type Hint struct {
	// Message is the actionable suggestion text.
	// Should be a complete sentence without trailing punctuation (formatter adds styling).
	Message string
}

// HintGenerator is a function that examines a StructuredError and generates
// zero or more actionable hints to help the user resolve the error.
//
// Generators implement typed error detection using errors.As() to extract
// context from specific error types, then produce contextual suggestions.
//
// Design:
//   - Pure functions: no side effects, no shared mutable state
//   - Return empty slice (not nil) when no hints available
//   - Order matters: most relevant hints first
//   - Limit: typically 1-3 hints per generator to avoid overwhelming users
//
// Example generators:
//   - FileNotFoundHintGenerator: reads directory, suggests similar filenames
//   - YAMLSyntaxHintGenerator: extracts line/column from ParseError
//   - InvalidStateHintGenerator: uses Levenshtein to suggest closest state
//   - MissingInputHintGenerator: lists required inputs with examples
//   - CommandFailureHintGenerator: checks permissions, suggests verification
//
// Thread safety: Generators must be safe for concurrent invocation.
// Stateless by design; context comes from StructuredError parameter.
type HintGenerator func(err *StructuredError) []Hint
