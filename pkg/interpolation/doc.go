// Package interpolation provides template variable interpolation for AWF workflows.
//
// This package implements the public API for resolving template variables
// ({{inputs.name}}, {{states.step.output}}, etc.) in workflow definitions.
// It supports seven namespaces: inputs, states, workflow, env, error, context, and loop.
//
// Key features:
//   - Template resolution using Go text/template engine with custom functions
//   - Reference extraction and validation against known properties
//   - Shell escaping for safe command interpolation
//   - Type-safe context management with structured data models
//
// # Core Components
//
// ## Resolver (resolver.go, template_resolver.go)
//
// Resolver interface defines the contract for variable interpolation:
//   - TemplateResolver: Implements Resolver using Go text/template
//   - Custom functions: base64, json, yaml, upper, lower, title, trim, split, join, contains, hasPrefix, hasSuffix
//   - Escaping modes: ShellEscape for commands, NoEscape for non-shell contexts
//
// ## Reference Parsing (reference.go)
//
// Reference types and extraction:
//   - ExtractReferences: Parse template string for all {{...}} patterns
//   - ParseReference: Parse single reference path into structured Reference
//   - ReferenceType: Categorize namespace (inputs, states, workflow, env, error, context, loop, unknown)
//   - Validation maps: ValidWorkflowProperties, ValidStateProperties, ValidErrorProperties, ValidContextProperties
//
// ## Context (resolver.go)
//
// Context provides all variable namespaces for interpolation:
//   - Context: Root context with all namespaces
//   - StepStateData: Step execution results (output, stderr, exit_code, status, response, tokens_used)
//   - WorkflowData: Workflow metadata (id, name, current_state, started_at, duration)
//   - LoopData: Loop iteration state (item, index, first, last, length, parent)
//   - ErrorData: Error information for error hooks (message, state, exit_code, type)
//   - ContextData: Runtime context (working_dir, user, hostname)
//
// ## Escaping (escaping.go)
//
// Shell safety functions:
//   - ShellEscape: Wrap in single quotes and escape embedded quotes
//   - NoEscape: Return string unchanged for non-shell contexts
//
// ## Serialization (serializer.go)
//
// Convert values for interpolation:
//   - Serialize: Convert any value to string representation
//   - JSON encoding for complex types
//   - Type-specific formatting for primitives
//
// # Template Variable Syntax
//
// Template interpolation uses {{var}} syntax (Go template style):
//
// ## Inputs Namespace
//
//	{{inputs.name}}           # input parameter
//	{{inputs.config.port}}    # nested input (if input is map)
//
// ## States Namespace
//
//	{{states.step_name.Output}}    # step output (PascalCase property)
//	{{states.step_name.Stderr}}    # stderr output
//	{{states.step_name.ExitCode}}  # exit code
//	{{states.step_name.Status}}    # execution status
//	{{states.step_name.Response}}  # parsed JSON response (agent steps)
//	{{states.step_name.TokensUsed}}    # total tokens used (agent steps)
//
// ## Workflow Namespace
//
//	{{workflow.ID}}           # workflow instance ID
//	{{workflow.Name}}         # workflow name
//	{{workflow.CurrentState}} # current step name
//	{{workflow.StartedAt}}    # start timestamp
//	{{workflow.Duration}}     # formatted duration
//
// ## Environment Namespace
//
//	{{env.VAR_NAME}}          # environment variable
//
// ## Error Namespace (error hooks only)
//
//	{{error.Message}}         # error message
//	{{error.State}}           # step where error occurred
//	{{error.ExitCode}}        # exit code
//	{{error.Type}}            # error type
//
// ## Context Namespace
//
//	{{context.WorkingDir}}    # current working directory
//	{{context.User}}          # username
//	{{context.Hostname}}      # hostname
//
// ## Loop Namespace (inside loops only)
//
//	{{loop.Item}}             # current item value
//	{{loop.Index}}            # 0-based iteration index
//	{{loop.Index1}}           # 1-based iteration index
//	{{loop.First}}            # true on first iteration
//	{{loop.Last}}             # true on last iteration (for_each only)
//	{{loop.Length}}           # total items count (for_each only, -1 for while)
//	{{loop.Parent.Index}}     # nested loop access (F043)
//
// # Usage Examples
//
// ## Basic Interpolation
//
// Create a context and resolve template:
//
//	ctx := interpolation.NewContext()
//	ctx.Inputs["name"] = "Alice"
//	ctx.Workflow.Name = "hello-world"
//
//	resolver := NewTemplateResolver()
//	result, err := resolver.Resolve("Hello, {{inputs.name}} from {{workflow.Name}}", ctx)
//	// result: "Hello, Alice from hello-world"
//
// ## Step State Access
//
// Access previous step outputs:
//
//	ctx.States["fetch_data"] = interpolation.StepStateData{
//	    Output:   "/tmp/data.json",
//	    ExitCode: 0,
//	    Status:   "completed",
//	}
//
//	resolver.Resolve("Process file: {{states.fetch_data.Output}}", ctx)
//	// result: "Process file: /tmp/data.json"
//
// ## Loop Context
//
// Access loop iteration state:
//
//	ctx.Loop = &interpolation.LoopData{
//	    Item:   "apple",
//	    Index:  0,
//	    First:  true,
//	    Last:   false,
//	    Length: 3,
//	}
//
//	resolver.Resolve("Item {{loop.Index1}}/{{loop.Length}}: {{loop.Item}}", ctx)
//	// result: "Item 1/3: apple"
//
// ## Shell Escaping
//
// Safe interpolation for shell commands:
//
//	userInput := "file with spaces.txt"
//	escaped := interpolation.ShellEscape(userInput)
//	// escaped: "'file with spaces.txt'"
//
//	unsafeInput := "'; rm -rf /"
//	escaped = interpolation.ShellEscape(unsafeInput)
//	// escaped: "''\''; rm -rf /'"
//
// ## Reference Extraction
//
// Extract all template references for validation:
//
//	refs, err := interpolation.ExtractReferences("Run {{inputs.cmd}} on {{workflow.Name}}")
//	// refs[0]: {Type: TypeInputs, Path: "cmd", Raw: "{{inputs.cmd}}"}
//	// refs[1]: {Type: TypeWorkflow, Path: "Name", Raw: "{{workflow.Name}}"}
//
//	// Check if reference is valid
//	for _, ref := range refs {
//	    if ref.Type == interpolation.TypeWorkflow {
//	        if !interpolation.ValidWorkflowProperties[ref.Path] {
//	            // invalid workflow property
//	        }
//	    }
//	}
//
// ## Custom Template Functions
//
// Use built-in template functions:
//
//	ctx.Inputs["data"] = "hello world"
//	resolver.Resolve("{{upper inputs.data}}", ctx)
//	// result: "HELLO WORLD"
//
//	ctx.Inputs["json_str"] = `{"key":"value"}`
//	resolver.Resolve("{{json inputs.json_str}}", ctx)
//	// result: {"key":"value"} (formatted JSON)
//
// # Security Considerations
//
// ## Shell Injection Prevention
//
// Always escape user-provided values in shell commands:
//   - Use ShellEscape for inputs, state outputs, and env vars
//   - Wrap values in single quotes and escape embedded quotes
//   - Template resolver uses ShellEscape by default for command steps
//
// ## Property Validation
//
// Validate references before execution:
//   - ExtractReferences parses all {{...}} patterns
//   - ValidWorkflowProperties, ValidStateProperties, ValidErrorProperties, ValidContextProperties maps
//   - Validation errors at workflow load time, not runtime
//
// # Property Name Casing (F050)
//
// PascalCase properties (uppercase first letter):
//   - states.step.Output, states.step.Stderr, states.step.ExitCode, states.step.Status, states.step.Response, states.step.TokensUsed
//   - workflow.ID, workflow.Name, workflow.CurrentState, workflow.StartedAt, workflow.Duration
//   - error.Message, error.State, error.ExitCode, error.Type
//   - context.WorkingDir, context.User, context.Hostname
//   - loop.Item, loop.Index, loop.Index1, loop.First, loop.Last, loop.Length, loop.Parent
//
// Lowercase properties still accepted with warnings (F050 migration path):
//   - states.step.output -> states.step.Output
//   - workflow.name -> workflow.Name
//
// # Design Principles
//
// ## Public API Surface
//
// This is a public package (pkg/) for external consumers:
//   - Stable API with semantic versioning
//   - Clean separation between resolver interface and implementation
//   - No internal/ package dependencies
//
// ## Go Template Engine
//
// Uses text/template for powerful interpolation:
//   - Dot notation: {{.states.step.output}}
//   - Custom functions: base64, json, upper, lower, split, etc.
//   - Conditional logic: {{if .inputs.debug}}...{{end}}
//   - Loops: {{range .inputs.items}}...{{end}}
//
// ## Thread Safety
//
// Context is not thread-safe by design:
//   - Create separate Context instances for concurrent interpolation
//   - TemplateResolver is stateless and safe for concurrent use
//   - Shell escaping functions are pure and thread-safe
//
// # Related Documentation
//
// See also:
//   - pkg/expression: Boolean expression evaluation using interpolation context
//   - pkg/validation: Input validation before interpolation
//   - internal/infrastructure/expression: Infrastructure adapter implementing expression evaluation
//   - docs/template-syntax.md: Complete template syntax reference
//   - CLAUDE.md: Project conventions and template interpolation semantics
package interpolation
