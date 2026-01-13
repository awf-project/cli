package workflow

import (
	"fmt"
	"strings"
)

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

	// Validate loop expressions
	if step.Loop != nil {
		v.validateLoopExpressions(step, stepPath, currentIdx)
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

// validateLoopExpressions validates template references in loop configuration fields.
// Uses the existing ValidateTemplate method to check all loop expression fields:
// - MaxIterationsExpr: dynamic max iterations expression
// - Condition: while loop condition
// - Items: for_each loop items expression
// - BreakCondition: early exit condition
func (v *TemplateValidator) validateLoopExpressions(step *Step, stepPath string, currentIdx int) {
	loop := step.Loop
	loopPath := fmt.Sprintf("%s.loop", stepPath)

	// Validate MaxIterationsExpr if set
	if loop.MaxIterationsExpr != "" {
		v.ValidateTemplate(loop.MaxIterationsExpr, loopPath, "max_iterations", currentIdx, false)
	}

	// Validate Condition (for while loops)
	if loop.Condition != "" {
		v.ValidateTemplate(loop.Condition, loopPath, "condition", currentIdx, false)
	}

	// Validate Items (for for_each loops)
	if loop.Items != "" {
		v.ValidateTemplate(loop.Items, loopPath, "items", currentIdx, false)
	}

	// Validate BreakCondition if set
	if loop.BreakCondition != "" {
		v.ValidateTemplate(loop.BreakCondition, loopPath, "break_condition", currentIdx, false)
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

	for i := range refs {
		v.ValidateReference(&refs[i], stepName, fieldName, currentStepIndex, isErrorHook)
	}
}

// ValidateReference validates a single reference against the workflow context.
func (v *TemplateValidator) ValidateReference(ref *TemplateReference, stepName, fieldName string, currentStepIndex int, isErrorHook bool) {
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
	case TypeLoop:
		v.validateLoopRef(ref, stepName, fieldName)
	case TypeUnknown:
		v.result.AddError(ErrUnknownReferenceType, stepName,
			fmt.Sprintf("unknown reference type %q in %s", ref.Raw, fieldName))
	}
}

func (v *TemplateValidator) validateInputRef(ref *TemplateReference, stepName, fieldName string) {
	// Check for arithmetic expressions (e.g., "limit * inputs.threshold")
	// These need special handling to extract individual input references
	if containsArithmeticOperator(ref.Path) || containsArithmeticOperator(ref.Raw) {
		// Use the Raw field to get the full expression content
		// Raw is like "{{inputs.limit * inputs.threshold}}"
		expr := extractExpressionContent(ref.Raw)
		v.validateArithmeticExpressionInputs(expr, stepName, fieldName)
		return
	}
	if !v.inputNames[ref.Path] {
		v.result.AddError(ErrUndefinedInput, stepName,
			fmt.Sprintf("undefined input %q referenced in %s", ref.Path, fieldName))
	}
}

// extractExpressionContent extracts the content between {{ and }}.
func extractExpressionContent(raw string) string {
	// Remove {{ and }} from raw
	content := strings.TrimPrefix(raw, "{{")
	content = strings.TrimSuffix(content, "}}")
	return strings.TrimSpace(content)
}

// containsArithmeticOperator checks if a path contains arithmetic operators.
func containsArithmeticOperator(path string) bool {
	for _, c := range path {
		switch c {
		case '+', '-', '*', '/', '%', '(', ')':
			return true
		}
	}
	return false
}

// validateArithmeticExpressionInputs extracts and validates input references
// from arithmetic expressions like "limit * inputs.threshold".
// The expression format after parsing is: "firstVar operator inputs.secondVar..."
// where the "inputs." prefix is parsed away from the first variable.
func (v *TemplateValidator) validateArithmeticExpressionInputs(expr, stepName, fieldName string) {
	// Extract input names from the expression
	// The expression is in format: "limit * inputs.threshold" (first var has no prefix)
	// We need to extract: "limit" and "threshold"
	inputNames := extractInputNamesFromExpr(expr)

	for _, inputName := range inputNames {
		if !v.inputNames[inputName] {
			v.result.AddError(ErrUndefinedInput, stepName,
				fmt.Sprintf("undefined input %q referenced in %s", inputName, fieldName))
		}
	}
}

// extractInputNamesFromExpr extracts input variable names from an arithmetic expression.
// Handles expressions like:
// - "limit * inputs.threshold" -> ["limit", "threshold"]
// - "a + b" -> ["a", "b"]
// - "inputs.x * inputs.y" -> ["x", "y"]
func extractInputNamesFromExpr(expr string) []string {
	var names []string

	// Split on arithmetic operators and whitespace
	tokens := splitOnOperators(expr)

	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		// Handle "inputs.varname" pattern
		if strings.HasPrefix(token, "inputs.") {
			name := strings.TrimPrefix(token, "inputs.")
			if name != "" {
				names = append(names, name)
			}
		} else if isValidIdentifier(token) && !isReservedNamespace(token) {
			// Bare identifier (first var in expression)
			// Skip reserved namespace names like "inputs", "states", etc.
			names = append(names, token)
		}
	}

	return names
}

// isReservedNamespace checks if the identifier is a reserved template namespace.
// These can appear as fragments when parsing arithmetic expressions and should be skipped.
func isReservedNamespace(name string) bool {
	switch name {
	case "inputs", "states", "workflow", "env", "error", "context":
		return true
	}
	return false
}

