package application

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
)

const maxExternalFileSize = 1024 * 1024

// allowedAWFPathKeys lists the AWF map keys eligible for local-over-global path resolution.
// Only scripts_dir and prompts_dir can have workflow-local overrides.
var allowedAWFPathKeys = []string{"scripts_dir", "prompts_dir"}

// resolveLocalOverGlobal prefers a workflow-local file over the global XDG path when the
// interpolated path falls under scripts_dir or prompts_dir. Returns the original path otherwise.
func resolveLocalOverGlobal(interpolatedPath, sourceDir string, awfMap map[string]string) string {
	for _, key := range allowedAWFPathKeys {
		globalDir, ok := awfMap[key]
		if !ok || globalDir == "" {
			continue
		}

		suffix, hasPrefix := strings.CutPrefix(interpolatedPath, globalDir+string(filepath.Separator))
		if !hasPrefix {
			continue
		}

		// Derive local subdir name from map key: "scripts_dir" → "scripts"
		// Resolve against parent of sourceDir: .awf/workflows/ → .awf/scripts/
		localSubdir := strings.TrimSuffix(key, "_dir")
		localPath := filepath.Join(filepath.Dir(sourceDir), localSubdir, suffix)

		if _, err := os.Stat(localPath); err == nil {
			return localPath
		}
	}

	return interpolatedPath
}

// extractFilePathSuffix extracts the filename/path component after a directory prefix in a string.
// Returns the suffix and its length in the original string.
func extractFilePathSuffix(remainingStr string) string {
	endIdx := len(remainingStr)
	for i, ch := range remainingStr {
		if ch == ' ' || ch == '"' || ch == '\'' || ch == '|' || ch == '&' {
			endIdx = i
			break
		}
	}
	return remainingStr[:endIdx]
}

// resolveCommandAWFPaths replaces global AWF paths with local equivalents within a command/dir string.
// For each AWF path variable that appears, checks if a local .awf/<type>/ equivalent exists
// and replaces the global path with the local path if it does.
// This handles FR-001/FR-002 for command and dir fields where AWF variables are interpolated.
func resolveCommandAWFPaths(cmd, sourceDir string, awfMap map[string]string) string {
	if cmd == "" || len(awfMap) == 0 {
		return cmd
	}

	result := cmd

	for _, key := range allowedAWFPathKeys {
		globalDir, ok := awfMap[key]
		if !ok || globalDir == "" {
			continue
		}

		localSubdir := strings.TrimSuffix(key, "_dir")
		localDir := filepath.Join(filepath.Dir(sourceDir), localSubdir)

		// Handle case where entire string is just the global directory (e.g., Dir field)
		if result == globalDir {
			if _, err := os.Stat(localDir); err == nil {
				result = localDir
			}
			continue
		}

		// Handle file paths within commands
		result = replaceAWFPathsInString(result, globalDir, localDir)
	}

	return result
}

// replaceAWFPathsInString replaces occurrences of a global AWF path with its local equivalent in a string.
// When a local file does not exist for a given occurrence, that occurrence is skipped and scanning
// continues from after the unresolvable match — so later occurrences are still resolved.
func replaceAWFPathsInString(str, globalDir, localDir string) string {
	globalPrefix := globalDir + string(filepath.Separator)

	// Build the result by scanning left-to-right with an offset so each unresolvable
	// occurrence is passed over rather than causing an early exit.
	var (
		result strings.Builder
		offset int
	)

	for {
		idx := strings.Index(str[offset:], globalPrefix)
		if idx == -1 {
			result.WriteString(str[offset:])
			break
		}

		absIdx := offset + idx
		remainingStr := str[absIdx+len(globalPrefix):]
		suffix := extractFilePathSuffix(remainingStr)

		if suffix == "" {
			result.WriteString(str[offset:])
			break
		}

		localPath := filepath.Join(localDir, suffix)

		if _, err := os.Stat(localPath); err == nil {
			// Replace this occurrence with the local path.
			result.WriteString(str[offset:absIdx])
			result.WriteString(localPath)
			offset = absIdx + len(globalPrefix) + len(suffix)
		} else {
			// Local file absent: keep the global path and advance past this occurrence
			// so subsequent occurrences can still be resolved.
			result.WriteString(str[offset : absIdx+len(globalPrefix)+len(suffix)])
			offset = absIdx + len(globalPrefix) + len(suffix)
		}
	}

	return result.String()
}

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

	resolvedPath := resolveLocalOverGlobal(interpolatedPath, wf.SourceDir, intCtx.AWF)
	if strings.HasPrefix(resolvedPath, "~/") {
		var homeDir string
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand tilde in file path: %w", err)
		}
		resolvedPath = filepath.Join(homeDir, resolvedPath[2:])

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
