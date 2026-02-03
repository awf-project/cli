package errfmt

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fatih/color"
	domainerrors "github.com/vanoix/awf/internal/domain/errors"
	"github.com/vanoix/awf/internal/domain/ports"
)

// HumanErrorFormatter implements the ErrorFormatter port interface, providing
// human-readable CLI output for structured errors with color and formatting.
//
// Output format:
//
//	[USER.INPUT.MISSING_FILE] workflow file not found
//	  Details:
//	    path: /workflow.yaml
//	  Hint: Did you mean 'my-workflow.yaml'?
//
// Usage:
//
//	formatter := NewHumanErrorFormatter(true, false, generators...)
//	output := formatter.FormatError(structuredErr)
//	fmt.Println(output)
//
// Component: C047 Structured Error Codes Taxonomy (extended by C048)
// Layer: Infrastructure
type HumanErrorFormatter struct {
	colorEnabled bool
	noHints      bool
	generators   []domainerrors.HintGenerator
}

// Compile-time assertion that HumanErrorFormatter implements ports.ErrorFormatter
var _ ports.ErrorFormatter = (*HumanErrorFormatter)(nil)

// NewHumanErrorFormatter creates a new HumanErrorFormatter instance.
//
// Parameters:
//   - colorEnabled: Whether to enable colored output
//   - noHints: Whether to suppress hint generation
//   - generators: Optional hint generators (if nil or empty, no hints generated)
//
// Returns:
//   - A new HumanErrorFormatter ready to format structured errors with optional hints
//
// Example:
//
//	formatter := NewHumanErrorFormatter(true, false, FileNotFoundHintGenerator)
//	output := formatter.FormatError(err)
func NewHumanErrorFormatter(colorEnabled, noHints bool, generators ...domainerrors.HintGenerator) *HumanErrorFormatter {
	return &HumanErrorFormatter{
		colorEnabled: colorEnabled,
		noHints:      noHints,
		generators:   generators,
	}
}

// FormatError converts a StructuredError into human-readable string representation.
//
// Implements the ErrorFormatter port interface. Returns a formatted string with:
//   - Error code prefix in brackets (e.g., "[USER.INPUT.MISSING_FILE]")
//   - Human-readable message
//   - Details section with key-value pairs (if present)
//   - Color coding (if enabled): red for error code, normal for message
//
// Parameters:
//   - err: The structured error to format
//
// Returns:
//   - Human-readable string representation of the error
//
// Example:
//
//	formatter := NewHumanErrorFormatter(true)
//	err := domainerrors.NewStructuredError(
//	    domainerrors.ErrorCodeUserInputMissingFile,
//	    "workflow file not found",
//	    map[string]any{"path": "/workflow.yaml"},
//	    nil,
//	)
//	output := formatter.FormatError(err)
//	// [USER.INPUT.MISSING_FILE] workflow file not found
//	//   Details:
//	//     path: /workflow.yaml
func (f *HumanErrorFormatter) FormatError(err *domainerrors.StructuredError) string {
	var builder strings.Builder

	// Create color helper
	red := color.New(color.FgRed)
	if !f.colorEnabled {
		color.NoColor = true
		defer func() { color.NoColor = false }()
	}

	// Format error code in brackets with color
	errorCodeStr := fmt.Sprintf("[%s]", err.Code)
	if f.colorEnabled {
		errorCodeStr = red.Sprint(errorCodeStr)
	}

	// Write error code and message
	builder.WriteString(errorCodeStr)
	if err.Message != "" {
		builder.WriteString(" ")
		builder.WriteString(err.Message)
	}

	// Add details section if present and non-empty
	if len(err.Details) > 0 {
		builder.WriteString("\n  Details:")

		// Sort keys for deterministic output
		keys := make([]string, 0, len(err.Details))
		for k := range err.Details {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// Format each detail entry
		for _, key := range keys {
			value := err.Details[key]
			builder.WriteString("\n    ")
			builder.WriteString(key)
			builder.WriteString(": ")
			builder.WriteString(formatValue(value))
		}
	}

	// Add hints section if not suppressed and generators available
	if !f.noHints && len(f.generators) > 0 {
		hints := f.generateHints(err)
		if len(hints) > 0 {
			f.renderHints(&builder, hints)
		}
	}

	return builder.String()
}

// formatValue converts any value to a string representation for human readability.
func formatValue(v any) string {
	if v == nil {
		return "<nil>"
	}

	switch val := v.(type) {
	case string:
		return val
	case []string:
		return fmt.Sprintf("[%s]", strings.Join(val, ", "))
	case []int:
		parts := make([]string, len(val))
		for i, n := range val {
			parts[i] = fmt.Sprintf("%d", n)
		}
		return fmt.Sprintf("[%s]", strings.Join(parts, ", "))
	case []any:
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = formatValue(item)
		}
		return fmt.Sprintf("[%s]", strings.Join(parts, ", "))
	case map[string]any:
		// Format nested maps compactly
		var builder strings.Builder
		builder.WriteString("{")
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				builder.WriteString(", ")
			}
			builder.WriteString(k)
			builder.WriteString(": ")
			builder.WriteString(formatValue(val[k]))
		}
		builder.WriteString("}")
		return builder.String()
	default:
		// Use %v for all other types (int, float, bool, etc.)
		return fmt.Sprintf("%v", v)
	}
}

// generateHints invokes all registered hint generators and collects their suggestions.
// Returns empty slice if no hints are generated.
//
// Implementation for C048 T008 - iterates through all generators and collects hints.
func (f *HumanErrorFormatter) generateHints(err *domainerrors.StructuredError) []domainerrors.Hint {
	var allHints []domainerrors.Hint

	// Iterate through all generators and collect hints
	for _, generator := range f.generators {
		if generator == nil {
			continue
		}

		hints := generator(err)
		if len(hints) > 0 {
			allHints = append(allHints, hints...)
		}
	}

	return allHints
}

// renderHints formats and appends hint messages to the output with dim styling.
// Each hint is prefixed with "Hint:" and rendered on a separate line.
//
// Implementation for C048 T008 - renders hints with dim styling.
func (f *HumanErrorFormatter) renderHints(builder *strings.Builder, hints []domainerrors.Hint) {
	if len(hints) == 0 {
		return
	}

	// Create dim color helper for hints
	dim := color.New(color.Faint)
	if !f.colorEnabled {
		color.NoColor = true
		defer func() { color.NoColor = false }()
	}

	// Render each hint
	for _, hint := range hints {
		builder.WriteString("\n  ")
		hintText := "Hint: " + hint.Message
		if f.colorEnabled {
			hintText = dim.Sprint(hintText)
		}
		builder.WriteString(hintText)
	}
}
