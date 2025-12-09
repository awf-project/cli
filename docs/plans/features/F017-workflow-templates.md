# F017: Templates de Workflows

## Metadata
- **Statut**: backlog
- **Phase**: 3-Advanced
- **Version**: v0.3.0
- **Priorité**: medium
- **Estimation**: M

## Description

Support reusable workflow templates with parameters. Define common patterns once and instantiate with different configurations. Enable workflow composition and reduce duplication across workflow definitions.

## Critères d'Acceptance

- [ ] Define templates with parameters
- [ ] Instantiate templates in workflows
- [ ] Template parameters override defaults
- [ ] Templates can include other templates
- [ ] Template validation at load time
- [ ] Clear error for missing parameters

## Dépendances

- **Bloqué par**: F002, F007
- **Débloque**: F023

## Fichiers Impactés

```
internal/infrastructure/repository/template_repository.go
internal/domain/workflow/template.go
internal/application/template_resolver.go
configs/workflows/templates/
```

## Tâches Techniques

- [ ] Define Template struct
  - [ ] Name
  - [ ] Parameters (with defaults)
  - [ ] States
- [ ] Define TemplateReference struct
  - [ ] Template name
  - [ ] Parameter values
- [ ] Implement TemplateRepository
  - [ ] Load templates from templates/ dir
  - [ ] Cache loaded templates
- [ ] Implement TemplateResolver
  - [ ] Resolve template references
  - [ ] Substitute parameters
  - [ ] Merge into workflow
- [ ] Support `use_template:` in state definitions
- [ ] Handle template inheritance
- [ ] Detect circular template references
- [ ] Write unit tests
- [ ] Write integration tests

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
