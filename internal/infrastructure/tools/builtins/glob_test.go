package builtins_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/infrastructure/tools/builtins"
)

// callGlob invokes the Glob built-in tool and returns the matched paths split
// from the single text content block.
func callGlob(t *testing.T, args map[string]any) (matches []string, isError *bool, err error) {
	t.Helper()
	p := builtins.NewProvider()
	result, callErr := p.CallTool(context.Background(), "Glob", args)
	if callErr != nil {
		return nil, nil, callErr
	}
	require.NotNil(t, result)
	require.Len(t, result.Content, 1)
	require.Equal(t, "text", result.Content[0].Type)
	flag := result.IsError
	if result.Content[0].Text == "" {
		return nil, &flag, nil
	}
	return strings.Split(result.Content[0].Text, "\n"), &flag, nil
}

func TestGlob_SimpleWildcard(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.go", "b.go", "c.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), nil, 0o644))
	}

	matches, isErr, err := callGlob(t, map[string]any{
		"pattern": filepath.Join(dir, "*.go"),
	})
	require.NoError(t, err)
	require.NotNil(t, isErr)
	assert.False(t, *isErr)
	sort.Strings(matches)
	assert.Equal(t, []string{filepath.Join(dir, "a.go"), filepath.Join(dir, "b.go")}, matches)
}

func TestGlob_CwdJoinedWithRelativePattern(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "found.md"), nil, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skip.txt"), nil, 0o644))

	matches, _, err := callGlob(t, map[string]any{
		"pattern": "*.md",
		"cwd":     dir,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join(dir, "found.md")}, matches)
}

func TestGlob_CharacterClass(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"file1.log", "file2.log", "file3.log", "other.log"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), nil, 0o644))
	}

	matches, _, err := callGlob(t, map[string]any{
		"pattern": filepath.Join(dir, "file[12].log"),
	})
	require.NoError(t, err)
	sort.Strings(matches)
	assert.Equal(t, []string{
		filepath.Join(dir, "file1.log"),
		filepath.Join(dir, "file2.log"),
	}, matches)
}

func TestGlob_NoMatchReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "only.txt"), nil, 0o644))

	matches, isErr, err := callGlob(t, map[string]any{
		"pattern": filepath.Join(dir, "*.go"),
	})
	require.NoError(t, err)
	require.NotNil(t, isErr)
	assert.False(t, *isErr, "no-match must not be flagged as an error")
	assert.Empty(t, matches)
}

func TestGlob_InvalidPatternReturnsError(t *testing.T) {
	// `filepath.Glob` returns filepath.ErrBadPattern for unmatched bracket.
	_, _, err := callGlob(t, map[string]any{
		"pattern": "/tmp/[unclosed",
	})
	require.Error(t, err)
}

func TestGlob_EmptyCwdIsIgnored(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), nil, 0o644))

	matches, _, err := callGlob(t, map[string]any{
		"pattern": filepath.Join(dir, "*.txt"),
		"cwd":     "",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join(dir, "x.txt")}, matches)
}

func TestGlob_MatchesDotFiles(t *testing.T) {
	// filepath.Glob does NOT replicate shell-style dotfile exclusion: `*` matches
	// a leading dot. This test pins that behavior so a future surprise change
	// gets flagged here rather than in production.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".hidden"), nil, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "visible"), nil, 0o644))

	matches, _, err := callGlob(t, map[string]any{
		"pattern": filepath.Join(dir, "*"),
	})
	require.NoError(t, err)
	sort.Strings(matches)
	assert.Equal(t, []string{
		filepath.Join(dir, ".hidden"),
		filepath.Join(dir, "visible"),
	}, matches)
}

func TestGlob_RequiresPattern(t *testing.T) {
	p := builtins.NewProvider()
	_, err := p.CallTool(context.Background(), "Glob", map[string]any{})
	require.Error(t, err, "missing required pattern argument must error")
}

// TestGlobHandler_RejectsAbsolutePattern verifies that an absolute glob pattern is
// rejected when a cwd is provided. Without this check, filepath.Join silently ignores
// cwd and the pattern can escape the sandbox to enumerate arbitrary filesystem paths.
func TestGlobHandler_RejectsAbsolutePattern(t *testing.T) {
	dir := t.TempDir()

	p := builtins.NewProvider()
	_, err := p.CallTool(context.Background(), "Glob", map[string]any{
		"pattern": "/etc/passwd",
		"cwd":     dir,
	})

	require.Error(t, err, "absolute pattern with cwd must return an error")
	assert.Contains(t, err.Error(), "absolute glob patterns not allowed")
}

// TestGlobHandler_AbsolutePatternWithoutCwd verifies that an absolute pattern is
// still accepted when no cwd is provided (existing behavior). The filterPathsWithinRoot
// guard handles sandbox enforcement in that case.
func TestGlobHandler_AbsolutePatternWithoutCwd(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "match.go"), nil, 0o644))

	matches, isErr, err := callGlob(t, map[string]any{
		"pattern": filepath.Join(dir, "*.go"),
		// no cwd — absolute pattern is allowed
	})

	require.NoError(t, err)
	require.NotNil(t, isErr)
	assert.False(t, *isErr)
	assert.Equal(t, []string{filepath.Join(dir, "match.go")}, matches)
}
