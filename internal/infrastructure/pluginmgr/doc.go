// Package plugin implements infrastructure adapters for the plugin system.
//
// The plugin package provides concrete implementations for plugin discovery, loading,
// validation, registration, and lifecycle management. It enables AWF workflows to extend
// functionality through external RPC-based plugins (operations, filters, transformers)
// without modifying core code. The package handles manifest parsing, version compatibility
// checking, state persistence, and operation registry integration.
//
// # Architecture Role
//
// In the hexagonal architecture:
//   - Implements plugin loading and lifecycle management (infrastructure adapters)
//   - Provides OperationRegistry for runtime operation lookup and registration
//   - Integrates with domain/ports.CommandExecutor for plugin discovery
//   - Application layer orchestrates workflow execution via registered plugin operations
//   - Domain layer defines operation contracts without plugin coupling
//
// All plugin components use atomic file operations and thread-safe registries to support
// concurrent plugin loading during workflow initialization. The manifest parser validates
// semver constraints and capability declarations before plugin activation.
//
// # Plugin Management
//
// ## RPCPluginManager (rpc_manager.go)
//
// Plugin lifecycle orchestration:
//   - Discover: Scan plugins directory for valid manifests
//   - Load: Initialize plugin RPC client and register operations
//   - Init: Call plugin initialization hook with configuration
//   - Shutdown: Gracefully stop a running plugin
//   - ShutdownAll: Cleanup all active plugins on process termination
//   - Get: Retrieve loaded plugin by name
//   - List: Enumerate active plugin names
//
// ## Loader (loader.go)
//
// Plugin discovery and validation:
//   - DiscoverPlugins: Find plugin.awf manifests in plugins directory
//   - LoadPlugin: Parse manifest, validate constraints, prepare RPC client
//   - ValidatePlugin: Check manifest schema, version compatibility, capability declarations
//
// # Registry and Discovery
//
// ## OperationRegistry (registry.go)
//
// Runtime operation registration and lookup:
//   - RegisterOperation: Add plugin-provided operation (thread-safe)
//   - UnregisterOperation: Remove operation by name
//   - GetOperation: Retrieve operation implementation by name
//   - GetPluginOperations: List operations provided by a plugin
//   - UnregisterPluginOperations: Remove all operations from a plugin
//   - Count: Total registered operations
//   - Clear: Reset registry state
//
// # Manifest and Metadata
//
// ## ManifestParser (manifest_parser.go)
//
// Plugin metadata parsing:
//   - ParseManifest: Read plugin.awf YAML manifest
//   - Validates: name, version, awf_version (semver constraints), capabilities
//   - Supports metadata: author, description, license, homepage
//
// Capabilities: operation, filter, transform (plugin feature declarations)
//
// # State Persistence
//
// ## JSONPluginStateStore (state_store.go)
//
// Plugin state persistence:
//   - Save: Write plugin state to JSON file (atomic via temp file + rename)
//   - Load: Read plugin state from JSON file
//   - SetEnabled: Enable/disable plugin by name
//   - IsEnabled: Check if plugin is enabled
//   - GetConfig: Retrieve plugin-specific configuration
//   - SetConfig: Update plugin-specific configuration
//   - GetState: Access full plugin state
//   - ListDisabled: Enumerate disabled plugins
//
// Uses file locking to prevent concurrent modification during workflow execution.
//
// # Version Handling
//
// ## Version (version.go)
//
// Semantic versioning support:
//   - ParseVersion: Parse semver string (e.g., "1.2.3")
//   - ParseConstraint: Parse version constraint (e.g., ">=1.0.0", "~1.2", "^2.0")
//   - CheckVersionConstraint: Validate version against constraint
//   - IsCompatible: Check plugin compatibility with AWF version
//   - Compare: Semver comparison (major.minor.patch)
//
// Operators: =, !=, >, >=, <, <=, ~ (tilde range), ^ (caret range)
package pluginmgr
