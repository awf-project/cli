package ui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	"github.com/awf-project/cli/internal/domain/ports"
)

var _ ports.UserInputReader = (*StdinInputReader)(nil)

// StdinInputReader implements UserInputReader for terminal-based conversation input.
type StdinInputReader struct {
	source io.Reader
	reader *bufio.Reader
	writer io.Writer
}

// NewStdinInputReader creates a StdinInputReader reading from r and writing the prompt to w.
func NewStdinInputReader(r io.Reader, w io.Writer) *StdinInputReader {
	return &StdinInputReader{
		source: r,
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

// PreflightInput rejects unusable CLI stdin without reading or waiting on a pipe.
// Explicit in-memory readers remain supported for scripted input and tests.
func (s *StdinInputReader) PreflightInput(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.source == nil {
		return errors.New("stdin is unavailable")
	}
	if s.reader.Buffered() > 0 {
		return nil
	}
	if file, ok := s.source.(*os.File); ok {
		if _, err := file.Stat(); err != nil {
			return fmt.Errorf("stdin is unavailable: %w", err)
		}
		if !term.IsTerminal(int(file.Fd())) { //nolint:gosec // G115: file descriptors fit in int on supported platforms.
			return fmt.Errorf("conversation input requires a terminal; stdin is redirected or closed")
		}
		return nil
	}
	if remaining, ok := s.source.(interface{ Len() int }); ok && remaining.Len() == 0 {
		return io.EOF
	}
	return nil
}
