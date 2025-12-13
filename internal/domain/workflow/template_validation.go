package workflow

import "fmt"

// TemplateValidator validates template interpolation references in workflows.
type TemplateValidator struct {
	workflow         *Workflow
	analyzer         TemplateAnalyzer
	executionOrder   []string
	stepIndex        map[string]int
	inputNames       map[string]bool
	stepNames        map[string]bool
	parallelBranches map[string]string // maps branch name -> parent parallel step name
	result           *ValidationResult
}

// NewTemplateValidator creates a validator for the given workflow.
// The analyzer parameter is used to extract template references.
func NewTemplateValidator(w *Workflow, analyzer TemplateAnalyzer) *TemplateValidator {
	if w == nil || analyzer == nil {
		return nil
	}

	// Build execution order (errors are handled by main Validate())
	order, err := ComputeExecutionOrder(w.Steps, w.Initial)
	if err != nil {
		return nil // Let main validation catch this
	}

	// Build step index for forward reference detection
	stepIndex := make(map[string]int, len(order))
	for i, name := range order {
		stepIndex[name] = i
	}

	// Build parallel branches map
	parallelBranches := buildParallelBranches(w.Steps)

	return &TemplateValidator{
		workflow:         w,
		analyzer:         analyzer,
		executionOrder:   order,
		stepIndex:        stepIndex,
		inputNames:       buildInputNames(w.Inputs),
		stepNames:        buildStepNames(w.Steps),
		parallelBranches: parallelBranches,
		result:           &ValidationResult{},
	}
}

// buildParallelBranches maps each branch step to its parent parallel step.
func buildParallelBranches(steps map[string]*Step) map[string]string {
	branches := make(map[string]string)
	for name, step := range steps {
		if step.Type == StepTypeParallel {
			for _, branch := range step.Branches {
				branches[branch] = name
			}
		}
	}
	return branches
}

// Validate performs template reference validation on the entire workflow.
// Returns a ValidationResult containing all errors and warnings found.
// Does not fail-fast; collects all issues in a single pass.
func (v *TemplateValidator) Validate() *ValidationResult {
	if v == nil || v.workflow == nil {
		return &ValidationResult{}
	}

	// Validate all steps
	for _, stepName := range v.executionOrder {
		step, ok := v.workflow.Steps[stepName]
		if !ok {
			continue
		}
		v.ValidateStep(step, false)
	}

	// Also validate steps not in execution order (might be unreachable but still validate)
	for stepName, step := range v.workflow.Steps {
		if _, inOrder := v.stepIndex[stepName]; !inOrder {
			v.ValidateStep(step, false)
		}
	}

	// Validate workflow-level hooks
	v.validateWorkflowHooks()

	return v.result
}

// validateWorkflowHooks validates template references in workflow-level hooks.
func (v *TemplateValidator) validateWorkflowHooks() {
	hooks := v.workflow.Hooks

	// WorkflowStart - not error hook
	for _, action := range hooks.WorkflowStart {
		v.validateHookAction(action, "hooks.workflow_start", false)
	}

	// WorkflowEnd - not error hook
	for _, action := range hooks.WorkflowEnd {
		v.validateHookAction(action, "hooks.workflow_end", false)
	}

	// WorkflowError - IS error hook ({{error.*}} allowed)
	for _, action := range hooks.WorkflowError {
		v.validateHookAction(action, "hooks.workflow_error", true)
	}

	// WorkflowCancel - not error hook
	for _, action := range hooks.WorkflowCancel {
		v.validateHookAction(action, "hooks.workflow_cancel", false)
	}
}

// validateHookAction validates a single hook action.
func (v *TemplateValidator) validateHookAction(action HookAction, path string, isErrorHook bool) {
	// -1 means hooks are validated without step index context (inputs/workflow refs are always valid)
	if action.Command != "" {
		v.ValidateTemplate(action.Command, path, "command", -1, isErrorHook)
	}
	if action.Log != "" {
		v.ValidateTemplate(action.Log, path, "log", -1, isErrorHook)
	}
}

