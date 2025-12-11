# Implementation Plan: F017 - Workflow Templates

## Summary

Implement reusable workflow templates with parameters following the existing hexagonal architecture patterns. Templates are YAML files that define parameterized step patterns, loaded by a new `TemplateRepository`, resolved by a new `TemplateService`, and expanded at workflow load time before execution. The implementation leverages existing stubs (`yamlTemplate`, `UseTemplate` field, error types) and follows patterns established by F016 (loops).

## ASCII Art Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           TEMPLATE RESOLUTION FLOW                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  1. YAML FILES                                                              │
│  ┌─────────────────────────┐     ┌─────────────────────────────────────┐   │
│  │  .awf/templates/        │     │  .awf/workflows/                    │   │
│  │  └─ ai-analyze.yaml     │     │  └─ code-review.yaml                │   │
│  │     name: ai-analyze    │     │     states:                         │   │
│  │     parameters:         │     │       analyze:                      │   │
│  │       - prompt (req)    │◄────│         use_template: ai-analyze    │   │
│  │       - model (def)     │     │         parameters:                 │   │
│  │     states:             │     │           prompt: "Review this"     │   │
│  │       analyze: ...      │     │         on_success: format          │   │
│  └─────────────────────────┘     └─────────────────────────────────────┘   │
│                                                                             │
│  2. LOAD TIME EXPANSION                                                     │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  WorkflowRepository.Load()                                          │   │
│  │       │                                                             │   │
│  │       ▼                                                             │   │
│  │  TemplateService.ExpandWorkflow(wf)                                 │   │
│  │       │                                                             │   │
│  │       ├──► For each step with use_template:                         │   │
│  │       │       │                                                     │   │
│  │       │       ├──► TemplateRepository.GetTemplate(name)             │   │
│  │       │       │                                                     │   │
│  │       │       ├──► ValidateParameters(template, step.Parameters)    │   │
│  │       │       │                                                     │   │
│  │       │       ├──► SubstituteParameters(template.States, params)    │   │
│  │       │       │                                                     │   │
│  │       │       └──► MergeIntoStep(step, expanded)                    │   │
│  │       │              - Copy Command, Dir, Timeout from template     │   │
│  │       │              - Preserve OnSuccess, OnFailure from workflow  │   │
│  │       │                                                             │   │
│  │       └──► Return expanded workflow                                 │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  3. EXECUTION (unchanged)                                                   │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  ExecutionService.Run(expandedWorkflow)                             │   │
│  │  - Steps execute normally                                           │   │
│  │  - Template origin is transparent                                   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Implementation Steps

### Step 1: Create Domain Entity

- **File:** `internal/domain/workflow/template.go`
- **Action:** CREATE
- **Changes:**
```go
package workflow

import (
    "errors"
    "fmt"
)

// Template represents a reusable workflow template.
type Template struct {
    Name       string
    Parameters []TemplateParam
    States     map[string]*Step // template-defined states
}

// TemplateParam defines a template parameter.
type TemplateParam struct {
    Name     string
    Required bool
    Default  any
}

// WorkflowTemplateRef references a template from a step.
type WorkflowTemplateRef struct {
    TemplateName string
    Parameters   map[string]any
}

// Validate checks if the template is valid.
func (t *Template) Validate() error {
    if t.Name == "" {
        return errors.New("template name is required")
    }
    if len(t.States) == 0 {
        return errors.New("template must define at least one state")
    }
    // Check for duplicate parameter names
    seen := make(map[string]bool)
    for _, p := range t.Parameters {
        if p.Name == "" {
            return errors.New("parameter name is required")
        }
        if seen[p.Name] {
            return fmt.Errorf("duplicate parameter name: %s", p.Name)
        }
        seen[p.Name] = true
    }
    return nil
}

// GetRequiredParams returns names of required parameters.
func (t *Template) GetRequiredParams() []string {
    var required []string
    for _, p := range t.Parameters {
        if p.Required {
            required = append(required, p.Name)
        }
    }
    return required
}

// GetDefaultValues returns a map of parameter defaults.
func (t *Template) GetDefaultValues() map[string]any {
    defaults := make(map[string]any)
    for _, p := range t.Parameters {
        if p.Default != nil {
            defaults[p.Name] = p.Default
        }
    }
    return defaults
}
```

### Step 2: Add Port Interface

- **File:** `internal/domain/ports/repository.go`
- **Action:** MODIFY
- **Changes:** Add after `WorkflowRepository`:
```go
// TemplateRepository defines the contract for loading workflow templates.
type TemplateRepository interface {
    GetTemplate(ctx context.Context, name string) (*workflow.Template, error)
    ListTemplates(ctx context.Context) ([]string, error)
    Exists(ctx context.Context, name string) bool
}
```

### Step 3: Implement Template Repository

