package builtins

import (
	"context"
	"fmt"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
)

var bashSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"command": map[string]any{
			"type": "string",
		},
		"timeout_seconds": map[string]any{
			"type": "integer",
		},
		"cwd": map[string]any{
			"type": "string",
		},
	},
	"required": []string{"command"},
}

func (p *Provider) bashHandler(ctx context.Context, args map[string]any) (*ports.ToolResult, error) {
	if p.executor == nil {
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: "builtins.bash: no executor configured"}},
			IsError: true,
		}, nil
	}

	command, ok := args["command"].(string)
	if !ok {
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: "command must be a string"}},
			IsError: true,
		}, nil
	}

	cwd := ""
	if v, ok := args["cwd"].(string); ok && v != "" {
		resolved, err := p.resolvePath(v)
		if err != nil {
			return &ports.ToolResult{
				Content: []ports.ToolContent{{Type: "text", Text: fmt.Sprintf("builtins.bash: %s", err.Error())}},
				IsError: true,
			}, nil
		}
		cwd = resolved
	}

	if v, ok := args["timeout_seconds"]; ok {
		var secs float64
		switch t := v.(type) {
		case int:
			secs = float64(t)
		case float64:
			secs = t
		}
		if secs > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, time.Duration(secs)*time.Second)
			defer cancel()
		}
	}

	cmd := &ports.Command{
		Program:      command,
		Dir:          cwd,
		IsScriptFile: false,
	}

	result, err := p.executor.Execute(ctx, cmd)
	if err != nil {
		return nil, err
	}

	if result.ExitCode != 0 {
		text := fmt.Sprintf("exit code %d\n%s", result.ExitCode, result.Stderr)
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: text}},
			IsError: true,
		}, nil
	}

	return &ports.ToolResult{
		Content: []ports.ToolContent{{Type: "text", Text: result.Stdout + result.Stderr}},
	}, nil
}
