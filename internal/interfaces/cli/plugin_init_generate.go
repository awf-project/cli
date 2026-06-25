package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func generatePluginRepository(ctx context.Context, options pluginInitOptions, out io.Writer) error {
	files, err := renderPluginInitTemplate(options)
	if err != nil {
		return err
	}

	if err := detectPluginInitConflicts(options, files); err != nil {
		return err
	}

	if err := writePluginInitFiles(ctx, options, files); err != nil {
		return err
	}

	return printPluginInitNextSteps(options, out)
}

func detectPluginInitConflicts(options pluginInitOptions, files []pluginInitFile) error {
	if err := rejectPluginInitOutputRootSymlink(options.outputDir); err != nil {
		return err
	}

	for _, file := range files {
		if err := rejectPluginInitSymlinkTarget(options.outputDir, file.path); err != nil {
			return err
		}
		if options.force {
			continue
		}

		target, err := pluginInitTargetPath(options.outputDir, file.path)
		if err != nil {
			return err
		}

		if _, err := os.Lstat(target); err == nil {
			return fmt.Errorf("plugin init output %s already exists; use --force to overwrite", file.path)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect plugin init output %s: %w", file.path, err)
		}
	}

	return nil
}

func writePluginInitFiles(ctx context.Context, options pluginInitOptions, files []pluginInitFile) error {
	if err := rejectPluginInitOutputRootSymlink(options.outputDir); err != nil {
		return err
	}
	if err := os.MkdirAll(options.outputDir, 0o755); err != nil { //nolint:gosec // T065 requires generated directories to be user-readable/executable.
		return fmt.Errorf("create plugin init output %s: %w", options.outputDir, err)
	}
	if err := os.Chmod(options.outputDir, 0o755); err != nil { //nolint:gosec // T065 requires stable 0755 generated directory permissions.
		return fmt.Errorf("set plugin init output mode %s: %w", options.outputDir, err)
	}

	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}

		target, err := pluginInitTargetPath(options.outputDir, file.path)
		if err != nil {
			return err
		}
		if err := rejectPluginInitSymlinkTarget(options.outputDir, file.path); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil { //nolint:gosec // T065 requires generated directories to be 0755.
			return fmt.Errorf("create plugin init directory for %s: %w", file.path, err)
		}
		if err := chmodPluginInitPathParents(options.outputDir, filepath.Dir(target)); err != nil {
			return fmt.Errorf("set plugin init directory mode for %s: %w", file.path, err)
		}
		if err := os.WriteFile(target, file.content, 0o644); err != nil { //nolint:gosec // T065 requires generated regular files to be 0644.
			return fmt.Errorf("write plugin init file %s: %w", file.path, err)
		}
		if err := os.Chmod(target, 0o644); err != nil { //nolint:gosec // T065 requires stable 0644 generated file permissions.
			return fmt.Errorf("set plugin init file mode %s: %w", file.path, err)
		}
	}

	return nil
}

func printPluginInitNextSteps(options pluginInitOptions, out io.Writer) error {
	steps := []string{
		"cd " + options.outputDir,
		"make test",
		"make build",
		"make install-local",
		"awf plugin enable " + options.distributionName,
		"awf plugin list --operations",
		"awf run examples/demo.yaml",
	}

	for _, step := range steps {
		if _, err := fmt.Fprintln(out, step); err != nil {
			return fmt.Errorf("write plugin init next steps: %w", err)
		}
	}

	return nil
}

func pluginInitTargetPath(outputDir, generatedPath string) (string, error) {
	var emptyPath string

	cleanGeneratedPath := filepath.Clean(filepath.FromSlash(generatedPath))
	if filepath.IsAbs(cleanGeneratedPath) || cleanGeneratedPath == "." || cleanGeneratedPath == ".." ||
		strings.HasPrefix(cleanGeneratedPath, ".."+string(filepath.Separator)) {
		return emptyPath, fmt.Errorf("invalid plugin init generated path %s", generatedPath)
	}

	target := filepath.Join(outputDir, cleanGeneratedPath)
	cleanOutputDir := filepath.Clean(outputDir)
	rel, err := filepath.Rel(cleanOutputDir, target)
	if err != nil {
		return emptyPath, fmt.Errorf("resolve plugin init generated path %s: %w", generatedPath, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return emptyPath, fmt.Errorf("invalid plugin init generated path %s", generatedPath)
	}

	return target, nil
}

func rejectPluginInitOutputRootSymlink(outputDir string) error {
	info, err := os.Lstat(filepath.Clean(outputDir))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect plugin init output directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("plugin init output directory uses symbolic link")
	}

	return nil
}

func rejectPluginInitSymlinkTarget(outputDir, generatedPath string) error {
	cleanGeneratedPath := filepath.Clean(filepath.FromSlash(generatedPath))
	if _, err := pluginInitTargetPath(outputDir, generatedPath); err != nil {
		return err
	}

	cleanOutputDir := filepath.Clean(outputDir)
	current := cleanOutputDir
	for _, component := range strings.Split(cleanGeneratedPath, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect plugin init output %s: %w", generatedPath, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			rel, relErr := filepath.Rel(cleanOutputDir, current)
			if relErr != nil {
				return fmt.Errorf("inspect plugin init output %s: %w", generatedPath, relErr)
			}
			return fmt.Errorf("plugin init output %s uses symbolic link %s", generatedPath, filepath.ToSlash(rel))
		}
	}

	return nil
}

func chmodPluginInitPathParents(outputDir, dir string) error {
	cleanOutputDir := filepath.Clean(outputDir)
	for {
		if err := os.Chmod(dir, 0o755); err != nil { //nolint:gosec // T065 requires stable 0755 generated directory permissions.
			return err
		}
		if filepath.Clean(dir) == cleanOutputDir {
			return nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}
