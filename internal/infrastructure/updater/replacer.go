package updater

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const binaryFileMode = 0o755

// ReplaceBinary atomically replaces the binary at execPath with newData.
// Resolves symlinks before replacement. Uses os.Rename for atomic swap
// on the same filesystem; falls back to copy+chmod for cross-filesystem.
func ReplaceBinary(execPath string, newData []byte) error {
	if len(newData) == 0 {
		return fmt.Errorf("replace binary: empty data")
	}

	// Resolving symlinks ensures we replace the actual binary, not the symlink.
	realPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve binary path: %w", err)
	}

	// Same directory as target ensures same filesystem for atomic rename.
	dir := filepath.Dir(realPath)
	tmpFile, err := os.CreateTemp(dir, ".awf-upgrade-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, writeErr := tmpFile.Write(newData); writeErr != nil {
		_ = tmpFile.Close()    //nolint:errcheck // best-effort cleanup on write failure
		_ = os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("write temp binary: %w", writeErr)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("close temp binary: %w", err)
	}

	if err := os.Chmod(tmpPath, binaryFileMode); err != nil {
		_ = os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("chmod temp binary: %w", err)
	}

	if err := os.Rename(tmpPath, realPath); err != nil {
		// Fallback: copy + chmod when rename fails across filesystems.
		if fallbackErr := crossFSReplace(tmpPath, realPath); fallbackErr != nil {
			_ = os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup
			return fmt.Errorf("replace binary (cross-filesystem fallback): %w", fallbackErr)
		}
		_ = os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup
	}

	return nil
}

// crossFSReplace copies src to dst when os.Rename fails (cross-filesystem).
// Writes to a temp file in the same directory as dst, then renames atomically
// to avoid truncating the existing binary on I/O failure.
func crossFSReplace(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	// Write to temp file in dst's directory (same filesystem) for atomic rename.
	dstDir := filepath.Dir(dst)
	tmpFile, err := os.CreateTemp(dstDir, ".awf-cross-fs-*")
	if err != nil {
		return fmt.Errorf("create temp for cross-fs: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := io.Copy(tmpFile, srcFile); err != nil {
		_ = tmpFile.Close()    //nolint:errcheck // best-effort cleanup
		_ = os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("copy binary: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("close temp: %w", err)
	}

	if err := os.Chmod(tmpPath, binaryFileMode); err != nil {
		_ = os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("chmod temp: %w", err)
	}

	// Atomic rename within same filesystem — dst is untouched on any prior failure.
	if err := os.Rename(tmpPath, dst); err != nil {
		_ = os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("rename temp to destination: %w", err)
	}

	return nil
}

// IsPackageManagerPath returns true if the binary path appears to be managed
// by a system package manager (homebrew, snap, nix, apt).
func IsPackageManagerPath(path string) bool {
	packageManagerPaths := []string{
		"/homebrew/",
		"linuxbrew/",
		"/snap/",
		"/nix/store/",
		"/nix/profile/",
	}

	for _, pm := range packageManagerPaths {
		if strings.Contains(path, pm) {
			return true
		}
	}

	// /usr/bin is typically managed by apt/yum; /usr/local/bin is user-managed
	if strings.HasPrefix(path, "/usr/bin/") {
		return true
	}

	return false
}
