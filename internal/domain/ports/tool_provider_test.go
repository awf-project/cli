package ports_test

import (
	"context"

	"github.com/awf-project/cli/internal/domain/ports"
)

type fakeProvider struct{}

func (f *fakeProvider) ListTools(_ context.Context) ([]ports.ToolDefinition, error) {
	return nil, nil
}

func (f *fakeProvider) CallTool(_ context.Context, _ string, _ map[string]any) (*ports.ToolResult, error) {
	return nil, nil
}

func (f *fakeProvider) Close(_ context.Context) error {
	return nil
}

// Compile-time assertion: fakeProvider must implement ports.ToolProvider.
var _ ports.ToolProvider = (*fakeProvider)(nil)