// splitOnOperators splits an expression on arithmetic operators.
func splitOnOperators(expr string) []string {
	var tokens []string
	var current strings.Builder

	for _, c := range expr {
		switch c {
		case '+', '-', '*', '/', '%', '(', ')', ' ', '\t':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(c)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// isValidIdentifier checks if a string is a valid Go-style identifier.
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if i == 0 {
			if !isLetter(c) && c != '_' {
				return false
			}
		} else {
			if !isLetter(c) && !isDigit(c) && c != '_' {
				return false
			}
		}
	}
	return true
}

func isLetter(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isDigit(c rune) bool {
	return c >= '0' && c <= '9'
}

func (v *TemplateValidator) validateStateRef(ref *TemplateReference, stepName, fieldName string, currentStepIndex int) {
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
		// Missing property - state refs require a property like .Output, .Stderr, etc.
		v.result.AddError(ErrInvalidStateProperty, stepName,
			fmt.Sprintf("missing property for state reference %q in %s (expected .Output, .Stderr, .ExitCode, or .Status)", referencedStep, fieldName))
		return
	}

	// F050: Enforce uppercase casing for state properties to match Go export conventions.
	// Lowercase properties (output, stderr, exit_code, status) were never functional
	// with Go templates, which require exported fields (uppercase).
	if suggestion, isLowercase := lowercaseToUppercase[ref.Property]; isLowercase {
		v.result.AddError(ErrInvalidStateProperty, stepName,
			fmt.Sprintf("invalid state property %q for step %q in %s, use '%s' instead",
				ref.Property, referencedStep, fieldName, suggestion))
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

func (v *TemplateValidator) validateWorkflowRef(ref *TemplateReference, stepName, fieldName string) {
	// Check for lowercase properties and suggest PascalCase
	if suggestion, isLowercase := lowercaseToUppercaseWorkflow[ref.Path]; isLowercase {
		v.result.AddError(ErrInvalidWorkflowProperty, stepName,
			fmt.Sprintf("invalid workflow property %q in %s, use '%s' instead", ref.Path, fieldName, suggestion))
		return
	}

	if !ValidWorkflowProperties[ref.Path] {
		v.result.AddError(ErrInvalidWorkflowProperty, stepName,
			fmt.Sprintf("invalid workflow property %q in %s", ref.Path, fieldName))
	}
}

func (v *TemplateValidator) validateErrorRef(ref *TemplateReference, stepName, fieldName string, isErrorHook bool) {
	// Error references are only valid in error hook contexts
	if !isErrorHook {
		v.result.AddError(ErrErrorRefOutsideErrorHook, stepName,
			fmt.Sprintf("error reference %q used outside of error hook context in %s", ref.Raw, fieldName))
		return
	}

	// Check for lowercase properties and suggest PascalCase
	if suggestion, isLowercase := lowercaseToUppercaseError[ref.Path]; isLowercase {
		v.result.AddError(ErrInvalidErrorProperty, stepName,
			fmt.Sprintf("invalid error property %q in %s, use '%s' instead", ref.Path, fieldName, suggestion))
		return
	}

	// Validate the property
	if !ValidErrorProperties[ref.Path] {
		v.result.AddError(ErrInvalidErrorProperty, stepName,
			fmt.Sprintf("invalid error property %q in %s", ref.Path, fieldName))
	}
}

func (v *TemplateValidator) validateContextRef(ref *TemplateReference, stepName, fieldName string) {
	// Check for lowercase properties and suggest PascalCase
	if suggestion, isLowercase := lowercaseToUppercaseContext[ref.Path]; isLowercase {
		v.result.AddError(ErrInvalidContextProperty, stepName,
			fmt.Sprintf("invalid context property %q in %s, use '%s' instead", ref.Path, fieldName, suggestion))
		return
	}

	if !ValidContextProperties[ref.Path] {
		v.result.AddError(ErrInvalidContextProperty, stepName,
			fmt.Sprintf("invalid context property %q in %s", ref.Path, fieldName))
	}
}

func (v *TemplateValidator) validateLoopRef(ref *TemplateReference, stepName, fieldName string) {
	// Loop references are only accessible in template interpolation
	// We validate the property exists in ValidLoopProperties
	if !ValidLoopProperties[ref.Path] {
		v.result.AddError(ErrInvalidLoopProperty, stepName,
			fmt.Sprintf("invalid loop property %q in %s", ref.Path, fieldName))
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
				EnqueueIfNotVisited(&queue, visited, branch)
			}
		}

		// Add successors to queue (handle cycles gracefully)
		EnqueueIfNotVisited(&queue, visited, step.OnSuccess)
		EnqueueIfNotVisited(&queue, visited, step.OnFailure)
	}

	return order, nil
}

// EnqueueIfNotVisited adds state to queue if it hasn't been visited yet.
// This helper consolidates the repeated pattern of checking visited status
// before enqueueing in BFS traversal algorithms.
//
// Parameters:
//   - queue: pointer to the queue slice to modify in-place
//   - visited: map tracking which states have been visited
//   - state: the state name to potentially add to the queue
//
// The function modifies queue in-place only if visited[state] is false or
// state is not present in the visited map.
func EnqueueIfNotVisited(queue *[]string, visited map[string]bool, state string) {
	// If visited map is nil or state is not marked as visited, add to queue
	if visited == nil || !visited[state] {
		*queue = append(*queue, state)
	}
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
