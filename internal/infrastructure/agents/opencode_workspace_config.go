//go:build !windows

package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// workspaceLockTimeout is the maximum duration to wait for an exclusive flock on the
// workspace lock file before aborting. Five seconds is generous for any legitimate
// contention between sibling AWF processes; a longer wait risks stalling workflows.
const workspaceLockTimeout = 5 * time.Second

// acquireWorkspaceLock opens (or creates) the lock file at lockPath, starts a
// goroutine to acquire LOCK_EX via syscall.Flock, and waits up to
// workspaceLockTimeout for success. On success it returns the open *os.File and a
// release function that closes the file (which releases the advisory lock). The
// caller is responsible for calling release (typically via defer) before returning.
//
// Returns a non-nil error if the lock file cannot be opened, the flock call fails,
// or the timeout expires.
func acquireWorkspaceLock(lockPath string) (lf *os.File, release func(), err error) {
	lf, openErr := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if openErr != nil {
		return nil, nil, fmt.Errorf("open lock file: %w", openErr)
	}

	flockDone := make(chan error, 1)
	go func() {
		flockDone <- syscall.Flock(int(lf.Fd()), syscall.LOCK_EX) //nolint:gosec // G115: file descriptor values are within int range on all supported platforms
	}()

	lockTimeout := time.NewTimer(workspaceLockTimeout)
	defer lockTimeout.Stop()
	select {
	case ferr := <-flockDone:
		if ferr != nil {
			_ = lf.Close()
			return nil, nil, fmt.Errorf("acquire lock: %w", ferr)
		}
	case <-lockTimeout.C:
		_ = lf.Close()
		return nil, nil, fmt.Errorf("timed out acquiring lock after %s", workspaceLockTimeout)
	}

	release = func() {
		_ = lf.Close() //nolint:errcheck // lock file close; advisory lock released on fd close
	}
	return lf, release, nil
}

// opencodeLockPath returns a per-workspace flock target rooted in os.TempDir() so
// the sidecar never appears in the user's workspace (avoids git-ignore churn and
// stale artifacts). The 8-byte SHA-256 prefix of the absolute workspace path is
// collision-resistant for any realistic number of workspaces on one host while
// keeping the path short enough to remain readable in lsof/strace output.
func opencodeLockPath(workspaceDir string) string {
	abs, err := filepath.Abs(workspaceDir)
	if err != nil {
		abs = workspaceDir
	}
	sum := sha256.Sum256([]byte(abs))
	return filepath.Join(os.TempDir(), "awf-opencode-"+hex.EncodeToString(sum[:8])+".lock")
}

// opencodeMCPEntry represents a single MCP server entry in opencode.json under
// the "mcp" key. Only the "local" type is used by AWF proxy registrations.
type opencodeMCPEntry struct {
	Type    string   `json:"type"`
	Command []string `json:"command"`
	Enabled bool     `json:"enabled"`
}

// atomicWriteJSON writes data as indented JSON to configPath atomically via a temp
// file + rename. On failure it cleans up the temp file and returns an error.
func atomicWriteJSON(configPath string, top map[string]json.RawMessage) error {
	tmpPath := fmt.Sprintf("%s.%d.%d.tmp", configPath, os.Getpid(), time.Now().UnixNano())
	data, marshalErr := json.MarshalIndent(top, "", "  ")
	if marshalErr != nil {
		return fmt.Errorf("opencode workspace config: marshal top-level: %w", marshalErr)
	}
	tf, createErr := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if createErr != nil {
		return fmt.Errorf("opencode workspace config: create temp file: %w", createErr)
	}
	if _, werr := tf.Write(data); werr != nil {
		_ = tf.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("opencode workspace config: write temp file: %w", werr)
	}
	if serr := tf.Sync(); serr != nil {
		_ = tf.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("opencode workspace config: sync temp file: %w", serr)
	}
	_ = tf.Close()
	if renameErr := os.Rename(tmpPath, configPath); renameErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("opencode workspace config: rename to final: %w", renameErr)
	}
	return nil
}

// readOpenCodeConfig reads and JSON-parses configPath into a top-level raw-message map.
// Returns (map, createdByUs=false, nil) when the file exists and parses cleanly,
// (empty map, createdByUs=true, nil) when the file does not exist yet, or
// (nil, false, err) on any other I/O or parse error.
func readOpenCodeConfig(configPath string) (top map[string]json.RawMessage, createdByUs bool, err error) {
	top = make(map[string]json.RawMessage)
	existing, readErr := os.ReadFile(configPath)
	switch {
	case readErr == nil:
		if parseErr := json.Unmarshal(existing, &top); parseErr != nil {
			return nil, false, fmt.Errorf("opencode workspace config: parse opencode.json: %w", parseErr)
		}
	case os.IsNotExist(readErr):
		createdByUs = true
	default:
		return nil, false, fmt.Errorf("opencode workspace config: read opencode.json: %w", readErr)
	}
	return top, createdByUs, nil
}

// decodeMCPMap extracts the "mcp" key from top as a map[string]opencodeMCPEntry.
// If the key is absent or corrupt, returns an empty map without error (corrupt → fresh).
func decodeMCPMap(top map[string]json.RawMessage) map[string]opencodeMCPEntry {
	mcpMap := make(map[string]opencodeMCPEntry)
	if raw, ok := top["mcp"]; ok {
		_ = json.Unmarshal(raw, &mcpMap) //nolint:errcheck // corrupt mcp key → start fresh to avoid blocking
	}
	return mcpMap
}

