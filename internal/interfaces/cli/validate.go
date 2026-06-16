package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/skills"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/spf13/cobra"
)

// agentRoleCombinedPromptWarnBytes is the combined size (role content +
// inline system_prompt, in bytes) above which a non-blocking validation warning
// is emitted. This threshold is a CLI-layer concern: it guards against oversized
// system prompt payloads before execution, not a domain invariant.
const agentRoleCombinedPromptWarnBytes = 10 * 1024

func newValidateCommand(cfg *Config) *cobra.Command {
	var skipPlugins bool
	var validatorTimeout time.Duration
	var packFlag string
	var dirFlag string

	cmd := &cobra.Command{
		Use:   "validate [workflow]",
		Short: "Validate a workflow definition",
		Long: `Validate a workflow YAML file without executing it.

Checks for:
  - Valid YAML syntax
  - Required fields (name, initial state, terminal state)
  - Valid state references
  - No cycles in state transitions (if detectable)

Use --pack to validate all workflows in an installed pack.
Use --dir to validate all .yaml files in a directory.

Examples:
  awf validate my-workflow
  awf validate analyze-code --verbose
  awf validate --pack hello
  awf validate --dir ./workflows/`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case packFlag != "":
				return runValidatePack(cmd, cfg, packFlag, skipPlugins, validatorTimeout)
			case dirFlag != "":
				return runValidateDir(cmd, cfg, dirFlag, skipPlugins, validatorTimeout)
			case len(args) == 1:
				return runValidate(cmd, cfg, args[0], skipPlugins, validatorTimeout)
			default:
				return fmt.Errorf("provide a workflow name, --pack, or --dir")
			}
		},
	}

	cmd.Flags().BoolVar(&skipPlugins, "skip-plugins", false, "Skip plugin validators")
	cmd.Flags().DurationVar(&validatorTimeout, "validator-timeout", 5*time.Second, "Timeout for each plugin validator")
	cmd.Flags().StringVar(&packFlag, "pack", "", "Validate all workflows in an installed pack")
	cmd.Flags().StringVar(&dirFlag, "dir", "", "Validate all .yaml workflow files in a directory")

	return cmd
}

func runValidate(cmd *cobra.Command, cfg *Config, workflowName string, skipPlugins bool, validatorTimeout time.Duration) error {
	ctx := context.Background()

	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)

	// Route through WorkflowFacade when wired (T069). The facade owns identifier
	// resolution and validation.
	if cfg.Facade != nil {
		return runValidateViaFacade(cmd, cfg, writer, ctx, workflowName, skipPlugins, validatorTimeout)
	}

	// Facade not wired: return a meaningful error stub. Production always wires
	// the facade via NewRootCommandAutoFacade, so this branch is never reached in
	// normal usage.
	err := fmt.Errorf("validate requires facade wiring (use NewRootCommandAutoFacade)")
	return writeErrorAndExit(writer, err, ExitSystem)
}

// runValidateViaFacade validates a single workflow through ports.WorkflowFacade.Validate
// (T069). The facade resolves the canonical identifier and reports validity; a resolver
// rejection (e.g. unknown workflow) is returned as an error and mapped to ExitUser,
// while a report carrying validation errors is mapped to ExitWorkflow — matching the
// legacy path's exit-code taxonomy.
func runValidateViaFacade(cmd *cobra.Command, cfg *Config, writer *ui.OutputWriter, ctx context.Context, workflowName string, skipPlugins bool, validatorTimeout time.Duration) error { //nolint:revive // context.Context not first param: writer is a pre-built dependency, not a new chain
	report, err := cfg.Facade.Validate(ctx, ports.RunRequest{
		Identifier:   workflowName,
		ValidateOpts: ports.ValidateOptions{SkipPlugins: skipPlugins, ValidatorTimeout: validatorTimeout},
	})
	if err != nil {
		return writeErrorAndExit(writer, err, ExitUser)
	}

	// Extract human-readable messages from the typed ValidationError slice.
	var errMessages []string
	for _, ve := range report.Errors {
		errMessages = append(errMessages, ve.Message)
	}

	var validationErr error
	if !report.Valid && len(errMessages) > 0 {
		validationErr = fmt.Errorf("%s", strings.Join(errMessages, "\n  "))
	}

	if cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatQuiet {
		result := ui.ValidationResult{
			Valid:    validationErr == nil,
			Workflow: workflowName,
		}
		if validationErr != nil {
			result.Errors = errMessages
		}
		if writeErr := writer.WriteValidation(result); writeErr != nil {
			return writeErr
		}
		if validationErr != nil {
			return &exitError{code: ExitWorkflow, err: validationErr}
		}
		return nil
	}

	if cfg.OutputFormat == ui.FormatTable {
		result := ui.ValidationResultTable{
			Valid:    validationErr == nil,
			Workflow: workflowName,
		}
		if validationErr != nil {
			result.Errors = errMessages
		}
		if writeErr := writer.WriteValidationTable(&result); writeErr != nil {
			return writeErr
		}
		if validationErr != nil {
			return &exitError{code: ExitWorkflow, err: validationErr}
		}
		return nil
	}

	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})
	if validationErr != nil {
		return writeErrorAndExit(writer, validationErr, ExitWorkflow)
	}
	formatter.Success(fmt.Sprintf("Workflow '%s' is valid", workflowName))
	return nil
}

