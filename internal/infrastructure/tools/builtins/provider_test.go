package builtins_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/infrastructure/tools/builtins"
)

func TestProvider_ListTools_ReturnsSix(t *testing.T) {
	p := builtins.NewProvider()
	ctx := context.Background()

	tools, err := p.ListTools(ctx)

	require.NoError(t, err)
	assert.Len(t, tools, 6)
}

func TestProvider_ListTools_SourceBuiltin(t *testing.T) {
	p := builtins.NewProvider()
	ctx := context.Background()

	tools, err := p.ListTools(ctx)

	require.NoError(t, err)
	for _, td := range tools {
		assert.Equal(t, "builtin", td.Source, "tool %s has wrong Source", td.Name)
	}
}

func TestProvider_CallTool_UnknownTool_ReturnsError(t *testing.T) {
	p := builtins.NewProvider()
	ctx := context.Background()

	result, err := p.CallTool(ctx, "NonExistent", map[string]any{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "tool not found")
}

func TestProvider_CallTool_MissingRequiredArg_ReturnsError(t *testing.T) {
	p := builtins.NewProvider()
	ctx := context.Background()

	result, err := p.CallTool(ctx, "Read", map[string]any{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "missing required argument")
}

func TestProvider_Close_ReturnsNil(t *testing.T) {
	p := builtins.NewProvider()

	err := p.Close(context.Background())

	assert.NoError(t, err)
}

func TestProvider_ListTools_ToolNames(t *testing.T) {
	p := builtins.NewProvider()
	ctx := context.Background()

	tools, err := p.ListTools(ctx)

	require.NoError(t, err)

	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}

	assert.True(t, names["Read"], "should have Read tool")
	assert.True(t, names["Write"], "should have Write tool")
	assert.True(t, names["Edit"], "should have Edit tool")
	assert.True(t, names["Bash"], "should have Bash tool")
	assert.True(t, names["Glob"], "should have Glob tool")
	assert.True(t, names["Grep"], "should have Grep tool")
}

func TestProvider_ListTools_DescriptionsNonEmpty(t *testing.T) {
	p := builtins.NewProvider()
	ctx := context.Background()

	tools, err := p.ListTools(ctx)

	require.NoError(t, err)
	for _, td := range tools {
		assert.NotEmpty(t, td.Description, "tool %s must have a non-empty Description", td.Name)
	}
}

func TestProvider_ListTools_DescriptionContents(t *testing.T) {
	p := builtins.NewProvider()
	ctx := context.Background()

	tools, err := p.ListTools(ctx)
	require.NoError(t, err)

	byName := make(map[string]string, len(tools))
	for _, td := range tools {
		byName[td.Name] = td.Description
	}

	tests := []struct {
		tool     string
		contains string
	}{
		{"Read", "path"},
		{"Write", "content"},
		{"Edit", "old"},
		{"Bash", "command"},
		{"Glob", "pattern"},
		{"Grep", "pattern"},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			desc, ok := byName[tt.tool]
			require.True(t, ok, "tool %s must be registered", tt.tool)
			assert.Contains(t, desc, tt.contains, "description for %s must mention %q", tt.tool, tt.contains)
		})
	}
}

func TestProvider_InputSchema_ValidStructure(t *testing.T) {
	p := builtins.NewProvider()
	ctx := context.Background()

	tools, err := p.ListTools(ctx)
	require.NoError(t, err)

	for _, tool := range tools {
		t.Run(tool.Name, func(t *testing.T) {
			assert.NotNil(t, tool.InputSchema, "InputSchema should not be nil")
			assert.Equal(t, "object", tool.InputSchema["type"], "should be object type")
			assert.NotNil(t, tool.InputSchema["properties"], "should have properties")
			assert.NotNil(t, tool.InputSchema["required"], "should have required field")
		})
	}
}

func TestProvider_CallTool_Write_HappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	p := builtins.NewProvider()
	result, err := p.CallTool(context.Background(), "Write", map[string]any{
		"path":    path,
		"content": "hello world",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(data))
}

func TestProvider_CallTool_Write_AtomicFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "atomic.txt")

	p := builtins.NewProvider()
	_, err := p.CallTool(context.Background(), "Write", map[string]any{
		"path":    path,
		"content": "atomic content",
	})

	require.NoError(t, err)

	files, err := os.ReadDir(dir)
	require.NoError(t, err)

	tempCount := 0
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".tmp") {
			tempCount++
		}
	}
	assert.Equal(t, 0, tempCount, "no temp files should remain after atomic write")
}

func TestProvider_CallTool_Edit_ReplaceFirst(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	require.NoError(t, os.WriteFile(path, []byte("foo bar foo"), 0o644))

	p := builtins.NewProvider()
	result, err := p.CallTool(context.Background(), "Edit", map[string]any{
		"path": path,
		"old":  "foo",
		"new":  "baz",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "baz bar foo", string(data), "should replace only first occurrence")
}

func TestProvider_CallTool_Edit_ReplaceAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit_all.txt")
	require.NoError(t, os.WriteFile(path, []byte("foo bar foo"), 0o644))

	p := builtins.NewProvider()
	result, err := p.CallTool(context.Background(), "Edit", map[string]any{
		"path":        path,
		"old":         "foo",
		"new":         "baz",
		"replace_all": true,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "baz bar baz", string(data), "should replace all occurrences")
}

func TestProvider_CallTool_Edit_OldNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notfound.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello world"), 0o644))

	p := builtins.NewProvider()
	result, err := p.CallTool(context.Background(), "Edit", map[string]any{
		"path": path,
		"old":  "xyz",
		"new":  "abc",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "should return IsError=true when old string not found")
}
