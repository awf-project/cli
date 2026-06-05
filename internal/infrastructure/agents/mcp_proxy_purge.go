package agents

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/pkg/interpolation"
)

const (
	// mcpListDefaultTimeout bounds a single `<cli> mcp list` call. Purge runs at
	// startup, so this must stay small enough not to noticeably delay the run, yet
	// large enough for a healthy CLI to answer. Override via mcpListTimeoutEnv.
	mcpListDefaultTimeout = 5 * time.Second
	// mcpRemoveTimeout bounds a single `<cli> mcp remove <name>` call.
	mcpRemoveTimeout = 3 * time.Second
	// mcpListTimeoutEnv lets advanced users widen (or tighten) the list timeout,
	// e.g. AWF_MCP_PROXY_LIST_TIMEOUT=15s for a CLI that is slow to enumerate.
	mcpListTimeoutEnv = "AWF_MCP_PROXY_LIST_TIMEOUT"
	// exitCodeCommandNotFound is the conventional shell exit code (127) returned
	// when the CLI binary is not on PATH. Because we run via `sh -c "<cli> ..."`,
	// a missing binary surfaces as this exit code with a nil error rather than a
	// Go execution error — so we detect "not installed" here, not in the error path.
	exitCodeCommandNotFound = 127
)

// PurgeOrphanMCPRegistrations removes any persistent MCP server registration
// whose name starts with mcpProxyNamePrefix from Gemini and OpenCode CLIs.
//
// Both CLIs are queried via `<cli> mcp list`; matching entries are removed via
// `<cli> mcp remove <name>`. Each failure mode is classified and logged distinctly
// (see purgeForCLI) and none block startup — the function always returns nil because
// purge is best-effort.
//
// Environment variables:
//   - AWF_MCP_PROXY_NO_PURGE: when set to any non-empty value, returns immediately
//     without executing any commands (for users who intentionally keep awf-proxy-
//     prefixed registrations).
//   - AWF_MCP_PROXY_LIST_TIMEOUT: overrides the per-CLI `mcp list` timeout (Go
//     duration, e.g. "10s"). Defaults to mcpListDefaultTimeout.
func PurgeOrphanMCPRegistrations(ctx context.Context, exec ports.CommandExecutor, logger ports.Logger) error {
	if os.Getenv("AWF_MCP_PROXY_NO_PURGE") != "" {
		logger.Debug("AWF_MCP_PROXY_NO_PURGE is set; skipping orphan MCP purge")
		return nil
	}

	timeout := resolveListTimeout(logger)
	purgeForCLI(ctx, exec, logger, "gemini", parseGeminiMCPList, timeout)
	purgeForCLI(ctx, exec, logger, "opencode", parseOpencodeMCPList, timeout)

	return nil
}

// resolveListTimeout reads AWF_MCP_PROXY_LIST_TIMEOUT, falling back to the default
// on an unset, unparseable, or non-positive value.
func resolveListTimeout(logger ports.Logger) time.Duration {
	raw := os.Getenv(mcpListTimeoutEnv)
	if raw == "" {
		return mcpListDefaultTimeout
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		logger.Debug("invalid AWF_MCP_PROXY_LIST_TIMEOUT; using default",
			"value", raw, "default", mcpListDefaultTimeout)
		return mcpListDefaultTimeout
	}
	return d
}

// purgeForCLI runs `<cli> mcp list`, parses orphan names, and removes them. It
// distinguishes the failure modes so the logs are actionable rather than guessing:
//
//   - timeout (context deadline): the CLI is installed but did not answer in time;
//     logged at WARN with a hint to widen the timeout or opt out, since orphans are
//     left in place this run.
//   - execution error: the command could not be launched at all; DEBUG.
//   - exit 127: the CLI is not installed (shell could not find it); DEBUG, expected.
//   - other non-zero exit: the CLI ran but reported an error; DEBUG with stderr.
//
// The function never returns an error: purge is best-effort and must not block startup.
func purgeForCLI(ctx context.Context, exec ports.CommandExecutor, logger ports.Logger, cli string, parse func(string) []string, timeout time.Duration) {
	listCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := exec.Execute(listCtx, &ports.Command{Program: cli + " mcp list"})
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		logger.Warn("mcp purge: `mcp list` timed out; orphan registrations left untouched this run "+
			"(raise AWF_MCP_PROXY_LIST_TIMEOUT, or set AWF_MCP_PROXY_NO_PURGE=1 to disable purge)",
			"cli", cli, "timeout", timeout)
		return
	case err != nil:
		logger.Debug("mcp purge: `mcp list` could not be executed", "cli", cli, "error", err)
		return
	case result.ExitCode == exitCodeCommandNotFound:
		logger.Debug("mcp purge: CLI not installed; nothing to purge", "cli", cli)
		return
	case result.ExitCode != 0:
		logger.Debug("mcp purge: `mcp list` exited non-zero; skipping",
			"cli", cli, "exit_code", result.ExitCode, "stderr", firstLine(result.Stderr))
		return
	}

	names := parse(result.Stdout)
	for _, name := range names {
		// The name comes from `<cli> mcp list` output, which we parse without strict
		// validation. interpolation.ShellEscape defangs any shell metacharacter that might
		// have slipped through a future format change in the upstream CLI.
		removeErr := func() error {
			removeCtx, removeCancel := context.WithTimeout(ctx, mcpRemoveTimeout)
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

// firstLine returns the first non-empty line of s, trimmed. Used to keep stderr
// snippets in logs to a single line instead of dumping multi-line CLI output.
func firstLine(s string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(first)
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
