package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	apptools "github.com/awf-project/cli/internal/application/tools"
	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

type WorkflowService struct {
	repo                   ports.WorkflowRepository
	store                  ports.StateStore
	executor               ports.CommandExecutor
	logger                 ports.Logger
	validator              ports.ExpressionValidator
	validatorProvider      ports.WorkflowValidatorProvider
	packDiscoverer         ports.PackDiscoverer
	opProvider             ports.OperationProvider
	templateAnalyzer       workflow.TemplateAnalyzer
	skillRepo              ports.SkillRepository
	lastValidationWarnings []workflow.ValidationError
}

func NewWorkflowService(
	repo ports.WorkflowRepository,
	store ports.StateStore,
	executor ports.CommandExecutor,
	logger ports.Logger,
	validator ports.ExpressionValidator,
) *WorkflowService {
	return &WorkflowService{
		repo:      repo,
		store:     store,
		executor:  executor,
		logger:    logger,
		validator: validator,
	}
}

func (s *WorkflowService) SetValidatorProvider(p ports.WorkflowValidatorProvider) {
	s.validatorProvider = p
}

func (s *WorkflowService) SetPackDiscoverer(d ports.PackDiscoverer) {
	s.packDiscoverer = d
}

func (s *WorkflowService) SetPluginOperationProvider(p ports.OperationProvider) {
	s.opProvider = p
}

// SetTemplateAnalyzer wires the analyzer used by the template-reference validation
// phase (forward references, unknown namespaces, {{error.*}} outside error hooks).
// It is an optional dependency: when no analyzer is wired, ValidateLoadedWorkflow
// skips that phase, preserving read-only-facade behavior for callers that only need
// the structural and expression-compilation checks.
func (s *WorkflowService) SetTemplateAnalyzer(a workflow.TemplateAnalyzer) {
	s.templateAnalyzer = a
}

func (s *WorkflowService) SetSkillRepository(repo ports.SkillRepository) {
	s.skillRepo = repo
}

// LastValidationWarnings returns the structured ValidationError warnings from the most
// recent ValidateWorkflow call. Warnings do not fail validation but are surfaced here
// for callers that want to display or log them (e.g. UNSUPPORTED_PROVIDER).
// The slice is replaced on each ValidateWorkflow invocation; nil means no warnings.
func (s *WorkflowService) LastValidationWarnings() []workflow.ValidationError {
	return s.lastValidationWarnings
}

func (s *WorkflowService) ListAllWorkflows(ctx context.Context) ([]workflow.WorkflowEntry, error) {
	infos, err := s.repo.ListWithSource(ctx)
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}

	entries := make([]workflow.WorkflowEntry, 0, len(infos))
	for _, info := range infos {
		src := string(info.Source)
		entry := workflow.WorkflowEntry{
			Name:     info.Name,
			Source:   src,
			Scope:    src,
			Workflow: info.Name,
		}
		if wf, loadErr := s.repo.Load(ctx, info.Name); loadErr == nil {
			entry.Version = wf.Version
			entry.Description = wf.Description
		}
		entries = append(entries, entry)
	}

	if s.packDiscoverer != nil {
		packEntries, packErr := s.packDiscoverer.DiscoverWorkflows(ctx)
		if packErr == nil {
			entries = append(entries, packEntries...)
		}
	}

	return entries, nil
}

func (s *WorkflowService) GetWorkflow(ctx context.Context, name string) (*workflow.Workflow, error) {
	if packName, wfName, ok := strings.Cut(name, "/"); ok && s.packDiscoverer != nil {
		wf, err := s.packDiscoverer.LoadWorkflow(ctx, packName, wfName)
		if err != nil {
			return nil, fmt.Errorf("load pack workflow %s: %w", name, err)
		}
		return wf, nil
	}

	wf, err := s.repo.Load(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("load workflow %s: %w", name, err)
	}
	return wf, nil
}

func (s *WorkflowService) ValidateWorkflow(ctx context.Context, name string) error {
	wf, err := s.GetWorkflow(ctx, name)
	if err != nil {
		return err
	}
	return s.ValidateLoadedWorkflow(ctx, wf, name, ports.ValidateOptions{})
}

