//go:build integration

package plugins_test

import (
	"sync"
	"testing"

	"github.com/awf-project/awf/internal/application"
	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/tests/integration/testhelpers"
)

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

func skipInCI(t *testing.T) {
	t.Helper()
	testhelpers.SkipInCI(t)
}

func skipIfCLIMissing(t *testing.T, cliName string) {
	t.Helper()
	testhelpers.SkipIfCLIMissing(t, cliName)
}

func getRepoRoot(t *testing.T) string {
	t.Helper()
	return testhelpers.GetRepoRoot(t)
}

func setupTestWorkflowService(t *testing.T, workflowsDir, statesDir string) (*application.ExecutionService, ports.StateStore) {
	t.Helper()
	return testhelpers.SetupTestWorkflowService(t, workflowsDir, statesDir)
}
