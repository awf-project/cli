package ui

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadLineWithContext_HappyPath(t *testing.T) {
	ctx := context.Background()
	reader := bufio.NewReader(strings.NewReader("hello\n"))

	line, err := readLineWithContext(ctx, reader)

	require.NoError(t, err)
	assert.Equal(t, "hello", line)
}

func TestReadLineWithContext_PreCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	reader := bufio.NewReader(strings.NewReader("hello\n"))

	_, err := readLineWithContext(ctx, reader)

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestReadLineWithContext_EOF(t *testing.T) {
	ctx := context.Background()
	reader := bufio.NewReader(strings.NewReader(""))

	_, err := readLineWithContext(ctx, reader)

	require.Error(t, err)
	assert.True(t, errors.Is(err, io.EOF))
}

func TestReadLineWithContext_MidReadCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	pr, pw := io.Pipe()
	defer pw.Close()

	reader := bufio.NewReader(pr)

	_, err := readLineWithContext(ctx, reader)

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
}