// ValidateLoadedWorkflow runs the full validation pipeline (domain state/expression
// validation, prompt files, plugin validators, MCP proxy) on an already-loaded workflow.
// It is shared by ValidateWorkflow (load-then-validate) and the WorkflowFacade adapter,
// which loads via the canonical-identifier Resolver and must not re-load by bare name.
// A nil validator skips only the expression-compilation step so a read-only facade without
// an expression validator wired still performs the remaining structural checks.
// opts controls the plugin-validator phase: SkipPlugins suppresses it entirely;
// ValidatorTimeout > 0 bounds it with a context deadline.
func (s *WorkflowService) ValidateLoadedWorkflow(ctx context.Context, wf *workflow.Workflow, name string, opts ports.ValidateOptions) error {
	if s.validator != nil {
		if err := wf.Validate(s.validator.Compile, nil); err != nil {
			var stateRefErr *workflow.StateReferenceError
			if errors.As(err, &stateRefErr) {
				availableAny := make([]any, len(stateRefErr.AvailableStates))
				for i, s := range stateRefErr.AvailableStates {
					availableAny[i] = s
				}
				return domerrors.NewWorkflowError(
					domerrors.ErrorCodeWorkflowValidationMissingState,
					stateRefErr.Error(),
					map[string]any{
						"state":            stateRefErr.ReferencedState,
						"available_states": availableAny,
						"step":             stateRefErr.StepName,
						"field":            stateRefErr.Field,
					},
					err,
				)
			}
			return fmt.Errorf("validate workflow %s: %w", name, err)
		}
	}

	if err := s.validatePromptFiles(wf); err != nil {
		return err
	}

	if err := s.validateSkillRefs(ctx, wf); err != nil {
		return err
	}

	if err := s.validateWithPluginProvider(ctx, wf, opts); err != nil {
		return err
	}

	if err := s.validateMCPProxy(wf); err != nil {
		return err
	}

	return s.validateTemplateReferences(wf)
}

func (s *WorkflowService) validateSkillRefs(ctx context.Context, wf *workflow.Workflow) error {
	if s.skillRepo == nil {
		return nil
	}

	for _, step := range wf.Steps {
		for _, ref := range step.Skills {
			if _, err := resolveSkillRef(ctx, s.skillRepo, ref, wf.SourceDir); err != nil {
				var notFound *workflow.SkillNotFoundError
				if errors.As(err, &notFound) && skillDirExists(notFound.Name, notFound.SearchPaths) {
					return workflow.ValidationError{
						Level:   workflow.ValidationLevelError,
						Code:    workflow.ErrSkillMissingSkillMD,
						Message: fmt.Sprintf("skill %q has no SKILL.md", notFound.Name),
						Path:    fmt.Sprintf("states.%s.skills", step.Name),
					}
				}
				return err
			}
		}
	}
	return nil
}

