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

const (
	maxExternalFileSize = 1024 * 1024
	// AWFPackNameKey is the awfMap key used to inject pack context into path resolution.
	// When set, resolveLocalOverGlobal and resolveCommandAWFPaths apply 3-tier pack resolution.
	AWFPackNameKey = "pack_name"
)

// allowedAWFPathKeys lists the AWF map keys eligible for local-over-global path resolution.
// Only scripts_dir and prompts_dir can have workflow-local overrides.
var allowedAWFPathKeys = []string{"scripts_dir", "prompts_dir"}

// resolveLocalOverGlobal provides 2-tier or 3-tier path resolution depending on context:
// - For local workflows (no pack context): checks .awf/<type>/ then falls back to global XDG
// - For pack workflows (pack_name in awfMap): checks .awf/<type>/<pack>/ → <pack_root>/<type>/ → global XDG
// Signature unchanged for backward compatibility; pack context is injected via awfMap["pack_name"].
func resolveLocalOverGlobal(interpolatedPath, sourceDir string, awfMap map[string]string) string {
	packName := awfMap[AWFPackNameKey]

	for _, key := range allowedAWFPathKeys {
		globalDir, ok := awfMap[key]
		if !ok || globalDir == "" {
			continue
		}

		suffix, hasPrefix := strings.CutPrefix(interpolatedPath, globalDir+string(filepath.Separator))
		if !hasPrefix {
			continue
		}

		// For pack workflows, implement 3-tier resolution:
		// 1. User override in .awf/<type>/<pack>/suffix
		// 2. Pack embedded in <pack_root>/<type>/suffix
		// 3. Global XDG (fallback, same as local workflows)
		if packName != "" {
			if path := resolvePackPathTiers(suffix, key, packName, sourceDir); path != "" {
				return path
			}
		}

		// For local workflows or when pack tiers don't exist, fall back to local-over-global (2-tier):
		// 1. .awf/<type>/suffix
		// 2. Global XDG
		localSubdir := strings.TrimSuffix(key, "_dir")
		localPath := filepath.Join(filepath.Dir(sourceDir), localSubdir, suffix)

		if _, err := os.Stat(localPath); err == nil {
			return localPath
		}
	}

	return interpolatedPath
}

// resolvePackPathTiers implements 3-tier resolution for pack workflows:
// Tier 1: .awf/<type>/<pack>/suffix (user override, found by searching parent directories)
// Tier 2: <pack_root>/<type>/suffix (embedded in pack)
// Tier 3: handled by caller (global XDG fallback)
// Returns the resolved path if found, empty string if no tiers resolve.
func resolvePackPathTiers(suffix, dirKey, packName, sourceDir string) string {
	// Derive <type> from dirKey: "scripts_dir" → "scripts"
	dirType := strings.TrimSuffix(dirKey, "_dir")

	// Tier 1: Check .awf/<type>/<pack>/<suffix> (user override, found by searching parent directories)
	sourceDir = filepath.Clean(sourceDir)
	currentDir := filepath.Dir(sourceDir)
	for {
		tier1Path := filepath.Join(currentDir, ".awf", dirType, packName, suffix)
		if _, err := os.Stat(tier1Path); err == nil {
			return tier1Path
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break
		}
		currentDir = parentDir
	}

	// Tier 2: Check <pack_root>/<type>/<suffix> (embedded in pack)
	// Derive pack root from sourceDir: assuming format is .../workflows/<file>
	sourceDirParent := filepath.Dir(sourceDir)
	if filepath.Base(sourceDirParent) == "workflows" {
		packRoot := filepath.Dir(sourceDirParent)
		tier2Path := filepath.Join(packRoot, dirType, suffix)
		if _, err := os.Stat(tier2Path); err == nil {
			return tier2Path
		}
	}

	return ""
}

// findPackLocalDir finds the appropriate local directory for pack resources by trying
// the 3-tier resolution chain: tier 1 (user override), tier 2 (pack embedded), or fallback.
func findPackLocalDir(sourceDir, localSubdir, packName string) string {
	// Tier 1: Check .awf/<type>/<pack>/ (user override)
	currentDir := filepath.Dir(sourceDir)
	for {
		tier1Dir := filepath.Join(currentDir, ".awf", localSubdir, packName)
		if _, err := os.Stat(tier1Dir); err == nil {
			return tier1Dir
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break
		}
		currentDir = parentDir
	}

	// Tier 2: Check <pack_root>/<type>/ (embedded in pack)
	sourceDirParent := filepath.Dir(sourceDir)
	if filepath.Base(sourceDirParent) == "workflows" {
		packRoot := filepath.Dir(sourceDirParent)
		tier2Dir := filepath.Join(packRoot, localSubdir)
		if _, err := os.Stat(tier2Dir); err == nil {
			return tier2Dir
		}
	}

	// Fallback: use source dir parent, will be caught by replaceAWFPathsInString
	return filepath.Join(filepath.Dir(sourceDir), localSubdir)
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
// For pack workflows, applies 3-tier resolution; for local workflows, applies 2-tier.
// Signature unchanged; pack context injected via awfMap["pack_name"].
func resolveCommandAWFPaths(cmd, sourceDir string, awfMap map[string]string) string {
	if cmd == "" || len(awfMap) == 0 {
		return cmd
	}

	packName := awfMap[AWFPackNameKey]
	result := cmd

	for _, key := range allowedAWFPathKeys {
		globalDir, ok := awfMap[key]
		if !ok || globalDir == "" {
			continue
		}

		localSubdir := strings.TrimSuffix(key, "_dir")

		// For pack workflows, resolve via 3-tier chain; for local workflows, use standard 2-tier
		var localDir string
		if packName != "" {
			localDir = findPackLocalDir(sourceDir, localSubdir, packName)
		} else {
			// For local workflows, use standard 2-tier resolution
			localDir = filepath.Join(filepath.Dir(sourceDir), localSubdir)
		}

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
