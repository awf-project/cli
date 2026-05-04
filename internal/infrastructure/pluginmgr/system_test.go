package pluginmgr_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
)

type nopLogger struct{}

func (l *nopLogger) Debug(string, ...any)                    {}
func (l *nopLogger) Info(string, ...any)                     {}
func (l *nopLogger) Warn(string, ...any)                     {}
func (l *nopLogger) Error(string, ...any)                    {}
func (l *nopLogger) WithContext(map[string]any) ports.Logger { return l }

func TestInitSystem_EmptyDirs(t *testing.T) {
	dir := t.TempDir()
	result, err := pluginmgr.InitSystem(context.Background(), nil, filepath.Join(dir, "plugins"), &nopLogger{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Cleanup()
	if result.Service == nil {
		t.Error("expected non-nil service even with empty dirs")
	}
	if result.RPCManager != nil {
		t.Error("expected nil RPCManager with no plugin dirs")
	}
}

func TestInitSystem_NonExistentDirs(t *testing.T) {
	dir := t.TempDir()
	result, err := pluginmgr.InitSystem(context.Background(), []string{"/nonexistent/a", "/nonexistent/b"}, filepath.Join(dir, "plugins"), &nopLogger{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer result.Cleanup()
	if result.Service == nil {
		t.Error("expected non-nil service")
	}
}
