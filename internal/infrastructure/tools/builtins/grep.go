package builtins

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/awf-project/cli/internal/domain/ports"
)

// maxGrepLineBytes is the per-line scanner buffer ceiling for grepFile. The bufio.Scanner
// default (64 KiB) is too small for minified JS, large JSON blobs, or base64-encoded content
// that appears as a single line. 1 MiB is a generous upper bound that prevents OOM while
// handling real-world source files without silently truncating or returning scanner errors.
const maxGrepLineBytes = 1 * 1024 * 1024

// MaxGrepLines caps the number of matching lines accumulated in "content" mode.
// This prevents grepHandler from building an unbounded in-memory slice when a
// regex matches most lines of a large file tree (e.g., grepping for "." across
// the entire workspace). The "files_with_matches" and "count" modes are not
// bounded here because they accumulate file paths or a single integer rather
// than full line content.
const MaxGrepLines = 10_000

var grepSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"pattern": map[string]any{
			"type": "string",
		},
		"path": map[string]any{
			"type": "string",
		},
		"glob": map[string]any{
			"type": "string",
		},
		"output_mode": map[string]any{
			"type": "string",
		},
		"case_insensitive": map[string]any{
			"type": "boolean",
		},
	},
	"required": []string{"pattern"},
}

func (p *Provider) grepHandler(_ context.Context, args map[string]any) (*ports.ToolResult, error) {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: "pattern must be a string"}},
			IsError: true,
		}, nil
	}

	if ci, ok := args["case_insensitive"].(bool); ok && ci {
		pattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("builtins.grep: %w", err)
	}

	searchPath := "."
	if v, ok := args["path"].(string); ok && v != "" {
		searchPath = v
	}
	resolvedSearchPath, err := p.resolvePath(searchPath)
	if err != nil {
		return &ports.ToolResult{
			Content: []ports.ToolContent{{Type: "text", Text: fmt.Sprintf("builtins.grep: %s", err.Error())}},
			IsError: true,
		}, nil
	}

	globFilter := ""
	if g, ok := args["glob"].(string); ok {
		globFilter = g
	}
	outputMode := "content"
	if m, ok := args["output_mode"].(string); ok && m != "" {
		outputMode = m
	}

	contentLines, matchedFiles, totalCount, truncated, err := grepSearch(re, resolvedSearchPath, globFilter, outputMode)
	if err != nil {
		return nil, err
	}

	var text string
	switch outputMode {
	case "files_with_matches":
		text = strings.Join(matchedFiles, "\n")
	case "count":
		text = fmt.Sprintf("%d", totalCount)
	default:
		text = strings.Join(contentLines, "\n")
		if truncated {
			text += fmt.Sprintf("\n[builtins.grep: truncated at %d lines; refine your pattern or use a narrower path/glob]", MaxGrepLines)
		}
	}

	return &ports.ToolResult{
		Content: []ports.ToolContent{{Type: "text", Text: text}},
	}, nil
}

func grepSearch(re *regexp.Regexp, searchPath, globFilter, outputMode string) (contentLines []string, matchedFiles []string, totalCount int, truncated bool, err error) { //nolint:gocritic // unnamedResult: call site binds to (contentLines, matchedFiles, count, truncated, err) making intent clear
	info, statErr := os.Stat(searchPath)
	if statErr != nil {
		return nil, nil, 0, false, statErr
	}

	if !info.IsDir() {
		err = grepFile(searchPath, re, outputMode, &contentLines, &matchedFiles, &totalCount, &truncated)
		return contentLines, matchedFiles, totalCount, truncated, err
	}

	err = filepath.WalkDir(searchPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if globFilter != "" {
			matched, matchErr := filepath.Match(globFilter, filepath.Base(path))
			if matchErr != nil {
				return matchErr
			}
			if !matched {
				return nil
			}
		}
		// Stop walking when the content line limit has been reached; additional
		// files would only add more lines but the truncation message is already set.
		if truncated {
			return filepath.SkipAll
		}
		return grepFile(path, re, outputMode, &contentLines, &matchedFiles, &totalCount, &truncated)
	})
	if err != nil {
		return nil, nil, 0, false, err
	}

	return contentLines, matchedFiles, totalCount, truncated, nil
}

func grepFile(path string, re *regexp.Regexp, outputMode string, contentLines, matchedFiles *[]string, totalCount *int, truncated *bool) error { //nolint:gocritic // paramTypeCombine: contentLines and matchedFiles are semantically distinct slices
	f, err := os.Open(path) //nolint:gosec // path comes from WalkDir traversal under a rootDir-validated searchPath
	if err != nil {
		return err
	}
	defer f.Close()

	fileMatched := false
	scanner := bufio.NewScanner(f)
	// Grow the scanner buffer from the default 64 KiB up to maxGrepLineBytes so
	// files with very long lines (minified JS, base64 blobs) do not trip
	// bufio.ErrTooLong and silently abort the grep for that file.
	scanner.Buffer(make([]byte, 64*1024), maxGrepLineBytes)
	for scanner.Scan() {
		// Stop accumulating content lines once the cap is reached. totalCount still
		// increments so callers can observe how many matches were found beyond the cap.
		if outputMode == "content" && len(*contentLines) >= MaxGrepLines {
			if !*truncated {
				*truncated = true
			}
		}
		line := scanner.Text()
		if re.MatchString(line) {
			*totalCount++
			if outputMode == "content" && !*truncated {
				*contentLines = append(*contentLines, line)
			}
			if !fileMatched {
				fileMatched = true
				if outputMode == "files_with_matches" {
					*matchedFiles = append(*matchedFiles, path)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("builtins.grep: %w", err)
	}
	return nil
}
