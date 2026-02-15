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

const maxPromptFileSize = 1024 * 1024 // 1MB

// loadPromptFile loads and interpolates a prompt template file referenced by AgentConfig.PromptFile.
// Returns the fully interpolated prompt content ready for agent execution.
//
// Path resolution order:
//  1. Template interpolation of the path itself (e.g., {{.awf.prompts_dir}}/plan.md)
//  2. Tilde expansion for home directory (~/)
//  3. Relative path resolution against workflow source directory
//
// Applies 1MB size limit (NFR-001) to prevent accidental loading of large files.
func loadPromptFile(
	ctx context.Context,
	promptFile string,
	wf *workflow.Workflow,
	intCtx *interpolation.Context,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	// Step 1: Template interpolation of the path
	resolver := interpolation.NewTemplateResolver()
	interpolatedPath, err := resolver.Resolve(promptFile, intCtx)
	if err != nil {
		return "", fmt.Errorf("interpolate prompt_file path: %w", err)
	}

	// Step 2: Tilde expansion
	expandedPath := interpolatedPath
	if strings.HasPrefix(interpolatedPath, "~/") {
		var homeDir string
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand tilde in prompt_file path: %w", err)
		}
		expandedPath = filepath.Join(homeDir, interpolatedPath[2:])
	}

	// Step 3: Relative path resolution
	resolvedPath := expandedPath
	if !filepath.IsAbs(expandedPath) {
		resolvedPath = filepath.Join(wf.SourceDir, expandedPath)
	}

	// Fallback: if tilde-expanded path doesn't exist, try as relative path
	if strings.HasPrefix(interpolatedPath, "~/") && expandedPath != interpolatedPath {
		if _, statErr := os.Stat(resolvedPath); os.IsNotExist(statErr) {
			fallbackPath := filepath.Join(wf.SourceDir, interpolatedPath[2:])
			if _, statErr = os.Stat(fallbackPath); statErr == nil {
				resolvedPath = fallbackPath
			}
		}
	}

	// Validate file
	fileInfo, err := os.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("prompt file not found: %s", resolvedPath)
		}
		if os.IsPermission(err) {
			return "", fmt.Errorf("permission denied reading prompt file: %s", resolvedPath)
		}
		return "", fmt.Errorf("read prompt file: %w", err)
	}

	if fileInfo.IsDir() {
		return "", fmt.Errorf("prompt_file path is a directory, not a file: %s", resolvedPath)
	}

	if fileInfo.Size() > maxPromptFileSize {
		return "", fmt.Errorf("prompt file exceeds 1MB limit: %s (%d bytes)", resolvedPath, fileInfo.Size())
	}

	// Read file
	file, err := os.Open(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("open prompt file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, maxPromptFileSize))
	if err != nil {
		return "", fmt.Errorf("read prompt file: %w", err)
	}

	return string(content), nil
}
