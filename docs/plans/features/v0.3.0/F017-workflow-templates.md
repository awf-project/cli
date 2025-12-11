# F017: Templates de Workflows

## Metadata
- **Status**: implemented
- **Phase**: 3-Advanced
- **Version**: v0.3.0
- **Priority**: medium
- **Estimation**: M

## Description

Support reusable workflow templates with parameters. Define common patterns once and instantiate with different configurations. Enable workflow composition and reduce duplication across workflow definitions.

## Acceptance Criteria

- [x] Define templates with parameters
- [x] Instantiate templates in workflows
- [x] Template parameters override defaults
- [x] Templates can include other templates
- [x] Template validation at load time
- [x] Clear error for missing parameters

## Dependencies

- **Blocked by**: F002, F007
- **Unblocks**: F023

## Impacted Files

```
internal/infrastructure/repository/template_repository.go
internal/domain/workflow/template.go
internal/application/template_resolver.go
configs/workflows/templates/
```

## Technical Tasks

- [x] Define Template struct
  - [x] Name
  - [x] Parameters (with defaults)
  - [x] States
- [x] Define TemplateReference struct
  - [x] Template name
  - [x] Parameter values
- [x] Implement TemplateRepository
  - [x] Load templates from templates/ dir
  - [x] Cache loaded templates
- [x] Implement TemplateResolver
  - [x] Resolve template references
  - [x] Substitute parameters
  - [x] Merge into workflow
- [x] Support `use_template:` in state definitions
- [x] Handle template inheritance
- [x] Detect circular template references
- [x] Write unit tests
- [x] Write integration tests

## Notes

Template definition:
```yaml
# configs/workflows/templates/ai-analyze.yaml
name: ai-analyze
parameters:
  - name: prompt
    required: true
  - name: model
    default: claude
  - name: timeout
    default: 120s
states:
  analyze:
    type: step
    command: "{{parameters.model}} -c '{{parameters.prompt}}'"
    timeout: "{{parameters.timeout}}"
    capture:
      stdout: analysis
```

Template usage:
```yaml
states:
  code_analysis:
    use_template: ai-analyze
    parameters:
      prompt: "Analyze this code: {{states.extract.output}}"
      model: gemini
    on_success: format
```
