---
title: "Workflow Packs"
---

Workflow packs are distributable bundles of YAML workflows, prompt files, and scripts that can be installed from GitHub Releases. They enable sharing reusable workflow collections across projects and teams.

## Overview

A workflow pack contains:
- **Workflows** — YAML workflow definitions (`.yaml` files)
- **Prompts** — Prompt files referenced by workflows
- **Scripts** — Script files referenced by workflows
- **Manifest** — `manifest.yaml` describing the pack metadata and contents

Packs are installed via `awf workflow install` and executed via `awf run pack/workflow` namespace syntax.

## Quick Start

```bash
# Install a workflow pack
awf workflow install myorg/awf-workflow-speckit

# List installed packs
awf workflow list

# Run a workflow from the pack
awf run speckit/specify --input file=main.go

# View pack details
awf workflow info speckit

# Update to latest version
awf workflow update speckit
```

## Searching for Packs

Discover available workflow packs on GitHub:

```bash
# Browse all packs (tagged with awf-workflow topic)
awf workflow search

# Search by keyword
awf workflow search speckit

# JSON output for scripting
awf workflow search --output=json
```

Results show the repository name, star count, and description. Set `GITHUB_TOKEN` for higher API rate limits.

## Installing Packs

Install packs from GitHub Releases:

```bash
# Install latest version
awf workflow install myorg/awf-workflow-speckit

# Install specific version
awf workflow install myorg/awf-workflow-speckit@1.2.0

# Install with version constraint
awf workflow install myorg/awf-workflow-speckit --version ">=1.0.0 <2.0.0"

# Install globally (available to all projects)
awf workflow install myorg/awf-workflow-speckit --global

# Force reinstall
awf workflow install myorg/awf-workflow-speckit --force
```

AWF downloads the release archive, verifies the SHA-256 checksum, validates the manifest (including AWF version compatibility), and installs atomically.

### Installation Locations

| Scope | Directory | Flag |
|-------|-----------|------|
| Local (default) | `.awf/workflow-packs/<name>/` | — |
| Global | `~/.local/share/awf/workflow-packs/<name>/` | `--global` |

Local packs are project-specific. Global packs are available to all projects.

### Plugin Dependencies

If a pack declares plugin dependencies in its manifest, non-blocking warnings are emitted during installation:

```
Warning: pack requires plugin "jira" (>=1.0.0) — install with: awf plugin install <owner>/jira
```

The pack installs regardless of missing plugins. Install required plugins separately with `awf plugin install`.

## Listing Packs

View all installed packs:

```bash
awf workflow list
```

This discovers packs from local and global directories, deduplicating by name (local takes precedence over global). A `(local)` pseudo-entry appears when `.awf/workflows/` contains local workflow files.

## Viewing Pack Details

Inspect an installed pack:

```bash
awf workflow info speckit
```

Displays:
- Pack name, version, description, author, license
- List of available workflows
- Plugin dependency warnings with install commands
- Embedded README content (if present)

## Updating Packs

Update a pack to the latest release from its source repository:

```bash
# Update a specific pack
awf workflow update speckit

# Update all installed packs
awf workflow update --all
```

The update checks `state.json` for the source repository, queries GitHub Releases for newer versions, and performs an atomic replacement. User overrides in `.awf/prompts/<pack>/` and `.awf/scripts/<pack>/` are preserved.

## Removing Packs

Remove an installed pack:

```bash
awf workflow remove speckit
```

This removes the pack directory and all its contents. The command searches local then global directories.

## Running Pack Workflows

Pack workflows use `pack/workflow` namespace syntax:

```bash
# Run a workflow from an installed pack
awf run speckit/specify --input file=main.go

# Dry-run to preview
awf run speckit/specify --dry-run

# Interactive mode
awf run speckit/specify --interactive
```

Local workflows (without a `/` separator) are unaffected.

### Resource Resolution Order

Pack workflows resolve prompts and scripts using a 3-tier hierarchy:

1. `.awf/prompts/<pack>/...` — user override (highest priority)
2. `.awf/workflow-packs/<pack>/prompts/...` — pack embedded
3. `~/.config/awf/prompts/...` — global XDG (lowest priority)

This allows customizing pack behavior without modifying pack files directly.

## Pack Workflows in `awf list`

The `awf list` command includes pack workflows alongside local and global workflows:

```bash
awf list
```

Pack workflows appear with `pack/workflow` namespace prefix and `pack` source label. The `awf list` and `awf workflow list` commands show the same packs and versions.

## Manifest Format

Pack authors create a `manifest.yaml` at the pack root:

```yaml
name: speckit
version: "1.0.0"
description: "Specification-driven development workflows"
author: "Your Name"
license: "MIT"
awf_version: ">=0.5.0"
workflows:
  - specify
  - implement
  - review
plugins:
  jira: ">=1.0.0"
```

### Manifest Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Pack identifier (lowercase, hyphens: `^[a-z][a-z0-9-]*$`) |
| `version` | string | Yes | Semantic version |
| `description` | string | No | Brief description |
| `author` | string | No | Pack author |
| `license` | string | No | License identifier |
| `awf_version` | string | Yes | AWF CLI version constraint (semver range) |
| `workflows` | array | Yes | List of workflow names (must have corresponding `.yaml` files) |
| `plugins` | object | No | Required plugins with version constraints |

### Pack Directory Structure

```
awf-workflow-speckit/
├── manifest.yaml          # Pack metadata
├── README.md              # Optional documentation
├── workflows/
│   ├── specify.yaml       # Workflow definitions
│   └── implement.yaml
├── prompts/
│   └── system.md          # Prompt files
└── scripts/
    └── setup.sh           # Script files
```

## Distributing Packs via GitHub Releases

Pack authors publish `.tar.gz` archives to GitHub Releases. Workflow packs are platform-independent (no OS/architecture suffix needed).

### Release Assets

Each release must include:
- A `.tar.gz` archive (e.g., `awf-workflow-speckit_1.0.0.tar.gz`)
- A `checksums.txt` file with SHA-256 hashes

### GoReleaser Configuration

```yaml
project_name: awf-workflow-speckit

archives:
  - format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}"
    files:
      - manifest.yaml
      - README.md
      - workflows/*
      - prompts/*
      - scripts/*

checksum:
  name_template: checksums.txt
  algorithm: sha256

release:
  github:
    owner: your-org
    name: awf-workflow-speckit
```

### Authentication

AWF uses the same authentication chain as plugin installation:

1. `GITHUB_TOKEN` environment variable
2. `gh auth token` output
3. Unauthenticated requests (lower rate limits)

## See Also

- [Commands](commands.md) — CLI command reference for all workflow subcommands
- [Plugins](plugins.md) — Plugin system documentation
- [Workflow Syntax](workflow-syntax.md) — YAML workflow definition reference
