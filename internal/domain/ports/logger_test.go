package ports_test

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
)

type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...any)             {}
func (m *mockLogger) Info(msg string, fields ...any)              {}
func (m *mockLogger) Warn(msg string, fields ...any)              {}
func (m *mockLogger) Error(msg string, fields ...any)             {}
func (m *mockLogger) WithContext(ctx map[string]any) ports.Logger { return m }

func TestLogger(t *testing.T) {
	mock := &mockLogger{}
	result := mock.WithContext(map[string]any{"workflow_id": "test"})
	assert.NotNil(t, result)
}
