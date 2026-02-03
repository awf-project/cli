//go:build integration

package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/infrastructure/executor"
	infraExpr "github.com/vanoix/awf/internal/infrastructure/expression"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/internal/infrastructure/store"
	"github.com/vanoix/awf/pkg/interpolation"
)

// Feature: C019 - Shared Test Helpers
// Common utilities for memory management and integration tests

// =============================================================================
// Mock Logger
// =============================================================================

// mockLogger provides a simple logger implementation for testing.
type mockLogger struct {
	warnings []string
	errors   []string
	info     []string
	mu       sync.Mutex
}

func (m *mockLogger) Debug(msg string, fields ...any) {}

func (m *mockLogger) Info(msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.info = append(m.info, msg)
}

func (m *mockLogger) Warn(msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.warnings = append(m.warnings, msg)
}

func (m *mockLogger) Error(msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors = append(m.errors, msg)
}

func (m *mockLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

// =============================================================================
// CI Environment Detection
// =============================================================================

// skipInCI skips the test if running in a CI environment.
// Tests requiring external API access use this helper since CI lacks credentials.
func skipInCI(t *testing.T) {
	t.Helper()
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		t.Skip("Skipping test in CI environment")
	}
}

// =============================================================================
// C030: Skip Helper Functions
// =============================================================================

// skipIfRoot skips the test if running as root user.
// Tests that verify permission denials should not run as root.
func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Getuid() == 0 {
		t.Skip("Test requires non-root user")
	}
}

// skipIfCLIMissing skips the test if a required CLI tool is not installed.
// Common tools: claude, codex, gemini, dot (graphviz), golangci-lint
func skipIfCLIMissing(t *testing.T, cliName string) {
	t.Helper()
	if _, err := exec.LookPath(cliName); err != nil {
		t.Skipf("CLI tool %q not found in PATH", cliName)
	}
}

// skipOnPlatform skips the test if running on specified platform(s).
// Platform values: "windows", "darwin", "linux"
func skipOnPlatform(t *testing.T, platforms ...string) {
	t.Helper()
	for _, platform := range platforms {
		if runtime.GOOS == platform {
			t.Skipf("Test skipped on platform: %s", platform)
		}
	}
}

// skipIfToolMissing is a general helper that skips if any system tool is unavailable.
// Alias for skipIfCLIMissing for backward compatibility.
func skipIfToolMissing(t *testing.T, toolName string) {
	t.Helper()
	skipIfCLIMissing(t, toolName)
}

// =============================================================================
// Repository Root Helper
// =============================================================================

// getRepoRoot returns the repository root directory.
// It walks up from the current directory until it finds a go.mod file.
func getRepoRoot(t *testing.T) string {
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

// =============================================================================
// Workflow Service Setup
// =============================================================================

// setupTestWorkflowService creates a fully configured workflow service for integration tests.
func setupTestWorkflowService(t *testing.T, workflowsDir, statesDir string) (*application.ExecutionService, ports.StateStore) {
	t.Helper()

	// Real components for integration testing
	repo := repository.NewYAMLRepository(workflowsDir)
	stateStore := store.NewJSONStore(statesDir)
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	// Expression evaluator for loop conditions (infrastructure adapter per C042)
	evaluator := infraExpr.NewExprEvaluator()

	// Wire up services
	wfSvc := application.NewWorkflowService(repo, stateStore, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, stateStore, logger, resolver, nil, evaluator,
	)

	return execSvc, stateStore
}
