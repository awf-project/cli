package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// Ensure CLIInputCollector implements InputCollector.
var _ ports.InputCollector = (*CLIInputCollector)(nil)

// CLIInputCollector implements InputCollector for terminal-based input collection.
// It uses bufio.Reader for stdin interaction with support for:
//   - Enum display as numbered lists (1-9)
//   - Validation with error messages and retry
//   - Optional input skipping with default values
//   - Type coercion (string → int, bool)
//   - Graceful cancellation (Ctrl+C, EOF)
type CLIInputCollector struct {
	reader    *bufio.Reader
	writer    io.Writer
	colorizer *Colorizer
}

// NewCLIInputCollector creates a new CLI input collector with the given I/O streams.
// The colorizer parameter controls terminal color output for prompts and errors.
func NewCLIInputCollector(reader io.Reader, writer io.Writer, colorizer *Colorizer) *CLIInputCollector {
	return &CLIInputCollector{
		reader:    bufio.NewReader(reader),
		writer:    writer,
		colorizer: colorizer,
	}
}

// PromptForInput prompts the user to provide a value for a single workflow input.
//
// Behavior:
//   - Display input metadata (name, type, description, required status)
//   - Show numbered list (1-9) for enum inputs with <= 9 options
//   - Show freetext prompt for other inputs or enums with >9 options
//   - Validate input values against workflow.InputValidation constraints
//   - Re-prompt on validation errors with specific error messages
//   - Handle empty input: error for required, default/nil for optional
//   - Apply type coercion based on input.Type (string/integer/boolean)
//   - Detect EOF (Ctrl+D) and return cancellation error
func (c *CLIInputCollector) PromptForInput(input *workflow.Input) (any, error) {
	for {
		// Display prompt
		c.displayPrompt(input)

		// Read line from stdin
		line, err := c.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("input cancelled")
			}
			return nil, err
		}

		value := strings.TrimSpace(line)

		// Handle empty input
		if value == "" {
			if input.Required {
				c.displayError("Error: this field is required")
				continue
			}
			// Optional input
			if input.Default != nil {
				return input.Default, nil
			}
			return nil, nil
		}

		// Handle enum selection
		if input.Validation != nil && len(input.Validation.Enum) > 0 {
			if len(input.Validation.Enum) <= 9 {
				// Numeric selection for small enums
				idx, parseErr := strconv.Atoi(value)
				if parseErr != nil || idx < 1 || idx > len(input.Validation.Enum) {
					c.displayError(fmt.Sprintf("Error: invalid selection, choose 1-%d", len(input.Validation.Enum)))
					continue
				}
				value = input.Validation.Enum[idx-1]
			} else {
				// Freetext validation for large enums
				if !containsString(input.Validation.Enum, value) {
					c.displayError("Error: invalid value, must be one of the available values")
					continue
				}
			}
		}

		// Type coercion
		typed, coerceErr := c.coerceType(value, input.Type)
		if coerceErr != nil {
			c.displayError(fmt.Sprintf("Error: %v", coerceErr))
			continue
		}

		// Validation (skip enum validation since already handled)
		if input.Validation != nil {
			if valErr := c.validate(typed, input); valErr != nil {
				c.displayError(fmt.Sprintf("Error: %v", valErr))
				continue
			}
		}

		return typed, nil
	}
}

// displayPrompt shows the input prompt with metadata.
func (c *CLIInputCollector) displayPrompt(input *workflow.Input) {
	// Name and type
	fmt.Fprintf(c.writer, "%s (%s)", input.Name, input.Type)

	// Required/optional indicator
	if input.Required {
		fmt.Fprintf(c.writer, " [required]")
	} else {
		fmt.Fprintf(c.writer, " [optional]")
	}

	// Description
	if input.Description != "" {
		fmt.Fprintf(c.writer, "\n  %s", input.Description)
	}

	// Default value
	if input.Default != nil {
		fmt.Fprintf(c.writer, " (default: %v)", input.Default)
	}

	// Enum options
	if input.Validation != nil && len(input.Validation.Enum) > 0 {
		if len(input.Validation.Enum) <= 9 {
			// Numbered list for small enums
			fmt.Fprintln(c.writer)
			for i, opt := range input.Validation.Enum {
				fmt.Fprintf(c.writer, "  %d. %s\n", i+1, opt)
			}
		} else {
			// Available values for large enums
			fmt.Fprintf(c.writer, "\n  Available values: %s", strings.Join(input.Validation.Enum, ", "))
		}
	}

	fmt.Fprintf(c.writer, "\n> ")
}

