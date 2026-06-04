package mcp

import (
	"context"

	"github.com/awf-project/cli/internal/domain/ports"
)

// fakeProvider is the single configurable test double for ports.ToolProvider used across
// the package's tests. Configure tools/listErr to drive ListTools, and callResult/callErr/
// callPanic to drive CallTool.
type fakeProvider struct {
	tools      []ports.ToolDefinition
	listErr    error
	callResult *ports.ToolResult
	callErr    error
	callPanic  bool
}

func (f *fakeProvider) ListTools(ctx context.Context) ([]ports.ToolDefinition, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.tools, nil
}

func (f *fakeProvider) CallTool(ctx context.Context, name string, args map[string]any) (*ports.ToolResult, error) {
	if f.callPanic {
		panic("simulated provider panic")
	}
	if f.callErr != nil {
		return nil, f.callErr
	}
	return f.callResult, nil
}

func (f *fakeProvider) Close(ctx context.Context) error {
	return nil
}

// recordingProvider is a ports.ToolProvider whose CallTool delegates to an injected
// closure, letting tests inspect the exact args the handler forwards (e.g. nil vs empty
// map). ListTools returns tools/listErr like fakeProvider.
type recordingProvider struct {
	tools   []ports.ToolDefinition
	listErr error
	onCall  func(ctx context.Context, name string, args map[string]any) (*ports.ToolResult, error)
}

func (r *recordingProvider) ListTools(ctx context.Context) ([]ports.ToolDefinition, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.tools, nil
}

func (r *recordingProvider) CallTool(ctx context.Context, name string, args map[string]any) (*ports.ToolResult, error) {
	return r.onCall(ctx, name, args)
}

func (r *recordingProvider) Close(ctx context.Context) error {
	return nil
}

// assertToolExists reports whether a tool name is present in the registry.
func assertToolExists(names map[string]struct{}, toolName string) bool {
	_, exists := names[toolName]
	return exists
}