func skillDirExists(name string, searchPaths []string) bool {
	for _, searchPath := range searchPaths {
		if info, err := os.Stat(filepath.Join(searchPath, name)); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// validateTemplateReferences runs the template-interpolation reference checks
// (forward references, unknown namespaces, {{error.*}} used outside error-hook
// contexts) via the domain TemplateValidator. It is skipped when no TemplateAnalyzer
// is wired, so read-only facades without an analyzer keep performing only the
// structural and expression-compilation checks. Multiple errors are aggregated into a
// single error, mirroring the validate command's historical multi-error formatting.
func (s *WorkflowService) validateTemplateReferences(wf *workflow.Workflow) error {
	if s.templateAnalyzer == nil {
		return nil
	}

	tv := workflow.NewTemplateValidator(wf, s.templateAnalyzer)
	if tv == nil {
		return nil
	}

	result := tv.Validate()
	if result == nil || !result.HasErrors() {
		return nil
	}

	if len(result.Errors) == 1 {
		return result.Errors[0]
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "validation failed with %d errors:", len(result.Errors))
	for i := range result.Errors {
		fmt.Fprintf(&sb, "\n  %s", result.Errors[i].Error())
	}
	return fmt.Errorf("%s", sb.String())
}

// promptFileError constructs an ErrorCodeUserInputMissingFile structured error
// with consistent metadata for prompt-file validation failures.
func promptFileError(msg, resolvedPath, stepName string, cause error) error {
	return domerrors.NewStructuredError(
		domerrors.ErrorCodeUserInputMissingFile,
		msg,
		map[string]any{
			"path": resolvedPath,
			"step": stepName,
		},
		cause,
	)
}

func (s *WorkflowService) validatePromptFiles(wf *workflow.Workflow) error {
	for _, step := range wf.Steps {
		if step.Type != workflow.StepTypeAgent || step.Agent == nil {
			continue
		}

		if step.Agent.PromptFile == "" {
			continue
		}

		// Skip validation for paths with template expressions — resolved at runtime
		if strings.Contains(step.Agent.PromptFile, "{{") {
			continue
		}

		path := step.Agent.PromptFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(wf.SourceDir, path)
		}

		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return promptFileError(
					fmt.Sprintf("prompt_file not found: %s", step.Agent.PromptFile),
					path, step.Name, err,
				)
			}
			return promptFileError(
				fmt.Sprintf("prompt_file cannot be accessed: %s", step.Agent.PromptFile),
				path, step.Name, err,
			)
		}

		if info.IsDir() {
			return promptFileError(
				fmt.Sprintf("prompt_file is a directory, not a file: %s", step.Agent.PromptFile),
				path, step.Name, nil,
			)
		}

		f, err := os.Open(path)
		if err != nil {
			return promptFileError(
				fmt.Sprintf("prompt_file cannot be read: %s", step.Agent.PromptFile),
				path, step.Name, err,
			)
		}
		_ = f.Close()
	}

	return nil
}

func (s *WorkflowService) validateWithPluginProvider(ctx context.Context, wf *workflow.Workflow, opts ports.ValidateOptions) error {
	if opts.SkipPlugins {
		return nil
	}
	if s.validatorProvider == nil {
		return nil
	}

	if opts.ValidatorTimeout > 0 {
		ctx2, cancel := context.WithTimeout(ctx, opts.ValidatorTimeout)
		defer cancel()
		ctx = ctx2
	}

	workflowJSON, err := json.Marshal(wf)
	if err != nil {
		return fmt.Errorf("marshal workflow for plugin validation: %w", err)
	}

	results, err := s.validatorProvider.ValidateWorkflow(ctx, workflowJSON)
	if err != nil {
		return fmt.Errorf("plugin validation error: %w", err)
	}

	for _, result := range results {
		if result.Severity == ports.SeverityError {
			return fmt.Errorf("workflow validation failed: %s", result.Message)
		}
	}

	return nil
}

// validateMCPProxy performs cross-block validation for mcp_proxy configurations.
// It iterates all steps with mcp_proxy enabled and:
//   - Emits a WARN log (non-fatal) when the agent provider is codex or opencode.
//   - Accumulates a structured ValidationError{Level:Warning} for UNSUPPORTED_PROVIDER
//     so callers can surface it via LastValidationWarnings().
//   - Validates plugin_tools[] entries against the injected OperationProvider.
//
// When opProvider is nil, plugin-level checks are skipped silently.
// Structural checks (UNKNOWN_KEY) already ran in the YAML mapper.
// Warnings never fail validation (never added to allErrs).
func (s *WorkflowService) validateMCPProxy(wf *workflow.Workflow) error {
	knownPlugins := s.buildKnownPluginSet()

	// Reset warnings from previous calls.
	s.lastValidationWarnings = nil

	var allErrs []error
	for _, step := range wf.Steps {
		if step.MCPProxy == nil || !step.MCPProxy.Enable {
			continue
		}

		// Accumulate warning (non-fatal) for unsupported providers.
		if warn := s.warnIfUnsupportedProvider(step); warn != nil {
			s.lastValidationWarnings = append(s.lastValidationWarnings, *warn)
		}

		if s.opProvider == nil {
			continue
		}

		allErrs = append(allErrs, s.validateMCPProxyPluginTools(step, knownPlugins)...)
	}

	return errors.Join(allErrs...)
}