// coerceType converts a string value to the appropriate type.
func (c *CLIInputCollector) coerceType(value string, inputType string) (any, error) {
	switch inputType {
	case "integer":
		return strconv.Atoi(value)
	case "boolean":
		switch strings.ToLower(value) {
		case "true", "yes", "1":
			return true, nil
		case "false", "no", "0":
			return false, nil
		default:
			return nil, fmt.Errorf("invalid boolean value: %s (use true/false, yes/no)", value)
		}
	default: // string
		return value, nil
	}
}

// validate checks the value against input validation constraints.
func (c *CLIInputCollector) validate(value any, input *workflow.Input) error {
	v := input.Validation
	if v == nil {
		return nil
	}

	// Maximum input length check to prevent memory exhaustion (10MB limit)
	const maxInputLength = 10 * 1024 * 1024 // 10MB
	if str, ok := value.(string); ok {
		if len(str) > maxInputLength {
			return fmt.Errorf("input value too large (max %d bytes)", maxInputLength)
		}
	}

	// Pattern validation (for strings)
	if v.Pattern != "" {
		str, ok := value.(string)
		if ok {
			// Timeout protection against ReDoS attacks
			type regexResult struct {
				matched bool
				err     error
			}
			resultChan := make(chan regexResult, 1)

			go func() {
				matched, regexErr := regexp.MatchString(v.Pattern, str)
				resultChan <- regexResult{matched: matched, err: regexErr}
			}()

			select {
			case result := <-resultChan:
				if result.err != nil {
					return fmt.Errorf("invalid pattern: %w", result.err)
				}
				if !result.matched {
					return fmt.Errorf("value does not match pattern %s", v.Pattern)
				}
			case <-time.After(100 * time.Millisecond):
				return fmt.Errorf("pattern validation timeout (possible ReDoS)")
			}
		}
	}

	// Min/Max validation (for integers)
	if v.Min != nil || v.Max != nil {
		intVal, ok := value.(int)
		if ok {
			if v.Min != nil && intVal < *v.Min {
				return fmt.Errorf("value must be at minimum %d", *v.Min)
			}
			if v.Max != nil && intVal > *v.Max {
				return fmt.Errorf("value must be at maximum %d", *v.Max)
			}
		}
	}

	// File exists validation
	if v.FileExists {
		str, ok := value.(string)
		if ok {
			// Prevent path traversal: restrict to current directory and ./configs
			allowedBases := []string{".", "./configs"}
			allowed := false

			for _, base := range allowedBases {
				absBase, absErr := filepath.Abs(base)
				if absErr != nil {
					continue
				}
				absPath, absErr := filepath.Abs(str)
				if absErr != nil {
					return fmt.Errorf("invalid path: %s", str)
				}

				rel, relErr := filepath.Rel(absBase, absPath)
				if relErr != nil {
					continue
				}

				// Reject paths starting with ".." (path traversal attempt)
				if !strings.HasPrefix(rel, "..") && !strings.HasPrefix(rel, string(filepath.Separator)) {
					allowed = true
					break
				}
			}

			if !allowed {
				return fmt.Errorf("path not allowed (must be in current directory or ./configs): %s", str)
			}

			if _, statErr := os.Stat(str); os.IsNotExist(statErr) {
				return fmt.Errorf("file does not exist: %s", str)
			}
		}
	}

	// File extension validation
	if len(v.FileExtension) > 0 {
		str, ok := value.(string)
		if ok {
			valid := false
			for _, ext := range v.FileExtension {
				if strings.HasSuffix(str, ext) {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("file extension must be one of: %s", strings.Join(v.FileExtension, ", "))
			}
		}
	}

	return nil
}

// displayError shows an error message.
func (c *CLIInputCollector) displayError(msg string) {
	fmt.Fprintln(c.writer, msg)
}

// containsString checks if a slice contains a string.
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
