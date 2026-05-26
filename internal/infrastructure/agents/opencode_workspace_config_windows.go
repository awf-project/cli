//go:build windows

package agents

import "errors"

// errNotSupportedOnWindows is returned by workspace-config helpers on Windows.
// OpenCode's workspace config relies on POSIX flock (syscall.LOCK_EX) which is
// not available on Windows. OpenCode itself does not have a first-class Windows
// release, so this stub prevents compilation failures without silently no-oping
// in a way that could confuse callers.
var errNotSupportedOnWindows = errors.New("opencode workspace config not supported on Windows")

// addOpenCodeMCPServer is a no-op stub on Windows.
// The real implementation uses syscall.Flock which is unavailable on Windows.
func addOpenCodeMCPServer(_ string, _ string, _ []string) (func() error, error) {
	return func() error { return nil }, errNotSupportedOnWindows
}

// removeOpenCodeMCPServer is a no-op stub on Windows.
func removeOpenCodeMCPServer(_ string, _ string, _ bool) error {
	return errNotSupportedOnWindows
}
