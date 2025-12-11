# F028: Webhooks

## Metadata
- **Status**: backlog
- **Phase**: 5-Interfaces
- **Version**: v1.0.0
- **Priority**: medium
- **Estimation**: M

## Description

Implement webhook notifications for workflow events. Send HTTP callbacks on workflow start, completion, failure, and step transitions. Enable integration with external systems for monitoring and automation.

## Acceptance Criteria

- [ ] Configure webhooks per workflow
- [ ] Events: workflow_start, workflow_end, workflow_error, step_complete
- [ ] POST to configured URL with event payload
- [ ] Signature for security (HMAC)
- [ ] Retry on failure
- [ ] Configurable timeout
- [ ] Global and per-workflow webhooks

## Dependencies

- **Blocked by**: F001, F008
- **Unblocks**: _none_

## Impacted Files

```
internal/infrastructure/webhook/dispatcher.go
internal/infrastructure/webhook/signer.go
internal/domain/webhook/config.go
internal/application/webhook_service.go
```

## Technical Tasks

- [ ] Define WebhookConfig struct
  - [ ] URL
  - [ ] Events []string
  - [ ] Secret (for signing)
  - [ ] Timeout
  - [ ] Retry config
- [ ] Define WebhookEvent struct
  - [ ] Type (workflow_start, etc.)
  - [ ] WorkflowID
  - [ ] WorkflowName
  - [ ] Timestamp
  - [ ] Payload (event-specific data)
- [ ] Implement WebhookDispatcher
  - [ ] Queue events
  - [ ] Send HTTP POST
  - [ ] Handle retries
  - [ ] Log outcomes
- [ ] Implement request signing
  - [ ] HMAC-SHA256 signature
  - [ ] X-AWF-Signature header
  - [ ] Timestamp in header
- [ ] Integrate with execution flow
  - [ ] Emit events at lifecycle points
  - [ ] Non-blocking dispatch
- [ ] Support global webhooks
  - [ ] In settings.yaml
  - [ ] Apply to all workflows
- [ ] Support per-workflow webhooks
  - [ ] In workflow definition
  - [ ] Override global
- [ ] Write unit tests
- [ ] Write integration tests

## Notes

Webhook configuration:
```yaml
# Global (settings.yaml)
webhooks:
  - url: https://hooks.example.com/awf
    events: [workflow_end, workflow_error]
    secret: ${WEBHOOK_SECRET}
    timeout: 10s
    retry:
      max_attempts: 3
      delay: 5s

# Per-workflow
webhooks:
  - url: https://slack.example.com/hook
    events: [workflow_error]
    secret: ${SLACK_WEBHOOK_SECRET}
```

Webhook payload:
```json
{
  "event": "workflow_end",
  "workflow_id": "analyze-code-20231209-143022",
  "workflow_name": "analyze-code",
  "timestamp": "2023-12-09T14:31:15Z",
  "payload": {
    "status": "success",
    "duration": 53.2,
    "exit_code": 0
  }
}
```

Signature verification (receiver):
```python
import hmac
expected = hmac.new(secret, body, 'sha256').hexdigest()
actual = request.headers['X-AWF-Signature']
if not hmac.compare_digest(expected, actual):
    raise SecurityError("Invalid signature")
```
