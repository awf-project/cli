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
	"github.com/awf-project/cli/internal/infrastructure/analyzer"
	"github.com/awf-project/cli/internal/infrastructure/expression"
	infrastructurePlugin "github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/roles"
	"github.com/awf-project/cli/internal/infrastructure/skills"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/spf13/cobra"
)

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

	// Create output writer
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)

	// Detect pack/workflow namespace syntax
	packName, baseName := parseWorkflowNamespace(workflowName)
	var repo *repository.CompositeRepository
	if packName != "" {
		packDir := findPackDir(packName)
		if packDir == "" {
			return writeErrorAndExit(writer, fmt.Errorf("workflow pack %q not found", packName), ExitUser)
		}
		workflowsDir := filepath.Join(packDir, "workflows")
		repo = repository.NewCompositeRepository([]repository.SourcedPath{
			{Path: workflowsDir, Source: repository.SourceLocal},
		})
		workflowName = baseName
	} else {
		repo = NewWorkflowRepository()
	}

	// Create validator
	validator := expression.NewExprValidator()

	// Create service
	svc := application.NewWorkflowService(repo, nil, nil, nil, validator)

	// Inject an OperationProvider so that mcp_proxy.plugin_tools checks run.
	// A CompositeOperationProvider with no sub-providers returns empty results,
	// which causes UNKNOWN_PLUGIN errors for any plugin reference — the correct
	// behavior when no plugins are installed in the current environment.
	svc.SetPluginOperationProvider(infrastructurePlugin.NewCompositeOperationProvider())

	// Load workflow first to check existence
	wf, err := svc.GetWorkflow(ctx, workflowName)
	if err != nil {
		return writeErrorAndExit(writer, err, ExitUser)
	}

	// Validate workflow structure
	validationErr := svc.ValidateWorkflow(ctx, workflowName)

	// If workflow structure is valid, validate template interpolation references
	if validationErr == nil {
		templateAnalyzer := analyzer.NewInterpolationAnalyzer()
		templateValidator := workflow.NewTemplateValidator(wf, templateAnalyzer)
		result := templateValidator.Validate()
		if result != nil && result.HasErrors() {
			// Format all errors for display
			if len(result.Errors) == 1 {
				validationErr = result.Errors[0]
			} else {
				// Create a multi-line error with all validation errors
				var sb strings.Builder
				fmt.Fprintf(&sb, "validation failed with %d errors:", len(result.Errors))
				for _, err := range result.Errors {
					fmt.Fprintf(&sb, "\n  %s", err.Error())
				}
				validationErr = fmt.Errorf("%s", sb.String())
			}
		}
	}

	// If workflow is valid, also validate template references
	if validationErr == nil {
		templatePaths := []string{
			".awf/templates",
			filepath.Join(cfg.StoragePath, "templates"),
		}
		templateRepo := repository.NewYAMLTemplateRepository(templatePaths)
		templateSvc := application.NewTemplateService(templateRepo, nil)

		for name, step := range wf.Steps {
			if step.TemplateRef != nil {
				if err := templateSvc.ValidateTemplateRef(ctx, step.TemplateRef); err != nil {
					validationErr = fmt.Errorf("step %q: %w", name, err)
					break
				}
			}
		}
	}

	var skillWarnings []string
	if validationErr == nil {
		skillWarnings, validationErr = validateSkillRefs(wf)
	}

	var roleWarnings []string
	if validationErr == nil {
		roleRepo := roles.NewFilesystemAgentRoleRepository(nil)
		roleWarnings, validationErr = validateRoleRefs(wf, roleRepo)
	}

	// Collect disabled plugin warnings (non-blocking, skipped on quiet format or when --skip-plugins)
	var pluginWarnings []string
	if !skipPlugins && validationErr == nil && cfg.OutputFormat != ui.FormatQuiet {
		pluginWarnings = collectDisabledPluginWarnings(ctx, cfg, wf)
	}
	pluginWarnings = append(pluginWarnings, skillWarnings...)
	pluginWarnings = append(pluginWarnings, roleWarnings...)

	// validatorTimeout is reserved for plugin validator invocation (wired in GREEN phase)
	_ = validatorTimeout

	// JSON/quiet format
	if cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatQuiet {
		result := ui.ValidationResult{
			Valid:    validationErr == nil,
			Workflow: workflowName,
			Warnings: pluginWarnings,
		}
		if validationErr != nil {
			result.Errors = []string{validationErr.Error()}
		}
		if err := writer.WriteValidation(result); err != nil {
			return err
		}
		if validationErr != nil {
			return &exitError{code: ExitWorkflow, err: validationErr}
		}
		return nil
	}

	// Table format: use structured output with inputs and steps
	if cfg.OutputFormat == ui.FormatTable {
		result := ui.ValidationResultTable{
			Valid:    validationErr == nil,
			Workflow: workflowName,
			Warnings: pluginWarnings,
		}
		if validationErr != nil {
			result.Errors = []string{validationErr.Error()}
		}
		// Build inputs info
		for _, inp := range wf.Inputs {
			defaultVal := ""
			if inp.Default != nil {
				defaultVal = fmt.Sprintf("%v", inp.Default)
			}
			result.Inputs = append(result.Inputs, ui.InputInfo{
				Name:     inp.Name,
				Type:     inp.Type,
				Required: inp.Required,
				Default:  defaultVal,
			})
		}
		// Build steps info
		for _, step := range wf.Steps {
			next := step.OnSuccess
			if next == "" {
				if step.Type == "terminal" {
					next = "(terminal)"
				} else {
					next = "-"
				}
			}
			result.Steps = append(result.Steps, ui.StepSummary{
				Name: step.Name,
				Type: string(step.Type),
				Next: next,
			})
		}
		if err := writer.WriteValidationTable(&result); err != nil {
			return err
		}
		if validationErr != nil {
			return &exitError{code: ExitWorkflow, err: validationErr}
		}
		return nil
	}

	// Text format
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	if validationErr != nil {
		return writeErrorAndExit(writer, validationErr, ExitWorkflow)
	}

	// Show success
	formatter.Success(fmt.Sprintf("Workflow '%s' is valid", workflowName))

	// Print disabled plugin warnings
	for _, warning := range pluginWarnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "\n  warning: %s", warning)
		fmt.Fprintf(cmd.ErrOrStderr(), "\n  Hint: Run 'awf plugin enable <name>' to re-enable")
	}
	if len(pluginWarnings) > 0 {
		fmt.Fprintln(cmd.ErrOrStderr())
	}

	// Verbose output
	if cfg.Verbose {
		formatter.Println()
		formatter.Printf("Name:        %s\n", wf.Name)
		if wf.Version != "" {
			formatter.Printf("Version:     %s\n", wf.Version)
		}
		if wf.Description != "" {
			formatter.Printf("Description: %s\n", wf.Description)
		}
		formatter.Printf("Initial:     %s\n", wf.Initial)
		formatter.Printf("Steps:       %d\n", len(wf.Steps))

		if len(wf.Inputs) > 0 {
			formatter.Println()
			formatter.Println("Inputs:")
			for _, input := range wf.Inputs {
				required := ""
				if input.Required {
					required = " (required)"
				}
				formatter.Printf("  - %s: %s%s\n", input.Name, input.Type, required)
			}
		}
	}

	return nil
}

