package builtins

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/awf-project/cli/internal/domain/ports"
)

var readSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"path": map[string]any{
			"type": "string",
		},
		"offset": map[string]any{
			"type": "integer",
		},
		"limit": map[string]any{
			"type": "integer",
		},
	},
	"required": []string{"path"},
}

func (p *Provider) readHandler(_ context.Context, args map[string]any) (*ports.ToolResult, error) {
	pathVal, ok := args["path"].(string)
	if !ok {
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: "path must be a string"}},
			IsError: true,
		}, nil
	}
	path, err := p.resolvePath(pathVal)
	if err != nil {
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: fmt.Sprintf("builtins.read: %s", err.Error())}},
			IsError: true,
		}, nil
	}

	data, truncated, err := readCappedFile(path)
	if err != nil {
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: fmt.Sprintf("builtins.read: %s", err.Error())}},
			IsError: true,
		}, nil
	}

	lines := splitLines(data)

	offset := 0
	if v, ok := args["offset"]; ok {
		if n, ok := toInt(v); ok {
			offset = n
		}
	}
	if offset > len(lines) {
		offset = len(lines)
	}
	lines = lines[offset:]

	if v, ok := args["limit"]; ok {
		if n, ok := toInt(v); ok && n < len(lines) {
			lines = lines[:n]
		}
	}

	text := strings.Join(lines, "")
	if truncated {
		text += fmt.Sprintf("\n[builtins.read: truncated at %d bytes; use offset/limit to page]", MaxReadBytes)
	}

	return &ports.ToolResult{
		Content: []ports.ToolContent{{Type: "text", Text: text}},
		IsError: false,
	}, nil
}

// readCappedFile reads up to MaxReadBytes from path. truncated is true when the
// file was longer than MaxReadBytes and data is the first MaxReadBytes bytes.
// One extra byte is read beyond the cap to detect truncation reliably.
func readCappedFile(path string) (data []byte, truncated bool, err error) {
	f, err := os.Open(path) //nolint:gosec // G304: path has been validated by resolvePath against rootDir
	if err != nil {
		return nil, false, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	limited := io.LimitReader(f, int64(MaxReadBytes)+1)
	data, err = io.ReadAll(limited)
	if err != nil {
		return nil, false, fmt.Errorf("read: %w", err)
	}
	if len(data) > MaxReadBytes {
		return data[:MaxReadBytes], true, nil
	}
	return data, false, nil
}

func splitLines(data []byte) []string {
	if len(data) == 0 {
		return []string{""}
	}
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, string(data[start:i+1]))
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, string(data[start:]))
	}
	return lines
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case float32:
		return int(n), true
	}
	return 0, false
}