// buildKnownPluginSet returns a set of all plugin names registered in the OperationProvider.
// Returns an empty map when opProvider is nil.
func (s *WorkflowService) buildKnownPluginSet() map[string]bool {
	if s.opProvider == nil {
		return nil
	}
	known := make(map[string]bool)
	for _, op := range s.opProvider.ListOperations() {
		if op.PluginName != "" {
			known[op.PluginName] = true
		}
	}
	return known
}

// warnIfUnsupportedProvider emits a WARN log when the step's agent provider operates
// the MCP proxy in coexistence mode (codex, copilot, opencode) and mcp_proxy is enabled.
// This is non-fatal (warning-only). It also returns a structured ValidationError at warning
// level for the accumulator so callers can surface it via structured output.
func (s *WorkflowService) warnIfUnsupportedProvider(step *workflow.Step) *workflow.ValidationError {
	if step.Agent == nil || s.logger == nil {
		return nil
	}
	provider := strings.ToLower(step.Agent.Provider)
	if !slices.Contains(apptools.CoexistenceProviders(), provider) {
		return nil
	}
	s.logger.Warn(
		fmt.Sprintf("mcp_proxy on provider=%s is not supported; proxy will be ignored at runtime", provider),
		"code", string(domerrors.ErrorCodeUserMCPProxyUnsupportedProvider),
		"step", step.Name,
	)
	ve := &workflow.ValidationError{
		Level:   workflow.ValidationLevelWarning,
		Code:    workflow.ValidationCode(domerrors.ErrorCodeUserMCPProxyUnsupportedProvider),
		Message: fmt.Sprintf("mcp_proxy on provider=%s runs in coexistence mode; built-in tools are not blocked", provider),
		Path:    fmt.Sprintf("states.%s.mcp_proxy", step.Name),
	}
	return ve
}

// validateMCPProxyPluginTools validates plugin_tools entries for a single step.
// Collects ALL violations (unknown plugin + unknown operations) and returns them all,
// per project rule: "YAML parsing now reports all errors" (accumulate, never short-circuit).
func (s *WorkflowService) validateMCPProxyPluginTools(step *workflow.Step, knownPlugins map[string]bool) []error {
	var errs []error
	for i, pt := range step.MCPProxy.PluginTools {
		pluginPath := fmt.Sprintf("states.%s.mcp_proxy.plugin_tools[%d].plugin", step.Name, i)

		if !knownPlugins[pt.Plugin] {
			errs = append(errs, domerrors.NewStructuredError(
				domerrors.ErrorCodeUserMCPProxyUnknownPlugin,
				fmt.Sprintf("%s: plugin %q not found in operation registry", string(domerrors.ErrorCodeUserMCPProxyUnknownPlugin), pt.Plugin),
				map[string]any{
					"plugin": pt.Plugin,
					"step":   step.Name,
					"path":   pluginPath,
				},
				nil,
			))
			// Unknown plugin: skip expose validation for this entry to avoid noise.
			continue
		}

		errs = append(errs, s.validateMCPProxyExposedOps(step.Name, i, pt.Plugin, pt.Expose)...)
	}
	return errs
}

// validateMCPProxyExposedOps validates that each operation name in the expose list
// belongs to the specified plugin in the OperationProvider.
// Returns all violations found, never short-circuiting on first error.
func (s *WorkflowService) validateMCPProxyExposedOps(stepName string, toolIdx int, pluginName string, expose []string) []error {
	var errs []error
	for j, opName := range expose {
		opPath := fmt.Sprintf("states.%s.mcp_proxy.plugin_tools[%d].expose[%d]", stepName, toolIdx, j)
		op, found := s.opProvider.GetOperation(opName)
		if !found || op.PluginName != pluginName {
			errs = append(errs, domerrors.NewStructuredError(
				domerrors.ErrorCodeUserMCPProxyUnknownOperation,
				fmt.Sprintf("%s: operation %q not found in plugin %q", string(domerrors.ErrorCodeUserMCPProxyUnknownOperation), opName, pluginName),
				map[string]any{
					"operation": opName,
					"plugin":    pluginName,
					"step":      stepName,
					"path":      opPath,
				},
				nil,
			))
		}
	}
	return errs
}
