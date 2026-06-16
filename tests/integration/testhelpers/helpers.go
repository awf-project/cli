//go:build integration

package testhelpers

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
)

// MockLogger provides a simple logger implementation for testing.
type MockLogger struct {
	warnings []string
	errors   []string
	info     []string
	mu       sync.Mutex
}

func (m *MockLogger) Debug(msg string, fields ...any) {}

func (m *MockLogger) Info(msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.info = append(m.info, msg)
}

func (m *MockLogger) Warn(msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.warnings = append(m.warnings, msg)
}

func (m *MockLogger) Error(msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors = append(m.errors, msg)
}

func (m *MockLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

// Errors returns all error messages captured by the logger.
func (m *MockLogger) Errors() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.errors))
	copy(result, m.errors)
	return result
}

// Infos returns all info messages captured by the logger.
func (m *MockLogger) Infos() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.info))
	copy(result, m.info)
	return result
}

// SkipInCI skips the test if running in a CI environment.
func SkipInCI(t *testing.T) {
	t.Helper()
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		t.Skip("Skipping test in CI environment")
	}
}

// SkipIfCLIMissing skips the test if a required CLI tool is not installed.
func SkipIfCLIMissing(t *testing.T, cliName string) {
	t.Helper()
	if _, err := exec.LookPath(cliName); err != nil {
		t.Skipf("CLI tool %q not found in PATH", cliName)
	}
}

// GetRepoRoot returns the repository root directory.
func GetRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("should get current directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repository root (no go.mod found)")
		}
		dir = parent
	}
}
