package audit

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileAuditTrailWriterClosePreservesUnderlyingCloseErrorThenBecomesIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	writer, err := NewFileAuditTrailWriter(path)
	require.NoError(t, err)

	// A real file that has already been closed is a controlled os.File close
	// failure. It exercises the writer's public Close contract without adding a
	// production-only test seam.
	failedFile, err := os.Open(path)
	require.NoError(t, err)
	require.NoError(t, failedFile.Close())
	writer.file = failedFile

	firstErr := writer.Close()
	require.Error(t, firstErr)
	assert.True(t, errors.Is(firstErr, os.ErrClosed), "first close must return the underlying close error")
	assert.NoError(t, writer.Close(), "subsequent Close calls must be idempotent")
}
