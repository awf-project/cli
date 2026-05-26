package builtins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/awf-project/cli/internal/domain/ports"
)

var globSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"pattern": map[string]any{
			"type": "string",
		},
		"cwd": map[string]any{
			"type": "string",
		},
	},
	"required": []string{"pattern"},
}

func (p *Provider) globHandler(_ context.Context, args map[string]any) (*ports.ToolResult, error) {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: "pattern must be a string"}},
			IsError: true,
		}, nil
	}

	if cwd, ok := args["cwd"].(string); ok && cwd != "" {
		// Reject absolute patterns when a cwd is provided: filepath.Join would silently
		// discard cwd and return the absolute path unchanged, bypassing sandbox restrictions.
		if filepath.IsAbs(pattern) {
			return nil, fmt.Errorf("absolute glob patterns not allowed when cwd is set: %s", pattern)
		}
		resolvedCwd, err := p.resolvePath(cwd)
		if err != nil {
			return &ports.ToolResult{
				Content: []ports.ToolContent{{Type: "text", Text: fmt.Sprintf("builtins.glob: %s", err.Error())}},
				IsError: true,
			}, nil
		}
		pattern = filepath.Join(resolvedCwd, pattern)
	}

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	if p.rootDir != "" {
		rootAbs := p.rootAbs
		if rootAbs == "" {
			// rootAbs was not pre-computed (rare: Abs failed during WithRootDir);
			// compute it now and accept the extra syscall.
			var err error
			rootAbs, err = filepath.Abs(p.rootDir)
			if err != nil {
				return nil, fmt.Errorf("builtins.glob: resolve rootDir: %w", err)
			}
		}
		matches = filterPathsWithinRoot(matches, rootAbs)
	}

	return &ports.ToolResult{
		Content: []ports.ToolContent{{Type: "text", Text: strings.Join(matches, "\n")}},
	}, nil
}

// filterPathsWithinRoot returns only the matches whose absolute path resolves within rootAbs.
// rootAbs must already be an absolute, cleaned path (pre-computed by the caller).
// Used to defang globs that could otherwise escape the sandbox via absolute patterns or
// patterns containing `..` that bypass the cwd join.
func filterPathsWithinRoot(matches []string, rootAbs string) []string {
	rootPrefix := rootAbs + string(os.PathSeparator)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		abs, err := filepath.Abs(filepath.Clean(m))
		if err != nil {
			continue
		}
		if abs == rootAbs || strings.HasPrefix(abs, rootPrefix) {
			out = append(out, m)
		}
	}
	return out
}