// encodeMCPMap re-encodes mcpMap into top["mcp"], or deletes top["mcp"] if mcpMap is empty.
func encodeMCPMap(top map[string]json.RawMessage, mcpMap map[string]opencodeMCPEntry) error {
	if len(mcpMap) == 0 {
		delete(top, "mcp")
		return nil
	}
	mcpRaw, marshalErr := json.Marshal(mcpMap)
	if marshalErr != nil {
		return fmt.Errorf("opencode workspace config: marshal mcp: %w", marshalErr)
	}
	top["mcp"] = json.RawMessage(mcpRaw)
	return nil
}

// addOpenCodeMCPServer writes name → command into ./opencode.json under the mcp
// key, preserving all other top-level keys (including $schema and any user-defined
// keys). Returns an idempotent cleanup that removes only the named entry and
// deletes the file if it becomes empty AND was created by this call.
//
// Concurrency: multiple AWF processes may run concurrently in the same workspace
// directory. A per-workspace flock target in os.TempDir() (see opencodeLockPath)
// is used to serialize read-modify-write cycles: acquire LOCK_EX → read
// opencode.json → modify in-memory → atomic write via tempfile + rename →
// release lock. The lock target lives outside the workspace so the user never
// sees a sidecar; it is never deleted because deletion would race with another
// process acquiring it.
//
// Cleanup semantics: removes only the entry keyed by name. If the resulting mcp
// map is empty AND the file was created from scratch (i.e. no pre-existing file),
// the file is deleted to avoid leaving cruft in user workspaces.
//
// The workspaceDir parameter is the directory where opencode.json will be written.
// Pass os.Getwd() at the call site to write in the process working directory.
func addOpenCodeMCPServer(workspaceDir, name string, command []string) (func() error, error) {
	configPath := filepath.Join(workspaceDir, "opencode.json")
	lockPath := opencodeLockPath(workspaceDir)

	_, release, lockErr := acquireWorkspaceLock(lockPath)
	if lockErr != nil {
		return nil, fmt.Errorf("opencode workspace config: %w", lockErr)
	}
	defer release()

	top, createdByUs, err := readOpenCodeConfig(configPath)
	if err != nil {
		return nil, err
	}

	mcpMap := decodeMCPMap(top)
	mcpMap[name] = opencodeMCPEntry{
		Type:    "local",
		Command: command,
		Enabled: true,
	}
	if encErr := encodeMCPMap(top, mcpMap); encErr != nil {
		return nil, encErr
	}
	if writeErr := atomicWriteJSON(configPath, top); writeErr != nil {
		return nil, writeErr
	}

	// Build the cleanup closure. It uses context.Background() (same pattern as
	// geminiMCPInjector) so teardown runs even when the parent context is cancelled.
	// sync.Once guarantees idempotency — a second call is a no-op that returns nil.
	var once sync.Once
	var cleanupErr error
	cleanupFn := func() error {
		once.Do(func() {
			cleanupErr = removeOpenCodeMCPServer(workspaceDir, name, createdByUs)
		})
		return cleanupErr
	}
	return cleanupFn, nil
}

// removeOpenCodeMCPServer removes the entry for name from opencode.json, then
// deletes the file if the mcp map becomes empty AND the file was created by AWF
// (createdByUs == true). Uses the same flock + atomic-rename pattern as addOpenCodeMCPServer.
//
// When createdByUs is true, opencode may have annotated the file with additional
// keys (e.g. "$schema") between add and remove. These annotations are considered
// transient — the file belongs to AWF and is deleted regardless of extra keys.
func removeOpenCodeMCPServer(workspaceDir, name string, createdByUs bool) error {
	configPath := filepath.Join(workspaceDir, "opencode.json")
	lockPath := opencodeLockPath(workspaceDir)

	_, release, lockErr := acquireWorkspaceLock(lockPath)
	if lockErr != nil {
		return fmt.Errorf("opencode workspace config cleanup: %w", lockErr)
	}
	defer release()

	existing, readErr := os.ReadFile(configPath)
	if os.IsNotExist(readErr) {
		// Already gone — nothing to do.
		return nil
	}
	if readErr != nil {
		return fmt.Errorf("opencode workspace config cleanup: read opencode.json: %w", readErr)
	}

	top := make(map[string]json.RawMessage)
	if parseErr := json.Unmarshal(existing, &top); parseErr != nil {
		return fmt.Errorf("opencode workspace config cleanup: parse opencode.json: %w", parseErr)
	}

	mcpMap := decodeMCPMap(top)
	delete(mcpMap, name)

	// If the mcp map is now empty AND we created the file from scratch, the entire
	// file is our artifact — delete it. Note: opencode itself canonicalizes the file
	// when it loads (e.g. it may inject "$schema"), so we cannot demand top is empty
	// or contains only "mcp". When createdByUs is true, no user content can have
	// reached this file via legitimate edits during the few seconds of a workflow
	// step — any extra keys are opencode's own annotation and safe to discard.
	if len(mcpMap) == 0 && createdByUs {
		return os.Remove(configPath)
	}

	if encErr := encodeMCPMap(top, mcpMap); encErr != nil {
		return fmt.Errorf("opencode workspace config cleanup: %w", encErr)
	}

	// If top is now completely empty, delete the file.
	if len(top) == 0 {
		return os.Remove(configPath)
	}

	if writeErr := atomicWriteJSON(configPath, top); writeErr != nil {
		return fmt.Errorf("opencode workspace config cleanup: %w", writeErr)
	}
	return nil
}
