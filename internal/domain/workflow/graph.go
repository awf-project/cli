package workflow

import (
	"strconv"
	"strings"
)

// VisitState represents the DFS visit state of a node during graph traversal.
// Used for three-color marking in cycle detection:
// - Unvisited: node has not been encountered yet
// - Visiting: node is currently in the DFS path (on stack)
// - Visited: node has been fully processed (all descendants explored)
type VisitState string

const (
	// VisitStateUnvisited indicates a node has not been encountered yet (white in DFS).
	VisitStateUnvisited VisitState = "unvisited"
	// VisitStateVisiting indicates a node is currently in the DFS path (gray in DFS).
	VisitStateVisiting VisitState = "visiting"
	// VisitStateVisited indicates a node has been fully processed (black in DFS).
	VisitStateVisited VisitState = "visited"
)

// String returns the string representation of the VisitState.
func (v VisitState) String() string {
	return string(v)
}

// FindCycleStart finds the index of target in path, returning -1 if not found.
// Used for locating where a cycle begins in a DFS path during cycle detection.
func FindCycleStart(path []string, target string) int {
	for i, state := range path {
		if state == target {
			return i
		}
	}
	return -1
}

// BuildCyclePath constructs the cycle path from startIndex to the end of path,
// appending target to close the cycle loop.
func BuildCyclePath(path []string, startIndex int, target string) []string {
	// Handle edge cases: invalid index or empty path
	if len(path) == 0 || startIndex < 0 || startIndex >= len(path) {
		return []string{target}
	}

	// Build cycle: elements from startIndex to end, plus target to close the loop
	cycleLen := len(path) - startIndex + 1
	cycle := make([]string, cycleLen)

	// Copy elements from startIndex to end
	copy(cycle, path[startIndex:])

	// Append target to close the cycle
	cycle[cycleLen-1] = target

	return cycle
}

// ValidateGraph performs graph validation on a workflow's state machine.
// It checks for:
// - All referenced states exist (on_success, on_failure targets)
// - All states are reachable from the initial state (orphan detection)
// - At least one terminal state exists
// - Cycle detection (warning, not error)
//
// Returns a ValidationResult containing errors and warnings.
//
//nolint:gocognit // Complexity 31: graph validation performs DFS cycle detection, reachability analysis, transition validation. Graph algorithms inherently complex.
func ValidateGraph(steps map[string]*Step, initial string) *ValidationResult {
	result := &ValidationResult{}

	// Check for empty steps
	if len(steps) == 0 {
		result.AddError(ErrMissingInitialState, "states", "no states defined")
		return result
	}

	// Check initial state exists
	if _, ok := steps[initial]; !ok {
		result.AddError(ErrMissingInitialState, "initial", "initial state '"+initial+"' not found")
		return result // Can't continue validation without initial state
	}

	// Validate all transitions reference existing states
	for name, step := range steps {
		// Check on_success
		if step.OnSuccess != "" {
			if _, ok := steps[step.OnSuccess]; !ok {
				result.AddError(ErrInvalidTransition, "states."+name+".on_success",
					"transition target '"+step.OnSuccess+"' not found")
			}
		}

		// Check on_failure
		if step.OnFailure != "" {
			if _, ok := steps[step.OnFailure]; !ok {
				result.AddError(ErrInvalidTransition, "states."+name+".on_failure",
					"transition target '"+step.OnFailure+"' not found")
			}
		}

		// Check parallel branches
		if step.Type == StepTypeParallel {
			for i, branch := range step.Branches {
				if _, ok := steps[branch]; !ok {
					result.AddError(ErrInvalidTransition, "states."+name+".branches["+strconv.Itoa(i)+"]",
						"branch '"+branch+"' not found")
				}
			}
		}
	}

	// Check for unreachable states
	reachable := FindReachableStates(steps, initial)
	for name := range steps {
		if !reachable[name] {
			result.AddError(ErrUnreachableState, "states."+name,
				"state '"+name+"' is not reachable from initial state '"+initial+"'")
		}
	}

	// Check for at least one terminal state (among reachable states)
	hasTerminal := false
	for name, step := range steps {
		if step.Type == StepTypeTerminal && reachable[name] {
			hasTerminal = true
			break
		}
	}
	if !hasTerminal {
		result.AddError(ErrNoTerminalState, "states", "no reachable terminal state found")
	}

	// Detect cycles (warnings, not errors)
	cycles := DetectCycles(steps, initial)
	for _, cycle := range cycles {
		result.AddWarning(ErrCycleDetected, "", "cycle detected: "+cycle)
	}

	return result
}

// FindReachableStates performs DFS from initial to find all reachable states.
func FindReachableStates(steps map[string]*Step, initial string) map[string]bool {
	reachable := make(map[string]bool)

	// Check if initial state exists
	if _, ok := steps[initial]; !ok {
		return reachable
	}

	// DFS traversal
	var visit func(name string)
	visit = func(name string) {
		// Already visited
		if reachable[name] {
			return
		}

		step, ok := steps[name]
		if !ok {
			return
		}

		reachable[name] = true

		// Visit all transitions
		for _, next := range GetTransitions(step) {
			visit(next)
		}
	}

	visit(initial)
	return reachable
}

