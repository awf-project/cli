# F022: Custom Operations

## Metadata
- **Statut**: backlog
- **Phase**: 4-Extensibility
- **Version**: v0.4.0
- **Priorité**: high
- **Estimation**: L

## Description

Allow defining custom operations beyond shell commands. Support built-in operations (http, file, transform) and plugin-provided operations. Operations encapsulate common tasks with typed inputs and outputs.

## Critères d'Acceptance

- [ ] Built-in HTTP operation (GET, POST, etc.)
- [ ] Built-in file operations (read, write, copy)
- [ ] Built-in transform operations (jq, jsonpath)
- [ ] Plugin-provided custom operations
- [ ] Typed operation inputs/outputs
- [ ] Operation discovery and help
- [ ] Error handling per operation

## Dépendances

- **Bloqué par**: F021
- **Débloque**: _none_

## Fichiers Impactés

```
internal/domain/operation/operation.go
internal/domain/operation/registry.go
internal/infrastructure/operations/http.go
internal/infrastructure/operations/file.go
internal/infrastructure/operations/transform.go
```

## Tâches Techniques

- [ ] Define Operation interface
  - [ ] Name() string
  - [ ] Execute(ctx, params) (Result, error)
  - [ ] Schema() InputSchema
- [ ] Define InputSchema for validation
- [ ] Implement HTTP operation
  - [ ] method, url, headers, body
  - [ ] Response capture (body, status, headers)
  - [ ] Timeout, retry
- [ ] Implement File operations
  - [ ] read: path → content
  - [ ] write: path, content → ok
  - [ ] copy: src, dest → ok
  - [ ] delete: path → ok
- [ ] Implement Transform operations
  - [ ] jq: input, query → result
  - [ ] jsonpath: input, path → result
  - [ ] regex: input, pattern, replace → result
- [ ] Register built-in operations
- [ ] Support `operation:` in state definition
- [ ] Write unit tests for each operation
- [ ] Write documentation

## Notes

Operation usage in workflow:
```yaml
fetch_data:
  type: step
  operation: http
  params:
    method: GET
    url: "https://api.example.com/data"
    headers:
      Authorization: "Bearer {{secrets.API_TOKEN}}"
    timeout: 30s
  capture:
    body: api_response
    status: http_status
  on_success: process

process:
  type: step
  operation: jq
  params:
    input: "{{states.fetch_data.output.body}}"
    query: ".items[] | select(.active == true)"
  capture:
    result: active_items
```

Operations provide type safety and reusability compared to raw shell commands.
