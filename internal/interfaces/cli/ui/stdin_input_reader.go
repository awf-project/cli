package ui

import (
	"bufio"
	"context"
	"fmt"
	"io"

	"github.com/awf-project/cli/internal/domain/ports"
)

var _ ports.UserInputReader = (*StdinInputReader)(nil)

// StdinInputReader implements UserInputReader for terminal-based conversation input.
type StdinInputReader struct {
	reader *bufio.Reader
	writer io.Writer
}

// NewStdinInputReader creates a StdinInputReader reading from r and writing the prompt to w.
func NewStdinInputReader(r io.Reader, w io.Writer) *StdinInputReader {
	return &StdinInputReader{
		reader: bufio.NewReader(r),
		writer: w,
	}
}

// ReadInput prints "> " and reads one line from stdin.
// Returns empty string when the user submits no input (signals conversation end).
// Returns error on context cancellation or I/O failure.
func (s *StdinInputReader) ReadInput(ctx context.Context) (string, error) {
	_, _ = fmt.Fprint(s.writer, "> ")
	return readLineWithContext(ctx, s.reader)
}
