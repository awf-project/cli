package builtins

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/awf-project/cli/internal/domain/ports"
)

var _ ports.ToolProvider = (*Provider)(nil)

// MaxReadBytes caps how many bytes a single Read or Edit handler will load
// from disk in one call. The agent can still page through a large file via
// the `offset`/`limit` arguments on Read. The cap protects the subprocess
// against prompt-injection that asks the agent to read /dev/zero or another
// arbitrarily-large source, which would otherwise OOM the mcp-serve process.
const MaxReadBytes = 5 * 1024 * 1024 // 5 MiB

type handler func(ctx context.Context, args map[string]any) (*ports.ToolResult, error)

type toolEntry struct {
	definition ports.ToolDefinition
	handler    handler
}

// Provider implements ports.ToolProvider for the built-in file-operation tools.
//
// Tool naming convention: built-in tools intentionally use PascalCase (Read, Write,
// Edit, Bash, Glob, Grep) to align with the names emitted by Anthropic-class agents
// (Claude Code, OpenCode) in their tool_use events. This is the only deliberate
// exception to the plugin convention `<plugin>_<op>` (snake_case) documented in
// ADR 017; plugin-sourced tools continue to follow snake_case. The PascalCase
// alignment makes the proxy a drop-in for the agent's native tools.
type Provider struct {
	tools    map[string]toolEntry
	executor ports.CommandExecutor
	rootDir  string
	// rootAbs is the pre-computed absolute path of rootDir. Computed once in NewProvider
	// via WithRootDir to avoid repeated filepath.Abs calls in every handler invocation.
	// Empty when rootDir is empty (unrestricted mode). NewProvider does not return an error,
	// so if filepath.Abs fails (broken working directory), rootAbs stays empty and resolvePath
	// falls back to computing it per-call, preserving correctness at the cost of one extra syscall.
	rootAbs string
}

// Option configures a Provider at construction time.
type Option func(*Provider)

// WithExecutor injects a CommandExecutor used by the Bash handler.
func WithExecutor(exec ports.CommandExecutor) Option {
	return func(p *Provider) {
		p.executor = exec
	}
}

// WithRootDir restricts all file-touching handlers (Read, Write, Edit, Glob, Grep, Bash cwd)
// to paths under dir. When dir is empty, no restriction is applied — callers that opt out
// must justify the broader access. Production callers (mcp-serve) always set this from the
// proxy config, which defaults to the workspace working directory. Tests may leave it empty
// when intentionally reading paths outside the working directory (e.g. t.TempDir()).
//
// The check is a lexical prefix match on the absolute, cleaned path. It does not
// follow symlinks, which leaves a residual TOCTOU window; callers requiring stronger
// guarantees should run mcp-serve in a chrooted or sandboxed environment.
func WithRootDir(dir string) Option {
	return func(p *Provider) {
		p.rootDir = dir
		// Pre-compute the absolute path so resolvePath avoids a repeated syscall.
		// Failure is intentionally swallowed: if the working directory is unavailable
		// here, resolvePath will re-compute per-call and return the same error then.
		if dir != "" {
			if abs, err := filepath.Abs(dir); err == nil {
				p.rootAbs = abs
			}
		}
	}
}

// NewProvider returns a Provider with Read, Write, Edit, Bash, Glob, and Grep registered.
func NewProvider(opts ...Option) *Provider {
	p := &Provider{
		tools: make(map[string]toolEntry),
	}
	for _, o := range opts {
		o(p)
	}
	p.register("Read",
		"Read a file from disk. Args: path (string, required), offset (int, optional, 0-based line index), limit (int, optional, max lines to read). Returns file contents.",
		readSchema, p.readHandler)
	p.register("Write",
		"Write content to a file. Args: path (string, required), content (string, required). Overwrites existing files atomically. Returns confirmation.",
		writeSchema, p.writeHandler)
	p.register("Edit",
		"Edit a file by replacing a literal string. Args: path, old, new (all required); optional replace_all (bool). Fails if old is absent in the file.",
		editSchema, p.editHandler)
	p.register("Bash",
		"Execute a shell command. Args: command (string, required), cwd (string, optional), timeout_seconds (int, optional). Returns stdout/stderr and exit code.",
		bashSchema, p.bashHandler)
	p.register("Glob",
		"Match files by glob pattern. Args: pattern (string, required), cwd (string, optional, defaults to working directory). Returns a list of matching paths.",
		globSchema, p.globHandler)
	p.register("Grep",
		"Search file contents with a regex. Args: pattern (string, required), path (string, optional), glob (string, optional file glob filter), output_mode (string, optional: content|files_with_matches|count), case_insensitive (bool, optional). Returns matching lines.",
		grepSchema, p.grepHandler)
	return p
}

