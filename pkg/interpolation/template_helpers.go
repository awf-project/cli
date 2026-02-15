package interpolation

import (
	"fmt"
	"io"
	"os"
	"strings"
)

//nolint:unused // used in readFileTemplateFunc
const maxFileReadSize = 1 << 20 // 1MB

// splitTemplateFunc splits a string by delimiter into a slice.
// Each element is trimmed of leading/trailing whitespace.
//
//nolint:unused // registered in TemplateResolver FuncMap
func splitTemplateFunc(s, delimiter string) []string {
	parts := strings.Split(s, delimiter)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// joinTemplateFunc joins a slice of strings with a separator.
//
//nolint:unused // registered in TemplateResolver FuncMap
func joinTemplateFunc(items []string, separator string) string {
	return strings.Join(items, separator)
}

// readFileTemplateFunc reads a file's contents, limited to 1MB.
//
//nolint:unused // registered in TemplateResolver FuncMap
func readFileTemplateFunc(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %q: %w", path, err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("failed to read file %q: path is a directory", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %q: %w", path, err)
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, maxFileReadSize+1))
	if err != nil {
		return "", fmt.Errorf("failed to read file %q: %w", path, err)
	}

	if len(content) > maxFileReadSize {
		return "", fmt.Errorf("file %q exceeds 1MB size limit", path)
	}

	return string(content), nil
}

// trimSpaceTemplateFunc removes leading and trailing whitespace.
//
//nolint:unused // registered in TemplateResolver FuncMap
func trimSpaceTemplateFunc(s string) string {
	return strings.TrimSpace(s)
}
