package builtins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
)

var writeSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"path": map[string]any{
			"type": "string",
		},
		"content": map[string]any{
			"type": "string",
		},
	},
	"required": []string{"path", "content"},
}

// MaxWriteBytes caps the maximum content size for a single Write call.
// Matching MaxReadBytes (5 MiB) so an agent cannot trivially allocate unbounded
// memory by writing a file larger than what it could read back.
const MaxWriteBytes = 5 * 1024 * 1024 // 5 MiB

func (p *Provider) writeHandler(_ context.Context, args map[string]any) (*ports.ToolResult, error) {
	pathVal, ok := args["path"].(string)
	if !ok {
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: "path must be a string"}},
			IsError: true,
		}, nil
	}
	content, ok := args["content"].(string)
	if !ok {
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: "content must be a string"}},
			IsError: true,
		}, nil
	}
	path, err := p.resolvePath(pathVal)
	if err != nil {
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: fmt.Sprintf("builtins.write: %s", err.Error())}},
			IsError: true,
		}, nil
	}

	if len(content) > MaxWriteBytes {
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: fmt.Sprintf("builtins.write: content exceeds %d bytes limit", MaxWriteBytes)}},
			IsError: true,
		}, nil
	}

	if err := atomicWrite(path, []byte(content)); err != nil {
		return nil, fmt.Errorf("builtins.write: %w", err)
	}

	return &ports.ToolResult{
		Content: []ports.ToolContent{{Type: "text", Text: "OK"}},
		IsError: false,
	}, nil
}

// atomicWrite writes data to path using a temp file + rename to prevent partial writes.
// The temp file uses PID+timestamp to avoid collisions from concurrent calls.
// Parent directories are created automatically (0755) when they do not exist.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // G301: 0755 is standard for directories; write access is controlled by rootDir guard in resolvePath
		return err
	}
	tmp := filepath.Join(dir, fmt.Sprintf(".awf_write_%d_%d.tmp", os.Getpid(), time.Now().UnixNano()))

	if err := os.WriteFile(tmp, data, 0o644); err != nil { //nolint:gosec // G306: 0644 is standard for user-created files; temp file is renamed atomically
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
