package ports_test

import (
	"testing"

	"github.com/vanoix/awf/internal/domain/ports"
)

// mockLogger verifies the Logger interface can be implemented
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...any) {}
func (m *mockLogger) Info(msg string, fields ...any)  {}
func (m *mockLogger) Warn(msg string, fields ...any)  {}
func (m *mockLogger) Error(msg string, fields ...any) {}
func (m *mockLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

func TestLoggerInterface(t *testing.T) {
	// Verify interface compliance
	var _ ports.Logger = (*mockLogger)(nil)
}

func TestLoggerWithContext(t *testing.T) {
	mock := &mockLogger{}
	ctx := map[string]any{"workflow_id": "test-123"}

	result := mock.WithContext(ctx)
	if result == nil {
		t.Error("WithContext should return a Logger")
	}
}