- **File:** `internal/infrastructure/repository/template_repository.go`
- **Action:** CREATE
- **Changes:**
```go
package repository

import (
    "context"
    "os"
    "path/filepath"
    "strings"
    "sync"

    "gopkg.in/yaml.v3"
    "github.com/vanoix/awf/internal/domain/workflow"
)

// YAMLTemplateRepository loads templates from YAML files.
type YAMLTemplateRepository struct {
    searchPaths []string
    cache       map[string]*workflow.Template
    mu          sync.RWMutex
}

// NewYAMLTemplateRepository creates a template repository.
func NewYAMLTemplateRepository(searchPaths []string) *YAMLTemplateRepository {
    return &YAMLTemplateRepository{
        searchPaths: searchPaths,
        cache:       make(map[string]*workflow.Template),
    }
}

// GetTemplate loads a template by name.
func (r *YAMLTemplateRepository) GetTemplate(ctx context.Context, name string) (*workflow.Template, error) {
    // Check cache
    r.mu.RLock()
    if t, ok := r.cache[name]; ok {
        r.mu.RUnlock()
        return t, nil
    }
    r.mu.RUnlock()

    // Search for template file
    for _, dir := range r.searchPaths {
        path := filepath.Join(dir, name+".yaml")
        if _, err := os.Stat(path); err == nil {
            t, err := r.loadTemplate(path)
            if err != nil {
                return nil, err
            }
            // Cache it
            r.mu.Lock()
            r.cache[name] = t
            r.mu.Unlock()
            return t, nil
        }
    }

    return nil, &TemplateNotFoundError{TemplateName: name}
}

// ListTemplates returns all available template names.
func (r *YAMLTemplateRepository) ListTemplates(ctx context.Context) ([]string, error) {
    var names []string
    seen := make(map[string]bool)

    for _, dir := range r.searchPaths {
        entries, err := os.ReadDir(dir)
        if err != nil {
            continue // skip non-existent directories
        }
        for _, e := range entries {
            if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
                continue
            }
            name := strings.TrimSuffix(e.Name(), ".yaml")
            if !seen[name] {
                seen[name] = true
                names = append(names, name)
            }
        }
    }
    return names, nil
}

// Exists checks if a template exists.
func (r *YAMLTemplateRepository) Exists(ctx context.Context, name string) bool {
    for _, dir := range r.searchPaths {
        path := filepath.Join(dir, name+".yaml")
        if _, err := os.Stat(path); err == nil {
            return true
        }
    }
    return false
}

func (r *YAMLTemplateRepository) loadTemplate(path string) (*workflow.Template, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, WrapParseError(path, err)
    }

    var yt yamlTemplate
    if err := yaml.Unmarshal(data, &yt); err != nil {
        return nil, WrapParseError(path, err)
    }

    return mapTemplate(path, &yt)
}
```

### Step 4: Add Template Mapper

- **File:** `internal/infrastructure/repository/yaml_mapper.go`
- **Action:** MODIFY
- **Changes:** Add two functions:
```go
// mapTemplate converts yamlTemplate to domain Template.
func mapTemplate(filePath string, y *yamlTemplate) (*workflow.Template, error) {
    t := &workflow.Template{
        Name:       y.Name,
        Parameters: mapTemplateParams(y.Parameters),
        States:     make(map[string]*workflow.Step),
    }

    // Map states
    for name, step := range y.States.Steps {
        domainStep, err := mapStep(filePath, name, step)
        if err != nil {
            return nil, err
        }
        t.States[name] = domainStep
    }

    return t, nil
}

// mapTemplateParams converts yamlTemplateParam slice to domain.
func mapTemplateParams(params []yamlTemplateParam) []workflow.TemplateParam {
    result := make([]workflow.TemplateParam, len(params))
    for i, p := range params {
        result[i] = workflow.TemplateParam{
            Name:     p.Name,
            Required: p.Required,
            Default:  p.Default,
        }
    }
    return result
}

// mapTemplateRef converts use_template + parameters to WorkflowTemplateRef.
func mapTemplateRef(useTemplate string, parameters map[string]any) *workflow.WorkflowTemplateRef {
    if useTemplate == "" {
        return nil
    }
    return &workflow.WorkflowTemplateRef{
        TemplateName: useTemplate,
        Parameters:   parameters,
    }
}
```

Also modify `mapStep` to include template ref:
```go
// In mapStep(), add after existing field mappings:
step.TemplateRef = mapTemplateRef(y.UseTemplate, y.Parameters)
```

### Step 5: Add TemplateRef to Step

- **File:** `internal/domain/workflow/step.go`
- **Action:** MODIFY
- **Changes:** Add field to `Step` struct:
```go
type Step struct {
    // ... existing fields ...
    TemplateRef *WorkflowTemplateRef // template reference (for use_template steps)
}
```

### Step 6: Create Template Service

