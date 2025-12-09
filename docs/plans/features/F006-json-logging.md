# F006: Logging JSON Structuré

## Metadata
- **Statut**: backlog
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priorité**: high
- **Estimation**: S

## Description

Implement structured JSON logging for workflow execution. Log all significant events (step start, step complete, errors) with contextual metadata. Support both file output and console output. Enable log analysis and debugging.

## Critères d'Acceptance

- [ ] Log events in JSON format
- [ ] Include timestamp, level, message, workflow context
- [ ] Log to file (storage/logs/)
- [ ] Optional console output
- [ ] Configurable log level
- [ ] Mask sensitive values (API keys, passwords)
- [ ] Implements Logger port interface

## Dépendances

- **Bloqué par**: F001
- **Débloque**: F008

## Fichiers Impactés

```
internal/infrastructure/logger/json_logger.go
internal/infrastructure/logger/console_logger.go
internal/domain/ports/logger.go
storage/logs/
```

## Tâches Techniques

- [ ] Define Logger port interface
  - [ ] Info(msg, fields)
  - [ ] Error(msg, error, fields)
  - [ ] Debug(msg, fields)
  - [ ] Warn(msg, fields)
  - [ ] WithContext(ctx) for workflow context
- [ ] Implement JSONLogger
  - [ ] Use zap for structured logging
  - [ ] Write to file
  - [ ] Include standard fields (timestamp, level, workflow_id, step)
- [ ] Implement ConsoleLogger
  - [ ] Human-readable output
  - [ ] Color support
  - [ ] Respect log level
- [ ] Implement secret masking
  - [ ] Detect keys matching SECRET_*, API_KEY*, PASSWORD*
  - [ ] Replace values with ***
- [ ] Log file naming: `{workflow-name}-{workflow-id}.log`
- [ ] Write unit tests

## Notes

Log entry structure:
```json
{
  "timestamp": "2023-12-09T14:30:22Z",
  "level": "info",
  "message": "step_completed",
  "workflow_id": "analyze-code-20231209-143022",
  "workflow_name": "analyze-code",
  "step": "validate",
  "duration": 1.2,
  "exit_code": 0
}
```

Use zap for performance. Configure via workflow logging section.
