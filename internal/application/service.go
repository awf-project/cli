package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	apptools "github.com/awf-project/cli/internal/application/tools"
	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/operation"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
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
	stepTypeProvider       ports.StepTypeProvider
	packDiscoverer         ports.PackDiscoverer
	opProvider             ports.OperationProvider
	templateAnalyzer       workflow.TemplateAnalyzer
	agentRegistry          ports.AgentRegistry
	userInputReaderMu      sync.RWMutex
	userInputReader        ports.UserInputReader
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

// SetStepTypeProvider supplies plugin registration checks after discovery.
func (s *WorkflowService) SetStepTypeProvider(p ports.StepTypeProvider) { s.stepTypeProvider = p }

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

func (s *WorkflowService) SetAgentRegistry(registry ports.AgentRegistry) {
	s.agentRegistry = registry
}

func (s *WorkflowService) SetUserInputReader(reader ports.UserInputReader) {
	s.userInputReaderMu.Lock()
	defer s.userInputReaderMu.Unlock()
	s.userInputReader = reader
}

func (s *WorkflowService) getUserInputReader() ports.UserInputReader {
	s.userInputReaderMu.RLock()
	defer s.userInputReaderMu.RUnlock()
	return s.userInputReader
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
			entries = slices.Concat(entries, packEntries)
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

func (s *WorkflowService) ValidateLoadedWorkflowWithInputs(ctx context.Context, wf *workflow.Workflow, name string, opts ports.ValidateOptions, inputs map[string]any) error {
	return s.validateLoadedWorkflow(ctx, wf, name, opts, inputs, true)
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
	return s.validateLoadedWorkflow(ctx, wf, name, opts, nil, false)
}

func (s *WorkflowService) validateLoadedWorkflow(ctx context.Context, wf *workflow.Workflow, name string, opts ports.ValidateOptions, supplied map[string]any, requireInputs bool) error {
	wf = cloneWorkflowForValidation(wf)

	if s.validator != nil {
		var checker workflow.StepTypeChecker
		if s.stepTypeProvider != nil {
			checker = s.stepTypeProvider.HasStepType
		}
		if err := wf.Validate(s.validator.Compile, checker); err != nil {
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

	return errors.Join(
		s.validateTemplateReferences(wf),
		s.validateAgentInputAvailability(wf, supplied, requireInputs),
		s.validateOperationInputs(wf, supplied, requireInputs),
		s.validateAgentProviders(ctx, wf, supplied, requireInputs),
	)
}

func (s *WorkflowService) validateAgentProviders(ctx context.Context, wf *workflow.Workflow, supplied map[string]any, requireInputs bool) error {
	if s.agentRegistry == nil || wf == nil {
		return nil
	}

	declared, available := validationInputs(wf, supplied)
	stepNames := make([]string, 0, len(wf.Steps))
	for stepName := range wf.Steps {
		stepNames = append(stepNames, stepName)
	}
	sort.Strings(stepNames)

	var failures []error
	for _, stepName := range stepNames {
		step := wf.Steps[stepName]
		if step == nil || step.Agent == nil {
			continue
		}
		failures = slices.Concat(failures, s.validateAgentProviderStep(
			ctx, stepName, step.Agent, declared, available, requireInputs,
		))
	}

	return joinErrors(failures)
}

func (s *WorkflowService) validateAgentProviderStep(ctx context.Context, stepName string, agent *workflow.AgentConfig,
	declared map[string]bool, available map[string]any, requireInputs bool,
) []error {
	userInputReader := s.getUserInputReader()
	var inputErr error
	if agent.IsConversationMode() {
		if userInputReader == nil {
			inputErr = errors.New("conversation input reader is unavailable")
		} else if checker, ok := userInputReader.(ports.UserInputPreflighter); ok {
			inputErr = checker.PreflightInput(ctx)
		}
	}
	providerName, deferred, err := resolveStaticString(
		agent.Provider, declared, available, requireInputs, s.templateAnalyzer,
	)
	if err != nil {
		return []error{err}
	}
	if deferred {
		if inputErr != nil {
			return []error{providerValidationError(agent.Provider, stepName, "interactive_input_required", inputErr)}
		}
		return nil
	}

	provider, err := s.agentRegistry.Get(providerName)
	if err != nil || provider == nil {
		return []error{providerValidationError(providerName, stepName, "unavailable", err)}
	}
	if err := provider.Validate(); err != nil {
		return []error{providerValidationError(providerName, stepName, "unavailable", err)}
	}
	preflighter, ok := provider.(ports.AgentPreflighter)
	if !ok {
		return []error{providerValidationError(providerName, stepName, "preflight_unsupported", nil)}
	}
	resolvedOptions, deferred, missing, optionErrors := resolveStaticValue(
		agent.Options, declared, available, requireInputs, s.templateAnalyzer,
	)
	if deferred || missing || len(optionErrors) > 0 {
		if len(optionErrors) == 0 && inputErr != nil {
			return []error{providerValidationError(providerName, stepName, "interactive_input_required", inputErr)}
		}
		return optionErrors
	}
	options, ok := resolvedOptions.(map[string]any)
	if !ok && resolvedOptions != nil {
		return nil
	}
	if err := preflighter.Preflight(ctx, options); err != nil {
		return []error{providerValidationError(providerName, stepName, "not_ready", err)}
	}
	if inputErr != nil {
		return []error{providerValidationError(providerName, stepName, "interactive_input_required", inputErr)}
	}
	return nil
}

func providerValidationError(provider, step, reason string, cause error) error {
	return domerrors.NewStructuredError(
		domerrors.ErrorCodeUserInputValidationFailed,
		fmt.Sprintf("provider %q failed validation for step %q: %s", provider, step, reason),
		map[string]any{"provider": provider, "step": step, "reason": reason},
		cause,
	)
}

func cloneWorkflowForValidation(wf *workflow.Workflow) *workflow.Workflow {
	if wf == nil {
		return nil
	}
	cloned := *wf
	cloned.Steps = make(map[string]*workflow.Step, len(wf.Steps))
	for name, step := range wf.Steps {
		if step == nil {
			cloned.Steps[name] = nil
			continue
		}
		clonedStep := *step
		if step.Agent != nil {
			clonedAgent := *step.Agent
			clonedAgent.Options = cloneStringMap(step.Agent.Options)
			clonedStep.Agent = &clonedAgent
		}
		clonedStep.OperationInputs = cloneStringMap(step.OperationInputs)
		cloned.Steps[name] = &clonedStep
	}
	return &cloned
}

func cloneStringMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = cloneValidationValue(value)
	}
	return cloned
}

func (s *WorkflowService) validateAgentInputAvailability(wf *workflow.Workflow, supplied map[string]any, requireInputs bool) error {
	if wf == nil || !requireInputs {
		return nil
	}
	declared, available := validationInputs(wf, supplied)
	stepNames := make([]string, 0, len(wf.Steps))
	for stepName := range wf.Steps {
		stepNames = append(stepNames, stepName)
	}
	sort.Strings(stepNames)

	var failures []error
	for _, stepName := range stepNames {
		step := wf.Steps[stepName]
		if step == nil || step.Agent == nil {
			continue
		}
		failures = slices.Concat(failures, missingInputErrors(step.Agent.Prompt,
			operationFieldPath(stepName, "agent.prompt"), declared, available, s.templateAnalyzer))
		failures = slices.Concat(failures, missingInputErrors(step.Agent.Provider,
			operationFieldPath(stepName, "agent.provider"), declared, available, s.templateAnalyzer))
		failures = slices.Concat(failures, missingInputValueErrors(step.Agent.Options,
			operationFieldPath(stepName, "agent.options"), declared, available, s.templateAnalyzer))
	}
	return joinErrors(failures)
}

// walkValidationStrings visits executable strings in diagnostic order without
// copying containers. Static resolution separately owns value reconstruction.
func walkValidationStrings(value any, path string, visit func(string, string) []error) []error {
	switch value := value.(type) {
	case string:
		return visit(value, path)
	case map[string]any:
		var failures []error
		for _, key := range sortedValidationKeys(value) {
			failures = slices.Concat(failures, walkValidationStrings(
				value[key], workflow.AppendTemplateFieldPath(path, key), visit,
			))
		}
		return failures
	case []any:
		var failures []error
		for i := range value {
			failures = slices.Concat(failures, walkValidationStrings(value[i], fmt.Sprintf("%s[%d]", path, i), visit))
		}
		return failures
	default:
		return nil
	}
}

func sortedValidationKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func missingInputValueErrors(value any, path string, declared map[string]bool, available map[string]any,
	analyzer workflow.TemplateAnalyzer,
) []error {
	return walkValidationStrings(value, path, func(value, path string) []error {
		return missingInputErrors(value, path, declared, available, analyzer)
	})
}

func undefinedInputValueErrors(value any, path string, declared map[string]bool,
	analyzer workflow.TemplateAnalyzer,
) []error {
	return walkValidationStrings(value, path, func(value, path string) []error {
		references, err := workflow.AnalyzeInputTemplateReferences(analyzer, value)
		if err != nil {
			return []error{fmt.Errorf("analyze template references at %s: %w", path, err)}
		}
		var failures []error
		for _, ref := range references {
			if declared[ref.Name] {
				continue
			}
			failures = append(failures, workflow.ValidationError{
				Level: workflow.ValidationLevelError, Code: workflow.ErrUndefinedInput,
				Path: path, Message: fmt.Sprintf("undefined input %q", ref.Name),
			})
		}
		return failures
	})
}

func missingInputErrors(value, path string, declared map[string]bool, available map[string]any,
	analyzer workflow.TemplateAnalyzer,
) []error {
	var failures []error
	references, err := workflow.AnalyzeInputTemplateReferences(analyzer, value)
	if err != nil {
		return []error{fmt.Errorf("analyze template references at %s: %w", path, err)}
	}
	for _, ref := range references {
		if !declared[ref.Name] {
			continue
		}
		if _, ok := available[ref.Name]; !ok {
			failures = append(failures, workflow.ValidationError{
				Level: workflow.ValidationLevelError, Code: workflow.ErrUndefinedInput,
				Path: path, Message: fmt.Sprintf("referenced input %q is missing", ref.Name),
			})
		}
	}
	return failures
}

func validationInputs(wf *workflow.Workflow, supplied map[string]any) (
	declared map[string]bool,
	available map[string]any,
) {
	available = make(map[string]any, len(wf.Inputs)+len(supplied))
	declared = make(map[string]bool, len(wf.Inputs))
	for _, input := range wf.Inputs {
		declared[input.Name] = true
		if input.Default != nil {
			available[input.Name] = cloneValidationValue(input.Default)
		}
	}
	for key, value := range supplied {
		available[key] = cloneValidationValue(value)
	}
	return declared, available
}

func joinErrors(errs []error) error {
	var joined error
	for _, err := range errs {
		joined = errors.Join(joined, err)
	}
	return joined
}

func (s *WorkflowService) validateOperationInputs(wf *workflow.Workflow, supplied map[string]any, requireInputs bool) error {
	if s.opProvider == nil || wf == nil {
		return nil
	}

	declared, available := validationInputs(wf, supplied)

	stepNames := make([]string, 0, len(wf.Steps))
	for stepName := range wf.Steps {
		stepNames = append(stepNames, stepName)
	}
	sort.Strings(stepNames)

	var failures []error
	for _, stepName := range stepNames {
		step := wf.Steps[stepName]
		if step == nil || step.Type != workflow.StepTypeOperation || step.Operation == "" {
			continue
		}

		failures = slices.Concat(failures, s.validateOperationStep(stepName, step, declared, available, requireInputs))
	}
	return joinErrors(failures)
}

func (s *WorkflowService) validateOperationStep(stepName string, step *workflow.Step,
	declared map[string]bool, available map[string]any, requireInputs bool,
) []error {
	var failures []error
	if requireInputs {
		missingNameInputs := missingInputErrors(step.Operation,
			operationFieldPath(stepName, "operation"), declared, available, s.templateAnalyzer)
		if len(missingNameInputs) > 0 {
			failures = slices.Concat(failures, missingNameInputs)
			failures = slices.Concat(failures, undefinedInputValueErrors(
				step.OperationInputs, operationFieldPath(stepName, "inputs"), declared, s.templateAnalyzer,
			))
			failures = slices.Concat(failures, missingInputValueErrors(
				step.OperationInputs, operationFieldPath(stepName, "inputs"),
				declared, available, s.templateAnalyzer,
			))
			return failures
		}
	}

	operationName, deferred, refErr := resolveStaticString(
		step.Operation, declared, available, requireInputs, s.templateAnalyzer,
	)
	if refErr != nil {
		failures = append(failures, refErr)
		return failures
	}
	if deferred {
		return failures
	}

	schema, ok := s.opProvider.GetOperation(operationName)
	if !ok {
		cause := fmt.Errorf("%w: %s", operation.ErrOperationNotFound, operationName)
		failures = append(failures, operationValidationError(operationName, stepName,
			[]string{operationFieldPath(stepName, "operation")}, nil, cause))
		return failures
	}
	if schema == nil {
		cause := fmt.Errorf("%w: operation %s returned a nil schema", operation.ErrInvalidInputs, operationName)
		failures = append(failures, operationValidationError(operationName, stepName,
			[]string{operationFieldPath(stepName, "inputs")}, nil, cause))
		return failures
	}

	resolved, deferredFields, missingFields, referenceErrors := resolveStaticOperationInputs(
		step.OperationInputs, declared, available, requireInputs, s.templateAnalyzer,
	)
	failures = slices.Concat(failures, referenceErrors)
	validationSchema := staticOperationSchema(schema, deferredFields, missingFields)

	findings, validationErr := operation.ValidateInputsDetailed(validationSchema, resolved)
	fieldPaths := operationFindingPaths(stepName, findings, missingFields)
	if len(fieldPaths) > 0 {
		if validationErr == nil {
			validationErr = fmt.Errorf("%w: referenced workflow input is missing", operation.ErrInvalidInputs)
		}
		requiredFields := missingRequiredOperationFields(validationSchema, resolved)
		failures = append(failures, operationValidationError(
			operationName, stepName, fieldPaths, requiredFields, validationErr,
		))
	}
	return failures
}

func staticOperationSchema(schema *pluginmodel.OperationSchema, deferred, missing map[string]bool) *pluginmodel.OperationSchema {
	cloned := cloneOperationSchema(schema)
	for field := range deferred {
		delete(cloned.Inputs, field)
	}
	for field := range missing {
		delete(cloned.Inputs, field)
	}
	return cloned
}

func operationFindingPaths(stepName string, findings []operation.InputValidationFinding, missing map[string]bool) []string {
	fieldPaths := make([]string, 0, len(findings)+len(missing))
	inputsPath := operationFieldPath(stepName, "inputs")
	for field := range missing {
		fieldPaths = append(fieldPaths, workflow.AppendTemplateFieldPath(inputsPath, field))
	}
	for _, finding := range findings {
		if finding.Field == "" {
			fieldPaths = append(fieldPaths, inputsPath)
		} else {
			fieldPaths = append(fieldPaths, workflow.AppendTemplateFieldPath(inputsPath, finding.Field))
		}
	}
	sort.Strings(fieldPaths)
	return slices.Compact(fieldPaths)
}

func resolveStaticOperationInputs(inputs map[string]any, declared map[string]bool, available map[string]any,
	requireInputs bool, analyzer workflow.TemplateAnalyzer,
) (
	resolved map[string]any,
	deferred map[string]bool,
	missing map[string]bool,
	referenceErrors []error,
) {
	resolved = make(map[string]any, len(inputs))
	deferred = make(map[string]bool)
	missing = make(map[string]bool)
	for _, key := range sortedValidationKeys(inputs) {
		value := inputs[key]
		resolvedValue, valueDeferred, valueMissing, errs := resolveStaticValue(
			value, declared, available, requireInputs, analyzer,
		)
		referenceErrors = slices.Concat(referenceErrors, errs)
		if valueMissing {
			missing[key] = true
			continue
		}
		if valueDeferred {
			deferred[key] = true
			resolved[key] = cloneValidationValue(value)
			continue
		}
		resolved[key] = resolvedValue
	}
	return resolved, deferred, missing, referenceErrors
}

func resolveStaticValue(value any, declared map[string]bool, available map[string]any, requireInputs bool,
	analyzer workflow.TemplateAnalyzer,
) (
	resolved any,
	deferred bool,
	missing bool,
	referenceErrors []error,
) {
	switch value := value.(type) {
	case string:
		resolved, deferred, err := resolveStaticString(value, declared, available, requireInputs, analyzer)
		if err != nil {
			return nil, false, false, []error{err}
		}
		missing := requireInputs && hasUnavailableInput(value, declared, available, analyzer)
		return resolved, deferred, missing, nil
	case map[string]any:
		result := make(map[string]any, len(value))
		var deferred, missing bool
		var errs []error
		for _, key := range sortedValidationKeys(value) {
			resolved, nestedDeferred, nestedMissing, nestedErrs := resolveStaticValue(
				value[key], declared, available, requireInputs, analyzer,
			)
			result[key] = resolved
			deferred = deferred || nestedDeferred
			missing = missing || nestedMissing
			errs = slices.Concat(errs, nestedErrs)
		}
		return result, deferred, missing, errs
	case []any:
		result := make([]any, len(value))
		var deferred, missing bool
		var errs []error
		for i := range value {
			resolved, nestedDeferred, nestedMissing, nestedErrs := resolveStaticValue(
				value[i], declared, available, requireInputs, analyzer,
			)
			result[i] = resolved
			deferred = deferred || nestedDeferred
			missing = missing || nestedMissing
			errs = slices.Concat(errs, nestedErrs)
		}
		return result, deferred, missing, errs
	default:
		return value, false, false, nil
	}
}

func resolveStaticString(value string, declared map[string]bool, available map[string]any, requireInputs bool,
	analyzer workflow.TemplateAnalyzer,
) (
	resolved string,
	deferred bool,
	err error,
) {
	resolved = value
	references, analyzeErr := workflow.AnalyzeInputTemplateReferences(analyzer, value)
	if analyzeErr != nil {
		return value[:0], false, fmt.Errorf("analyze template references: %w", analyzeErr)
	}
	for _, ref := range references {
		name := ref.Name
		if !declared[name] {
			return value[:0], false, workflow.ValidationError{
				Level:   workflow.ValidationLevelError,
				Code:    workflow.ErrUndefinedInput,
				Message: fmt.Sprintf("undefined input %q", name),
			}
		}
		inputValue, ok := available[name]
		if !ok {
			if requireInputs {
				continue
			}
			return value, true, nil
		}
		if ref.Direct {
			resolved = strings.ReplaceAll(resolved, ref.Raw, fmt.Sprint(inputValue))
		}
	}
	if strings.Contains(resolved, "{{") {
		return resolved, true, nil
	}
	return resolved, false, nil
}

func hasUnavailableInput(value string, declared map[string]bool, available map[string]any,
	analyzer workflow.TemplateAnalyzer,
) bool {
	references, err := workflow.AnalyzeInputTemplateReferences(analyzer, value)
	if err != nil {
		return false
	}
	for _, ref := range references {
		if declared[ref.Name] {
			if _, ok := available[ref.Name]; !ok {
				return true
			}
		}
	}
	return false
}

func cloneValidationValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(value))
		for key, nested := range value {
			cloned[key] = cloneValidationValue(nested)
		}
		return cloned
	case []any:
		cloned := make([]any, len(value))
		for i := range value {
			cloned[i] = cloneValidationValue(value[i])
		}
		return cloned
	default:
		return value
	}
}

