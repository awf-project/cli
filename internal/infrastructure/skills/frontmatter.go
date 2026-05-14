package skills

import "strings"

func StripFrontmatter(content string) string {
	const open = "---\n"

	if !strings.HasPrefix(content, open) {
		return content
	}

	rest := content[len(open):]

	if _, body, found := strings.Cut(rest, "\n---\n"); found {
		return strings.TrimSpace(body)
	}

	if after, found := strings.CutSuffix(rest, "\n---"); found {
		_ = after
		return ""
	}

	return content
}
