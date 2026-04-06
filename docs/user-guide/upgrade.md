---
title: "Upgrading AWF"
---

The `awf upgrade` command updates AWF to the latest version directly from GitHub releases.

## Check for Updates

```bash
awf upgrade --check
```

This reports whether a newer version is available without making any changes.

## Upgrade to Latest

```bash
awf upgrade
```

Downloads the latest stable release, verifies its SHA256 checksum, and replaces the current binary atomically.

## Install a Specific Version

```bash
awf upgrade --version v0.5.0
```

Installs the specified version, allowing both upgrades and downgrades.

## Force Upgrade

```bash
awf upgrade --force
```

Skips version comparison and package manager detection. Required when:
- Running a development build (`awf version` shows "dev")
- Binary is installed via a package manager (homebrew, snap, nix)
- You want to reinstall the same version

## Authentication

GitHub API has a rate limit of 60 requests/hour for unauthenticated users. For higher limits:

```bash
export GITHUB_TOKEN=ghp_your_token_here
awf upgrade --check
```

AWF also tries `gh auth token` automatically if the GitHub CLI is installed.

## Flags

| Flag | Description |
|------|-------------|
| `--check` | Check for updates without installing |
| `--force` | Force upgrade (skip version/package manager checks) |
| `--version` | Install a specific version (e.g., `v0.5.0`) |

## Troubleshooting

### Permission Denied

If you see a permission error, the binary directory is not writable:

```bash
sudo awf upgrade
```

### Package Manager Warning

If AWF was installed via homebrew, snap, or nix, `awf upgrade` will warn you. Use your package manager instead, or pass `--force` to override.

### Dev Build

Development builds (compiled from source) cannot determine their version. Use `--force`:

```bash
awf upgrade --force
```

## See Also

- [Commands](commands.md) — CLI command reference
- [Workflow Packs](workflow-packs.md) — Installing and managing workflow packs
- [Plugins](plugins.md) — Plugin installation and management
