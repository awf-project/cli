// Package workflow provides the core domain entities for AWF workflow execution.
//
// This package defines the workflow entity model, state machine execution,
// and configuration types for orchestrating AI agent interactions. It follows
// hexagonal architecture principles with zero dependencies on infrastructure.
//
// # Architecture
//
// The workflow package is the heart of the domain layer:
//   - Defines workflow entities (Workflow, Step, State) with validation logic
//   - Implements state machine execution model with transitions
//   - Provides thread-safe execution context for concurrent step execution
//   - Declares port interfaces via ExpressionCompiler for infrastructure adapters
//
// All types in this package are pure domain models with business logic only.
// Infrastructure concerns (file I/O, HTTP, shell execution) are delegated to
// ports defined in internal/domain/ports.
//
// # Core Entities
//
// ## Workflow (workflow.go)
//
// The root entity representing a complete workflow definition:
//   - Workflow: Complete workflow with name, inputs, steps, and hooks
//   - Input: Input parameter definition with type and validation rules
//   - InputValidation: Validation constraints for input parameters
//
// ## Step (step.go)
//
// Step types define the state machine nodes:
//   - Step: Single step in workflow state machine (8 types supported)
//   - StepType: Enum for step types (command, parallel, terminal, for_each, while, operation, call_workflow, agent)
//   - RetryConfig: Retry behavior with backoff and jitter
//   - CaptureConfig: Output capture configuration for command steps
//
// ## ExecutionContext (context.go)
//
// Thread-safe runtime state management:
//   - ExecutionContext: Thread-safe workflow execution state with sync.RWMutex
//   - StepState: Execution state of a single step (status, output, timing)
//   - ExecutionStatus: Status enum (pending, running, completed, failed, cancelled)
//   - LoopContext: Loop iteration state for for_each and while steps
//
// # Configuration Types
//
// ## Parallel Execution (parallel.go)
//
// Parallel step configuration and strategies:
//   - Strategy values: all_succeed, any_succeed, best_effort
//   - MaxConcurrent: Limit concurrent branch execution
//   - DependsOn: Declare ordering constraints between branches
//
// ## Loops (loop.go)
//
// Iterative execution with for_each and while:
//   - LoopConfig: Loop configuration with type, items, condition, and body
//   - LoopContext: Runtime iteration state with item, index, first/last flags
//   - Nested loops supported via Parent reference (F043)
//
// ## Conditional Transitions (condition.go)
//
// Expression-based state transitions:
//   - Transition: Single conditional transition with condition expression and target
//   - Transitions: Ordered list of transitions evaluated sequentially
//   - Condition: Boolean expression evaluated against execution context
//
// ## Hooks (hooks.go)
//
// Pre/post execution actions:
//   - WorkflowHooks: Lifecycle hooks (WorkflowStart, WorkflowEnd, WorkflowError, WorkflowCancel)
//   - StepHooks: Step-level hooks (Pre, Post)
//   - HookAction: Log message or shell command to execute
//
// # Execution Results
//
// ## Step Execution (context.go)
//
// Step execution captures:
//   - StepState: Status, output, stderr, exit code, timing, error
//   - Response: Parsed JSON response from agent steps (map[string]any)
//   - Tokens: Token usage tracking for AI agent steps
//   - OutputPath/StderrPath: Temp file paths for streamed output (C019)
//
// ## Workflow Outcomes (workflow.go)
//
// Terminal step status:
//   - TerminalStatus: success or failure
//   - Terminal steps define workflow completion state
//
// # Templates
//
// ## Workflow Templates (template.go)
//
// Reusable workflow fragments:
//   - Template: Named template with parameters and state definitions
//   - TemplateParam: Parameter definition (name, required, default)
//   - WorkflowTemplateRef: Reference to template from step with parameter bindings
//
// ## Template Validation (template_validation.go)
//
// Template-specific validation:
//   - ValidateTemplateReference: Check parameter bindings match template signature
//   - ValidateTemplateExpansion: Verify expanded template produces valid workflow fragment
//
// # AI Agent Integration (F039)
//
// ## Agent Configuration (agent_config.go)
//
// AI agent invocation:
//   - AgentConfig: Provider, prompt, options, timeout, mode
//   - Provider values: claude, codex, gemini, opencode, custom
//   - Mode: single (one-shot) or conversation (multi-turn)
//
// ## Conversation Mode (F033, conversation.go)
//
// Multi-turn agent interactions:
//   - ConversationConfig: Max turns, stop conditions, context window management
//   - ConversationState: Conversation history and message accumulation
//   - ConversationMessage: Single message with role (user/assistant/system) and content
//   - ContextWindowState: Token budget tracking and history truncation state
//
// # Subworkflows (F023)
//
// ## Call Workflow (subworkflow.go)
//
// Invoke child workflows:
//   - CallWorkflowConfig: Workflow name, inputs, circular detection
//   - CallStack: Active workflow names for circular call prevention
//
// # Validation
//
// ## Workflow Validation (validation.go)
//
// Comprehensive validation rules:
//   - Workflow.Validate(): Check name, initial state, terminal states, step graph
//   - Step.Validate(): Type-specific validation (command, agent, parallel, loop, operation)
//   - Graph validation: Detect unreachable states, cycles, missing targets
//
// ## Validation Errors (validation_errors.go)
//
// Structured error types:
//   - ValidationError: Base validation error with field and reason
//   - GraphValidationError: Graph-specific errors (unreachable, cycle, missing target)
//   - InputValidationError: Input parameter validation failures
//
// # Usage Examples
//
// ## Basic Workflow Construction
//
// Create a simple workflow with command and terminal steps:
//
//	wf := &workflow.Workflow{
//	    Name:    "hello-world",
//	    Version: "1.0.0",
//	    Initial: "greet",
//	    Steps: map[string]*workflow.Step{
//	        "greet": {
//	            Name:      "greet",
//	            Type:      workflow.StepTypeCommand,
//	            Command:   "echo 'Hello, {{inputs.name}}'",
//	            OnSuccess: "end",
//	        },
//	        "end": {
//	            Name:   "end",
//	            Type:   workflow.StepTypeTerminal,
//	            Status: workflow.TerminalSuccess,
//	        },
//	    },
//	}
//
//	// Validate workflow
//	if err := wf.Validate(validator, nil); err != nil {
//	    log.Fatal(err)
//	}
//
// ## Execution Context Management
//
// Thread-safe execution context for workflow runs:
//
//	ctx := workflow.NewExecutionContext("wf-123", "hello-world")
//	ctx.SetInput("name", "Alice")
//	ctx.SetCurrentStep("greet")
//
//	// Record step execution
//	state := workflow.StepState{
//	    Name:        "greet",
//	    Status:      workflow.StatusCompleted,
//	    Output:      "Hello, Alice",
//	    ExitCode:    0,
//	    StartedAt:   time.Now().Add(-1 * time.Second),
//	    CompletedAt: time.Now(),
//	}
//	ctx.SetStepState("greet", state)
//
//	// Access results thread-safely
//	if output, ok := ctx.GetStepOutput("greet"); ok {
//	    fmt.Println(output) // "Hello, Alice"
//	}
//
// ## Parallel Execution
//
// Workflow with parallel branches:
//
//	wf := &workflow.Workflow{
//	    Name:    "parallel-demo",
//	    Initial: "parallel",
//	    Steps: map[string]*workflow.Step{
//	        "parallel": {
//	            Name:          "parallel",
//	            Type:          workflow.StepTypeParallel,
//	            Branches:      []string{"task1", "task2", "task3"},
//	            Strategy:      "all_succeed",
//	            MaxConcurrent: 2,
//	            OnSuccess:     "end",
//	        },
//	        "task1": {
//	            Name:    "task1",
//	            Type:    workflow.StepTypeCommand,
//	            Command: "echo 'Task 1'",
//	        },
//	        "task2": {
//	            Name:    "task2",
//	            Type:    workflow.StepTypeCommand,
//	            Command: "echo 'Task 2'",
//	        },
//	        "task3": {
//	            Name:    "task3",
//	            Type:    workflow.StepTypeCommand,
//	            Command: "echo 'Task 3'",
//	        },
//	        "end": {
//	            Name:   "end",
//	            Type:   workflow.StepTypeTerminal,
//	            Status: workflow.TerminalSuccess,
//	        },
//	    },
//	}
//
// ## Loop Execution
//
// For-each loop over items:
//
//	loopStep := &workflow.Step{
//	    Name: "process_items",
//	    Type: workflow.StepTypeForEach,
//	    Loop: &workflow.LoopConfig{
//	        Items: []string{"apple", "banana", "cherry"},
//	        Body:  []string{"print_item"},
//	    },
//	    OnSuccess: "end",
//	}
//
//	printStep := &workflow.Step{
//	    Name:    "print_item",
//	    Type:    workflow.StepTypeCommand,
//	    Command: "echo 'Item {{loop.index}}: {{loop.item}}'",
//	}
//
//	// Runtime loop context access
//	loopCtx := &workflow.LoopContext{
//	    Item:   "apple",
//	    Index:  0,
//	    First:  true,
//	    Last:   false,
//	    Length: 3,
//	}
//
// ## AI Agent Invocation
//
// Single-shot agent execution:
//
//	agentStep := &workflow.Step{
//	    Name: "analyze",
//	    Type: workflow.StepTypeAgent,
//	    Agent: &workflow.AgentConfig{
//	        Provider: "claude",
//	        Prompt:   "Analyze this log: {{inputs.log_file}}",
//	        Options: map[string]any{
//	            "model":       "claude-3-sonnet",
//	            "temperature": 0.7,
//	            "max_tokens":  1000,
//	        },
//	        Timeout: 60,
//	        Mode:    "single",
//	    },
//	    OnSuccess: "end",
//	}
//
// ## Conversation Mode Agent
//
// Interactive multi-turn agent conversation. The user drives the loop by
// providing input at each turn and ends the session with "exit" or "quit".
//
//	conversationStep := &workflow.Step{
//	    Name: "chat",
//	    Type: workflow.StepTypeAgent,
//	    Agent: &workflow.AgentConfig{
//	        Provider:     "claude",
//	        Mode:         "conversation",
//	        SystemPrompt: "You are a helpful coding assistant.",
//	        Prompt:       "Help me debug this code: {{inputs.code}}",
//	    },
//	    OnSuccess: "end",
//	}
//
// ## Conditional Transitions
//
// Expression-based state transitions:
//
//	step := &workflow.Step{
//	    Name:    "check_status",
//	    Type:    workflow.StepTypeCommand,
//	    Command: "curl https://api.example.com/status",
//	    Transitions: workflow.Transitions{
//	        {
//	            Condition: "{{states.check_status.exit_code}} == 0",
//	            Target:    "success",
//	        },
//	        {
//	            Condition: "{{states.check_status.exit_code}} == 404",
//	            Target:    "not_found",
//	        },
//	        {
//	            Condition: "true", // default fallback
//	            Target:    "error",
//	        },
//	    },
//	}
//
// ## Retry Configuration
//
// Exponential backoff with jitter:
//
//	step := &workflow.Step{
//	    Name:    "flaky_api",
//	    Type:    workflow.StepTypeCommand,
//	    Command: "curl https://api.example.com",
//	    Retry: &workflow.RetryConfig{
//	        MaxAttempts:        3,
//	        InitialDelayMs:     1000,
//	        MaxDelayMs:         10000,
//	        Backoff:            "exponential",
//	        Multiplier:         2.0,
//	        Jitter:             0.1,
//	        RetryableExitCodes: []int{1, 2, 7}, // connection errors
//	    },
//	    OnSuccess: "end",
//	}
//
// ## Hooks
//
// Lifecycle event handlers:
//
//	wf := &workflow.Workflow{
//	    Name:    "hooked-workflow",
//	    Initial: "main",
//	    Hooks: workflow.WorkflowHooks{
//	        WorkflowStart: workflow.Hook{
//	            {Log: "Starting workflow execution"},
//	            {Command: "date > /tmp/start.txt"},
//	        },
//	        WorkflowEnd: workflow.Hook{
//	            {Log: "Workflow completed successfully"},
//	        },
//	        WorkflowError: workflow.Hook{
//	            {Log: "Workflow failed: {{error.message}}"},
//	            {Command: "notify-admin.sh '{{error.message}}'"},
//	        },
//	    },
//	    Steps: map[string]*workflow.Step{
//	        "main": {
//	            Name:    "main",
//	            Type:    workflow.StepTypeCommand,
//	            Command: "echo 'Main task'",
//	            Hooks: workflow.StepHooks{
//	                Pre: workflow.Hook{
//	                    {Log: "Starting main task"},
//	                },
//	                Post: workflow.Hook{
//	                    {Log: "Main task completed"},
//	                },
//	            },
//	            OnSuccess: "end",
//	        },
//	        "end": {
//	            Name:   "end",
//	            Type:   workflow.StepTypeTerminal,
//	            Status: workflow.TerminalSuccess,
//	        },
//	    },
//	}
//
// # Design Principles
//
// ## Pure Domain Logic
//
// Zero infrastructure dependencies:
//   - No file I/O, HTTP, database, or shell execution code
//   - Infrastructure concerns delegated to ports (interfaces)
//   - Domain logic expressed through validation rules and state transitions
//
// ## Thread Safety
//
// ExecutionContext uses sync.RWMutex for concurrent access:
//   - Parallel step execution requires thread-safe state management
//   - All context mutations acquire write lock
//   - Read operations use read lock for performance
//   - Validated with `go test -race ./...`
//
// ## Validation First
//
// Static validation before execution:
//   - Workflow.Validate() checks structure before runtime
//   - Graph validation detects unreachable states and cycles
//   - Type-specific validation for each step type
//   - Expression validation delegated to ExpressionCompiler port
//
// ## Expression Compilation Interface
//
// ExpressionCompiler port abstraction:
//   - Domain declares interface, infrastructure implements
//   - Used for validating conditional expressions in agent configs
//   - Enables testing without concrete template engine
//
// # Related Documentation
//
// See also:
//   - internal/domain/ports: Port interfaces for adapters
//   - internal/application: Application services orchestrating workflow execution
//   - docs/architecture.md: Hexagonal architecture overview
//   - CLAUDE.md: Project conventions and workflow execution semantics
package workflow
