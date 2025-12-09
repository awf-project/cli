# F021: Plugin System

## Metadata
- **Statut**: backlog
- **Phase**: 4-Extensibility
- **Version**: v0.4.0
- **Priorité**: high
- **Estimation**: XL

## Description

Implement a plugin system allowing third-party extensions. Support custom operations, commands, and validators. Use Go plugin or RPC-based approach for extensibility without recompiling AWF core.

## Critères d'Acceptance

- [ ] Plugin discovery from plugins/ directory
- [ ] Plugin lifecycle (load, init, shutdown)
- [ ] Plugin manifest (name, version, capabilities)
- [ ] Plugins can register custom operations
- [ ] Plugins can register CLI commands
- [ ] Plugins can register validators
- [ ] Sandboxed plugin execution
- [ ] Plugin versioning and compatibility

## Dépendances

- **Bloqué par**: F001, F003
- **Débloque**: F022

## Fichiers Impactés

```
internal/plugin/manager.go
internal/plugin/loader.go
internal/plugin/registry.go
internal/plugin/manifest.go
pkg/plugin/sdk/
plugins/
```

## Tâches Techniques

- [ ] Choose plugin architecture
  - [ ] Option A: Go plugin (*.so, limited portability)
  - [ ] Option B: HashiCorp go-plugin (RPC)
  - [ ] Option C: WASM (future-proof)
- [ ] Define Plugin interface
  - [ ] Name() string
  - [ ] Version() string
  - [ ] Init(config) error
  - [ ] Shutdown() error
  - [ ] Operations() []Operation
  - [ ] Commands() []Command
- [ ] Define plugin manifest format
  - [ ] name, version, description
  - [ ] awf_version_constraint
  - [ ] capabilities
  - [ ] config schema
- [ ] Implement PluginManager
  - [ ] Discover plugins
  - [ ] Load and validate
  - [ ] Initialize
  - [ ] Registry access
  - [ ] Shutdown
- [ ] Implement PluginRegistry
  - [ ] Register operations
  - [ ] Register commands
  - [ ] Lookup by name
- [ ] Create Plugin SDK package
  - [ ] Base interfaces
  - [ ] Helper functions
  - [ ] Testing utilities
- [ ] Implement example plugin
- [ ] Write documentation
- [ ] Write tests

## Notes

Plugin manifest (plugin.yaml):
```yaml
name: awf-plugin-slack
version: 1.0.0
description: Slack notifications for AWF
awf_version: ">=0.4.0"
capabilities:
  - operations
config:
  webhook_url:
    type: string
    required: true
```

Consider HashiCorp go-plugin for RPC-based plugins - more portable than Go's plugin package.