func (p *Provider) register(name, description string, schema map[string]any, h handler) {
	p.tools[name] = toolEntry{
		definition: ports.ToolDefinition{
			Name:        name,
			Description: description,
			InputSchema: schema,
			Source:      "builtin",
		},
		handler: h,
	}
}

// ListTools returns the definitions of all registered built-in tools.
// Results are sorted by name to ensure deterministic ordering across calls;
// map iteration over p.tools is random.
func (p *Provider) ListTools(_ context.Context) ([]ports.ToolDefinition, error) {
	defs := make([]ports.ToolDefinition, 0, len(p.tools))
	for _, e := range p.tools {
		defs = append(defs, e.definition)
	}
	slices.SortFunc(defs, func(a, b ports.ToolDefinition) int { return cmp.Compare(a.Name, b.Name) })
	return defs, nil
}

// CallTool dispatches to the named tool after validating args against its JSON Schema.
//
// Returns a Go error for unknown tool names and schema-validation failures.
// Returns IsError:true inside ToolResult for execution-level failures (file not found, etc.).
func (p *Provider) CallTool(ctx context.Context, name string, args map[string]any) (*ports.ToolResult, error) {
	entry, ok := p.tools[name]
	if !ok {
		return nil, fmt.Errorf("builtins: tool not found: %s", name)
	}
	if err := validateArgs(entry.definition.InputSchema, args); err != nil {
		return nil, fmt.Errorf("builtins.%s: %w", name, err)
	}
	return entry.handler(ctx, args)
}

// Close is a no-op; the built-in provider holds no external resources.
func (p *Provider) Close(_ context.Context) error {
	return nil
}

// resolvePath cleans rawPath, makes it absolute, and (when rootDir is set) verifies it
// resolves within rootDir. Returns the validated absolute path on success.
//
// The validation is lexical: it does not call filepath.EvalSymlinks, which means
// a symlink crafted before resolvePath runs could still escape the root. The lexical
// check is sufficient for the prompt-injection threat model (an agent emitting raw
// paths in a tool_call) while avoiding the surprises and test fragility EvalSymlinks
// introduces. Operators needing hard isolation should run mcp-serve in a sandbox.
func (p *Provider) resolvePath(rawPath string) (string, error) {
	if rawPath == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(filepath.Clean(rawPath))
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", rawPath, err)
	}
	if p.rootDir == "" {
		return abs, nil
	}
	// Use the pre-computed rootAbs when available; fall back to filepath.Abs for
	// the rare case where rootAbs was not set (e.g. Abs failed during WithRootDir).
	root := p.rootAbs
	if root == "" {
		root, err = filepath.Abs(p.rootDir)
		if err != nil {
			return "", fmt.Errorf("resolve rootDir %q: %w", p.rootDir, err)
		}
	}
	if abs == root {
		return abs, nil
	}
	if !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q is outside rootDir %q", abs, root)
	}
	return abs, nil
}

// validateArgs checks that all required fields declared in the JSON Schema are present.
func validateArgs(schema map[string]any, args map[string]any) error { //nolint:gocritic // paramTypeCombine: schema and args are semantically distinct despite identical types
	required, ok := schema["required"]
	if !ok {
		return nil
	}

	// Fast path: schema["required"] is already []string (the common case for
	// programmatically-constructed schemas like the builtins). This avoids the
	// round-trip JSON marshal/unmarshal when the type is already correct.
	var fields []string
	switch v := required.(type) {
	case []string:
		fields = v
	case []any:
		// YAML-unmarshaled schemas produce []any with string elements.
		fields = make([]string, 0, len(v))
		for _, elem := range v {
			s, ok := elem.(string)
			if !ok {
				return fmt.Errorf("invalid schema: required element is not a string: %T", elem)
			}
			fields = append(fields, s)
		}
	default:
		return fmt.Errorf("invalid schema: required must be []string, got %T", required)
	}

	for _, f := range fields {
		if _, exists := args[f]; !exists {
			return fmt.Errorf("missing required argument: %s", f)
		}
	}
	return nil
}