// ValidateStep validates template references within a single step.
// The isErrorHook parameter indicates if this is an error hook context
// (where {{error.*}} references are allowed).
func (v *TemplateValidator) ValidateStep(step *Step, isErrorHook bool) {
	if step == nil {
		return
	}

	stepPath := fmt.Sprintf("steps.%s", step.Name)
	currentIdx := v.stepIndex[step.Name]

	// Validate Command field
	if step.Command != "" {
		v.ValidateTemplate(step.Command, stepPath, "command", currentIdx, isErrorHook)
	}

	// Validate Dir field
	if step.Dir != "" {
		v.ValidateTemplate(step.Dir, stepPath, "dir", currentIdx, isErrorHook)
	}

	// Validate step hooks
	for _, action := range step.Hooks.Pre {
		hookPath := fmt.Sprintf("%s.hooks.pre", stepPath)
		v.validateHookAction(action, hookPath, false)
	}
	for _, action := range step.Hooks.Post {
		hookPath := fmt.Sprintf("%s.hooks.post", stepPath)
		v.validateHookAction(action, hookPath, false)
	}
}

// ValidateTemplate extracts and validates all references in a template string.
// stepName and fieldName provide context for error messages.
// currentStepIndex is used to detect forward references.
func (v *TemplateValidator) ValidateTemplate(template, stepName, fieldName string, currentStepIndex int, isErrorHook bool) {
	refs, err := v.analyzer.ExtractReferences(template)
	if err != nil {
		v.result.AddError(ErrUnknownReferenceType, stepName, fmt.Sprintf("failed to parse template: %v", err))
		return
	}

	for _, ref := range refs {
		v.ValidateReference(ref, stepName, fieldName, currentStepIndex, isErrorHook)
	}
}

// ValidateReference validates a single reference against the workflow context.
func (v *TemplateValidator) ValidateReference(ref TemplateReference, stepName, fieldName string, currentStepIndex int, isErrorHook bool) {
	switch ref.Type {
	case TypeInputs:
		v.validateInputRef(ref, stepName, fieldName)
	case TypeStates:
		v.validateStateRef(ref, stepName, fieldName, currentStepIndex)
	case TypeWorkflow:
		v.validateWorkflowRef(ref, stepName, fieldName)
	case TypeEnv:
		// Environment variables are validated at runtime, not statically
		// No validation needed
	case TypeError:
		v.validateErrorRef(ref, stepName, fieldName, isErrorHook)
	case TypeContext:
		v.validateContextRef(ref, stepName, fieldName)
	case TypeUnknown:
		v.result.AddError(ErrUnknownReferenceType, stepName,
			fmt.Sprintf("unknown reference type %q in %s", ref.Raw, fieldName))
	}
}

func (v *TemplateValidator) validateInputRef(ref TemplateReference, stepName, fieldName string) {
	if !v.inputNames[ref.Path] {
		v.result.AddError(ErrUndefinedInput, stepName,
			fmt.Sprintf("undefined input %q referenced in %s", ref.Path, fieldName))
	}
}

func (v *TemplateValidator) validateStateRef(ref TemplateReference, stepName, fieldName string, currentStepIndex int) {
	referencedStep := ref.Path

	// Check if the step exists
	if !v.stepNames[referencedStep] {
		v.result.AddError(ErrUndefinedStep, stepName,
			fmt.Sprintf("undefined step %q referenced in %s", referencedStep, fieldName))
		return
	}

	// Check for forward references (step A references step B's output, but B runs after A)
	// For hooks (currentStepIndex == -1), we validate state refs based on available context
	if currentStepIndex >= 0 {
		// Extract the current step name from the path (e.g., "steps.start" -> "start")
		currentStepName := v.getStepNameFromPath(stepName)

		// Check for cross-referencing parallel branches
		// If current step and referenced step are both branches of the same parallel step,
		// flag it as a potential issue (they run concurrently)
		currentParent, currentIsBranch := v.parallelBranches[currentStepName]
		refParent, refIsBranch := v.parallelBranches[referencedStep]

		if currentIsBranch && refIsBranch && currentParent == refParent {
			// Both steps are branches of the same parallel step - cannot reference each other
			v.result.AddWarning(ErrForwardReference, stepName,
				fmt.Sprintf("parallel branch %q references sibling branch %q in %s (branches run concurrently)", currentStepName, referencedStep, fieldName))
			return
		}

		refIdx, exists := v.stepIndex[referencedStep]
		if exists && refIdx >= currentStepIndex {
			v.result.AddError(ErrForwardReference, stepName,
				fmt.Sprintf("forward reference to step %q in %s (referenced step has not executed yet)", referencedStep, fieldName))
			return
		}
	}

	// Check for valid property (if provided)
	if ref.Property == "" {
		// Missing property - state refs require a property like .output, .stderr, etc.
		v.result.AddError(ErrInvalidStateProperty, stepName,
			fmt.Sprintf("missing property for state reference %q in %s (expected .output, .stderr, .exit_code, or .status)", referencedStep, fieldName))
		return
	}

	if !ValidStateProperties[ref.Property] {
		v.result.AddError(ErrInvalidStateProperty, stepName,
			fmt.Sprintf("invalid state property %q for step %q in %s", ref.Property, referencedStep, fieldName))
	}
}