// runValidateDir validates all .yaml workflow files in a directory.
func runValidateDir(cmd *cobra.Command, cfg *Config, dir string, _ bool, _ time.Duration) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read directory %s: %w", dir, err)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			names = append(names, strings.TrimSuffix(e.Name(), ".yaml"))
		}
	}

	if len(names) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No .yaml files found in %s\n", dir)
		return nil
	}

	repo := repository.NewCompositeRepository([]repository.SourcedPath{
		{Path: absDir, Source: repository.SourceLocal},
	})
	validator := expression.NewExprValidator()
	svc := application.NewWorkflowService(repo, nil, nil, nil, validator)

	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	var failed int
	for _, name := range names {
		if err := svc.ValidateWorkflow(context.Background(), name); err != nil {
			formatter.Error(fmt.Sprintf("  FAIL  %s: %s", name, err))
			failed++
		} else {
			formatter.Success(fmt.Sprintf("  OK    %s", name))
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d of %d workflow(s) failed validation", failed, len(names))
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nAll %d workflow(s) valid\n", len(names))
	return nil
}

// runValidatePack validates all workflows in an installed pack.
func runValidatePack(cmd *cobra.Command, cfg *Config, packName string, skipPlugins bool, validatorTimeout time.Duration) error {
	packDir := findPackDir(packName)
	if packDir == "" {
		return fmt.Errorf("workflow pack %q not found", packName)
	}

	workflowsDir := filepath.Join(packDir, "workflows")
	return runValidateDir(cmd, cfg, workflowsDir, skipPlugins, validatorTimeout)
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

func validateRoleRefs(wf *workflow.Workflow, repo ports.AgentRoleRepository) ([]string, error) {
	ctx := context.Background()
	var warnings []string

	for _, step := range wf.Steps {
		if step.Agent == nil || step.Agent.Role == "" {
			continue
		}

		role := step.Agent.Role

		// Defense in depth: reject path-traversal even though domain + infra also reject it.
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
			if errors.As(err, &notFound) {
				if roleDirExistsWithoutAgentsMD(notFound) {
					return nil, workflow.ValidationError{
						Level:   workflow.ValidationLevelError,
						Code:    workflow.ErrRoleMissingAgentsMD,
						Message: fmt.Sprintf("role %q has no AGENTS.md", notFound.Name),
						Path:    fmt.Sprintf("states.%s.agent.role", step.Name),
					}
				}
				return nil, workflow.ValidationError{
					Level:   workflow.ValidationLevelError,
					Code:    workflow.ErrRoleNotFound,
					Message: err.Error(),
					Path:    fmt.Sprintf("states.%s.agent.role", step.Name),
				}
			}
			return nil, workflow.ValidationError{
				Level:   workflow.ValidationLevelError,
				Code:    workflow.ErrRoleNotFound,
				Message: err.Error(),
				Path:    fmt.Sprintf("states.%s.agent.role", step.Name),
			}
		}

		if agentRole.Content == "" {
			warnings = append(warnings, fmt.Sprintf("[%s] role %q has empty AGENTS.md body", workflow.ErrRoleEmptyContent, agentRole.Name))
		} else {
			var rawSize int64
			if agentRole.SourcePath != "" {
				if info, statErr := os.Stat(agentRole.SourcePath); statErr == nil {
					rawSize = info.Size()
				}
			}
			if rawSize > 500*1024 {
				warnings = append(warnings, fmt.Sprintf("role %q: AGENTS.md exceeds 500KB size threshold", agentRole.Name))
			} else if len(agentRole.Content)+len(step.Agent.SystemPrompt) > 10*1024 {
				warnings = append(warnings, fmt.Sprintf("role %q: combined role content and system_prompt exceeds 10KB threshold", agentRole.Name))
			}
		}
	}

	return warnings, nil
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
// is missing an AGENTS.md file. For path-based refs (single search path that
// equals the role path itself), it checks the path directly rather than
// joining name into the path (which would double-nest).
func roleDirExistsWithoutAgentsMD(notFound *workflow.AgentRoleNotFoundError) bool {
	if len(notFound.SearchPaths) == 1 {
		dir := notFound.SearchPaths[0]
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return true
		}
		return false
	}
	return skillDirExists(notFound.Name, notFound.SearchPaths)
}

// collectDisabledPluginWarnings checks operation steps for disabled plugin references.
// Returns warning strings for each operation whose plugin prefix is explicitly disabled.
// Gracefully returns nil if the plugin system cannot be initialized.
func collectDisabledPluginWarnings(ctx context.Context, cfg *Config, wf *workflow.Workflow) []string {
	result, err := initPluginSystemReadOnly(ctx, cfg)
	if err != nil {
		return nil
	}
	defer result.Cleanup()

	var warnings []string
	for _, step := range wf.Steps {
		if step.Type != workflow.StepTypeOperation {
			continue
		}
		parts := strings.SplitN(step.Operation, ".", 2)
		if len(parts) < 2 {
			continue
		}
		pluginPrefix := parts[0]
		if !result.Service.IsPluginEnabled(pluginPrefix) {
			warnings = append(warnings, fmt.Sprintf(
				"step %q uses operation %q but plugin %q is disabled",
				step.Name, step.Operation, pluginPrefix,
			))
		}
	}
	return warnings
}