// DetectCycles uses DFS with color marking to detect cycles in the state graph.
// Returns a list of cycle paths found (e.g., ["A -> B -> C -> A"]).
func DetectCycles(steps map[string]*Step, initial string) []string {
	color := make(map[string]VisitState)
	var cycles []string

	// Initialize all states as unvisited
	for name := range steps {
		color[name] = VisitStateUnvisited
	}

	// DFS from initial state
	var dfs func(name string, path []string) bool
	dfs = func(name string, path []string) bool {
		step, ok := steps[name]
		if !ok {
			return false
		}

		color[name] = VisitStateVisiting
		path = append(path, name)

		for _, next := range GetTransitions(step) {
			if _, exists := steps[next]; !exists {
				continue // Skip invalid transitions (handled elsewhere)
			}

			switch color[next] {
			case VisitStateVisiting:
				// Found a cycle - build the cycle path
				cycleStart := FindCycleStart(path, next)
				if cycleStart >= 0 {
					cyclePath := BuildCyclePath(path, cycleStart, next)
					cycles = append(cycles, formatCyclePath(cyclePath))
				} else {
					// Self-loop
					cycles = append(cycles, formatCyclePath([]string{next, next}))
				}
			case VisitStateUnvisited:
				dfs(next, path)
			case VisitStateVisited:
				// Skip already fully explored nodes
			}
		}

		color[name] = VisitStateVisited
		return false
	}

	// Start DFS from initial if it exists
	if _, ok := steps[initial]; ok {
		dfs(initial, nil)
	}

	return cycles
}

// formatCyclePath formats a cycle path as "A -> B -> C -> A".
func formatCyclePath(path []string) string {
	return strings.Join(path, " -> ")
}

// NextDefaultStep returns the next step name following the default (unconditional) path.
// Checks Transitions for an entry with empty When first; for loop steps falls back to
// Loop.OnComplete; otherwise falls back to OnSuccess.
func NextDefaultStep(step *Step) string {
	if step == nil {
		return ""
	}
	for _, tr := range step.Transitions {
		if tr.When == "" {
			return tr.Goto
		}
	}
	if step.Loop != nil && step.Loop.OnComplete != "" {
		return step.Loop.OnComplete
	}
	return step.OnSuccess
}

// ExecutionOrder returns steps in default-path order by walking the graph from Initial,
// following default transitions (empty When) and falling back to OnSuccess.
// For loop steps (for_each/while), body steps are included inline before continuing
// to the loop's on_complete target.
// Stops at terminal steps, visited steps (cycle prevention), or missing references.
func ExecutionOrder(wf *Workflow) []Step {
	if wf == nil || len(wf.Steps) == 0 || wf.Initial == "" {
		return nil
	}

	visited := make(map[string]bool, len(wf.Steps))
	steps := make([]Step, 0, len(wf.Steps))
	current := wf.Initial

	for current != "" && !visited[current] {
		step, ok := wf.Steps[current]
		if !ok {
			break
		}
		visited[current] = true
		steps = append(steps, *step)

		if step.Type == StepTypeTerminal {
			break
		}

		// For loop steps, include body steps inline
		if step.Loop != nil {
			for _, bodyName := range step.Loop.Body {
				if visited[bodyName] {
					continue
				}
				bodyStep, exists := wf.Steps[bodyName]
				if !exists {
					continue
				}
				visited[bodyName] = true
				steps = append(steps, *bodyStep)
			}
		}

		current = NextDefaultStep(step)
	}

	return steps
}

// GetTransitions returns all outbound transitions from a step.
// For command/operation/call_workflow steps: on_success, on_failure
// For parallel steps: on_success, on_failure, and all branches
// For loop steps (for_each/while): on_success, on_failure, body steps, and on_complete
// For terminal steps: empty
func GetTransitions(step *Step) []string {
	if step == nil {
		return nil
	}

	// Terminal steps have no outbound transitions
	if step.Type == StepTypeTerminal {
		return nil
	}

	var transitions []string

	// Add on_success if defined
	if step.OnSuccess != "" {
		transitions = append(transitions, step.OnSuccess)
	}

	// Add on_failure if defined
	if step.OnFailure != "" {
		transitions = append(transitions, step.OnFailure)
	}

	// For parallel steps, add branches
	if step.Type == StepTypeParallel {
		transitions = append(transitions, step.Branches...)
	}

	// For loop steps, add body steps and on_complete
	if step.Loop != nil {
		transitions = append(transitions, step.Loop.Body...)
		if step.Loop.OnComplete != "" {
			transitions = append(transitions, step.Loop.OnComplete)
		}
	}

	return transitions
}
