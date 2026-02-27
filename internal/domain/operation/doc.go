// Package operation provides the domain interface and registry for executable operations.
//
// This package defines the Operation interface that all operations (HTTP, file I/O, transforms)
// must implement, along with the OperationRegistry for managing operation lifecycle and discovery.
// It follows hexagonal architecture principles with zero infrastructure dependencies.
//
// # Architecture Role
//
// In the hexagonal architecture pattern:
//   - Domain layer defines Operation interface and OperationRegistry (this package)
//   - Infrastructure layer implements concrete operations (HTTP, file, transform adapters)
//   - Application layer orchestrates operation execution through OperationRegistry
//   - Domain layer depends on nothing; all dependencies point inward
//
// This package bridges the workflow execution system with plugin-contributed operations,
// enabling dynamic operation registration without coupling to specific implementations.
//
// # Core Types
//
// ## Operation Interface (operation.go)
//
// Interface for executable operations:
//   - Name(): Returns unique operation identifier (e.g., "http.get", "file.read")
//   - Execute(ctx, inputs): Executes operation with typed inputs, returns OperationResult
//   - Schema(): Returns OperationSchema metadata for validation and discovery
//
// All operations must implement this interface to integrate with workflow execution.
//
// ## OperationRegistry (registry.go)
//
// Registry for operation lifecycle management:
//   - Register(Operation): Add operation to registry, reject duplicates
//   - Unregister(name): Remove operation from registry
//   - Get(name): Retrieve operation by name (returns bool for not-found)
//   - List(): Enumerate all registered operations
//
// Registry implements ports.OperationProvider for seamless ExecutionService integration.
//
// ## Reused Domain Types
//
// Operation metadata types from internal/domain/pluginmodel:
//   - OperationSchema: Name, description, inputs, outputs, plugin name
//   - InputSchema: Type, required flag, default value, description, validation rules
//   - OperationResult: Success flag, outputs map, error message
//
// # Operation Execution Flow
//
// 1. Workflow step references operation by name (e.g., "operation: http.get")
// 2. ExecutionService calls OperationRegistry.Get("http.get")
// 3. Registry returns Operation interface implementation
// 4. ExecutionService calls Operation.Schema() for input validation
// 5. InputSchema.ValidateInputs() validates inputs, applies defaults
// 6. ExecutionService calls Operation.Execute(ctx, validatedInputs)
// 7. Operation returns OperationResult with success flag and outputs
//
// # Input Validation
//
// Operations validate inputs against their InputSchema:
//   - Required field checking (missing required inputs = validation error)
//   - Type validation (string, integer, boolean, array, object)
//   - Default value application (omitted optional inputs get defaults)
//   - Validation rule enforcement (url, email, pattern matching)
//
// Validation happens before execution to provide early, clear error messages.
//
// # Usage Examples
//
// ## Implementing an Operation
//
// Create a custom operation:
//
//	type FileReadOperation struct{}
//
//	func (f *FileReadOperation) Name() string {
//	    return "file.read"
//	}
//
//	func (f *FileReadOperation) Execute(ctx context.Context, inputs map[string]any) (*plugin.OperationResult, error) {
//	    path, _ := inputs["path"].(string)
//	    content, err := os.ReadFile(path)
//	    if err != nil {
//	        return &plugin.OperationResult{Success: false, Error: err.Error()}, nil
//	    }
//	    return &plugin.OperationResult{
//	        Success: true,
//	        Outputs: map[string]any{"content": string(content)},
//	    }, nil
//	}
//
//	func (f *FileReadOperation) Schema() *plugin.OperationSchema {
//	    return &plugin.OperationSchema{
//	        Name:        "file.read",
//	        Description: "Read file contents",
//	        Inputs: map[string]plugin.InputSchema{
//	            "path": {Type: plugin.InputTypeString, Required: true, Description: "File path"},
//	        },
//	        Outputs: []string{"content"},
//	    }
//	}
//
// ## Registering Operations
//
// Add operations to registry:
//
//	registry := operation.NewOperationRegistry()
//
//	// Register file operation
//	fileOp := &FileReadOperation{}
//	if err := registry.Register(fileOp); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Register HTTP operation
//	httpOp := &HTTPGetOperation{}
//	if err := registry.Register(httpOp); err != nil {
//	    log.Fatal(err)
//	}
//
// ## Discovering Operations
//
// List available operations:
//
//	operations := registry.List()
//	for _, op := range operations {
//	    schema := op.Schema()
//	    fmt.Printf("Operation: %s - %s\n", schema.Name, schema.Description)
//	    for name, input := range schema.Inputs {
//	        fmt.Printf("  Input: %s (%s) - %s\n", name, input.Type, input.Description)
//	    }
//	}
//
// ## Executing Operations
//
// Execute by name:
//
//	op, found := registry.Get("file.read")
//	if !found {
//	    log.Fatal("operation not found")
//	}
//
//	// Validate inputs
//	inputs := map[string]any{"path": "/etc/hosts"}
//	if err := op.Schema().ValidateInputs(inputs); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Execute with context
//	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//	defer cancel()
//
//	result, err := op.Execute(ctx, inputs)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if !result.Success {
//	    log.Fatalf("operation failed: %s", result.Error)
//	}
//	fmt.Println(result.Outputs["content"])
//
// ## Workflow Integration
//
// Use operations in workflow YAML:
//
//	name: file-processing
//	version: 1.0.0
//	initial: read_file
//
//	steps:
//	  read_file:
//	    type: operation
//	    operation: file.read
//	    inputs:
//	      path: "{{inputs.file_path}}"
//	    on_success: process
//
//	  process:
//	    type: operation
//	    operation: transform.jq
//	    inputs:
//	      data: "{{states.read_file.outputs.content}}"
//	      query: ".name"
//	    on_success: end
//
//	  end:
//	    type: terminal
//	    status: success
//
// # Design Principles
//
// ## Interface Segregation
//
// Operation interface is minimal and focused:
//   - Three methods only: Name, Execute, Schema
//   - No plugin-specific dependencies
//   - Enables testing with mock implementations
//   - Supports diverse operation types (network, I/O, data processing)
//
// ## Registry as Singleton
//
// OperationRegistry acts as central catalog:
//   - Single source of truth for available operations
//   - Thread-safe for concurrent reads
//   - Mutex-protected for registration mutations
//   - No global state - injected via dependency injection
//
// ## Pure Domain
//
// Zero infrastructure dependencies:
//   - No file I/O, HTTP, or external systems in this package
//   - Concrete operations implemented in infrastructure layer
//   - Domain types (OperationSchema, InputSchema) reused from plugin package
//
// ## Context Propagation
//
// Operations respect context for cancellation:
//   - Execute receives context.Context parameter
//   - Operations check ctx.Done() for cancellation signals
//   - Timeout enforcement delegated to caller (ExecutionService)
//
// # Related Packages
//
//   - internal/domain/pluginmodel: OperationSchema, InputSchema, OperationResult types
//   - internal/domain/ports: OperationProvider port interface
//   - internal/infrastructure/pluginmgr: Concrete operation implementations
//   - internal/application: ExecutionService orchestrating operation execution
package operation
