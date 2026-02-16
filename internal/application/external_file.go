package application

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/pkg/interpolation"
)

const maxExternalFileSize = 1024 * 1024

// loadExternalFile loads file contents with path resolution and 1MB size limit.
// Shared by loadPromptFile and loadScriptFile.
func loadExternalFile(
	ctx context.Context,
	filePath string,
	wf *workflow.Workflow,
	intCtx *interpolation.Context,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	resolver := interpolation.NewTemplateResolver()
	interpolatedPath, err := resolver.Resolve(filePath, intCtx)
	if err != nil {
		return "", fmt.Errorf("interpolate file path: %w", err)
	}

	resolvedPath := interpolatedPath
	if strings.HasPrefix(interpolatedPath, "~/") {
		var homeDir string
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand tilde in file path: %w", err)
		}
		resolvedPath = filepath.Join(homeDir, interpolatedPath[2:])

		// Convenience: if ~/path doesn't exist in $HOME, try workflow.SourceDir/path instead.
		// Allows users to write "~/scripts/foo.sh" for workflow-relative files.
		if _, statErr := os.Stat(resolvedPath); os.IsNotExist(statErr) {
			fallbackPath := filepath.Join(wf.SourceDir, interpolatedPath[2:])
			if _, statErr = os.Stat(fallbackPath); statErr == nil {
				resolvedPath = fallbackPath
			}
		}
	} else if !filepath.IsAbs(resolvedPath) {
		resolvedPath = filepath.Join(wf.SourceDir, resolvedPath)
	}

	fileInfo, err := os.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", resolvedPath)
		}
		if os.IsPermission(err) {
			return "", fmt.Errorf("permission denied reading file: %s", resolvedPath)
		}
		return "", fmt.Errorf("read file: %w", err)
	}

	if fileInfo.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s", resolvedPath)
	}

	if fileInfo.Size() > maxExternalFileSize {
		return "", fmt.Errorf("file exceeds 1MB limit: %s (%d bytes)", resolvedPath, fileInfo.Size())
	}
	file, err := os.Open(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, maxExternalFileSize))
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	return string(content), nil
}
