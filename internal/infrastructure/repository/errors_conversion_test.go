package repository

import (
	"testing"

	domerrors "github.com/vanoix/awf/internal/domain/errors"
)

func TestParseError_ToStructuredError_ExtractsLineNumber(t *testing.T) {
	// Create a ParseError with line info extracted from yaml.v3 message
	yamlErr := &ParseError{
		File:    "/path/to/file.yaml",
		Line:    10,
		Column:  5,
		Message: "yaml: line 10: column 5: mapping values are not allowed in this context",
		Cause:   nil,
	}

	// Convert to StructuredError
	structErr := yamlErr.ToStructuredError()

	// Verify conversion
	if structErr.Code != domerrors.ErrorCodeWorkflowParseYAMLSyntax {
		t.Errorf("Expected code %v, got %v", domerrors.ErrorCodeWorkflowParseYAMLSyntax, structErr.Code)
	}

	// Verify line number is preserved
	if line, ok := structErr.Details["line"]; !ok {
		t.Error("Line number not in Details")
	} else if line != 10 {
		t.Errorf("Expected line 10, got %v", line)
	}

	// Verify column number is preserved
	if column, ok := structErr.Details["column"]; !ok {
		t.Error("Column number not in Details")
	} else if column != 5 {
		t.Errorf("Expected column 5, got %v", column)
	}

	// Verify file is preserved
	if file, ok := structErr.Details["file"]; !ok {
		t.Error("File not in Details")
	} else if file != "/path/to/file.yaml" {
		t.Errorf("Expected file '/path/to/file.yaml', got %v", file)
	}
}