// runValidateDir validates all .yaml workflow files in a directory by routing
// through the ports.BatchValidator port exposed by the facade.
//
// The rendered output is "  OK    <name>" / "  FAIL  <name>: <error>" with a
// summary line and exit-code propagation.
func runValidateDir(cmd *cobra.Command, cfg *Config, dir string, skipPlugins bool, validatorTimeout time.Duration) error {
	ctx := context.Background()

	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	opts := ports.ValidateOptions{SkipPlugins: skipPlugins, ValidatorTimeout: validatorTimeout}

	if bv, ok := cfg.Facade.(ports.BatchValidator); ok {
		results, err := bv.ValidateDir(ctx, dir, opts)
		if err != nil {
			return fmt.Errorf("validate dir %s: %w", dir, err)
		}
		return renderBatchResults(cmd, formatter, results, dir)
	}

	// Facade not wired: return a meaningful stub error. Production always wires
	// the facade via NewRootCommandAutoFacade, so this branch is never reached in
	// normal usage.
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)
	err := fmt.Errorf("validate requires facade wiring (use NewRootCommandAutoFacade)")
	return writeErrorAndExit(writer, err, ExitSystem)
}

// runValidatePack validates all workflows in an installed pack by routing
// through the ports.BatchValidator port exposed by the facade.
func runValidatePack(cmd *cobra.Command, cfg *Config, packName string, skipPlugins bool, validatorTimeout time.Duration) error {
	ctx := context.Background()

	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	opts := ports.ValidateOptions{SkipPlugins: skipPlugins, ValidatorTimeout: validatorTimeout}

	if bv, ok := cfg.Facade.(ports.BatchValidator); ok {
		results, err := bv.ValidatePack(ctx, packName, opts)
		if err != nil {
			return fmt.Errorf("validate pack %q: %w", packName, err)
		}
		return renderBatchResults(cmd, formatter, results, packName)
	}

	// Facade not wired: return a meaningful stub error. Production always wires
	// the facade via NewRootCommandAutoFacade, so this branch is never reached in
	// normal usage.
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)
	err := fmt.Errorf("validate requires facade wiring (use NewRootCommandAutoFacade)")
	return writeErrorAndExit(writer, err, ExitSystem)
}

