// Package roles implements the AgentRoleRepository port for the AWF CLI.
//
// # Overview
//
// This package provides a filesystem-backed repository for discovering and loading
// agent role definitions from AGENTS.md files. Agent roles define who an AI agent
// is (its persona, expertise, and behavior) as opposed to skills, which define what
// it knows. Roles are loaded synchronously and deterministically at workflow
// validation or execution time.
//
// # Search Strategy
//
// The repository follows a multi-path search strategy identical in structure to the
// skills repository. When the AWF_ROLES_PATH environment variable is set, it acts as
// an exclusive override: only the colon-separated list of paths in that variable is
// searched, and the default chain is bypassed entirely.
//
// When AWF_ROLES_PATH is not set, four directories are searched in strict priority
// order (first match wins):
//
//  1. .awf/roles/           — AWF-native local directory (project-level, highest priority)
//  2. .agents/roles/        — cross-client local directory (compatible with other agent tooling)
//  3. $XDG_CONFIG_HOME/awf/roles/ — AWF-native global directory (respects XDG Base Dir spec)
//  4. ~/.agents/roles/      — cross-client global directory (lowest priority)
//
// Paths 1 and 2 are relative to the current working directory (the project root when
// executing awf). Paths 3 and 4 are absolute paths derived from the user environment.
// If UserHomeDir returns an error for path 4, the path is silently omitted rather than
// producing a parasitic join such as "/.agents/roles".
//
// # AWF_ROLES_PATH Environment Variable
//
// Setting AWF_ROLES_PATH replaces the entire default search chain:
//
//	export AWF_ROLES_PATH=/team/roles:/opt/shared/roles
//
// Multiple paths are separated by the OS path list separator (colon on Unix,
// semicolon on Windows). When this variable is set, the four default paths above
// are not consulted at all. To restore the default chain, unset the variable or
// set it to the empty string.
//
// # AGENTS.md File Format
//
// Each role is a directory containing a single AGENTS.md file. The directory name
// becomes the role name. The AGENTS.md file may optionally begin with a YAML
// frontmatter block delimited by --- lines:
//
//	---
//	title: Senior Go Engineer
//	tags: [go, backend]
//	---
//	You are a senior Go engineer with 10+ years of experience...
//
// The frontmatter block is stripped by skills.StripFrontmatter before the content is
// returned to callers. Only the body (everything after the closing ---) is exposed as
// the role content. If no frontmatter is present, the entire file content is returned.
//
// # Dependency on infra-skills
//
// This package reuses skills.StripFrontmatter to strip YAML frontmatter from
// AGENTS.md files. This is an intentional architectural coupling: the frontmatter
// format is identical for both SKILL.md and AGENTS.md, and duplicating the parsing
// logic would violate the single source of truth principle. The dependency is
// declared in .go-arch-lint.yml under the infra-roles component.
//
// # Error Types
//
// The repository returns typed errors from the domain layer:
//
//   - *workflow.AgentRoleNotFoundError: returned when a role cannot be located in any
//     of the configured search paths. The error includes the role name and the full list
//     of paths that were searched, enabling callers to produce actionable diagnostics.
//
// A plain fmt.Errorf (wrapped with %w) is returned for lower-level I/O failures such
// as permission errors when reading an AGENTS.md file that was located via stat.
//
// # Name Validation
//
// The Load method rejects role names containing path separators (/ or \) or the
// double-dot sequence (..) to prevent path traversal attacks. These checks are
// defense-in-depth: the application layer (ResolveAgentRole) and the CLI validator
// also perform independent path traversal rejection.
//
// # Size Warning
//
// When a loaded AGENTS.md file exceeds workflow.AgentRoleSizeWarnBytes (500 KB), a
// structured warning is emitted via the injected ports.Logger. The warning includes
// the file path and size as structured fields and a human-readable message. This
// threshold is defined in the domain layer (domain/workflow/agent_role.go) so that
// both the infrastructure loader and the CLI validator share a single constant.
//
// RawSizeBytes on the returned AgentRole captures the file size before frontmatter
// stripping, enabling CLI-layer size checks without re-reading the file.
//
// # Concurrency
//
// The repository is stateless after construction: all search paths are resolved and
// frozen in NewFilesystemAgentRoleRepository. Individual Load and LoadFromPath calls
// are safe to invoke from concurrent goroutines because they do not mutate shared
// state — they only perform filesystem reads and construct new AgentRole values.
//
// # Relationship to Skills Repository
//
// The roles repository mirrors the design of the skills repository
// (internal/infrastructure/skills). Both repositories:
//
//   - Construct a search path slice at init time from an env var or defaults
//   - Look for a well-known file (AGENTS.md vs SKILL.md) inside a named subdirectory
//   - Strip YAML frontmatter before returning content
//   - Return typed NotFoundError with SearchPaths for diagnostics
//   - Emit a structured size warning via an injected logger
//
// The parallel design is intentional to keep both mechanisms consistent for operators
// and contributors.
//
// # Architecture Constraints
//
// See .go-arch-lint.yml for the declared dependency rules of infra-roles:
//
//   - Allowed: domain-workflow, domain-ports, infra-skills, infra-xdg
//   - Allowed vendors: go-stdlib only
//
// This package must not import application, interfaces, or any external vendor
// libraries.
package roles