func cloneOperationSchema(schema *pluginmodel.OperationSchema) *pluginmodel.OperationSchema {
	cloned := *schema
	cloned.Inputs = make(map[string]pluginmodel.InputSchema, len(schema.Inputs))
	for key, input := range schema.Inputs {
		cloned.Inputs[key] = input
	}
	return &cloned
}

func missingRequiredOperationFields(schema *pluginmodel.OperationSchema, inputs map[string]any) []string {
	var fields []string
	for name, input := range schema.Inputs {
		value, exists := inputs[name]
		if input.Required && (!exists || value == nil || (input.Type == pluginmodel.InputTypeString && value == "")) {
			fields = append(fields, name)
		}
	}
	sort.Strings(fields)
	return fields
}

func operationValidationError(
	operationName, stepName string,
	fieldPaths, requiredFields []string,
	cause error,
) error {
	message := fmt.Sprintf("invalid inputs for operation %q in step %q", operationName, stepName)
	if errors.Is(cause, operation.ErrOperationNotFound) {
		message = fmt.Sprintf("operation %q not found in step %q", operationName, stepName)
	} else if len(requiredFields) > 0 {
		reasons := make([]string, len(requiredFields))
		for i, field := range requiredFields {
			reasons[i] = fmt.Sprintf("required field %q is missing or empty", field)
			// Backend selection may be supplied by provider configuration at execution time.
			if field != "backend" {
				reasons[i] += fmt.Sprintf(" (%s is required)", field)
			}
		}
		message += ": " + strings.Join(reasons, "; ")
	}
	return domerrors.NewStructuredError(domerrors.ErrorCodeUserInputValidationFailed,
		message,
		map[string]any{"operation": operationName, "step": stepName, "field_paths": fieldPaths}, cause)
}

