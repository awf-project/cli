package builtins

import (
	"context"
	"fmt"
	"strings"

	"github.com/awf-project/cli/internal/domain/ports"
)

var editSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"path": map[string]any{
			"type": "string",
		},
		"old": map[string]any{
			"type": "string",
		},
		"new": map[string]any{
			"type": "string",
		},
		"replace_all": map[string]any{
			"type": "boolean",
		},
	},
	"required": []string{"path", "old", "new"},
}

func (p *Provider) editHandler(_ context.Context, args map[string]any) (*ports.ToolResult, error) {
	pathVal, ok := args["path"].(string)
	if !ok {
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: "path must be a string"}},
			IsError: true,
		}, nil
	}
	oldStr, ok := args["old"].(string)
	if !ok {
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: "old must be a string"}},
			IsError: true,
		}, nil
	}
	newStr, ok := args["new"].(string)
	if !ok {
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: "new must be a string"}},
			IsError: true,
		}, nil
	}
	path, err := p.resolvePath(pathVal)
	if err != nil {
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: fmt.Sprintf("builtins.edit: %s", err.Error())}},
			IsError: true,
		}, nil
	}

	replaceAll := false
	if v, ok := args["replace_all"]; ok {
		if b, ok := v.(bool); ok {
			replaceAll = b
		}
	}

	data, truncated, err := readCappedFile(path)
	if err != nil {
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: fmt.Sprintf("builtins.edit: %s", err.Error())}},
			IsError: true,
		}, nil
	}
	if truncated {
		// Edit on a truncated read would silently drop the tail of the file on rewrite —
		// refuse rather than corrupt.
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: fmt.Sprintf("builtins.edit: file exceeds %d bytes; refuse to edit truncated content", MaxReadBytes)}},
			IsError: true,
		}, nil
	}

	content := string(data)
	if !strings.Contains(content, oldStr) {
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: "builtins.edit: old string not found"}},
			IsError: true,
		}, nil
	}

	var updated string
	if replaceAll {
		updated = strings.ReplaceAll(content, oldStr, newStr)
	} else {
		updated = strings.Replace(content, oldStr, newStr, 1)
	}

	if err := atomicWrite(path, []byte(updated)); err != nil {
		return nil, fmt.Errorf("builtins.edit: %w", err)
	}

	return &ports.ToolResult{
		Content: []ports.ToolContent{{Type: "text", Text: "OK"}},
		IsError: false,
	}, nil
}
