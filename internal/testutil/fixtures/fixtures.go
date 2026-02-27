package fixtures

import (
	"fmt"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// This file contains workflow fixture factories for creating common test workflows
// with configurable parameters and sensible defaults.
//
// All fixtures:
// - Return new *workflow.Workflow instances
// - Accept variadic parameters for customization
// - Provide sensible defaults when no parameters given
// - Return valid workflows (pass Validate())
// - Are thread-safe (no shared state)
//
// Usage:
//
//	wf := testutil.SimpleWorkflow()                    // Default simple workflow
//	wf := testutil.SimpleWorkflow("my-workflow")       // Custom name
//	wf := testutil.LinearWorkflow(5)                   // 5 command steps + terminal
//	wf := testutil.ParallelWorkflow(3, "all_succeed")  // 3 branches with strategy
//	wf := testutil.LoopWorkflow("for_each", items)     // ForEach loop
//	wf := testutil.ConversationWorkflow("anthropic", "claude-3-opus")

// SimpleWorkflow creates a minimal valid workflow with a single command step and terminal.
//
// Parameters:
//   - args[0] (string): Custom workflow name (default: "simple-workflow")
//   - args[1] (string): Custom command (default: "echo 'hello'")
//
// Returns a workflow with:
//   - 1 command step (executes shell command)
//   - 1 terminal step (success)
//
// Example:
//
//	wf := SimpleWorkflow()                        // Uses defaults
//	wf := SimpleWorkflow("my-workflow")           // Custom name
//	wf := SimpleWorkflow("test", "echo 'test'")   // Custom name and command
func SimpleWorkflow(args ...any) *workflow.Workflow {
	// Parse parameters with defaults
	name := "simple-workflow"
	command := "echo 'hello'"

	if len(args) > 0 {
		if n, ok := args[0].(string); ok {
			name = n
		}
	}
	if len(args) > 1 {
		if cmd, ok := args[1].(string); ok {
			command = cmd
		}
	}

	// Create workflow with command step and terminal
	return &workflow.Workflow{
		Name:    name,
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   command,
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}
}

// LinearWorkflow creates a workflow with N sequential command steps followed by terminal.
//
// Parameters:
//   - args[0] (int): Number of command steps (default: 3)
//
// Returns a workflow with:
//   - N command steps in sequence (step1 -> step2 -> ... -> stepN)
//   - 1 terminal step at the end
//   - Each step has OnSuccess pointing to next step
//
// Example:
//
//	wf := LinearWorkflow()     // 3 command steps + terminal
//	wf := LinearWorkflow(5)    // 5 command steps + terminal
func LinearWorkflow(args ...any) *workflow.Workflow {
	// Parse parameters with defaults
	stepCount := 3
	if len(args) > 0 {
		if count, ok := args[0].(int); ok {
			stepCount = count
		}
	}

	// Minimum of 1 step
	if stepCount < 1 {
		stepCount = 1
	}

	// Create steps map
	steps := make(map[string]*workflow.Step)

	// Create sequential command steps
	for i := 1; i <= stepCount; i++ {
		stepName := fmt.Sprintf("step%d", i)
		nextStep := "done"
		if i < stepCount {
			nextStep = fmt.Sprintf("step%d", i+1)
		}

		steps[stepName] = &workflow.Step{
			Name:      stepName,
			Type:      workflow.StepTypeCommand,
			Command:   fmt.Sprintf("echo 'step %d'", i),
			OnSuccess: nextStep,
		}
	}

	// Add terminal step
	steps["done"] = &workflow.Step{
		Name:   "done",
		Type:   workflow.StepTypeTerminal,
		Status: workflow.TerminalSuccess,
	}

	return &workflow.Workflow{
		Name:    "linear-workflow",
		Initial: "step1",
		Steps:   steps,
	}
}

// ParallelWorkflow creates a workflow with parallel execution branches.
//
// Parameters:
//   - args[0] (int): Number of parallel branches (default: 2)
//   - args[1] (string): Parallel strategy (default: "all_succeed")
//     Valid values: "all_succeed", "any_succeed", "best_effort"
//
// Returns a workflow with:
//   - 1 parallel step with N branches
//   - N branch command steps
//   - 1 terminal step after parallel completion
//
// Example:
//
//	wf := ParallelWorkflow()                      // 2 branches, all_succeed
//	wf := ParallelWorkflow(5)                     // 5 branches
//	wf := ParallelWorkflow(3, "any_succeed")      // 3 branches, any_succeed strategy
func ParallelWorkflow(args ...any) *workflow.Workflow {
	// Parse parameters with defaults
	branchCount := 2
	strategy := "all_succeed"

	if len(args) > 0 {
		if count, ok := args[0].(int); ok {
			branchCount = count
		}
	}
	if len(args) > 1 {
		if s, ok := args[1].(string); ok {
			strategy = s
		}
	}

	// Minimum of 2 branches
	if branchCount < 2 {
		branchCount = 2
	}

	// Create steps map
	steps := make(map[string]*workflow.Step)

	// Create branch names
	branches := make([]string, branchCount)
	for i := 0; i < branchCount; i++ {
		branchName := fmt.Sprintf("branch%d", i+1)
		branches[i] = branchName

		// Create branch step - each branch needs OnSuccess
		steps[branchName] = &workflow.Step{
			Name:      branchName,
			Type:      workflow.StepTypeCommand,
			Command:   fmt.Sprintf("echo 'branch %d'", i+1),
			OnSuccess: "done",
		}
	}

	// Create parallel step
	steps["parallel"] = &workflow.Step{
		Name:      "parallel",
		Type:      workflow.StepTypeParallel,
		Branches:  branches,
		Strategy:  strategy,
		OnSuccess: "done",
	}

	// Add terminal step
	steps["done"] = &workflow.Step{
		Name:   "done",
		Type:   workflow.StepTypeTerminal,
		Status: workflow.TerminalSuccess,
	}

	return &workflow.Workflow{
		Name:    "parallel-workflow",
		Initial: "parallel",
		Steps:   steps,
	}
}

// LoopWorkflow creates a workflow with for_each or while loop step.
//
// Parameters:
//   - args[0] (string): Loop type - "for_each" or "while" (default: "for_each")
//   - args[1] ([]string or string): For for_each: items array, for while: condition expression
//
// Returns a workflow with:
//   - 1 loop step (for_each or while type)
//   - Loop body with command step(s)
//   - 1 terminal step after loop completion
//
// Example:
//
//	wf := LoopWorkflow()                                        // for_each with default items
//	wf := LoopWorkflow("for_each")                              // Explicit for_each
//	wf := LoopWorkflow("for_each", []string{"a", "b", "c"})     // Custom items
//	wf := LoopWorkflow("while")                                 // While loop with default condition
func LoopWorkflow(args ...any) *workflow.Workflow {
	// Parse parameters with defaults
	loopType := "for_each"
	items := "{{inputs.items}}"
	condition := "{{states.counter.output}} < 5"

	if len(args) > 0 {
		if lt, ok := args[0].(string); ok {
			loopType = lt
		}
	}

	// Handle items/condition parameter
	if len(args) > 1 {
		switch loopType {
		case "for_each":
			// For for_each, args[1] can be []string
			if itemsArray, ok := args[1].([]string); ok {
				// Convert array to template expression (tests just verify Loop.Items is not empty)
				items = fmt.Sprintf("%v", itemsArray)
			}
		case "while":
			// For while, args[1] is condition string
			if cond, ok := args[1].(string); ok {
				condition = cond
			}
		}
	}

	// Create steps map
	steps := make(map[string]*workflow.Step)

	// Create loop body step - body steps point back to the loop step
	steps["body_step"] = &workflow.Step{
		Name:      "body_step",
		Type:      workflow.StepTypeCommand,
		Command:   "echo 'loop iteration'",
		OnSuccess: "loop", // Loop body steps reference the loop step
	}

	// Create loop step based on type
	var loopStep *workflow.Step
	if loopType == "while" {
		loopStep = &workflow.Step{
			Name: "loop",
			Type: workflow.StepTypeWhile,
			Loop: &workflow.LoopConfig{
				Type:       workflow.LoopTypeWhile,
				Condition:  condition,
				Body:       []string{"body_step"},
				OnComplete: "done",
			},
			OnSuccess: "done",
		}
	} else {
		// for_each is default
		loopStep = &workflow.Step{
			Name: "loop",
			Type: workflow.StepTypeForEach,
			Loop: &workflow.LoopConfig{
				Type:       workflow.LoopTypeForEach,
				Items:      items,
				Body:       []string{"body_step"},
				OnComplete: "done",
			},
			OnSuccess: "done",
		}
	}
	steps["loop"] = loopStep

	// Add terminal step
	steps["done"] = &workflow.Step{
		Name:   "done",
		Type:   workflow.StepTypeTerminal,
		Status: workflow.TerminalSuccess,
	}

	return &workflow.Workflow{
		Name:    "loop-workflow",
		Initial: "loop",
		Steps:   steps,
	}
}

// ConversationWorkflow creates a workflow with AI agent conversation step.
//
// Parameters:
//   - args[0] (string): AI provider (default: "anthropic")
//     Valid values: "anthropic", "openai", "google", "custom"
//   - args[1] (string): Model name (default: "claude-3-sonnet")
//   - args[2] (int): Number of conversation turns (default: 1, multi-turn if > 1)
//
// Returns a workflow with:
//   - 1+ agent steps with AI configuration
//   - AgentConfig with Provider, Model, Prompt
//   - 1 terminal step after agent execution
//
// Example:
//
//	wf := ConversationWorkflow()                              // anthropic claude-3-sonnet, single turn
//	wf := ConversationWorkflow("openai", "gpt-4")             // openai gpt-4
//	wf := ConversationWorkflow("anthropic", "claude-3-opus", 3) // 3-turn conversation
func ConversationWorkflow(args ...any) *workflow.Workflow {
	// Parse parameters with defaults
	provider := "anthropic"
	model := "claude-3-sonnet"
	turns := 1

	if len(args) > 0 {
		if p, ok := args[0].(string); ok {
			provider = p
		}
	}
	if len(args) > 1 {
		if m, ok := args[1].(string); ok {
			model = m
		}
	}
	if len(args) > 2 {
		if t, ok := args[2].(int); ok {
			turns = t
		}
	}

	// Minimum of 1 turn
	if turns < 1 {
		turns = 1
	}

	// Create steps map
	steps := make(map[string]*workflow.Step)

	// Create agent step(s)
	for i := 1; i <= turns; i++ {
		stepName := fmt.Sprintf("agent%d", i)
		nextStep := "done"
		if i < turns {
			nextStep = fmt.Sprintf("agent%d", i+1)
		}

		steps[stepName] = &workflow.Step{
			Name: stepName,
			Type: workflow.StepTypeAgent,
			Agent: &workflow.AgentConfig{
				Provider: provider,
				Prompt:   "You are a helpful assistant.",
				Options: map[string]any{
					"model": model,
				},
			},
			OnSuccess: nextStep,
		}
	}

	// Add terminal step
	steps["done"] = &workflow.Step{
		Name:   "done",
		Type:   workflow.StepTypeTerminal,
		Status: workflow.TerminalSuccess,
	}

	return &workflow.Workflow{
		Name:    "conversation-workflow",
		Initial: "agent1",
		Steps:   steps,
	}
}
