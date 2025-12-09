# F025: API REST

## Metadata
- **Statut**: backlog
- **Phase**: 5-Interfaces
- **Version**: v1.0.0
- **Priorité**: high
- **Estimation**: XL

## Description

Expose AWF functionality via REST API. Allow remote workflow execution, status monitoring, and management. Enable integration with other systems and build web interfaces. Implements the hexagonal architecture's API adapter.

## Critères d'Acceptance

- [ ] Start API server: `awf server`
- [ ] POST /workflows/{name}/run - execute workflow
- [ ] GET /executions/{id} - get execution status
- [ ] GET /executions/{id}/logs - stream logs
- [ ] GET /workflows - list workflows
- [ ] DELETE /executions/{id} - cancel execution
- [ ] Authentication (API key, JWT)
- [ ] Rate limiting
- [ ] OpenAPI documentation

## Dépendances

- **Bloqué par**: F001, F005
- **Débloque**: F026

## Fichiers Impactés

```
cmd/awf/main.go
internal/interfaces/api/server.go
internal/interfaces/api/handlers/
internal/interfaces/api/middleware/
internal/interfaces/api/dto/
api/openapi.yaml
```

## Tâches Techniques

- [ ] Choose HTTP framework
  - [ ] Option: net/http + gorilla/mux
  - [ ] Option: chi
  - [ ] Option: gin
- [ ] Implement API server
  - [ ] Configuration (port, TLS)
  - [ ] Graceful shutdown
  - [ ] Health endpoint
- [ ] Implement handlers
  - [ ] POST /workflows/{name}/run
  - [ ] GET /executions
  - [ ] GET /executions/{id}
  - [ ] GET /executions/{id}/logs (SSE)
  - [ ] DELETE /executions/{id}
  - [ ] GET /workflows
  - [ ] GET /workflows/{name}
  - [ ] POST /workflows (upload)
- [ ] Define DTOs
  - [ ] RunRequest, RunResponse
  - [ ] ExecutionStatus
  - [ ] WorkflowInfo
- [ ] Implement middleware
  - [ ] Authentication
  - [ ] Rate limiting
  - [ ] Request logging
  - [ ] CORS
- [ ] Write OpenAPI spec
- [ ] Write integration tests

## Notes

API examples:
```bash
# Run workflow
curl -X POST http://localhost:8080/workflows/analyze-code/run \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"inputs": {"file_path": "app.py"}}'

# Response
{
  "execution_id": "analyze-code-20231209-143022",
  "status": "running",
  "started_at": "2023-12-09T14:30:22Z"
}

# Check status
curl http://localhost:8080/executions/analyze-code-20231209-143022

# Stream logs (SSE)
curl http://localhost:8080/executions/analyze-code-20231209-143022/logs
```

Server config:
```yaml
api:
  enabled: true
  port: 8080
  tls:
    enabled: true
    cert: /path/to/cert.pem
    key: /path/to/key.pem
  auth:
    type: jwt  # or api_key
    secret: ${API_SECRET}
  rate_limit:
    requests_per_minute: 60
```
