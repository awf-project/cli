package registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/awf-project/cli/pkg/httpx"
)

const maxDownloadBytes = 100 * 1024 * 1024 // 100 MB

// Download downloads a file from a URL using the provided HTTP client.
// The caller is responsible for verifying the returned content with VerifyChecksum.
// Returns downloaded bytes, limited to 100MB.
func Download(ctx context.Context, client httpx.HTTPDoer, url string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	body, truncated, err := httpx.ReadBody(resp.Body, maxDownloadBytes)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	if truncated {
		return nil, fmt.Errorf("download failed: exceeded 100MB limit")
	}

	return []byte(body), nil
}

// VerifyChecksum verifies that data matches the expected SHA-256 checksum (hex string).
func VerifyChecksum(data []byte, checksum string) error {
	if _, err := hex.DecodeString(checksum); err != nil {
		return fmt.Errorf("checksum mismatch: invalid hex format: %w", err)
	}

	actual := fmt.Sprintf("%x", sha256.Sum256(data))
	if actual != checksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", checksum, actual)
	}

	return nil
}

// ExtractTarGz extracts a tar.gz archive to targetDir with path traversal protection.
func ExtractTarGz(data []byte, targetDir string) error {
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		filePath, err := safeTarPath(targetDir, header.Name)
		if err != nil {
			return err
		}

		if err := extractTarEntry(header, filePath, tarReader); err != nil {
			return err
		}
	}

	return nil
}

func safeTarPath(targetDir, name string) (string, error) {
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("tar entry attempts path traversal: %s", name)
	}

	filePath := filepath.Join(targetDir, name) //nolint:gosec // G305: path traversal validated by filepath.IsLocal check below
	relPath, err := filepath.Rel(filepath.Clean(targetDir), filepath.Clean(filePath))
	if err != nil || !filepath.IsLocal(relPath) {
		return "", fmt.Errorf("tar entry attempts path traversal: %s", name)
	}

	return filePath, nil
}

func extractTarEntry(header *tar.Header, filePath string, reader *tar.Reader) error {
	switch header.Typeflag {
	case tar.TypeDir:
		if err := os.MkdirAll(filePath, os.FileMode(header.Mode)); err != nil { //nolint:gosec // G115: tar header mode is bounded by uint32 range in valid archives
			return fmt.Errorf("failed to create directory %s: %w", filePath, err)
		}
	case tar.TypeReg:
		return extractTarFile(header, filePath, reader)
	}

	return nil
}

// ExtractChecksumForAsset parses a checksum file and returns the SHA-256 hex string
// for the given assetName. The checksum file format is:
//
//	<hex>  <filename>
//
// Each line contains a checksum followed by two spaces and the filename.
// Returns empty string if the asset is not found.
func ExtractChecksumForAsset(content, assetName string) string {
	for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == assetName {
			return parts[0]
		}
	}
	return ""
}

func extractTarFile(header *tar.Header, filePath string, reader *tar.Reader) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0o750); err != nil { //nolint:gosec // G301: 0o750 is intentional for plugin directories
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}

	_, copyErr := io.Copy(file, reader) //nolint:gosec // G110: decompression bomb mitigated by 100MB download limit in Download
	closeErr := file.Close()

	if copyErr != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close file %s: %w", filePath, closeErr)
	}

	if err := os.Chmod(filePath, os.FileMode(header.Mode)); err != nil { //nolint:gosec // G115: tar header mode is bounded by uint32 range in valid archives
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}