- **File:** `internal/application/template_service.go`
- **Action:** CREATE
- **Changes:**
```go
package application

import (
    "context"
    "fmt"
    "strings"

    "github.com/vanoix/awf/internal/domain/ports"
    "github.com/vanoix/awf/internal/domain/workflow"
    "github.com/vanoix/awf/internal/infrastructure/repository"
    "github.com/vanoix/awf/pkg/interpolation"
)

// TemplateService handles template resolution and expansion.
type TemplateService struct {
    repo     ports.TemplateRepository
    resolver interpolation.Resolver
    logger   ports.Logger
}

// NewTemplateService creates a new template service.
func NewTemplateService(
    repo ports.TemplateRepository,
    resolver interpolation.Resolver,
    logger ports.Logger,
) *TemplateService {
    return &TemplateService{
        repo:     repo,
        resolver: resolver,
        logger:   logger,
    }
}

// ExpandWorkflow resolves all template references in a workflow.
func (s *TemplateService) ExpandWorkflow(ctx context.Context, wf *workflow.Workflow) error {
    visited := make(map[string]bool)
    
    for name, step := range wf.Steps {
        if step.TemplateRef == nil {
            continue
        }
        
        if err := s.expandStep(ctx, wf, name, step, visited); err != nil {
            return err
        }
    }
    
    return nil
}

func (s *TemplateService) expandStep(
    ctx context.Context,
    wf *workflow.Workflow,
    stepName string,
    step *workflow.Step,
    visited map[string]bool,
) error {
    ref := step.TemplateRef
    
    // Circular reference detection
    if visited[ref.TemplateName] {
        chain := make([]string, 0, len(visited)+1)
        for k := range visited {
            chain = append(chain, k)
        }
        chain = append(chain, ref.TemplateName)
        return &repository.CircularTemplateError{Chain: chain}
    }
    visited[ref.TemplateName] = true
    defer delete(visited, ref.TemplateName)
    
    // Load template
    tmpl, err := s.repo.GetTemplate(ctx, ref.TemplateName)
    if err != nil {
        return err
    }
    
    // Validate and merge parameters
    params, err := s.mergeParameters(tmpl, ref.Parameters)
    if err != nil {
        return err
    }
    
    // Get the primary state from template (first one or named same as template)
    var templateStep *workflow.Step
    if ts, ok := tmpl.States[tmpl.Name]; ok {
        templateStep = ts
    } else {
        // Use first state
        for _, ts := range tmpl.States {
            templateStep = ts
            break
        }
    }
    
    if templateStep == nil {
        return fmt.Errorf("template %q has no states", tmpl.Name)
    }
    
    // Substitute parameters in template fields
    expandedCmd, err := s.substituteParams(templateStep.Command, params)
    if err != nil {
        return fmt.Errorf("expand command: %w", err)
    }
    
    expandedDir := templateStep.Dir
    if expandedDir != "" {
        expandedDir, err = s.substituteParams(expandedDir, params)
        if err != nil {
            return fmt.Errorf("expand dir: %w", err)
        }
    }
    
    // Merge template into step (step values take precedence for transitions)
    step.Type = templateStep.Type
    step.Command = expandedCmd
    if step.Dir == "" {
        step.Dir = expandedDir
    }
    if step.Timeout == 0 && templateStep.Timeout > 0 {
        step.Timeout = templateStep.Timeout
    }
    if step.Retry == nil && templateStep.Retry != nil {
        step.Retry = templateStep.Retry
    }
    if step.Capture == nil && templateStep.Capture != nil {
        step.Capture = templateStep.Capture
    }
    // OnSuccess/OnFailure from workflow step take precedence (already set)
    
    // Clear template ref after expansion
    step.TemplateRef = nil
    
    s.logger.Debug("expanded template",
        "step", stepName,
        "template", ref.TemplateName,
        "command", expandedCmd)
    
    return nil
}

func (s *TemplateService) mergeParameters(
    tmpl *workflow.Template,
    provided map[string]any,
) (map[string]any, error) {
    params := tmpl.GetDefaultValues()
    
    // Override with provided values
    for k, v := range provided {
        params[k] = v
    }
    
    // Check required parameters
    for _, name := range tmpl.GetRequiredParams() {
        if _, ok := params[name]; !ok {
            return nil, &repository.MissingParameterError{
                TemplateName:  tmpl.Name,
                ParameterName: name,
                Required:      tmpl.GetRequiredParams(),
            }
        }
    }
    
    return params, nil
}

func (s *TemplateService) substituteParams(template string, params map[string]any) (string, error) {
    // Replace {{parameters.X}} with values
    result := template
    for name, value := range params {
        placeholder := fmt.Sprintf("{{parameters.%s}}", name)
        result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
    }
    return result, nil
}

// ValidateTemplateRef validates a template reference without expanding.
func (s *TemplateService) ValidateTemplateRef(ctx context.Context, ref *workflow.WorkflowTemplateRef) error {
    tmpl, err := s.repo.GetTemplate(ctx, ref.TemplateName)
    if err != nil {
        return err
    }
    
    _, err = s.mergeParameters(tmpl, ref.Parameters)
    return err
}
```