func operationFieldPath(stepName, suffix string) string {
	if workflowPathIdentifier(stepName) {
		return "steps." + stepName + "." + suffix
	}
	encoded, _ := json.Marshal(stepName)
	return "steps[" + string(encoded) + "]." + suffix
}

func workflowPathIdentifier(value string) bool {
	if value == "" || !workflowPathIdentifierStart(value[0]) {
		return false
	}
	for i := 1; i < len(value); i++ {
		if char := value[i]; !workflowPathIdentifierStart(char) && (char < '0' || char > '9') {
			return false
		}
	}
	return true
}

func workflowPathIdentifierStart(char byte) bool {
	return (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || char == '_'
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

		allErrs = slices.Concat(allErrs, s.validateMCPProxyPluginTools(step, knownPlugins))
	}

	return joinErrors(allErrs)
}

// buildKnownPluginSet returns a set of all plugin names registered in the OperationProvider.
// Returns an empty map when opProvider is nil.
func (s *WorkflowService) buildKnownPluginSet() map[string]bool {
	if s.opProvider == nil {
		return nil
	}
	known := make(map[string]bool)
	for _, op := range s.opProvider.ListOperations() {
		if op == nil {
			continue
		}
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

		errs = slices.Concat(errs, s.validateMCPProxyExposedOps(step.Name, i, pt.Plugin, pt.Expose))
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
