//go:build integration

package integration_test

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"testing"

	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/infrastructure/executor"
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
// Simple Expression Evaluator
// =============================================================================

// simpleExpressionEvaluator evaluates basic expressions for integration tests.
type simpleExpressionEvaluator struct{}

func newSimpleExpressionEvaluator() *simpleExpressionEvaluator {
	return &simpleExpressionEvaluator{}
}

func (e *simpleExpressionEvaluator) Evaluate(expr string, ctx *interpolation.Context) (bool, error) {
	// Handle common test expressions
	switch expr {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}

	// Handle loop.Index comparisons
	if ctx != nil && ctx.Loop != nil {
		// Pattern: loop.Index < N
		if len(expr) > 11 && expr[:11] == "loop.Index " {
			// Simple parser for "loop.Index < N" or "loop.Index <= N"
			var value int
			if _, err := fmt.Sscanf(expr, "loop.Index < %d", &value); err == nil {
				return ctx.Loop.Index < value, nil
			}
			if _, err := fmt.Sscanf(expr, "loop.Index <= %d", &value); err == nil {
				return ctx.Loop.Index <= value, nil
			}
			if _, err := fmt.Sscanf(expr, "loop.Index > %d", &value); err == nil {
				return ctx.Loop.Index > value, nil
			}
			if _, err := fmt.Sscanf(expr, "loop.Index >= %d", &value); err == nil {
				return ctx.Loop.Index >= value, nil
			}
			if _, err := fmt.Sscanf(expr, "loop.Index == %d", &value); err == nil {
				return ctx.Loop.Index == value, nil
			}
			if _, err := fmt.Sscanf(expr, "loop.Index != %d", &value); err == nil {
				return ctx.Loop.Index != value, nil
			}
		}
	}

	// Check for states.X.output == "value" pattern
	if ctx != nil && ctx.States != nil {
		for stepName, state := range ctx.States {
			// states.X.exit_code == 0
			if expr == "states."+stepName+".exit_code == 0" {
				return state.ExitCode == 0, nil
			}
			// states.X.exit_code != 0
			if expr == "states."+stepName+".exit_code != 0" {
				return state.ExitCode != 0, nil
			}
			// states.X.output == "value" (simplified)
			if expr == `states.`+stepName+`.output == "ready"` {
				return state.Output == "ready\n" || state.Output == "ready", nil
			}
			if expr == `states.`+stepName+`.output == "stop"` {
				return state.Output == "stop\n" || state.Output == "stop", nil
			}
		}
	}

	return false, nil
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

	// Expression evaluator for loop conditions
	evaluator := newSimpleExpressionEvaluator()

	// Wire up services
	wfSvc := application.NewWorkflowService(repo, stateStore, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, stateStore, logger, resolver, nil, evaluator,
	)

	return execSvc, stateStore
}
