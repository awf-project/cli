# awf-plugin-echo

Example AWF plugin demonstrating the `sdk.Serve()` pattern. Exposes a single `echo` operation that returns its input text unchanged.

## Build and Install

```bash
make install
```

This builds the plugin binary and installs it alongside `plugin.yaml` into `~/.local/share/awf/plugins/awf-plugin-echo/`.

## Operations

### `echo`

Returns the input text, optionally prefixed.

| Input | Type | Required | Description |
|-------|------|----------|-------------|
| `text` | string | Yes | Text to echo |
| `prefix` | string | No | Optional prefix prepended to the output |

| Output | Type | Description |
|--------|------|-------------|
| `output` | string | Echoed text (with optional prefix) |
| `text` | string | Original input text |
| `prefix` | string | Prefix used (empty string if none) |

## Usage in Workflows

```yaml
name: echo-example
version: "1.0.0"

states:
  initial: say_hello

  say_hello:
    type: operation
    operation: awf-plugin-echo.echo
    inputs:
      text: "Hello, AWF!"
      prefix: ">>>"
    on_success: done
    on_failure: error

  done:
    type: terminal
    status: success

  error:
    type: terminal
    status: failure
```

Enable the plugin first:

```bash
awf plugin enable awf-plugin-echo
awf run echo-example
```

## Plugin Structure

```go
package main

import (
    "context"

    "github.com/awf-project/cli/pkg/plugin/sdk"
)

type EchoPlugin struct {
    sdk.BasePlugin // provides Name(), Version(), Init(), Shutdown()
}

func (p *EchoPlugin) Operations() []string {
    return []string{"echo"}
}

func (p *EchoPlugin) HandleOperation(_ context.Context, name string, inputs map[string]any) (*sdk.OperationResult, error) {
    text := sdk.GetStringDefault(inputs, "text", "")
    return sdk.NewSuccessResult(text, nil), nil
}

func main() {
    sdk.Serve(&EchoPlugin{
        BasePlugin: sdk.BasePlugin{
            PluginName:    "echo",
            PluginVersion: "1.0.0",
        },
    })
}
```
