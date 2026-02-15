//go:build integration

package testhelpers

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/awf-project/awf/internal/application"
	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/infrastructure/executor"
	infraExpr "github.com/awf-project/awf/internal/infrastructure/expression"
	"github.com/awf-project/awf/internal/infrastructure/repository"
	"github.com/awf-project/awf/internal/infrastructure/store"
	"github.com/awf-project/awf/pkg/interpolation"
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

// SkipInCI skips the test if running in a CI environment.
func SkipInCI(t *testing.T) {
	t.Helper()
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		t.Skip("Skipping test in CI environment")
	}
}

// SkipIfRoot skips the test if running as root user.
func SkipIfRoot(t *testing.T) {
	t.Helper()
	if os.Getuid() == 0 {
		t.Skip("Test requires non-root user")
	}
}

// SkipIfCLIMissing skips the test if a required CLI tool is not installed.
func SkipIfCLIMissing(t *testing.T, cliName string) {
	t.Helper()
	if _, err := exec.LookPath(cliName); err != nil {
		t.Skipf("CLI tool %q not found in PATH", cliName)
	}
}

// SkipOnPlatform skips the test if running on specified platform(s).
func SkipOnPlatform(t *testing.T, platforms ...string) {
	t.Helper()
	for _, platform := range platforms {
		if runtime.GOOS == platform {
			t.Skipf("Test skipped on platform: %s", platform)
		}
	}
}

// SkipIfToolMissing is an alias for SkipIfCLIMissing.
func SkipIfToolMissing(t *testing.T, toolName string) {
	t.Helper()
	SkipIfCLIMissing(t, toolName)
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

// SetupTestWorkflowService creates a fully configured workflow service for integration tests.
func SetupTestWorkflowService(t *testing.T, workflowsDir, statesDir string) (*application.ExecutionService, ports.StateStore) {
	t.Helper()

	repo := repository.NewYAMLRepository(workflowsDir)
	stateStore := store.NewJSONStore(statesDir)
	shellExec := executor.NewShellExecutor()
	logger := &MockLogger{}
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()

	wfSvc := application.NewWorkflowService(repo, stateStore, shellExec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, shellExec, parallelExec, stateStore, logger, resolver, nil, evaluator,
	)

	return execSvc, stateStore
}
