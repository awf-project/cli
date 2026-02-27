package application

import "github.com/awf-project/cli/internal/domain/ports"

// mockLogger implements ports.Logger for testing.
// This is a shared test helper for application package tests.
// Template service tests use testutil.MockLogger instead (see C044).
type mockLogger struct {
	warnings []string
	errors   []string
}

func (m *mockLogger) Debug(msg string, fields ...any) {}
func (m *mockLogger) Info(msg string, fields ...any)  {}
func (m *mockLogger) Warn(msg string, fields ...any) {
	if m.warnings == nil {
		m.warnings = []string{}
	}
	m.warnings = append(m.warnings, msg)
}

func (m *mockLogger) Error(msg string, fields ...any) {
	if m.errors == nil {
		m.errors = []string{}
	}
	m.errors = append(m.errors, msg)
}

func (m *mockLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}
