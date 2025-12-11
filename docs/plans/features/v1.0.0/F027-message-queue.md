# F027: Message Queue Support

## Metadata
- **Status**: backlog
- **Phase**: 5-Interfaces
- **Version**: v1.0.0
- **Priority**: medium
- **Estimation**: L

## Description

Integrate with message queues for async workflow triggering. Support consuming from queues and publishing results. Enable event-driven architectures and decoupled systems integration.

## Acceptance Criteria

- [ ] Consume messages from queue to trigger workflows
- [ ] Publish workflow results to queue
- [ ] Support multiple queue backends (Redis, RabbitMQ, SQS)
- [ ] Message acknowledgment on completion
- [ ] Dead letter queue for failures
- [ ] Configurable concurrency
- [ ] Graceful shutdown with in-flight handling

## Dependencies

- **Blocked by**: F001
- **Unblocks**: _none_

## Impacted Files

```
internal/interfaces/mq/consumer.go
internal/interfaces/mq/publisher.go
internal/interfaces/mq/adapters/redis.go
internal/interfaces/mq/adapters/rabbitmq.go
internal/interfaces/mq/adapters/sqs.go
cmd/awf/commands/worker.go
```

## Technical Tasks

- [ ] Define MessageQueue interface
  - [ ] Consume(ctx, handler)
  - [ ] Publish(ctx, message)
  - [ ] Ack(messageId)
  - [ ] Nack(messageId)
- [ ] Define Message struct
  - [ ] ID
  - [ ] WorkflowName
  - [ ] Inputs
  - [ ] Metadata
  - [ ] ReplyTo (for results)
- [ ] Implement Redis adapter
  - [ ] BRPOPLPUSH for reliable consumption
  - [ ] LPUSH for publishing
- [ ] Implement RabbitMQ adapter
  - [ ] AMQP connection
  - [ ] Consumer with manual ack
  - [ ] Publisher
- [ ] Implement SQS adapter
  - [ ] AWS SDK
  - [ ] Long polling
  - [ ] Visibility timeout
- [ ] Implement Worker command
  - [ ] `awf worker --queue=workflows`
  - [ ] Consume messages
  - [ ] Execute workflows
  - [ ] Publish results
- [ ] Handle concurrency
  - [ ] Worker pool
  - [ ] Max concurrent executions
- [ ] Handle failures
  - [ ] Retry policy
  - [ ] Dead letter queue
- [ ] Write tests

## Notes

Worker command:
```bash
# Start worker
awf worker --queue=workflows --concurrency=5

# With Redis
AWF_MQ_REDIS_URL=redis://localhost:6379 awf worker

# With RabbitMQ
AWF_MQ_AMQP_URL=amqp://guest:guest@localhost:5672 awf worker
```

Message format:
```json
{
  "id": "msg-123",
  "workflow": "analyze-code",
  "inputs": {
    "file_path": "app.py"
  },
  "reply_to": "results-queue",
  "metadata": {
    "correlation_id": "req-456"
  }
}
```

Configuration:
```yaml
mq:
  type: redis  # redis | rabbitmq | sqs
  redis:
    url: redis://localhost:6379
    queue: awf-workflows
  concurrency: 5
  dead_letter:
    enabled: true
    queue: awf-dlq
```
