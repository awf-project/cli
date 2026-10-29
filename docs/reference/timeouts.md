---
title: "Timeouts"
---

AWF applies execution deadlines only when a timeout is explicitly configured or inherited from a template. Omitting a timeout does not silently impose a five-second, thirty-second, or five-minute limit.

## Timeout values

For workflow steps and agent steps, `timeout` accepts integer seconds or a Go duration string:

```yaml
states:
  build:
    type: step
    command: ./build.sh
    timeout: 10m

  review:
    type: agent
    provider: openai_compatible
    prompt: Review the build output.
    # No timeout: execution follows the caller's context.
```

The rules are:

| Configuration | Behavior |
|---|---|
| Omitted | No local execution deadline is added. |
| `0` | No local execution deadline; on a templated step, explicitly disables an inherited template timeout. |
| Positive value | Adds a local deadline in seconds or the specified Go duration. |
| Negative value | Rejected during validation. |

`call_workflow`, custom plugin steps, conversation turns, agent providers, and `http.request` follow the same opt-in execution policy. The HTTP operation accepts a positive integer number of seconds when a request-specific deadline is needed.

## Inherited deadlines

Child execution always remains attached to its caller's context. A shorter parent deadline wins, even if a child timeout is zero or longer. A child timeout cannot detach work from cancellation requested by the CLI, API, ACP server, or TUI.

When a workflow template supplies a timeout, an omitted step timeout inherits that template value. Use an explicit `timeout: 0` on the step to disable the inherited local limit.

## HTTP connection setup

The shared HTTP client retains technical setup limits: 30 seconds to establish
a TCP connection and 10 seconds to complete the TLS handshake. A shorter caller
deadline or cancellation ends the request sooner, including while setup is in progress.

These limits end when their setup phase completes. They do not impose a total
request deadline or limit waiting for response headers or reading the response
body. Long-running HTTP operations and agent responses can continue after setup
without an execution timeout.

Setting a workflow timeout to zero disables the local execution budget; it does
not disable these connection setup safeguards. Explicitly supplied HTTP clients
and transports retain their own configured limits.

## Timeout errors and cancellation

AWF distinguishes a locally configured timeout from an earlier parent deadline and from user cancellation:

- A local timeout reports the configured duration and identifies the step as the timeout owner.
- An earlier parent deadline preserves the parent deadline error without inventing local timeout details.
- User cancellation preserves `context.Canceled` rather than being reported as a timeout.

External agent CLIs, remote APIs, proxies, and hosting platforms may enforce their own limits. AWF cannot remove deadlines imposed outside the workflow process.

## See also

- [Workflow Syntax](../user-guide/workflow-syntax.md) - Timeout fields for each state type
- [Conversation Steps](../user-guide/conversation-steps.md) - Interactive and cross-step agent sessions
- [Plugins](../user-guide/plugins.md) - HTTP operations and custom plugin steps
