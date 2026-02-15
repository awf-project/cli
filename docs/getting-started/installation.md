# Installation

## Prerequisites

**Required:**
- Go 1.21 or later
- Make (for building from source)

**Optional (for development):**
- [golangci-lint](https://golangci-lint.run/) - Linter

### Installing golangci-lint

```bash
# Arch Linux
paru -S golangci-lint

# macOS
brew install golangci-lint

# Other (via Go)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

## Quick Install

The fastest way to install AWF:

```bash
go install github.com/awf-project/awf/cmd/awf@latest
```

This installs the `awf` binary to your `$GOPATH/bin` directory.

## From Source

For the latest development version or to contribute:

```bash
# Clone the repository
git clone https://github.com/awf-project/awf.git
cd awf

# Build the binary
make build

# Install to /usr/local/bin (optional)
make install
```

The binary will be available at `./bin/awf` after building.

## Verify Installation

```bash
awf version
```

Expected output:
```
awf version X.Y.Z
```

## Shell Completion

Generate shell autocompletion scripts:

```bash
# Bash
awf completion bash > /etc/bash_completion.d/awf

# Zsh
awf completion zsh > "${fpath[1]}/_awf"

# Fish
awf completion fish > ~/.config/fish/completions/awf.fish

# PowerShell
awf completion powershell > awf.ps1
```

## Dependencies

AWF uses these Go packages:

| Package | Purpose |
|---------|---------|
| `spf13/cobra` | CLI framework |
| `gopkg.in/yaml.v3` | YAML parsing |
| `fatih/color` | Terminal colors |
| `google/uuid` | UUID generation |
| `golang.org/x/sync/errgroup` | Parallel execution |
| `modernc.org/sqlite` | History storage (SQLite) |
| `expr-lang/expr` | Expression evaluation |

## Next Steps

- [Quick Start](quickstart.md) - Create your first workflow
- [Commands](../user-guide/commands.md) - Learn all CLI commands
