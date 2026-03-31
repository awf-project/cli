package registry_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/awf-project/cli/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHTTPDoer for Download tests
type mockDownloadDoer struct {
	statusCode int
	body       []byte
	err        error
}

func (m *mockDownloadDoer) Do(req *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}

	return &http.Response{
		StatusCode: m.statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(m.body)),
	}, nil
}

// ============================================================================
// Download Tests
// ============================================================================

func TestDownload_HappyPath(t *testing.T) {
	content := []byte("plugin binary content")
	mockDoer := &mockDownloadDoer{
		statusCode: 200,
		body:       content,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := registry.Download(ctx, mockDoer, "https://example.com/plugin.tar.gz")

	require.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestDownload_NetworkError(t *testing.T) {
	mockDoer := &mockDownloadDoer{
		err: fmt.Errorf("connection refused"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := registry.Download(ctx, mockDoer, "https://example.com/plugin.tar.gz")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed")
}

func TestDownload_HTTPErrorStatus(t *testing.T) {
	mockDoer := &mockDownloadDoer{
		statusCode: 404,
		body:       []byte("Not Found"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := registry.Download(ctx, mockDoer, "https://example.com/nonexistent.tar.gz")

	require.Error(t, err)
}

func TestDownload_Exceeds100MBLimit(t *testing.T) {
	largeContent := make([]byte, 101*1024*1024) // 101 MB
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	mockDoer := &mockDownloadDoer{
		statusCode: 200,
		body:       largeContent,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := registry.Download(ctx, mockDoer, "https://example.com/huge.tar.gz")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "100MB")
}

func TestDownload_ContextCancellation(t *testing.T) {
	mockDoer := &mockDownloadDoer{
		statusCode: 200,
		body:       []byte("data"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := registry.Download(ctx, mockDoer, "https://example.com/plugin.tar.gz")

	require.Error(t, err)
}

// ============================================================================
// VerifyChecksum Tests
// ============================================================================

func TestVerifyChecksum_HappyPath(t *testing.T) {
	data := []byte("plugin binary content")
	checksum := fmt.Sprintf("%x", sha256.Sum256(data))

	err := registry.VerifyChecksum(data, checksum)

	assert.NoError(t, err)
}

func TestVerifyChecksum_ChecksumMismatch(t *testing.T) {
	data := []byte("plugin binary content")
	wrongChecksum := "0000000000000000000000000000000000000000000000000000000000000000"

	err := registry.VerifyChecksum(data, wrongChecksum)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "mismatch")
}

func TestVerifyChecksum_EmptyData(t *testing.T) {
	data := []byte{}
	checksum := fmt.Sprintf("%x", sha256.Sum256(data))

	err := registry.VerifyChecksum(data, checksum)

	assert.NoError(t, err)
}

func TestVerifyChecksum_InvalidChecksumFormat(t *testing.T) {
	data := []byte("plugin binary content")
	invalidChecksum := "not-hex-string"

	err := registry.VerifyChecksum(data, invalidChecksum)

	require.Error(t, err)
}

// ============================================================================
// ExtractTarGz Tests
// ============================================================================

func TestExtractTarGz_HappyPath(t *testing.T) {
	targetDir := t.TempDir()

	// Create a simple tar.gz with one file
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	content := []byte("test file content")
	header := &tar.Header{
		Name: "test.txt",
		Mode: 0o644,
		Size: int64(len(content)),
	}
	require.NoError(t, tarWriter.WriteHeader(header))
	_, err := tarWriter.Write(content)
	require.NoError(t, err)

	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())

	err = registry.ExtractTarGz(buf.Bytes(), targetDir)

	require.NoError(t, err)

	// Verify file was extracted
	filePath := filepath.Join(targetDir, "test.txt")
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestExtractTarGz_WithDirectory(t *testing.T) {
	targetDir := t.TempDir()

	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	// Create directory entry
	dirHeader := &tar.Header{
		Name:     "subdir",
		Mode:     0o755,
		Typeflag: tar.TypeDir,
	}
	require.NoError(t, tarWriter.WriteHeader(dirHeader))

	// Create file in subdirectory
	content := []byte("nested content")
	fileHeader := &tar.Header{
		Name: "subdir/nested.txt",
		Mode: 0o644,
		Size: int64(len(content)),
	}
	require.NoError(t, tarWriter.WriteHeader(fileHeader))
	_, err := tarWriter.Write(content)
	require.NoError(t, err)

	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())

	err = registry.ExtractTarGz(buf.Bytes(), targetDir)

	require.NoError(t, err)

	filePath := filepath.Join(targetDir, "subdir", "nested.txt")
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestExtractTarGz_InvalidGzipData(t *testing.T) {
	targetDir := t.TempDir()

	invalidGzipData := []byte("not a gzip file")

	err := registry.ExtractTarGz(invalidGzipData, targetDir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "gzip")
}

func TestExtractTarGz_InvalidTarData(t *testing.T) {
	targetDir := t.TempDir()

	// Create valid gzip but invalid tar
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	_, err := gzipWriter.Write([]byte("not a tar file"))
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())

	err = registry.ExtractTarGz(buf.Bytes(), targetDir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tar")
}

func TestExtractTarGz_PathTraversalAttempt(t *testing.T) {
	targetDir := t.TempDir()

	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	// Attempt to escape target directory
	content := []byte("malicious content")
	header := &tar.Header{
		Name: "../escape.txt",
		Mode: 0o644,
		Size: int64(len(content)),
	}
	require.NoError(t, tarWriter.WriteHeader(header))
	_, err := tarWriter.Write(content)
	require.NoError(t, err)

	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())

	err = registry.ExtractTarGz(buf.Bytes(), targetDir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "traversal")
}

func TestExtractTarGz_AbsolutePathAttempt(t *testing.T) {
	targetDir := t.TempDir()

	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	// Attempt to use absolute path
	content := []byte("malicious content")
	header := &tar.Header{
		Name: "/etc/evil.txt",
		Mode: 0o644,
		Size: int64(len(content)),
	}
	require.NoError(t, tarWriter.WriteHeader(header))
	_, err := tarWriter.Write(content)
	require.NoError(t, err)

	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())

	err = registry.ExtractTarGz(buf.Bytes(), targetDir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "traversal")
}

func TestExtractTarGz_EmptyArchive(t *testing.T) {
	targetDir := t.TempDir()

	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)
	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())

	err := registry.ExtractTarGz(buf.Bytes(), targetDir)

	assert.NoError(t, err)
}

func TestExtractTarGz_MultipleFiles(t *testing.T) {
	targetDir := t.TempDir()

	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	files := map[string][]byte{
		"file1.txt": []byte("content 1"),
		"file2.txt": []byte("content 2"),
		"file3.txt": []byte("content 3"),
	}

	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		require.NoError(t, tarWriter.WriteHeader(header))
		_, err := tarWriter.Write(content)
		require.NoError(t, err)
	}

	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())

	err := registry.ExtractTarGz(buf.Bytes(), targetDir)

	require.NoError(t, err)

	for name, expectedContent := range files {
		filePath := filepath.Join(targetDir, name)
		data, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, expectedContent, data)
	}
}