### Step 7: Wire in CLI Layer

- **File:** `internal/interfaces/cli/run.go`
- **Action:** MODIFY
- **Changes:** In `runWorkflow()`, after creating `wfSvc`, add:
```go
// Create template service
templatePaths := []string{
    ".awf/templates",
    filepath.Join(cfg.StoragePath, "templates"),
}
templateRepo := repository.NewYAMLTemplateRepository(templatePaths)
templateSvc := application.NewTemplateService(templateRepo, resolver, logger)

// ... after loading workflow but before execution ...
// Expand templates in workflow
wf, err := wfSvc.GetWorkflow(ctx, workflowName)
if err != nil {
    return err
}
if err := templateSvc.ExpandWorkflow(ctx, wf); err != nil {
    return fmt.Errorf("expand templates: %w", err)
}
```

### Step 8: Add Template Validation to Validate Command

- **File:** `internal/interfaces/cli/validate.go`
- **Action:** MODIFY
- **Changes:** Add template reference validation in `runValidate()`:
```go
// After workflow validation, check template refs
for name, step := range wf.Steps {
    if step.TemplateRef != nil {
        if err := templateSvc.ValidateTemplateRef(ctx, step.TemplateRef); err != nil {
            return fmt.Errorf("step %q: %w", name, err)
        }
    }
}
```

## Test Plan

### Unit Tests

| File | Test Cases |
|------|------------|
| `internal/domain/workflow/template_test.go` | `TestTemplate_Validate`, `TestTemplate_GetRequiredParams`, `TestTemplate_GetDefaultValues` |
| `internal/infrastructure/repository/template_repository_test.go` | `TestGetTemplate_Success`, `TestGetTemplate_NotFound`, `TestGetTemplate_Cache`, `TestListTemplates` |
| `internal/application/template_service_test.go` | `TestExpandWorkflow_Simple`, `TestExpandWorkflow_MissingParam`, `TestExpandWorkflow_CircularRef`, `TestMergeParameters`, `TestSubstituteParams` |

### Integration Tests

| File | Test Cases |
|------|------------|
| `tests/integration/template_test.go` | Full workflow with template execution, nested template validation, parameter override |

### Test Fixtures

```
tests/fixtures/templates/
├── simple-echo.yaml      # Basic template: one required param
├── ai-analyze.yaml       # From F017 spec: prompt(req), model(def), timeout(def)
└── circular-a.yaml       # For circular detection test
```

**Example `simple-echo.yaml`:**
```yaml
name: simple-echo
parameters:
  - name: message
    required: true
  - name: prefix
    default: "[INFO]"
states:
  echo:
    type: command
    command: "echo '{{parameters.prefix}} {{parameters.message}}'"
```

## Files to Modify

| File | Action | Complexity | LOC Est. |
|------|--------|------------|----------|
| `internal/domain/workflow/template.go` | CREATE | M | ~80 |
| `internal/domain/workflow/step.go` | MODIFY | S | +1 |
| `internal/domain/ports/repository.go` | MODIFY | S | +6 |
| `internal/infrastructure/repository/template_repository.go` | CREATE | M | ~100 |
| `internal/infrastructure/repository/yaml_mapper.go` | MODIFY | S | +30 |
| `internal/application/template_service.go` | CREATE | M | ~150 |
| `internal/interfaces/cli/run.go` | MODIFY | S | +15 |
| `internal/interfaces/cli/validate.go` | MODIFY | S | +10 |
| Tests (3 files) | CREATE | M | ~200 |

**Total: ~600 LOC**

## Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| **Circular template refs** | High | `visited` map in `expandStep()`, error type already exists |
| **Parameter substitution before other interpolation** | Medium | Templates expand at load time, `{{inputs.*}}` resolved at runtime - no conflict |
| **Template file not found at runtime** | Medium | Validate during `awf validate` command |
| **Breaking change to Step struct** | Low | Adding `TemplateRef` field is additive, nil by default |
| **Cache invalidation** | Low | Simple cache, restart required for template changes (acceptable for v1) |

## Implementation Order

1. **Domain** (Step 1, 5): `template.go`, add `TemplateRef` to `step.go`
2. **Ports** (Step 2): Add `TemplateRepository` interface
3. **Infrastructure** (Step 3, 4): `template_repository.go`, update `yaml_mapper.go`
4. **Application** (Step 6): `template_service.go`
5. **CLI** (Step 7, 8): Wire in `run.go`, update `validate.go`
6. **Tests**: Unit tests, integration tests

