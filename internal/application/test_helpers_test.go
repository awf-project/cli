package application

import (
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/pkg/interpolation"
)

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

// newMockLogger creates a new mockLogger instance
func newMockLogger() *mockLogger {
	return &mockLogger{}
}

// mockResolver provides simple passthrough resolution for testing
type mockResolver struct{}

func (m *mockResolver) Resolve(template string, ctx *interpolation.Context) (string, error) {
	return template, nil
}

// newMockResolver creates a new mockResolver instance
func newMockResolver() *mockResolver {
	return &mockResolver{}
}
