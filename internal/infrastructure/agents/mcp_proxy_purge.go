package agents

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/pkg/interpolation"
)

// PurgeOrphanMCPRegistrations removes any persistent MCP server registration
// whose name starts with mcpProxyNamePrefix from Gemini and OpenCode CLIs.
//
// Both CLIs are queried via `<cli> mcp list`; matching entries are removed via
// `<cli> mcp remove <name>`. Failures (CLI not installed, no orphans found,
// individual remove fails) are logged at debug level and do NOT block startup.
// Returns nil even on partial failure — purge is best-effort.
//
// Environment variable opt-out: when AWF_MCP_PROXY_NO_PURGE is set to any
// non-empty value the function returns immediately without executing any
// commands. This escape hatch is intended for advanced users who intentionally
// maintain MCP server registrations whose names share the awf-proxy- prefix.
func PurgeOrphanMCPRegistrations(ctx context.Context, exec ports.CommandExecutor, logger ports.Logger) error {
	if os.Getenv("AWF_MCP_PROXY_NO_PURGE") != "" {
		logger.Debug("AWF_MCP_PROXY_NO_PURGE is set; skipping orphan MCP purge")
		return nil
	}

	purgeForCLI(ctx, exec, logger, "gemini", parseGeminiMCPList)
	purgeForCLI(ctx, exec, logger, "opencode", parseOpencodeMCPList)

	return nil
}

// purgeForCLI runs `<cli> mcp list`, parses orphan names, and removes them.
// Any per-CLI or per-entry error is logged at debug level; the function never
// returns an error because purge is best-effort and must not block startup.
func purgeForCLI(ctx context.Context, exec ports.CommandExecutor, logger ports.Logger, cli string, parse func(string) []string) {
	listCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	result, err := exec.Execute(listCtx, &ports.Command{Program: cli + " mcp list"})
	if err != nil {
		logger.Debug("mcp list failed; CLI may not be installed or returned non-zero",
			"cli", cli, "error", err)
		return
	}

	names := parse(result.Stdout)
	for _, name := range names {
		// The name comes from `<cli> mcp list` output, which we parse without strict
		// validation. interpolation.ShellEscape defangs any shell metacharacter that might
		// have slipped through a future format change in the upstream CLI.
		removeErr := func() error {
			removeCtx, removeCancel := context.WithTimeout(ctx, 3*time.Second)
			defer removeCancel()
			_, err := exec.Execute(removeCtx, &ports.Command{Program: cli + " mcp remove " + interpolation.ShellEscape(name)})
			return err
		}()
		if removeErr != nil {
			logger.Debug("failed to remove orphan MCP registration",
				"cli", cli, "name", name, "error", removeErr)
			continue
		}
		logger.Info("purged orphan MCP registration", "cli", cli, "name", name)
	}
}

// parseGeminiMCPList extracts MCP server names matching mcpProxyNamePrefix from
// the output of `gemini mcp list`.
//
// Observed output format (one entry per line):
//
//	✓ awf-proxy-XXXXXXXX: <command...> (stdio) - <state>
//	No MCP servers configured.
//
// The parser is lenient: it trims leading punctuation/whitespace, splits on ':'
// and checks whether the first token matches the prefix. Lines that do not
// contain a colon, or whose first token does not start with mcpProxyNamePrefix,
// are silently skipped. This makes the parser forward-compatible with minor
// formatting changes in future Gemini CLI versions.
func parseGeminiMCPList(output string) []string {
	var names []string
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Strip leading status character(s) and whitespace (e.g. "✓ ", "✗ ", "  ")
		// by finding the first ':' that separates name from the rest of the entry.
		before, _, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		// The segment before the colon may be "✓ awf-proxy-XXXX" — trim whitespace
		// and any non-letter prefix characters to isolate the server name.
		candidate := strings.TrimSpace(before)
		// Drop leading non-alphanumeric characters (status symbols like ✓, ✗, ●).
		candidate = strings.TrimLeft(candidate, "✓✗●◉ \t")
		if strings.HasPrefix(candidate, mcpProxyNamePrefix) {
			names = append(names, candidate)
		}
	}
	return names
}

// parseOpencodeMCPList extracts MCP server names matching mcpProxyNamePrefix from
// the output of `opencode mcp list`.
//
// Assumed output format (based on `opencode mcp --help` and similar CLI conventions):
//
//	awf-proxy-XXXXXXXX   stdio   /path/to/cmd
//	user-server          stdio   /path/to/other
//
// The parser treats the first whitespace-delimited token on each line as the
// server name and checks whether it starts with mcpProxyNamePrefix. If the
// format deviates — e.g. the CLI emits a header row or decorated output — the
// parser silently skips non-matching lines, preserving safety.
//
// Note: If `opencode mcp list` output format changes in a future release, only
// this function needs updating; the purge logic is isolated here.
func parseOpencodeMCPList(output string) []string {
	var names []string
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// First whitespace-separated token is the server name.
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		candidate := fields[0]
		if strings.HasPrefix(candidate, mcpProxyNamePrefix) {
			names = append(names, candidate)
		}
	}
	return names
}
