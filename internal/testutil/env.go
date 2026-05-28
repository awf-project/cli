package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// ChdirIsolated changes the working directory to dir and isolates HOME,
// XDG_CONFIG_HOME, XDG_DATA_HOME, and AWF_ROLES_PATH so that role and skill
// search chains (which derive paths from these variables) resolve under dir
// rather than the real user home directory.
//
// Without isolation, global search-path positions such as ~/.agents/roles and
// $XDG_CONFIG_HOME/awf/roles would resolve against the real home directory,
// making tests non-deterministic across machines.
//
// AWF_ROLES_PATH is cleared explicitly so that an operator-level env override
// does not mask the default search chain under test.
//
// The original working directory is restored via t.Cleanup.
//
// Usage:
//
//	func TestSomething(t *testing.T) {
//	    tmpDir := t.TempDir()
//	    testutil.ChdirIsolated(t, tmpDir)
//	    // ... test against deterministic paths
//	}
func ChdirIsolated(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, ".local", "share"))
	t.Setenv("AWF_ROLES_PATH", "")
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("ChdirIsolated: os.Getwd: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck,gosec // best-effort restore of original working directory; G104 false positive in cleanup context
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("ChdirIsolated: os.Chdir(%q): %v", dir, err)
	}
}
