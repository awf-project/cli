# F006: Logging JSON Structuré

## Metadata
- **Statut**: done
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priorité**: high
- **Estimation**: S

## Description

Implement structured JSON logging for workflow execution. Log all significant events (step start, step complete, errors) with contextual metadata. Support both file output and console output. Enable log analysis and debugging.

## Critères d'Acceptance

- [x] Log events in JSON format
- [x] Include timestamp, level, message, workflow context
- [x] Log to file (storage/logs/)
- [x] Optional console output
- [x] Configurable log level
- [x] Mask sensitive values (API keys, passwords)
- [x] Implements Logger port interface

## Dépendances

- **Bloqué par**: F001
- **Débloque**: F008

## Fichiers Impactés

```
internal/domain/ports/logger.go              # Existing - Logger interface
internal/infrastructure/logger/json_logger.go
internal/infrastructure/logger/json_logger_test.go
internal/infrastructure/logger/console_logger.go
internal/infrastructure/logger/console_logger_test.go
internal/infrastructure/logger/multi_logger.go
internal/infrastructure/logger/multi_logger_test.go
internal/infrastructure/logger/masker.go
internal/infrastructure/logger/masker_test.go
storage/logs/
```

## Tâches Techniques

- [x] Define Logger port interface
  - [x] Info(msg, fields)
  - [x] Error(msg, error, fields)
  - [x] Debug(msg, fields)
  - [x] Warn(msg, fields)
  - [x] WithContext(ctx) for workflow context
- [x] Implement JSONLogger
  - [x] Use zap for structured logging
  - [x] Write to file
  - [x] Include standard fields (timestamp, level, workflow_id, step)
- [x] Implement ConsoleLogger
  - [x] Human-readable output
  - [x] Color support
  - [x] Respect log level
- [x] Implement secret masking
  - [x] Detect keys matching SECRET_*, API_KEY*, PASSWORD*
  - [x] Replace values with ***
- [x] Log file naming: `{workflow-name}-{workflow-id}.log`
- [x] Write unit tests

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
