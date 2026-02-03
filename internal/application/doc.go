// Package application provides application services that orchestrate domain operations.
//
// The application layer sits between the interfaces layer (CLI, API) and the domain layer,
// coordinating workflow execution through ports and domain entities. Services in this package
// implement use cases and business workflows without containing business rules (which reside
// in the domain layer). All infrastructure dependencies are injected via port interfaces.
//
// # Architecture Role
//
// In the hexagonal architecture:
//   - Application services receive requests from interfaces layer (CLI commands)
//   - Services orchestrate domain entities (Workflow, Step, ExecutionContext)
//   - Services delegate infrastructure work to ports (CommandExecutor, StateStore)
//   - Services coordinate cross-cutting concerns (logging, state persistence, history)
//
// The application layer enforces the dependency rule: it depends on domain ports (inward)
// but is depended upon by the interfaces layer (outward). Infrastructure adapters implement
// the ports that services consume.
//
// # Core Orchestration Services
//
// ## WorkflowService (service.go)
//
// High-level workflow operations:
//   - ListWorkflows: Enumerate available workflows
//   - GetWorkflow: Load workflow definition by name
//   - ValidateWorkflow: Static validation of workflow structure
//   - WorkflowExists: Check if workflow is available
//
// Dependencies: WorkflowRepository, StateStore, CommandExecutor, Logger
//
// ## ExecutionService (execution_service.go)
//
// Core workflow execution orchestrator:
//   - ExecuteWorkflow: Full workflow execution with state machine traversal
//   - ExecuteStep: Single step execution dispatcher (routes by StepType)
//   - HandleTransitions: Conditional transition evaluation
//   - SaveCheckpoint: Persist execution state for resumption
//
// Step type handlers:
//   - executeCommandStep: Shell command execution with capture
//   - executeAgentStep: AI agent invocation (single-shot and conversation)
//   - executeParallelStep: Concurrent branch execution
//   - executeForEachStep: Loop over items
//   - executeWhileStep: Conditional loop
//   - executeOperationStep: Plugin operation invocation
//   - executeCallWorkflowStep: Nested sub-workflow execution
//   - executeTerminalStep: Workflow completion
//
// Dependencies: WorkflowService, CommandExecutor, ParallelExecutor, StateStore,
// Logger, ExpressionEvaluator, HookExecutor, LoopExecutor, TemplateService,
// HistoryService, OperationProvider, AgentRegistry, ConversationExecutor
//
// Optional configuration:
//   - SetOutputWriters: Configure stdout/stderr streaming
//   - SetTemplateService: Enable template expansion
//   - SetOperationProvider: Enable plugin operations
//   - SetAgentRegistry: Enable AI agent steps
//   - SetEvaluator: Enable conditional transitions
//   - SetConversationManager: Enable multi-turn conversations
//
// ## HistoryService (history_service.go)
//
// Workflow execution history and analytics:
//   - RecordExecution: Persist workflow run record with metadata
//   - GetExecutionHistory: Query execution records (filtering, pagination)
//   - GetExecutionStats: Aggregate statistics (counts, durations, success rates)
//   - PruneOldHistory: Clean up old execution records
//
// Dependencies: HistoryStore, Logger
//
// ## TemplateService (template_service.go)
//
// Workflow template expansion and validation:
//   - ExpandTemplate: Resolve template reference into concrete steps
//   - ValidateTemplate: Check template syntax and parameter bindings
//   - LoadTemplate: Retrieve template definition by name
//
// Dependencies: TemplateRepository, Logger, ExpressionValidator
//
// ## PluginService (plugin_service.go)
//
// Plugin lifecycle management:
//   - DiscoverPlugins: Scan and list available plugins
//   - LoadPlugin: Initialize plugin with configuration
//   - EnablePlugin: Activate plugin for workflow use
//   - DisablePlugin: Deactivate plugin
//   - GetPluginStatus: Check plugin enabled/disabled state
//
// Dependencies: PluginManager, PluginStateStore, Logger
//
// ## InputCollectionService (input_collection_service.go)
//
// Interactive input gathering:
//   - CollectInputs: Prompt user for missing workflow inputs
//   - ValidateInputs: Check input values against validation rules
//   - BuildInputMap: Construct workflow input map from user responses
//
// Dependencies: InputCollector, Logger
//
// # Specialized Executors
//
// ## ParallelExecutor (parallel_executor.go)
//
// Concurrent branch execution with strategies:
//   - Execute: Run branches concurrently with semaphore control
//   - all_succeed: All branches must succeed
//   - any_succeed: At least one branch succeeds
//   - best_effort: Execute all, report best outcome
//
// Features:
//   - MaxConcurrent: Limit concurrent goroutines
//   - DependsOn: Branch execution ordering constraints
//   - Graceful cancellation via context
//
// Dependencies: StepExecutor, Logger
//
// ## LoopExecutor (loop_executor.go)
//
// Iterative step execution:
//   - ExecuteForEach: Iterate over items collection
//   - ExecuteWhile: Conditional loop with guard expression
//   - BuildLoopContext: Construct loop variables (item, index, first, last)
//   - EvaluateLoopCondition: Check while loop guard
//
// Loop context variables:
//   - {{loop.item}}: Current item value
//   - {{loop.index}}: Zero-based iteration index
//   - {{loop.first}}: Boolean first iteration flag
//   - {{loop.last}}: Boolean last iteration flag
//   - {{loop.length}}: Total items count (for_each only)
//
// Dependencies: StepExecutor, ExpressionEvaluator, Logger
//
// ## InteractiveExecutor (interactive_executor.go)
//
// Step-by-step interactive execution:
//   - ExecuteInteractive: Prompt user before each step
//   - PromptUserAction: Present execution options (run, skip, retry, abort, edit)
//   - HandleUserChoice: Process user action and update state
//
// User actions:
//   - Run: Execute step normally
//   - Skip: Skip step and proceed to next
//   - Retry: Re-attempt previous step
//   - Abort: Halt workflow execution
//   - Edit: Modify step parameters before execution
//
// Dependencies: InteractivePrompt, StepExecutor, Logger
//
// ## DryRunExecutor (dry_run_executor.go)
//
// Workflow validation without side effects:
//   - ExecuteDryRun: Traverse workflow without executing commands
//   - LogPlannedActions: Report steps that would execute
//   - ValidateGraph: Check state transitions are valid
//
// Dependencies: WorkflowService, Logger
//
// ## HookExecutor (hook_executor.go)
//
// Workflow and step lifecycle hooks:
//   - ExecuteWorkflowHooks: Workflow start/end/error/cancel events
//   - ExecuteStepHooks: Step pre/post execution hooks
//   - ExecuteHookAction: Process individual hook (log or command)
//
// Hook types:
//   - Log: Structured log message with template interpolation
//   - Command: Shell command execution
//
// Dependencies: CommandExecutor, Logger, ExpressionEvaluator
//
// ## ConversationManager (conversation_manager.go)
//
// Multi-turn AI agent conversations:
//   - ExecuteConversation: Manage conversation loop with turn limits
//   - EvaluateStopConditions: Check conversation termination criteria
//   - ManageContextWindow: Trim conversation history for token budget
//   - RecordMessage: Append user/assistant messages to history
//
// Context window strategies:
//   - truncate_middle: Remove middle messages, keep first/last
//   - summarize: Use LLM to summarize old messages
//   - truncate_oldest: Drop oldest messages first
//
// Dependencies: AgentProvider, Tokenizer, ExpressionEvaluator, Logger
//
// # Supporting Services
//
// ## OutputStreamer (output_streamer.go)
//
// Real-time command output streaming:
//   - StreamOutput: Write command stdout to writer during execution
//   - StreamError: Write command stderr to writer during execution
//   - CaptureOutput: Buffer output for context interpolation
//
// Dependencies: io.Writer (stdout, stderr)
//
// ## OutputLimiter (output_limiter.go)
//
// Memory protection for large outputs:
//   - LimitOutput: Enforce maximum output size
//   - TruncateOutput: Trim output exceeding limit
//   - ReportTruncation: Log truncation events
//
// Prevents out-of-memory errors from unbounded command output accumulation.
//
// Dependencies: Logger
//
// ## MemoryMonitor (memory_monitor.go)
//
// Runtime memory usage tracking:
//   - MonitorMemory: Periodic memory usage checks
//   - ReportMemoryPressure: Alert on high memory usage
//   - TriggerGarbageCollection: Force GC under memory pressure
//
// Dependencies: Logger
//
// # Usage Examples
//
// ## Basic Workflow Execution
//
// Orchestrate workflow execution with injected dependencies:
//
//	wfSvc := NewWorkflowService(repo, store, executor, logger)
//	execSvc := NewExecutionService(wfSvc, executor, parallelExec, store, logger, resolver, historySvc)
//
//	// Validate workflow before execution
//	if err := wfSvc.ValidateWorkflow(ctx, "deploy-app"); err != nil {
//	    log.Fatalf("validation failed: %v", err)
//	}
//
//	// Execute workflow with inputs
//	result, err := execSvc.ExecuteWorkflow(ctx, "deploy-app", map[string]any{
//	    "environment": "production",
//	    "version": "v1.2.3",
//	})
//	if err != nil {
//	    log.Fatalf("execution failed: %v", err)
//	}
//
// ## Configure Optional Features
//
// Enable specialized executors:
//
//	execSvc.SetOutputWriters(os.Stdout, os.Stderr)
//	execSvc.SetTemplateService(templateSvc)
//	execSvc.SetOperationProvider(pluginRegistry)
//	execSvc.SetAgentRegistry(agentRegistry)
//	execSvc.SetEvaluator(expressionEvaluator)
//	execSvc.SetConversationManager(conversationMgr)
//
// ## Interactive Mode Execution
//
// Run workflow with step-by-step control:
//
//	interactiveExec := NewInteractiveExecutor(prompter, stepExecutor, logger)
//	result, err := interactiveExec.ExecuteInteractive(ctx, workflow, inputs)
//
// ## Query Execution History
//
//	historySvc := NewHistoryService(historyStore, logger)
//
//	// Get recent executions
//	history, err := historySvc.GetExecutionHistory(ctx, &HistoryQuery{
//	    WorkflowName: "deploy-app",
//	    Limit: 10,
//	})
//
//	// Get aggregate statistics
//	stats, err := historySvc.GetExecutionStats(ctx, "deploy-app")
//	fmt.Printf("Success rate: %.1f%%\n", stats.SuccessRate*100)
//
// ## Manage Plugins
//
//	pluginSvc := NewPluginService(manager, stateStore, logger)
//
//	// Discover available plugins
//	plugins, err := pluginSvc.DiscoverPlugins(ctx)
//
//	// Enable plugin for use
//	if err := pluginSvc.EnablePlugin(ctx, "my-plugin"); err != nil {
//	    log.Fatalf("failed to enable plugin: %v", err)
//	}
//
// ## Expand Workflow Template
//
//	templateSvc := NewTemplateService(templateRepo, logger)
//
//	// Expand template reference
//	steps, err := templateSvc.ExpandTemplate(ctx, &workflow.WorkflowTemplateRef{
//	    Template: "deploy-steps",
//	    With: map[string]any{
//	        "environment": "staging",
//	    },
//	})
//
// ## Collect Missing Inputs
//
//	inputSvc := NewInputCollectionService(collector, logger)
//
//	// Prompt user for undefined inputs
//	inputs, err := inputSvc.CollectInputs(ctx, workflow, providedInputs)
//
// # Design Principles
//
// ## Dependency Injection
//
// All services use constructor injection:
//   - Port interfaces injected via NewXxxService constructors
//   - No global state or singletons
//   - Testable with mock implementations
//   - Optional dependencies via SetXxx methods
//
// ## Orchestration, Not Business Logic
//
// Services coordinate but don't contain rules:
//   - Validation logic lives in domain entities (Workflow.Validate)
//   - State transitions handled by domain (ExecutionContext)
//   - Services delegate to domain methods, don't duplicate logic
//
// ## Error Propagation
//
// Services wrap errors with context:
//   - Use fmt.Errorf with %w for error chains
//   - Preserve underlying error for error.Is checks
//   - Add operation context for debugging
//
// Example:
//
//	if err := s.repo.Load(ctx, name); err != nil {
//	    return fmt.Errorf("load workflow %s: %w", name, err)
//	}
//
// ## Context Propagation
//
// All operations accept context.Context:
//   - Enables cancellation and timeout control
//   - Graceful shutdown of long-running operations
//   - Pass context to all port calls
//
// ## Thread Safety
//
// Services are stateless and thread-safe:
//   - No mutable state fields (all dependencies are ports)
//   - Multiple goroutines can call service methods concurrently
//   - State mutations delegated to thread-safe ExecutionContext
//
// # Related Packages
//
//   - internal/domain/workflow: Core entities orchestrated by services
//   - internal/domain/ports: Port interfaces implemented by infrastructure
//   - internal/infrastructure: Concrete port implementations
//   - internal/interfaces/cli: CLI commands that invoke services
package application
