// Package repository provides infrastructure adapters for workflow and template persistence.
//
// The repository layer implements the WorkflowRepository and TemplateRepository ports from
// the domain layer, handling YAML file parsing, workflow/template loading, and the mapping
// between YAML structures and domain entities. This package bridges the domain's abstract
// persistence contracts with concrete filesystem-based storage.
//
// # Architecture Role
//
// In the hexagonal architecture:
//   - Implements ports: WorkflowRepository (domain/ports), TemplateRepository (domain/ports)
//   - Consumed by: Application services (WorkflowService, TemplateService)
//   - Depends on: YAML parsing (gopkg.in/yaml.v3), filesystem (os)
//
// The repository layer isolates YAML syntax details from domain logic, enabling the domain
// to remain persistence-agnostic. YAML mapping functions translate between yamlWorkflow and
// workflow.Workflow entities, enforcing domain invariants during deserialization.
//
// # Key Types
//
// ## YAMLRepository (yaml_repository.go)
//
// Loads workflow definitions from YAML files in a configured base directory.
//   - Load: Parse and map YAML file to domain Workflow entity
//   - List: Enumerate available workflows
//   - Exists: Check workflow file existence
//
// Handles error wrapping (parse errors, missing files) and enforces domain validation
// during mapping.
//
// ## YAMLTemplateRepository (template_repository.go)
//
// Loads workflow templates from YAML files with in-memory caching.
//   - GetTemplate: Load and cache template by name
//   - ListTemplates: Enumerate available templates from search paths
//   - Exists: Check template availability across multiple directories
//
// Supports multiple search paths with cache invalidation and concurrent access via RWMutex.
//
// ## CompositeRepository (composite_repository.go)
//
// Aggregates multiple YAMLRepository instances with priority-based resolution.
// Useful for layered workflow directories (user workflows override system workflows).
//   - Load: Search repositories in priority order, return first match
//   - List: Merge workflow lists from all repositories (priority order)
//   - Exists: Check across all aggregated repositories
//
// ## YAML Mapping Layer (yaml_mapper.go, yaml_types.go)
//
// Translates between YAML structures (yamlWorkflow, yamlStep, yamlInput) and domain
// entities (workflow.Workflow, workflow.Step, workflow.Input).
//   - mapToDomain: Convert yamlWorkflow to domain Workflow
//   - mapStep: Convert yamlStep to domain Step (dispatches by step type)
//   - mapInputs, mapTransitions, mapWorkflowHooks: Field-level conversions
//
// Enforces domain rules (required fields, valid transitions) during deserialization.
//
// # Usage Example
//
//	// Single directory repository
//	repo := repository.NewYAMLRepository("./configs/workflows")
//	wf, err := repo.Load(ctx, "deploy")
//
//	// Multi-directory composite with priority
//	paths := []repository.SourcedPath{
//	    {Path: "~/.awf/workflows", Source: repository.SourceUser},
//	    {Path: "./workflows", Source: repository.SourceProject},
//	    {Path: "/usr/share/awf/workflows", Source: repository.SourceSystem},
//	}
//	composite := repository.NewCompositeRepository(paths)
//	wf, err := composite.Load(ctx, "deploy") // User workflows override system
//
//	// Template repository with caching
//	templateRepo := repository.NewYAMLTemplateRepository([]string{"./templates"})
//	tmpl, err := templateRepo.GetTemplate(ctx, "http-request")
package repository
