package ui

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStdinInputReader_PreflightDoesNotConsumeInput(t *testing.T) {
	reader := NewStdinInputReader(strings.NewReader("hello\n"), io.Discard)
	require.NoError(t, reader.PreflightInput(context.Background()))
	line, err := reader.ReadInput(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "hello", line)
	assert.ErrorIs(t, reader.PreflightInput(context.Background()), io.EOF)
}

func TestStdinInputReader_PreflightRejectsUnavailableSources(t *testing.T) {
	t.Run("empty scripted input", func(t *testing.T) {
		reader := NewStdinInputReader(strings.NewReader(""), io.Discard)
		assert.ErrorIs(t, reader.PreflightInput(context.Background()), io.EOF)
	})
	t.Run("open pipe never blocks preflight", func(t *testing.T) {
		readEnd, writeEnd, err := os.Pipe()
		require.NoError(t, err)
		t.Cleanup(func() { _ = readEnd.Close(); _ = writeEnd.Close() })
		reader := NewStdinInputReader(readEnd, io.Discard)
		assert.ErrorContains(t, reader.PreflightInput(context.Background()), "requires a terminal")
	})
	t.Run("closed file", func(t *testing.T) {
		file, err := os.Open(os.DevNull)
		require.NoError(t, err)
		require.NoError(t, file.Close())
		assert.ErrorContains(t, NewStdinInputReader(file, io.Discard).PreflightInput(context.Background()), "unavailable")
	})
	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		assert.ErrorIs(t, NewStdinInputReader(strings.NewReader("ready"), io.Discard).PreflightInput(ctx), context.Canceled)
	})
}

func TestStdinInputReader_ReadInput_HappyPath(t *testing.T) {
	input := strings.NewReader("hello\n")
	output := &strings.Builder{}

	reader := NewStdinInputReader(input, output)
	line, err := reader.ReadInput(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "hello", line)
	assert.Equal(t, "> ", output.String())
}

func TestStdinInputReader_ReadInput_EmptyInput(t *testing.T) {
	input := strings.NewReader("\n")
	output := &strings.Builder{}

	reader := NewStdinInputReader(input, output)
	line, err := reader.ReadInput(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "", line)
	assert.Equal(t, "> ", output.String())
}

func TestStdinInputReader_ReadInput_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pr, pw := io.Pipe()
	defer pw.Close()

	output := &strings.Builder{}
	reader := NewStdinInputReader(pr, output)

	_, err := reader.ReadInput(ctx)

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
	assert.Equal(t, "> ", output.String())
}

func TestStdinInputReader_ReadInput_ContextDeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	pr, pw := io.Pipe()
	defer pw.Close()

	output := &strings.Builder{}
	reader := NewStdinInputReader(pr, output)

	_, err := reader.ReadInput(ctx)

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
	assert.Equal(t, "> ", output.String())
}

func TestStdinInputReader_ReadInput_IOError(t *testing.T) {
	errReader := &errorReader{}
	output := &strings.Builder{}

	reader := NewStdinInputReader(errReader, output)
	_, err := reader.ReadInput(context.Background())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errReader.err))
	assert.Equal(t, "> ", output.String())
}

func TestStdinInputReader_ReadInput_MultilineText(t *testing.T) {
	input := strings.NewReader("hello world test\n")
	output := &strings.Builder{}

	reader := NewStdinInputReader(input, output)
	line, err := reader.ReadInput(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "hello world test", line)
	assert.Equal(t, "> ", output.String())
}

func TestStdinInputReader_ReadInput_StripsLineEndings(t *testing.T) {
	input := strings.NewReader("hello\r\n")
	output := &strings.Builder{}

	reader := NewStdinInputReader(input, output)
	line, err := reader.ReadInput(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "hello", line)
}

func TestStdinInputReader_ReadInput_WhitespacePreserved(t *testing.T) {
	input := strings.NewReader("  hello  \n")
	output := &strings.Builder{}

	reader := NewStdinInputReader(input, output)
	line, err := reader.ReadInput(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "  hello  ", line)
}

func TestStdinInputReader_ReadInput_EOF(t *testing.T) {
	input := strings.NewReader("")
	output := &strings.Builder{}

	reader := NewStdinInputReader(input, output)
	_, err := reader.ReadInput(context.Background())

	require.Error(t, err)
	assert.True(t, errors.Is(err, io.EOF))
}

// errorReader is a test double that always returns an error on Read.
type errorReader struct {
	err error
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	if r.err == nil {
		r.err = errors.New("read error")
	}
	return 0, r.err
}