// renderBatchResults writes per-file OK/FAIL lines and a summary for the output
// produced by ports.BatchValidator methods (ValidateDir and ValidatePack).
// The format is identical to the legacy direct-service path so user-visible
// output is unchanged after the facade routing migration.
func renderBatchResults(cmd *cobra.Command, formatter *ui.Formatter, results []ports.FileValidationResult, scope string) error {
	if len(results) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No .yaml files found in %s\n", scope)
		return nil
	}

	var failed int
	for i := range results {
		r := &results[i]
		if r.Valid {
			formatter.Success(fmt.Sprintf("  OK    %s", r.Name))
		} else {
			var msgs []string
			for j := range r.Errors {
				msgs = append(msgs, r.Errors[j].Message)
			}
			formatter.Error(fmt.Sprintf("  FAIL  %s: %s", r.Name, strings.Join(msgs, "; ")))
			failed++
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d of %d workflow(s) failed validation", failed, len(results))
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nAll %d workflow(s) valid\n", len(results))
	return nil
}

func validateSkillRefs(wf *workflow.Workflow) ([]string, error) {
	hasSkills := false
	for _, step := range wf.Steps {
		if len(step.Skills) > 0 {
			hasSkills = true
			break
		}
	}
	if !hasSkills {
		return nil, nil
	}

	repo := skills.NewFilesystemSkillRepository(nil)
	ctx := context.Background()
	var warnings []string

	for _, step := range wf.Steps {
		for _, ref := range step.Skills {
			var skill *workflow.Skill
			var err error

			if ref.IsPathBased() {
				absPath := ref.Path
				if !filepath.IsAbs(absPath) {
					absPath = filepath.Join(wf.SourceDir, filepath.Clean(absPath))
				}
				skill, err = repo.LoadFromPath(ctx, absPath)
			} else {
				skill, err = repo.Load(ctx, ref.Name)
				if err != nil {
					var notFound *workflow.SkillNotFoundError
					if errors.As(err, &notFound) && skillDirExists(notFound.Name, notFound.SearchPaths) {
						return nil, workflow.ValidationError{
							Level:   workflow.ValidationLevelError,
							Code:    workflow.ErrSkillMissingSkillMD,
							Message: fmt.Sprintf("skill %q has no SKILL.md", notFound.Name),
							Path:    fmt.Sprintf("states.%s.skills", step.Name),
						}
					}
					return nil, err
				}
			}

			if err != nil {
				return nil, err
			}

			if skill.Content == "" {
				warnings = append(warnings, fmt.Sprintf("skill %q has empty content", skill.Name))
			}
		}
	}

	return warnings, nil
}

func validateRoleRefs(ctx context.Context, wf *workflow.Workflow, repo ports.AgentRoleRepository) ([]string, error) {
	var warnings []string

	for _, step := range wf.Steps {
		if step.Agent == nil || step.Agent.Role == "" {
			continue
		}

		role := step.Agent.Role

		// Defense in depth: block path-traversal sequences, mirroring the
		// infrastructure Load guard. application.isRolePathRef routes refs
		// containing '/' or '\' (or starting with '.', '~') to LoadFromPath;
		// pure name-refs are routed to Load, which also rejects '/' and '\'.
		// Since path-refs may legitimately contain separators, we only block
		// '..' here (which is invalid in both name-refs and path-refs).
		// Name-refs containing '/' or '\' are already unreachable: isRolePathRef
		// classifies them as path-refs and LoadFromPath handles them safely via
		// filepath.Clean — no additional guard is needed at this layer.
		if strings.Contains(role, "..") {
			return nil, workflow.ValidationError{
				Level:   workflow.ValidationLevelError,
				Code:    workflow.ErrRoleNotFound,
				Message: "role path contains path-traversal pattern (..): " + role,
				Path:    fmt.Sprintf("states.%s.agent.role", step.Name),
			}
		}

		agentRole, err := application.ResolveAgentRole(ctx, repo, role, wf.SourceDir)
		if err != nil {
			var notFound *workflow.AgentRoleNotFoundError
			if errors.As(err, &notFound) && roleDirExistsWithoutAgentsMD(notFound) {
				return nil, workflow.ValidationError{
					Level:   workflow.ValidationLevelError,
					Code:    workflow.ErrRoleMissingAgentsMD,
					Message: fmt.Sprintf("role %q has no AGENTS.md", notFound.Name),
					Path:    fmt.Sprintf("states.%s.agent.role", step.Name),
				}
			}
			// Use a user-oriented message that identifies the role by name without
			// exposing internal filesystem search paths, which are environment-specific
			// and would pollute machine-readable output (JSON/quiet format). The full
			// path list is available in the underlying AgentRoleNotFoundError for
			// callers that need it.
			roleName := role
			if notFound != nil {
				roleName = notFound.Name
			}
			return nil, workflow.ValidationError{
				Level:   workflow.ValidationLevelError,
				Code:    workflow.ErrRoleNotFound,
				Message: fmt.Sprintf("resolve role %q: not found", roleName),
				Path:    fmt.Sprintf("states.%s.agent.role", step.Name),
			}
		}

		warnings = append(warnings, roleContentWarnings(agentRole, step.Agent.SystemPrompt)...)
	}

	return warnings, nil
}

// roleContentWarnings returns non-blocking warnings about a resolved role's
// AGENTS.md content: empty body, oversized file, or an oversized combined
// role+system_prompt payload. It returns nil when the content is within limits.
//
// The checks are evaluated in priority order and the first match wins, so at
// most one warning is emitted per role. Order matters: a file whose body is
// empty after frontmatter stripping reports "empty body" even if the raw file
// exceeds the size threshold. RawSizeBytes is captured at load time, so no
// filesystem access is needed here.
func roleContentWarnings(role *workflow.AgentRole, systemPrompt string) []string {
	if role.Content == "" {
		return []string{fmt.Sprintf("role %q has empty AGENTS.md body", role.Name)}
	}
	if role.RawSizeBytes > workflow.AgentRoleSizeWarnBytes {
		return []string{fmt.Sprintf("role %q: AGENTS.md exceeds %dKB size threshold", role.Name, workflow.AgentRoleSizeWarnBytes/1024)}
	}
	if len(role.Content)+len(systemPrompt) > agentRoleCombinedPromptWarnBytes {
		return []string{fmt.Sprintf("role %q: combined role content and system_prompt exceeds %dKB threshold", role.Name, agentRoleCombinedPromptWarnBytes/1024)}
	}
	return nil
}

func skillDirExists(name string, searchPaths []string) bool {
	for _, searchPath := range searchPaths {
		if _, err := os.Stat(filepath.Join(searchPath, name)); err == nil {
			return true
		}
	}
	return false
}

// roleDirExistsWithoutAgentsMD checks whether the role directory exists but
// is missing an AGENTS.md file. For path-based refs (IsPathRef=true), the
// single search path equals the role directory itself, so it is checked
// directly. For name-based refs, the name is joined into each search path.
func roleDirExistsWithoutAgentsMD(notFound *workflow.AgentRoleNotFoundError) bool {
	if notFound.IsPathRef {
		if len(notFound.SearchPaths) == 0 {
			return false
		}
		dir := notFound.SearchPaths[0]
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return true
		}
		return false
	}
	return skillDirExists(notFound.Name, notFound.SearchPaths)
}
