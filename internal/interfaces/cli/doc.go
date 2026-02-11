// Package cli provides the Cobra-based command-line interface for AWF.
//
// The CLI layer is the primary user-facing entry point, translating command-line
// invocations into application service calls. All commands follow the hexagonal
// architecture pattern: CLI → Application Services → Domain → Infrastructure.
// The package handles user input collection, output formatting, signal handling,
// and exit code mapping according to the AWF error taxonomy.
//
// # Architecture Role
//
// In the hexagonal architecture:
//   - CLI commands receive user input from cobra.Command flags and arguments
//   - Commands delegate business logic to application services (WorkflowService, ExecutionService)
//   - Commands handle cross-cutting concerns: signal handling, exit codes, output formatting
//   - Commands inject infrastructure adapters (repositories, loggers, executors) into services
//
// The CLI layer is an adapter in the "interfaces" layer, depending on both application
// services (orchestration) and infrastructure adapters (implementation details). No domain
// logic resides here—only input parsing, output rendering, and coordination.
//
// # Command Structure
//
// ## Root Command (root.go)
//
// Application container and version command:
//   - App: Dependency injection container with Config and Formatter
//   - NewApp: Creates application with loaded configuration
//   - NewRootCommand: Builds cobra command tree with global flags
//   - newVersionCommand: Displays AWF version, commit, and build date
//
// Global flags:
//   - --no-color: Disable colorized output
//   - --json: JSON-formatted output
//   - --no-hints: Suppress helpful tips
//
// ## Core Commands
//
// ### run (run.go)
//
// Execute workflow with state machine traversal:
//   - Flags: --input, --resume, --agent-input, --mock, --dry-run, --interactive, --no-save
//   - Input resolution: CLI flags → project config → interactive prompts
//   - Dry run mode: validates without execution, renders execution plan
//   - Interactive mode: step-by-step execution with user confirmation
//   - Resume support: continues from saved checkpoint state
//   - Error categorization: maps domain/infra errors to exit codes (1-4)
//
// ### list (list.go)
//
// Enumerate available workflows and prompts:
//   - list: Display all workflows from configured paths
//   - list prompts: Show available interactive prompts with descriptions
//   - Output format: table (default) or JSON
//
// ### validate (validate.go)
//
// Static workflow validation:
//   - Parse workflow YAML
//   - Validate structure, state references, transitions
//   - Check for cycles, unreachable states
//   - Exit 0 on success, 2 on validation errors
//
// ### status (status.go)
//
// Check running workflow status:
//   - Query execution state from StateStore
//   - Display current step, progress, outputs
//   - Show execution timeline and duration
//
// ### resume (resume.go)
//
// Continue interrupted workflow:
//   - Load checkpoint state from StateStore
//   - Resume execution from last completed step
//   - Preserve input values and outputs
//
// ### history (history.go)
//
// Query workflow execution history:
//   - List past executions with filtering (workflow name, status, date range)
//   - Show execution statistics (duration, success rate)
//   - Prune old history records
//
// ## Supporting Commands
//
// ### init (init.go)
//
// Initialize AWF in current directory:
//   - Create .awf/ directory structure
//   - Generate default awf.yaml configuration
//   - Create workflows/ directory with example
//
// ### config (config.go, config_cmd.go)
//
// Configuration management:
//   - config show: Display current configuration
//   - config set <key> <value>: Update configuration value
//   - config paths: Show search paths for workflows/templates/plugins
//
// ### plugin (plugin_cmd.go, plugins.go)
//
// Plugin lifecycle management:
//   - plugin list: Show available plugins
//   - plugin enable <name>: Activate plugin
//   - plugin disable <name>: Deactivate plugin
//   - plugin status <name>: Check plugin state
//
// ### diagram (diagram.go)
//
// Generate workflow visualization:
//   - Render workflow state machine as Mermaid diagram
//   - Output to stdout or file
//
// ### migration (migration.go)
//
// Version migration utilities:
//   - Migrate workflow syntax to latest version
//   - Update deprecated fields
//
// # Signal Handling (signal_handler.go)
//
// Graceful shutdown on SIGINT/SIGTERM:
//   - setupSignalHandler: Starts goroutine listening for OS signals
//   - Context cancellation: Propagates cancellation through execution stack
//   - Process group cleanup: Terminates child processes on signal
//   - Cleanup callback: Executes user-provided cleanup function before exit
//   - Goroutine leak prevention: Returns cleanup function that MUST be deferred
//
// # Exit Codes (exitcodes.go)
//
// AWF error taxonomy mapping:
//   - ExitSuccess (0): Successful execution
//   - ExitUser (1): User error (bad input, missing file, invalid flags)
//   - ExitWorkflow (2): Workflow error (invalid state reference, cycle detection, schema violation)
//   - ExitExecution (3): Execution error (command failed, timeout, agent error)
//   - ExitSystem (4): System error (IO failure, permissions, resource exhaustion)
//
// # Error Handling (error.go)
//
// Error categorization and formatting:
//   - exitError: Wraps domain/infra errors with exit code
//   - categorizeError: Maps error types to exit codes via taxonomy
//   - writeErrorAndExit: Formats error, writes to stderr, exits with correct code
//   - formatLog: Colorized log output with level-based styling
//
// # Design Principles
//
//   - Thin adapter layer: All business logic in application/domain layers
//   - Fail fast: Validate inputs before service calls
//   - User-friendly errors: Context-rich messages with actionable guidance
//   - Testability: Commands accept io.Writer for output, injectable services
//   - Signal safety: Always defer signal handler cleanup to prevent leaks
//   - Exit code discipline: Consistent taxonomy mapping for automation/scripting
package cli
