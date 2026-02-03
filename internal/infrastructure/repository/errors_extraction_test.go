package repository

import (
	"errors"
	"testing"
)

func TestWrapParseError_ExtractsLineNumberFromYAMLError(t *testing.T) {
	tests := []struct {
		name       string
		errorMsg   string
		wantLine   int
		wantColumn int
	}{
		{
			name:       "yaml error with line only",
			errorMsg:   "yaml: line 10: mapping values are not allowed in this context",
			wantLine:   10,
			wantColumn: -1,
		},
		{
			name:       "yaml error with line and column",
			errorMsg:   "yaml: line 42: column 15: did not find expected key",
			wantLine:   42,
			wantColumn: 15,
		},
		{
			name:       "yaml error with no line info",
			errorMsg:   "yaml: unmarshal error",
			wantLine:   -1,
			wantColumn: -1,
		},
		{
			name:       "plain error message",
			errorMsg:   "file not found",
			wantLine:   -1,
			wantColumn: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			causeErr := errors.New(tt.errorMsg)
			parseErr := WrapParseError("/path/to/file.yaml", causeErr)

			if parseErr.Line != tt.wantLine {
				t.Errorf("Line = %d, want %d", parseErr.Line, tt.wantLine)
			}

			if parseErr.Column != tt.wantColumn {
				t.Errorf("Column = %d, want %d", parseErr.Column, tt.wantColumn)
			}

			if parseErr.File != "/path/to/file.yaml" {
				t.Errorf("File = %s, want /path/to/file.yaml", parseErr.File)
			}

			if parseErr.Message != tt.errorMsg {
				t.Errorf("Message = %s, want %s", parseErr.Message, tt.errorMsg)
			}
		})
	}
}