// getStepNameFromPath extracts the step name from a path like "steps.start" -> "start"
func (v *TemplateValidator) getStepNameFromPath(path string) string {
	const prefix = "steps."
	if len(path) > len(prefix) && path[:len(prefix)] == prefix {
		// Extract just the step name, not any additional path components
		remaining := path[len(prefix):]
		for i, c := range remaining {
			if c == '.' {
				return remaining[:i]
			}
		}
		return remaining
	}
	return path
}

func (v *TemplateValidator) validateWorkflowRef(ref TemplateReference, stepName, fieldName string) {
	if !ValidWorkflowProperties[ref.Path] {
		v.result.AddError(ErrInvalidWorkflowProperty, stepName,
			fmt.Sprintf("invalid workflow property %q in %s", ref.Path, fieldName))
	}
}

func (v *TemplateValidator) validateErrorRef(ref TemplateReference, stepName, fieldName string, isErrorHook bool) {
	// Error references are only valid in error hook contexts
	if !isErrorHook {
		v.result.AddError(ErrErrorRefOutsideErrorHook, stepName,
			fmt.Sprintf("error reference %q used outside of error hook context in %s", ref.Raw, fieldName))
		return
	}

	// Validate the property
	if !ValidErrorProperties[ref.Path] {
		v.result.AddError(ErrInvalidErrorProperty, stepName,
			fmt.Sprintf("invalid error property %q in %s", ref.Path, fieldName))
	}
}

func (v *TemplateValidator) validateContextRef(ref TemplateReference, stepName, fieldName string) {
	if !ValidContextProperties[ref.Path] {
		v.result.AddError(ErrInvalidContextProperty, stepName,
			fmt.Sprintf("invalid context property %q in %s", ref.Path, fieldName))
	}
}

// ComputeExecutionOrder determines the topological order of step execution.
// This is needed to detect forward references (step A references step B's output,
// but B runs after A in the execution order).
// Returns the ordered list of step names, or an error if a valid order cannot be computed.
func ComputeExecutionOrder(steps map[string]*Step, initial string) ([]string, error) {
	if len(steps) == 0 || initial == "" {
		return nil, nil
	}

	if _, ok := steps[initial]; !ok {
		return nil, nil
	}

	// BFS traversal from initial state to build execution order
	var order []string
	visited := make(map[string]bool)
	queue := []string{initial}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		step, ok := steps[current]
		if !ok {
			continue
		}

		order = append(order, current)

		// For parallel steps, add branches first (they run as part of the parallel step)
		// then add OnSuccess which happens after all branches complete
		if step.Type == StepTypeParallel {
			for _, branch := range step.Branches {
				if !visited[branch] {
					queue = append(queue, branch)
				}
			}
		}

		// Add successors to queue (handle cycles gracefully)
		if step.OnSuccess != "" && !visited[step.OnSuccess] {
			queue = append(queue, step.OnSuccess)
		}
		if step.OnFailure != "" && !visited[step.OnFailure] {
			queue = append(queue, step.OnFailure)
		}
	}

	return order, nil
}

func buildInputNames(inputs []Input) map[string]bool {
	names := make(map[string]bool, len(inputs))
	for _, input := range inputs {
		names[input.Name] = true
	}
	return names
}

func buildStepNames(steps map[string]*Step) map[string]bool {
	names := make(map[string]bool, len(steps))
	for name := range steps {
		names[name] = true
	}
	return names
}
